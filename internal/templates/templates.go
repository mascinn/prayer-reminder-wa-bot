package templates

import (
	"fmt"
	"strings"
	"time"

	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
	"remind-bot/internal/storage"
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

_Waktu masuk ±15 menit lagi. Dimohon kepada petugas untuk bersiap-siap._

━━━━━━━━━━━━━━━━━
ℹ️ *React jika petugas tidak menjalankan tugas:*
👆 = Adzan
👇 = Imam
✌️ = Keduanya
_(Batas laporan/ralat s.d 23:59 WIB)_`,
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

*Dimohon kepada petugas untuk mempersiapkan diri dan bangun lebih awal.*

━━━━━━━━━━━━━━━━━
ℹ️ *React jika petugas tidak menjalankan tugas:*
👆 = Adzan
👇 = Imam
✌️ = Keduanya
_(Batas laporan/ralat s.d 23:59 WIB besok)_`,
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
🔗 *Catat Setoran di SISEKA:*
🌐 https://siseka-wasii.vercel.app/
👤 Username : *bph*
🔑 Password : *barengbareng*
────────────────────────
_Pengingat otomatis dikirim setiap Senin s.d. Jumat pukul 15:00 WIB._`,
		strings.Join(scheduleLines, "\n"),
		todaySection,
	)

	return ReminderMessage{
		Text:         msg,
		MentionedJID: uniqueJIDs(jids),
	}
}

// BuildKultumMonthlyScheduleView formats the calendar schedule of kultum speakers for the current month.
func BuildKultumMonthlyScheduleView(reg *phonebook.Registry, now time.Time) ReminderMessage {
	var jids []string

	daysInMonth := matrix.DaysInMonth(now)
	todayDay := now.Day()
	tomorrow := now.AddDate(0, 0, 1)
	speakerTomorrow := matrix.GetKultumSpeakerForDay(tomorrow.Day())

	var lines []string
	for day := 1; day <= daysInMonth; day++ {
		speaker := matrix.GetKultumSpeakerForDay(day)
		if day == todayDay {
			lines = append(lines, fmt.Sprintf("👉 %d. %s (Hari Ini)", day, speaker))
		} else {
			lines = append(lines, fmt.Sprintf("   %d. %s", day, speaker))
		}
	}

	tagTomorrow := formatOfficerTag(reg, speakerTomorrow, &jids)

	msg := fmt.Sprintf(`🎙️ *JADWAL KULTUM SUBUH*
Masjid Al-Wasii - UNILA
Periode: %s
────────────────────────
%s
────────────────────────
_Petugas kultum berikutnya (Besok):_ %s`,
		matrix.FormatIndonesianMonthYear(now),
		strings.Join(lines, "\n"),
		tagTomorrow,
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

// BuildMonthlyRecap formats the monthly attendance overview for !rekap command.
func BuildMonthlyRecap(recap *storage.MonthlyRecapData) string {
	if recap == nil || len(recap.OfficerStats) == 0 {
		return fmt.Sprintf("📊 *REKAP KEAKTIFAN PETUGAS SHOLAT*\nPeriode: %02d/%04d\n\n_Belum ada data jadwal/rekap yang tercatat pada bulan ini._", recap.Month, recap.Year)
	}

	t := time.Date(recap.Year, time.Month(recap.Month), 1, 0, 0, 0, 0, time.UTC)
	periodeStr := matrix.FormatIndonesianMonthYear(t)

	var sb strings.Builder
	sb.WriteString("📊 *REKAP KEAKTIFAN PETUGAS SHOLAT*\n")
	sb.WriteString(fmt.Sprintf("Periode: %s\n\n", periodeStr))
	sb.WriteString("Daftar Petugas:\n")

	for i, off := range recap.OfficerStats {
		icon := "✅"
		if off.TotalMissed > 0 {
			icon = "⚠️"
		}
		sb.WriteString(fmt.Sprintf("%d. %-8s : %d/%d (%.0f%%) | %dx Tidak Menjalankan %s\n",
			i+1, off.OfficerName, off.TotalExecuted, off.TotalAssigned, off.Percentage, off.TotalMissed, icon))
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Sholat Tercatat : %d Waktu\n", recap.TotalDuties))
	sb.WriteString(fmt.Sprintf("Tingkat Pelaksanaan   : %.1f%%\n\n", recap.OverallPct))
	sb.WriteString("_Ketik `!rekap detail [nama]` untuk rincian waktu sholat._")

	return sb.String()
}

// BuildOfficerDetailRecap formats the detail of missed duties for an officer in a month.
func BuildOfficerDetailRecap(year int, month int, officerName string, missed []storage.MissedDutyDetail, totalAssigned, totalExecuted int) string {
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	periodeStr := matrix.FormatIndonesianMonthYear(t)
	totalMissed := totalAssigned - totalExecuted

	var sb strings.Builder
	sb.WriteString("📋 *RINCIAN TIDAK MENJALANKAN TUGAS*\n")
	sb.WriteString(fmt.Sprintf("Petugas: *%s*\n", officerName))
	sb.WriteString(fmt.Sprintf("Periode: %s\n", periodeStr))
	sb.WriteString(fmt.Sprintf("Total Jadwal: %d | Menjalankan: %d | Tidak: %d\n\n", totalAssigned, totalExecuted, totalMissed))

	if len(missed) == 0 {
		if totalAssigned == 0 {
			sb.WriteString(fmt.Sprintf("_Tidak ada jadwal tugas atas nama *%s* pada periode ini._\n", officerName))
		} else {
			sb.WriteString("🎉 *Alhamdulillah! Semua jadwal tugas pada periode ini telah dilaksanakan dengan baik.* ✅\n")
		}
		return sb.String()
	}

	sb.WriteString("Daftar Sholat yang Tidak Dilaksanakan:\n")
	for i, m := range missed {
		dateFormatted := m.PrayerDate
		if parsedDate, err := time.Parse("2006-01-02", m.PrayerDate); err == nil {
			dateFormatted = parsedDate.Format("02/01/2006")
		}
		sb.WriteString(fmt.Sprintf("%d. 📅 %s - %s (Tugas: %s)\n", i+1, dateFormatted, m.PrayerName, m.Role))
		if m.ReporterJID != "" {
			rep := strings.TrimSuffix(m.ReporterJID, "@s.whatsapp.net")
			if idx := strings.Index(rep, "@"); idx != -1 {
				rep = rep[:idx]
			}
			sb.WriteString(fmt.Sprintf("   └ Dilaporkan oleh: %s\n", rep))
		}
	}

	return sb.String()
}

