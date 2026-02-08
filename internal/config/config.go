package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Lidarr   LidarrConfig    `yaml:"lidarr"`
	Slskd    SlskdConfig     `yaml:"slskd"`
	Release  ReleaseSettings `yaml:"release"`
	Search   SearchSettings  `yaml:"search"`
	Download DownloadSettings `yaml:"download"`
	Timing   TimingSettings  `yaml:"timing"`
	Logging  LoggingConfig   `yaml:"logging"`
}

type LidarrConfig struct {
	APIKey      string `yaml:"api_key"`
	HostURL     string `yaml:"host_url"`
	DownloadDir string `yaml:"download_dir"`
	DisableSync bool   `yaml:"disable_sync"`
}

type SlskdConfig struct {
	APIKey         string `yaml:"api_key"`
	HostURL        string `yaml:"host_url"`
	URLBase        string `yaml:"url_base"`
	DownloadDir    string `yaml:"download_dir"`
	DeleteSearches bool   `yaml:"delete_searches"`
	StalledTimeout int    `yaml:"stalled_timeout"` // seconds
}

type ReleaseSettings struct {
	UseMostCommonTrackNum bool     `yaml:"use_most_common_tracknum"`
	AllowMultiDisc        bool     `yaml:"allow_multi_disc"`
	AcceptedCountries     []string `yaml:"accepted_countries"`
	SkipRegionCheck       bool     `yaml:"skip_region_check"`
	AcceptedFormats       []string `yaml:"accepted_formats"`
}

type SearchSettings struct {
	SearchTimeout             int      `yaml:"search_timeout"`
	MaximumPeerQueue          int      `yaml:"maximum_peer_queue"`
	MinimumPeerUploadSpeed    int      `yaml:"minimum_peer_upload_speed"`
	MinimumFilenameMatchRatio float64  `yaml:"minimum_filename_match_ratio"`
	AllowedFiletypes          []string `yaml:"allowed_filetypes"`
	IgnoredUsers              []string `yaml:"ignored_users"`
	SearchForTracks           bool     `yaml:"search_for_tracks"`
	AlbumPrependArtist        bool     `yaml:"album_prepend_artist"`
	TrackPrependArtist        bool     `yaml:"track_prepend_artist"`
	SearchType                string   `yaml:"search_type"` // first_page, incrementing_page, all
	NumberOfAlbumsToGrab      int      `yaml:"number_of_albums_to_grab"`
	RemoveWantedOnFailure     bool     `yaml:"remove_wanted_on_failure"`
	TitleBlacklist            []string `yaml:"title_blacklist"`
	SearchSource              string   `yaml:"search_source"` // missing, cutoff_unmet, all
	EnableSearchDenylist      bool     `yaml:"enable_search_denylist"`
	MaxSearchFailures         int      `yaml:"max_search_failures"`
}

type DownloadSettings struct {
	DownloadFiltering     bool     `yaml:"download_filtering"`
	UseExtensionWhitelist bool     `yaml:"use_extension_whitelist"`
	ExtensionsWhitelist   []string `yaml:"extensions_whitelist"`
}

type TimingSettings struct {
	SearchWaitSeconds     int `yaml:"search_wait_seconds"`
	DownloadPollSeconds   int `yaml:"download_poll_seconds"`
	ImportPollSeconds     int `yaml:"import_poll_seconds"`
	StallCheckIntervalSec int `yaml:"stall_check_interval_seconds"`
}

type LoggingConfig struct {
	Level   string `yaml:"level"`
	Format  string `yaml:"format"`
	Datefmt string `yaml:"datefmt"`
}

// Load reads configuration from YAML file with environment variable expansion
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expanded := expandEnvVars(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set defaults for optional fields
	config.setDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &config, nil
}

