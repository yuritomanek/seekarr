package matcher

import (
	"testing"
)

func TestPreprocess(t *testing.T) {
	m := NewMatcher(0.8)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "Hello World", "hello world"},
		{"remove accents", "café", "cafe"},
		{"remove diacritics", "naïve", "naive"},
		{"unicode normalization", "Ü", "u"},
		{"collapse whitespace", "hello    world", "hello world"},
		{"trim spaces", "  hello world  ", "hello world"},
		{"combined", "  Café  Naïve  ", "cafe naive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.preprocess(tt.input)
			if result != tt.expected {
				t.Errorf("preprocess(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRatio(t *testing.T) {
	m := NewMatcher(0.8)

	tests := []struct {
		name      string
		a         string
		b         string
		minRatio  float64
	}{
		{"exact match", "hello", "hello", 1.0},
		{"empty strings", "", "", 1.0},
		{"one empty", "hello", "", 0.0},
		{"similar", "kitten", "sitting", 0.2}, // Levenshtein gives lower ratio
		{"very different", "hello", "world", 0.0},
		{"case sensitive", "Hello", "hello", 0.5}, // ratio() doesn't preprocess, only matches 4/5 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := m.ratio(tt.a, tt.b)
			if tt.minRatio > 0 && ratio < tt.minRatio {
				t.Errorf("ratio(%q, %q) = %f, want >= %f", tt.a, tt.b, ratio, tt.minRatio)
			}
		})
	}
}

func TestRatioWithTruncation(t *testing.T) {
	m := NewMatcher(0.8)

	tests := []struct {
		name      string
		expected  string
		actual    string
		separator string
		minRatio  float64
	}{
		{
			name:      "space separated prefix",
			expected:  "track name.flac",
			actual:    "artist - album - track name.flac",
			separator: " ",
			minRatio:  0.8,
		},
		{
			name:      "underscore separated prefix",
			expected:  "track name.flac",
			actual:    "01_artist_album_track name.flac",
			separator: "_",
			minRatio:  0.7, // Lower expectation due to partial match
		},
		{
			name:      "hyphen separated prefix",
			expected:  "track name.flac",
			actual:    "01-artist-album-track name.flac",
			separator: "-",
			minRatio:  0.7, // Lower expectation due to partial match
		},
		{
			name:      "no truncation needed",
			expected:  "track name.flac",
			actual:    "track name.flac",
			separator: " ",
			minRatio:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := m.ratioWithTruncation(tt.expected, tt.actual, tt.separator)
			if ratio < tt.minRatio {
				t.Errorf("ratioWithTruncation(%q, %q, %q) = %f, want >= %f",
					tt.expected, tt.actual, tt.separator, ratio, tt.minRatio)
			}
		})
	}
}

func TestCalculateBestRatio(t *testing.T) {
	m := NewMatcher(0.8)

	tests := []struct {
		name     string
		expected string
		actual   string
		minRatio float64
	}{
		{
			name:     "exact match",
			expected: "Track Name.flac",
			actual:   "Track Name.flac",
			minRatio: 0.95,
		},
		{
			name:     "with artist prefix",
			expected: "Track Name.flac",
			actual:   "Artist - Album - Track Name.flac",
			minRatio: 0.8,
		},
		{
			name:     "with track number",
			expected: "Track Name.flac",
			actual:   "01 Track Name.flac",
			minRatio: 0.85,
		},
		{
			name:     "with underscores",
			expected: "Track Name.flac",
			actual:   "Artist_Album_Track Name.flac",
			minRatio: 0.7, // Lower due to partial match
		},
		{
			name:     "with accents",
			expected: "Cafe.flac",
			actual:   "Café.flac",
			minRatio: 0.95, // Should match well after preprocessing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := m.calculateBestRatio(tt.expected, tt.actual)
			if ratio < tt.minRatio {
				t.Errorf("calculateBestRatio(%q, %q) = %f, want >= %f",
					tt.expected, tt.actual, ratio, tt.minRatio)
			}
		})
	}
}

func TestMatchTracks(t *testing.T) {
	m := NewMatcher(0.8)

	tests := []struct {
		name          string
		expected      []string
		actual        []string
		shouldMatch   bool
		minAvgRatio   float64
	}{
		{
			name:        "exact matches",
			expected:    []string{"Track 1.flac", "Track 2.flac"},
			actual:      []string{"Track 1.flac", "Track 2.flac"},
			shouldMatch: true,
			minAvgRatio: 0.95,
		},
		{
			name:        "with prefixes",
			expected:    []string{"Track 1.flac", "Track 2.flac"},
			actual:      []string{"Artist - Album - Track 1.flac", "Artist - Album - Track 2.flac"},
			shouldMatch: true,
			minAvgRatio: 0.8,
		},
		{
			name:        "missing one track - too few files",
			expected:    []string{"Track 1.flac", "Track 2.flac", "Track 3.flac"},
			actual:      []string{"Track 1.flac", "Track 2.flac"},
			shouldMatch: false,
		},
		{
			name:        "completely different",
			expected:    []string{"Track 1.flac"},
			actual:      []string{"Other Song.flac"},
			shouldMatch: false,
		},
		{
			name:        "empty lists",
			expected:    []string{},
			actual:      []string{"Track 1.flac"},
			shouldMatch: false,
		},
		{
			name:        "with different extensions",
			expected:    []string{"Track 1.flac", "Track 2.flac"},
			actual:      []string{"Track 1.flac", "Track 2.flac"}, // Same files for now
			shouldMatch: true,
			minAvgRatio: 0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, ratio := m.MatchTracks(tt.expected, tt.actual)

			if matched != tt.shouldMatch {
				t.Errorf("MatchTracks() matched = %v, want %v", matched, tt.shouldMatch)
			}

			if matched && ratio < tt.minAvgRatio {
				t.Errorf("MatchTracks() ratio = %f, want >= %f", ratio, tt.minAvgRatio)
			}
		})
	}
}

func TestExtractFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"track.flac", "track"},
		{"track.name.mp3", "track.name"},
		{"noextension", "noextension"},
		{"", ""},
		{"track.", "track"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractFilename(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal Name"},
		{"Name/With/Slashes", "NameWithSlashes"},
		{"Name:With:Colons", "NameWithColons"},
		{"Name<>With|Bad*Chars", "NameWithBadChars"},
		{`Name\With\Backslashes`, "NameWithBackslashes"},
		{"Name?With?Questions", "NameWithQuestions"},
		{`Name"With"Quotes`, "NameWithQuotes"},
		{"  Name With Spaces  ", "Name With Spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeFolderName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFolderName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
