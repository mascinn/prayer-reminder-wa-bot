package matrix

import (
	"fmt"
	"strings"
	"time"
)

// PrayerName represents the Islamic prayer name.
type PrayerName string

const (
	PrayerSubuh   PrayerName = "Subuh"
	PrayerZhuhur  PrayerName = "Zhuhur"
	PrayerAshar   PrayerName = "Ashar"
	PrayerMaghrib PrayerName = "Maghrib"
	PrayerIsya    PrayerName = "Isya"
)

// DutyAssignment holds the assigned officer for Adzan and Imam.
type DutyAssignment struct {
	Adzan   string
	Imam    string
	Skipped bool // True for Friday Zhuhur
}

// DaySchedule holds duty assignments for all five daily prayers on a specific day.
type DaySchedule struct {
	DayName string
	Subuh   DutyAssignment
	Zhuhur  DutyAssignment
	Ashar   DutyAssignment
	Maghrib DutyAssignment
	Isya    DutyAssignment
}

// GetDuty returns the duty assignment for a specific prayer on that day.
func (ds DaySchedule) GetDuty(prayer PrayerName) DutyAssignment {
	switch prayer {
	case PrayerSubuh:
		return ds.Subuh
	case PrayerZhuhur:
		return ds.Zhuhur
	case PrayerAshar:
		return ds.Ashar
	case PrayerMaghrib:
		return ds.Maghrib
	case PrayerIsya:
		return ds.Isya
	default:
		return DutyAssignment{}
	}
}

// WeeklyMatrix maps time.Weekday (Sunday=0 ... Saturday=6) to DaySchedule.
var WeeklyMatrix = map[time.Weekday]DaySchedule{
	time.Monday: {
		DayName: "Senin",
		Subuh:   DutyAssignment{Adzan: "Imam", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Adzan: "Ruzi", Imam: "Basit"},
		Ashar:   DutyAssignment{Adzan: "Arjuna", Imam: "Ananda"},
		Maghrib: DutyAssignment{Adzan: "Ruzi", Imam: "Ananda"},
		Isya:    DutyAssignment{Adzan: "Basit", Imam: "Iskandar"},
	},
	time.Tuesday: {
		DayName: "Selasa",
		Subuh:   DutyAssignment{Adzan: "Basit", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Adzan: "Arjuna", Imam: "Ruzi"},
		Ashar:   DutyAssignment{Adzan: "Makhasin", Imam: "Fajar"},
		Maghrib: DutyAssignment{Adzan: "Imam", Imam: "T"},
		Isya:    DutyAssignment{Adzan: "Makhasin", Imam: "Imam"},
	},
	time.Wednesday: {
		DayName: "Rabu",
		Subuh:   DutyAssignment{Adzan: "Basit", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Adzan: "T", Imam: "Ruzi"},
		Ashar:   DutyAssignment{Adzan: "Ruzi", Imam: "T"},
		Maghrib: DutyAssignment{Adzan: "Iskandar", Imam: "Arjuna"},
		Isya:    DutyAssignment{Adzan: "Ruzi", Imam: "Imam"},
	},
	time.Thursday: {
		DayName: "Kamis",
		Subuh:   DutyAssignment{Adzan: "Ananda", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Adzan: "T", Imam: "Ruzi"},
		Ashar:   DutyAssignment{Adzan: "Fajar", Imam: "Arjuna"},
		Maghrib: DutyAssignment{Adzan: "Basit", Imam: "Imam"},
		Isya:    DutyAssignment{Adzan: "Fajar", Imam: "Makhasin"},
	},
	time.Friday: {
		DayName: "Jumat",
		Subuh:   DutyAssignment{Adzan: "Makhasin", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Skipped: true}, // Zhuhur is skipped on Friday (replaced by Sholat Jumat)
		Ashar:   DutyAssignment{Adzan: "Ananda", Imam: "Basit"},
		Maghrib: DutyAssignment{Adzan: "Fajar", Imam: "Arjuna"},
		Isya:    DutyAssignment{Adzan: "Imam", Imam: "Makhasin"},
	},
	time.Saturday: {
		DayName: "Sabtu",
		Subuh:   DutyAssignment{Adzan: "Basit", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Adzan: "Fajar", Imam: "Ananda"},
		Ashar:   DutyAssignment{Adzan: "Iskandar", Imam: "Makhasin"},
		Maghrib: DutyAssignment{Adzan: "Iskandar", Imam: "Ananda"},
		Isya:    DutyAssignment{Adzan: "Imam", Imam: "Arjuna"},
	},
	time.Sunday: {
		DayName: "Minggu",
		Subuh:   DutyAssignment{Adzan: "Arjuna", Imam: "Haris"},
		Zhuhur:  DutyAssignment{Adzan: "Iskandar", Imam: "Fajar"},
		Ashar:   DutyAssignment{Adzan: "Makhasin", Imam: "Ananda"},
		Maghrib: DutyAssignment{Adzan: "Iskandar", Imam: "Fajar"},
		Isya:    DutyAssignment{Adzan: "Fajar", Imam: "Iskandar"},
	},
}

// GetDaySchedule returns the schedule for the given weekday.
func GetDaySchedule(weekday time.Weekday) DaySchedule {
	return WeeklyMatrix[weekday]
}

// KultumQueue contains the ordered 10-member round-robin rotation for Subuh Kultum.
// 1. Iskandar -> 2. Haris -> 3. Thoriq -> 4. Ruzi -> 5. Fajar ->
// 6. Ananda -> 7. Makhasin -> 8. Arjuna -> 9. Imam -> 10. Basit -> repeat to 1.
var KultumQueue = []string{
	"Iskandar",
	"Haris",
	"Thoriq",
	"Ruzi",
	"Fajar",
	"Ananda",
	"Makhasin",
	"Arjuna",
	"Imam",
	"Basit",
}

// GetKultumSpeaker returns the speaker name for the given index (0-indexed).
func GetKultumSpeaker(index int) string {
	if len(KultumQueue) == 0 {
		return ""
	}
	modIdx := ((index % len(KultumQueue)) + len(KultumQueue)) % len(KultumQueue)
	return KultumQueue[modIdx]
}

// NextKultumIndex returns the next round-robin index.
func NextKultumIndex(currentIndex int) int {
	if len(KultumQueue) == 0 {
		return 0
	}
	return (currentIndex + 1) % len(KultumQueue)
}

// FormatIndonesianDate returns formatted Indonesian date string e.g. "Senin, 31 Agustus 2026".
func FormatIndonesianDate(t time.Time) string {
	dayNames := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	monthNames := []string{
		"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}

	dayName := dayNames[t.Weekday()]
	monthName := monthNames[t.Month()]
	return fmt.Sprintf("%s, %02d %s %04d", dayName, t.Day(), monthName, t.Year())
}

// IndonesianDayName returns the Indonesian name of a weekday.
func IndonesianDayName(weekday time.Weekday) string {
	dayNames := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	return dayNames[weekday]
}

// NormalizePrayerName turns various spellings (Dzuhur, Dhuhur, Zhuhur) into canonical PrayerName.
func NormalizePrayerName(s string) PrayerName {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch lower {
	case "subuh", "fajr":
		return PrayerSubuh
	case "zhuhur", "dzuhur", "dhuhur", "dhuhr", "zuhur":
		return PrayerZhuhur
	case "ashar", "asr", "asar":
		return PrayerAshar
	case "maghrib", "magrib":
		return PrayerMaghrib
	case "isya", "isha", "isya'":
		return PrayerIsya
	default:
		return PrayerName(s)
	}
}
