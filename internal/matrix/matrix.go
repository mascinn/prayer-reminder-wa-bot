package matrix

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
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
	Adzan   string `json:"adzan,omitempty"`
	Imam    string `json:"imam,omitempty"`
	Skipped bool   `json:"skipped,omitempty"` // True for Friday Zhuhur
}

// DaySchedule holds duty assignments for all five daily prayers on a specific day.
type DaySchedule struct {
	DayName string         `json:"day_name"`
	Subuh   DutyAssignment `json:"subuh"`
	Zhuhur  DutyAssignment `json:"zhuhur"`
	Ashar   DutyAssignment `json:"ashar"`
	Maghrib DutyAssignment `json:"maghrib"`
	Isya    DutyAssignment `json:"isya"`
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

// WeeklyMatrixRaw represents the raw day-by-day JSON structure.
type WeeklyMatrixRaw struct {
	Monday    DaySchedule `json:"monday"`
	Tuesday   DaySchedule `json:"tuesday"`
	Wednesday DaySchedule `json:"wednesday"`
	Thursday  DaySchedule `json:"thursday"`
	Friday    DaySchedule `json:"friday"`
	Saturday  DaySchedule `json:"saturday"`
	Sunday    DaySchedule `json:"sunday"`
}

// ScheduleConfig holds the dynamic matrix and kultum queue loaded from JSON/environment.
type ScheduleConfig struct {
	KultumQueue  []string        `json:"kultum_queue"`
	WeeklyMatrix WeeklyMatrixRaw `json:"weekly_matrix"`

	matrixMap map[time.Weekday]DaySchedule
	mu        sync.RWMutex
}

// DefaultScheduleConfig provides the fallback baseline.
// DefaultScheduleConfig provides the fallback baseline with generic example names.
var DefaultScheduleConfig = &ScheduleConfig{
	KultumQueue: []string{
		"Ahmad",
		"Zaid",
		"Bilal",
		"Umar",
		"Ali",
		"Usman",
		"Hamzah",
		"Hasan",
		"Husain",
		"Salman",
	},
	WeeklyMatrix: WeeklyMatrixRaw{
		Monday: DaySchedule{
			DayName: "Senin",
			Subuh:   DutyAssignment{Adzan: "Ahmad", Imam: "Zaid"},
			Zhuhur:  DutyAssignment{Adzan: "Bilal", Imam: "Umar"},
			Ashar:   DutyAssignment{Adzan: "Ali", Imam: "Usman"},
			Maghrib: DutyAssignment{Adzan: "Hamzah", Imam: "Hasan"},
			Isya:    DutyAssignment{Adzan: "Husain", Imam: "Salman"},
		},
		Tuesday: DaySchedule{
			DayName: "Selasa",
			Subuh:   DutyAssignment{Adzan: "Zaid", Imam: "Ahmad"},
			Zhuhur:  DutyAssignment{Adzan: "Ali", Imam: "Bilal"},
			Ashar:   DutyAssignment{Adzan: "Usman", Imam: "Hamzah"},
			Maghrib: DutyAssignment{Adzan: "Hasan", Imam: "Husain"},
			Isya:    DutyAssignment{Adzan: "Salman", Imam: "Ahmad"},
		},
		Wednesday: DaySchedule{
			DayName: "Rabu",
			Subuh:   DutyAssignment{Adzan: "Bilal", Imam: "Ahmad"},
			Zhuhur:  DutyAssignment{Adzan: "Umar", Imam: "Zaid"},
			Ashar:   DutyAssignment{Adzan: "Ali", Imam: "Hamzah"},
			Maghrib: DutyAssignment{Adzan: "Usman", Imam: "Hasan"},
			Isya:    DutyAssignment{Adzan: "Husain", Imam: "Salman"},
		},
		Thursday: DaySchedule{
			DayName: "Kamis",
			Subuh:   DutyAssignment{Adzan: "Usman", Imam: "Zaid"},
			Zhuhur:  DutyAssignment{Adzan: "Umar", Imam: "Bilal"},
			Ashar:   DutyAssignment{Adzan: "Hamzah", Imam: "Ali"},
			Maghrib: DutyAssignment{Adzan: "Hasan", Imam: "Husain"},
			Isya:    DutyAssignment{Adzan: "Salman", Imam: "Ahmad"},
		},
		Friday: DaySchedule{
			DayName: "Jumat",
			Subuh:   DutyAssignment{Adzan: "Hamzah", Imam: "Ahmad"},
			Zhuhur:  DutyAssignment{Skipped: true},
			Ashar:   DutyAssignment{Adzan: "Bilal", Imam: "Ali"},
			Maghrib: DutyAssignment{Adzan: "Usman", Imam: "Hasan"},
			Isya:    DutyAssignment{Adzan: "Husain", Imam: "Salman"},
		},
		Saturday: DaySchedule{
			DayName: "Sabtu",
			Subuh:   DutyAssignment{Adzan: "Hasan", Imam: "Zaid"},
			Zhuhur:  DutyAssignment{Adzan: "Ahmad", Imam: "Umar"},
			Ashar:   DutyAssignment{Adzan: "Ali", Imam: "Hamzah"},
			Maghrib: DutyAssignment{Adzan: "Bilal", Imam: "Usman"},
			Isya:    DutyAssignment{Adzan: "Salman", Imam: "Ahmad"},
		},
		Sunday: DaySchedule{
			DayName: "Minggu",
			Subuh:   DutyAssignment{Adzan: "Husain", Imam: "Ahmad"},
			Zhuhur:  DutyAssignment{Adzan: "Zaid", Imam: "Bilal"},
			Ashar:   DutyAssignment{Adzan: "Umar", Imam: "Ali"},
			Maghrib: DutyAssignment{Adzan: "Hamzah", Imam: "Hasan"},
			Isya:    DutyAssignment{Adzan: "Usman", Imam: "Salman"},
		},
	},
}

// activeSchedule holds the currently active global schedule instance.
var (
	activeSchedule   *ScheduleConfig
	activeScheduleMu sync.RWMutex
)

func init() {
	DefaultScheduleConfig.initMap()
	activeSchedule = DefaultScheduleConfig
}

func (sc *ScheduleConfig) initMap() {
	sc.matrixMap = map[time.Weekday]DaySchedule{
		time.Monday:    sc.WeeklyMatrix.Monday,
		time.Tuesday:   sc.WeeklyMatrix.Tuesday,
		time.Wednesday: sc.WeeklyMatrix.Wednesday,
		time.Thursday:  sc.WeeklyMatrix.Thursday,
		time.Friday:    sc.WeeklyMatrix.Friday,
		time.Saturday:  sc.WeeklyMatrix.Saturday,
		time.Sunday:    sc.WeeklyMatrix.Sunday,
	}
}

// LoadSchedule loads the duty matrix and kultum queue from environment JSON or file.
func LoadSchedule(filePath string, jsonStr string) *ScheduleConfig {
	// 1. From JSON string (e.g. SCHEDULE_JSON in Fly secrets)
	if strings.TrimSpace(jsonStr) != "" {
		var cfg ScheduleConfig
		if err := json.Unmarshal([]byte(jsonStr), &cfg); err == nil && len(cfg.KultumQueue) > 0 {
			cfg.initMap()
			SetActiveSchedule(&cfg)
			log.Printf("[Matrix] Loaded schedule config with %d kultum speakers from SCHEDULE_JSON.", len(cfg.KultumQueue))
			return &cfg
		}
		log.Println("[Matrix] Warning: Failed to parse SCHEDULE_JSON, falling back to file.")
	}

	// 2. Candidate files in order of preference
	candidates := []string{
		filePath,
		"./data/schedule.json",
		"../data/schedule.json",
		"../../data/schedule.json",
		"/data/schedule.json",
		"schedule.json",
		"../schedule.json",
		"../../schedule.json",
		"schedule.example.json",
		"../schedule.example.json",
		"../../schedule.example.json",
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if data, err := os.ReadFile(path); err == nil {
			var cfg ScheduleConfig
			if err := json.Unmarshal(data, &cfg); err == nil && len(cfg.KultumQueue) > 0 {
				cfg.initMap()
				SetActiveSchedule(&cfg)
				log.Printf("[Matrix] Successfully loaded schedule from %s.", path)
				return &cfg
			}
		}
	}

	log.Println("[Matrix] Notice: Using default built-in schedule matrix.")
	SetActiveSchedule(DefaultScheduleConfig)
	return DefaultScheduleConfig
}

// SetActiveSchedule updates the active global schedule instance.
func SetActiveSchedule(sc *ScheduleConfig) {
	activeScheduleMu.Lock()
	defer activeScheduleMu.Unlock()
	sc.initMap()
	activeSchedule = sc
}

// GetActiveSchedule returns the current active ScheduleConfig.
func GetActiveSchedule() *ScheduleConfig {
	activeScheduleMu.RLock()
	defer activeScheduleMu.RUnlock()
	return activeSchedule
}

// GetDaySchedule returns the schedule for the given weekday from active configuration.
func GetDaySchedule(weekday time.Weekday) DaySchedule {
	return GetActiveSchedule().GetDaySchedule(weekday)
}

// GetKultumSpeaker returns the speaker name for the given index (0-indexed).
func GetKultumSpeaker(index int) string {
	return GetActiveSchedule().GetKultumSpeaker(index)
}

// GetKultumSpeakerForDay returns the speaker name assigned for a specific calendar day (1-31).
// Day 1 maps to index 0, rolling over the queue and resetting every 1st of the month.
func GetKultumSpeakerForDay(day int) string {
	return GetActiveSchedule().GetKultumSpeakerForDay(day)
}

// NextKultumIndex returns the next round-robin index.
func NextKultumIndex(currentIndex int) int {
	return GetActiveSchedule().NextKultumIndex(currentIndex)
}

// KultumQueue returns the active slice of kultum speaker names.
func KultumQueue() []string {
	return GetActiveSchedule().GetKultumQueue()
}

// KultumQueueLen returns the length of the active kultum queue.
func KultumQueueLen() int {
	return GetActiveSchedule().KultumQueueLen()
}

// (ScheduleConfig methods)
func (sc *ScheduleConfig) GetDaySchedule(weekday time.Weekday) DaySchedule {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.matrixMap[weekday]
}

func (sc *ScheduleConfig) GetKultumSpeaker(index int) string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if len(sc.KultumQueue) == 0 {
		return ""
	}
	modIdx := ((index % len(sc.KultumQueue)) + len(sc.KultumQueue)) % len(sc.KultumQueue)
	return sc.KultumQueue[modIdx]
}

