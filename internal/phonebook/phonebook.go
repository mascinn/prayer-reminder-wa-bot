package phonebook

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

// Member represents a community member with their display name, phone number(s), and known aliases.
type Member struct {
	DisplayName string   `json:"display_name"`
	Phone       string   `json:"phone,omitempty"`  // Single number fallback: 628xxx
	Phones      []string `json:"phones,omitempty"` // Multiple numbers support: ["628xxx", "628yyy"]
	Aliases     []string `json:"aliases"`
}

// AllPhones returns all registered phone numbers for the member in international format.
func (m Member) AllPhones() []string {
	var list []string
	if len(m.Phones) > 0 {
		for _, p := range m.Phones {
			cleaned := cleanPhone(p)
			if cleaned != "" {
				list = append(list, cleaned)
			}
		}
	}
	if len(list) == 0 && strings.TrimSpace(m.Phone) != "" {
		cleaned := cleanPhone(m.Phone)
		if cleaned != "" {
			list = append(list, cleaned)
		}
	}
	return list
}

// WhatsAppJID returns the primary WhatsApp user JID (e.g. 628xxx@s.whatsapp.net).
func (m Member) WhatsAppJID() string {
	phones := m.AllPhones()
	if len(phones) == 0 {
		return ""
	}
	return phones[0] + "@s.whatsapp.net"
}

// WhatsAppJIDs returns all WhatsApp user JIDs for the member.
func (m Member) WhatsAppJIDs() []string {
	var jids []string
	for _, p := range m.AllPhones() {
		jids = append(jids, p+"@s.whatsapp.net")
	}
	return jids
}

// MentionTag returns the WhatsApp mention string (e.g. "@628xxx" or "@628xxx @628yyy").
func (m Member) MentionTag() string {
	phones := m.AllPhones()
	if len(phones) == 0 {
		return "@" + m.DisplayName
	}
	var tags []string
	for _, p := range phones {
		tags = append(tags, "@"+p)
	}
	return strings.Join(tags, " ")
}

// Registry stores all registered members and provides lookup utilities.
type Registry struct {
	members []Member
	lookup  map[string]Member
}

// DefaultMembers provides the canonical structure/aliases without hardcoding private phone numbers in Git.
var DefaultMembers = []Member{
	{DisplayName: "Fajar", Aliases: []string{"fajar", "fajar aji pangestu"}},
	{DisplayName: "Iskandar", Aliases: []string{"iskandar"}},
	{DisplayName: "Ananda", Aliases: []string{"ananda", "nanda", "ananda kusuma"}},
	{DisplayName: "Arif", Aliases: []string{"arif", "arif hidayat"}},
	{DisplayName: "Arjuna", Aliases: []string{"arjuna", "juna", "arjuna yulizar mahendra"}},
	{DisplayName: "Basit", Aliases: []string{"basit", "abdul basit", "basit diwa fakara"}},
	{DisplayName: "Imam", Aliases: []string{"imam", "imam rifai"}},
	{DisplayName: "Haris", Aliases: []string{"haris", "dhiki", "diki", "dhiki harisno"}},
	{DisplayName: "Thoriq", Aliases: []string{"thoriq", "torik", "t", "torik lianda"}},
	{DisplayName: "Ruzi", Aliases: []string{"ruzi", "ruzi yandi"}},
	{DisplayName: "Makhasin", Aliases: []string{"makhasin", "khasin"}},
}

// NewRegistryFromMembers creates a Registry from a slice of Members.
func NewRegistryFromMembers(members []Member) *Registry {
	r := &Registry{
		members: members,
		lookup:  make(map[string]Member),
	}
	for _, m := range members {
		r.lookup[normalize(m.DisplayName)] = m
		for _, alias := range m.Aliases {
			r.lookup[normalize(alias)] = m
		}
	}
	return r
}

// LoadRegistry loads member mappings from environment JSON string, file path, or fallback candidates.
func LoadRegistry(membersFile string, membersJSON string) *Registry {
	// 1. If MEMBERS_JSON is provided via env / secrets
	if strings.TrimSpace(membersJSON) != "" {
		var members []Member
		if err := json.Unmarshal([]byte(membersJSON), &members); err == nil && len(members) > 0 {
			log.Printf("[Phonebook] Loaded %d members from MEMBERS_JSON environment variable.", len(members))
			return NewRegistryFromMembers(members)
		}
		log.Println("[Phonebook] Warning: Failed to parse MEMBERS_JSON, falling back to file.")
	}

	// 2. Candidate files in order of preference
	candidates := []string{
		membersFile,
		"./data/members.json",
		"../data/members.json",
		"../../data/members.json",
		"/data/members.json",
		"members.json",
		"../members.json",
		"../../members.json",
		"members.example.json",
		"../members.example.json",
		"../../members.example.json",
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if data, err := os.ReadFile(path); err == nil {
			var members []Member
			if err := json.Unmarshal(data, &members); err == nil && len(members) > 0 {
				log.Printf("[Phonebook] Successfully loaded %d members from %s.", len(members), path)
				return NewRegistryFromMembers(members)
			}
		}
	}

	log.Println("[Phonebook] Warning: No members.json found. Using default placeholder member list.")
	return NewRegistryFromMembers(DefaultMembers)
}

// NewRegistry initializes a Registry with the default members list.
func NewRegistry() *Registry {
	return LoadRegistry("", "")
}

// Find finds a member by exact name or alias (case-insensitive).
func (r *Registry) Find(nameOrAlias string) (Member, bool) {
	m, ok := r.lookup[normalize(nameOrAlias)]
	return m, ok
}

// FormatMention formats a name or alias into WhatsApp mention tag(s) (@628xxx or @628xxx @628yyy).
func (r *Registry) FormatMention(nameOrAlias string) string {
	if m, ok := r.Find(nameOrAlias); ok && len(m.AllPhones()) > 0 {
		return m.MentionTag()
	}
	return "@" + strings.TrimSpace(nameOrAlias)
}

// GetJID returns the primary WhatsApp user JID for a given name or alias.
func (r *Registry) GetJID(nameOrAlias string) (string, bool) {
	if m, ok := r.Find(nameOrAlias); ok && len(m.AllPhones()) > 0 {
		return m.WhatsAppJID(), true
	}
	return "", false
}

// GetAllJIDs returns all WhatsApp user JIDs for a given name or alias (supports multiple numbers).
func (r *Registry) GetAllJIDs(nameOrAlias string) []string {
	if m, ok := r.Find(nameOrAlias); ok {
		return m.WhatsAppJIDs()
	}
	return nil
}

// GetAllMembers returns a slice of all registered members.
func (r *Registry) GetAllMembers() []Member {
	return r.members
}

func cleanPhone(phone string) string {
	digits := ""
	for _, ch := range strings.TrimSpace(phone) {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}
	if strings.HasPrefix(digits, "08") {
		digits = "628" + digits[2:]
	}
	return digits
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
