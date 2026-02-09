package processor

import (
	"context"
	"log/slog"
	"testing"

	"github.com/yuritomanek/seekarr/internal/config"
	"github.com/yuritomanek/seekarr/internal/lidarr"
	"github.com/yuritomanek/seekarr/internal/slskd"
)

// mockLidarrClient is a minimal mock for testing
type mockLidarrClient struct{}

func (m *mockLidarrClient) GetWanted(ctx context.Context, opts lidarr.GetWantedOptions) (*lidarr.WantedResponse, error) {
	return &lidarr.WantedResponse{Records: []lidarr.Album{}}, nil
}

func (m *mockLidarrClient) GetAlbum(ctx context.Context, id int) (*lidarr.Album, error) {
	return &lidarr.Album{}, nil
}

func (m *mockLidarrClient) GetTracks(ctx context.Context, albumID int, releaseID *int) ([]lidarr.Track, error) {
	return []lidarr.Track{}, nil
}

func (m *mockLidarrClient) UpdateAlbum(ctx context.Context, album *lidarr.Album) (*lidarr.Album, error) {
	return album, nil
}

func (m *mockLidarrClient) GetQueue(ctx context.Context, page int, pageSize int) (*lidarr.QueueResponse, error) {
	return &lidarr.QueueResponse{Records: []lidarr.QueueItem{}}, nil
}

func (m *mockLidarrClient) PostCommand(ctx context.Context, cmd lidarr.Command) (*lidarr.CommandResponse, error) {
	return &lidarr.CommandResponse{ID: 1}, nil
}

func (m *mockLidarrClient) GetCommand(ctx context.Context, id int) (*lidarr.CommandResponse, error) {
	return &lidarr.CommandResponse{ID: id, Status: "completed"}, nil
}

// mockSlskdClient is a minimal mock for testing
type mockSlskdClient struct{}

func (m *mockSlskdClient) GetVersion(ctx context.Context) (string, error) {
	return "0.22.3", nil
}

func (m *mockSlskdClient) Search(ctx context.Context, req slskd.SearchRequest) (*slskd.SearchResponse, error) {
	return &slskd.SearchResponse{ID: "test-search"}, nil
}

func (m *mockSlskdClient) GetSearchState(ctx context.Context, searchID string) (*slskd.SearchResponse, error) {
	return &slskd.SearchResponse{ID: searchID, State: "Completed"}, nil
}

func (m *mockSlskdClient) GetSearchResults(ctx context.Context, searchID string) ([]slskd.SearchResult, error) {
	return []slskd.SearchResult{}, nil
}

func (m *mockSlskdClient) DeleteSearch(ctx context.Context, searchID string) error {
	return nil
}

func (m *mockSlskdClient) GetDirectory(ctx context.Context, username, directory string) (*slskd.Directory, error) {
	return &slskd.Directory{}, nil
}

func (m *mockSlskdClient) EnqueueDownloads(ctx context.Context, username string, files []string) error {
	return nil
}

func (m *mockSlskdClient) GetDownloads(ctx context.Context) (slskd.DownloadsResponse, error) {
	return slskd.DownloadsResponse{}, nil
}

func (m *mockSlskdClient) GetUserDownloads(ctx context.Context, username string) (*slskd.UserDownloads, error) {
	return &slskd.UserDownloads{}, nil
}

func (m *mockSlskdClient) CancelDownload(ctx context.Context, username, downloadID string) error {
	return nil
}

func (m *mockSlskdClient) RemoveCompletedDownloads(ctx context.Context) error {
	return nil
}

func TestNewProcessor(t *testing.T) {
	// Create temporary directory for state files
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Lidarr: config.LidarrConfig{
			DownloadDir: tmpDir,
		},
		Slskd: config.SlskdConfig{
			DownloadDir: tmpDir,
		},
		Search: config.SearchSettings{
			SearchType:                "first_page",
			MinimumFilenameMatchRatio: 0.8,
			MaxSearchFailures:         3,
		},
	}

	lidarrClient := &mockLidarrClient{}
	slskdClient := &mockSlskdClient{}

	processor, err := NewProcessor(cfg, lidarrClient, slskdClient, slog.Default())
	if err != nil {
		t.Fatalf("NewProcessor() error: %v", err)
	}

	if processor == nil {
		t.Fatal("NewProcessor() returned nil processor")
	}

	if processor.cfg != cfg {
		t.Error("processor config not set correctly")
	}

	if processor.lidarr == nil {
		t.Error("processor lidarr client not initialized")
	}

	if processor.slskd == nil {
		t.Error("processor slskd client not initialized")
	}

	if processor.matcher == nil {
		t.Error("processor matcher not initialized")
	}

	if processor.organizer == nil {
		t.Error("processor organizer not initialized")
	}

	if processor.denylist == nil {
		t.Error("processor denylist not initialized")
	}

	if processor.pageTrack == nil {
		t.Error("processor page tracker not initialized")
	}
}

// Note: More comprehensive tests would require mocking all the interactions
// between components. For now, we verify the processor can be constructed correctly.
