package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
)

func TestStorageKultumQueue(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStorage(dbPath, "", "")
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initial kultum index should default to 0
	idx, err := store.GetKultumIndex()
	if err != nil {
		t.Fatalf("GetKultumIndex error: %v", err)
	}
	if idx != 0 {
		t.Errorf("Initial index = %d; want 0", idx)
	}

	// Advance through all 10 indices
	queueLen := 10
	for i := 0; i < queueLen; i++ {
		used, err := store.AdvanceKultumIndex(queueLen)
		if err != nil {
			t.Fatalf("AdvanceKultumIndex step %d error: %v", i, err)
		}
		if used != i {
			t.Errorf("Step %d: used index = %d; want %d", i, used, i)
		}
	}

	// Next retrieved index after 10 advances should have wrapped back to 0
	newIdx, err := store.GetKultumIndex()
	if err != nil {
		t.Fatalf("GetKultumIndex error: %v", err)
	}
	if newIdx != 0 {
		t.Errorf("Index after full wrap = %d; want 0", newIdx)
	}
}

func TestStorageGenericStateAndCache(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStorage(dbPath, "", "")
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Test Set and Get state
	err = store.SetState("last_run", "2026-08-31")
	if err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	val, err := store.GetState("last_run")
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if val != "2026-08-31" {
		t.Errorf("GetState('last_run') = %q; want '2026-08-31'", val)
	}

	// Test Cache Jadwal
	sampleJSON := `{"dzuhur":"12:03","ashar":"15:20"}`
	err = store.CacheJadwal("2026-08-31", sampleJSON)
	if err != nil {
		t.Fatalf("CacheJadwal failed: %v", err)
	}

	cached, err := store.GetCachedJadwal("2026-08-31")
	if err != nil {
		t.Fatalf("GetCachedJadwal failed: %v", err)
	}
	if cached != sampleJSON {
		t.Errorf("GetCachedJadwal = %q; want %q", cached, sampleJSON)
	}
}

func TestStorageMembersAndDutySchedule(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStorage(dbPath, "", "")
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Test Save & Load Members
	members := []phonebook.Member{
		{DisplayName: "Ahmad", Phones: []string{"6281100000001", "6281100000002"}, Aliases: []string{"ahmad"}},
		{DisplayName: "Zaid", Phone: "6281100000003", Aliases: []string{"zaid"}},
	}
	if err := store.SaveMembers(members); err != nil {
		t.Fatalf("SaveMembers failed: %v", err)
	}

	loadedMembers, err := store.LoadMembers()
	if err != nil {
		t.Fatalf("LoadMembers failed: %v", err)
	}
	if len(loadedMembers) != 2 {
		t.Fatalf("LoadMembers count = %d; want 2", len(loadedMembers))
	}
	if loadedMembers[0].DisplayName != "Ahmad" || len(loadedMembers[0].Phones) != 2 {
		t.Errorf("Loaded member 0 mismatch: %+v", loadedMembers[0])
	}

	// Test Save & Load Duty Matrix
	matrixMap := map[time.Weekday]matrix.DaySchedule{
		time.Monday: {
			DayName: "Senin",
			Subuh:   matrix.DutyAssignment{Adzan: "Ahmad", Imam: "Zaid"},
			Zhuhur:  matrix.DutyAssignment{Adzan: "Zaid", Imam: "Ahmad"},
		},
	}
	if err := store.SaveDutySchedule(matrixMap); err != nil {
		t.Fatalf("SaveDutySchedule failed: %v", err)
	}

	loadedMatrix, err := store.LoadDutyMatrix()
	if err != nil {
		t.Fatalf("LoadDutyMatrix failed: %v", err)
	}
	if loadedMatrix[time.Monday].Subuh.Adzan != "Ahmad" {
		t.Errorf("Monday Subuh Adzan = %q; want Ahmad", loadedMatrix[time.Monday].Subuh.Adzan)
	}

	// Test LogReminder
	if err := store.LogReminder("subuh", "04:45", "Ahmad", "Zaid", "Umar", "SUCCESS"); err != nil {
		t.Fatalf("LogReminder failed: %v", err)
	}
}