// expandEnvVars expands environment variables in ${VAR} or $VAR format
func expandEnvVars(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match
		if match[1] == '{' {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}

// setDefaults applies default values for optional configuration fields
func (c *Config) setDefaults() {
	// Slskd defaults
	if c.Slskd.URLBase == "" {
		c.Slskd.URLBase = "/"
	}
	if c.Slskd.StalledTimeout == 0 {
		c.Slskd.StalledTimeout = 3600 // 1 hour
	}

	// Search defaults
	if c.Search.SearchTimeout == 0 {
		c.Search.SearchTimeout = 5000
	}
	if c.Search.MaximumPeerQueue == 0 {
		c.Search.MaximumPeerQueue = 50
	}
	if c.Search.MinimumFilenameMatchRatio == 0 {
		c.Search.MinimumFilenameMatchRatio = 0.8
	}
	if c.Search.SearchType == "" {
		c.Search.SearchType = "incrementing_page"
	}
	if c.Search.NumberOfAlbumsToGrab == 0 {
		c.Search.NumberOfAlbumsToGrab = 10
	}
	if c.Search.SearchSource == "" {
		c.Search.SearchSource = "missing"
	}
	if c.Search.MaxSearchFailures == 0 {
		c.Search.MaxSearchFailures = 3
	}

	// Timing defaults
	if c.Timing.SearchWaitSeconds == 0 {
		c.Timing.SearchWaitSeconds = 5
	}
	if c.Timing.DownloadPollSeconds == 0 {
		c.Timing.DownloadPollSeconds = 10
	}
	if c.Timing.ImportPollSeconds == 0 {
		c.Timing.ImportPollSeconds = 2
	}
	if c.Timing.StallCheckIntervalSec == 0 {
		c.Timing.StallCheckIntervalSec = 60 // Check for stalls every minute
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "INFO"
	}
	if c.Logging.Datefmt == "" {
		c.Logging.Datefmt = time.RFC3339
	}
}

// Validate checks required fields and value ranges
func (c *Config) Validate() error {
	// Required Lidarr fields
	if c.Lidarr.APIKey == "" {
		return fmt.Errorf("lidarr api_key is required")
	}
	if c.Lidarr.HostURL == "" {
		return fmt.Errorf("lidarr host_url is required")
	}
	if _, err := url.Parse(c.Lidarr.HostURL); err != nil {
		return fmt.Errorf("lidarr host_url must be valid URL: %w", err)
	}
	if c.Lidarr.DownloadDir == "" {
		return fmt.Errorf("lidarr download_dir is required")
	}

	// Required Slskd fields
	if c.Slskd.APIKey == "" {
		return fmt.Errorf("slskd api_key is required")
	}
	if c.Slskd.HostURL == "" {
		return fmt.Errorf("slskd host_url is required")
	}
	if _, err := url.Parse(c.Slskd.HostURL); err != nil {
		return fmt.Errorf("slskd host_url must be valid URL: %w", err)
	}
	if c.Slskd.DownloadDir == "" {
		return fmt.Errorf("slskd download_dir is required")
	}

	// Validate search settings
	if c.Search.MinimumFilenameMatchRatio < 0 || c.Search.MinimumFilenameMatchRatio > 1 {
		return fmt.Errorf("minimum_filename_match_ratio must be between 0 and 1, got %f", c.Search.MinimumFilenameMatchRatio)
	}
	if c.Search.SearchType != "first_page" && c.Search.SearchType != "incrementing_page" && c.Search.SearchType != "all" {
		return fmt.Errorf("search_type must be one of: first_page, incrementing_page, all (got %q)", c.Search.SearchType)
	}
	if c.Search.SearchSource != "missing" && c.Search.SearchSource != "cutoff_unmet" && c.Search.SearchSource != "all" {
		return fmt.Errorf("search_source must be one of: missing, cutoff_unmet, all (got %q)", c.Search.SearchSource)
	}
	if c.Search.NumberOfAlbumsToGrab < 1 {
		return fmt.Errorf("number_of_albums_to_grab must be at least 1, got %d", c.Search.NumberOfAlbumsToGrab)
	}

	// Validate timing settings
	if c.Timing.SearchWaitSeconds < 0 {
		return fmt.Errorf("search_wait_seconds must be non-negative, got %d", c.Timing.SearchWaitSeconds)
	}
	if c.Timing.DownloadPollSeconds < 1 {
		return fmt.Errorf("download_poll_seconds must be at least 1, got %d", c.Timing.DownloadPollSeconds)
	}
	if c.Timing.ImportPollSeconds < 1 {
		return fmt.Errorf("import_poll_seconds must be at least 1, got %d", c.Timing.ImportPollSeconds)
	}

	return nil
}

// Example generates an example configuration file content
func Example() string {
	return `# Seekarr Configuration

lidarr:
  api_key: ${LIDARR_API_KEY}
  host_url: http://lidarr:8686
  download_dir: /downloads
  disable_sync: false

slskd:
  api_key: ${SLSKD_API_KEY}
  host_url: http://slskd:5030
  url_base: /
  download_dir: /downloads
  delete_searches: false
  stalled_timeout: 3600

release:
  use_most_common_tracknum: true
  allow_multi_disc: true
  accepted_countries:
    - Europe
    - Japan
    - United States
    - United Kingdom
  skip_region_check: false
  accepted_formats:
    - CD
    - Digital Media
    - Vinyl

search:
  search_timeout: 5000
  maximum_peer_queue: 50
  minimum_peer_upload_speed: 0
  minimum_filename_match_ratio: 0.8
  allowed_filetypes:
    - flac 24/192
    - flac 16/44.1
    - flac
    - mp3 320
    - mp3
  ignored_users: []
  search_for_tracks: true
  album_prepend_artist: false
  track_prepend_artist: true
  search_type: incrementing_page  # first_page, incrementing_page, all
  number_of_albums_to_grab: 10
  remove_wanted_on_failure: false
  title_blacklist: []
  search_source: missing  # missing, cutoff_unmet, all
  enable_search_denylist: false
  max_search_failures: 3

download:
  download_filtering: true
  use_extension_whitelist: false
  extensions_whitelist:
    - lrc
    - nfo
    - txt

timing:
  search_wait_seconds: 5
  download_poll_seconds: 10
  import_poll_seconds: 2
  stall_check_interval_seconds: 60

logging:
  level: INFO
  format: ""
  datefmt: ""
`
}

