package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yuritomanek/seekarr/internal/config"
	"github.com/yuritomanek/seekarr/internal/lidarr"
	"github.com/yuritomanek/seekarr/internal/processor"
	"github.com/yuritomanek/seekarr/internal/slskd"
	"github.com/yuritomanek/seekarr/internal/state"
)

// Version information (set by goreleaser at build time)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Exit with proper status code
	os.Exit(run())
}

func run() int {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Show version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("seekarr %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		return 0
	}

	// Set up structured logging
	logger := setupLogger()

	logger.Info("starting seekarr", "version", version)

	// Load configuration
	cfg, err := loadConfig(logger)
	if err != nil {
		// loadConfig already logged the detailed error
		return 1
	}

	logger.Info("configuration loaded",
		"lidarr_url", cfg.Lidarr.HostURL,
		"slskd_url", cfg.Slskd.HostURL,
		"search_type", cfg.Search.SearchType)

	// Acquire lock file to prevent concurrent runs
	lockPath := filepath.Join(cfg.Slskd.DownloadDir, ".seekarr.lock")
	lockFile := state.NewLockFile(lockPath)

	if err := lockFile.Acquire(); err != nil {
		logger.Error("failed to acquire lock file", "error", err, "path", lockPath)
		logger.Error("is another instance of seekarr already running?")
		return 1
	}
	defer func() {
		if err := lockFile.Release(); err != nil {
			logger.Warn("failed to release lock file", "error", err)
		}
	}()

	logger.Info("lock file acquired", "path", lockPath)

	// Create API clients
	lidarrClient := lidarr.NewClient(
		cfg.Lidarr.HostURL,
		cfg.Lidarr.APIKey,
	)

	slskdClient := slskd.NewClient(
		cfg.Slskd.HostURL,
		cfg.Slskd.APIKey,
		cfg.Slskd.URLBase,
	)

	// Verify connectivity
	logger.Info("verifying connectivity to slskd")
	if err := verifySlskdConnection(slskdClient); err != nil {
		logger.Error("failed to connect to slskd", "error", err)
		return 1
	}

	// Create processor
	proc, err := processor.NewProcessor(cfg, lidarrClient, slskdClient, logger)
	if err != nil {
		logger.Error("failed to create processor", "error", err)
		return 1
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Run processor - either once or in daemon mode
	if cfg.Daemon.Enabled {
		logger.Info("starting daemon mode", "interval_minutes", cfg.Daemon.IntervalMinutes)
		return runDaemon(ctx, cancel, proc, sigChan, cfg, logger)
	}

	// Single run mode
	return runOnce(ctx, cancel, proc, sigChan, logger)
}

// runOnce executes a single processor run
func runOnce(ctx context.Context, cancel context.CancelFunc, proc *processor.Processor, sigChan chan os.Signal, logger *slog.Logger) int {
	// Run processor in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- proc.Run(ctx)
	}()

	// Wait for completion or signal
	select {
	case err := <-errChan:
		if err != nil {
			logger.Error("processor failed", "error", err)
			return 1
		}
		logger.Info("processor completed successfully")
		return 0

	case sig := <-sigChan:
		logger.Warn("received signal, initiating graceful shutdown", "signal", sig)
		cancel() // Cancel context to stop processor

		// Wait for processor to finish cleanup
		if err := <-errChan; err != nil && err != context.Canceled {
			logger.Error("processor failed during shutdown", "error", err)
			return 1
		}

		logger.Info("shutdown complete")
		return 0
	}
}

