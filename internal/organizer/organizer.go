package organizer

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yuritomanek/seekarr/internal/matcher"
)

// DownloadedAlbum represents an album that has been downloaded and needs organization
type DownloadedAlbum struct {
	ArtistName  string
	AlbumName   string
	FolderPath  string // Current folder path in download directory
	MediumCount int    // Number of discs
	Tracks      []DownloadedTrack
}

// DownloadedTrack represents a track with its disc number
type DownloadedTrack struct {
	Filename     string
	MediumNumber int // Disc number
}

// Organizer handles file organization and metadata tagging
type Organizer struct {
	downloadDir string
	logger      *slog.Logger
}

// NewOrganizer creates a new file organizer
func NewOrganizer(downloadDir string, logger *slog.Logger) *Organizer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Organizer{
		downloadDir: downloadDir,
		logger:      logger,
	}
}

// OrganizeAlbums processes a list of downloaded albums
// For single-disc: Renames folder to sanitized artist name
// For multi-disc: Tags files with metadata and reorganizes into Artist/Album structure
func (o *Organizer) OrganizeAlbums(albums []DownloadedAlbum) error {
	// Sort by artist name for better organization
	// (In Go, we could use sort.Slice here, but for simplicity keeping order as-is)

	for _, album := range albums {
		if err := o.organizeAlbum(album); err != nil {
			o.logger.Error("failed to organize album",
				"artist", album.ArtistName,
				"album", album.AlbumName,
				"error", err)
			return fmt.Errorf("organize album %s - %s: %w", album.ArtistName, album.AlbumName, err)
		}
	}

	return nil
}

// organizeAlbum organizes a single album
func (o *Organizer) organizeAlbum(album DownloadedAlbum) error {
	sanitizedArtist := matcher.SanitizeFolderName(album.ArtistName)

	if album.MediumCount > 1 {
		// Multi-disc: Tag files and reorganize
		return o.organizeMultiDisc(album, sanitizedArtist)
	}

	// Single disc: Just rename folder
	return o.organizeSingleDisc(album, sanitizedArtist)
}

// organizeSingleDisc renames the folder to the sanitized artist name
func (o *Organizer) organizeSingleDisc(album DownloadedAlbum, sanitizedArtist string) error {
	oldPath := filepath.Join(o.downloadDir, album.FolderPath)
	newPath := filepath.Join(o.downloadDir, sanitizedArtist)

	// Check if source exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("source folder does not exist: %s", oldPath)
	}

	// If already at correct name, skip
	if oldPath == newPath {
		o.logger.Info("folder already correctly named", "path", newPath)
		return nil
	}

	// Handle collision: If target exists, try appending counter
	if _, err := os.Stat(newPath); err == nil {
		newPath = o.findAvailablePath(newPath)
	}

	o.logger.Info("renaming folder", "from", oldPath, "to", newPath)
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename folder: %w", err)
	}

	return nil
}

// organizeMultiDisc tags files with metadata and reorganizes into Artist/Album structure
func (o *Organizer) organizeMultiDisc(album DownloadedAlbum, sanitizedArtist string) error {
	folderPath := filepath.Join(o.downloadDir, album.FolderPath)
	sanitizedAlbum := matcher.SanitizeFolderName(album.AlbumName)

	// Step 1: Tag all files with metadata
	for _, track := range album.Tracks {
		filePath := filepath.Join(folderPath, track.Filename)
		if err := o.tagFile(filePath, album.ArtistName, album.AlbumName, track.MediumNumber); err != nil {
			o.logger.Warn("failed to tag file",
				"file", track.Filename,
				"error", err)
			// Continue with other files even if one fails
		}
	}

	// Step 2: Create target directory structure
	artistDir := filepath.Join(o.downloadDir, sanitizedArtist)
	albumDir := filepath.Join(artistDir, sanitizedAlbum)

	if err := os.MkdirAll(albumDir, 0755); err != nil {
		return fmt.Errorf("create album directory: %w", err)
	}

	// Step 3: Move all files to target directory
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return fmt.Errorf("read folder: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		srcPath := filepath.Join(folderPath, file.Name())
		dstPath := filepath.Join(albumDir, file.Name())

		// Handle collision
		if _, err := os.Stat(dstPath); err == nil {
			dstPath = o.findAvailablePath(dstPath)
		}

		if err := os.Rename(srcPath, dstPath); err != nil {
			o.logger.Warn("failed to move file",
				"from", srcPath,
				"to", dstPath,
				"error", err)
		}
	}

	// Step 4: Remove original folder if empty
	if err := os.Remove(folderPath); err != nil {
		o.logger.Warn("failed to remove original folder",
			"path", folderPath,
			"error", err)
	}

	o.logger.Info("organized multi-disc album",
		"artist", album.ArtistName,
		"album", album.AlbumName,
		"discs", album.MediumCount)

	return nil
}

