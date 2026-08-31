package phonebook

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// Member represents a community member with their display name, phone number, and known aliases.
type Member struct {
	DisplayName string   `json:"display_name"`
	Phone       string   `json:"phone"` // International format without + or spaces: 628xxx
	Aliases     []string `json:"aliases"`
}

// WhatsAppJID returns the WhatsApp user JID (e.g. 628xxx@s.whatsapp.net).
func (m Member) WhatsAppJID() string {
	if m.Phone == "" {
		return ""
	}
	return m.Phone + "@s.whatsapp.net"
}

// MentionTag returns the WhatsApp mention string (e.g. @628xxx).
func (m Member) MentionTag() string {
	if m.Phone == "" {
		return "@" + m.DisplayName
	}
	return "@" + m.Phone
}

// Registry stores all registered members and provides lookup utilities.
type Registry struct {
	members []Member
	lookup  map[string]Member
}

// DefaultMembers provides the canonical structure/aliases without hardcoding private phone numbers in Git.
var DefaultMembers = []Member{
	{DisplayName: "Fajar", Aliases: []string{"fajar"}},
	{DisplayName: "Iskandar", Aliases: []string{"iskandar"}},
	{DisplayName: "Ananda", Aliases: []string{"ananda", "nanda"}},
	{DisplayName: "Arif", Aliases: []string{"arif"}},
	{DisplayName: "Arjuna", Aliases: []string{"arjuna", "juna"}},
	{DisplayName: "Basit", Aliases: []string{"basit", "abdul basit"}},
	{DisplayName: "Imam", Aliases: []string{"imam"}},
	{DisplayName: "Haris", Aliases: []string{"haris", "dhiki", "diki"}},
	{DisplayName: "Thoriq", Aliases: []string{"thoriq", "torik", "t"}},
	{DisplayName: "Ruzi", Aliases: []string{"ruzi"}},
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
	// 1. If MEMBERS_JSON is provided via env / Fly secrets
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

// FormatMention formats a name or alias into a WhatsApp mention tag (@628xxx) if found,
// or @Name if not found in the phonebook.
func (r *Registry) FormatMention(nameOrAlias string) string {
	if m, ok := r.Find(nameOrAlias); ok && m.Phone != "" {
		return m.MentionTag()
	}
	return "@" + strings.TrimSpace(nameOrAlias)
}

// GetJID returns the WhatsApp user JID for a given name or alias.
func (r *Registry) GetJID(nameOrAlias string) (string, bool) {
	if m, ok := r.Find(nameOrAlias); ok && m.Phone != "" {
		return m.WhatsAppJID(), true
	}
	return "", false
}

// GetAllMembers returns a slice of all registered members.
func (r *Registry) GetAllMembers() []Member {
	return r.members
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ExportDefaultJSON returns the JSON string representation of members.
func ExportDefaultJSON(members []Member) (string, error) {
	bytes, err := json.MarshalIndent(members, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal members: %w", err)
	}
	return string(bytes), nil
}
