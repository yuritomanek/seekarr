package filter

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yuritomanek/seekarr/internal/slskd"
)

// Filter handles file filtering based on quality criteria
type Filter struct {
	allowedFiletypes []string
}

// NewFilter creates a new filter with the given allowed filetypes
func NewFilter(allowedFiletypes []string) *Filter {
	return &Filter{
		allowedFiletypes: allowedFiletypes,
	}
}

// FileMatches checks if a file matches any of the allowed filetypes
func (f *Filter) FileMatches(file slskd.SearchFile) bool {
	if len(f.allowedFiletypes) == 0 {
		return true // No filter, accept all
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		return false
	}
	ext = ext[1:] // Remove leading dot

	for _, allowedType := range f.allowedFiletypes {
		if f.matchesFiletype(file, ext, allowedType) {
			return true
		}
	}

	return false
}

// matchesFiletype checks if a file matches a specific filetype pattern
// Patterns can be:
// - "flac" (any FLAC file)
// - "flac 24/192" (FLAC with 24-bit depth and 192kHz sample rate)
// - "flac 16/44.1" (FLAC with 16-bit depth and 44.1kHz sample rate)
// - "mp3" (any MP3 file)
// - "mp3 320" (MP3 with 320kbps bitrate)
func (f *Filter) matchesFiletype(file slskd.SearchFile, ext, pattern string) bool {
	parts := strings.Fields(strings.ToLower(pattern))
	if len(parts) == 0 {
		return false
	}

	// Check extension matches
	wantedExt := parts[0]
	if ext != wantedExt {
		return false
	}

	// If just extension, it matches
	if len(parts) == 1 {
		return true
	}

	// Check bitrate/quality specifications
	if ext == "flac" && len(parts) == 2 {
		// Format: "flac 24/192" or "flac 16/44.1"
		qualityParts := strings.Split(parts[1], "/")
		if len(qualityParts) == 2 {
			wantedDepth, err1 := strconv.Atoi(qualityParts[0])
			wantedRate, err2 := parseFloatRate(qualityParts[1])
			if err1 != nil || err2 != nil {
				return false
			}

			// Check if file has the required depth and sample rate
			if file.BitDepth != nil && file.SampleRate != nil {
				return *file.BitDepth == wantedDepth && *file.SampleRate == wantedRate
			}
			return false
		}
	} else if ext == "mp3" && len(parts) == 2 {
		// Format: "mp3 320"
		wantedBitrate, err := strconv.Atoi(parts[1])
		if err != nil {
			return false
		}

		// Check if file has the required bitrate
		if file.BitRate != nil {
			return *file.BitRate == wantedBitrate
		}
		return false
	}

	return false
}

// parseFloatRate converts "192" to 192000 and "44.1" to 44100
func parseFloatRate(s string) (int, error) {
	if strings.Contains(s, ".") {
		// Handle "44.1" -> 44100
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		return int(f * 1000), nil
	}
	// Handle "192" -> 192000
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return i * 1000, nil
}

// FilterFiles returns only files that match the allowed filetypes
func (f *Filter) FilterFiles(files []slskd.SearchFile) []slskd.SearchFile {
	if len(f.allowedFiletypes) == 0 {
		return files // No filter, return all
	}

	var filtered []slskd.SearchFile
	for _, file := range files {
		if f.FileMatches(file) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// FilterFilesDebug returns filtered files and information about what was filtered
func (f *Filter) FilterFilesDebug(files []slskd.SearchFile) ([]slskd.SearchFile, []FileFilterInfo) {
	if len(f.allowedFiletypes) == 0 {
		return files, nil // No filter, return all
	}

	var filtered []slskd.SearchFile
	var filterInfo []FileFilterInfo

	for _, file := range files {
		matched := f.FileMatches(file)
		if matched {
			filtered = append(filtered, file)
		}

		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != "" {
			ext = ext[1:] // Remove leading dot
		}

		info := FileFilterInfo{
			Filename:   filepath.Base(file.Filename),
			Extension:  ext,
			BitRate:    file.BitRate,
			SampleRate: file.SampleRate,
			BitDepth:   file.BitDepth,
			Matched:    matched,
		}
		filterInfo = append(filterInfo, info)
	}

	return filtered, filterInfo
}

// FileFilterInfo contains debug information about file filtering
type FileFilterInfo struct {
	Filename   string
	Extension  string
	BitRate    *int
	SampleRate *int
	BitDepth   *int
	Matched    bool
}
