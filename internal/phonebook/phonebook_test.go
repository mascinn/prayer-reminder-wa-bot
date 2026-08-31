package phonebook

import (
	"testing"
)

var sampleTestMembers = []Member{
	{DisplayName: "Ahmad", Phone: "6281100000001", Aliases: []string{"ahmad", "ahmad testing"}},
	{DisplayName: "Zaid", Phone: "6281100000002", Aliases: []string{"zaid"}},
	{DisplayName: "Umar", Phone: "6281100000003", Aliases: []string{"umar"}},
	{DisplayName: "Bilal", Phone: "6281100000004", Aliases: []string{"bilal"}},
	{DisplayName: "Ali", Phone: "6281100000005", Aliases: []string{"ali"}},
	{DisplayName: "Usman", Phone: "6281100000006", Aliases: []string{"usman"}},
	{DisplayName: "Hamzah", Phone: "6281100000007", Aliases: []string{"hamzah"}},
	{DisplayName: "Hasan", Phone: "6281100000008", Aliases: []string{"hasan"}},
	{DisplayName: "Husain", Phone: "6281100000009", Aliases: []string{"husain"}},
	{DisplayName: "Salman", Phone: "6281100000010", Aliases: []string{"salman"}},
	{DisplayName: "Khalid", Phone: "6281100000011", Aliases: []string{"khalid"}},
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
		{"Ahmad", "Ahmad", "6281100000001", "6281100000001@s.whatsapp.net", "@6281100000001"},
		{"ahmad", "Ahmad", "6281100000001", "6281100000001@s.whatsapp.net", "@6281100000001"},
		{"ahmad testing", "Ahmad", "6281100000001", "6281100000001@s.whatsapp.net", "@6281100000001"},
		{"Zaid", "Zaid", "6281100000002", "6281100000002@s.whatsapp.net", "@6281100000002"},
		{"Umar", "Umar", "6281100000003", "6281100000003@s.whatsapp.net", "@6281100000003"},
		{"Bilal", "Bilal", "6281100000004", "6281100000004@s.whatsapp.net", "@6281100000004"},
		{"Ali", "Ali", "6281100000005", "6281100000005@s.whatsapp.net", "@6281100000005"},
		{"Usman", "Usman", "6281100000006", "6281100000006@s.whatsapp.net", "@6281100000006"},
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
	jsonStr := `[{"display_name":"Testing","phone":"6289999999999","aliases":["testing"]}]`
	reg := LoadRegistry("", jsonStr)

	m, ok := reg.Find("testing")
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

func TestMultiplePhones(t *testing.T) {
	members := []Member{
		{
			DisplayName: "Ahmad",
			Phones:      []string{"6281100000001", "6281100000002"},
			Aliases:     []string{"ahmad", "ahmad test"},
		},
	}
	reg := NewRegistryFromMembers(members)
	m, ok := reg.Find("ahmad test")
	if !ok {
		t.Fatal("Failed to find ahmad test")
	}
	jids := m.WhatsAppJIDs()
	if len(jids) != 2 {
		t.Fatalf("len(jids) = %d; want 2", len(jids))
	}
	if jids[0] != "6281100000001@s.whatsapp.net" || jids[1] != "6281100000002@s.whatsapp.net" {
		t.Errorf("Unexpected JIDs: %v", jids)
	}
	tag := reg.FormatMention("ahmad")
	if tag != "@6281100000001 @6281100000002" {
		t.Errorf("FormatMention = %q; want '@6281100000001 @6281100000002'", tag)
	}
}
