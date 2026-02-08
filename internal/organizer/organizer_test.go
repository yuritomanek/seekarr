package organizer

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestOrganizeSingleDisc(t *testing.T) {
	// Create temporary download directory
	tmpDir := t.TempDir()

	// Create test folder structure
	testFolder := "Some.Random.Folder.Name"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	// Create a dummy file
	testFile := filepath.Join(folderPath, "track.flac")
	if err := os.WriteFile(testFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create organizer
	org := NewOrganizer(tmpDir, slog.Default())

	// Organize album
	album := DownloadedAlbum{
		ArtistName:  "Test Artist",
		AlbumName:   "Test Album",
		FolderPath:  testFolder,
		MediumCount: 1,
	}

	if err := org.OrganizeAlbums([]DownloadedAlbum{album}); err != nil {
		t.Fatalf("OrganizeAlbums() error: %v", err)
	}

	// Verify folder was renamed
	expectedPath := filepath.Join(tmpDir, "Test Artist")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected folder not found: %s", expectedPath)
	}

	// Verify old folder is gone
	if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
		t.Errorf("old folder still exists: %s", folderPath)
	}

	// Verify file still exists in new location
	expectedFile := filepath.Join(expectedPath, "track.flac")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("file not found in new location: %s", expectedFile)
	}
}

func TestOrganizeSingleDisc_Collision(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing artist folder
	existingPath := filepath.Join(tmpDir, "Test Artist")
	if err := os.Mkdir(existingPath, 0755); err != nil {
		t.Fatalf("failed to create existing folder: %v", err)
	}

	// Create test folder to organize
	testFolder := "Random.Folder"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	org := NewOrganizer(tmpDir, slog.Default())

	album := DownloadedAlbum{
		ArtistName:  "Test Artist",
		AlbumName:   "Test Album",
		FolderPath:  testFolder,
		MediumCount: 1,
	}

	if err := org.OrganizeAlbums([]DownloadedAlbum{album}); err != nil {
		t.Fatalf("OrganizeAlbums() error: %v", err)
	}

	// Verify folder was renamed with collision suffix
	expectedPath := filepath.Join(tmpDir, "Test Artist_1")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected folder with collision suffix not found: %s", expectedPath)
	}

	// Verify original folder still exists
	if _, err := os.Stat(existingPath); os.IsNotExist(err) {
		t.Errorf("original folder was removed: %s", existingPath)
	}
}

func TestOrganizeMultiDisc(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test folder with files
	testFolder := "Download.Folder"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	// Create dummy files
	files := []string{"01-track1.flac", "02-track2.flac", "03-track3.flac"}
	for _, file := range files {
		filePath := filepath.Join(folderPath, file)
		if err := os.WriteFile(filePath, []byte("dummy"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	org := NewOrganizer(tmpDir, slog.Default())

	album := DownloadedAlbum{
		ArtistName:  "Test Artist",
		AlbumName:   "Test Album",
		FolderPath:  testFolder,
		MediumCount: 2, // Multi-disc
		Tracks: []DownloadedTrack{
			{Filename: "01-track1.flac", MediumNumber: 1},
			{Filename: "02-track2.flac", MediumNumber: 1},
			{Filename: "03-track3.flac", MediumNumber: 2},
		},
	}

	if err := org.OrganizeAlbums([]DownloadedAlbum{album}); err != nil {
		t.Fatalf("OrganizeAlbums() error: %v", err)
	}

	// Verify Artist/Album directory structure was created
	expectedDir := filepath.Join(tmpDir, "Test Artist", "Test Album")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("expected directory not found: %s", expectedDir)
	}

	// Verify all files were moved
	for _, file := range files {
		expectedFile := filepath.Join(expectedDir, file)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("file not found in new location: %s", expectedFile)
		}
	}

	// Verify old folder was removed
	if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
		t.Errorf("old folder still exists: %s", folderPath)
	}
}

