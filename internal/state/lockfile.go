package state

import (
	"fmt"
	"os"
	"syscall"
)

// LockFile manages concurrent execution prevention using file locking
type LockFile struct {
	path string
	file *os.File
}

// NewLockFile creates a new lock file manager
func NewLockFile(path string) *LockFile {
	return &LockFile{path: path}
}

// Acquire attempts to acquire the lock file
// Returns an error if the lock is already held by another process
func (lf *LockFile) Acquire() error {
	// Create or open the lock file
	f, err := os.OpenFile(lf.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}

	// Try to acquire an exclusive lock (non-blocking)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("another instance is already running")
		}
		return fmt.Errorf("acquire lock: %w", err)
	}

	// Write PID to lock file for debugging
	if _, err := f.WriteString(fmt.Sprintf("%d\n", os.Getpid())); err != nil {
		f.Close()
		return fmt.Errorf("write PID to lock file: %w", err)
	}

	lf.file = f
	return nil
}

// Release releases the lock file
// The lock is automatically released when the process exits, but explicit
// release allows for cleanup
func (lf *LockFile) Release() error {
	if lf.file == nil {
		return nil
	}

	// Unlock the file
	if err := syscall.Flock(int(lf.file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("unlock file: %w", err)
	}

	// Close the file
	if err := lf.file.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}

	// Remove the lock file
	if err := os.Remove(lf.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove lock file: %w", err)
	}

	lf.file = nil
	return nil
}
