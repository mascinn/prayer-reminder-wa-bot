package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
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
		`CREATE TABLE IF NOT EXISTS canteen_schedules (
			weekday INTEGER PRIMARY KEY,
			day_name TEXT NOT NULL,
			officers_json TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS duty_attendance_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id TEXT NOT NULL UNIQUE,
			prayer_date TEXT NOT NULL,
			prayer_name TEXT NOT NULL,
			adzan_officer TEXT NOT NULL,
			imam_officer TEXT NOT NULL,
			adzan_executed INTEGER DEFAULT 1,
			imam_executed INTEGER DEFAULT 1,
			last_reaction TEXT DEFAULT '',
			reporter_jid TEXT DEFAULT '',
			cutoff_time TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_duty_logs_msg_id ON duty_attendance_logs(message_id);`,
		`CREATE INDEX IF NOT EXISTS idx_duty_logs_prayer_date ON duty_attendance_logs(prayer_date);`,
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

	existingCanteen, err := s.LoadCanteenSchedule()
	if err == nil && len(existingCanteen) == 0 {
		defaultCanteen := map[time.Weekday][]string{
			time.Monday:    {"Ruzi", "Arjuna"},
			time.Tuesday:   {"Fajar", "Imam"},
			time.Wednesday: {"Torik", "Basit"},
			time.Thursday:  {"Makhasin", "Ananda"},
			time.Friday:    {"Iskandar", "Haris", "Arif"},
		}
		if err := s.SaveCanteenSchedule(defaultCanteen); err == nil {
			log.Println("[Storage] Auto-seeded default canteen duty schedule into database.")
		}
	}
}

// --- Canteen Duty Schedule Operations ---

// SaveCanteenSchedule persists the weekly canteen collection roster.
func (s *Storage) SaveCanteenSchedule(schedule map[time.Weekday][]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dayNames := map[time.Weekday]string{
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
	}

	for weekday, officers := range schedule {
		officersJSON, err := json.Marshal(officers)
		if err != nil {
			continue
		}
		dayName := dayNames[weekday]
		if dayName == "" {
			dayName = weekday.String()
		}

		query := `
		INSERT INTO canteen_schedules (weekday, day_name, officers_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(weekday) DO UPDATE SET
			day_name = excluded.day_name,
			officers_json = excluded.officers_json,
			updated_at = excluded.updated_at;
		`
		if _, err := s.db.Exec(query, int(weekday), dayName, string(officersJSON), time.Now().UTC()); err != nil {
			return fmt.Errorf("failed to save canteen schedule for %s: %w", dayName, err)
		}
	}
	return nil
}

// LoadCanteenSchedule retrieves the weekly canteen collection roster.
func (s *Storage) LoadCanteenSchedule() (map[time.Weekday][]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT weekday, officers_json FROM canteen_schedules ORDER BY weekday ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[time.Weekday][]string)
	for rows.Next() {
		var weekdayInt int
		var officersJSON string
		if err := rows.Scan(&weekdayInt, &officersJSON); err != nil {
			continue
		}
		var officers []string
		if err := json.Unmarshal([]byte(officersJSON), &officers); err == nil {
			result[time.Weekday(weekdayInt)] = officers
		}
	}
	return result, nil
}

