package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yuritomanek/seekarr/internal/config"
	"github.com/yuritomanek/seekarr/internal/lidarr"
	"github.com/yuritomanek/seekarr/internal/processor"
	"github.com/yuritomanek/seekarr/internal/slskd"
	"github.com/yuritomanek/seekarr/internal/state"
)

const version = "1.0.0"

func main() {
	// Exit with proper status code
	os.Exit(run())
}

func run() int {
	// Set up structured logging
	logger := setupLogger()

	logger.Info("starting seekarr", "version", version)

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		return 1
	}

	logger.Info("configuration loaded",
		"lidarr_url", cfg.Lidarr.HostURL,
		"slskd_url", cfg.Slskd.HostURL,
		"search_type", cfg.Search.SearchType)

	// Acquire lock file to prevent concurrent runs
	lockPath := filepath.Join(cfg.Lidarr.DownloadDir, ".seekarr.lock")
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

// setupLogger creates a structured logger with appropriate output format
func setupLogger() *slog.Logger {
	// Use JSON output if LOG_FORMAT=json, otherwise text
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	// Check for debug mode
	if os.Getenv("DEBUG") == "true" {
		opts.Level = slog.LevelDebug
	}

	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// loadConfig loads configuration from file and environment
func loadConfig() (*config.Config, error) {
	// Look for config file in standard locations
	configPaths := []string{
		os.Getenv("SEEKARR_CONFIG"),
		"config.yaml",
		"config.yml",
		"/etc/seekarr/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "seekarr", "config.yaml"),
	}

	var configPath string
	for _, path := range configPaths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	if configPath == "" {
		return nil, fmt.Errorf("no config file found (searched: %v)", configPaths)
	}

	// Load and validate config
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config from %s: %w", configPath, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
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
