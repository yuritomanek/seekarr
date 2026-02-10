package processor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuritomanek/seekarr/internal/config"
	"github.com/yuritomanek/seekarr/internal/filter"
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
	filter    *filter.Filter
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

// countMatched counts how many tracks matched in match info
func countMatched(info []matcher.TrackMatchInfo) int {
	count := 0
	for _, i := range info {
		if i.Matched {
			count++
		}
	}
	return count
}

// formatOptionalInt formats an optional int pointer for logging
func formatOptionalInt(val *int) string {
	if val == nil {
		return "N/A"
	}
	return fmt.Sprintf("%d", *val)
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
	f := filter.NewFilter(cfg.Search.AllowedFiletypes)
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
		filter:    f,
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
	successfulDownloads, err := p.monitorDownloads(ctx, downloadList)
	if err != nil {
		return fmt.Errorf("monitor downloads: %w", err)
	}

	// Phase 4: Organize files
	if err := p.organizeDownloads(successfulDownloads); err != nil {
		return fmt.Errorf("organize downloads: %w", err)
	}

	// Phase 5: Trigger Lidarr import
	if !p.cfg.Lidarr.DisableSync {
		if err := p.triggerImport(ctx, successfulDownloads); err != nil {
			return fmt.Errorf("trigger import: %w", err)
		}
	}

	// Phase 6: Save state
	if err := p.denylist.Save(); err != nil {
		p.logger.Warn("failed to save denylist", "error", err)
	}

	p.logger.Info("processing complete", "successful", len(successfulDownloads), "failed", failedCount)
	return nil
}

