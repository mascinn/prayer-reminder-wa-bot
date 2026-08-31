package storage

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Storage manages persistent SQLite state for the bot.
type Storage struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStorage initializes SQLite connection and runs schema migrations.
func NewStorage(dbPath string) (*Storage, error) {
	// Enable WAL mode and busy timeout for concurrent safety
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

func (s *Storage) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS bot_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.Exec(query)
	return err
}

// Close closes the underlying SQLite database.
func (s *Storage) Close() error {
	return s.db.Close()
}

// GetState retrieves a key value from bot_state.
func (s *Storage) GetState(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var val string
	err := s.db.QueryRow("SELECT value FROM bot_state WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
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
	if err != nil {
		return 0, err
	}
	if val == "" {
		return 0, nil
	}
	idx, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Warning: Invalid kultum_index %q, resetting to 0", val)
		return 0, nil
	}
	return idx, nil
}

// SetKultumIndex updates the current Kultum speaker index.
func (s *Storage) SetKultumIndex(index int) error {
	return s.SetState("kultum_index", strconv.Itoa(index))
}

// AdvanceKultumIndex atomically retrieves current index and advances it to nextIndex.
// Returns the index that was used before advancing.
func (s *Storage) AdvanceKultumIndex(queueLength int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// CacheJadwal saves prayer times json for a date string (YYYY-MM-DD).
func (s *Storage) CacheJadwal(dateKey string, jsonContent string) error {
	return s.SetState("jadwal_"+dateKey, jsonContent)
}

// GetCachedJadwal retrieves cached prayer times json for a date string (YYYY-MM-DD).
func (s *Storage) GetCachedJadwal(dateKey string) (string, error) {
	return s.GetState("jadwal_" + dateKey)
}
