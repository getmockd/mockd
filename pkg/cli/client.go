package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/requestlog"
)

const (
	// APIKeyHeader is the HTTP header for API key authentication.
	APIKeyHeader = "X-API-Key"
)

// AdminClient provides methods for communicating with the mockd admin API.
type AdminClient interface {
	// ListMocks returns all configured mocks.
	ListMocks() ([]*config.MockConfiguration, error)
	// ListMocksByType returns mocks filtered by type (http, websocket, graphql, etc.)
	ListMocksByType(mockType string) ([]*config.MockConfiguration, error)
	// GetMock returns a specific mock by ID.
	GetMock(id string) (*config.MockConfiguration, error)
	// CreateMock creates a new mock or merges into existing one.
	// Returns a CreateMockResult with the mock and action taken.
	CreateMock(mock *config.MockConfiguration) (*CreateMockResult, error)
	// UpdateMock updates an existing mock by ID.
	UpdateMock(id string, mock *config.MockConfiguration) (*config.MockConfiguration, error)
	// DeleteMock deletes a mock by ID.
	DeleteMock(id string) error
	// ImportConfig imports a mock collection, optionally replacing existing mocks.
	ImportConfig(collection *config.MockCollection, replace bool) (*ImportResult, error)
	// ExportConfig exports all mocks as a collection.
	ExportConfig(name string) (*config.MockCollection, error)
	// GetLogs returns request log entries with optional filtering.
	GetLogs(filter *LogFilter) (*LogResult, error)
	// ClearLogs deletes all request log entries.
	ClearLogs() (int, error)
	// Health checks if the server is running.
	Health() error
	// GetChaosConfig returns the current chaos configuration.
	GetChaosConfig() (map[string]interface{}, error)
	// SetChaosConfig updates the chaos configuration.
	SetChaosConfig(config map[string]interface{}) error
	// GetMQTTStatus returns the current MQTT broker status.
	GetMQTTStatus() (map[string]interface{}, error)
	// GetStats returns server statistics.
	GetStats() (*StatsResult, error)
	// GetPorts returns all ports in use by mockd.
	GetPorts() ([]PortInfo, error)
	// GetPortsVerbose returns all ports with optional extended info.
	GetPortsVerbose(verbose bool) ([]PortInfo, error)

	// ListWorkspaces returns all workspaces on the admin server.
	ListWorkspaces() ([]*WorkspaceDTO, error)
	// CreateWorkspace creates a new workspace on the admin.
	CreateWorkspace(name string) (*WorkspaceResult, error)
	// RegisterEngine registers an engine with the admin.
	RegisterEngine(name, host string, port int) (*RegisterEngineResult, error)
	// AddEngineWorkspace assigns a workspace to an engine.
	AddEngineWorkspace(engineID, workspaceID, workspaceName string) error
	// BulkCreateMocks creates multiple mocks in a single request.
	BulkCreateMocks(mocks []*mock.Mock, workspaceID string) (*BulkCreateResult, error)
}

// LogFilter specifies filtering criteria for request logs.
type LogFilter struct {
	Protocol  string // Filter by protocol (http, grpc, mqtt, soap, graphql, websocket, sse)
	Method    string
	Path      string
	MatchedID string
	Limit     int
	Offset    int
}

// LogResult contains request log query results.
type LogResult struct {
	Requests []*requestlog.Entry
	Count    int
	Total    int
}

// ImportResult contains import operation results.
type ImportResult struct {
	Message           string
	Imported          int
	Total             int
	StatefulResources int
}

// CreateMockResult contains the result of a create mock operation.
// This can be either a new creation or a merge into an existing mock.
type CreateMockResult struct {
	Mock          *config.MockConfiguration
	Action        string   // "created" or "merged"
	Message       string   // Human-readable message
	TargetMockID  string   // For merge: ID of the mock merged into
	AddedServices []string // For gRPC merge: services/methods added
	AddedTopics   []string // For MQTT merge: topics added
	TotalServices []string // For gRPC merge: all services after merge
	TotalTopics   []string // For MQTT merge: all topics after merge
}

// IsMerge returns true if this result was a merge operation.
func (r *CreateMockResult) IsMerge() bool {
	return r.Action == "merged"
}

// StatsResult contains server statistics.
type StatsResult struct {
	Uptime        int   `json:"uptime"`
	TotalRequests int64 `json:"totalRequests"`
	MockCount     int   `json:"mockCount"`
}

