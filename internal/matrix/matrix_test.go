package matrix

import (
	"testing"
	"time"
)

func TestWeeklyMatrixDuties(t *testing.T) {
	tests := []struct {
		weekday    time.Weekday
		dayName    string
		subuhAdz   string
		subuhImam  string
		zhuhurAdz  string
		zhuhurImam string
		zhuhurSkip bool
		asharAdz   string
		asharImam  string
		maghribAdz string
		maghribImm string
		isyaAdz    string
		isyaImam   string
	}{
		{
			weekday: time.Monday, dayName: "Senin",
			subuhAdz: "Ahmad", subuhImam: "Zaid",
			zhuhurAdz: "Bilal", zhuhurImam: "Umar", zhuhurSkip: false,
			asharAdz: "Ali", asharImam: "Usman",
			maghribAdz: "Hamzah", maghribImm: "Hasan",
			isyaAdz: "Husain", isyaImam: "Salman",
		},
		{
			weekday: time.Tuesday, dayName: "Selasa",
			subuhAdz: "Zaid", subuhImam: "Ahmad",
			zhuhurAdz: "Ali", zhuhurImam: "Bilal", zhuhurSkip: false,
			asharAdz: "Usman", asharImam: "Hamzah",
			maghribAdz: "Hasan", maghribImm: "Husain",
			isyaAdz: "Salman", isyaImam: "Ahmad",
		},
		{
			weekday: time.Wednesday, dayName: "Rabu",
			subuhAdz: "Bilal", subuhImam: "Ahmad",
			zhuhurAdz: "Umar", zhuhurImam: "Zaid", zhuhurSkip: false,
			asharAdz: "Ali", asharImam: "Hamzah",
			maghribAdz: "Usman", maghribImm: "Hasan",
			isyaAdz: "Husain", isyaImam: "Salman",
		},
		{
			weekday: time.Thursday, dayName: "Kamis",
			subuhAdz: "Usman", subuhImam: "Zaid",
			zhuhurAdz: "Umar", zhuhurImam: "Bilal", zhuhurSkip: false,
			asharAdz: "Hamzah", asharImam: "Ali",
			maghribAdz: "Hasan", maghribImm: "Husain",
			isyaAdz: "Salman", isyaImam: "Ahmad",
		},
		{
			weekday: time.Friday, dayName: "Jumat",
			subuhAdz: "Hamzah", subuhImam: "Ahmad",
			zhuhurAdz: "", zhuhurImam: "", zhuhurSkip: true, // Friday Zhuhur is skipped
			asharAdz: "Bilal", asharImam: "Ali",
			maghribAdz: "Usman", maghribImm: "Hasan",
			isyaAdz: "Husain", isyaImam: "Salman",
		},
		{
			weekday: time.Saturday, dayName: "Sabtu",
			subuhAdz: "Hasan", subuhImam: "Zaid",
			zhuhurAdz: "Ahmad", zhuhurImam: "Umar", zhuhurSkip: false,
			asharAdz: "Ali", asharImam: "Hamzah",
			maghribAdz: "Bilal", maghribImm: "Usman",
			isyaAdz: "Salman", isyaImam: "Ahmad",
		},
		{
			weekday: time.Sunday, dayName: "Minggu",
			subuhAdz: "Husain", subuhImam: "Ahmad",
			zhuhurAdz: "Zaid", zhuhurImam: "Bilal", zhuhurSkip: false,
			asharAdz: "Umar", asharImam: "Ali",
			maghribAdz: "Hamzah", maghribImm: "Hasan",
			isyaAdz: "Usman", isyaImam: "Salman",
		},
	}

	for _, tc := range tests {
		sched := GetDaySchedule(tc.weekday)
		if sched.DayName != tc.dayName {
			t.Errorf("Weekday %v: DayName = %q; want %q", tc.weekday, sched.DayName, tc.dayName)
		}

		// Subuh
		if sched.Subuh.Adzan != tc.subuhAdz || sched.Subuh.Imam != tc.subuhImam {
			t.Errorf("[%s Subuh] got Adzan=%q, Imam=%q; want Adzan=%q, Imam=%q",
				tc.dayName, sched.Subuh.Adzan, sched.Subuh.Imam, tc.subuhAdz, tc.subuhImam)
		}

		// Zhuhur
		if sched.Zhuhur.Skipped != tc.zhuhurSkip {
			t.Errorf("[%s Zhuhur] Skipped = %v; want %v", tc.dayName, sched.Zhuhur.Skipped, tc.zhuhurSkip)
		}
		if !tc.zhuhurSkip {
			if sched.Zhuhur.Adzan != tc.zhuhurAdz || sched.Zhuhur.Imam != tc.zhuhurImam {
				t.Errorf("[%s Zhuhur] got Adzan=%q, Imam=%q; want Adzan=%q, Imam=%q",
					tc.dayName, sched.Zhuhur.Adzan, sched.Zhuhur.Imam, tc.zhuhurAdz, tc.zhuhurImam)
			}
		}

		// Ashar
		if sched.Ashar.Adzan != tc.asharAdz || sched.Ashar.Imam != tc.asharImam {
			t.Errorf("[%s Ashar] got Adzan=%q, Imam=%q; want Adzan=%q, Imam=%q",
				tc.dayName, sched.Ashar.Adzan, sched.Ashar.Imam, tc.asharAdz, tc.asharImam)
		}

		// Maghrib
		if sched.Maghrib.Adzan != tc.maghribAdz || sched.Maghrib.Imam != tc.maghribImm {
			t.Errorf("[%s Maghrib] got Adzan=%q, Imam=%q; want Adzan=%q, Imam=%q",
				tc.dayName, sched.Maghrib.Adzan, sched.Maghrib.Imam, tc.maghribAdz, tc.maghribImm)
		}

		// Isya
		if sched.Isya.Adzan != tc.isyaAdz || sched.Isya.Imam != tc.isyaImam {
			t.Errorf("[%s Isya] got Adzan=%q, Imam=%q; want Adzan=%q, Imam=%q",
				tc.dayName, sched.Isya.Adzan, sched.Isya.Imam, tc.isyaAdz, tc.isyaImam)
		}
	}
}

