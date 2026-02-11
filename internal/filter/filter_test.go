package filter

import (
	"testing"

	"github.com/yuritomanek/seekarr/internal/slskd"
)

func TestNewFilter(t *testing.T) {
	filetypes := []string{"flac", "mp3"}
	f := NewFilter(filetypes)

	if f == nil {
		t.Fatal("NewFilter() returned nil")
	}

	if len(f.allowedFiletypes) != 2 {
		t.Errorf("expected 2 allowed filetypes, got %d", len(f.allowedFiletypes))
	}
}

func TestFileMatches(t *testing.T) {
	tests := []struct {
		name             string
		allowedFiletypes []string
		file             slskd.SearchFile
		want             bool
	}{
		{
			name:             "no filter - accept all",
			allowedFiletypes: []string{},
			file:             slskd.SearchFile{Filename: "test.mp3"},
			want:             true,
		},
		{
			name:             "flac file matches flac filter",
			allowedFiletypes: []string{"flac"},
			file:             slskd.SearchFile{Filename: "test.flac"},
			want:             true,
		},
		{
			name:             "mp3 file doesn't match flac filter",
			allowedFiletypes: []string{"flac"},
			file:             slskd.SearchFile{Filename: "test.mp3"},
			want:             false,
		},
		{
			name:             "file without extension",
			allowedFiletypes: []string{"flac"},
			file:             slskd.SearchFile{Filename: "test"},
			want:             false,
		},
		{
			name:             "flac 24/192 matches exact spec",
			allowedFiletypes: []string{"flac 24/192"},
			file: slskd.SearchFile{
				Filename:   "test.flac",
				BitDepth:   intPtr(24),
				SampleRate: intPtr(192000),
			},
			want: true,
		},
		{
			name:             "flac 24/192 doesn't match 16/44.1",
			allowedFiletypes: []string{"flac 24/192"},
			file: slskd.SearchFile{
				Filename:   "test.flac",
				BitDepth:   intPtr(16),
				SampleRate: intPtr(44100),
			},
			want: false,
		},
		{
			name:             "flac 16/44.1 matches exact spec",
			allowedFiletypes: []string{"flac 16/44.1"},
			file: slskd.SearchFile{
				Filename:   "test.flac",
				BitDepth:   intPtr(16),
				SampleRate: intPtr(44100),
			},
			want: true,
		},
		{
			name:             "flac without quality info doesn't match quality spec",
			allowedFiletypes: []string{"flac 24/192"},
			file:             slskd.SearchFile{Filename: "test.flac"},
			want:             false,
		},
		{
			name:             "mp3 320 matches exact bitrate",
			allowedFiletypes: []string{"mp3 320"},
			file: slskd.SearchFile{
				Filename: "test.mp3",
				BitRate:  intPtr(320),
			},
			want: true,
		},
		{
			name:             "mp3 320 doesn't match 192",
			allowedFiletypes: []string{"mp3 320"},
			file: slskd.SearchFile{
				Filename: "test.mp3",
				BitRate:  intPtr(192),
			},
			want: false,
		},
		{
			name:             "mp3 without bitrate info doesn't match bitrate spec",
			allowedFiletypes: []string{"mp3 320"},
			file:             slskd.SearchFile{Filename: "test.mp3"},
			want:             false,
		},
		{
			name:             "any flac matches generic flac filter",
			allowedFiletypes: []string{"flac"},
			file: slskd.SearchFile{
				Filename:   "test.flac",
				BitDepth:   intPtr(24),
				SampleRate: intPtr(192000),
			},
			want: true,
		},
		{
			name:             "multiple filetypes - first matches",
			allowedFiletypes: []string{"flac", "mp3"},
			file:             slskd.SearchFile{Filename: "test.flac"},
			want:             true,
		},
		{
			name:             "multiple filetypes - second matches",
			allowedFiletypes: []string{"flac", "mp3"},
			file:             slskd.SearchFile{Filename: "test.mp3"},
			want:             true,
		},
		{
			name:             "case insensitive extension",
			allowedFiletypes: []string{"flac"},
			file:             slskd.SearchFile{Filename: "test.FLAC"},
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.allowedFiletypes)
			got := f.FileMatches(tt.file)
			if got != tt.want {
				t.Errorf("FileMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesFiletype_EdgeCases(t *testing.T) {
	f := NewFilter([]string{"flac"})

	// Empty pattern
	if f.matchesFiletype(slskd.SearchFile{Filename: "test.flac"}, "flac", "") {
		t.Error("expected empty pattern to not match")
	}

	// Invalid quality format
	if f.matchesFiletype(slskd.SearchFile{Filename: "test.flac"}, "flac", "flac invalid") {
		t.Error("expected invalid quality format to not match")
	}

	// Invalid bitrate for mp3
	if f.matchesFiletype(slskd.SearchFile{Filename: "test.mp3"}, "mp3", "mp3 abc") {
		t.Error("expected invalid bitrate to not match")
	}

	// Invalid depth/rate for flac
	if f.matchesFiletype(slskd.SearchFile{Filename: "test.flac"}, "flac", "flac abc/def") {
		t.Error("expected invalid depth/rate to not match")
	}
}

func TestParseFloatRate(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"192", 192000, false},
		{"96", 96000, false},
		{"44.1", 44100, false},
		{"48", 48000, false},
		{"invalid", 0, true},
		{"12.3", 12300, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFloatRate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloatRate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseFloatRate(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFilterFiles(t *testing.T) {
	tests := []struct {
		name             string
		allowedFiletypes []string
		files            []slskd.SearchFile
		wantCount        int
	}{
		{
			name:             "no filter returns all",
			allowedFiletypes: []string{},
			files: []slskd.SearchFile{
				{Filename: "test1.flac"},
				{Filename: "test2.mp3"},
				{Filename: "test3.wav"},
			},
			wantCount: 3,
		},
		{
			name:             "flac filter returns only flac",
			allowedFiletypes: []string{"flac"},
			files: []slskd.SearchFile{
				{Filename: "test1.flac"},
				{Filename: "test2.mp3"},
				{Filename: "test3.flac"},
			},
			wantCount: 2,
		},
		{
			name:             "empty file list",
			allowedFiletypes: []string{"flac"},
			files:            []slskd.SearchFile{},
			wantCount:        0,
		},
		{
			name:             "quality filter",
			allowedFiletypes: []string{"flac 24/192"},
			files: []slskd.SearchFile{
				{Filename: "test1.flac", BitDepth: intPtr(24), SampleRate: intPtr(192000)},
				{Filename: "test2.flac", BitDepth: intPtr(16), SampleRate: intPtr(44100)},
				{Filename: "test3.flac", BitDepth: intPtr(24), SampleRate: intPtr(192000)},
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.allowedFiletypes)
			got := f.FilterFiles(tt.files)
			if len(got) != tt.wantCount {
				t.Errorf("FilterFiles() returned %d files, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestFilterFilesDebug(t *testing.T) {
	tests := []struct {
		name             string
		allowedFiletypes []string
		files            []slskd.SearchFile
		wantFiltered     int
		wantInfo         int
	}{
		{
			name:             "no filter returns all with no info",
			allowedFiletypes: []string{},
			files: []slskd.SearchFile{
				{Filename: "test1.flac"},
				{Filename: "test2.mp3"},
			},
			wantFiltered: 2,
			wantInfo:     0,
		},
		{
			name:             "with filter returns info for all",
			allowedFiletypes: []string{"flac"},
			files: []slskd.SearchFile{
				{Filename: "test1.flac"},
				{Filename: "test2.mp3"},
			},
			wantFiltered: 1,
			wantInfo:     2,
		},
		{
			name:             "info includes metadata",
			allowedFiletypes: []string{"flac 24/192"},
			files: []slskd.SearchFile{
				{
					Filename:   "dir/test1.flac",
					BitDepth:   intPtr(24),
					SampleRate: intPtr(192000),
					BitRate:    intPtr(1200),
				},
			},
			wantFiltered: 1,
			wantInfo:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.allowedFiletypes)
			filtered, info := f.FilterFilesDebug(tt.files)

			if len(filtered) != tt.wantFiltered {
				t.Errorf("FilterFilesDebug() returned %d filtered files, want %d", len(filtered), tt.wantFiltered)
			}

			if len(info) != tt.wantInfo {
				t.Errorf("FilterFilesDebug() returned %d info items, want %d", len(info), tt.wantInfo)
			}

			// Verify info structure
			for _, i := range info {
				if i.Filename == "" {
					t.Error("FileFilterInfo has empty Filename")
				}
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
