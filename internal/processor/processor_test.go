package processor

import (
	"context"
	"log/slog"
	"os"
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

func (m *mockSlskdClient) EnqueueDownloads(ctx context.Context, username string, files []slskd.EnqueueFile) error {
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

// mockLidarrClientWithCommands allows testing different command statuses
type mockLidarrClientWithCommands struct {
	mockLidarrClient
	commands map[int]*lidarr.CommandResponse
}

func (m *mockLidarrClientWithCommands) GetCommand(ctx context.Context, id int) (*lidarr.CommandResponse, error) {
	if cmd, ok := m.commands[id]; ok {
		return cmd, nil
	}
	return &lidarr.CommandResponse{ID: id, Status: "completed", Message: "Success"}, nil
}

func (m *mockLidarrClientWithCommands) PostCommand(ctx context.Context, cmd lidarr.Command) (*lidarr.CommandResponse, error) {
	// Generate ID based on path to make testing deterministic
	id := len(m.commands) + 1
	return &lidarr.CommandResponse{ID: id}, nil
}

// mockSlskdClientWithTracking tracks RemoveCompletedDownloads calls
type mockSlskdClientWithTracking struct {
	mockSlskdClient
	removeCalled bool
}

func (m *mockSlskdClientWithTracking) RemoveCompletedDownloads(ctx context.Context) error {
	m.removeCalled = true
	return nil
}

func TestPollImportCompletion(t *testing.T) {
	tests := []struct {
		name            string
		commands        map[int]*lidarr.CommandResponse
		commandToFolder map[int]string
		wantSuccessful  []string
	}{
		{
			name: "all successful",
			commands: map[int]*lidarr.CommandResponse{
				1: {ID: 1, Status: "completed", Message: "Importing 5 tracks"},
				2: {ID: 2, Status: "completed", Message: "Importing 3 tracks"},
			},
			commandToFolder: map[int]string{
				1: "Artist One",
				2: "Artist Two",
			},
			wantSuccessful: []string{"Artist One", "Artist Two"},
		},
		{
			name: "one failed",
			commands: map[int]*lidarr.CommandResponse{
				1: {ID: 1, Status: "completed", Message: "Importing 5 tracks"},
				2: {ID: 2, Status: "completed", Message: "Failed to import"},
			},
			commandToFolder: map[int]string{
				1: "Artist One",
				2: "Artist Two",
			},
			wantSuccessful: []string{"Artist One"},
		},
		{
			name: "all failed",
			commands: map[int]*lidarr.CommandResponse{
				1: {ID: 1, Status: "failed", Message: "Error"},
				2: {ID: 2, Status: "completed", Message: "Failed to import"},
			},
			commandToFolder: map[int]string{
				1: "Artist One",
				2: "Artist Two",
			},
			wantSuccessful: []string{},
		},
		{
			name:            "empty",
			commands:        map[int]*lidarr.CommandResponse{},
			commandToFolder: map[int]string{},
			wantSuccessful:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			cfg := &config.Config{
				Lidarr: config.LidarrConfig{DownloadDir: tmpDir},
				Slskd:  config.SlskdConfig{DownloadDir: tmpDir},
				Timing: config.TimingSettings{ImportPollSeconds: 0}, // No delay in tests
				Search: config.SearchSettings{
					SearchType:                "first_page",
					MinimumFilenameMatchRatio: 0.8,
					MaxSearchFailures:         3,
				},
			}

			lidarrClient := &mockLidarrClientWithCommands{commands: tt.commands}
			slskdClient := &mockSlskdClient{}

			processor, err := NewProcessor(cfg, lidarrClient, slskdClient, slog.Default())
			if err != nil {
				t.Fatalf("NewProcessor() error: %v", err)
			}

			ctx := context.Background()
			successful := processor.pollImportCompletion(ctx, tt.commandToFolder)

			if len(successful) != len(tt.wantSuccessful) {
				t.Errorf("got %d successful folders, want %d", len(successful), len(tt.wantSuccessful))
			}

			// Check each successful folder
			successMap := make(map[string]bool)
			for _, s := range successful {
				successMap[s] = true
			}
			for _, want := range tt.wantSuccessful {
				if !successMap[want] {
					t.Errorf("expected folder %q not in successful list", want)
				}
			}
		})
	}
}

func TestCleanupImportedFolders(t *testing.T) {
	tests := []struct {
		name                string
		folders             []string
		createFolders       bool
		cleanupDelaySeconds int
		wantRemoveCalled    bool
	}{
		{
			name:                "cleanup with folders",
			folders:             []string{"Artist One", "Artist Two"},
			createFolders:       true,
			cleanupDelaySeconds: 0,
			wantRemoveCalled:    true,
		},
		{
			name:                "cleanup with delay",
			folders:             []string{"Artist One"},
			createFolders:       true,
			cleanupDelaySeconds: 1,
			wantRemoveCalled:    true,
		},
		{
			name:                "no folders",
			folders:             []string{},
			createFolders:       false,
			cleanupDelaySeconds: 0,
			wantRemoveCalled:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create test folders
			if tt.createFolders {
				for _, folder := range tt.folders {
					folderPath := tmpDir + "/" + folder
					if err := os.MkdirAll(folderPath, 0755); err != nil {
						t.Fatalf("failed to create test folder: %v", err)
					}
					// Create a test file in the folder
					testFile := folderPath + "/test.txt"
					if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
						t.Fatalf("failed to create test file: %v", err)
					}
				}
			}

			cfg := &config.Config{
				Lidarr: config.LidarrConfig{DownloadDir: tmpDir},
				Slskd:  config.SlskdConfig{DownloadDir: tmpDir},
				Daemon: config.DaemonSettings{
					CleanupDelaySeconds: tt.cleanupDelaySeconds,
				},
				Search: config.SearchSettings{
					SearchType:                "first_page",
					MinimumFilenameMatchRatio: 0.8,
					MaxSearchFailures:         3,
				},
			}

			lidarrClient := &mockLidarrClient{}
			slskdClient := &mockSlskdClientWithTracking{}

			processor, err := NewProcessor(cfg, lidarrClient, slskdClient, slog.Default())
			if err != nil {
				t.Fatalf("NewProcessor() error: %v", err)
			}

			ctx := context.Background()
			processor.cleanupImportedFolders(ctx, tt.folders)

			// Verify folders were deleted
			if tt.createFolders {
				for _, folder := range tt.folders {
					folderPath := tmpDir + "/" + folder
					if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
						t.Errorf("folder %q was not deleted", folder)
					}
				}
			}

			// Verify RemoveCompletedDownloads was called
			if slskdClient.removeCalled != tt.wantRemoveCalled {
				t.Errorf("RemoveCompletedDownloads called = %v, want %v",
					slskdClient.removeCalled, tt.wantRemoveCalled)
			}
		})
	}
}
