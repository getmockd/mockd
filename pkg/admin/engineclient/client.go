package engineclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// Client is an HTTP client for communicating with an Engine.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string // optional auth token
}

// Option configures a Client.
type Option func(*Client)

// WithTimeout sets the HTTP timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithToken sets the auth token.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// New creates a new engine client.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
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

// Health checks if the engine is healthy.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.get(ctx, "/health")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("engine unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// Status returns the engine status.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	resp, err := c.get(ctx, "/status")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}
	return &status, nil
}

// Deploy deploys mocks to the engine.
func (c *Client) Deploy(ctx context.Context, req *DeployRequest) (*DeployResponse, error) {
	resp, err := c.post(ctx, "/deploy", req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result DeployResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode deploy response: %w", err)
	}
	return &result, nil
}

// Undeploy removes all mocks from the engine.
func (c *Client) Undeploy(ctx context.Context) error {
	resp, err := c.delete(ctx, "/deploy")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// ListMocks returns all mocks on the engine.
func (c *Client) ListMocks(ctx context.Context) ([]*config.MockConfiguration, error) {
	resp, err := c.get(ctx, "/mocks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result MockListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode mocks: %w", err)
	}
	return result.Mocks, nil
}

// GetMock returns a specific mock.
func (c *Client) GetMock(ctx context.Context, id string) (*config.MockConfiguration, error) {
	resp, err := c.get(ctx, "/mocks/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var mock config.MockConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&mock); err != nil {
		return nil, fmt.Errorf("failed to decode mock: %w", err)
	}
	return &mock, nil
}

// DeleteMock deletes a mock from the engine.
func (c *Client) DeleteMock(ctx context.Context, id string) error {
	resp, err := c.delete(ctx, "/mocks/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// ListRequests returns request logs from the engine.
func (c *Client) ListRequests(ctx context.Context, filter *requestlog.Filter) (*RequestListResponse, error) {
	path := "/requests"
	if filter != nil {
		q := url.Values{}
		if filter.Limit > 0 {
			q.Set("limit", strconv.Itoa(filter.Limit))
		}
		if filter.Offset > 0 {
			q.Set("offset", strconv.Itoa(filter.Offset))
		}
		if filter.Protocol != "" {
			q.Set("protocol", filter.Protocol)
		}
		if filter.Method != "" {
			q.Set("method", filter.Method)
		}
		if filter.Path != "" {
			q.Set("path", filter.Path)
		}
		if filter.MatchedID != "" {
			q.Set("matched", filter.MatchedID)
		}
		if filter.StatusCode != 0 {
			q.Set("status", strconv.Itoa(filter.StatusCode))
		}
		if filter.HasError != nil {
			if *filter.HasError {
				q.Set("hasError", "true")
			} else {
				q.Set("hasError", "false")
			}
		}
		// Protocol-specific filters
		if filter.GRPCService != "" {
			q.Set("grpcService", filter.GRPCService)
		}
		if filter.MQTTTopic != "" {
			q.Set("mqttTopic", filter.MQTTTopic)
		}
		if filter.MQTTClientID != "" {
			q.Set("mqttClientId", filter.MQTTClientID)
		}
		if filter.SOAPOperation != "" {
			q.Set("soapOperation", filter.SOAPOperation)
		}
		if filter.GraphQLOpType != "" {
			q.Set("graphqlOpType", filter.GraphQLOpType)
		}
		if filter.WSConnectionID != "" {
			q.Set("wsConnectionId", filter.WSConnectionID)
		}
		if filter.SSEConnectionID != "" {
			q.Set("sseConnectionId", filter.SSEConnectionID)
		}
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result RequestListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode requests: %w", err)
	}
	return &result, nil
}

// ClearRequests clears all request logs.
func (c *Client) ClearRequests(ctx context.Context) (int, error) {
	resp, err := c.delete(ctx, "/requests")
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
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Cleared, nil
}

// ClearRequestsByMockID clears request logs for a specific mock.
func (c *Client) ClearRequestsByMockID(ctx context.Context, mockID string) (int, error) {
	resp, err := c.delete(ctx, "/requests/mock/"+mockID)
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
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Cleared, nil
}

// GetProtocols returns protocol status.
func (c *Client) GetProtocols(ctx context.Context) (map[string]ProtocolStatus, error) {
	resp, err := c.get(ctx, "/protocols")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result map[string]ProtocolStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode protocols: %w", err)
	}
	return result, nil
}

// CreateMock creates a new mock on the engine.
func (c *Client) CreateMock(ctx context.Context, mock *config.MockConfiguration) (*config.MockConfiguration, error) {
	resp, err := c.post(ctx, "/mocks", mock)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		return nil, ErrDuplicate
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var created config.MockConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("failed to decode mock: %w", err)
	}
	return &created, nil
}

// UpdateMock updates a mock on the engine.
func (c *Client) UpdateMock(ctx context.Context, id string, mock *config.MockConfiguration) (*config.MockConfiguration, error) {
	resp, err := c.put(ctx, "/mocks/"+url.PathEscape(id), mock)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var updated config.MockConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("failed to decode mock: %w", err)
	}
	return &updated, nil
}

// ToggleMock enables or disables a mock.
func (c *Client) ToggleMock(ctx context.Context, id string, enabled bool) (*config.MockConfiguration, error) {
	body := map[string]bool{"enabled": enabled}
	resp, err := c.post(ctx, "/mocks/"+url.PathEscape(id)+"/toggle", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var mock config.MockConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&mock); err != nil {
		return nil, fmt.Errorf("failed to decode mock: %w", err)
	}
	return &mock, nil
}

