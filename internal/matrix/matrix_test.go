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
			subuhAdz: "Imam", subuhImam: "Haris",
			zhuhurAdz: "Ruzi", zhuhurImam: "Basit", zhuhurSkip: false,
			asharAdz: "Arjuna", asharImam: "Ananda",
			maghribAdz: "Ruzi", maghribImm: "Ananda",
			isyaAdz: "Basit", isyaImam: "Iskandar",
		},
		{
			weekday: time.Tuesday, dayName: "Selasa",
			subuhAdz: "Basit", subuhImam: "Haris",
			zhuhurAdz: "Arjuna", zhuhurImam: "Ruzi", zhuhurSkip: false,
			asharAdz: "Makhasin", asharImam: "Fajar",
			maghribAdz: "Imam", maghribImm: "T",
			isyaAdz: "Makhasin", isyaImam: "Imam",
		},
		{
			weekday: time.Wednesday, dayName: "Rabu",
			subuhAdz: "Basit", subuhImam: "Haris",
			zhuhurAdz: "T", zhuhurImam: "Ruzi", zhuhurSkip: false,
			asharAdz: "Ruzi", asharImam: "T",
			maghribAdz: "Iskandar", maghribImm: "Arjuna",
			isyaAdz: "Ruzi", isyaImam: "Imam",
		},
		{
			weekday: time.Thursday, dayName: "Kamis",
			subuhAdz: "Ananda", subuhImam: "Haris",
			zhuhurAdz: "T", zhuhurImam: "Ruzi", zhuhurSkip: false,
			asharAdz: "Fajar", asharImam: "Arjuna",
			maghribAdz: "Basit", maghribImm: "Imam",
			isyaAdz: "Fajar", isyaImam: "Makhasin",
		},
		{
			weekday: time.Friday, dayName: "Jumat",
			subuhAdz: "Makhasin", subuhImam: "Haris",
			zhuhurAdz: "", zhuhurImam: "", zhuhurSkip: true, // Friday Zhuhur is skipped
			asharAdz: "Ananda", asharImam: "Basit",
			maghribAdz: "Fajar", maghribImm: "Arjuna",
			isyaAdz: "Imam", isyaImam: "Makhasin",
		},
		{
			weekday: time.Saturday, dayName: "Sabtu",
			subuhAdz: "Basit", subuhImam: "Haris",
			zhuhurAdz: "Fajar", zhuhurImam: "Ananda", zhuhurSkip: false,
			asharAdz: "Iskandar", asharImam: "Makhasin",
			maghribAdz: "Iskandar", maghribImm: "Ananda",
			isyaAdz: "Imam", isyaImam: "Arjuna",
		},
		{
			weekday: time.Sunday, dayName: "Minggu",
			subuhAdz: "Arjuna", subuhImam: "Haris",
			zhuhurAdz: "Iskandar", zhuhurImam: "Fajar", zhuhurSkip: false,
			asharAdz: "Makhasin", asharImam: "Ananda",
			maghribAdz: "Iskandar", maghribImm: "Fajar",
			isyaAdz: "Fajar", isyaImam: "Iskandar",
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
		"Iskandar", // 1 (index 0)
		"Haris",    // 2 (index 1)
		"Thoriq",   // 3 (index 2)
		"Ruzi",     // 4 (index 3)
		"Fajar",    // 5 (index 4)
		"Ananda",   // 6 (index 5)
		"Makhasin", // 7 (index 6)
		"Arjuna",   // 8 (index 7)
		"Imam",     // 9 (index 8)
		"Basit",    // 10 (index 9)
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
	if GetKultumSpeaker(10) != "Iskandar" {
		t.Errorf("Speaker at index 10 = %q; want 'Iskandar' (wrapped)", GetKultumSpeaker(10))
	}
	if GetKultumSpeaker(11) != "Haris" {
		t.Errorf("Speaker at index 11 = %q; want 'Haris' (wrapped)", GetKultumSpeaker(11))
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
