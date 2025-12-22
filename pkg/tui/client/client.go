package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/recording"
)

// Client is an HTTP client for the mockd Admin API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Admin API client.
// baseURL should be in the format "http://localhost:9090"
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewDefaultClient creates a client pointing to localhost:9090.
func NewDefaultClient() *Client {
	return NewClient("http://localhost:9090")
}

// doRequest performs an HTTP request and decodes the JSON response.
func (c *Client) doRequest(method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Check for error responses
	if resp.StatusCode >= 400 {
		var errResp admin.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}
		return fmt.Errorf("%s: %s", errResp.Error, errResp.Message)
	}

	// For 204 No Content, don't try to decode
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	// Decode response
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// --- Health Operations ---

// GetHealth checks the health of the mockd server.
func (c *Client) GetHealth() (*admin.HealthResponse, error) {
	var result admin.HealthResponse
	err := c.doRequest("GET", "/health", nil, &result)
	return &result, err
}

// --- Mock Operations ---

// ListMocks retrieves all mocks.
func (c *Client) ListMocks() ([]*config.MockConfiguration, error) {
	var result admin.MockListResponse
	err := c.doRequest("GET", "/mocks", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Mocks, nil
}

// ListMocksFiltered retrieves mocks filtered by enabled status.
func (c *Client) ListMocksFiltered(enabled bool) ([]*config.MockConfiguration, error) {
	path := fmt.Sprintf("/mocks?enabled=%t", enabled)
	var result admin.MockListResponse
	err := c.doRequest("GET", path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Mocks, nil
}

// GetMock retrieves a specific mock by ID.
func (c *Client) GetMock(id string) (*config.MockConfiguration, error) {
	var result config.MockConfiguration
	err := c.doRequest("GET", "/mocks/"+id, nil, &result)
	return &result, err
}

// CreateMock creates a new mock.
func (c *Client) CreateMock(mock *config.MockConfiguration) (*config.MockConfiguration, error) {
	var result config.MockConfiguration
	err := c.doRequest("POST", "/mocks", mock, &result)
	return &result, err
}

// UpdateMock updates an existing mock.
func (c *Client) UpdateMock(id string, mock *config.MockConfiguration) (*config.MockConfiguration, error) {
	var result config.MockConfiguration
	err := c.doRequest("PUT", "/mocks/"+id, mock, &result)
	return &result, err
}

// DeleteMock deletes a mock.
func (c *Client) DeleteMock(id string) error {
	return c.doRequest("DELETE", "/mocks/"+id, nil, nil)
}

// ToggleMock enables or disables a mock.
func (c *Client) ToggleMock(id string, enabled bool) (*config.MockConfiguration, error) {
	req := admin.ToggleRequest{Enabled: enabled}
	var result config.MockConfiguration
	err := c.doRequest("POST", "/mocks/"+id+"/toggle", &req, &result)
	return &result, err
}

// --- Traffic/Request Operations ---

// RequestLogFilter represents filter options for request logs.
type RequestLogFilter struct {
	Method    string
	Path      string
	MatchedID string
	Limit     int
	Offset    int
}

// GetTraffic retrieves request logs with optional filtering.
func (c *Client) GetTraffic(filter *RequestLogFilter) ([]*config.RequestLogEntry, error) {
	path := "/requests"
	if filter != nil {
		path += "?"
		if filter.Method != "" {
			path += fmt.Sprintf("method=%s&", filter.Method)
		}
		if filter.Path != "" {
			path += fmt.Sprintf("path=%s&", filter.Path)
		}
		if filter.MatchedID != "" {
			path += fmt.Sprintf("matched=%s&", filter.MatchedID)
		}
		if filter.Limit > 0 {
			path += fmt.Sprintf("limit=%d&", filter.Limit)
		}
		if filter.Offset > 0 {
			path += fmt.Sprintf("offset=%d&", filter.Offset)
		}
	}

	var result admin.RequestLogListResponse
	err := c.doRequest("GET", path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Requests, nil
}

// GetRequest retrieves a specific request log entry.
func (c *Client) GetRequest(id string) (*config.RequestLogEntry, error) {
	var result config.RequestLogEntry
	err := c.doRequest("GET", "/requests/"+id, nil, &result)
	return &result, err
}

// ClearTraffic clears all request logs.
func (c *Client) ClearTraffic() error {
	return c.doRequest("DELETE", "/requests", nil, nil)
}

// --- Recording Operations (HTTP Recordings) ---

// RecordingFilter represents filter options for recordings.
type RecordingFilter struct {
	SessionID string
	Method    string
	Path      string
	Limit     int
}

// ListRecordings retrieves all HTTP recordings.
func (c *Client) ListRecordings(filter *RecordingFilter) ([]*recording.Recording, error) {
	path := "/recordings"
	if filter != nil {
		path += "?"
		if filter.SessionID != "" {
			path += fmt.Sprintf("session=%s&", filter.SessionID)
		}
		if filter.Method != "" {
			path += fmt.Sprintf("method=%s&", filter.Method)
		}
		if filter.Path != "" {
			path += fmt.Sprintf("path=%s&", filter.Path)
		}
		if filter.Limit > 0 {
			path += fmt.Sprintf("limit=%d&", filter.Limit)
		}
	}

	var result admin.RecordingListResponse
	err := c.doRequest("GET", path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Recordings, nil
}

// GetRecording retrieves a specific recording.
func (c *Client) GetRecording(id string) (*recording.Recording, error) {
	var result recording.Recording
	err := c.doRequest("GET", "/recordings/"+id, nil, &result)
	return &result, err
}

// DeleteRecording deletes a recording.
func (c *Client) DeleteRecording(id string) error {
	return c.doRequest("DELETE", "/recordings/"+id, nil, nil)
}

// ExportRecording exports recordings to JSON.
func (c *Client) ExportRecording(sessionID string, recordingIDs []string) ([]byte, error) {
	req := admin.ExportRequest{
		SessionID:    sessionID,
		RecordingIDs: recordingIDs,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/recordings/export", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// ConvertRecording converts recordings to mocks.
func (c *Client) ConvertRecording(sessionID string, recordingIDs []string, deduplicate, includeHeaders bool) ([]string, error) {
	req := admin.ConvertRequest{
		SessionID:      sessionID,
		RecordingIDs:   recordingIDs,
		Deduplicate:    deduplicate,
		IncludeHeaders: includeHeaders,
	}

	var result admin.ConvertResult
	err := c.doRequest("POST", "/recordings/convert", &req, &result)
	if err != nil {
		return nil, err
	}
	return result.MockIDs, nil
}

// --- Stream Recording Operations ---

// StreamRecordingFilter represents filter options for stream recordings.
type StreamRecordingFilter struct {
	Protocol  string
	Path      string
	Status    string
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

// ListStreamRecordings retrieves all stream recordings (WebSocket/SSE).
func (c *Client) ListStreamRecordings(filter *StreamRecordingFilter) ([]*recording.RecordingSummary, error) {
	path := "/stream-recordings"
	if filter != nil {
		path += "?"
		if filter.Protocol != "" {
			path += fmt.Sprintf("protocol=%s&", filter.Protocol)
		}
		if filter.Path != "" {
			path += fmt.Sprintf("path=%s&", filter.Path)
		}
		if filter.Status != "" {
			path += fmt.Sprintf("status=%s&", filter.Status)
		}
		if filter.Limit > 0 {
			path += fmt.Sprintf("limit=%d&", filter.Limit)
		}
		if filter.Offset > 0 {
			path += fmt.Sprintf("offset=%d&", filter.Offset)
		}
		if filter.SortBy != "" {
			path += fmt.Sprintf("sortBy=%s&", filter.SortBy)
		}
		if filter.SortOrder != "" {
			path += fmt.Sprintf("sortOrder=%s&", filter.SortOrder)
		}
	}

	var result admin.StreamRecordingListResponse
	err := c.doRequest("GET", path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Recordings, nil
}

// GetStreamRecording retrieves a specific stream recording.
func (c *Client) GetStreamRecording(id string) (*recording.StreamRecording, error) {
	var result recording.StreamRecording
	err := c.doRequest("GET", "/stream-recordings/"+id, nil, &result)
	return &result, err
}

// DeleteStreamRecording deletes a stream recording.
func (c *Client) DeleteStreamRecording(id string) error {
	return c.doRequest("DELETE", "/stream-recordings/"+id, nil, nil)
}

// StartReplay starts replaying a stream recording.
func (c *Client) StartReplay(id, mode string, timingScale float64, strictMatching bool, timeout int) (string, error) {
	req := admin.StartReplayRequest{
		Mode:           mode,
		TimingScale:    timingScale,
		StrictMatching: strictMatching,
		Timeout:        timeout,
	}

	var result admin.StartReplayResponse
	err := c.doRequest("POST", "/stream-recordings/"+id+"/replay", &req, &result)
	if err != nil {
		return "", err
	}
	return result.SessionID, nil
}

// StopReplay stops a replay session.
func (c *Client) StopReplay(sessionID string) error {
	return c.doRequest("DELETE", "/replay/"+sessionID, nil, nil)
}

// GetReplayStatus retrieves the status of a replay session.
func (c *Client) GetReplayStatus(sessionID string) (*admin.ReplayStatusResponse, error) {
	var result admin.ReplayStatusResponse
	err := c.doRequest("GET", "/replay/"+sessionID, nil, &result)
	return &result, err
}

// ListReplaySessions lists all active replay sessions.
func (c *Client) ListReplaySessions() ([]*admin.ReplayStatusResponse, error) {
	var result []*admin.ReplayStatusResponse
	err := c.doRequest("GET", "/replay", nil, &result)
	return result, err
}

// ExportStreamRecording exports a stream recording.
func (c *Client) ExportStreamRecording(id string) ([]byte, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/stream-recordings/"+id+"/export", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// ConvertStreamRecording converts a stream recording to a mock configuration.
func (c *Client) ConvertStreamRecording(id string, opts *admin.ConvertRecordingRequest) (*admin.ConvertRecordingResponse, error) {
	var result admin.ConvertRecordingResponse
	err := c.doRequest("POST", "/stream-recordings/"+id+"/convert", opts, &result)
	return &result, err
}

// --- Status Operations ---

// GetProxyStatus retrieves the proxy status.
func (c *Client) GetProxyStatus() (*admin.ProxyStatusResponse, error) {
	var result admin.ProxyStatusResponse
	err := c.doRequest("GET", "/proxy/status", nil, &result)
	return &result, err
}

// GetStats retrieves server statistics.
// Note: This is a placeholder - actual stats endpoint may vary
func (c *Client) GetStats() (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.doRequest("GET", "/stats", nil, &result)
	return result, err
}

// --- Additional Helper Operations ---

// Ping checks if the server is reachable.
func (c *Client) Ping() error {
	_, err := c.GetHealth()
	return err
}

// GetStreamRecordingStats retrieves storage statistics for stream recordings.
func (c *Client) GetStreamRecordingStats() (*admin.StreamRecordingStatsResponse, error) {
	var result admin.StreamRecordingStatsResponse
	err := c.doRequest("GET", "/stream-recordings/stats", nil, &result)
	return &result, err
}

// GetActiveSessions retrieves active recording sessions.
func (c *Client) GetActiveSessions() ([]*recording.StreamRecordingSessionInfo, error) {
	var result []*recording.StreamRecordingSessionInfo
	err := c.doRequest("GET", "/stream-recordings/sessions", nil, &result)
	return result, err
}
