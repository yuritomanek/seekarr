package slskd

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

// Client defines the interface for interacting with Slskd API
type Client interface {
	GetVersion(ctx context.Context) (string, error)
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
	GetSearchState(ctx context.Context, searchID string) (*SearchResponse, error)
	GetSearchResults(ctx context.Context, searchID string) ([]SearchResult, error)
	DeleteSearch(ctx context.Context, searchID string) error
	GetDirectory(ctx context.Context, username, directory string) (*Directory, error)
	EnqueueDownloads(ctx context.Context, username string, files []EnqueueFile) error
	GetDownloads(ctx context.Context) (DownloadsResponse, error)
	GetUserDownloads(ctx context.Context, username string) (*UserDownloads, error)
	CancelDownload(ctx context.Context, username, downloadID string) error
	RemoveCompletedDownloads(ctx context.Context) error
}

// client implements the Slskd API client
type client struct {
	baseURL    string
	urlBase    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Slskd API client
func NewClient(baseURL, apiKey, urlBase string) Client {
	if urlBase == "" {
		urlBase = "/"
	}
	return &client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		urlBase:    strings.Trim(urlBase, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetVersion fetches the Slskd version
func (c *client) GetVersion(ctx context.Context) (string, error) {
	endpoint := "/api/v0/application/version"

	// Construct URL
	fullPath := endpoint
	if c.urlBase != "" && c.urlBase != "/" {
		fullPath = "/" + c.urlBase + endpoint
	}

	u, err := url.Parse(c.baseURL + fullPath)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response as plain string
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// Trim quotes if present
	version := strings.Trim(string(bodyBytes), "\"")
	return version, nil
}

// Search executes a search on Slskd
func (c *client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	endpoint := "/api/v0/searches"

	var response SearchResponse
	if err := c.doRequest(ctx, "POST", endpoint, nil, req, &response); err != nil {
		return nil, fmt.Errorf("execute search: %w", err)
	}

	return &response, nil
}

// GetSearchState fetches the state of a search
func (c *client) GetSearchState(ctx context.Context, searchID string) (*SearchResponse, error) {
	endpoint := fmt.Sprintf("/api/v0/searches/%s", searchID)

	var response SearchResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, nil, &response); err != nil {
		return nil, fmt.Errorf("get search state %s: %w", searchID, err)
	}

	return &response, nil
}

// GetSearchResults fetches the results of a search
func (c *client) GetSearchResults(ctx context.Context, searchID string) ([]SearchResult, error) {
	endpoint := fmt.Sprintf("/api/v0/searches/%s/responses", searchID)

	var results []SearchResult
	if err := c.doRequest(ctx, "GET", endpoint, nil, nil, &results); err != nil {
		return nil, fmt.Errorf("get search results %s: %w", searchID, err)
	}

	return results, nil
}

// DeleteSearch deletes a search from Slskd history
func (c *client) DeleteSearch(ctx context.Context, searchID string) error {
	endpoint := fmt.Sprintf("/api/v0/searches/%s", searchID)

	if err := c.doRequest(ctx, "DELETE", endpoint, nil, nil, nil); err != nil {
		return fmt.Errorf("delete search %s: %w", searchID, err)
	}

	return nil
}

// GetDirectory fetches a directory listing from a user
func (c *client) GetDirectory(ctx context.Context, username, directory string) (*Directory, error) {
	endpoint := fmt.Sprintf("/api/v0/users/%s/directory", username)

	req := DirectoryRequest{
		Username:  username,
		Directory: directory,
	}

	var response Directory
	if err := c.doRequest(ctx, "POST", endpoint, nil, req, &response); err != nil {
		return nil, fmt.Errorf("get directory %s from %s: %w", directory, username, err)
	}

	return &response, nil
}

// EnqueueDownloads enqueues files for download from a user
func (c *client) EnqueueDownloads(ctx context.Context, username string, files []EnqueueFile) error {
	endpoint := fmt.Sprintf("/api/v0/transfers/downloads/%s", username)

	// Body should be an array of objects with filename and size
	if err := c.doRequest(ctx, "POST", endpoint, nil, files, nil); err != nil {
		return fmt.Errorf("enqueue downloads for %s: %w", username, err)
	}

	return nil
}

// GetDownloads fetches all downloads grouped by username
func (c *client) GetDownloads(ctx context.Context) (DownloadsResponse, error) {
	endpoint := "/api/v0/transfers/downloads"

	var response DownloadsResponse
	if err := c.doRequest(ctx, "GET", endpoint, nil, nil, &response); err != nil {
		return nil, fmt.Errorf("get downloads: %w", err)
	}

	return response, nil
}

// GetUserDownloads fetches downloads for a specific user
func (c *client) GetUserDownloads(ctx context.Context, username string) (*UserDownloads, error) {
	endpoint := fmt.Sprintf("/api/v0/transfers/downloads/%s", username)

	var response UserDownloads
	if err := c.doRequest(ctx, "GET", endpoint, nil, nil, &response); err != nil {
		return nil, fmt.Errorf("get downloads for %s: %w", username, err)
	}

	return &response, nil
}

// CancelDownload cancels a specific download
func (c *client) CancelDownload(ctx context.Context, username, downloadID string) error {
	endpoint := fmt.Sprintf("/api/v0/transfers/downloads/%s/%s", username, downloadID)

	if err := c.doRequest(ctx, "DELETE", endpoint, nil, nil, nil); err != nil {
		return fmt.Errorf("cancel download %s for %s: %w", downloadID, username, err)
	}

	return nil
}

// RemoveCompletedDownloads removes all completed downloads from the list
func (c *client) RemoveCompletedDownloads(ctx context.Context) error {
	endpoint := "/api/v0/transfers/downloads/completed"

	if err := c.doRequest(ctx, "DELETE", endpoint, nil, nil, nil); err != nil {
		return fmt.Errorf("remove completed downloads: %w", err)
	}

	return nil
}

// doRequest executes an HTTP request to the Slskd API
func (c *client) doRequest(ctx context.Context, method, endpoint string, params url.Values, body, result interface{}) error {
	// Construct URL with optional url_base prefix
	fullPath := endpoint
	if c.urlBase != "" && c.urlBase != "/" {
		fullPath = "/" + c.urlBase + endpoint
	}

	u, err := url.Parse(c.baseURL + fullPath)
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

	req.Header.Set("X-API-Key", c.apiKey)
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
