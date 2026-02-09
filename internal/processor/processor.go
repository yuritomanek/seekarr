package processor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuritomanek/seekarr/internal/config"
	"github.com/yuritomanek/seekarr/internal/lidarr"
	"github.com/yuritomanek/seekarr/internal/matcher"
	"github.com/yuritomanek/seekarr/internal/organizer"
	"github.com/yuritomanek/seekarr/internal/slskd"
	"github.com/yuritomanek/seekarr/internal/state"
)

// Processor orchestrates the main workflow: fetch, search, download, organize, import
type Processor struct {
	cfg       *config.Config
	lidarr    lidarr.Client // Interface, not pointer to interface
	slskd     slskd.Client  // Interface, not pointer to interface
	matcher   *matcher.Matcher
	organizer *organizer.Organizer
	denylist  *state.Denylist
	pageTrack *state.PageTracker
	logger    *slog.Logger
}

// DownloadedItem tracks a downloaded album for organization
type DownloadedItem struct {
	ArtistName  string
	AlbumName   string
	AlbumID     int
	FolderName  string
	Username    string
	Directory   string
	MediumCount int
	Tracks      []organizer.DownloadedTrack
}

// NewProcessor creates a new processor with all dependencies
func NewProcessor(
	cfg *config.Config,
	lidarrClient lidarr.Client,
	slskdClient slskd.Client,
	logger *slog.Logger,
) (*Processor, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Initialize components
	m := matcher.NewMatcher(cfg.Search.MinimumFilenameMatchRatio)
	org := organizer.NewOrganizer(cfg.Slskd.DownloadDir, logger)

	// Initialize state management
	denylistPath := filepath.Join(cfg.Slskd.DownloadDir, "search_denylist.json")
	denylist, err := state.NewDenylist(denylistPath)
	if err != nil {
		return nil, fmt.Errorf("initialize denylist: %w", err)
	}

	pageTrackPath := filepath.Join(cfg.Slskd.DownloadDir, ".current_page.txt")
	pageTrack, err := state.NewPageTracker(pageTrackPath, 1) // Start at page 1
	if err != nil {
		return nil, fmt.Errorf("initialize page tracker: %w", err)
	}

	return &Processor{
		cfg:       cfg,
		lidarr:    lidarrClient,
		slskd:     slskdClient,
		matcher:   m,
		organizer: org,
		denylist:  denylist,
		pageTrack: pageTrack,
		logger:    logger,
	}, nil
}

// Run executes the main processing workflow
func (p *Processor) Run(ctx context.Context) error {
	p.logger.Info("starting seekarr processor")

	// Phase 1: Fetch wanted albums from Lidarr
	albums, err := p.fetchWantedAlbums(ctx)
	if err != nil {
		return fmt.Errorf("fetch wanted albums: %w", err)
	}

	if len(albums) == 0 {
		p.logger.Info("no wanted albums found")
		return nil
	}

	p.logger.Info("found wanted albums", "count", len(albums))

	// Phase 2: Search and queue downloads
	downloadList, failedCount := p.searchAndQueueDownloads(ctx, albums)

	if len(downloadList) == 0 {
		p.logger.Info("no albums matched, nothing to download")
		return nil
	}

	p.logger.Info("queued downloads", "count", len(downloadList), "failed", failedCount)

	// Phase 3: Monitor downloads
	if err := p.monitorDownloads(ctx, downloadList); err != nil {
		return fmt.Errorf("monitor downloads: %w", err)
	}

	// Phase 4: Organize files
	if err := p.organizeDownloads(downloadList); err != nil {
		return fmt.Errorf("organize downloads: %w", err)
	}

	// Phase 5: Trigger Lidarr import
	if !p.cfg.Lidarr.DisableSync {
		if err := p.triggerImport(ctx, downloadList); err != nil {
			return fmt.Errorf("trigger import: %w", err)
		}
	}

	// Phase 6: Save state
	if err := p.denylist.Save(); err != nil {
		p.logger.Warn("failed to save denylist", "error", err)
	}

	p.logger.Info("processing complete", "successful", len(downloadList), "failed", failedCount)
	return nil
}