func TestOrganizeMultiDisc_WithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test folder with files and subdirectories
	testFolder := "Download.Folder"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	// Create dummy files
	files := []string{"track1.flac", "track2.flac"}
	for _, file := range files {
		filePath := filepath.Join(folderPath, file)
		if err := os.WriteFile(filePath, []byte("dummy"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Create a subdirectory (should be skipped during move)
	subDir := filepath.Join(folderPath, "subfolder")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	org := NewOrganizer(tmpDir, slog.Default())

	album := DownloadedAlbum{
		ArtistName:  "Test Artist",
		AlbumName:   "Test Album",
		FolderPath:  testFolder,
		MediumCount: 2,
		Tracks: []DownloadedTrack{
			{Filename: "track1.flac", MediumNumber: 1},
			{Filename: "track2.flac", MediumNumber: 2},
		},
	}

	if err := org.OrganizeAlbums([]DownloadedAlbum{album}); err != nil {
		t.Fatalf("OrganizeAlbums() error: %v", err)
	}

	// Verify files were moved
	expectedDir := filepath.Join(tmpDir, "Test Artist", "Test Album")
	for _, file := range files {
		expectedFile := filepath.Join(expectedDir, file)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("file not found: %s", expectedFile)
		}
	}

	// Subdirectory should remain in original location (not moved)
	// The original folder won't be deleted if it's not empty
}

func TestSanitizeFolderName(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with invalid characters in folder name
	testFolder := "Test.Folder"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	org := NewOrganizer(tmpDir, slog.Default())

	album := DownloadedAlbum{
		ArtistName:  "Artist/With:Invalid<Characters>",
		AlbumName:   "Test Album",
		FolderPath:  testFolder,
		MediumCount: 1,
	}

	if err := org.OrganizeAlbums([]DownloadedAlbum{album}); err != nil {
		t.Fatalf("OrganizeAlbums() error: %v", err)
	}

	// Verify folder was created with sanitized name
	expectedPath := filepath.Join(tmpDir, "ArtistWithInvalidCharacters")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected sanitized folder not found: %s", expectedPath)
	}
}

func TestMoveToFailedImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test folder
	testFolder := "Failed.Album"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	// Create a dummy file
	testFile := filepath.Join(folderPath, "track.flac")
	if err := os.WriteFile(testFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	org := NewOrganizer(tmpDir, slog.Default())

	if err := org.MoveToFailedImports(folderPath); err != nil {
		t.Fatalf("MoveToFailedImports() error: %v", err)
	}

	// Verify folder was moved to failed_imports
	expectedPath := filepath.Join(tmpDir, "failed_imports", testFolder)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("folder not found in failed_imports: %s", expectedPath)
	}

	// Verify file still exists
	expectedFile := filepath.Join(expectedPath, "track.flac")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("file not found in failed_imports: %s", expectedFile)
	}

	// Verify original folder is gone
	if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
		t.Errorf("original folder still exists: %s", folderPath)
	}
}

func TestMoveToFailedImports_Collision(t *testing.T) {
	tmpDir := t.TempDir()

	// Create failed_imports directory with existing folder
	failedDir := filepath.Join(tmpDir, "failed_imports")
	if err := os.MkdirAll(failedDir, 0755); err != nil {
		t.Fatalf("failed to create failed_imports: %v", err)
	}

	existingFolder := filepath.Join(failedDir, "Failed.Album")
	if err := os.Mkdir(existingFolder, 0755); err != nil {
		t.Fatalf("failed to create existing folder: %v", err)
	}

	// Create test folder to move
	testFolder := "Failed.Album"
	folderPath := filepath.Join(tmpDir, testFolder)
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	org := NewOrganizer(tmpDir, slog.Default())

	if err := org.MoveToFailedImports(folderPath); err != nil {
		t.Fatalf("MoveToFailedImports() error: %v", err)
	}

	// Verify folder was moved with collision suffix
	expectedPath := filepath.Join(failedDir, "Failed.Album_1")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("folder with collision suffix not found: %s", expectedPath)
	}

	// Verify original folder in failed_imports still exists
	if _, err := os.Stat(existingFolder); os.IsNotExist(err) {
		t.Errorf("original folder in failed_imports was removed: %s", existingFolder)
	}
}

func TestFindAvailablePath(t *testing.T) {
	tmpDir := t.TempDir()
	org := NewOrganizer(tmpDir, slog.Default())

	// Create existing files
	basePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(basePath, []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test_1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test_2.txt"), []byte("test"), 0644)

	// Find available path
	availablePath := org.findAvailablePath(basePath)
	expectedPath := filepath.Join(tmpDir, "test_3.txt")

	if availablePath != expectedPath {
		t.Errorf("findAvailablePath() = %s, want %s", availablePath, expectedPath)
	}
}