// fetchWantedAlbums retrieves wanted albums from Lidarr with pagination
func (p *Processor) fetchWantedAlbums(ctx context.Context) ([]lidarr.Album, error) {
	var allAlbums []lidarr.Album
	searchType := p.cfg.Search.SearchType

	// Determine page size from config
	pageSize := p.cfg.Search.NumberOfAlbumsToGrab
	if pageSize <= 0 {
		pageSize = 50 // Default
	}

	switch searchType {
	case "all":
		// Fetch all pages
		page := 1
		for {
			resp, err := p.lidarr.GetWanted(ctx, lidarr.GetWantedOptions{
				Page:     page,
				PageSize: pageSize,
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
			PageSize: pageSize,
			Missing:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("fetch page %d: %w", page, err)
		}

		allAlbums = resp.Records

		// Calculate total pages and increment
		totalPages := (resp.TotalRecords + pageSize - 1) / pageSize // Round up
		if err := p.pageTrack.Next(totalPages); err != nil {
			p.logger.Warn("failed to increment page", "error", err)
		}

	case "first_page":
		// Fetch only first page
		resp, err := p.lidarr.GetWanted(ctx, lidarr.GetWantedOptions{
			Page:     1,
			PageSize: pageSize,
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
		// Check title blacklist
		albumTitle := strings.ToLower(album.Title)
		blacklisted := false
		for _, term := range p.cfg.Search.TitleBlacklist {
			if strings.Contains(albumTitle, strings.ToLower(term)) {
				p.logger.Debug("skipping blacklisted album",
					"album", album.Title,
					"artist", album.Artist.ArtistName,
					"term", term)
				blacklisted = true
				break
			}
		}
		if blacklisted {
			continue
		}

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
		MaximumPeerQueueLength: p.cfg.Search.MaximumPeerQueue,
		MinimumPeerUploadSpeed: p.cfg.Search.MinimumPeerUploadSpeed,
	}

	searchResp, err := p.slskd.Search(ctx, searchReq)
	if err != nil {
		p.logger.Warn("search failed", "error", err)
		return DownloadedItem{}, false
	}

	p.logger.Debug("search initiated", "searchID", searchResp.ID, "state", searchResp.State)

	// Delete search when done if configured
	if p.cfg.Slskd.DeleteSearches {
		defer func() {
			if err := p.slskd.DeleteSearch(ctx, searchResp.ID); err != nil {
				p.logger.Debug("failed to delete search", "searchID", searchResp.ID, "error", err)
			}
		}()
	}

	// Wait for search to complete by polling state
	maxWaitTime := time.Duration(p.cfg.Timing.SearchWaitSeconds) * time.Second
	pollInterval := 500 * time.Millisecond
	startTime := time.Now()

	for {
		state, err := p.slskd.GetSearchState(ctx, searchResp.ID)
		if err != nil {
			p.logger.Warn("failed to get search state", "searchID", searchResp.ID, "error", err)
			break
		}

		p.logger.Debug("search state", "searchID", searchResp.ID, "state", state.State)

		if strings.HasPrefix(state.State, "Completed") {
			break
		}

		if time.Since(startTime) >= maxWaitTime {
			p.logger.Debug("search timeout reached", "searchID", searchResp.ID, "elapsed", time.Since(startTime))
			break
		}

		time.Sleep(pollInterval)
	}

	// Get search results
	results, err := p.slskd.GetSearchResults(ctx, searchResp.ID)
	if err != nil {
		p.logger.Warn("failed to get search results", "searchID", searchResp.ID, "error", err)
		return DownloadedItem{}, false
	}

	p.logger.Debug("fetched search results", "searchID", searchResp.ID, "results", len(results))

	if len(results) == 0 {
		p.logger.Debug("no search results", "searchID", searchResp.ID)
		return DownloadedItem{}, false
	}

	p.logger.Debug("processing search results", "results", len(results))

	// Build expected track list (without extensions - matcher will handle file format variations)
	expectedTracks := make([]string, len(tracks))
	for i, track := range tracks {
		expectedTracks[i] = track.Title
	}

	// Try to match results
	for _, result := range results {
		// Check ignored users
		ignored := false
		for _, ignoredUser := range p.cfg.Search.IgnoredUsers {
			if strings.EqualFold(result.Username, ignoredUser) {
				p.logger.Debug("skipping ignored user", "username", result.Username)
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}

		p.logger.Debug("processing result",
			"username", result.Username,
			"totalFiles", len(result.Files))

		// Filter files by allowed filetypes first
		filteredFiles, filterInfo := p.filter.FilterFilesDebug(result.Files)

		// Log sample of filtered files (first 5)
		sampleSize := 5
		if len(filterInfo) < sampleSize {
			sampleSize = len(filterInfo)
		}
		for i := 0; i < sampleSize; i++ {
			info := filterInfo[i]
			p.logger.Debug("file filter",
				"username", result.Username,
				"file", info.Filename,
				"ext", info.Extension,
				"bitrate", formatOptionalInt(info.BitRate),
				"sampleRate", formatOptionalInt(info.SampleRate),
				"bitDepth", formatOptionalInt(info.BitDepth),
				"matched", info.Matched)
		}

		p.logger.Debug("filtered by filetype",
			"username", result.Username,
			"before", len(result.Files),
			"after", len(filteredFiles),
			"allowedTypes", strings.Join(p.cfg.Search.AllowedFiletypes, ", "))

		if len(filteredFiles) == 0 {
			p.logger.Debug("skipping user - no files match allowed filetypes",
				"username", result.Username)
			continue
		}

		// Group files by directory
		// Note: slskd returns paths with backslashes regardless of OS
		dirFiles := make(map[string][]string)
		for _, file := range filteredFiles {
			// Normalize Windows backslashes to forward slashes
			normalizedPath := strings.ReplaceAll(file.Filename, "\\", "/")
			dir := filepath.Dir(normalizedPath)
			filename := filepath.Base(normalizedPath)
			dirFiles[dir] = append(dirFiles[dir], filename)
		}

		p.logger.Debug("grouped into directories",
			"username", result.Username,
			"directories", len(dirFiles))

		// Check each directory for matches
		for dir, files := range dirFiles {
			p.logger.Debug("checking directory",
				"username", result.Username,
				"directory", dir,
				"files", len(files),
				"expectedTracks", len(expectedTracks))

			// Use debug matcher to get detailed match info
			matched, ratio, matchInfo := p.matcher.MatchTracksDebug(expectedTracks, files)

			// Log each track match attempt
			for _, info := range matchInfo {
				p.logger.Debug("track match",
					"expected", info.ExpectedTrack,
					"bestMatch", info.BestMatch,
					"ratio", fmt.Sprintf("%.2f", info.BestRatio),
					"matched", info.Matched,
					"threshold", p.cfg.Search.MinimumFilenameMatchRatio)
			}

			p.logger.Debug("directory match result",
				"username", result.Username,
				"directory", dir,
				"matched", matched,
				"avgRatio", fmt.Sprintf("%.2f", ratio),
				"matchedTracks", countMatched(matchInfo),
				"totalTracks", len(expectedTracks))

			if matched {
				p.logger.Info("found match",
					"username", result.Username,
					"directory", dir,
					"ratio", fmt.Sprintf("%.2f", ratio),
					"files", len(files))

				// Build file objects to download (from filtered files)
				var enqueueFiles []slskd.EnqueueFile
				for _, file := range filteredFiles {
					normalizedPath := strings.ReplaceAll(file.Filename, "\\", "/")
					if filepath.Dir(normalizedPath) == dir {
						enqueueFiles = append(enqueueFiles, slskd.EnqueueFile{
							Filename: file.Filename, // Keep original path for slskd
							Size:     file.Size,
						})
					}
				}

				// Enqueue downloads
				if err := p.slskd.EnqueueDownloads(ctx, result.Username, enqueueFiles); err != nil {
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

				// Build track list from actual downloaded files
				// Map track titles to their medium numbers for lookup
				trackMediums := make(map[string]int)
				for _, track := range tracks {
					trackMediums[strings.ToLower(track.Title)] = track.MediumNumber
				}

				for _, file := range filteredFiles {
					normalizedPath := strings.ReplaceAll(file.Filename, "\\", "/")
					if filepath.Dir(normalizedPath) == dir {
						filename := filepath.Base(normalizedPath)
						// Try to determine medium number by matching filename to track title
						mediumNum := 1 // Default to disc 1
						filenameNoExt := matcher.ExtractFilename(filename)
						for title, medium := range trackMediums {
							if strings.Contains(strings.ToLower(filenameNoExt), title) {
								mediumNum = medium
								break
							}
						}

						item.Tracks = append(item.Tracks, organizer.DownloadedTrack{
							Filename:     filename,
							MediumNumber: mediumNum,
						})
					}
				}

				return item, true
			}
		}
	}

	return DownloadedItem{}, false
}

// monitorDownloads polls Slskd until all downloads complete or timeout
// Returns only the successfully completed downloads
func (p *Processor) monitorDownloads(ctx context.Context, downloadList []DownloadedItem) ([]DownloadedItem, error) {
	if len(downloadList) == 0 {
		return nil, nil
	}

	p.logger.Info("monitoring downloads", "count", len(downloadList))

	startTime := time.Now()
	pollInterval := time.Duration(p.cfg.Timing.DownloadPollSeconds) * time.Second
	stalledTimeout := time.Duration(p.cfg.Slskd.StalledTimeout) * time.Second

	// Track which items are still pending, which succeeded, and retry counts
	pending := make(map[int]bool)
	succeeded := make(map[int]bool)
	retryCount := make(map[int]int)
	maxRetries := 3
	for i := range downloadList {
		pending[i] = true
		retryCount[i] = 0
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
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
					// Normalize both paths for comparison
					normalizedDownloadDir := strings.ReplaceAll(dirDownload.Directory, "\\", "/")
					if normalizedDownloadDir == item.Directory {
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

			// Separate files into completed, in-progress, and errored
			var completedFiles []slskd.DownloadFile
			var erroredFiles []slskd.DownloadFile
			var inProgressFiles []slskd.DownloadFile

			for _, file := range dirFiles {
				if file.IsErrored() {
					erroredFiles = append(erroredFiles, file)
				} else if file.IsCompleted() {
					completedFiles = append(completedFiles, file)
				} else {
					inProgressFiles = append(inProgressFiles, file)
				}
			}

			// Handle errors with retry logic
			if len(erroredFiles) > 0 {
				p.logger.Warn("some files failed",
					"directory", item.Directory,
					"completed", len(completedFiles),
					"errored", len(erroredFiles),
					"inProgress", len(inProgressFiles),
					"retries", retryCount[idx])

				// Cancel the errored files from slskd
				for _, file := range erroredFiles {
					p.logger.Debug("cancelling failed file", "file", file.Filename, "state", file.State)
					if err := p.slskd.CancelDownload(ctx, item.Username, file.ID); err != nil {
						p.logger.Debug("failed to cancel download", "error", err)
					}
				}

				// Check if we should retry
				if retryCount[idx] < maxRetries {
					retryCount[idx]++
					p.logger.Info("retrying failed files",
						"directory", item.Directory,
						"filesCount", len(erroredFiles),
						"attempt", retryCount[idx])

					// Re-enqueue the failed files
					var retryFiles []slskd.EnqueueFile
					for _, file := range erroredFiles {
						// Extract just the filename from the full path
						normalizedPath := strings.ReplaceAll(file.Filename, "\\", "/")
						if filepath.Dir(normalizedPath) == item.Directory {
							retryFiles = append(retryFiles, slskd.EnqueueFile{
								Filename: file.Filename,
								Size:     file.Size,
							})
						}
					}

					if len(retryFiles) > 0 {
						if err := p.slskd.EnqueueDownloads(ctx, item.Username, retryFiles); err != nil {
							p.logger.Warn("failed to re-enqueue files", "error", err)
						}
					}

					// Keep monitoring this item
					unfinished++
				} else {
					// Exceeded max retries
					// If there are still files in progress, wait for them to finish
					if len(inProgressFiles) > 0 {
						p.logger.Debug("max retries exceeded but files still in progress, waiting",
							"directory", item.Directory,
							"inProgress", len(inProgressFiles))
						unfinished++
					} else {
						// All files done - import any successful tracks
						// Lidarr will track what's still missing for the next run
						if len(completedFiles) > 0 {
							totalFiles := len(completedFiles) + len(erroredFiles)
							successRate := float64(len(completedFiles)) / float64(totalFiles)
							p.logger.Warn("max retries exceeded, importing partial album",
								"directory", item.Directory,
								"retries", retryCount[idx],
								"completed", len(completedFiles),
								"failed", len(erroredFiles),
								"successRate", fmt.Sprintf("%.0f%%", successRate*100))
							succeeded[idx] = true
						} else {
							// No files succeeded at all
							p.logger.Error("giving up after max retries - no files succeeded",
								"directory", item.Directory,
								"retries", retryCount[idx])
						}
						pending[idx] = false
					}
				}
			} else if len(inProgressFiles) > 0 {
				// Still downloading
				unfinished++
			} else {
				// All complete, no errors
				p.logger.Info("download complete", "directory", item.Directory, "files", len(completedFiles))
				pending[idx] = false
				succeeded[idx] = true
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

	// Build list of successful downloads
	var successfulDownloads []DownloadedItem
	for idx, item := range downloadList {
		if succeeded[idx] {
			successfulDownloads = append(successfulDownloads, item)
		}
	}

	failedCount := len(downloadList) - len(successfulDownloads)
	if failedCount > 0 {
		p.logger.Warn("some downloads failed", "failed", failedCount, "succeeded", len(successfulDownloads))
	}

	return successfulDownloads, nil
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
			Path: path,
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
					"message", cmd.Message,
					"body", cmd.Body)

				if strings.Contains(strings.ToLower(cmd.Message), "failed") {
					// TODO: Move to failed imports
					p.logger.Warn("import failed", "commandID", id, "body", cmd.Body)
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