// fetchWantedAlbums retrieves wanted albums from Lidarr with pagination
func (p *Processor) fetchWantedAlbums(ctx context.Context) ([]lidarr.Album, error) {
	var allAlbums []lidarr.Album
	searchType := p.cfg.Search.SearchType

	switch searchType {
	case "all":
		// Fetch all pages
		page := 1
		for {
			resp, err := p.lidarr.GetWanted(ctx, lidarr.GetWantedOptions{
				Page:     page,
				PageSize: 50,
				Missing:  true,
			})
			if err != nil {
				return nil, fmt.Errorf("fetch page %d: %w", page, err)
			}

			allAlbums = append(allAlbums, resp.Records...)

			if len(allAlbums) >= resp.TotalRecords {
				break
			}
			page++
		}

	case "incrementing_page":
		// Fetch current page and increment
		page := p.pageTrack.Current()
		resp, err := p.lidarr.GetWanted(ctx, lidarr.GetWantedOptions{
			Page:     page,
			PageSize: 50,
			Missing:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("fetch page %d: %w", page, err)
		}

		allAlbums = resp.Records

		// Calculate total pages and increment
		totalPages := (resp.TotalRecords + 49) / 50 // Round up
		if err := p.pageTrack.Next(totalPages); err != nil {
			p.logger.Warn("failed to increment page", "error", err)
		}

	case "first_page":
		// Fetch only first page
		resp, err := p.lidarr.GetWanted(ctx, lidarr.GetWantedOptions{
			Page:     1,
			PageSize: 50,
			Missing:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("fetch first page: %w", err)
		}

		allAlbums = resp.Records

	default:
		return nil, fmt.Errorf("invalid search_type: %s", searchType)
	}

	// Filter out albums already in Lidarr's queue
	return p.filterQueuedAlbums(ctx, allAlbums)
}

// filterQueuedAlbums removes albums that are already in Lidarr's download queue
func (p *Processor) filterQueuedAlbums(ctx context.Context, albums []lidarr.Album) ([]lidarr.Album, error) {
	queue, err := p.lidarr.GetQueue(ctx, 1, 1000) // page=1, pageSize=1000
	if err != nil {
		p.logger.Warn("failed to fetch queue, skipping queue filtering", "error", err)
		return albums, nil
	}

	// Build set of queued album IDs
	queuedAlbums := make(map[int]bool)
	for _, item := range queue.Records {
		if item.AlbumID != nil && *item.AlbumID > 0 {
			queuedAlbums[*item.AlbumID] = true
		}
	}

	// Filter albums
	var filtered []lidarr.Album
	for _, album := range albums {
		if !queuedAlbums[album.ID] {
			filtered = append(filtered, album)
		} else {
			p.logger.Debug("skipping queued album", "album", album.Title, "artist", album.Artist.ArtistName)
		}
	}

	return filtered, nil
}

// searchAndQueueDownloads searches for albums and queues downloads
func (p *Processor) searchAndQueueDownloads(ctx context.Context, albums []lidarr.Album) ([]DownloadedItem, int) {
	var downloadList []DownloadedItem
	failedCount := 0

	for _, album := range albums {
		// Check denylist
		if p.denylist.IsDenylisted(album.ID, p.cfg.Search.MaxSearchFailures) {
			entry := p.denylist.GetEntry(album.ID)
			p.logger.Debug("skipping denylisted album",
				"album", album.Title,
				"artist", album.Artist.ArtistName,
				"failures", entry.Failures)
			continue
		}

		// Choose best release
		release, err := p.chooseRelease(ctx, album)
		if err != nil {
			p.logger.Warn("failed to choose release",
				"album", album.Title,
				"error", err)
			p.denylist.RecordAttempt(album.ID, false)
			failedCount++
			continue
		}

		// Get tracks
		tracks, err := p.lidarr.GetTracks(ctx, album.ID, nil)
		if err != nil {
			p.logger.Warn("failed to fetch tracks",
				"album", album.Title,
				"error", err)
			p.denylist.RecordAttempt(album.ID, false)
			failedCount++
			continue
		}

		// Attempt to search and download
		query := fmt.Sprintf("%s %s", album.Artist.ArtistName, album.Title)
		item, found := p.searchForAlbum(ctx, query, tracks, album, release)

		if found {
			downloadList = append(downloadList, item)
			p.denylist.RecordAttempt(album.ID, true)
			p.logger.Info("queued download",
				"album", album.Title,
				"artist", album.Artist.ArtistName,
				"username", item.Username)
		} else {
			p.denylist.RecordAttempt(album.ID, false)
			failedCount++
			p.logger.Warn("no match found",
				"album", album.Title,
				"artist", album.Artist.ArtistName)
		}
	}

	return downloadList, failedCount
}

// chooseRelease selects the best release variant for an album
func (p *Processor) chooseRelease(ctx context.Context, album lidarr.Album) (*lidarr.Release, error) {
	// If album already has releases, use them; otherwise fetch
	releases := album.Releases
	if len(releases) == 0 {
		fullAlbum, err := p.lidarr.GetAlbum(ctx, album.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch album: %w", err)
		}
		releases = fullAlbum.Releases
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases available")
	}

	// Find most common track count
	trackCounts := make(map[int]int)
	for _, r := range releases {
		trackCounts[r.TrackCount]++
	}

	mostCommonCount := 0
	maxOccurrences := 0
	for count, occurrences := range trackCounts {
		if occurrences > maxOccurrences {
			mostCommonCount = count
			maxOccurrences = occurrences
		}
	}

	// Try to find matching release - prefer official releases with most common track count
	for _, release := range releases {
		if release.Status == "Official" && release.TrackCount == mostCommonCount {
			p.logger.Debug("selected release",
				"album", album.Title,
				"format", release.Format,
				"country", release.Country,
				"tracks", release.TrackCount)
			return &release, nil
		}
	}

	// Fallback: first official release
	for _, release := range releases {
		if release.Status == "Official" {
			p.logger.Debug("selected first official release",
				"album", album.Title,
				"format", release.Format)
			return &release, nil
		}
	}

	// Fallback: return first release
	p.logger.Debug("no ideal release found, using first available", "album", album.Title)
	return &releases[0], nil
}

// searchForAlbum searches Slskd for an album and queues download if found
func (p *Processor) searchForAlbum(ctx context.Context, query string, tracks []lidarr.Track, album lidarr.Album, release *lidarr.Release) (DownloadedItem, bool) {
	p.logger.Info("searching", "query", query)

	// Execute search
	searchReq := slskd.SearchRequest{
		SearchText:             query,
		SearchTimeout:          p.cfg.Search.SearchTimeout,
		FilterResponses:        true,
		MaximumPeerQueueLength: 50,
		MinimumPeerUploadSpeed: 0,
	}

	searchResp, err := p.slskd.Search(ctx, searchReq)
	if err != nil {
		p.logger.Warn("search failed", "error", err)
		return DownloadedItem{}, false
	}

	// Wait for search to complete
	time.Sleep(time.Duration(p.cfg.Timing.SearchWaitSeconds) * time.Second)

	// Get search results
	results, err := p.slskd.GetSearchResults(ctx, searchResp.ID)
	if err != nil {
		p.logger.Warn("failed to get search results", "error", err)
		return DownloadedItem{}, false
	}

	if len(results) == 0 {
		p.logger.Debug("no search results")
		return DownloadedItem{}, false
	}

	p.logger.Debug("processing search results", "results", len(results))

	// Build expected track list
	expectedTracks := make([]string, len(tracks))
	for i, track := range tracks {
		expectedTracks[i] = track.Title + ".flac" // TODO: Support other formats
	}

	// Try to match results
	for _, result := range results {
		// Group files by directory
		dirFiles := make(map[string][]string)
		for _, file := range result.Files {
			dir := filepath.Dir(file.Filename)
			filename := filepath.Base(file.Filename)
			dirFiles[dir] = append(dirFiles[dir], filename)
		}

		// Check each directory for matches
		for dir, files := range dirFiles {
			matched, ratio := p.matcher.MatchTracks(expectedTracks, files)
			if matched {
				p.logger.Info("found match",
					"username", result.Username,
					"directory", dir,
					"ratio", fmt.Sprintf("%.2f", ratio),
					"files", len(files))

				// Build file paths to download
				var filePaths []string
				for _, file := range result.Files {
					if filepath.Dir(file.Filename) == dir {
						filePaths = append(filePaths, file.Filename)
					}
				}

				// Enqueue downloads
				if err := p.slskd.EnqueueDownloads(ctx, result.Username, filePaths); err != nil {
					p.logger.Warn("failed to enqueue downloads", "error", err)
					continue
				}

				// Build downloaded item
				item := DownloadedItem{
					ArtistName:  album.Artist.ArtistName,
					AlbumName:   album.Title,
					AlbumID:     album.ID,
					FolderName:  filepath.Base(dir),
					Username:    result.Username,
					Directory:   dir,
					MediumCount: release.MediumCount,
				}

				// Build track list with disc numbers
				for _, track := range tracks {
					item.Tracks = append(item.Tracks, organizer.DownloadedTrack{
						Filename:     track.Title + ".flac",
						MediumNumber: track.MediumNumber,
					})
				}

				return item, true
			}
		}
	}

	return DownloadedItem{}, false
}

// monitorDownloads polls Slskd until all downloads complete or timeout
func (p *Processor) monitorDownloads(ctx context.Context, downloadList []DownloadedItem) error {
	if len(downloadList) == 0 {
		return nil
	}

	p.logger.Info("monitoring downloads", "count", len(downloadList))

	startTime := time.Now()
	pollInterval := time.Duration(p.cfg.Timing.DownloadPollSeconds) * time.Second
	stalledTimeout := time.Duration(p.cfg.Slskd.StalledTimeout) * time.Second

	// Track which items are still pending
	pending := make(map[int]bool)
	for i := range downloadList {
		pending[i] = true
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		unfinished := 0

		for idx, item := range downloadList {
			if !pending[idx] {
				continue // Already completed or errored
			}

			// Get downloads for this user
			downloads, err := p.slskd.GetDownloads(ctx)
			if err != nil {
				p.logger.Warn("failed to fetch downloads", "error", err)
				time.Sleep(pollInterval)
				continue
			}

			// Find matching directory
			var dirFiles []slskd.DownloadFile
			for _, userDownload := range downloads {
				if userDownload.Username != item.Username {
					continue
				}
				for _, dirDownload := range userDownload.Directories {
					if dirDownload.Directory == item.Directory {
						dirFiles = dirDownload.Files
						break
					}
				}
			}

			if len(dirFiles) == 0 {
				p.logger.Debug("no downloads found for item", "username", item.Username, "directory", item.Directory)
				pending[idx] = false
				continue
			}

			// Check for errors
			hasErrors := false
			allComplete := true
			for _, file := range dirFiles {
				if file.IsErrored() {
					hasErrors = true
					p.logger.Warn("download error detected",
						"file", file.Filename,
						"state", file.State)
					break
				}
				if file.IsInProgress() {
					allComplete = false
				}
			}

			if hasErrors {
				p.logger.Error("cancelling download due to errors", "directory", item.Directory)
				// TODO: Cancel downloads and clean up
				pending[idx] = false
				continue
			}

			if allComplete {
				p.logger.Info("download complete", "directory", item.Directory)
				pending[idx] = false
			} else {
				unfinished++
			}
		}

		// Check if all done
		if unfinished == 0 {
			p.logger.Info("all downloads complete")
			break
		}

		// Check for timeout
		if time.Since(startTime) > stalledTimeout {
			p.logger.Warn("download timeout reached", "elapsed", time.Since(startTime))
			break
		}

		p.logger.Debug("downloads in progress", "remaining", unfinished)
		time.Sleep(pollInterval)
	}

	return nil
}

// organizeDownloads organizes downloaded files into proper structure
func (p *Processor) organizeDownloads(downloadList []DownloadedItem) error {
	if len(downloadList) == 0 {
		return nil
	}

	p.logger.Info("organizing downloads", "count", len(downloadList))

	var albums []organizer.DownloadedAlbum
	for _, item := range downloadList {
		album := organizer.DownloadedAlbum{
			ArtistName:  item.ArtistName,
			AlbumName:   item.AlbumName,
			FolderPath:  item.FolderName,
			MediumCount: item.MediumCount,
			Tracks:      item.Tracks,
		}
		albums = append(albums, album)
	}

	if err := p.organizer.OrganizeAlbums(albums); err != nil {
		return fmt.Errorf("organize albums: %w", err)
	}

	p.logger.Info("organization complete")
	return nil
}

// triggerImport triggers Lidarr to import organized files
func (p *Processor) triggerImport(ctx context.Context, downloadList []DownloadedItem) error {
	if len(downloadList) == 0 {
		return nil
	}

	p.logger.Info("triggering Lidarr import", "count", len(downloadList))

	// Group by artist for import
	artistFolders := make(map[string]bool)
	for _, item := range downloadList {
		sanitized := matcher.SanitizeFolderName(item.ArtistName)
		artistFolders[sanitized] = true
	}

	// Trigger import for each artist folder
	var commandIDs []int
	for artistFolder := range artistFolders {
		path := filepath.Join(p.cfg.Lidarr.DownloadDir, artistFolder)

		cmd := lidarr.Command{
			Name: "DownloadedAlbumsScan",
			Body: map[string]interface{}{
				"path": path,
			},
		}

		resp, err := p.lidarr.PostCommand(ctx, cmd)
		if err != nil {
			p.logger.Warn("failed to trigger import", "path", path, "error", err)
			continue
		}

		commandIDs = append(commandIDs, resp.ID)
		p.logger.Info("triggered import", "path", path, "commandID", resp.ID)
	}

	// Poll for completion
	if len(commandIDs) > 0 {
		p.pollImportCompletion(ctx, commandIDs)
	}

	return nil
}

// pollImportCompletion polls Lidarr until import commands complete
func (p *Processor) pollImportCompletion(ctx context.Context, commandIDs []int) {
	pollInterval := time.Duration(p.cfg.Timing.ImportPollSeconds) * time.Second
	pending := make(map[int]bool)
	for _, id := range commandIDs {
		pending[id] = true
	}

	p.logger.Info("polling import completion", "commands", len(commandIDs))

	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			return
		default:
		}

		for id := range pending {
			cmd, err := p.lidarr.GetCommand(ctx, id)
			if err != nil {
				p.logger.Warn("failed to fetch command status", "commandID", id, "error", err)
				continue
			}

			if cmd.Status == "completed" || cmd.Status == "failed" {
				p.logger.Info("import command finished",
					"commandID", id,
					"status", cmd.Status,
					"message", cmd.Message)

				if strings.Contains(strings.ToLower(cmd.Message), "failed") {
					// TODO: Move to failed imports
					p.logger.Warn("import failed", "commandID", id)
				}

				delete(pending, id)
			}
		}

		if len(pending) > 0 {
			time.Sleep(pollInterval)
		}
	}

	p.logger.Info("all imports complete")
}
