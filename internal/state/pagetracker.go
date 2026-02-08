package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// PageTracker manages pagination state for incrementing_page search mode
type PageTracker struct {
	mu       sync.Mutex
	filePath string
	current  int
}

// NewPageTracker creates a new page tracker with the given file path and default page
func NewPageTracker(filePath string, defaultPage int) (*PageTracker, error) {
	pt := &PageTracker{
		filePath: filePath,
		current:  defaultPage,
	}

	// Try to load existing page number
	if err := pt.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load page tracker: %w", err)
	}

	return pt, nil
}

// Load reads the current page number from file
func (pt *PageTracker) Load() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	data, err := os.ReadFile(pt.filePath)
	if err != nil {
		return err
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil // Keep default
	}

	page, err := strconv.Atoi(content)
	if err != nil {
		return fmt.Errorf("parse page number: %w", err)
	}

	pt.current = page
	return nil
}

// Current returns the current page number (thread-safe)
func (pt *PageTracker) Current() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.current
}

// Next increments the page number and saves it atomically
// If current page exceeds totalPages, wraps back to 1
func (pt *PageTracker) Next(totalPages int) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.current++
	if pt.current > totalPages {
		pt.current = 1
	}

	return pt.saveAtomic()
}

// saveAtomic writes the page number to a temporary file and atomically renames it
// This prevents corruption if the process crashes during write
func (pt *PageTracker) saveAtomic() error {
	// Create parent directory if needed
	dir := filepath.Dir(pt.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write to temporary file in same directory
	tmpFile, err := os.CreateTemp(dir, ".current_page.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write page number
	if _, err := tmpFile.WriteString(strconv.Itoa(pt.current)); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write page number: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomically rename temp file to actual file
	if err := os.Rename(tmpPath, pt.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
