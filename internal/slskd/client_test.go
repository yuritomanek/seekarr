package slskd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Error("missing or invalid API key")
		}

		if r.URL.Path != "/api/v0/searches" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Decode request
		var req SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.SearchText != "Test Artist Album" {
			t.Errorf("expected search text 'Test Artist Album', got %q", req.SearchText)
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResponse{
			ID:         "search-123",
			State:      "InProgress",
			SearchText: req.SearchText,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "/")

	resp, err := client.Search(context.Background(), SearchRequest{
		SearchText:             "Test Artist Album",
		SearchTimeout:          5000,
		FilterResponses:        true,
		MaximumPeerQueueLength: 50,
		MinimumPeerUploadSpeed: 0,
	})

	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if resp.ID != "search-123" {
		t.Errorf("expected ID 'search-123', got %q", resp.ID)
	}

	if resp.State != "InProgress" {
		t.Errorf("expected state 'InProgress', got %q", resp.State)
	}
}

func TestGetSearchResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/searches/search-123/responses" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		bitRate := 320
		sampleRate := 44100
		bitDepth := 16

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SearchResult{
			{
				Username: "user1",
				Files: []SearchFile{
					{
						Filename:   "Artist\\Album\\01 Track.flac",
						Size:       35840000,
						BitRate:    &bitRate,
						SampleRate: &sampleRate,
						BitDepth:   &bitDepth,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "/")

	results, err := client.GetSearchResults(context.Background(), "search-123")
	if err != nil {
		t.Fatalf("GetSearchResults() error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Username != "user1" {
		t.Errorf("expected username 'user1', got %q", results[0].Username)
	}

	if len(results[0].Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(results[0].Files))
	}

	file := results[0].Files[0]
	if *file.BitRate != 320 {
		t.Errorf("expected bitrate 320, got %d", *file.BitRate)
	}
}

func TestGetDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/users/user1/directory" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		bitRate := 320

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Directory{
			Name: "Artist\\Album",
			Files: []DirectoryFile{
				{
					Filename: "01 Track.flac",
					Size:     35840000,
					BitRate:  &bitRate,
				},
				{
					Filename: "02 Track.flac",
					Size:     38000000,
					BitRate:  &bitRate,
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "/")

	dir, err := client.GetDirectory(context.Background(), "user1", "Artist\\Album")
	if err != nil {
		t.Fatalf("GetDirectory() error: %v", err)
	}

	if dir.Name != "Artist\\Album" {
		t.Errorf("expected name 'Artist\\Album', got %q", dir.Name)
	}

	if len(dir.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(dir.Files))
	}

	if dir.Files[0].Filename != "01 Track.flac" {
		t.Errorf("expected filename '01 Track.flac', got %q", dir.Files[0].Filename)
	}
}

func TestEnqueueDownloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/transfers/downloads" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req EnqueueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Username != "user1" {
			t.Errorf("expected username 'user1', got %q", req.Username)
		}

		if len(req.Files) != 2 {
			t.Errorf("expected 2 files, got %d", len(req.Files))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "/")

	err := client.EnqueueDownloads(context.Background(), "user1", []string{
		"Artist\\Album\\01 Track.flac",
		"Artist\\Album\\02 Track.flac",
	})

	if err != nil {
		t.Fatalf("EnqueueDownloads() error: %v", err)
	}
}

func TestGetDownloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/transfers/downloads" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadsResponse{
			{
				Username: "user1",
				Directories: []DirectoryDownloads{
					{
						Directory: "Artist\\Album",
						Files: []DownloadFile{
							{
								ID:               "download-1",
								Filename:         "01 Track.flac",
								State:            "InProgress, Downloading",
								BytesTransferred: 1000000,
								Size:             35840000,
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "/")

	downloads, err := client.GetDownloads(context.Background())
	if err != nil {
		t.Fatalf("GetDownloads() error: %v", err)
	}

	if len(downloads) != 1 {
		t.Errorf("expected 1 user, got %d", len(downloads))
	}

	if downloads[0].Username != "user1" {
		t.Errorf("expected username 'user1', got %q", downloads[0].Username)
	}

	if len(downloads[0].Directories) != 1 {
		t.Errorf("expected 1 directory, got %d", len(downloads[0].Directories))
	}

	file := downloads[0].Directories[0].Files[0]
	if file.State != "InProgress, Downloading" {
		t.Errorf("expected state 'InProgress, Downloading', got %q", file.State)
	}
}

func TestDownloadFileStates(t *testing.T) {
	tests := []struct {
		name           string
		state          string
		expectError    bool
		expectComplete bool
		expectProgress bool
	}{
		{"downloading", "InProgress, Downloading", false, false, true},
		{"queued", "Queued, None", false, false, true},
		{"succeeded", "Completed, Succeeded", false, true, false},
		{"cancelled", "Completed, Cancelled", true, true, false},
		{"timed out", "Completed, TimedOut", true, true, false},
		{"errored", "Completed, Errored", true, true, false},
		{"rejected", "Completed, Rejected", true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &DownloadFile{State: tt.state}

			if file.IsErrored() != tt.expectError {
				t.Errorf("IsErrored() = %v, want %v", file.IsErrored(), tt.expectError)
			}

			if file.IsCompleted() != tt.expectComplete {
				t.Errorf("IsCompleted() = %v, want %v", file.IsCompleted(), tt.expectComplete)
			}

			if file.IsInProgress() != tt.expectProgress {
				t.Errorf("IsInProgress() = %v, want %v", file.IsInProgress(), tt.expectProgress)
			}
		})
	}
}

func TestClientWithURLBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect /slskd/api/v0/application/version
		if r.URL.Path != "/slskd/api/v0/application/version" {
			t.Errorf("expected path with url_base, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VersionResponse{Version: "0.22.3"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "/slskd")

	version, err := client.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion() error: %v", err)
	}

	if version != "0.22.3" {
		t.Errorf("expected version '0.22.3', got %q", version)
	}
}