// runDaemon executes the processor in a loop with periodic intervals
func runDaemon(ctx context.Context, cancel context.CancelFunc, proc *processor.Processor, sigChan chan os.Signal, cfg *config.Config, logger *slog.Logger) int {
	ticker := time.NewTicker(time.Duration(cfg.Daemon.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// Track whether a processor run is currently active
	running := make(chan struct{}, 1)
	running <- struct{}{} // Initially not running (token available)

	// Helper function to run processor asynchronously
	runProcessor := func() {
		select {
		case <-running:
			// Acquired the token, start the run
			go func() {
				defer func() {
					running <- struct{}{} // Release token when done
				}()

				if err := proc.Run(ctx); err != nil && err != context.Canceled {
					logger.Error("processor failed", "error", err)
				} else if err == nil {
					logger.Info("processor completed successfully")
				}
			}()
		default:
			logger.Warn("skipping scheduled run - processor is still running from previous interval")
		}
	}

	// Run immediately on startup
	runProcessor()

	for {
		select {
		case <-ticker.C:
			// Only start a new run if we're not shutting down
			select {
			case <-ctx.Done():
				logger.Info("context cancelled, skipping scheduled run")
				return 0
			default:
				logger.Info("starting periodic processor run")
				runProcessor()
			}

		case sig := <-sigChan:
			logger.Warn("received signal, shutting down daemon", "signal", sig)
			cancel()
			// Give processor a moment to finish cleanup (but don't block indefinitely)
			time.Sleep(500 * time.Millisecond)
			logger.Info("shutdown complete")
			return 0

		case <-ctx.Done():
			logger.Info("context cancelled, shutting down daemon")
			return 0
		}
	}
}

// setupLogger creates a structured logger with appropriate output format
func setupLogger() *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	// Check for debug mode via DEBUG or LOG_LEVEL env vars
	if os.Getenv("DEBUG") == "true" || os.Getenv("LOG_LEVEL") == "DEBUG" {
		opts.Level = slog.LevelDebug
	}

	logFormat := os.Getenv("LOG_FORMAT")

	switch logFormat {
	case "json":
		// Full structured JSON output
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "structured":
		// Full structured text output with timestamps
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		// Clean output for CLI usage
		handler = newCleanHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// cleanHandler provides simplified logging output for CLI tools
type cleanHandler struct {
	opts slog.HandlerOptions
	w    io.Writer
}

func newCleanHandler(w io.Writer, opts *slog.HandlerOptions) *cleanHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &cleanHandler{
		opts: *opts,
		w:    w,
	}
}

func (h *cleanHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *cleanHandler) Handle(ctx context.Context, r slog.Record) error {
	var buf []byte

	// Format based on level
	switch r.Level {
	case slog.LevelError:
		buf = append(buf, "ERROR: "...)
	case slog.LevelWarn:
		buf = append(buf, "WARN: "...)
	case slog.LevelDebug:
		buf = append(buf, "DEBUG: "...)
		// INFO level: no prefix, just the message
	}

	// Add the main message
	buf = append(buf, r.Message...)

	// Add any attributes
	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
		return true
	})

	buf = append(buf, '\n')
	_, err := h.w.Write(buf)
	return err
}

func (h *cleanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, return same handler (attrs would need to be stored)
	return h
}

func (h *cleanHandler) WithGroup(name string) slog.Handler {
	// For simplicity, return same handler
	return h
}

// loadConfig loads configuration from file and environment
func loadConfig(logger *slog.Logger) (*config.Config, error) {
	// Look for config file in standard locations
	configPaths := []string{
		os.Getenv("SEEKARR_CONFIG"),
		"config.yaml",
		"config.yml",
		"/etc/seekarr/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "seekarr", "config.yaml"),
	}

	var configPath string
	// Build list of searched paths (excluding empty ones)
	var searchedPaths []string
	for _, path := range configPaths {
		if path == "" {
			continue
		}
		searchedPaths = append(searchedPaths, path)
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	if configPath == "" {
		// Log formatted error message with helpful suggestions
		logger.Error("configuration file not found")
		logger.Error("searched locations:")
		for _, path := range searchedPaths {
			logger.Error(fmt.Sprintf("  - %s", path))
		}
		logger.Error("")
		logger.Error("to get started:")
		logger.Error("  1. Copy config.example.yaml to config.yaml")
		logger.Error("  2. Edit config.yaml with your API keys and paths")
		logger.Error("  3. Or set SEEKARR_CONFIG environment variable to your config file path")
		return nil, fmt.Errorf("configuration file not found")
	}

	// Load and validate config
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load configuration file", "path", configPath, "error", err)
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		logger.Error("configuration validation failed", "error", err)
		logger.Error("please check your config.yaml file for errors")
		return nil, err
	}

	return cfg, nil
}

// verifySlskdConnection checks that we can connect to slskd
func verifySlskdConnection(client slskd.Client) error {
	ctx := context.Background()
	version, err := client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("get slskd version: %w", err)
	}

	slog.Info("connected to slskd", "version", version)
	return nil
}