// tagFile writes metadata to an audio file
func (o *Organizer) tagFile(filePath, artist, album string, discNumber int) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".mp3":
		return o.tagMP3(filePath, artist, album, discNumber)
	case ".flac":
		return o.tagFLAC(filePath, artist, album, discNumber)
	default:
		// Unsupported format, skip
		o.logger.Debug("skipping unsupported format", "file", filePath, "ext", ext)
		return nil
	}
}

// tagMP3 writes ID3v2 tags to an MP3 file using ffmpeg
func (o *Organizer) tagMP3(filePath, artist, album string, discNumber int) error {
	return o.tagWithFFmpeg(filePath, artist, album, discNumber)
}

// tagFLAC writes Vorbis comments to a FLAC file using ffmpeg
func (o *Organizer) tagFLAC(filePath, artist, album string, discNumber int) error {
	return o.tagWithFFmpeg(filePath, artist, album, discNumber)
}

// tagWithFFmpeg uses ffmpeg to write metadata to audio files
// This approach works for all audio formats (FLAC, MP3, M4A, etc.)
func (o *Organizer) tagWithFFmpeg(filePath, artist, album string, discNumber int) error {
	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		o.logger.Debug("ffmpeg not found, skipping metadata tagging", "file", filePath)
		return nil // Don't fail if ffmpeg is not available
	}

	// Create temporary output file
	tmpFile := filePath + ".tmp"

	// Build ffmpeg command
	args := []string{
		"-i", filePath,
		"-map", "0",
		"-codec", "copy",
		"-metadata", fmt.Sprintf("artist=%s", artist),
		"-metadata", fmt.Sprintf("album=%s", album),
		"-metadata", fmt.Sprintf("album_artist=%s", artist),
	}

	if discNumber > 0 {
		args = append(args, "-metadata", fmt.Sprintf("disc=%d", discNumber))
	}

	args = append(args, "-y", tmpFile)

	cmd := exec.Command("ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}

	// Replace original file with tagged version
	if err := os.Rename(tmpFile, filePath); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("replace file: %w", err)
	}

	return nil
}

// findAvailablePath finds an available path by appending _1, _2, etc.
// For files, preserves extension (file_1.txt). For directories, appends to name (folder_1)
func (o *Organizer) findAvailablePath(basePath string) string {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath)

	// Check if this is a directory or file
	info, err := os.Stat(basePath)
	isDir := err == nil && info.IsDir()

	// For directories, don't split by extension
	// For files, split to preserve extension
	var name, ext string
	if isDir {
		name = base
		ext = ""
	} else {
		ext = filepath.Ext(base)
		name = strings.TrimSuffix(base, ext)
	}

	for i := 1; ; i++ {
		var newPath string
		if ext != "" {
			newPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
		} else {
			newPath = filepath.Join(dir, fmt.Sprintf("%s_%d", name, i))
		}
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}

// MoveToFailedImports moves a folder to the failed_imports directory
func (o *Organizer) MoveToFailedImports(folderPath string) error {
	failedDir := filepath.Join(o.downloadDir, "failed_imports")
	if err := os.MkdirAll(failedDir, 0755); err != nil {
		return fmt.Errorf("create failed_imports directory: %w", err)
	}

	folderName := filepath.Base(folderPath)
	targetPath := filepath.Join(failedDir, folderName)

	// Handle collision
	if _, err := os.Stat(targetPath); err == nil {
		targetPath = o.findAvailablePath(targetPath)
	}

	o.logger.Info("moving to failed imports", "from", folderPath, "to", targetPath)
	if err := os.Rename(folderPath, targetPath); err != nil {
		return fmt.Errorf("move to failed_imports: %w", err)
	}

	return nil
}
