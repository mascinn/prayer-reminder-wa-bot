package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.mau.fi/whatsmeow/types"

	"remind-bot/internal/api"
	"remind-bot/internal/config"
	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
	"remind-bot/internal/storage"
	"remind-bot/internal/templates"
	"remind-bot/internal/whatsapp"
)

// Scheduler coordinates cron jobs, daily API fetches, and in-memory prayer timers.
type Scheduler struct {
	cfg       *config.Config
	storage   *storage.Storage
	phonebook *phonebook.Registry
	apiClient *api.Client
	bot       *whatsapp.Bot

	cronEngine *cron.Cron
	timers     []*time.Timer
	timersMu   sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewScheduler creates and configures the scheduler with all cron triggers and commands.
func NewScheduler(
	cfg *config.Config,
	storage *storage.Storage,
	reg *phonebook.Registry,
	bot *whatsapp.Bot,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize cron with timezone location
	cronEngine := cron.New(cron.WithLocation(cfg.Location), cron.WithSeconds())

	s := &Scheduler{
		cfg:        cfg,
		storage:    storage,
		phonebook:  reg,
		apiClient:  api.NewClient(cfg.CityID),
		bot:        bot,
		cronEngine: cronEngine,
		ctx:        ctx,
		cancel:     cancel,
	}

	s.registerCommands()
	return s
}

// Start registers all recurring cron triggers and arms today's prayer schedule immediately.
func (s *Scheduler) Start() error {
	log.Printf("[Scheduler] Starting scheduler in timezone %s...", s.cfg.Location.String())

	// 1. Daily API Fetch at 00:01:00 WIB
	_, err := s.cronEngine.AddFunc("0 1 0 * * *", func() {
		log.Println("[Scheduler] Cron triggered: Daily prayer schedule fetch at 00:01 WIB")
		s.RunDailyFetch()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule daily fetch cron: %w", err)
	}

	// 2. Subuh & Kultum Reminder at 20:30:00 WIB every night
	_, err = s.cronEngine.AddFunc("0 30 20 * * *", func() {
		log.Println("[Scheduler] Cron triggered: Subuh & Kultum reminder at 20:30 WIB")
		s.RunSubuhKultumReminder()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule subuh kultum cron: %w", err)
	}

	// 3. Friday Prayer Reminder at 21:00:00 WIB every Thursday (Day 4)
	_, err = s.cronEngine.AddFunc("0 0 21 * * 4", func() {
		log.Println("[Scheduler] Cron triggered: Friday reminder check at 21:00 WIB (Thursday)")
		s.RunFridayReminder()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule Friday reminder cron: %w", err)
	}

	// 4. Canteen Collection Reminder at 15:00:00 WIB every Monday to Friday (Days 1-5)
	_, err = s.cronEngine.AddFunc("0 0 15 * * 1-5", func() {
		log.Println("[Scheduler] Cron triggered: Canteen collection reminder at 15:00 WIB")
		s.RunCanteenReminder()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule Canteen reminder cron: %w", err)
	}

	s.cronEngine.Start()

	// Perform initial fetch and arm today's remaining prayers on startup
	go s.RunDailyFetch()

	return nil
}

// Stop terminates all active timers and cron jobs.
func (s *Scheduler) Stop() {
	s.cancel()
	s.cronEngine.Stop()

	s.timersMu.Lock()
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil
	s.timersMu.Unlock()

	log.Println("[Scheduler] Stopped all cron jobs and active timers.")
}

// RunDailyFetch fetches today's prayer times from the API and arms timers for Zhuhur, Ashar, Maghrib, Isya.
func (s *Scheduler) RunDailyFetch() {
	now := time.Now().In(s.cfg.Location)
	log.Printf("[Scheduler] Fetching prayer times for %s...", matrix.FormatIndonesianDate(now))

	parsedTimes, rawJSON, err := s.apiClient.FetchJadwal(s.cfg.Location, now)
	if err != nil {
		log.Printf("[Scheduler] Error fetching prayer times from API: %v. Attempting to use cached schedule...", err)
		dateKey := fmt.Sprintf("%04d-%02d-%02d", now.Year(), now.Month(), now.Day())
		if cached, cErr := s.storage.GetCachedJadwal(dateKey); cErr == nil && cached != "" {
			if p, pErr := api.ParseRawSchedule(cached, now, s.cfg.Location); pErr == nil {
				parsedTimes = p
				log.Println("[Scheduler] Successfully loaded schedule from local SQLite cache.")
			}
		}
	} else if rawJSON != "" {
		dateKey := fmt.Sprintf("%04d-%02d-%02d", now.Year(), now.Month(), now.Day())
		_ = s.storage.CacheJadwal(dateKey, rawJSON)
	}

	if parsedTimes == nil {
		log.Printf("[Scheduler] Critical: Could not obtain prayer schedule for today %s", matrix.FormatIndonesianDate(now))
		return
	}

	s.armDaytimeTimers(now, parsedTimes)
}

func (s *Scheduler) armDaytimeTimers(now time.Time, pt *api.ParsedPrayerTimes) {
	s.timersMu.Lock()
	defer s.timersMu.Unlock()

	// Clear previous timers
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil

	todayWeekday := now.Weekday()
	daySchedule := matrix.GetDaySchedule(todayWeekday)

	prayers := []struct {
		name matrix.PrayerName
		time time.Time
		duty matrix.DutyAssignment
	}{
		{name: matrix.PrayerZhuhur, time: pt.Zhuhur, duty: daySchedule.Zhuhur},
		{name: matrix.PrayerAshar, time: pt.Ashar, duty: daySchedule.Ashar},
		{name: matrix.PrayerMaghrib, time: pt.Maghrib, duty: daySchedule.Maghrib},
		{name: matrix.PrayerIsya, time: pt.Isya, duty: daySchedule.Isya},
	}

	for _, p := range prayers {
		if p.duty.Skipped {
			log.Printf("[Scheduler] Skipping %s on %s (duty marked skipped / Friday)", p.name, daySchedule.DayName)
			continue
		}

		// Trigger reminder 15 minutes before prayer time
		reminderTime := p.time.Add(-15 * time.Minute)
		delay := reminderTime.Sub(now)

		if delay > 0 {
			prayerName := p.name
			actualTime := p.time
			duty := p.duty

			log.Printf("[Scheduler] Scheduled %s reminder for %s (in %v)",
				prayerName, reminderTime.Format("15:04:05"), delay.Round(time.Second))

			timer := time.AfterFunc(delay, func() {
				s.sendDaytimeReminder(prayerName, actualTime, duty)
			})
			s.timers = append(s.timers, timer)
		} else {
			log.Printf("[Scheduler] %s reminder time (%s) has already passed for today.",
				p.name, reminderTime.Format("15:04:05"))
		}
	}
}

func (s *Scheduler) getTargetJID() string {
	if val, err := s.storage.GetState("target_jid"); err == nil && val != "" {
		return val
	}
	return s.cfg.TargetJID
}

func (s *Scheduler) sendDaytimeReminder(prayer matrix.PrayerName, prayerTime time.Time, duty matrix.DutyAssignment) {
	log.Printf("[Scheduler] Sending 15-min reminder for %s...", prayer)
	msg := templates.BuildDaytimePrayerReminder(s.phonebook, prayer, prayerTime, duty)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	targetJID := s.getTargetJID()
	if targetJID == "" {
		log.Printf("[Scheduler] Target JID is not set, skipping %s reminder.", prayer)
		return
	}

	msgID, err := s.bot.SendReminder(ctx, targetJID, msg)
	if err != nil {
		log.Printf("[Scheduler] Failed to send daytime reminder for %s: %v", prayer, err)
		_ = s.storage.LogReminder(string(prayer), prayerTime.Format("15:04"), duty.Adzan, duty.Imam, "", fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] %s reminder successfully dispatched to WhatsApp (%s). Message ID: %s", prayer, targetJID, msgID)
		_ = s.storage.LogReminder(string(prayer), prayerTime.Format("15:04"), duty.Adzan, duty.Imam, "", "SUCCESS")

		nowInLoc := time.Now().In(s.cfg.Location)
		prayerDateStr := nowInLoc.Format("2006-01-02")
		cutoffTime := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 23, 59, 59, 0, s.cfg.Location)

		if err := s.storage.SaveDutyRecord(string(msgID), prayerDateStr, string(prayer), duty.Adzan, duty.Imam, cutoffTime); err != nil {
			log.Printf("[Scheduler] Failed to save duty record for msg %s: %v", msgID, err)
		}
	}
}

// RunSubuhKultumReminder executes the 20:30 WIB nightly Subuh & Kultum notification for tomorrow.
func (s *Scheduler) RunSubuhKultumReminder() {
	now := time.Now().In(s.cfg.Location)
	tomorrow := now.AddDate(0, 0, 1)
	tomorrowWeekday := tomorrow.Weekday()

	log.Printf("[Scheduler] Preparing Subuh & Kultum reminder for tomorrow (%s)...", matrix.FormatIndonesianDate(tomorrow))

	// Fetch tomorrow's Subuh duty from matrix
	tomorrowSchedule := matrix.GetDaySchedule(tomorrowWeekday)
	subuhDuty := tomorrowSchedule.Subuh

	// Kultum speaker is calculated from calendar day of tomorrow (Day 1 resets to Iskandar)
	speaker := matrix.GetKultumSpeakerForDay(tomorrow.Day())

	// Fetch tomorrow's Subuh time from API (or fallback)
	var subuhTimeStr string
	if pt, _, err := s.apiClient.FetchJadwal(s.cfg.Location, tomorrow); err == nil && pt != nil {
		subuhTimeStr = pt.Subuh.Format("15:04")
	}

	msg := templates.BuildSubuhKultumReminder(s.phonebook, tomorrow, subuhTimeStr, subuhDuty, speaker)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	targetJID := s.getTargetJID()
	if targetJID == "" {
		log.Println("[Scheduler] Target JID is not set, skipping Subuh/Kultum reminder.")
		return
	}

	msgID, err := s.bot.SendReminder(ctx, targetJID, msg)
	if err != nil {
		log.Printf("[Scheduler] Failed to send Subuh/Kultum reminder: %v", err)
		_ = s.storage.LogReminder("subuh_kultum", subuhTimeStr, subuhDuty.Adzan, subuhDuty.Imam, speaker, fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] Subuh/Kultum reminder sent successfully to %s. Speaker: %s (Day: %d), Message ID: %s", targetJID, speaker, tomorrow.Day(), msgID)
		_ = s.storage.LogReminder("subuh_kultum", subuhTimeStr, subuhDuty.Adzan, subuhDuty.Imam, speaker, "SUCCESS")

		prayerDateStr := tomorrow.Format("2006-01-02")
		cutoffTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 23, 59, 59, 0, s.cfg.Location)

		if err := s.storage.SaveDutyRecord(string(msgID), prayerDateStr, "Subuh", subuhDuty.Adzan, subuhDuty.Imam, cutoffTime); err != nil {
			log.Printf("[Scheduler] Failed to save Subuh duty record for msg %s: %v", msgID, err)
		}
	}
}

// RunFridayReminder sends Friday preparation reminder if enabled.
func (s *Scheduler) RunFridayReminder() {
	if !s.cfg.EnableJumatReminder {
		log.Println("[Scheduler] Friday reminder is disabled (ENABLE_JUMAT_REMINDER=false). Skipping.")
		return
	}

	now := time.Now().In(s.cfg.Location)
	fridayDate := now.AddDate(0, 0, 1) // Tomorrow is Friday

	log.Printf("[Scheduler] Sending Friday Reminder for %s...", matrix.FormatIndonesianDate(fridayDate))
	msg := templates.BuildFridayReminder(s.phonebook, fridayDate)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	targetJID := s.getTargetJID()
	if targetJID == "" {
		log.Println("[Scheduler] Target JID is not set, skipping Friday reminder.")
		return
	}

	if _, err := s.bot.SendReminder(ctx, targetJID, msg); err != nil {
		log.Printf("[Scheduler] Failed to send Friday reminder: %v", err)
		_ = s.storage.LogReminder("friday_prep", "21:00", "-", "-", "-", fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] Friday reminder dispatched successfully to %s.", targetJID)
		_ = s.storage.LogReminder("friday_prep", "21:00", "-", "-", "-", "SUCCESS")
	}
}

// RunCanteenReminder executes the 15:00 WIB Monday-Friday canteen collection reminder.
func (s *Scheduler) RunCanteenReminder() {
	now := time.Now().In(s.cfg.Location)
	weekday := now.Weekday()

	if weekday < time.Monday || weekday > time.Friday {
		log.Println("[Scheduler] Canteen reminder skipped (Weekend).")
		return
	}

	officers, err := s.storage.GetCanteenOfficers(weekday)
	if err != nil || len(officers) == 0 {
		log.Printf("[Scheduler] No canteen officers found for %s: %v", weekday, err)
		return
	}

	log.Printf("[Scheduler] Sending Canteen Reminder for %s (%s)...", matrix.FormatIndonesianDate(now), strings.Join(officers, ", "))
	msg := templates.BuildCanteenReminder(s.phonebook, now, officers)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	targetJID := s.getTargetJID()
	if targetJID == "" {
		log.Println("[Scheduler] Target JID is not set, skipping Canteen reminder.")
		return
	}

	if _, err := s.bot.SendReminder(ctx, targetJID, msg); err != nil {
		log.Printf("[Scheduler] Failed to send Canteen reminder: %v", err)
		_ = s.storage.LogReminder("canteen_collection", "15:00", strings.Join(officers, ", "), "-", "-", fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] Canteen reminder dispatched successfully to %s.", targetJID)
		_ = s.storage.LogReminder("canteen_collection", "15:00", strings.Join(officers, ", "), "-", "-", "SUCCESS")
	}
}

func (s *Scheduler) getDayPrayerTimes(t time.Time) map[matrix.PrayerName]string {
	rawTimes := make(map[matrix.PrayerName]string)
	parsed, _, err := s.apiClient.FetchJadwal(s.cfg.Location, t)
	if err == nil && parsed != nil {
		rawTimes[matrix.PrayerSubuh] = parsed.Subuh.Format("15:04")
		rawTimes[matrix.PrayerZhuhur] = parsed.Zhuhur.Format("15:04")
		rawTimes[matrix.PrayerAshar] = parsed.Ashar.Format("15:04")
		rawTimes[matrix.PrayerMaghrib] = parsed.Maghrib.Format("15:04")
		rawTimes[matrix.PrayerIsya] = parsed.Isya.Format("15:04")
	} else {
		rawTimes[matrix.PrayerSubuh] = "04:45"
		rawTimes[matrix.PrayerZhuhur] = "12:05"
		rawTimes[matrix.PrayerAshar] = "15:20"
		rawTimes[matrix.PrayerMaghrib] = "18:05"
		rawTimes[matrix.PrayerIsya] = "19:15"
	}
	return rawTimes
}

func (s *Scheduler) registerCommands() {
	// !menu / !help / !bantuan
	menuHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		return templates.BuildMenuGuide(), nil, nil
	}
	s.bot.RegisterCommand("menu", menuHandler)
	s.bot.RegisterCommand("help", menuHandler)
	s.bot.RegisterCommand("bantuan", menuHandler)

	// !ping
	s.bot.RegisterCommand("ping", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		return "🏓 *Pong!*\nBot Pengingat Sholat & Kultum Masjid Al-Wasii UNILA aktif 24/7.", nil, nil
	})

	// !jid
	s.bot.RegisterCommand("jid", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		currentTarget := s.getTargetJID()
		isCurrent := "Bukan Target"
		if chatJID.String() == currentTarget {
			isCurrent = "✅ Target Aktif Saat Ini"
		}
		res := fmt.Sprintf("📍 *Info JID WhatsApp*\n• Chat JID : `%s` (%s)\n• Pengirim : `%s`\n• Target Pengingat : `%s`\n\n_Ketik `!setgrup` di grup ini untuk menjadikan grup ini sebagai target pengingat sholat._",
			chatJID.String(), isCurrent, senderJID.String(), currentTarget)
		return res, nil, nil
	})

	// !setgrup / !settarget
	setGroupHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		if chatJID.Server != types.GroupServer {
			return "⚠️ Perintah ini harus diketik di dalam grup WhatsApp yang ingin dijadikan target pengingat.", nil, nil
		}
		if err := s.storage.SetState("target_jid", chatJID.String()); err != nil {
			return fmt.Sprintf("⚠️ Gagal menyimpan target grup: %v", err), nil, nil
		}
		return fmt.Sprintf("✅ *Grup Target Pengingat Berhasil Diubah!*\n• JID: `%s`\n\n_Mulai sekarang seluruh pengingat Sholat, Kultum Subuh, dan Jum'at otomatis dikirim ke grup ini._ 🕌", chatJID.String()), nil, nil
	}
	s.bot.RegisterCommand("setgrup", setGroupHandler)
	s.bot.RegisterCommand("settarget", setGroupHandler)

	// Single prayer handler creator
	singlePrayerHandler := func(prayer matrix.PrayerName) whatsapp.CommandHandlerFunc {
		return func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
			now := time.Now().In(s.cfg.Location)
			rawTimes := s.getDayPrayerTimes(now)
			sched := matrix.GetDaySchedule(now.Weekday())
			duty := sched.GetDuty(prayer)
			speaker := ""
			if prayer == matrix.PrayerSubuh {
				speaker = matrix.GetKultumSpeakerForDay(now.Day())
			}
			msg := templates.BuildSinglePrayerScheduleView(s.phonebook, now, prayer, rawTimes[prayer], duty, speaker)
			return "", &msg, nil
		}
	}

	// Register single prayer commands
	s.bot.RegisterCommand("subuh", singlePrayerHandler(matrix.PrayerSubuh))
	s.bot.RegisterCommand("fajr", singlePrayerHandler(matrix.PrayerSubuh))

	s.bot.RegisterCommand("zhuhur", singlePrayerHandler(matrix.PrayerZhuhur))
	s.bot.RegisterCommand("dzuhur", singlePrayerHandler(matrix.PrayerZhuhur))
	s.bot.RegisterCommand("dhuhur", singlePrayerHandler(matrix.PrayerZhuhur))
	s.bot.RegisterCommand("zuhur", singlePrayerHandler(matrix.PrayerZhuhur))
	s.bot.RegisterCommand("dhuhr", singlePrayerHandler(matrix.PrayerZhuhur))

	s.bot.RegisterCommand("ashar", singlePrayerHandler(matrix.PrayerAshar))
	s.bot.RegisterCommand("asar", singlePrayerHandler(matrix.PrayerAshar))
	s.bot.RegisterCommand("asr", singlePrayerHandler(matrix.PrayerAshar))

	s.bot.RegisterCommand("maghrib", singlePrayerHandler(matrix.PrayerMaghrib))
	s.bot.RegisterCommand("magrib", singlePrayerHandler(matrix.PrayerMaghrib))

	s.bot.RegisterCommand("isya", singlePrayerHandler(matrix.PrayerIsya))
	s.bot.RegisterCommand("isya'", singlePrayerHandler(matrix.PrayerIsya))
	s.bot.RegisterCommand("isha", singlePrayerHandler(matrix.PrayerIsya))

	// !besok (Jadwal sholat & kultum besok)
	besokHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)
		tomorrow := now.AddDate(0, 0, 1)
		rawTimes := s.getDayPrayerTimes(tomorrow)
		sched := matrix.GetDaySchedule(tomorrow.Weekday())
		speaker := matrix.GetKultumSpeakerForDay(tomorrow.Day())

		msg := templates.BuildJadwalPreview(s.phonebook, tomorrow, rawTimes, sched, speaker)
		return "", &msg, nil
	}
	s.bot.RegisterCommand("besok", besokHandler)

	// !jadwal (Harian / Hari Tertentu / Sholat Tertentu)
	s.bot.RegisterCommand("jadwal", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)

		if len(args) > 0 {
			argLower := strings.ToLower(args[0])

			// 1. !jadwal besok
			if argLower == "besok" || argLower == "tomorrow" {
				return besokHandler(ctx, chatJID, senderJID, args[1:])
			}

			// 2. !jadwal [hari] (misal: !jadwal senin, !jadwal jumat)
			if wd, ok := matrix.ParseIndonesianWeekday(argLower); ok {
				sched := matrix.GetDaySchedule(wd)
				// Preview template for specific weekday
				targetDate := now
				for targetDate.Weekday() != wd {
					targetDate = targetDate.AddDate(0, 0, 1)
				}
				rawTimes := s.getDayPrayerTimes(targetDate)
				speaker := matrix.GetKultumSpeakerForDay(targetDate.Day())
				msg := templates.BuildJadwalPreview(s.phonebook, targetDate, rawTimes, sched, speaker)
				return "", &msg, nil
			}

			// 3. !jadwal [sholat] (misal: !jadwal subuh, !jadwal zhuhur)
			normalizedPrayer := matrix.NormalizePrayerName(argLower)
			if normalizedPrayer == matrix.PrayerSubuh || normalizedPrayer == matrix.PrayerZhuhur ||
				normalizedPrayer == matrix.PrayerAshar || normalizedPrayer == matrix.PrayerMaghrib ||
				normalizedPrayer == matrix.PrayerIsya {
				handler := singlePrayerHandler(normalizedPrayer)
				return handler(ctx, chatJID, senderJID, args[1:])
			}
		}

		// Default: Today's schedule
		rawTimes := s.getDayPrayerTimes(now)
		daySchedule := matrix.GetDaySchedule(now.Weekday())
		speaker := matrix.GetKultumSpeakerForDay(now.Day())

		msg := templates.BuildJadwalPreview(s.phonebook, now, rawTimes, daySchedule, speaker)
		return "", &msg, nil
	})

	// !matriks / !jadwallengkap / !jadwalminggu
	matrixHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		weekly := matrix.GetFullWeeklyMatrix()
		return templates.BuildWeeklyMatrixView(weekly), nil, nil
	}
	s.bot.RegisterCommand("matriks", matrixHandler)
	s.bot.RegisterCommand("matrix", matrixHandler)
	s.bot.RegisterCommand("jadwallengkap", matrixHandler)
	s.bot.RegisterCommand("jadwalminggu", matrixHandler)

	// !tugas [nama] / !tugassaya / !jadwalsaya
	tugasHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)
		var officerName string

		if len(args) > 0 {
			officerName = strings.Join(args, " ")
		} else {
			// Check if sender's phone is in phonebook
			senderPhone := senderJID.User
			if m, ok := s.phonebook.FindByPhone(senderPhone); ok {
				officerName = m.DisplayName
			}
		}

		if strings.TrimSpace(officerName) == "" {
			return "ℹ️ *Format Perintah Tugas Pribadi:*\n• `!tugas [nama]` (Contoh: `!tugas Ahmad`)\n• Atau ketik `!tugas` langsung jika nomor Anda sudah terdaftar di database bot.", nil, nil
		}

		shifts := matrix.GetOfficerWeeklyDuties(officerName)
		kultumDays := matrix.GetOfficerKultumDaysInMonth(officerName, int(now.Month()), now.Year())

		msg := templates.BuildOfficerDutiesView(s.phonebook, officerName, shifts, kultumDays, now)
		return "", &msg, nil
	}
	s.bot.RegisterCommand("tugas", tugasHandler)
	s.bot.RegisterCommand("tugassaya", tugasHandler)
	s.bot.RegisterCommand("jadwalsaya", tugasHandler)

	// !kultum
	s.bot.RegisterCommand("kultum", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)
		msg := templates.BuildKultumMonthlyScheduleView(s.phonebook, now)
		return "", &msg, nil
	})

	// !test [prayer_name]
	s.bot.RegisterCommand("test", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		if len(args) == 0 {
			return "ℹ️ *Penggunaan:* `!test [subuh|zhuhur|ashar|maghrib|isya|jumat|kantin]`", nil, nil
		}

		now := time.Now().In(s.cfg.Location)
		target := strings.ToLower(args[0])

		switch target {
		case "subuh":
			tomorrow := now.AddDate(0, 0, 1)
			sched := matrix.GetDaySchedule(tomorrow.Weekday())
			speaker := matrix.GetKultumSpeakerForDay(tomorrow.Day())

			subuhTime := "04:44"
			if parsed, _, err := s.apiClient.FetchJadwal(s.cfg.Location, tomorrow); err == nil && parsed != nil {
				subuhTime = parsed.Subuh.Format("15:04")
			}
			msg := templates.BuildSubuhKultumReminder(s.phonebook, tomorrow, subuhTime, sched.Subuh, speaker)
			return "", &msg, nil

		case "zhuhur", "ashar", "maghrib", "isya":
			prayerName := matrix.NormalizePrayerName(target)
			sched := matrix.GetDaySchedule(now.Weekday())
			duty := sched.GetDuty(prayerName)

			prayerTime := now.Add(10 * time.Minute)
			if parsed, _, err := s.apiClient.FetchJadwal(s.cfg.Location, now); err == nil && parsed != nil {
				switch prayerName {
				case matrix.PrayerZhuhur:
					prayerTime = parsed.Zhuhur
				case matrix.PrayerAshar:
					prayerTime = parsed.Ashar
				case matrix.PrayerMaghrib:
					prayerTime = parsed.Maghrib
				case matrix.PrayerIsya:
					prayerTime = parsed.Isya
				}
			}
			msg := templates.BuildDaytimePrayerReminder(s.phonebook, prayerName, prayerTime, duty)
			return "", &msg, nil

		case "jumat":
			msg := templates.BuildFridayReminder(s.phonebook, now.AddDate(0, 0, 1))
			return "", &msg, nil

		case "kantin":
			officers, _ := s.storage.GetCanteenOfficers(now.Weekday())
			if len(officers) == 0 {
				officers = []string{"Ruzi", "Arjuna"} // preview sample
			}
			msg := templates.BuildCanteenReminder(s.phonebook, now, officers)
			return "", &msg, nil

		default:
			return fmt.Sprintf("⚠️ Target test %q tidak dikenali. Pilih: subuh, zhuhur, ashar, maghrib, isya, jumat, kantin", target), nil, nil
		}
	})

	// !kantin / !jadwalkantin
	canteenHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)
		weekly, err := s.storage.LoadCanteenSchedule()
		if err != nil || len(weekly) == 0 {
			return "⚠️ Data jadwal kantin belum tersedia di database.", nil, nil
		}
		msg := templates.BuildCanteenScheduleView(s.phonebook, now, weekly)
		return "", &msg, nil
	}
	s.bot.RegisterCommand("kantin", canteenHandler)
	s.bot.RegisterCommand("jadwalkantin", canteenHandler)

	// !rekap, !kehadiran, !keaktifan
	rekapHandler := func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)
		targetYear := now.Year()
		targetMonth := int(now.Month())

		// Subcommand: !rekap detail <nama> [bulan] [tahun]
		if len(args) >= 2 && strings.ToLower(args[0]) == "detail" {
			officerName := args[1]
			if len(args) >= 3 {
				if m, ok := parseMonth(args[2]); ok {
					targetMonth = m
				}
			}
			if len(args) >= 4 {
				if y, err := strconv.Atoi(args[3]); err == nil && y >= 2020 && y <= 2100 {
					targetYear = y
				}
			}

			missed, totalAssigned, totalExecuted, err := s.storage.GetMonthlyOfficerDetail(targetYear, targetMonth, officerName)
			if err != nil {
				return fmt.Sprintf("⚠️ Gagal mengambil detail keaktifan: %v", err), nil, nil
			}

			res := templates.BuildOfficerDetailRecap(targetYear, targetMonth, officerName, missed, totalAssigned, totalExecuted)
			return res, nil, nil
		}

		if len(args) == 1 {
			// Subcommand or single arg: !rekap [bulan]
			if m, ok := parseMonth(args[0]); ok {
				targetMonth = m
			} else if y, err := strconv.Atoi(args[0]); err == nil && y >= 2020 && y <= 2100 {
				targetYear = y
			} else {
				return "ℹ️ *Format Perintah Rekap:*\n• `!rekap` (Bulan ini)\n• `!rekap [bulan] [tahun]` (Contoh: `!rekap 08 2026`)\n• `!rekap detail [nama]` (Contoh: `!rekap detail Ahmad`)", nil, nil
			}
		} else if len(args) >= 2 {
			// !rekap [bulan] [tahun] e.g. !rekap 08 2026 or !rekap 2026 08
			m1, ok1 := parseMonth(args[0])
			y1, err1 := strconv.Atoi(args[1])
			if ok1 && err1 == nil && y1 >= 2020 && y1 <= 2100 {
				targetMonth = m1
				targetYear = y1
			} else {
				y2, err2 := strconv.Atoi(args[0])
				m2, ok2 := parseMonth(args[1])
				if ok2 && err2 == nil && y2 >= 2020 && y2 <= 2100 {
					targetMonth = m2
					targetYear = y2
				} else {
					return "ℹ️ *Format Perintah Rekap:*\n• `!rekap` (Bulan ini)\n• `!rekap [bulan] [tahun]` (Contoh: `!rekap 08 2026`)\n• `!rekap detail [nama]` (Contoh: `!rekap detail Ahmad`)", nil, nil
				}
			}
		}

		recapData, err := s.storage.GetMonthlyRecap(targetYear, targetMonth)
		if err != nil {
			return fmt.Sprintf("⚠️ Gagal mengambil data rekap: %v", err), nil, nil
		}

		res := templates.BuildMonthlyRecap(recapData)
		return res, nil, nil
	}
	s.bot.RegisterCommand("rekap", rekapHandler)
	s.bot.RegisterCommand("kehadiran", rekapHandler)
	s.bot.RegisterCommand("keaktifan", rekapHandler)
}

func parseMonth(token string) (int, bool) {
	token = strings.ToLower(strings.TrimSpace(token))
	if m, err := strconv.Atoi(token); err == nil && m >= 1 && m <= 12 {
		return m, true
	}
	months := map[string]int{
		"jan": 1, "januari": 1, "january": 1,
		"feb": 2, "februari": 2, "february": 2,
		"mar": 3, "maret": 3, "march": 3,
		"apr": 4, "april": 4,
		"mei": 5, "may": 5,
		"jun": 6, "juni": 6, "june": 6,
		"jul": 7, "juli": 7, "july": 7,
		"agu": 8, "agustus": 8, "agust": 8, "aug": 8, "august": 8,
		"sep": 9, "september": 9, "sept": 9,
		"okt": 10, "oktober": 10, "oct": 10, "october": 10,
		"nov": 11, "november": 11,
		"des": 12, "desember": 12, "dec": 12, "december": 12,
	}
	if m, ok := months[token]; ok {
		return m, true
	}
	return 0, false
}