// WorkspaceResult contains the result of creating a workspace.
type WorkspaceResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// RegisterEngineResult contains the result of registering an engine.
type RegisterEngineResult struct {
	ID             string `json:"id"`
	Token          string `json:"token,omitempty"`
	ConfigEndpoint string `json:"configEndpoint"`
}

// BulkCreateResult contains the result of a bulk mock creation.
type BulkCreateResult struct {
	Created  int          `json:"created"`
	Mocks    []*mock.Mock `json:"mocks"`
	Warnings []string     `json:"warnings,omitempty"`
}

// APIError represents an error response from the admin API.
type APIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// adminClient implements AdminClient using HTTP.
type adminClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// ClientOption configures an admin client.
type ClientOption func(*adminClient)

// WithTimeout sets the HTTP timeout for the client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *adminClient) {
		c.httpClient.Timeout = timeout
	}
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) ClientOption {
	return func(c *adminClient) {
		c.apiKey = key
	}
}

// LoadAPIKeyFromFile loads the API key from the default file location.
// Returns the key and nil error if successful, empty string and nil if file doesn't exist,
// or empty string and error if there was a read error.
// Deprecated: Use cliconfig.LoadAPIKeyFromFile() instead.
func LoadAPIKeyFromFile() (string, error) {
	return cliconfig.LoadAPIKeyFromFile()
}

// GetAPIKeyFilePath returns the default path for the API key file.
// Deprecated: Use cliconfig.GetAPIKeyFilePath() instead.
func GetAPIKeyFilePath() string {
	return cliconfig.GetAPIKeyFilePath()
}

