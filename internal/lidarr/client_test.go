package lidarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWanted(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Error("missing or invalid API key")
		}

		// Check path
		if r.URL.Path != "/api/v1/wanted/missing" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Check query params
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WantedResponse{
			Page:         1,
			PageSize:     10,
			TotalRecords: 1,
			Records: []Album{
				{
					ID:    123,
					Title: "Test Album",
					Artist: Artist{
						ID:         456,
						ArtistName: "Test Artist",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	resp, err := client.GetWanted(context.Background(), GetWantedOptions{
		Page:     1,
		PageSize: 10,
		Missing:  true,
	})

	if err != nil {
		t.Fatalf("GetWanted() error: %v", err)
	}

	if len(resp.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(resp.Records))
	}

	if resp.Records[0].ID != 123 {
		t.Errorf("expected album ID 123, got %d", resp.Records[0].ID)
	}

	if resp.Records[0].Title != "Test Album" {
		t.Errorf("expected title 'Test Album', got %q", resp.Records[0].Title)
	}
}

func TestGetAlbum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/album/123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Album{
			ID:    123,
			Title: "Test Album",
			Artist: Artist{
				ID:         456,
				ArtistName: "Test Artist",
			},
			Releases: []Release{
				{
					ID:          789,
					AlbumID:     123,
					TrackCount:  10,
					MediumCount: 1,
					Country:     []string{"United States"},
					Format:      "CD",
					Status:      "Official",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	album, err := client.GetAlbum(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetAlbum() error: %v", err)
	}

	if album.ID != 123 {
		t.Errorf("expected ID 123, got %d", album.ID)
	}

	if len(album.Releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(album.Releases))
	}

	if album.Releases[0].Format != "CD" {
		t.Errorf("expected format 'CD', got %q", album.Releases[0].Format)
	}
}

func TestGetTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/track" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		albumID := r.URL.Query().Get("albumId")
		if albumID != "123" {
			t.Errorf("expected albumId=123, got %s", albumID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Track{
			{
				ID:                  1,
				Title:               "Track 1",
				AlbumID:             123,
				MediumNumber:        1,
				AbsoluteTrackNumber: 1,
			},
			{
				ID:                  2,
				Title:               "Track 2",
				AlbumID:             123,
				MediumNumber:        1,
				AbsoluteTrackNumber: 2,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	tracks, err := client.GetTracks(context.Background(), 123, nil)
	if err != nil {
		t.Fatalf("GetTracks() error: %v", err)
	}

	if len(tracks) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(tracks))
	}

	if tracks[0].Title != "Track 1" {
		t.Errorf("expected title 'Track 1', got %q", tracks[0].Title)
	}
}

func TestPostCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/api/v1/command" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Decode request body
		var cmd Command
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			t.Fatalf("failed to decode command: %v", err)
		}

		if cmd.Name != "DownloadedAlbumsScan" {
			t.Errorf("expected command 'DownloadedAlbumsScan', got %q", cmd.Name)
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CommandResponse{
			ID:          999,
			Name:        cmd.Name,
			CommandName: cmd.Name,
			Status:      "queued",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	resp, err := client.PostCommand(context.Background(), Command{
		Name: "DownloadedAlbumsScan",
		Path: "/downloads/Artist",
	})

	if err != nil {
		t.Fatalf("PostCommand() error: %v", err)
	}

	if resp.ID != 999 {
		t.Errorf("expected command ID 999, got %d", resp.ID)
	}

	if resp.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", resp.Status)
	}
}

func TestClientErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Album not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	_, err := client.GetAlbum(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}

	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}