// ExportConfig exports the engine mocks as a collection.
func (c *Client) ExportConfig(ctx context.Context, name string) (*config.MockCollection, error) {
	path := "/export"
	if name != "" {
		path += "?name=" + url.QueryEscape(name)
	}
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var collection config.MockCollection
	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}
	return &collection, nil
}

// ImportResult contains the result of a config import operation.
type ImportResult struct {
	Imported int                 `json:"imported"`
	Total    int                 `json:"total"`
	Message  string              `json:"message"`
	Errors   []map[string]string `json:"errors,omitempty"`
}

// ImportConfig imports a configuration to the engine and returns the result
// including any per-mock errors that occurred during import.
func (c *Client) ImportConfig(ctx context.Context, collection *config.MockCollection, replace bool) (*ImportResult, error) {
	body := map[string]interface{}{
		"config":  collection,
		"replace": replace,
	}
	resp, err := c.post(ctx, "/config", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result ImportResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Response decoded fine on the HTTP side; the import succeeded
		// even if we can't parse the detailed result.
		return &ImportResult{Imported: len(collection.Mocks), Total: len(collection.Mocks)}, nil
	}
	return &result, nil
}

// GetRequest returns a specific request log entry.
func (c *Client) GetRequest(ctx context.Context, id string) (*RequestLogEntry, error) {
	resp, err := c.get(ctx, "/requests/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var entry RequestLogEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("failed to decode request: %w", err)
	}
	return &entry, nil
}

// GetChaos returns the chaos configuration.
func (c *Client) GetChaos(ctx context.Context) (*ChaosConfig, error) {
	resp, err := c.get(ctx, "/chaos")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var cfg ChaosConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode chaos config: %w", err)
	}
	return &cfg, nil
}

// SetChaos updates the chaos configuration.
func (c *Client) SetChaos(ctx context.Context, cfg *ChaosConfig) error {
	resp, err := c.put(ctx, "/chaos", cfg)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// GetChaosStats returns chaos injection statistics.
func (c *Client) GetChaosStats(ctx context.Context) (*ChaosStats, error) {
	resp, err := c.get(ctx, "/chaos/stats")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var stats ChaosStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode chaos stats: %w", err)
	}
	return &stats, nil
}

// ResetChaosStats resets chaos injection statistics.
func (c *Client) ResetChaosStats(ctx context.Context) error {
	resp, err := c.post(ctx, "/chaos/stats/reset", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// GetStateOverview returns overview of all stateful resources.
func (c *Client) GetStateOverview(ctx context.Context) (*StateOverview, error) {
	resp, err := c.get(ctx, "/state")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var overview StateOverview
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return nil, fmt.Errorf("failed to decode state overview: %w", err)
	}
	return &overview, nil
}

// ResetState resets stateful resources to initial state.
// If resourceName is empty, all resources are reset.
func (c *Client) ResetState(ctx context.Context, resourceName string) error {
	body := map[string]string{}
	if resourceName != "" {
		body["resource"] = resourceName
	}
	resp, err := c.post(ctx, "/state/reset", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// GetStateResource returns a specific stateful resource.
func (c *Client) GetStateResource(ctx context.Context, name string) (interface{}, error) {
	resp, err := c.get(ctx, "/state/resources/"+url.PathEscape(name))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var resource interface{}
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed to decode state resource: %w", err)
	}
	return resource, nil
}

// ClearStateResource clears a specific stateful resource.
func (c *Client) ClearStateResource(ctx context.Context, name string) error {
	resp, err := c.delete(ctx, "/state/resources/"+url.PathEscape(name))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}
	return nil
}

// ListHandlers returns all protocol handlers.
func (c *Client) ListHandlers(ctx context.Context) ([]*ProtocolHandler, error) {
	resp, err := c.get(ctx, "/handlers")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var response struct {
		Handlers []*ProtocolHandler `json:"handlers"`
		Count    int                `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode handlers: %w", err)
	}
	return response.Handlers, nil
}

// GetHandler returns a specific protocol handler.
func (c *Client) GetHandler(ctx context.Context, id string) (*ProtocolHandler, error) {
	resp, err := c.get(ctx, "/handlers/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var handler ProtocolHandler
	if err := json.NewDecoder(resp.Body).Decode(&handler); err != nil {
		return nil, fmt.Errorf("failed to decode handler: %w", err)
	}
	return &handler, nil
}

// ListSSEConnections returns all SSE connections.
func (c *Client) ListSSEConnections(ctx context.Context) ([]*SSEConnection, error) {
	resp, err := c.get(ctx, "/sse/connections")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Connections []*SSEConnection `json:"connections"`
		Count       int              `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode SSE connections: %w", err)
	}
	return result.Connections, nil
}

// GetSSEConnection returns a specific SSE connection.
func (c *Client) GetSSEConnection(ctx context.Context, id string) (*SSEConnection, error) {
	resp, err := c.get(ctx, "/sse/connections/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var conn SSEConnection
	if err := json.NewDecoder(resp.Body).Decode(&conn); err != nil {
		return nil, fmt.Errorf("failed to decode SSE connection: %w", err)
	}
	return &conn, nil
}

// CloseSSEConnection closes an SSE connection.
func (c *Client) CloseSSEConnection(ctx context.Context, id string) error {
	resp, err := c.delete(ctx, "/sse/connections/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}
	return nil
}

// GetSSEStats returns SSE statistics.
func (c *Client) GetSSEStats(ctx context.Context) (*SSEStats, error) {
	resp, err := c.get(ctx, "/sse/stats")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var stats SSEStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode SSE stats: %w", err)
	}
	return &stats, nil
}

// HTTP helpers

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *Client) put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
		return fmt.Errorf("%s: %s", errResp.Error, errResp.Message)
	}
	return fmt.Errorf("request failed: status %d", resp.StatusCode)
}
