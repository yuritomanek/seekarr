package slskd

import "time"

// SearchRequest represents a search request to Slskd
type SearchRequest struct {
	SearchText             string `json:"searchText"`
	SearchTimeout          int    `json:"searchTimeout"`
	FilterResponses        bool   `json:"filterResponses"`
	MaximumPeerQueueLength int    `json:"maximumPeerQueueLength"`
	MinimumPeerUploadSpeed int    `json:"minimumPeerUploadSpeed"`
}

// SearchResponse represents a search response from Slskd
type SearchResponse struct {
	ID         string `json:"id"`
	State      string `json:"state"` // InProgress, Completed
	SearchText string `json:"searchText"`
}

// SearchResult represents a single search result from a user
type SearchResult struct {
	Username string       `json:"username"`
	Files    []SearchFile `json:"files"`
}

// SearchFile represents a file in search results
type SearchFile struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	BitRate    *int   `json:"bitRate,omitempty"`
	SampleRate *int   `json:"sampleRate,omitempty"`
	BitDepth   *int   `json:"bitDepth,omitempty"`
}

// DirectoryRequest represents a request to browse a user's directory
type DirectoryRequest struct {
	Username  string `json:"username"`
	Directory string `json:"directory"`
}

// Directory represents a directory listing from a user
type Directory struct {
	Name  string          `json:"name"`
	Files []DirectoryFile `json:"files"`
}

// DirectoryFile represents a file in a directory listing
type DirectoryFile struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	BitRate    *int   `json:"bitRate,omitempty"`
	SampleRate *int   `json:"sampleRate,omitempty"`
	BitDepth   *int   `json:"bitDepth,omitempty"`
}

// EnqueueRequest represents a request to enqueue files for download
type EnqueueRequest struct {
	Username string   `json:"username"`
	Files    []string `json:"files"`
}

// EnqueueFile represents a file to enqueue for download
type EnqueueFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// DownloadsResponse represents the downloads grouped by username
type DownloadsResponse []UserDownloads

// UserDownloads represents downloads for a specific user
type UserDownloads struct {
	Username    string               `json:"username"`
	Directories []DirectoryDownloads `json:"directories"`
}

// DirectoryDownloads represents downloads for a directory
type DirectoryDownloads struct {
	Directory string         `json:"directory"`
	Files     []DownloadFile `json:"files"`
}

// DownloadFile represents a file being downloaded
type DownloadFile struct {
	ID               string     `json:"id"`
	Filename         string     `json:"filename"`
	State            string     `json:"state"` // "Phase, Status" format
	BytesTransferred int64      `json:"bytesTransferred"`
	Size             int64      `json:"size"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	EndedAt          *time.Time `json:"endedAt,omitempty"`
}

// VersionResponse represents Slskd version information
type VersionResponse struct {
	Version string `json:"version"`
}

// IsCompleted checks if a download is in a completed state
func (d *DownloadFile) IsCompleted() bool {
	return d.State != "" && len(d.State) >= 9 && d.State[:9] == "Completed"
}

// IsErrored checks if a download is in an error state
func (d *DownloadFile) IsErrored() bool {
	errorStates := []string{
		"Completed, Cancelled",
		"Completed, TimedOut",
		"Completed, Errored",
		"Completed, Rejected",
	}
	for _, state := range errorStates {
		if d.State == state {
			return true
		}
	}
	return false
}

// IsInProgress checks if a download is currently in progress
func (d *DownloadFile) IsInProgress() bool {
	return !d.IsCompleted()
}
