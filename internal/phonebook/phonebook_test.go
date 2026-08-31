package phonebook

import (
	"testing"
)

var sampleTestMembers = []Member{
	{DisplayName: "Fajar", Phone: "6285768971813", Aliases: []string{"fajar"}},
	{DisplayName: "Iskandar", Phone: "6285758426987", Aliases: []string{"iskandar"}},
	{DisplayName: "Ananda", Phone: "6285180530165", Aliases: []string{"ananda", "nanda"}},
	{DisplayName: "Arif", Phone: "6283181878854", Aliases: []string{"arif"}},
	{DisplayName: "Arjuna", Phone: "6285268988283", Aliases: []string{"arjuna", "juna"}},
	{DisplayName: "Basit", Phone: "6285766840697", Aliases: []string{"basit", "abdul basit"}},
	{DisplayName: "Imam", Phone: "6288274018823", Aliases: []string{"imam"}},
	{DisplayName: "Haris", Phone: "6282367759870", Aliases: []string{"haris", "dhiki", "diki"}},
	{DisplayName: "Thoriq", Phone: "6285664249480", Aliases: []string{"thoriq", "torik", "t"}},
	{DisplayName: "Ruzi", Phone: "6282298399181", Aliases: []string{"ruzi"}},
	{DisplayName: "Makhasin", Phone: "6285758970652", Aliases: []string{"makhasin", "khasin"}},
}

func TestPhonebookMappings(t *testing.T) {
	reg := NewRegistryFromMembers(sampleTestMembers)

	tests := []struct {
		inputName    string
		expectedName string
		expectedNum  string
		expectedJID  string
		expectedTag  string
	}{
		{"Fajar", "Fajar", "6285768971813", "6285768971813@s.whatsapp.net", "@6285768971813"},
		{"fajar", "Fajar", "6285768971813", "6285768971813@s.whatsapp.net", "@6285768971813"},
		{"Iskandar", "Iskandar", "6285758426987", "6285758426987@s.whatsapp.net", "@6285758426987"},
		{"Ananda", "Ananda", "6285180530165", "6285180530165@s.whatsapp.net", "@6285180530165"},
		{"nanda", "Ananda", "6285180530165", "6285180530165@s.whatsapp.net", "@6285180530165"},
		{"Arif", "Arif", "6283181878854", "6283181878854@s.whatsapp.net", "@6283181878854"},
		{"Arjuna", "Arjuna", "6285268988283", "6285268988283@s.whatsapp.net", "@6285268988283"},
		{"juna", "Arjuna", "6285268988283", "6285268988283@s.whatsapp.net", "@6285268988283"},
		{"Basit", "Basit", "6285766840697", "6285766840697@s.whatsapp.net", "@6285766840697"},
		{"abdul basit", "Basit", "6285766840697", "6285766840697@s.whatsapp.net", "@6285766840697"},
		{"Imam", "Imam", "6288274018823", "6288274018823@s.whatsapp.net", "@6288274018823"},
		{"Haris", "Haris", "6282367759870", "6282367759870@s.whatsapp.net", "@6282367759870"},
		{"Dhiki", "Haris", "6282367759870", "6282367759870@s.whatsapp.net", "@6282367759870"},
		{"diki", "Haris", "6282367759870", "6282367759870@s.whatsapp.net", "@6282367759870"},
		{"Thoriq", "Thoriq", "6285664249480", "6285664249480@s.whatsapp.net", "@6285664249480"},
		{"Torik", "Thoriq", "6285664249480", "6285664249480@s.whatsapp.net", "@6285664249480"},
		{"T", "Thoriq", "6285664249480", "6285664249480@s.whatsapp.net", "@6285664249480"},
		{"t", "Thoriq", "6285664249480", "6285664249480@s.whatsapp.net", "@6285664249480"},
		{"Ruzi", "Ruzi", "6282298399181", "6282298399181@s.whatsapp.net", "@6282298399181"},
		{"Makhasin", "Makhasin", "6285758970652", "6285758970652@s.whatsapp.net", "@6285758970652"},
		{"khasin", "Makhasin", "6285758970652", "6285758970652@s.whatsapp.net", "@6285758970652"},
	}

	for _, tc := range tests {
		m, ok := reg.Find(tc.inputName)
		if !ok {
			t.Errorf("Failed to find member for %q", tc.inputName)
			continue
		}
		if m.DisplayName != tc.expectedName {
			t.Errorf("Find(%q).DisplayName = %q; want %q", tc.inputName, m.DisplayName, tc.expectedName)
		}
		if m.Phone != tc.expectedNum {
			t.Errorf("Find(%q).Phone = %q; want %q", tc.inputName, m.Phone, tc.expectedNum)
		}
		if m.WhatsAppJID() != tc.expectedJID {
			t.Errorf("Find(%q).WhatsAppJID() = %q; want %q", tc.inputName, m.WhatsAppJID(), tc.expectedJID)
		}
		if tag := reg.FormatMention(tc.inputName); tag != tc.expectedTag {
			t.Errorf("FormatMention(%q) = %q; want %q", tc.inputName, tag, tc.expectedTag)
		}
	}
}

func TestLoadRegistryFromJSON(t *testing.T) {
	jsonStr := `[{"display_name":"Ahmad","phone":"6289999999999","aliases":["ahmad"]}]`
	reg := LoadRegistry("", jsonStr)

	m, ok := reg.Find("ahmad")
	if !ok {
		t.Fatal("Failed to find member from JSON string")
	}
	if m.Phone != "6289999999999" {
		t.Errorf("Phone = %q; want 6289999999999", m.Phone)
	}
}

func TestUnknownMember(t *testing.T) {
	reg := NewRegistryFromMembers(sampleTestMembers)
	tag := reg.FormatMention("UnknownPerson")
	if tag != "@UnknownPerson" {
		t.Errorf("FormatMention('UnknownPerson') = %q; want '@UnknownPerson'", tag)
	}

	jid, ok := reg.GetJID("UnknownPerson")
	if ok || jid != "" {
		t.Errorf("GetJID('UnknownPerson') should return false, got %q, %v", jid, ok)
	}
}
