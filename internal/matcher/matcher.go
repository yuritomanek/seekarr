package matcher

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/texttheater/golang-levenshtein/levenshtein"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Matcher handles fuzzy string matching for track names
type Matcher struct {
	minRatio float64
}

// NewMatcher creates a new matcher with the given minimum match ratio
func NewMatcher(minRatio float64) *Matcher {
	return &Matcher{minRatio: minRatio}
}

// MatchTracks checks if all expected tracks match files in the directory
// Returns true if all tracks matched and the average match ratio
func (m *Matcher) MatchTracks(expectedTracks []string, actualFiles []string) (bool, float64) {
	if len(expectedTracks) == 0 || len(actualFiles) == 0 {
		return false, 0.0
	}

	// Need at least as many files as expected tracks
	if len(actualFiles) < len(expectedTracks) {
		return false, 0.0
	}

	matched := 0
	totalRatio := 0.0

	for _, expected := range expectedTracks {
		bestRatio := 0.0

		for _, actual := range actualFiles {
			// Strip file extension from actual filename for better matching
			actualNoExt := ExtractFilename(actual)
			ratio := m.calculateBestRatio(expected, actualNoExt)
			if ratio > bestRatio {
				bestRatio = ratio
			}
		}

		if bestRatio >= m.minRatio {
			matched++
			totalRatio += bestRatio
		}
	}

	if matched == len(expectedTracks) {
		avgRatio := totalRatio / float64(len(expectedTracks))
		return true, avgRatio
	}

	return false, 0.0
}

// MatchTracksDebug is like MatchTracks but returns detailed match information
func (m *Matcher) MatchTracksDebug(expectedTracks []string, actualFiles []string) (bool, float64, []TrackMatchInfo) {
	var matchInfo []TrackMatchInfo

	if len(expectedTracks) == 0 || len(actualFiles) == 0 {
		return false, 0.0, matchInfo
	}

	if len(actualFiles) < len(expectedTracks) {
		return false, 0.0, matchInfo
	}

	matched := 0
	totalRatio := 0.0

	for _, expected := range expectedTracks {
		bestRatio := 0.0
		bestMatch := ""

		for _, actual := range actualFiles {
			actualNoExt := ExtractFilename(actual)
			ratio := m.calculateBestRatio(expected, actualNoExt)
			if ratio > bestRatio {
				bestRatio = ratio
				bestMatch = actual
			}
		}

		info := TrackMatchInfo{
			ExpectedTrack: expected,
			BestMatch:     bestMatch,
			BestRatio:     bestRatio,
			Matched:       bestRatio >= m.minRatio,
		}
		matchInfo = append(matchInfo, info)

		if bestRatio >= m.minRatio {
			matched++
			totalRatio += bestRatio
		}
	}

	if matched == len(expectedTracks) {
		avgRatio := totalRatio / float64(len(expectedTracks))
		return true, avgRatio, matchInfo
	}

	return false, 0.0, matchInfo
}

// TrackMatchInfo contains debug information about track matching
type TrackMatchInfo struct {
	ExpectedTrack string
	BestMatch     string
	BestRatio     float64
	Matched       bool
}

// calculateBestRatio tries multiple matching strategies and returns the best ratio
func (m *Matcher) calculateBestRatio(expected, actual string) float64 {
	// Preprocess both strings
	expectedNorm := m.preprocess(expected)
	actualNorm := m.preprocess(actual)

	ratios := []float64{
		// Direct comparison
		m.ratio(expectedNorm, actualNorm),

		// Space-separated truncation (handles "Artist - Album - Track.flac")
		m.ratioWithTruncation(expectedNorm, actualNorm, " "),

		// Underscore-separated truncation
		m.ratioWithTruncation(expectedNorm, actualNorm, "_"),

		// Hyphen-separated truncation
		m.ratioWithTruncation(expectedNorm, actualNorm, "-"),
	}

	max := 0.0
	for _, r := range ratios {
		if r > max {
			max = r
		}
	}

	return max
}

// preprocess normalizes a string for better matching
// - Unicode NFKD decomposition
// - Strip accents/diacritics
// - Lowercase
// - Collapse whitespace
func (m *Matcher) preprocess(s string) string {
	// Unicode normalization (NFKD) and accent removal
	t := transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)

	// Lowercase
	result = strings.ToLower(result)

	// Collapse multiple spaces into one
	spaceRe := regexp.MustCompile(`\s+`)
	result = spaceRe.ReplaceAllString(result, " ")

	// Trim
	result = strings.TrimSpace(result)

	return result
}

// ratio calculates similarity ratio between two strings using Levenshtein distance
// Returns a value between 0.0 (completely different) and 1.0 (identical)
func (m *Matcher) ratio(a, b string) float64 {
	if a == b {
		return 1.0
	}

	if a == "" || b == "" {
		return 0.0
	}

	distance := levenshtein.DistanceForStrings([]rune(a), []rune(b), levenshtein.DefaultOptions)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// ratioWithTruncation removes prefix from the actual string before comparing
// This handles filenames like "01 - Artist - Album - Track.flac" where we want to match "Track.flac"
func (m *Matcher) ratioWithTruncation(expected, actual, separator string) float64 {
	if separator == "" {
		return m.ratio(expected, actual)
	}

	// Count words in expected
	expectedWords := strings.Fields(expected)
	if len(expectedWords) == 0 {
		return 0.0
	}

	// Split actual by separator
	actualParts := strings.Split(actual, separator)
	if len(actualParts) <= len(expectedWords) {
		return m.ratio(expected, actual)
	}

	// Take last N parts where N = word count of expected
	startIdx := len(actualParts) - len(expectedWords)
	truncated := strings.Join(actualParts[startIdx:], " ")
	truncated = strings.TrimSpace(truncated)

	return m.ratio(expected, truncated)
}

// ExtractFilename removes the file extension from a filename
func ExtractFilename(filename string) string {
	lastDot := strings.LastIndex(filename, ".")
	if lastDot > 0 {
		return filename[:lastDot]
	}
	return filename
}

// SanitizeFolderName removes invalid filesystem characters
func SanitizeFolderName(name string) string {
	// Remove invalid characters: < > : " / \ | ? *
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := re.ReplaceAllString(name, "")
	return strings.TrimSpace(sanitized)
}
