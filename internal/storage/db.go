package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

// ReminderLog represents a recorded reminder event sent to WhatsApp.
type ReminderLog struct {
	ID             int64     `json:"id"`
	ReminderType   string    `json:"reminder_type"`
	PrayerTime     string    `json:"prayer_time"`
	AdzanOfficer   string    `json:"adzan_officer"`
	ImamOfficer    string    `json:"imam_officer"`
	KultumOfficer  string    `json:"kultum_officer"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// Storage manages persistent state (Turso libSQL Cloud or local SQLite).
type Storage struct {
	db       *sql.DB
	isTurso  bool
	mu       sync.RWMutex
}

// NewStorage initializes database connection (Turso Cloud if provided, else local SQLite).
func NewStorage(dbPath, tursoURL, tursoToken string) (*Storage, error) {
	var db *sql.DB
	var err error
	isTurso := false

	if strings.TrimSpace(tursoURL) != "" && strings.TrimSpace(tursoToken) != "" {
		dsn := fmt.Sprintf("%s?authToken=%s", tursoURL, tursoToken)
		db, err = sql.Open("libsql", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open Turso libSQL connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("failed to connect to Turso cloud database: %w", err)
		}
		isTurso = true
		log.Println("[Storage] Successfully connected to Turso SQLite Cloud database! ☁️")
	} else {
		dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
		db, err = sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
		}
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
		}
		log.Printf("[Storage] Using local SQLite database at: %s", dbPath)
	}

	s := &Storage{db: db, isTurso: isTurso}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

func (s *Storage) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS bot_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			display_name TEXT NOT NULL UNIQUE,
			phones_json TEXT NOT NULL,
			aliases_json TEXT NOT NULL,
			is_active INTEGER DEFAULT 1,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS duty_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			weekday INTEGER NOT NULL,
			day_name TEXT NOT NULL,
			prayer_name TEXT NOT NULL,
			adzan_member TEXT NOT NULL,
			imam_member TEXT NOT NULL,
			is_skipped INTEGER DEFAULT 0,
			UNIQUE(weekday, prayer_name)
		);`,
		`CREATE TABLE IF NOT EXISTS kultum_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			queue_order INTEGER NOT NULL UNIQUE,
			member_name TEXT NOT NULL,
			is_active INTEGER DEFAULT 1
		);`,
		`CREATE TABLE IF NOT EXISTS prayer_cache (
			date TEXT PRIMARY KEY,
			city_id TEXT NOT NULL,
			schedule_json TEXT NOT NULL,
			fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS reminder_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			reminder_type TEXT NOT NULL,
			prayer_time TEXT,
			adzan_officer TEXT,
			imam_officer TEXT,
			kultum_officer TEXT,
			status TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("executing migration query failed: %w", err)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}

// IsTurso returns true if storage is connected to Turso Cloud.
func (s *Storage) IsTurso() bool {
	return s.isTurso
}

// DB returns the underlying *sql.DB instance.
func (s *Storage) DB() *sql.DB {
	return s.db
}

// --- Bot State Operations ---

// GetState retrieves a key value from bot_state.
func (s *Storage) GetState(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var val string
	err := s.db.QueryRow("SELECT value FROM bot_state WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetState upserts a key-value pair in bot_state.
func (s *Storage) SetState(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO bot_state (key, value, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.Exec(query, key, value, time.Now().UTC())
	return err
}

// GetKultumIndex returns the current Kultum speaker index (0-indexed). Defaults to 0.
func (s *Storage) GetKultumIndex() (int, error) {
	val, err := s.GetState("kultum_index")
	if err != nil || val == "" {
		return 0, err
	}
	idx, err := strconv.Atoi(val)
	if err != nil {
		return 0, nil
	}
	return idx, nil
}

// SetKultumIndex updates the current Kultum speaker index.
func (s *Storage) SetKultumIndex(index int) error {
	return s.SetState("kultum_index", strconv.Itoa(index))
}

// AdvanceKultumIndex atomically retrieves current index and advances it to nextIndex.
func (s *Storage) AdvanceKultumIndex(queueLength int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queueLength <= 0 {
		queueLength = 1
	}

	var currentStr string
	err := s.db.QueryRow("SELECT value FROM bot_state WHERE key = 'kultum_index'").Scan(&currentStr)
	currentIdx := 0
	if err == nil && currentStr != "" {
		if idx, parseErr := strconv.Atoi(currentStr); parseErr == nil {
			currentIdx = idx
		}
	}

	nextIdx := (currentIdx + 1) % queueLength
	query := `
	INSERT INTO bot_state (key, value, updated_at)
	VALUES ('kultum_index', ?, ?)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = excluded.updated_at;
	`
	if _, err := s.db.Exec(query, strconv.Itoa(nextIdx), time.Now().UTC()); err != nil {
		return currentIdx, err
	}

	return currentIdx, nil
}

// --- Members Storage Operations ---

// LoadMembers reads all active members from database.
func (s *Storage) LoadMembers() ([]phonebook.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT display_name, phones_json, aliases_json FROM members WHERE is_active = 1 ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []phonebook.Member
	for rows.Next() {
		var name, phonesRaw, aliasesRaw string
		if err := rows.Scan(&name, &phonesRaw, &aliasesRaw); err != nil {
			continue
		}

		var phones, aliases []string
		_ = json.Unmarshal([]byte(phonesRaw), &phones)
		_ = json.Unmarshal([]byte(aliasesRaw), &aliases)

		m := phonebook.Member{
			DisplayName: name,
			Phones:      phones,
			Aliases:     aliases,
		}
		if len(phones) > 0 {
			m.Phone = phones[0]
		}
		members = append(members, m)
	}

	return members, nil
}

// SaveMembers saves or updates a slice of members into the database.
func (s *Storage) SaveMembers(members []phonebook.Member) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range members {
		phonesBytes, _ := json.Marshal(m.AllPhones())
		aliasesBytes, _ := json.Marshal(m.Aliases)

		query := `
		INSERT INTO members (display_name, phones_json, aliases_json, is_active, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(display_name) DO UPDATE SET
			phones_json = excluded.phones_json,
			aliases_json = excluded.aliases_json,
			is_active = 1,
			updated_at = excluded.updated_at;
		`
		if _, err := s.db.Exec(query, m.DisplayName, string(phonesBytes), string(aliasesBytes), time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

// --- Duty Schedule & Kultum Queue Operations ---

// LoadDutyMatrix reads the full weekly duty schedule from database.
func (s *Storage) LoadDutyMatrix() (map[time.Weekday]matrix.DaySchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT weekday, day_name, prayer_name, adzan_member, imam_member, is_skipped FROM duty_schedules ORDER BY weekday ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[time.Weekday]matrix.DaySchedule)
	for rows.Next() {
		var weekdayInt, isSkippedInt int
		var dayName, prayerNameStr, adzan, imam string
		if err := rows.Scan(&weekdayInt, &dayName, &prayerNameStr, &adzan, &imam, &isSkippedInt); err != nil {
			continue
		}

		w := time.Weekday(weekdayInt)
		sched := res[w]
		sched.DayName = dayName

		duty := matrix.DutyAssignment{
			Adzan:   adzan,
			Imam:    imam,
			Skipped: isSkippedInt == 1,
		}

		prayerName := matrix.NormalizePrayerName(prayerNameStr)
		switch prayerName {
		case matrix.PrayerSubuh:
			sched.Subuh = duty
		case matrix.PrayerZhuhur:
			sched.Zhuhur = duty
		case matrix.PrayerAshar:
			sched.Ashar = duty
		case matrix.PrayerMaghrib:
			sched.Maghrib = duty
		case matrix.PrayerIsya:
			sched.Isya = duty
		}
		res[w] = sched
	}

	return res, nil
}

// SaveDutySchedule saves weekly schedule into database.
func (s *Storage) SaveDutySchedule(matrixMap map[time.Weekday]matrix.DaySchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for weekday, sched := range matrixMap {
		prayers := []struct {
			name string
			duty matrix.DutyAssignment
		}{
			{"subuh", sched.Subuh},
			{"zhuhur", sched.Zhuhur},
			{"ashar", sched.Ashar},
			{"maghrib", sched.Maghrib},
			{"isya", sched.Isya},
		}

		for _, p := range prayers {
			skipInt := 0
			if p.duty.Skipped {
				skipInt = 1
			}
			query := `
			INSERT INTO duty_schedules (weekday, day_name, prayer_name, adzan_member, imam_member, is_skipped)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(weekday, prayer_name) DO UPDATE SET
				day_name = excluded.day_name,
				adzan_member = excluded.adzan_member,
				imam_member = excluded.imam_member,
				is_skipped = excluded.is_skipped;
			`
			if _, err := s.db.Exec(query, int(weekday), sched.DayName, p.name, p.duty.Adzan, p.duty.Imam, skipInt); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadKultumQueue reads the ordered kultum speaker rotation list.
func (s *Storage) LoadKultumQueue() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT member_name FROM kultum_queue WHERE is_active = 1 ORDER BY queue_order ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queue []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil && name != "" {
			queue = append(queue, name)
		}
	}
	return queue, nil
}

// SaveKultumQueue updates the full ordered kultum queue in database.
func (s *Storage) SaveKultumQueue(queue []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.db.Exec("DELETE FROM kultum_queue")
	for i, name := range queue {
		query := `INSERT INTO kultum_queue (queue_order, member_name, is_active) VALUES (?, ?, 1)`
		if _, err := s.db.Exec(query, i+1, name); err != nil {
			return err
		}
	}
	return nil
}

// --- Auto-Seed Initial Community Data ---

// AutoSeedInitialData inserts initial community data into database if empty.
func (s *Storage) AutoSeedInitialData(members []phonebook.Member, schedule *matrix.ScheduleConfig) {
	existingMembers, err := s.LoadMembers()
	if err == nil && len(existingMembers) == 0 && len(members) > 0 {
		if err := s.SaveMembers(members); err == nil {
			log.Printf("[Storage] Auto-seeded %d community members into database.", len(members))
		}
	}

	existingQueue, err := s.LoadKultumQueue()
	if err == nil && len(existingQueue) == 0 && schedule != nil && schedule.KultumQueueLen() > 0 {
		if err := s.SaveKultumQueue(schedule.GetKultumQueue()); err == nil {
			log.Printf("[Storage] Auto-seeded %d kultum rotation speakers into database.", schedule.KultumQueueLen())
		}
	}

	existingMatrix, err := s.LoadDutyMatrix()
	if err == nil && len(existingMatrix) == 0 && schedule != nil {
		matrixMap := map[time.Weekday]matrix.DaySchedule{
			time.Monday:    schedule.GetDaySchedule(time.Monday),
			time.Tuesday:   schedule.GetDaySchedule(time.Tuesday),
			time.Wednesday: schedule.GetDaySchedule(time.Wednesday),
			time.Thursday:  schedule.GetDaySchedule(time.Thursday),
			time.Friday:    schedule.GetDaySchedule(time.Friday),
			time.Saturday:  schedule.GetDaySchedule(time.Saturday),
			time.Sunday:    schedule.GetDaySchedule(time.Sunday),
		}
		if err := s.SaveDutySchedule(matrixMap); err == nil {
			log.Println("[Storage] Auto-seeded weekly duty matrix into database.")
		}
	}
}

// --- Reminder Audit Logs & Prayer Cache ---

// LogReminder records an automated dispatch event for audit logs.
func (s *Storage) LogReminder(reminderType, prayerTime, adzan, imam, kultum, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO reminder_logs (reminder_type, prayer_time, adzan_officer, imam_officer, kultum_officer, status, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := s.db.Exec(query, reminderType, prayerTime, adzan, imam, kultum, status, time.Now().UTC())
	return err
}

// CacheJadwal saves prayer times json for a date string (YYYY-MM-DD).
func (s *Storage) CacheJadwal(dateKey string, jsonContent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO prayer_cache (date, city_id, schedule_json, fetched_at)
	VALUES (?, '1014', ?, ?)
	ON CONFLICT(date) DO UPDATE SET
		schedule_json = excluded.schedule_json,
		fetched_at = excluded.fetched_at;
	`
	_, err := s.db.Exec(query, dateKey, jsonContent, time.Now().UTC())
	return err
}

// GetCachedJadwal retrieves cached prayer times json for a date string (YYYY-MM-DD).
func (s *Storage) GetCachedJadwal(dateKey string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jsonContent string
	err := s.db.QueryRow("SELECT schedule_json FROM prayer_cache WHERE date = ?", dateKey).Scan(&jsonContent)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return jsonContent, err
}
