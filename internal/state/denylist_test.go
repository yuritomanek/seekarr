package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDenylist(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	dl, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	if dl == nil {
		t.Fatal("NewDenylist() returned nil")
	}

	if dl.Count() != 0 {
		t.Errorf("new denylist should be empty, got %d entries", dl.Count())
	}
}

func TestDenylist_RecordAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	dl, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	albumID := 123

	// Record failure
	dl.RecordAttempt(albumID, false)

	entry := dl.GetEntry(albumID)
	if entry == nil {
		t.Fatal("GetEntry() returned nil after recording attempt")
	}

	if entry.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", entry.Failures)
	}

	// Record another failure
	dl.RecordAttempt(albumID, false)
	entry = dl.GetEntry(albumID)
	if entry.Failures != 2 {
		t.Errorf("expected 2 failures, got %d", entry.Failures)
	}

	// Record success
	dl.RecordAttempt(albumID, true)
	entry = dl.GetEntry(albumID)
	if entry != nil {
		t.Error("expected entry to be removed from denylist after successful attempt")
	}
}

func TestDenylist_IsDenylisted(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	dl, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	albumID := 456
	maxFailures := 3

	// Not denylisted initially
	if dl.IsDenylisted(albumID, maxFailures) {
		t.Error("album should not be denylisted initially")
	}

	// Record failures
	dl.RecordAttempt(albumID, false)
	dl.RecordAttempt(albumID, false)

	// Still not denylisted (2 < 3)
	if dl.IsDenylisted(albumID, maxFailures) {
		t.Error("album should not be denylisted with 2 failures when max is 3")
	}

	// Third failure
	dl.RecordAttempt(albumID, false)

	// Now denylisted (3 >= 3)
	if !dl.IsDenylisted(albumID, maxFailures) {
		t.Error("album should be denylisted with 3 failures when max is 3")
	}

	// Success clears denylist
	dl.RecordAttempt(albumID, true)
	if dl.IsDenylisted(albumID, maxFailures) {
		t.Error("album should not be denylisted after successful attempt")
	}
}

func TestDenylist_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	// Create and populate denylist
	dl1, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	dl1.RecordAttempt(100, false)
	dl1.RecordAttempt(100, false)
	dl1.RecordAttempt(200, false)

	if err := dl1.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("denylist file was not created")
	}

	// Load into new denylist
	dl2, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error on reload: %v", err)
	}

	if err := dl2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if dl2.Count() != 2 {
		t.Errorf("expected 2 entries after load, got %d", dl2.Count())
	}

	entry1 := dl2.GetEntry(100)
	if entry1 == nil || entry1.Failures != 2 {
		t.Errorf("expected entry for album 100 with 2 failures")
	}

	entry2 := dl2.GetEntry(200)
	if entry2 == nil || entry2.Failures != 1 {
		t.Errorf("expected entry for album 200 with 1 failure")
	}
}

func TestDenylist_GetEntry(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	dl, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	// Non-existent entry
	entry := dl.GetEntry(999)
	if entry != nil {
		t.Error("GetEntry() should return nil for non-existent album")
	}

	// Add entry
	dl.RecordAttempt(999, false)
	entry = dl.GetEntry(999)
	if entry == nil {
		t.Error("GetEntry() should return entry after recording attempt")
	}

	if entry.AlbumID != 999 {
		t.Errorf("expected AlbumID 999, got %d", entry.AlbumID)
	}
}

func TestDenylist_AtomicSave(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	dl, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	dl.RecordAttempt(1, false)
	if err := dl.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify no temp files left behind
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".tmp" {
			t.Errorf("temporary file left behind: %s", file.Name())
		}
	}

	// Verify the actual file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("denylist file does not exist after save")
	}
}

func TestDenylist_LastAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "denylist.json")

	dl, err := NewDenylist(filePath)
	if err != nil {
		t.Fatalf("NewDenylist() error: %v", err)
	}

	albumID := 789
	before := time.Now()

	dl.RecordAttempt(albumID, false)

	entry := dl.GetEntry(albumID)
	if entry == nil {
		t.Fatal("GetEntry() returned nil")
	}

	if entry.LastAttempt.Before(before) {
		t.Error("LastAttempt should be set to current time")
	}

	if entry.LastAttempt.After(time.Now().Add(time.Second)) {
		t.Error("LastAttempt should not be in the future")
	}
}