// NewAdminClient creates a new admin API client.
// The baseURL should be the admin API base URL (e.g., "http://localhost:4290").
func NewAdminClient(baseURL string, opts ...ClientOption) AdminClient {
	c := &adminClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewAdminClientWithAuth creates a new admin API client that automatically
// loads the API key from all configured sources (env, context, file).
// This is the recommended way to create a client for CLI commands.
func NewAdminClientWithAuth(baseURL string, opts ...ClientOption) AdminClient {
	// Use centralized API key resolution (env > context > file)
	apiKey := cliconfig.GetAPIKey()
	if apiKey != "" {
		opts = append([]ClientOption{WithAPIKey(apiKey)}, opts...)
	}
	return NewAdminClient(baseURL, opts...)
}

// NewClientFromConfig creates a new admin API client using resolved configuration.
// This is the preferred way to create a client - it handles URL and auth automatically.
func NewClientFromConfig(flagAdminURL string, opts ...ClientOption) AdminClient {
	cfg := cliconfig.ResolveClientConfigSimple(flagAdminURL)
	if cfg.APIKey != "" {
		opts = append([]ClientOption{WithAPIKey(cfg.APIKey)}, opts...)
	}
	return NewAdminClient(cfg.AdminURL, opts...)
}

// ListMocks returns all configured mocks.
func (c *adminClient) ListMocks() ([]*config.MockConfiguration, error) {
	resp, err := c.get("/mocks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Mocks []*config.MockConfiguration `json:"mocks"`
		Count int                         `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result.Mocks, nil
}

// ListMocksByType returns mocks filtered by type.
func (c *adminClient) ListMocksByType(mockType string) ([]*config.MockConfiguration, error) {
	mocks, err := c.ListMocks()
	if err != nil {
		return nil, err
	}

	var filtered []*config.MockConfiguration
	for _, m := range mocks {
		if string(m.Type) == mockType {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// GetMock returns a specific mock by ID.
func (c *adminClient) GetMock(id string) (*config.MockConfiguration, error) {
	resp, err := c.get("/mocks/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  "not_found",
			Message:    fmt.Sprintf("mock not found: %s", id),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var mock config.MockConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&mock); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &mock, nil
}

// CreateMock creates a new mock or merges into an existing one.
func (c *adminClient) CreateMock(mock *config.MockConfiguration) (*CreateMockResult, error) {
	body, err := json.Marshal(mock)
	if err != nil {
		return nil, fmt.Errorf("failed to encode mock: %w", err)
	}

	resp, err := c.post("/mocks", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Accept both 200 (merged) and 201 (created)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	// Read response body for parsing
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// First, try to detect if this is a merge response by looking for "action" field
	var mergeResponse struct {
		Action        string                    `json:"action"`
		Message       string                    `json:"message"`
		TargetMockID  string                    `json:"targetMockId"`
		AddedServices []string                  `json:"addedServices"`
		AddedTopics   []string                  `json:"addedTopics"`
		TotalServices []string                  `json:"totalServices"`
		TotalTopics   []string                  `json:"totalTopics"`
		Mock          *config.MockConfiguration `json:"mock"`
	}

	if err := json.Unmarshal(respBody, &mergeResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// If we have an action and mock field, this is a merge response
	if mergeResponse.Action != "" && mergeResponse.Mock != nil {
		return &CreateMockResult{
			Mock:          mergeResponse.Mock,
			Action:        mergeResponse.Action,
			Message:       mergeResponse.Message,
			TargetMockID:  mergeResponse.TargetMockID,
			AddedServices: mergeResponse.AddedServices,
			AddedTopics:   mergeResponse.AddedTopics,
			TotalServices: mergeResponse.TotalServices,
			TotalTopics:   mergeResponse.TotalTopics,
		}, nil
	}

	// Otherwise, this is a standard create response - the body IS the mock
	var created config.MockConfiguration
	if err := json.Unmarshal(respBody, &created); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &CreateMockResult{
		Mock:   &created,
		Action: "created",
	}, nil
}

// UpdateMock updates an existing mock by ID.
func (c *adminClient) UpdateMock(id string, mock *config.MockConfiguration) (*config.MockConfiguration, error) {
	body, err := json.Marshal(mock)
	if err != nil {
		return nil, fmt.Errorf("failed to encode mock: %w", err)
	}

	resp, err := c.put("/mocks/"+url.PathEscape(id), body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  "not_found",
			Message:    fmt.Sprintf("mock not found: %s", id),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var updated config.MockConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &updated, nil
}

// DeleteMock deletes a mock by ID.
func (c *adminClient) DeleteMock(id string) error {
	resp, err := c.delete("/mocks/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  "not_found",
			Message:    fmt.Sprintf("mock not found: %s", id),
		}
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}
	return nil
}

// ImportConfig imports a mock collection, optionally replacing existing mocks.
func (c *adminClient) ImportConfig(collection *config.MockCollection, replace bool) (*ImportResult, error) {
	reqBody := struct {
		Replace bool                   `json:"replace"`
		Config  *config.MockCollection `json:"config"`
	}{
		Replace: replace,
		Config:  collection,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode config: %w", err)
	}

	resp, err := c.post("/config", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Message           string `json:"message"`
		Imported          int    `json:"imported"`
		Total             int    `json:"total"`
		StatefulResources int    `json:"statefulResources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &ImportResult{
		Message:           result.Message,
		Imported:          result.Imported,
		Total:             result.Total,
		StatefulResources: result.StatefulResources,
	}, nil
}

// ExportConfig exports all mocks as a collection.
func (c *adminClient) ExportConfig(name string) (*config.MockCollection, error) {
	path := "/config"
	if name != "" {
		path += "?name=" + url.QueryEscape(name)
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var collection config.MockCollection
	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &collection, nil
}

// GetLogs returns request log entries with optional filtering.
func (c *adminClient) GetLogs(filter *LogFilter) (*LogResult, error) {
	path := "/requests"
	params := url.Values{}

	if filter != nil {
		if filter.Protocol != "" {
			params.Set("protocol", filter.Protocol)
		}
		if filter.Method != "" {
			params.Set("method", filter.Method)
		}
		if filter.Path != "" {
			params.Set("path", filter.Path)
		}
		if filter.MatchedID != "" {
			params.Set("matched", filter.MatchedID)
		}
		if filter.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", filter.Limit))
		}
		if filter.Offset > 0 {
			params.Set("offset", fmt.Sprintf("%d", filter.Offset))
		}
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Requests []*requestlog.Entry `json:"requests"`
		Count    int                 `json:"count"`
		Total    int                 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &LogResult{
		Requests: result.Requests,
		Count:    result.Count,
		Total:    result.Total,
	}, nil
}

// ClearLogs deletes all request log entries.
func (c *adminClient) ClearLogs() (int, error) {
	resp, err := c.delete("/requests")
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, c.parseError(resp)
	}

	var result struct {
		Cleared int `json:"cleared"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}
	return result.Cleared, nil
}

// Health checks if the server is running.
func (c *adminClient) Health() error {
	resp, err := c.get("/health")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// GetChaosConfig returns the current chaos configuration.
func (c *adminClient) GetChaosConfig() (map[string]interface{}, error) {
	resp, err := c.get("/chaos")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// SetChaosConfig updates the chaos configuration.
func (c *adminClient) SetChaosConfig(chaosConfig map[string]interface{}) error {
	body, err := json.Marshal(chaosConfig)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	resp, err := c.put("/chaos", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// GetMQTTStatus returns the current MQTT broker status.
func (c *adminClient) GetMQTTStatus() (map[string]interface{}, error) {
	resp, err := c.get("/mqtt/status")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// GetStats returns server statistics.
func (c *adminClient) GetStats() (*StatsResult, error) {
	resp, err := c.get("/stats")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result StatsResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// GetPorts returns all ports in use by mockd.
func (c *adminClient) GetPorts() ([]PortInfo, error) {
	return c.GetPortsVerbose(false)
}

// GetPortsVerbose returns all ports with optional extended info (engine ID, name, etc).
func (c *adminClient) GetPortsVerbose(verbose bool) ([]PortInfo, error) {
	path := "/ports"
	if verbose {
		path += "?verbose=true"
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Ports []PortInfo `json:"ports"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result.Ports, nil
}

// CreateWorkspace creates a new workspace on the admin.
func (c *adminClient) ListWorkspaces() ([]*WorkspaceDTO, error) {
	resp, err := c.doRequest("GET", "/workspaces", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Workspaces []*WorkspaceDTO `json:"workspaces"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result.Workspaces, nil
}

func (c *adminClient) CreateWorkspace(name string) (*WorkspaceResult, error) {
	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.post("/workspaces", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result WorkspaceResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// RegisterEngine registers an engine with the admin.
func (c *adminClient) RegisterEngine(name, host string, port int) (*RegisterEngineResult, error) {
	body, err := json.Marshal(map[string]interface{}{
		"name": name,
		"host": host,
		"port": port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.post("/engines/register", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result RegisterEngineResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// AddEngineWorkspace assigns a workspace to an engine.
func (c *adminClient) AddEngineWorkspace(engineID, workspaceID, workspaceName string) error {
	body, err := json.Marshal(map[string]string{
		"workspaceId":   workspaceID,
		"workspaceName": workspaceName,
	})
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.post("/engines/"+url.PathEscape(engineID)+"/workspaces", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}
	return nil
}

// BulkCreateMocks creates multiple mocks in a single request.
// Uses replace=true for idempotent behavior (re-running mockd up works).
func (c *adminClient) BulkCreateMocks(mocks []*mock.Mock, workspaceID string) (*BulkCreateResult, error) {
	body, err := json.Marshal(mocks)
	if err != nil {
		return nil, fmt.Errorf("failed to encode mocks: %w", err)
	}

	params := url.Values{}
	params.Set("replace", "true")
	if workspaceID != "" {
		params.Set("workspaceId", workspaceID)
	}
	path := "/mocks/bulk?" + params.Encode()

	resp, err := c.post(path, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result BulkCreateResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}

// get performs an HTTP GET request.
func (c *adminClient) get(path string) (*http.Response, error) {
	return c.doRequest(http.MethodGet, path, nil)
}

// post performs an HTTP POST request.
func (c *adminClient) post(path string, body []byte) (*http.Response, error) {
	return c.doRequest(http.MethodPost, path, body)
}

// put performs an HTTP PUT request.
func (c *adminClient) put(path string, body []byte) (*http.Response, error) {
	return c.doRequest(http.MethodPut, path, body)
}

// delete performs an HTTP DELETE request.
func (c *adminClient) delete(path string) (*http.Response, error) {
	return c.doRequest(http.MethodDelete, path, nil)
}

// doRequest performs an HTTP request.
func (c *adminClient) doRequest(method, path string, body []byte) (*http.Response, error) {
	fullURL := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Add API key header if configured
	if c.apiKey != "" {
		req.Header.Set(APIKeyHeader, c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    fmt.Sprintf("cannot connect to admin API at %s: %v", c.baseURL, err),
		}
	}
	return resp, nil
}

// parseError parses an error response from the API.
func (c *adminClient) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		return &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  errResp.Error,
			Message:    errResp.Message,
		}
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  "unknown_error",
		Message:    fmt.Sprintf("server returned status %d: %s", resp.StatusCode, string(body)),
	}
}

// FormatConnectionError returns a user-friendly error message for connection failures.
func FormatConnectionError(err error) string {
	if apiErr, ok := err.(*APIError); ok && apiErr.ErrorCode == "connection_error" {
		return fmt.Sprintf(`Error: %s

Suggestions:
  • Start the server: mockd start
  • Check if the server is running on the expected port
  • Verify the admin URL with: mockd config`, apiErr.Message)
	}
	return err.Error()
}

// FormatNotFoundError returns a user-friendly error message for not found errors.
func FormatNotFoundError(resourceType, id string) string {
	return fmt.Sprintf(`Error: %s not found: %s

Suggestions:
  • Check the ID with: mockd list
  • Verify you're connected to the right server`, resourceType, id)
}
