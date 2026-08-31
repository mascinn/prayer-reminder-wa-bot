package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// JadwalResponse represents the JSON response structure from api.myquran.com
type JadwalResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message,omitempty"`
	Data    struct {
		ID     int        `json:"id"`
		Lokasi string     `json:"lokasi"`
		Daerah string     `json:"daerah"`
		Jadwal JadwalData `json:"jadwal"`
	} `json:"data"`
}

// JadwalData holds the prayer times string for a day.
type JadwalData struct {
	Tanggal string `json:"tanggal"`
	Imsak   string `json:"imsak"`
	Subuh   string `json:"subuh"`
	Terbit  string `json:"terbit"`
	Dhuha   string `json:"dhuha"`
	Dzuhur  string `json:"dzuhur"`
	Ashar   string `json:"ashar"`
	Maghrib string `json:"maghrib"`
	Isya    string `json:"isya"`
	Date    string `json:"date"`
}

// ParsedPrayerTimes contains actual time.Time values for each prayer on a given date.
type ParsedPrayerTimes struct {
	Date    string
	Subuh   time.Time
	Zhuhur  time.Time
	Ashar   time.Time
	Maghrib time.Time
	Isya    time.Time
	Raw     JadwalData
}

// Client interacts with MyQuran Prayer Schedule API.
type Client struct {
	httpClient *http.Client
	cityID     string
	baseURL    string
}

// NewClient creates a new MyQuran API client.
func NewClient(cityID string) *Client {
	if cityID == "" {
		cityID = "1014" // Default: Kota Bandar Lampung / Rajabasa (Kemenag)
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		cityID:  cityID,
		baseURL: "https://api.myquran.com/v2/sholat/jadwal",
	}
}

// FetchJadwal fetches prayer schedule for a specific date in given location.
func (c *Client) FetchJadwal(loc *time.Location, targetDate time.Time) (*ParsedPrayerTimes, string, error) {
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	tInLoc := targetDate.In(loc)
	year := tInLoc.Year()
	month := int(tInLoc.Month())
	day := tInLoc.Day()

	url := fmt.Sprintf("%s/%s/%04d/%02d/%02d", c.baseURL, c.cityID, year, month, day)

	var lastErr error
	var respBody []byte

	// Retry up to 3 times
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("User-Agent", "MasjidAlWasii-Bot/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("Attempt %d: HTTP request to %s failed: %v", attempt, url, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		respBody, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned non-200 status %d: %s", resp.StatusCode, string(respBody))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		var parsed JadwalResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			lastErr = fmt.Errorf("failed to decode JSON response: %w", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		if !parsed.Status || parsed.Data.Jadwal.Dzuhur == "" {
			lastErr = fmt.Errorf("invalid API response status or empty schedule: %s", string(respBody))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		// Success parsing
		result, parseErr := parseSchedule(parsed.Data.Jadwal, tInLoc, loc)
		if parseErr != nil {
			return nil, "", parseErr
		}

		return result, string(respBody), nil
	}

	return nil, "", fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// ParseRawSchedule parses raw JadwalData JSON into ParsedPrayerTimes.
func ParseRawSchedule(rawJSON string, tInLoc time.Time, loc *time.Location) (*ParsedPrayerTimes, error) {
	var parsed JadwalResponse
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached schedule: %w", err)
	}
	return parseSchedule(parsed.Data.Jadwal, tInLoc, loc)
}

func parseSchedule(j JadwalData, targetDate time.Time, loc *time.Location) (*ParsedPrayerTimes, error) {
	subuh, err := parseTimeInDate(targetDate, j.Subuh, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid Subuh time %q: %w", j.Subuh, err)
	}

	dzuhur, err := parseTimeInDate(targetDate, j.Dzuhur, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid Dzuhur time %q: %w", j.Dzuhur, err)
	}

	ashar, err := parseTimeInDate(targetDate, j.Ashar, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid Ashar time %q: %w", j.Ashar, err)
	}

	maghrib, err := parseTimeInDate(targetDate, j.Maghrib, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid Maghrib time %q: %w", j.Maghrib, err)
	}

	isya, err := parseTimeInDate(targetDate, j.Isya, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid Isya time %q: %w", j.Isya, err)
	}

	return &ParsedPrayerTimes{
		Date:    fmt.Sprintf("%04d-%02d-%02d", targetDate.Year(), targetDate.Month(), targetDate.Day()),
		Subuh:   subuh,
		Zhuhur:  dzuhur,
		Ashar:   ashar,
		Maghrib: maghrib,
		Isya:    isya,
		Raw:     j,
	}, nil
}

func parseTimeInDate(baseDate time.Time, timeStr string, loc *time.Location) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(timeStr), ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("expected format HH:MM, got %s", timeStr)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(
		baseDate.Year(),
		baseDate.Month(),
		baseDate.Day(),
		hour,
		min,
		0,
		0,
		loc,
	), nil
}
