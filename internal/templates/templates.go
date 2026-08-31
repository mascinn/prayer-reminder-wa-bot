package templates

import (
	"fmt"
	"strings"
	"time"

	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
)

// ReminderMessage contains the final formatted text and list of JIDs to mention.
type ReminderMessage struct {
	Text         string
	MentionedJID []string
}

// BuildDaytimePrayerReminder formats the 15-minute reminder for Zhuhur, Ashar, Maghrib, Isya.
func BuildDaytimePrayerReminder(
	reg *phonebook.Registry,
	prayer matrix.PrayerName,
	prayerTime time.Time,
	duty matrix.DutyAssignment,
) ReminderMessage {
	var jids []string

	adzanTag := formatOfficerTag(reg, duty.Adzan, &jids)
	imamTag := formatOfficerTag(reg, duty.Imam, &jids)

	timeStr := prayerTime.Format("15:04")

	msg := fmt.Sprintf(`🕌 *PENGINGAT SHOLAT %s (%s WIB)*

Petugas:
📢 Adzan : %s
👳 Imam  : %s

_Waktu masuk ±15 menit lagi. Dimohon kepada petugas untuk bersiap-siap._`,
		strings.ToUpper(string(prayer)),
		timeStr,
		adzanTag,
		imamTag,
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// BuildSubuhKultumReminder formats the 20:30 WIB night reminder for tomorrow's Subuh & Kultum.
func BuildSubuhKultumReminder(
	reg *phonebook.Registry,
	tomorrowDate time.Time,
	subuhTimeStr string,
	duty matrix.DutyAssignment,
	kultumSpeaker string,
) ReminderMessage {
	var jids []string

	adzanTag := formatOfficerTag(reg, duty.Adzan, &jids)
	imamTag := formatOfficerTag(reg, duty.Imam, &jids)
	kultumTag := formatOfficerTag(reg, kultumSpeaker, &jids)

	if subuhTimeStr == "" {
		subuhTimeStr = "~04:45"
	}

	msg := fmt.Sprintf(`🌙 *PENGINGAT SUBUH & KULTUM BESOK*
%s
Subuh %s WIB

Petugas:
📢 Adzan  : %s
👳 Imam   : %s
🎙️ Kultum : %s

*Dimohon kepada petugas untuk mempersiapkan diri dan bangun lebih awal.*`,
		matrix.FormatIndonesianDate(tomorrowDate),
		subuhTimeStr,
		adzanTag,
		imamTag,
		kultumTag,
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// BuildFridayReminder formats the Thursday 21:00 WIB Friday preparation reminder.
func BuildFridayReminder(reg *phonebook.Registry, fridayDate time.Time) ReminderMessage {
	var jids []string

	msg := fmt.Sprintf(`🕌 *PENGINGAT PERSIAPAN SHOLAT JUM'AT*
📅 *Besok :* %s

Dimohon kepada seluruh ikhwah marbot untuk mempersiapkan fasilitas dan protokol Sholat Jum'at (kebersihan masjid, sound system/mic, tempat wudhu, dan kesiapan adzan).

_Semoga Allah senantiasa memberikan keberkahan atas setiap keikhlasan yang kita tunaikan._`,
		matrix.FormatIndonesianDate(fridayDate),
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// BuildJadwalPreview generates a full overview of today's schedule and duty matrix for `!jadwal` command.
func BuildJadwalPreview(
	reg *phonebook.Registry,
	today time.Time,
	rawTimes map[matrix.PrayerName]string,
	schedule matrix.DaySchedule,
	kultumSpeaker string,
) ReminderMessage {
	var jids []string

	subuhAdzan := formatOfficerTag(reg, schedule.Subuh.Adzan, &jids)
	subuhImam := formatOfficerTag(reg, schedule.Subuh.Imam, &jids)
	asharAdzan := formatOfficerTag(reg, schedule.Ashar.Adzan, &jids)
	asharImam := formatOfficerTag(reg, schedule.Ashar.Imam, &jids)
	maghribAdzan := formatOfficerTag(reg, schedule.Maghrib.Adzan, &jids)
	maghribImam := formatOfficerTag(reg, schedule.Maghrib.Imam, &jids)
	isyaAdzan := formatOfficerTag(reg, schedule.Isya.Adzan, &jids)
	isyaImam := formatOfficerTag(reg, schedule.Isya.Imam, &jids)

	var zhuhurBlock string
	if schedule.Zhuhur.Skipped {
		zhuhurBlock = "🕌 *Zhuhur/Jumat:* Sholat Jum'at Berjamaah\n"
	} else {
		zhuhurAdzan := formatOfficerTag(reg, schedule.Zhuhur.Adzan, &jids)
		zhuhurImam := formatOfficerTag(reg, schedule.Zhuhur.Imam, &jids)
		zhuhurBlock = fmt.Sprintf("☀️ *Zhuhur (%s WIB):*\n  • Adzan: %s\n  • Imam: %s\n", rawTimes[matrix.PrayerZhuhur], zhuhurAdzan, zhuhurImam)
	}

	kultumTag := formatOfficerTag(reg, kultumSpeaker, &jids)

	msg := fmt.Sprintf(`📋 *JADWAL & PETUGAS HARI INI*
📅 *%s*
────────────────────────
🌅 *Subuh (%s WIB):*
  • Adzan: %s
  • Imam: %s
  • Kultum: %s

%s
⛅ *Ashar (%s WIB):*
  • Adzan: %s
  • Imam: %s

🌇 *Maghrib (%s WIB):*
  • Adzan: %s
  • Imam: %s

🌌 *Isya (%s WIB):*
  • Adzan: %s
  • Imam: %s
────────────────────────
_Semoga Allah senantiasa memberikan keberkahan atas setiap keikhlasan yang kita tunaikan._`,
		matrix.FormatIndonesianDate(today),
		rawTimes[matrix.PrayerSubuh],
		subuhAdzan,
		subuhImam,
		kultumTag,
		zhuhurBlock,
		rawTimes[matrix.PrayerAshar],
		asharAdzan,
		asharImam,
		rawTimes[matrix.PrayerMaghrib],
		maghribAdzan,
		maghribImam,
		rawTimes[matrix.PrayerIsya],
		isyaAdzan,
		isyaImam,
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// BuildCanteenReminder formats the 15:00 WIB canteen cash collection notification.
func BuildCanteenReminder(reg *phonebook.Registry, date time.Time, officers []string) ReminderMessage {
	var jids []string
	var officerLines []string

	for _, officer := range officers {
		tag := formatOfficerTag(reg, officer, &jids)
		officerLines = append(officerLines, "👥 "+tag)
	}

	officersText := strings.Join(officerLines, "\n")
	if len(officerLines) == 0 {
		officersText = "_Tidak ada jadwal piket kantin hari ini._"
	}

	msg := fmt.Sprintf(`🍱 *PENGINGAT TARIK SETORAN KANTIN*
%s

Petugas:
%s

🔗 *Catat Setoran di SISEKA:*
🌐 https://siseka-wasii.vercel.app/
👤 Username : *bph*
🔑 Password : *barengbareng*

*Dimohon kepada petugas untuk segera melakukan penarikan dan pencatatan setoran kantin sore ini.*`,
		matrix.FormatIndonesianDate(date),
		officersText,
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// BuildCanteenScheduleView formats the full weekly schedule and highlights today's officers.
func BuildCanteenScheduleView(reg *phonebook.Registry, now time.Time, weeklySchedule map[time.Weekday][]string) ReminderMessage {
	var jids []string

	days := []struct {
		weekday time.Weekday
		name    string
	}{
		{time.Monday, "Senin"},
		{time.Tuesday, "Selasa"},
		{time.Wednesday, "Rabu"},
		{time.Thursday, "Kamis"},
		{time.Friday, "Jumat"},
	}

	var scheduleLines []string
	for _, d := range days {
		officers := weeklySchedule[d.weekday]
		var tagList []string
		for _, off := range officers {
			tag := formatOfficerTag(reg, off, &jids)
			tagList = append(tagList, tag)
		}
		tagsStr := strings.Join(tagList, ", ")
		if len(tagList) == 0 {
			tagsStr = "-"
		}
		prefix := "•"
		if d.weekday == now.Weekday() {
			prefix = "👉"
		}
		scheduleLines = append(scheduleLines, fmt.Sprintf("%s *%s* : %s", prefix, d.name, tagsStr))
	}

	todayOfficers := weeklySchedule[now.Weekday()]
	var todaySection string
	if len(todayOfficers) > 0 {
		var todayTags []string
		for _, off := range todayOfficers {
			tag := formatOfficerTag(reg, off, &jids)
			todayTags = append(todayTags, "👥 "+tag)
		}
		todaySection = fmt.Sprintf("\n────────────────────────\n👉 *Petugas Hari Ini (%s):*\n%s\n────────────────────────",
			matrix.FormatIndonesianDate(now), strings.Join(todayTags, "\n"))
	} else {
		todaySection = "\n────────────────────────\n_Tidak ada jadwal tarik kantin untuk hari ini (Sabtu/Ahad libur)._\n────────────────────────"
	}

	msg := fmt.Sprintf(`🍱 *JADWAL TARIK SETORAN KANTIN*
Masjid Al-Wasii - UNILA
────────────────────────
%s
%s
_Pengingat otomatis dikirim setiap Senin s.d. Jumat pukul 15:00 WIB._`,
		strings.Join(scheduleLines, "\n"),
		todaySection,
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// formatOfficerTag returns `@628xxx (DisplayName)` (or `@628xxx @628yyy (DisplayName)`) if found in phonebook, or just `@Name`
func formatOfficerTag(reg *phonebook.Registry, name string, jids *[]string) string {
	if name == "" {
		return "-"
	}
	if reg == nil {
		return "@" + name
	}
	m, ok := reg.Find(name)
	if ok && len(m.AllPhones()) > 0 {
		for _, jid := range m.WhatsAppJIDs() {
			if jid != "" {
				*jids = append(*jids, jid)
			}
		}
		return fmt.Sprintf("%s (%s)", m.MentionTag(), m.DisplayName)
	}
	return "@" + name
}

func uniqueJIDs(jids []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, jid := range jids {
		if jid != "" && !seen[jid] {
			seen[jid] = true
			result = append(result, jid)
		}
	}
	return result
}
