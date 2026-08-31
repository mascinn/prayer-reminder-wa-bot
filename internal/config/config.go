package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configurations for the reminder bot.
type Config struct {
	TargetJID           string
	DBPath              string
	EnableJumatReminder bool
	Timezone            string
	Location            *time.Location
	CityID              string
	AdminJIDs           []string
	LogLevel            string
	MembersFile         string
	MembersJSON         string
	ScheduleFile        string
	ScheduleJSON        string
}

// LoadConfig reads configuration from .env file (if present) and environment variables.
func LoadConfig() (*Config, error) {
	// Attempt to load .env file; ignore if missing
	_ = godotenv.Load()

	tzStr := getEnv("TIMEZONE", "Asia/Jakarta")
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		log.Printf("Warning: Failed to load timezone %q, falling back to FixedZone UTC+7: %v", tzStr, err)
		loc = time.FixedZone("WIB", 7*3600)
	}

	dbPath := getEnv("DB_PATH", "./data/bot.db")
	// Ensure directory exists for dbPath
	dbDir := filepath.Dir(dbPath)
	if dbDir != "" && dbDir != "." {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Printf("Warning: Failed to create directory for DB %q: %v", dbDir, err)
		}
	}

	enableJumat, _ := strconv.ParseBool(getEnv("ENABLE_JUMAT_REMINDER", "false"))

	adminJIDsStr := getEnv("ADMIN_JIDS", "")
	var adminJIDs []string
	if adminJIDsStr != "" {
		for _, jid := range strings.Split(adminJIDsStr, ",") {
			trimmed := strings.TrimSpace(jid)
			if trimmed != "" {
				adminJIDs = append(adminJIDs, trimmed)
			}
		}
	}

	cfg := &Config{
		TargetJID:           getEnv("TARGET_JID", ""),
		DBPath:              dbPath,
		EnableJumatReminder: enableJumat,
		Timezone:            tzStr,
		Location:            loc,
		CityID:              getEnv("CITY_ID", "1014"), // 1014 = Kota Bandar Lampung / Rajabasa / UNILA (Kemenag)
		AdminJIDs:           adminJIDs,
		LogLevel:            getEnv("LOG_LEVEL", "INFO"),
		MembersFile:         getEnv("MEMBERS_FILE", "./data/members.json"),
		MembersJSON:         getEnv("MEMBERS_JSON", ""),
		ScheduleFile:        getEnv("SCHEDULE_FILE", "./data/schedule.json"),
		ScheduleJSON:        getEnv("SCHEDULE_JSON", ""),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}
