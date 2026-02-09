# Seekarr

Seekarr bridges [Lidarr](https://lidarr.audio/) and [slskd](https://github.com/slskd/slskd) to automatically search for and download missing music from Soulseek.

## Features

- Fetches wanted albums from Lidarr and searches for them on Soulseek
- Fuzzy matching algorithm with configurable thresholds to find the best releases
- Filter by file format (FLAC, MP3), bitrate, and other quality metrics
- Automatically organizes downloaded music into Lidarr's expected structure
- Tracks search attempts and denylists albums after repeated failures
- Handles signals properly to finish current operations before exiting
- Monitors download progress and detects stalled downloads
- YAML-based configuration with environment variable support

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/yuritomanek/seekarr/releases):

- **macOS (Intel)**: `seekarr-v0.1.0-darwin-amd64.tar.gz`
- **macOS (Apple Silicon)**: `seekarr-v0.1.0-darwin-arm64.tar.gz`
- **Linux (x86_64)**: `seekarr-v0.1.0-linux-amd64.tar.gz`
- **Linux (ARM64)**: `seekarr-v0.1.0-linux-arm64.tar.gz`

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

Run once to process wanted albums:

```bash
seekarr
```

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

Run periodically with cron:

```cron
# Run every 30 minutes
*/30 * * * * /usr/local/bin/seekarr
```

### Docker

```bash
docker run -v /path/to/config.yaml:/config/config.yaml \
           -v /path/to/downloads:/downloads \
           seekarr/seekarr:latest
```

## How It Works

1. Queries Lidarr for missing or cutoff-unmet albums
2. Searches slskd for each album (artist + album name, optionally individual tracks)
3. Applies fuzzy matching and quality filters to find the best releases
4. Initiates downloads through slskd
5. Tracks download progress and detects stalled transfers
6. Moves and renames files to match Lidarr's expected structure
7. Triggers Lidarr to import the organized files

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

## Contributing

Contributions are welcome. Fork the repo, make your changes, and open a pull request. Run `make check` before submitting to ensure tests pass and code is formatted.

## License

[MIT License](LICENSE)

## Credits

Seekarr is a Go rewrite and enhancement of [Soularr](https://github.com/mrusse/soularr) by mrusse.

## Support

Report bugs and request features on [GitHub Issues](https://github.com/yuritomanek/seekarr/issues).