func TestStorageDutyAttendanceAndReactions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_duty.db")

	store, err := NewStorage(dbPath, "", "")
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, loc)
	cutoff := time.Date(2026, 9, 1, 23, 59, 59, 0, loc)

	msgID := "MSG-12345"
	if err := store.SaveDutyRecord(msgID, "2026-09-01", "Zhuhur", "Ahmad", "Zaid", cutoff); err != nil {
		t.Fatalf("SaveDutyRecord failed: %v", err)
	}

	// 1. Initial State: Both executed (1)
	record, err := store.GetDutyRecordByMessageID(msgID)
	if err != nil || record == nil {
		t.Fatalf("GetDutyRecordByMessageID failed: %v", err)
	}
	if !record.AdzanExecuted || !record.ImamExecuted {
		t.Errorf("Initial state should be executed=true, got Adzan=%v, Imam=%v", record.AdzanExecuted, record.ImamExecuted)
	}

	// 2. React 👆 (Adzan tidak menjalankan)
	handled, shouldBotReact, err := store.UpdateDutyReaction(msgID, "👆", "62811111111@s.whatsapp.net", now)
	if err != nil || !handled || !shouldBotReact {
		t.Fatalf("UpdateDutyReaction (👆) error: handled=%v, shouldBotReact=%v, err=%v", handled, shouldBotReact, err)
	}

	record, _ = store.GetDutyRecordByMessageID(msgID)
	if record.AdzanExecuted || !record.ImamExecuted {
		t.Errorf("After 👆: want Adzan=false, Imam=true; got Adzan=%v, Imam=%v", record.AdzanExecuted, record.ImamExecuted)
	}

	// 3. React 👇 (Imam tidak menjalankan)
	handled, shouldBotReact, err = store.UpdateDutyReaction(msgID, "👇", "62811111111@s.whatsapp.net", now)
	if err != nil || !handled || !shouldBotReact {
		t.Fatalf("UpdateDutyReaction (👇) error: handled=%v, shouldBotReact=%v, err=%v", handled, shouldBotReact, err)
	}

	record, _ = store.GetDutyRecordByMessageID(msgID)
	if !record.AdzanExecuted || record.ImamExecuted {
		t.Errorf("After 👇: want Adzan=true, Imam=false; got Adzan=%v, Imam=%v", record.AdzanExecuted, record.ImamExecuted)
	}

	// 4. React ✌️ (Keduanya tidak menjalankan)
	handled, shouldBotReact, err = store.UpdateDutyReaction(msgID, "✌️", "62811111111@s.whatsapp.net", now)
	if err != nil || !handled || !shouldBotReact {
		t.Fatalf("UpdateDutyReaction (✌️) error: handled=%v, shouldBotReact=%v, err=%v", handled, shouldBotReact, err)
	}

	record, _ = store.GetDutyRecordByMessageID(msgID)
	if record.AdzanExecuted || record.ImamExecuted {
		t.Errorf("After ✌️: want Adzan=false, Imam=false; got Adzan=%v, Imam=%v", record.AdzanExecuted, record.ImamExecuted)
	}

	// 5. Un-react "" (Batal / Ralat -> Both Menjalankan)
	handled, shouldBotReact, err = store.UpdateDutyReaction(msgID, "", "62811111111@s.whatsapp.net", now)
	if err != nil || !handled || shouldBotReact {
		t.Fatalf("UpdateDutyReaction (unreact) error: handled=%v, shouldBotReact=%v, err=%v", handled, shouldBotReact, err)
	}

	record, _ = store.GetDutyRecordByMessageID(msgID)
	if !record.AdzanExecuted || !record.ImamExecuted {
		t.Errorf("After un-react: want Adzan=true, Imam=true; got Adzan=%v, Imam=%v", record.AdzanExecuted, record.ImamExecuted)
	}

	// 6. Cutoff expiration (reaction after midnight should be ignored)
	afterMidnight := time.Date(2026, 9, 2, 0, 1, 0, 0, loc)
	handled, shouldBotReact, err = store.UpdateDutyReaction(msgID, "👆", "62811111111@s.whatsapp.net", afterMidnight)
	if err != nil {
		t.Fatalf("Unexpected error on past cutoff reaction: %v", err)
	}
	if handled {
		t.Errorf("Reaction after cutoff should not be handled (isHandled=false)")
	}

	// 7. Monthly Recap & Details
	// Add another duty for testing recap
	msgID2 := "MSG-67890"
	_ = store.SaveDutyRecord(msgID2, "2026-09-02", "Ashar", "Ahmad", "Bilal", cutoff.AddDate(0, 0, 1))
	_, _, _ = store.UpdateDutyReaction(msgID2, "👆", "62822222222@s.whatsapp.net", now.AddDate(0, 0, 1)) // Ahmad missed adzan

	recap, err := store.GetMonthlyRecap(2026, 9)
	if err != nil {
		t.Fatalf("GetMonthlyRecap failed: %v", err)
	}
	if recap.TotalDuties != 4 { // 2 duties x 2 officers = 4
		t.Errorf("Recap TotalDuties = %d; want 4", recap.TotalDuties)
	}

	// Check officer detail for Ahmad
	missed, assigned, executed, err := store.GetMonthlyOfficerDetail(2026, 9, "Ahmad")
	if err != nil {
		t.Fatalf("GetMonthlyOfficerDetail failed: %v", err)
	}
	if assigned != 2 || executed != 1 || len(missed) != 1 {
		t.Errorf("Ahmad stats: assigned=%d, executed=%d, missedCount=%d; want 2, 1, 1", assigned, executed, len(missed))
	}
	if len(missed) > 0 && missed[0].PrayerName != "Ashar" {
		t.Errorf("Ahmad missed prayer = %s; want Ashar", missed[0].PrayerName)
	}
}

func TestStoragePersistenceAcrossReopen(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "persist.db")

	// Open first time, set values
	store1, err := NewStorage(dbPath, "", "")
	if err != nil {
		t.Fatalf("First open failed: %v", err)
	}
	if err := store1.SetKultumIndex(4); err != nil {
		t.Fatalf("SetKultumIndex failed: %v", err)
	}
	store1.Close()

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Database file not created at %s", dbPath)
	}

	// Open second time, verify persistence
	store2, err := NewStorage(dbPath, "", "")
	if err != nil {
		t.Fatalf("Second open failed: %v", err)
	}
	defer store2.Close()

	idx, err := store2.GetKultumIndex()
	if err != nil {
		t.Fatalf("GetKultumIndex after reopen failed: %v", err)
	}
	if idx != 4 {
		t.Errorf("GetKultumIndex after reopen = %d; want 4", idx)
	}
}