// GetCanteenOfficers returns the assigned canteen officers for a specific weekday.
func (s *Storage) GetCanteenOfficers(weekday time.Weekday) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var officersJSON string
	err := s.db.QueryRow("SELECT officers_json FROM canteen_schedules WHERE weekday = ?", int(weekday)).Scan(&officersJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var officers []string
	if err := json.Unmarshal([]byte(officersJSON), &officers); err != nil {
		return nil, err
	}
	return officers, nil
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

// --- Duty Attendance Tracking & Reactions ---

// DutyAttendance represents a single prayer reminder duty and its execution status.
type DutyAttendance struct {
	ID            int64     `json:"id"`
	MessageID     string    `json:"message_id"`
	PrayerDate    string    `json:"prayer_date"` // YYYY-MM-DD
	PrayerName    string    `json:"prayer_name"`
	AdzanOfficer  string    `json:"adzan_officer"`
	ImamOfficer   string    `json:"imam_officer"`
	AdzanExecuted bool      `json:"adzan_executed"`
	ImamExecuted  bool      `json:"imam_executed"`
	LastReaction  string    `json:"last_reaction"`
	ReporterJID   string    `json:"reporter_jid"`
	CutoffTime    time.Time `json:"cutoff_time"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// OfficerAttendanceSummary aggregates attendance metrics for one officer in a month.
type OfficerAttendanceSummary struct {
	OfficerName   string  `json:"officer_name"`
	TotalAssigned int     `json:"total_assigned"`
	TotalExecuted int     `json:"total_executed"`
	TotalMissed   int     `json:"total_missed"`
	Percentage    float64 `json:"percentage"`
}

// MonthlyRecapData contains full month overview for !rekap command.
type MonthlyRecapData struct {
	Year         int                        `json:"year"`
	Month        int                        `json:"month"`
	TotalDuties  int                        `json:"total_duties"`
	OverallPct   float64                    `json:"overall_pct"`
	OfficerStats []OfficerAttendanceSummary `json:"officer_stats"`
}

// MissedDutyDetail represents an individual prayer time where the officer did not execute duty.
type MissedDutyDetail struct {
	PrayerDate  string    `json:"prayer_date"`
	PrayerName  string    `json:"prayer_name"`
	Role        string    `json:"role"` // "Adzan" or "Imam"
	ReporterJID string    `json:"reporter_jid"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CleanEmoji strips variation selectors and whitespace for robust comparison.
func CleanEmoji(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\uFE0F", "") // strip variation selector-16
	return s
}

func parseFlexibleTime(val string) (time.Time, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}, nil
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, val); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", val)
}

// SaveDutyRecord saves or updates an active duty reminder linked to a WhatsApp message ID.
func (s *Storage) SaveDutyRecord(messageID, prayerDate, prayerName, adzanOfficer, imamOfficer string, cutoffTime time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO duty_attendance_logs (
		message_id, prayer_date, prayer_name, adzan_officer, imam_officer,
		adzan_executed, imam_executed, last_reaction, reporter_jid, cutoff_time, updated_at
	) VALUES (?, ?, ?, ?, ?, 1, 1, '', '', ?, ?)
	ON CONFLICT(message_id) DO UPDATE SET
		prayer_date = excluded.prayer_date,
		prayer_name = excluded.prayer_name,
		adzan_officer = excluded.adzan_officer,
		imam_officer = excluded.imam_officer,
		cutoff_time = excluded.cutoff_time,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.Exec(query, messageID, prayerDate, prayerName, adzanOfficer, imamOfficer, cutoffTime.UTC(), time.Now().UTC())
	return err
}

