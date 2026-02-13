# Seekarr

Seekarr bridges [Lidarr](https://lidarr.audio/) and [slskd](https://github.com/slskd/slskd) to automatically search for and download missing music from Soulseek.

## Features

- Fetches wanted albums from Lidarr and searches for them on Soulseek
- Fuzzy matching algorithm with configurable thresholds to find the best releases
- Filter by file format (FLAC, MP3), bitrate, and other quality metrics
- Automatically organizes downloaded music into Lidarr's expected structure
- **Daemon mode**: Run continuously with configurable intervals (no cron needed!)
- **Auto-cleanup**: Automatically deletes imported files and cleans slskd UI
- Tracks search attempts and denylists albums after repeated failures
- Handles signals properly to finish current operations before exiting
- Monitors download progress and detects stalled downloads
- YAML-based configuration with environment variable support

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap yuritomanek/seekarr
brew install seekarr
```

The example configuration will be installed to `/opt/homebrew/etc/seekarr/` (Apple Silicon) or `/usr/local/etc/seekarr/` (Intel).

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/yuritomanek/seekarr/releases).

Extract and install:

```bash
tar -xzf seekarr-*.tar.gz
sudo mv seekarr /usr/local/bin/
chmod +x /usr/local/bin/seekarr
```

### Build from Source

Requires Go 1.25 or later and make.

```bash
git clone https://github.com/yuritomanek/seekarr.git
cd seekarr
make build
```

## Configuration

Copy the example config and edit it with your settings:

```bash
cp config.example.yaml ~/.config/seekarr/config.yaml
```

See [config.example.yaml](config.example.yaml) for all available options.

### Required Configuration

```yaml
lidarr:
  api_key: ${LIDARR_API_KEY}
  host_url: http://localhost:8686
  download_dir: /downloads

slskd:
  api_key: ${SLSKD_API_KEY}
  host_url: http://localhost:5030
  download_dir: /downloads
```

### Environment Variables

You can use environment variables in your configuration file:

```yaml
lidarr:
  api_key: ${LIDARR_API_KEY}
```

Or set them before running:

```bash
export LIDARR_API_KEY="your-api-key-here"
export SLSKD_API_KEY="your-api-key-here"
seekarr
```

### Configuration Locations

Seekarr searches for configuration in this order:

1. Path specified with `--config` flag
2. `./config.yaml` (current directory)
3. `~/.config/seekarr/config.yaml`
4. `/etc/seekarr/config.yaml`

## Usage

### Single Run Mode (Default)

Run once to process wanted albums:

```bash
seekarr
```

### Daemon Mode

Run continuously, checking for new albums at regular intervals:

```yaml
# config.yaml
daemon:
  enabled: true
  interval_minutes: 15  # Check every 15 minutes
  delete_after_import: true  # Clean up after successful imports
  cleanup_delay_seconds: 10  # Safety delay before cleanup
```

```bash
seekarr  # Runs continuously until stopped with Ctrl+C
```

**Benefits of daemon mode:**
- No need for cron jobs
- Single long-running process
- Automatic cleanup saves disk space
- Keeps slskd downloads page clean

### Logging

Control log output format with the `LOG_FORMAT` environment variable:

```bash
# Clean output (default)
seekarr

# Structured output with timestamps
LOG_FORMAT=structured seekarr

# JSON output for log aggregation
LOG_FORMAT=json seekarr
```

### Scheduling

**Option 1: Daemon Mode (Recommended)**

Enable daemon mode in your config for continuous operation:

```yaml
daemon:
  enabled: true
  interval_minutes: 15
```

**Option 2: Cron Jobs**

Alternatively, run periodically with cron (daemon mode disabled):

```cron
# Run every 30 minutes
*/30 * * * * /usr/local/bin/seekarr
```

## How It Works

1. Queries Lidarr for missing or cutoff-unmet albums
2. Searches slskd for each album (artist + album name, optionally individual tracks)
3. Applies fuzzy matching and quality filters to find the best releases
4. Initiates downloads through slskd
5. Tracks download progress and detects stalled transfers
6. Moves and renames files to match Lidarr's expected structure
7. Triggers Lidarr to import the organized files
8. **(Optional)** Waits for Lidarr to finish copying files (configurable delay)
9. **(Optional)** Deletes imported files and cleans up slskd downloads page

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests verbosely
make test-verbose
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build for specific platform
make build-darwin-amd64
make build-darwin-arm64
make build-linux-amd64
make build-linux-arm64
```

### Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet

# Run linter (requires golangci-lint)
make lint

# Run all checks
make check
```

### Project Structure

```
seekarr/
├── cmd/seekarr/          # Main entry point
├── internal/
│   ├── config/           # Configuration loading and validation
│   ├── lidarr/           # Lidarr API client
│   ├── slskd/            # slskd API client
│   ├── matcher/          # Fuzzy matching and filtering logic
│   ├── organizer/        # File organization and renaming
│   ├── processor/        # Core workflow orchestration
│   └── state/            # State management (denylist, page tracking, locks)
├── config.example.yaml   # Example configuration
└── Makefile              # Build automation
```

## Configuration Options

### Search Settings

- `search_timeout`: How long to wait for search results (milliseconds)
- `minimum_filename_match_ratio`: Minimum fuzzy match score (0.0 to 1.0)
- `search_type`: Search strategy (`first_page`, `incrementing_page`, `all`)
- `number_of_albums_to_grab`: How many albums to process per run
- `enable_search_denylist`: Automatically denylist albums after repeated failures
- `max_search_failures`: Number of failures before denylisting
- `sort_key`: How to sort wanted albums (e.g., `albums.title`, `albums.releaseDate`, `id`). Leave empty for Lidarr's default order
- `sort_dir`: Sort direction (`ascending`, `descending`). Only used if sort_key is set

### Release Filtering

- `accepted_countries`: Only accept releases from these countries
- `accepted_formats`: Allowed release formats (CD, Digital Media, Vinyl)
- `allow_multi_disc`: Whether to accept multi-disc releases

### Quality Filtering

- `allowed_filetypes`: Preferred audio formats in priority order (e.g., `flac 24/192`, `flac`, `mp3 320`)
- `minimum_peer_upload_speed`: Minimum upload speed in KB/s
- `maximum_peer_queue`: Maximum allowed queue position

### Timing

- `search_wait_seconds`: Delay between searches
- `download_poll_seconds`: How often to check download progress
- `import_poll_seconds`: How often to check import status

### Daemon Mode

- `enabled`: Run continuously instead of exiting after one run
- `interval_minutes`: How often to check for new wanted albums (default: 15)
- `delete_after_import`: Automatically delete organized folders after successful Lidarr import
- `cleanup_delay_seconds`: Safety delay after import completion before cleanup (default: 10)

**Note:** Only successfully imported albums are deleted. Failed imports are preserved for debugging.

## Contributing

Contributions are welcome. Fork the repo, make your changes, and open a pull request. Run `make check` before submitting to ensure tests pass and code is formatted.

## License

[MIT License](LICENSE)

## Credits

Seekarr is a Go rewrite and enhancement of [Soularr](https://github.com/mrusse/soularr) by mrusse.

## Support

Report bugs and request features on [GitHub Issues](https://github.com/yuritomanek/seekarr/issues).