func (sc *ScheduleConfig) GetKultumSpeakerForDay(day int) string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if len(sc.KultumQueue) == 0 || day < 1 {
		return ""
	}
	modIdx := (day - 1) % len(sc.KultumQueue)
	return sc.KultumQueue[modIdx]
}

func (sc *ScheduleConfig) NextKultumIndex(currentIndex int) int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if len(sc.KultumQueue) == 0 {
		return 0
	}
	return (currentIndex + 1) % len(sc.KultumQueue)
}

func (sc *ScheduleConfig) KultumQueueLen() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.KultumQueue)
}

func (sc *ScheduleConfig) GetKultumQueue() []string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	res := make([]string, len(sc.KultumQueue))
	copy(res, sc.KultumQueue)
	return res
}

// DaysInMonth returns the total number of days in the month of the given time.
func DaysInMonth(t time.Time) int {
	year, month, _ := t.Date()
	return time.Date(year, month+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// FormatIndonesianMonthYear returns formatted month and year e.g. "September 2026".
func FormatIndonesianMonthYear(t time.Time) string {
	monthNames := []string{
		"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	return fmt.Sprintf("%s %04d", monthNames[t.Month()], t.Year())
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

// OfficerShift describes a single assigned duty shift in the weekly matrix.
type OfficerShift struct {
	Weekday time.Weekday
	DayName string
	Prayer  PrayerName
	Role    string // "Adzan" or "Imam"
}

// ParseIndonesianWeekday parses an Indonesian weekday name (e.g. "senin", "selasa", "rabu", "kamis", "jumat", "jum'at", "sabtu", "minggu").
func ParseIndonesianWeekday(s string) (time.Weekday, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))
	lower = strings.ReplaceAll(lower, "'", "")
	switch lower {
	case "senin", "mon", "monday":
		return time.Monday, true
	case "selasa", "tue", "tuesday":
		return time.Tuesday, true
	case "rabu", "wed", "wednesday":
		return time.Wednesday, true
	case "kamis", "thu", "thursday":
		return time.Thursday, true
	case "jumat", "fri", "friday":
		return time.Friday, true
	case "sabtu", "sat", "saturday":
		return time.Saturday, true
	case "minggu", "ahad", "sun", "sunday":
		return time.Sunday, true
	default:
		return time.Sunday, false
	}
}

// GetFullWeeklyMatrix returns the full day-by-day schedule for Monday through Sunday.
func GetFullWeeklyMatrix() map[time.Weekday]DaySchedule {
	activeScheduleMu.RLock()
	defer activeScheduleMu.RUnlock()
	sc := activeSchedule
	if sc == nil {
		sc = DefaultScheduleConfig
	}
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make(map[time.Weekday]DaySchedule)
	for k, v := range sc.matrixMap {
		result[k] = v
	}
	return result
}

// GetOfficerWeeklyDuties searches the weekly matrix for all duties assigned to the given officer name.
func GetOfficerWeeklyDuties(officerName string) []OfficerShift {
	matrixMap := GetFullWeeklyMatrix()
	target := strings.TrimSpace(strings.ToLower(officerName))
	if target == "" {
		return nil
	}

	days := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
		time.Friday, time.Saturday, time.Sunday,
	}
	prayers := []PrayerName{
		PrayerSubuh, PrayerZhuhur, PrayerAshar, PrayerMaghrib, PrayerIsya,
	}

	var shifts []OfficerShift
	for _, wd := range days {
		sched, ok := matrixMap[wd]
		if !ok {
			continue
		}
		for _, p := range prayers {
			duty := sched.GetDuty(p)
			if duty.Skipped {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(duty.Adzan), target) {
				shifts = append(shifts, OfficerShift{
					Weekday: wd,
					DayName: sched.DayName,
					Prayer:  p,
					Role:    "Adzan",
				})
			}
			if strings.EqualFold(strings.TrimSpace(duty.Imam), target) {
				shifts = append(shifts, OfficerShift{
					Weekday: wd,
					DayName: sched.DayName,
					Prayer:  p,
					Role:    "Imam",
				})
			}
		}
	}
	return shifts
}

// GetOfficerKultumDaysInMonth returns the calendar days of the specified month/year where the officer gives Kultum.
func GetOfficerKultumDaysInMonth(officerName string, month int, year int) []int {
	target := strings.TrimSpace(strings.ToLower(officerName))
	if target == "" {
		return nil
	}

	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	totalDays := DaysInMonth(t)

	var days []int
	for d := 1; d <= totalDays; d++ {
		sp := GetKultumSpeakerForDay(d)
		if strings.EqualFold(strings.TrimSpace(sp), target) {
			days = append(days, d)
		}
	}
	return days
}

