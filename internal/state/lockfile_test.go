package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLockFile(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "seekarr.lock")

	lf := NewLockFile(lockPath)
	if lf == nil {
		t.Fatal("NewLockFile() returned nil")
	}
}

func TestLockFile_AcquireAndRelease(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "seekarr.lock")

	lf := NewLockFile(lockPath)

	// Acquire lock
	if err := lf.Acquire(); err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	// Verify lock file was created
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file was not created")
	}

	// Release lock
	if err := lf.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	// Lock file may still exist (that's ok), but lock should be released
}

func TestLockFile_DoubleAcquire(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "seekarr.lock")

	lf1 := NewLockFile(lockPath)
	lf2 := NewLockFile(lockPath)

	// First acquire should succeed
	if err := lf1.Acquire(); err != nil {
		t.Fatalf("first Acquire() error: %v", err)
	}
	defer lf1.Release()

	// Second acquire should fail (lock is held)
	if err := lf2.Acquire(); err == nil {
		t.Error("second Acquire() should fail when lock is held")
	}
}

func TestLockFile_ReleaseWithoutAcquire(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "seekarr.lock")

	lf := NewLockFile(lockPath)

	// Release without acquire should not panic
	if err := lf.Release(); err != nil {
		// Error is expected, just make sure it doesn't panic
		t.Logf("Release() error (expected): %v", err)
	}
}

func TestLockFile_AcquireAfterRelease(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "seekarr.lock")

	lf := NewLockFile(lockPath)

	// Acquire
	if err := lf.Acquire(); err != nil {
		t.Fatalf("first Acquire() error: %v", err)
	}

	// Release
	if err := lf.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	// Acquire again should succeed
	if err := lf.Acquire(); err != nil {
		t.Fatalf("second Acquire() error: %v", err)
	}
	defer lf.Release()
}

func TestLockFile_MultipleInstances(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "seekarr.lock")

	lf1 := NewLockFile(lockPath)
	lf2 := NewLockFile(lockPath)
	lf3 := NewLockFile(lockPath)

	// First instance acquires
	if err := lf1.Acquire(); err != nil {
		t.Fatalf("lf1.Acquire() error: %v", err)
	}
	defer lf1.Release()

	// Second and third should fail
	if err := lf2.Acquire(); err == nil {
		t.Error("lf2.Acquire() should fail")
		lf2.Release()
	}

	if err := lf3.Acquire(); err == nil {
		t.Error("lf3.Acquire() should fail")
		lf3.Release()
	}
}

func TestLockFile_NonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "nonexistent", "seekarr.lock")

	lf := NewLockFile(lockPath)

	// Acquire should fail if directory doesn't exist
	err := lf.Acquire()
	if err == nil {
		t.Error("Acquire() should fail for non-existent directory")
		lf.Release()
	}
}

func TestLockFile_CreateDirectoryAndAcquire(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "locks")
	lockPath := filepath.Join(subDir, "seekarr.lock")

	// Create directory first
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	lf := NewLockFile(lockPath)

	// Now acquire should succeed
	if err := lf.Acquire(); err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	defer lf.Release()
}