func TestKultumRoundRobinQueue(t *testing.T) {
	expectedSpeakers := []string{
		"Ahmad",  // 1 (index 0)
		"Zaid",   // 2 (index 1)
		"Bilal",  // 3 (index 2)
		"Umar",   // 4 (index 3)
		"Ali",    // 5 (index 4)
		"Usman",  // 6 (index 5)
		"Hamzah", // 7 (index 6)
		"Hasan",  // 8 (index 7)
		"Husain", // 9 (index 8)
		"Salman", // 10 (index 9)
	}

	if KultumQueueLen() != 10 {
		t.Fatalf("KultumQueue length = %d; want 10", KultumQueueLen())
	}

	for i, expected := range expectedSpeakers {
		actual := GetKultumSpeaker(i)
		if actual != expected {
			t.Errorf("Speaker at index %d = %q; want %q", i, actual, expected)
		}
	}

	// Test wrapping back to 1
	if GetKultumSpeaker(10) != "Ahmad" {
		t.Errorf("Speaker at index 10 = %q; want 'Ahmad' (wrapped)", GetKultumSpeaker(10))
	}
	if GetKultumSpeaker(11) != "Zaid" {
		t.Errorf("Speaker at index 11 = %q; want 'Zaid' (wrapped)", GetKultumSpeaker(11))
	}

	// Test NextKultumIndex
	if NextKultumIndex(9) != 0 {
		t.Errorf("NextKultumIndex(9) = %d; want 0", NextKultumIndex(9))
	}
	if NextKultumIndex(0) != 1 {
		t.Errorf("NextKultumIndex(0) = %d; want 1", NextKultumIndex(0))
	}
}

func TestPrayerNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected PrayerName
	}{
		{"dzuhur", PrayerZhuhur},
		{"Zhuhur", PrayerZhuhur},
		{"DHUHR", PrayerZhuhur},
		{"ashar", PrayerAshar},
		{"asr", PrayerAshar},
		{"maghrib", PrayerMaghrib},
		{"isya", PrayerIsya},
		{"subuh", PrayerSubuh},
	}

	for _, tc := range tests {
		res := NormalizePrayerName(tc.input)
		if res != tc.expected {
			t.Errorf("NormalizePrayerName(%q) = %v; want %v", tc.input, res, tc.expected)
		}
	}
}

func TestLoadScheduleFromJSON(t *testing.T) {
	defer SetActiveSchedule(DefaultScheduleConfig)
	jsonStr := `{
		"kultum_queue": ["Ali", "Umar"],
		"weekly_matrix": {
			"monday": {
				"day_name": "Senin",
				"subuh": {"adzan": "Ali", "imam": "Umar"}
			}
		}
	}`
	cfg := LoadSchedule("", jsonStr)
	if cfg.KultumQueueLen() != 2 {
		t.Errorf("KultumQueueLen = %d; want 2", cfg.KultumQueueLen())
	}
	if cfg.GetKultumSpeaker(0) != "Ali" {
		t.Errorf("Speaker 0 = %q; want Ali", cfg.GetKultumSpeaker(0))
	}
	sched := cfg.GetDaySchedule(time.Monday)
	if sched.Subuh.Adzan != "Ali" {
		t.Errorf("Monday Subuh Adzan = %q; want Ali", sched.Subuh.Adzan)
	}
}

func TestParseIndonesianWeekday(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Weekday
		valid    bool
	}{
		{"senin", time.Monday, true},
		{"Selasa", time.Tuesday, true},
		{"Rabu", time.Wednesday, true},
		{"kamis", time.Thursday, true},
		{"jumat", time.Friday, true},
		{"jum'at", time.Friday, true},
		{"sabtu", time.Saturday, true},
		{"minggu", time.Sunday, true},
		{"ahad", time.Sunday, true},
		{"invalid", time.Sunday, false},
	}

	for _, tc := range tests {
		wd, ok := ParseIndonesianWeekday(tc.input)
		if ok != tc.valid {
			t.Errorf("ParseIndonesianWeekday(%q) valid = %v; want %v", tc.input, ok, tc.valid)
		}
		if ok && wd != tc.expected {
			t.Errorf("ParseIndonesianWeekday(%q) = %v; want %v", tc.input, wd, tc.expected)
		}
	}
}

func TestGetOfficerWeeklyDutiesAndKultum(t *testing.T) {
	SetActiveSchedule(DefaultScheduleConfig)

	// Test finding duties for Ahmad
	shifts := GetOfficerWeeklyDuties("Ahmad")
	if len(shifts) == 0 {
		t.Errorf("GetOfficerWeeklyDuties('Ahmad') returned 0 shifts; want > 0")
	}

	// Test finding Kultum days for Ahmad in September (Day 1 resets to Ahmad/index 0)
	kultumDays := GetOfficerKultumDaysInMonth("Ahmad", 9, 2026)
	if len(kultumDays) == 0 {
		t.Errorf("GetOfficerKultumDaysInMonth('Ahmad') returned 0 days; want > 0")
	}
}


