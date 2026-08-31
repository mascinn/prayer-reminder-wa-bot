package api

import (
	"testing"
	"time"
)

func TestParseRawSchedule(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	rawJSON := `{
		"status": true,
		"data": {
			"id": 1001,
			"lokasi": "KAB. LAMPUNG TENGAH",
			"daerah": "LAMPUNG",
			"jadwal": {
				"tanggal": "Senin, 31/08/2026",
				"imsak": "04:34",
				"subuh": "04:44",
				"terbit": "05:56",
				"dhuha": "06:23",
				"dzuhur": "12:03",
				"ashar": "15:20",
				"maghrib": "18:03",
				"isya": "19:12",
				"date": "2026-08-31"
			}
		}
	}`

	baseDate := time.Date(2026, 8, 31, 10, 0, 0, 0, loc)
	parsed, err := ParseRawSchedule(rawJSON, baseDate, loc)
	if err != nil {
		t.Fatalf("ParseRawSchedule failed: %v", err)
	}

	if parsed.Date != "2026-08-31" {
		t.Errorf("Date = %q; want '2026-08-31'", parsed.Date)
	}
	if parsed.Subuh.Hour() != 4 || parsed.Subuh.Minute() != 44 {
		t.Errorf("Subuh time = %s; want 04:44", parsed.Subuh.Format("15:04"))
	}
	if parsed.Zhuhur.Hour() != 12 || parsed.Zhuhur.Minute() != 3 {
		t.Errorf("Zhuhur time = %s; want 12:03", parsed.Zhuhur.Format("15:04"))
	}
	if parsed.Ashar.Hour() != 15 || parsed.Ashar.Minute() != 20 {
		t.Errorf("Ashar time = %s; want 15:20", parsed.Ashar.Format("15:04"))
	}
	if parsed.Maghrib.Hour() != 18 || parsed.Maghrib.Minute() != 3 {
		t.Errorf("Maghrib time = %s; want 18:03", parsed.Maghrib.Format("15:04"))
	}
	if parsed.Isya.Hour() != 19 || parsed.Isya.Minute() != 12 {
		t.Errorf("Isya time = %s; want 19:12", parsed.Isya.Format("15:04"))
	}
}

func TestLiveMyQuranAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live API integration test in short mode")
	}

	loc := time.FixedZone("WIB", 7*3600)
	client := NewClient("1014")
	now := time.Now().In(loc)

	parsed, _, err := client.FetchJadwal(loc, now)
	if err != nil {
		t.Logf("Notice: Live API call returned error: %v (might be offline or network restricted)", err)
		return
	}

	if parsed == nil {
		t.Fatal("Parsed schedule is nil")
	}

	if parsed.Zhuhur.IsZero() || parsed.Subuh.IsZero() {
		t.Errorf("Prayer times should not be zero values: %+v", parsed)
	}
}
