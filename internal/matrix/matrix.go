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
var DefaultScheduleConfig = &ScheduleConfig{
	KultumQueue: []string{
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
	},
	WeeklyMatrix: WeeklyMatrixRaw{
		Monday: DaySchedule{
			DayName: "Senin",
			Subuh:   DutyAssignment{Adzan: "Imam", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Adzan: "Ruzi", Imam: "Basit"},
			Ashar:   DutyAssignment{Adzan: "Arjuna", Imam: "Ananda"},
			Maghrib: DutyAssignment{Adzan: "Ruzi", Imam: "Ananda"},
			Isya:    DutyAssignment{Adzan: "Basit", Imam: "Iskandar"},
		},
		Tuesday: DaySchedule{
			DayName: "Selasa",
			Subuh:   DutyAssignment{Adzan: "Basit", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Adzan: "Arjuna", Imam: "Ruzi"},
			Ashar:   DutyAssignment{Adzan: "Makhasin", Imam: "Fajar"},
			Maghrib: DutyAssignment{Adzan: "Imam", Imam: "T"},
			Isya:    DutyAssignment{Adzan: "Makhasin", Imam: "Imam"},
		},
		Wednesday: DaySchedule{
			DayName: "Rabu",
			Subuh:   DutyAssignment{Adzan: "Basit", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Adzan: "T", Imam: "Ruzi"},
			Ashar:   DutyAssignment{Adzan: "Ruzi", Imam: "T"},
			Maghrib: DutyAssignment{Adzan: "Iskandar", Imam: "Arjuna"},
			Isya:    DutyAssignment{Adzan: "Ruzi", Imam: "Imam"},
		},
		Thursday: DaySchedule{
			DayName: "Kamis",
			Subuh:   DutyAssignment{Adzan: "Ananda", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Adzan: "T", Imam: "Ruzi"},
			Ashar:   DutyAssignment{Adzan: "Fajar", Imam: "Arjuna"},
			Maghrib: DutyAssignment{Adzan: "Basit", Imam: "Imam"},
			Isya:    DutyAssignment{Adzan: "Fajar", Imam: "Makhasin"},
		},
		Friday: DaySchedule{
			DayName: "Jumat",
			Subuh:   DutyAssignment{Adzan: "Makhasin", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Skipped: true},
			Ashar:   DutyAssignment{Adzan: "Ananda", Imam: "Basit"},
			Maghrib: DutyAssignment{Adzan: "Fajar", Imam: "Arjuna"},
			Isya:    DutyAssignment{Adzan: "Imam", Imam: "Makhasin"},
		},
		Saturday: DaySchedule{
			DayName: "Sabtu",
			Subuh:   DutyAssignment{Adzan: "Basit", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Adzan: "Fajar", Imam: "Ananda"},
			Ashar:   DutyAssignment{Adzan: "Iskandar", Imam: "Makhasin"},
			Maghrib: DutyAssignment{Adzan: "Iskandar", Imam: "Ananda"},
			Isya:    DutyAssignment{Adzan: "Imam", Imam: "Arjuna"},
		},
		Sunday: DaySchedule{
			DayName: "Minggu",
			Subuh:   DutyAssignment{Adzan: "Arjuna", Imam: "Haris"},
			Zhuhur:  DutyAssignment{Adzan: "Iskandar", Imam: "Fajar"},
			Ashar:   DutyAssignment{Adzan: "Makhasin", Imam: "Ananda"},
			Maghrib: DutyAssignment{Adzan: "Iskandar", Imam: "Fajar"},
			Isya:    DutyAssignment{Adzan: "Fajar", Imam: "Iskandar"},
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
