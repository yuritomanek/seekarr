package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Denylist manages albums that have repeatedly failed to find matches
type Denylist struct {
	mu       sync.RWMutex
	entries  map[string]*DenylistEntry
	filePath string
}

// DenylistEntry tracks search failures for an album
type DenylistEntry struct {
	AlbumID     int       `json:"album_id"`
	Failures    int       `json:"failures"`
	LastAttempt time.Time `json:"last_attempt"`
}

// NewDenylist creates a new denylist manager
func NewDenylist(filePath string) (*Denylist, error) {
	d := &Denylist{
		entries:  make(map[string]*DenylistEntry),
		filePath: filePath,
	}

	// Load existing denylist if it exists
	if err := d.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load denylist: %w", err)
	}

	return d, nil
}

// Load reads the denylist from file
func (d *Denylist) Load() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := os.ReadFile(d.filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &d.entries); err != nil {
		return fmt.Errorf("unmarshal denylist: %w", err)
	}

	return nil
}

// Save writes the denylist to file atomically
func (d *Denylist) Save() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Create parent directory if needed
	dir := filepath.Dir(d.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(d.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal denylist: %w", err)
	}

	// Write to temporary file
	tmpFile, err := os.CreateTemp(dir, ".search_denylist.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write denylist: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomically rename
	if err := os.Rename(tmpPath, d.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// IsDenylisted checks if an album should be skipped based on failure count
func (d *Denylist) IsDenylisted(albumID int, maxFailures int) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	key := strconv.Itoa(albumID)
	entry, exists := d.entries[key]
	if !exists {
		return false
	}

	return entry.Failures >= maxFailures
}

// RecordAttempt records a search attempt result for an album
// If success is true, removes the album from the denylist
// If success is false, increments the failure count
func (d *Denylist) RecordAttempt(albumID int, success bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := strconv.Itoa(albumID)

	if success {
		// Remove from denylist on success
		delete(d.entries, key)
		return
	}

	// Increment failures
	entry, exists := d.entries[key]
	if !exists {
		entry = &DenylistEntry{
			AlbumID: albumID,
		}
		d.entries[key] = entry
	}

	entry.Failures++
	entry.LastAttempt = time.Now()
}

// GetEntry returns the denylist entry for an album (for logging/debugging)
func (d *Denylist) GetEntry(albumID int) *DenylistEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()

	key := strconv.Itoa(albumID)
	return d.entries[key]
}

// Count returns the number of denylisted albums
func (d *Denylist) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.entries)
}