// GetDutyRecordByMessageID retrieves a duty record by its WhatsApp message ID.
func (s *Storage) GetDutyRecordByMessageID(messageID string) (*DutyAttendance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, message_id, prayer_date, prayer_name, adzan_officer, imam_officer,
	       adzan_executed, imam_executed, last_reaction, reporter_jid, cutoff_time, created_at, updated_at
	FROM duty_attendance_logs
	WHERE message_id = ?;
	`
	row := s.db.QueryRow(query, messageID)

	var d DutyAttendance
	var adzanExec, imamExec int
	var cutoffStr, createdStr, updatedStr string

	err := row.Scan(
		&d.ID, &d.MessageID, &d.PrayerDate, &d.PrayerName, &d.AdzanOfficer, &d.ImamOfficer,
		&adzanExec, &imamExec, &d.LastReaction, &d.ReporterJID, &cutoffStr, &createdStr, &updatedStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	d.AdzanExecuted = (adzanExec == 1)
	d.ImamExecuted = (imamExec == 1)

	d.CutoffTime, _ = parseFlexibleTime(cutoffStr)
	d.CreatedAt, _ = parseFlexibleTime(createdStr)
	d.UpdatedAt, _ = parseFlexibleTime(updatedStr)

	return &d, nil
}

// UpdateDutyReaction processes an emoji reaction (or un-react) on a duty reminder message.
// Returns:
// - isHandled: true if this message ID was a tracked duty record and within cutoff.
// - shouldBotReact: true if bot should react with ✅, false if bot should remove its reaction ("").
// - err: any error encountered.
func (s *Storage) UpdateDutyReaction(messageID, rawEmoji, reporterJID string, now time.Time) (bool, bool, error) {
	record, err := s.GetDutyRecordByMessageID(messageID)
	if err != nil {
		return false, false, fmt.Errorf("failed to query duty record: %w", err)
	}
	if record == nil {
		// Not a tracked reminder message
		return false, false, nil
	}

	// Check if past cutoff time
	if !record.CutoffTime.IsZero() && now.UTC().After(record.CutoffTime) {
		log.Printf("[Storage] Reaction on message %s ignored: past cutoff time %s", messageID, record.CutoffTime.Format(time.RFC3339))
		return false, false, nil
	}

	clean := CleanEmoji(rawEmoji)

	var adzanExec, imamExec int
	var shouldBotReact bool
	var newReaction string

	if clean == "" {
		// Un-react: reset both to executed (1)
		adzanExec = 1
		imamExec = 1
		newReaction = ""
		shouldBotReact = false
	} else if strings.Contains(clean, "👆") {
		// Adzan tidak menjalankan
		adzanExec = 0
		imamExec = 1
		newReaction = rawEmoji
		shouldBotReact = true
	} else if strings.Contains(clean, "👇") {
		// Imam tidak menjalankan
		adzanExec = 1
		imamExec = 0
		newReaction = rawEmoji
		shouldBotReact = true
	} else if strings.Contains(clean, "✌") || strings.Contains(clean, "2️⃣") || clean == "2" {
		// Keduanya tidak menjalankan
		adzanExec = 0
		imamExec = 0
		newReaction = rawEmoji
		shouldBotReact = true
	} else {
		// Other unrelated emoji (e.g. 👍, ❤️) - ignore
		return false, false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	UPDATE duty_attendance_logs
	SET adzan_executed = ?,
	    imam_executed = ?,
	    last_reaction = ?,
	    reporter_jid = ?,
	    updated_at = ?
	WHERE message_id = ?;
	`
	_, err = s.db.Exec(query, adzanExec, imamExec, newReaction, reporterJID, now.UTC(), messageID)
	if err != nil {
		return false, false, fmt.Errorf("failed to update duty reaction: %w", err)
	}

	return true, shouldBotReact, nil
}

