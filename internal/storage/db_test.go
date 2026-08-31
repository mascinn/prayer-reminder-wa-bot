package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageKultumQueue(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStorage(dbPath)
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

	store, err := NewStorage(dbPath)
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

func TestStoragePersistenceAcrossReopen(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "persist.db")

	// Open first time, set values
	store1, err := NewStorage(dbPath)
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
	store2, err := NewStorage(dbPath)
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
