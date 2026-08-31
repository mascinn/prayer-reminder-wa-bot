package templates

import (
	"strings"
	"testing"
	"time"

	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
)

var testMembers = []phonebook.Member{
	{DisplayName: "Ahmad", Phone: "6281100000001", Aliases: []string{"ahmad"}},
	{DisplayName: "Zaid", Phone: "6281100000002", Aliases: []string{"zaid"}},
	{DisplayName: "Bilal", Phone: "6281100000003", Aliases: []string{"bilal"}},
	{DisplayName: "Umar", Phone: "6281100000004", Aliases: []string{"umar"}},
	{DisplayName: "Ali", Phone: "6281100000005", Aliases: []string{"ali"}},
}

func TestBuildDaytimePrayerReminder(t *testing.T) {
	reg := phonebook.NewRegistryFromMembers(testMembers)
	loc := time.FixedZone("WIB", 7*3600)
	prayerTime := time.Date(2026, 8, 31, 15, 20, 0, 0, loc) // Ashar

	duty := matrix.DutyAssignment{
		Adzan: "Ahmad",
		Imam:  "Zaid",
	}

	msg := BuildDaytimePrayerReminder(reg, matrix.PrayerAshar, prayerTime, duty)

	// Check that text contains relevant headers and mentions
	if !strings.Contains(msg.Text, "PENGINGAT SHOLAT ASHAR") {
		t.Errorf("Message missing prayer title: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6281100000001 (Ahmad)") {
		t.Errorf("Message missing Ahmad mention tag: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6281100000002 (Zaid)") {
		t.Errorf("Message missing Zaid mention tag: %s", msg.Text)
	}

	// Check MentionedJID slice
	expectedJIDs := []string{
		"6281100000001@s.whatsapp.net",
		"6281100000002@s.whatsapp.net",
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
		Adzan: "Bilal",
		Imam:  "Umar",
	}
	speaker := "Ali"

	msg := BuildSubuhKultumReminder(reg, tomorrow, "04:44", duty, speaker)

	if !strings.Contains(msg.Text, "PENGINGAT SUBUH & KULTUM") {
		t.Errorf("Message missing title: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6281100000003 (Bilal)") {
		t.Errorf("Message missing Bilal tag: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6281100000004 (Umar)") {
		t.Errorf("Message missing Umar tag: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "@6281100000005 (Ali)") {
		t.Errorf("Message missing Ali tag: %s", msg.Text)
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
