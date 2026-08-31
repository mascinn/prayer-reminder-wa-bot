package templates

import (
	"strings"
	"testing"
	"time"

	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
)

var testMembers = []phonebook.Member{
	{DisplayName: "Arjuna", Phone: "6285268988283", Aliases: []string{"arjuna", "juna"}},
	{DisplayName: "Ananda", Phone: "6285180530165", Aliases: []string{"ananda", "nanda"}},
	{DisplayName: "Basit", Phone: "6285766840697", Aliases: []string{"basit"}},
	{DisplayName: "Haris", Phone: "6282367759870", Aliases: []string{"haris"}},
	{DisplayName: "Thoriq", Phone: "6285664249480", Aliases: []string{"thoriq"}},
}

func TestBuildDaytimePrayerReminder(t *testing.T) {
	reg := phonebook.NewRegistryFromMembers(testMembers)
	loc := time.FixedZone("WIB", 7*3600)
	prayerTime := time.Date(2026, 8, 31, 15, 20, 0, 0, loc) // Ashar

	duty := matrix.DutyAssignment{
		Adzan: "Arjuna",
		Imam:  "Ananda",
	}

	msg := BuildDaytimePrayerReminder(reg, matrix.PrayerAshar, prayerTime, duty)

	// Check that text contains relevant headers and mentions
	if !strings.Contains(msg.Text, "PENGINGAT SHOLAT ASHAR") {
		t.Errorf("Message missing prayer title: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6285268988283 (Arjuna)") {
		t.Errorf("Message missing Arjuna mention tag: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6285180530165 (Ananda)") {
		t.Errorf("Message missing Ananda mention tag: %s", msg.Text)
	}

	// Check MentionedJID slice
	expectedJIDs := []string{
		"6285268988283@s.whatsapp.net",
		"6285180530165@s.whatsapp.net",
	}

	if len(msg.MentionedJID) != len(expectedJIDs) {
		t.Fatalf("MentionedJID length = %d; want %d", len(msg.MentionedJID), len(expectedJIDs))
	}
	for i, jid := range expectedJIDs {
		if msg.MentionedJID[i] != jid {
			t.Errorf("MentionedJID[%d] = %q; want %q", i, msg.MentionedJID[i], jid)
		}
	}
}

func TestBuildSubuhKultumReminder(t *testing.T) {
	reg := phonebook.NewRegistryFromMembers(testMembers)
	loc := time.FixedZone("WIB", 7*3600)
	tomorrow := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)

	duty := matrix.DutyAssignment{
		Adzan: "Basit",
		Imam:  "Haris",
	}
	speaker := "Thoriq"

	msg := BuildSubuhKultumReminder(reg, tomorrow, "04:44", duty, speaker)

	if !strings.Contains(msg.Text, "PENGINGAT SUBUH & KULTUM") {
		t.Errorf("Message missing title: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6285766840697 (Basit)") {
		t.Errorf("Message missing Basit tag: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6282367759870 (Haris)") {
		t.Errorf("Message missing Haris tag: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6285664249480 (Thoriq)") {
		t.Errorf("Message missing Thoriq tag: %s", msg.Text)
	}

	if len(msg.MentionedJID) != 3 {
		t.Errorf("MentionedJID length = %d; want 3", len(msg.MentionedJID))
	}
}

func TestBuildFridayReminder(t *testing.T) {
	reg := phonebook.NewRegistryFromMembers(testMembers)
	loc := time.FixedZone("WIB", 7*3600)
	friday := time.Date(2026, 9, 4, 0, 0, 0, 0, loc)

	msg := BuildFridayReminder(reg, friday)

	if !strings.Contains(msg.Text, "PENGINGAT PERSIAPAN SHOLAT JUM'AT") {
		t.Errorf("Message missing Friday title: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "sound system") {
		t.Errorf("Message missing preparation content: %s", msg.Text)
	}
}
