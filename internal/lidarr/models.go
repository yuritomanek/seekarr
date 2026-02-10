package lidarr

import "time"

// Album represents a Lidarr album
type Album struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	ArtistID  int       `json:"artistId"`
	Artist    Artist    `json:"artist"`
	Releases  []Release `json:"releases"`
	Monitored bool      `json:"monitored"`
}

// Artist represents a Lidarr artist
type Artist struct {
	ID         int    `json:"id"`
	ArtistName string `json:"artistName"`
}

// Release represents an album release variant
type Release struct {
	ID          int      `json:"id"`
	AlbumID     int      `json:"albumId"`
	TrackCount  int      `json:"trackCount"`
	MediumCount int      `json:"mediumCount"`
	Country     []string `json:"country"`
	Format      string   `json:"format"`
	Status      string   `json:"status"`
	Media       []Medium `json:"media"`
}

// Medium represents a disc/medium in a release
type Medium struct {
	MediumNumber int    `json:"mediumNumber"`
	MediumName   string `json:"mediumName"`
}

// Track represents a music track
type Track struct {
	ID                  int    `json:"id"`
	Title               string `json:"title"`
	AlbumID             int    `json:"albumId"`
	MediumNumber        int    `json:"mediumNumber"`
	AbsoluteTrackNumber int    `json:"absoluteTrackNumber"`
}

// WantedResponse represents paginated wanted albums response
type WantedResponse struct {
	Page          int     `json:"page"`
	PageSize      int     `json:"pageSize"`
	SortKey       string  `json:"sortKey"`
	SortDirection string  `json:"sortDirection"`
	TotalRecords  int     `json:"totalRecords"`
	Records       []Album `json:"records"`
}

// QueueResponse represents paginated queue response
type QueueResponse struct {
	Page          int         `json:"page"`
	PageSize      int         `json:"pageSize"`
	SortKey       string      `json:"sortKey"`
	SortDirection string      `json:"sortDirection"`
	TotalRecords  int         `json:"totalRecords"`
	Records       []QueueItem `json:"records"`
}

// QueueItem represents an item in the download queue
type QueueItem struct {
	ID      int    `json:"id"`
	AlbumID *int   `json:"albumId,omitempty"` // Can be nil for some entries
	Title   string `json:"title"`
	Status  string `json:"status"`
}

// Command represents a Lidarr command request
// For requests, parameters should be at the top level (not in body)
type Command struct {
	Name string `json:"name"`
	// Path is used for DownloadedAlbumsScan command
	Path string `json:"path,omitempty"`
	// Additional parameters can be added as needed
}

// CommandResponse represents a command status response
type CommandResponse struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	CommandName string                 `json:"commandName"`
	Message     string                 `json:"message"`
	Status      string                 `json:"status"` // queued, started, completed, failed
	Started     *time.Time             `json:"started,omitempty"`
	Ended       *time.Time             `json:"ended,omitempty"`
	Body        map[string]interface{} `json:"body,omitempty"`
}
