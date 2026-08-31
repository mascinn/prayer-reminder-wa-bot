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

// BuildDaytimePrayerReminder formats the 10-minute reminder for Zhuhur, Ashar, Maghrib, Isya.
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

_Waktu masuk ±10 menit lagi. Dimohon kepada petugas untuk bersiap-siap._`,
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
_%s • Subuh %s WIB_

Petugas:
📢 Adzan  : %s
👳 Imam   : %s
🎙️ Kultum : %s

_Dimohon kepada petugas untuk mempersiapkan diri dan bangun lebih awal._`,
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

// formatOfficerTag returns `@628xxx (DisplayName)` if found in phonebook, or just `@Name`
func formatOfficerTag(reg *phonebook.Registry, name string, jids *[]string) string {
	if name == "" {
		return "-"
	}
	if reg == nil {
		return "@" + name
	}
	m, ok := reg.Find(name)
	if ok && m.Phone != "" {
		jid := m.WhatsAppJID()
		if jid != "" {
			*jids = append(*jids, jid)
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