// GetMonthlyRecap calculates summary statistics for a given year and month (1-12).
func (s *Storage) GetMonthlyRecap(year int, month int) (*MonthlyRecapData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	monthPrefix := fmt.Sprintf("%04d-%02d-%%", year, month)

	query := `
	SELECT adzan_officer, imam_officer, adzan_executed, imam_executed
	FROM duty_attendance_logs
	WHERE prayer_date LIKE ?;
	`
	rows, err := s.db.Query(query, monthPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly attendance: %w", err)
	}
	defer rows.Close()

	type officerAccum struct {
		assigned int
		executed int
		missed   int
	}
	statsMap := make(map[string]*officerAccum)

	totalDutiesAssigned := 0
	totalDutiesExecuted := 0

	for rows.Next() {
		var adzan, imam string
		var adzanExec, imamExec int
		if err := rows.Scan(&adzan, &imam, &adzanExec, &imamExec); err != nil {
			return nil, err
		}

		adzan = strings.TrimSpace(adzan)
		if adzan != "" && !strings.EqualFold(adzan, "libur") && !strings.EqualFold(adzan, "kosong") && !strings.Contains(strings.ToLower(adzan), "sholat jum'at") {
			totalDutiesAssigned++
			if statsMap[adzan] == nil {
				statsMap[adzan] = &officerAccum{}
			}
			statsMap[adzan].assigned++
			if adzanExec == 1 {
				statsMap[adzan].executed++
				totalDutiesExecuted++
			} else {
				statsMap[adzan].missed++
			}
		}

		imam = strings.TrimSpace(imam)
		if imam != "" && !strings.EqualFold(imam, "libur") && !strings.EqualFold(imam, "kosong") && !strings.Contains(strings.ToLower(imam), "sholat jum'at") {
			totalDutiesAssigned++
			if statsMap[imam] == nil {
				statsMap[imam] = &officerAccum{}
			}
			statsMap[imam].assigned++
			if imamExec == 1 {
				statsMap[imam].executed++
				totalDutiesExecuted++
			} else {
				statsMap[imam].missed++
			}
		}
	}

	var officerList []OfficerAttendanceSummary
	for name, acc := range statsMap {
		pct := 0.0
		if acc.assigned > 0 {
			pct = (float64(acc.executed) / float64(acc.assigned)) * 100.0
		}
		officerList = append(officerList, OfficerAttendanceSummary{
			OfficerName:   name,
			TotalAssigned: acc.assigned,
			TotalExecuted: acc.executed,
			TotalMissed:   acc.missed,
			Percentage:    pct,
		})
	}

	// Sort officers: highest percentage first, then most assigned, then name
	sort.Slice(officerList, func(i, j int) bool {
		if officerList[i].Percentage != officerList[j].Percentage {
			return officerList[i].Percentage > officerList[j].Percentage
		}
		if officerList[i].TotalAssigned != officerList[j].TotalAssigned {
			return officerList[i].TotalAssigned > officerList[j].TotalAssigned
		}
		return officerList[i].OfficerName < officerList[j].OfficerName
	})

	overallPct := 0.0
	if totalDutiesAssigned > 0 {
		overallPct = (float64(totalDutiesExecuted) / float64(totalDutiesAssigned)) * 100.0
	}

	return &MonthlyRecapData{
		Year:         year,
		Month:        month,
		TotalDuties:  totalDutiesAssigned,
		OverallPct:   overallPct,
		OfficerStats: officerList,
	}, nil
}

// GetMonthlyOfficerDetail returns all dates/prayers where the specified officer did not execute duty in a month.
func (s *Storage) GetMonthlyOfficerDetail(year int, month int, officerName string) ([]MissedDutyDetail, int, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	monthPrefix := fmt.Sprintf("%04d-%02d-%%", year, month)
	target := strings.TrimSpace(officerName)

	query := `
	SELECT prayer_date, prayer_name, adzan_officer, imam_officer,
	       adzan_executed, imam_executed, reporter_jid, updated_at
	FROM duty_attendance_logs
	WHERE prayer_date LIKE ?
	  AND (LOWER(adzan_officer) = LOWER(?) OR LOWER(imam_officer) = LOWER(?))
	ORDER BY prayer_date ASC, id ASC;
	`
	rows, err := s.db.Query(query, monthPrefix, target, target)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to query officer details: %w", err)
	}
	defer rows.Close()

	var missed []MissedDutyDetail
	totalAssigned := 0
	totalExecuted := 0

	for rows.Next() {
		var pDate, pName, adzan, imam, reporter, updatedStr string
		var adzanExec, imamExec int

		if err := rows.Scan(&pDate, &pName, &adzan, &imam, &adzanExec, &imamExec, &reporter, &updatedStr); err != nil {
			return nil, 0, 0, err
		}

		updatedTime, _ := parseFlexibleTime(updatedStr)

		if strings.EqualFold(adzan, target) {
			totalAssigned++
			if adzanExec == 1 {
				totalExecuted++
			} else {
				missed = append(missed, MissedDutyDetail{
					PrayerDate:  pDate,
					PrayerName:  pName,
					Role:        "Adzan",
					ReporterJID: reporter,
					UpdatedAt:   updatedTime,
				})
			}
		}

		if strings.EqualFold(imam, target) {
			totalAssigned++
			if imamExec == 1 {
				totalExecuted++
			} else {
				missed = append(missed, MissedDutyDetail{
					PrayerDate:  pDate,
					PrayerName:  pName,
					Role:        "Imam",
					ReporterJID: reporter,
					UpdatedAt:   updatedTime,
				})
			}
		}
	}

	return missed, totalAssigned, totalExecuted, nil
}

