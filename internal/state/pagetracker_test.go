package state

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNewPageTracker(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	pt, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	if pt == nil {
		t.Fatal("NewPageTracker() returned nil")
	}

	if pt.Current() != 1 {
		t.Errorf("expected default page 1, got %d", pt.Current())
	}
}

func TestNewPageTracker_CustomDefault(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	pt, err := NewPageTracker(filePath, 5)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	if pt.Current() != 5 {
		t.Errorf("expected default page 5, got %d", pt.Current())
	}
}

func TestPageTracker_Current(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	pt, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	if pt.Current() != 1 {
		t.Errorf("Current() = %d, want 1", pt.Current())
	}
}

func TestPageTracker_Next(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	pt, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	// Increment to page 2
	if err := pt.Next(10); err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	if pt.Current() != 2 {
		t.Errorf("Current() = %d, want 2", pt.Current())
	}

	// Increment to page 3
	if err := pt.Next(10); err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	if pt.Current() != 3 {
		t.Errorf("Current() = %d, want 3", pt.Current())
	}
}

func TestPageTracker_Next_Wraparound(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	pt, err := NewPageTracker(filePath, 5)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	// Page 5 is the last page (totalPages=5)
	// Should wrap back to 1
	if err := pt.Next(5); err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	if pt.Current() != 1 {
		t.Errorf("Current() = %d, want 1 (after wraparound)", pt.Current())
	}
}

func TestPageTracker_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	// Create tracker and advance to page 3
	pt1, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	pt1.Next(10)
	pt1.Next(10)

	if pt1.Current() != 3 {
		t.Fatalf("expected page 3, got %d", pt1.Current())
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("page tracker file was not created")
	}

	// Load into new tracker
	pt2, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error on reload: %v", err)
	}

	if err := pt2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if pt2.Current() != 3 {
		t.Errorf("Current() = %d after load, want 3", pt2.Current())
	}
}

func TestPageTracker_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	// NewPageTracker should succeed (ignores IsNotExist errors)
	pt, err := NewPageTracker(filePath, 7)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	// Direct call to Load() returns error for non-existent file
	err = pt.Load()
	if err == nil {
		t.Fatal("Load() should return error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Load() should return IsNotExist error, got: %v", err)
	}

	// Should still have default page (Load preserves current on error)
	if pt.Current() != 7 {
		t.Errorf("Current() = %d, want 7 (default)", pt.Current())
	}
}

func TestPageTracker_AtomicSave(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	pt, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	if err := pt.Next(10); err != nil {
		t.Fatalf("Next() error: %v", err)
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

	// Verify the actual file exists and has correct content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	page, err := strconv.Atoi(string(content))
	if err != nil {
		t.Fatalf("invalid page content: %s", content)
	}

	if page != 2 {
		t.Errorf("file content = %d, want 2", page)
	}
}

func TestPageTracker_InvalidFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	// Create file with invalid content
	if err := os.WriteFile(filePath, []byte("not a number"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// NewPageTracker should return error for invalid file content
	pt, err := NewPageTracker(filePath, 5)
	if err == nil {
		t.Fatal("NewPageTracker() should return error for invalid file content")
	}

	// Verify it's a parse error
	if pt != nil {
		t.Error("NewPageTracker() should return nil tracker on error")
	}
}

func TestPageTracker_ZeroPage(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".current_page.txt")

	// Create file with page 0
	if err := os.WriteFile(filePath, []byte("0"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	pt, err := NewPageTracker(filePath, 1)
	if err != nil {
		t.Fatalf("NewPageTracker() error: %v", err)
	}

	// Implementation loads page 0 as-is (no validation)
	if pt.Current() != 0 {
		t.Errorf("Current() = %d after loading page 0, want 0", pt.Current())
	}
}
