package lidarr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client defines the interface for interacting with Lidarr API
type Client interface {
	GetWanted(ctx context.Context, opts GetWantedOptions) (*WantedResponse, error)
	GetAlbum(ctx context.Context, id int) (*Album, error)
	GetTracks(ctx context.Context, albumID int, releaseID *int) ([]Track, error)
	UpdateAlbum(ctx context.Context, album *Album) (*Album, error)
	GetQueue(ctx context.Context, page int, pageSize int) (*QueueResponse, error)
	PostCommand(ctx context.Context, cmd Command) (*CommandResponse, error)
	GetCommand(ctx context.Context, id int) (*CommandResponse, error)
}

// client implements the Lidarr API client
type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Lidarr API client
func NewClient(baseURL, apiKey string) Client {
	return &client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Minute}, // Longer timeout for import scans
	}
}

// GetWantedOptions configures a GetWanted request
type GetWantedOptions struct {
	Page     int
	PageSize int
	SortKey  string
	SortDir  string
	Missing  bool // true for missing, false for cutoff_unmet
}

// GetWanted fetches wanted albums (missing or cutoff unmet)
func (c *client) GetWanted(ctx context.Context, opts GetWantedOptions) (*WantedResponse, error) {
	endpoint := "/api/v1/wanted/missing"
	if !opts.Missing {
		endpoint = "/api/v1/wanted/cutoff"
	}

	params := url.Values{}
	if opts.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.SortKey != "" {
		params.Set("sortKey", opts.SortKey)
	}
	if opts.SortDir != "" {
		params.Set("sortDir", opts.SortDir)
	}

	var response WantedResponse
	if err := c.doRequest(ctx, "GET", endpoint, params, nil, &response); err != nil {
		return nil, fmt.Errorf("get wanted: %w", err)
	}

	return &response, nil
}

// GetAlbum fetches a specific album by ID
func (c *client) GetAlbum(ctx context.Context, id int) (*Album, error) {
	endpoint := fmt.Sprintf("/api/v1/album/%d", id)

	var album Album
	if err := c.doRequest(ctx, "GET", endpoint, nil, nil, &album); err != nil {
		return nil, fmt.Errorf("get album %d: %w", id, err)
	}

	return &album, nil
}

// GetTracks fetches tracks for an album, optionally filtered by release
func (c *client) GetTracks(ctx context.Context, albumID int, releaseID *int) ([]Track, error) {
	endpoint := "/api/v1/track"

	params := url.Values{}
	params.Set("albumId", fmt.Sprintf("%d", albumID))
	if releaseID != nil {
		params.Set("albumReleaseId", fmt.Sprintf("%d", *releaseID))
	}

	var tracks []Track
	if err := c.doRequest(ctx, "GET", endpoint, params, nil, &tracks); err != nil {
		return nil, fmt.Errorf("get tracks for album %d: %w", albumID, err)
	}

	return tracks, nil
}

// UpdateAlbum updates an album (e.g., to set monitored status)
func (c *client) UpdateAlbum(ctx context.Context, album *Album) (*Album, error) {
	endpoint := fmt.Sprintf("/api/v1/album/%d", album.ID)

	var updated Album
	if err := c.doRequest(ctx, "PUT", endpoint, nil, album, &updated); err != nil {
		return nil, fmt.Errorf("update album %d: %w", album.ID, err)
	}

	return &updated, nil
}

// GetQueue fetches the download queue with pagination
func (c *client) GetQueue(ctx context.Context, page int, pageSize int) (*QueueResponse, error) {
	endpoint := "/api/v1/queue"

	params := url.Values{}
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	if pageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", pageSize))
	}

	var response QueueResponse
	if err := c.doRequest(ctx, "GET", endpoint, params, nil, &response); err != nil {
		return nil, fmt.Errorf("get queue: %w", err)
	}

	return &response, nil
}

// PostCommand sends a command to Lidarr (e.g., DownloadedAlbumsScan)
func (c *client) PostCommand(ctx context.Context, cmd Command) (*CommandResponse, error) {
	endpoint := "/api/v1/command"

	var response CommandResponse
	if err := c.doRequest(ctx, "POST", endpoint, nil, cmd, &response); err != nil {
		return nil, fmt.Errorf("post command %s: %w", cmd.Name, err)
	}

	return &response, nil
}

// GetCommand fetches the status of a command by ID
func (c *client) GetCommand(ctx context.Context, id int) (*CommandResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/command/%d", id)

	var response CommandResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, nil, &response); err != nil {
		return nil, fmt.Errorf("get command %d: %w", id, err)
	}

	return &response, nil
}

// doRequest executes an HTTP request to the Lidarr API
func (c *client) doRequest(ctx context.Context, method, endpoint string, params url.Values, body, result interface{}) error {
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	if params != nil {
		u.RawQuery = params.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
