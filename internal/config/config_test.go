package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
lidarr:
  api_key: test-lidarr-key
  host_url: http://localhost:8686
  download_dir: /downloads

slskd:
  api_key: test-slskd-key
  host_url: http://localhost:5030
  download_dir: /downloads
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify required fields
	if cfg.Lidarr.APIKey != "test-lidarr-key" {
		t.Errorf("expected Lidarr APIKey 'test-lidarr-key', got %q", cfg.Lidarr.APIKey)
	}
	if cfg.Slskd.HostURL != "http://localhost:5030" {
		t.Errorf("expected Slskd HostURL 'http://localhost:5030', got %q", cfg.Slskd.HostURL)
	}

	// Verify defaults were applied
	if cfg.Slskd.URLBase != "/" {
		t.Errorf("expected default URLBase '/', got %q", cfg.Slskd.URLBase)
	}
	if cfg.Timing.SearchWaitSeconds != 5 {
		t.Errorf("expected default SearchWaitSeconds 5, got %d", cfg.Timing.SearchWaitSeconds)
	}
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	// Set environment variable
	os.Setenv("TEST_API_KEY", "secret-key-123")
	defer os.Unsetenv("TEST_API_KEY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
lidarr:
  api_key: ${TEST_API_KEY}
  host_url: http://localhost:8686
  download_dir: /downloads

slskd:
  api_key: $TEST_API_KEY
  host_url: http://localhost:5030
  download_dir: /downloads
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Lidarr.APIKey != "secret-key-123" {
		t.Errorf("expected expanded APIKey 'secret-key-123', got %q", cfg.Lidarr.APIKey)
	}
	if cfg.Slskd.APIKey != "secret-key-123" {
		t.Errorf("expected expanded APIKey 'secret-key-123', got %q", cfg.Slskd.APIKey)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError string
	}{
		{
			name: "missing lidarr api_key",
			config: Config{
				Lidarr: LidarrConfig{
					HostURL:     "http://localhost:8686",
					DownloadDir: "/downloads",
				},
				Slskd: SlskdConfig{
					APIKey:      "test",
					HostURL:     "http://localhost:5030",
					DownloadDir: "/downloads",
				},
			},
			expectError: "lidarr api_key is required",
		},
		{
			name: "invalid host url",
			config: Config{
				Lidarr: LidarrConfig{
					APIKey:      "test",
					HostURL:     "://invalid",
					DownloadDir: "/downloads",
				},
				Slskd: SlskdConfig{
					APIKey:      "test",
					HostURL:     "http://localhost:5030",
					DownloadDir: "/downloads",
				},
			},
			expectError: "lidarr host_url must be valid URL",
		},
		{
			name: "invalid match ratio",
			config: Config{
				Lidarr: LidarrConfig{
					APIKey:      "test",
					HostURL:     "http://localhost:8686",
					DownloadDir: "/downloads",
				},
				Slskd: SlskdConfig{
					APIKey:      "test",
					HostURL:     "http://localhost:5030",
					DownloadDir: "/downloads",
				},
				Search: SearchSettings{
					MinimumFilenameMatchRatio: 1.5,
				},
			},
			expectError: "minimum_filename_match_ratio must be between 0 and 1",
		},
		{
			name: "invalid search type",
			config: Config{
				Lidarr: LidarrConfig{
					APIKey:      "test",
					HostURL:     "http://localhost:8686",
					DownloadDir: "/downloads",
				},
				Slskd: SlskdConfig{
					APIKey:      "test",
					HostURL:     "http://localhost:5030",
					DownloadDir: "/downloads",
				},
				Search: SearchSettings{
					SearchType: "invalid_type",
				},
			},
			expectError: "search_type must be one of: first_page, incrementing_page, all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.setDefaults()
			err := tt.config.Validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			// Check that error message starts with expected prefix
			if tt.expectError != "" {
				errMsg := err.Error()
				hasPrefix := len(errMsg) >= len(tt.expectError) && errMsg[:len(tt.expectError)] == tt.expectError
				if !hasPrefix {
					t.Errorf("expected error starting with %q, got %q", tt.expectError, errMsg)
				}
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"URLBase", cfg.Slskd.URLBase, "/"},
		{"StalledTimeout", cfg.Slskd.StalledTimeout, 3600},
		{"SearchTimeout", cfg.Search.SearchTimeout, 5000},
		{"MinimumFilenameMatchRatio", cfg.Search.MinimumFilenameMatchRatio, 0.8},
		{"SearchType", cfg.Search.SearchType, "incrementing_page"},
		{"SearchWaitSeconds", cfg.Timing.SearchWaitSeconds, 5},
		{"DownloadPollSeconds", cfg.Timing.DownloadPollSeconds, 10},
		{"ImportPollSeconds", cfg.Timing.ImportPollSeconds, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("default %s = %v, expected %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}
