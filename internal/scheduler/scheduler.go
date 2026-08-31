package scheduler

import (
	"context"
	"fmt"
	"log"
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

	if err := s.bot.SendReminder(ctx, targetJID, msg); err != nil {
		log.Printf("[Scheduler] Failed to send daytime reminder for %s: %v", prayer, err)
		_ = s.storage.LogReminder(string(prayer), prayerTime.Format("15:04"), duty.Adzan, duty.Imam, "", fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] %s reminder successfully dispatched to WhatsApp (%s).", prayer, targetJID)
		_ = s.storage.LogReminder(string(prayer), prayerTime.Format("15:04"), duty.Adzan, duty.Imam, "", "SUCCESS")
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

	// Atomically get and advance Kultum speaker index
	usedIdx, err := s.storage.AdvanceKultumIndex(matrix.KultumQueueLen())
	if err != nil {
		log.Printf("[Scheduler] Error advancing kultum index: %v, using default index 0", err)
		usedIdx = 0
	}
	speaker := matrix.GetKultumSpeaker(usedIdx)

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

	if err := s.bot.SendReminder(ctx, targetJID, msg); err != nil {
		log.Printf("[Scheduler] Failed to send Subuh/Kultum reminder: %v", err)
		_ = s.storage.LogReminder("subuh_kultum", subuhTimeStr, subuhDuty.Adzan, subuhDuty.Imam, speaker, fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] Subuh/Kultum reminder sent successfully to %s. Speaker: %s (Queue Index: %d)", targetJID, speaker, usedIdx)
		_ = s.storage.LogReminder("subuh_kultum", subuhTimeStr, subuhDuty.Adzan, subuhDuty.Imam, speaker, "SUCCESS")
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

	if err := s.bot.SendReminder(ctx, targetJID, msg); err != nil {
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

	if err := s.bot.SendReminder(ctx, targetJID, msg); err != nil {
		log.Printf("[Scheduler] Failed to send Canteen reminder: %v", err)
		_ = s.storage.LogReminder("canteen_collection", "15:00", strings.Join(officers, ", "), "-", "-", fmt.Sprintf("FAILED: %v", err))
	} else {
		log.Printf("[Scheduler] Canteen reminder dispatched successfully to %s.", targetJID)
		_ = s.storage.LogReminder("canteen_collection", "15:00", strings.Join(officers, ", "), "-", "-", "SUCCESS")
	}
}

func (s *Scheduler) registerCommands() {
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

	// !jadwal
	s.bot.RegisterCommand("jadwal", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		now := time.Now().In(s.cfg.Location)
		parsed, _, err := s.apiClient.FetchJadwal(s.cfg.Location, now)
		rawTimes := make(map[matrix.PrayerName]string)
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

		daySchedule := matrix.GetDaySchedule(now.Weekday())
		kIdx, _ := s.storage.GetKultumIndex()
		speaker := matrix.GetKultumSpeaker(kIdx)

		msg := templates.BuildJadwalPreview(s.phonebook, now, rawTimes, daySchedule, speaker)
		return "", &msg, nil
	})

	// !kultum
	s.bot.RegisterCommand("kultum", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		currentIdx, _ := s.storage.GetKultumIndex()
		var sb strings.Builder
		sb.WriteString("🎙️ *ROTASI KULTUM SUBUH (ROUND-ROBIN)*\n")
		sb.WriteString("Masjid Al-Wasii - UNILA\n────────────────────────\n")
		for i, name := range matrix.KultumQueue() {
			marker := "  "
			if i == currentIdx {
				marker = "👉 "
			}
			tag := s.phonebook.FormatMention(name)
			sb.WriteString(fmt.Sprintf("%s%d. %s (%s)\n", marker, i+1, tag, name))
		}
		sb.WriteString("────────────────────────\n")
		sb.WriteString(fmt.Sprintf("_Petugas kultum berikutnya: *%s*_\n", matrix.GetKultumSpeaker(currentIdx)))
		return sb.String(), nil, nil
	})

	// !setkultum [1-10 / nama]
	s.bot.RegisterCommand("setkultum", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		if len(args) == 0 {
			return "ℹ️ *Penggunaan:* `!setkultum [1-10 / nama]`\nContoh: `!setkultum 1` atau `!setkultum iskandar`", nil, nil
		}

		arg := strings.ToLower(args[0])
		targetIdx := -1

		// Check if it's a number 1-10
		for i, name := range matrix.KultumQueue() {
			numStr := fmt.Sprintf("%d", i+1)
			if arg == numStr || strings.ToLower(name) == arg {
				targetIdx = i
				break
			}
			// Check aliases in phonebook
			if m, ok := s.phonebook.Find(arg); ok && strings.EqualFold(m.DisplayName, name) {
				targetIdx = i
				break
			}
		}

		if targetIdx == -1 {
			return fmt.Sprintf("⚠️ Nama/nomor antrean %q tidak ditemukan di daftar kultum.", args[0]), nil, nil
		}

		if err := s.storage.SetKultumIndex(targetIdx); err != nil {
			return fmt.Sprintf("⚠️ Gagal mengubah antrean kultum: %v", err), nil, nil
		}

		speaker := matrix.GetKultumSpeaker(targetIdx)
		tag := s.phonebook.FormatMention(speaker)
		return fmt.Sprintf("✅ *Antrean Kultum Berhasil Diubah!*\n👉 Giliran berikutnya: *%d. %s* (%s)\n_Akan bertugas pada jadwal Subuh berikutnya._", targetIdx+1, tag, speaker), nil, nil
	})

	// !test [prayer_name]
	s.bot.RegisterCommand("test", func(ctx context.Context, chatJID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error) {
		if len(args) == 0 {
			return "ℹ️ *Penggunaan:* `!test [subuh|zhuhur|ashar|maghrib|isya|jumat]`", nil, nil
		}

		now := time.Now().In(s.cfg.Location)
		target := strings.ToLower(args[0])

		switch target {
		case "subuh":
			tomorrow := now.AddDate(0, 0, 1)
			sched := matrix.GetDaySchedule(tomorrow.Weekday())
			kIdx, _ := s.storage.GetKultumIndex()
			speaker := matrix.GetKultumSpeaker(kIdx)

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
}
