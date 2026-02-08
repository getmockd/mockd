package testing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// MockServer is a test helper for running mockd in tests.
// It provides a fluent API for configuring mock endpoints and assertions.
type MockServer struct {
	t            testing.TB
	server       *engine.Server
	engineClient *engineclient.Client // HTTP client for engine control API
	httpSrv      *httptest.Server
	mocks        []*config.MockConfiguration
	mocksMu      sync.RWMutex
	started      bool
	baseURL      string
	controlURL   string         // URL for engine control API
	timesLimit   map[string]int // mock ID -> remaining times
	timesMu      sync.RWMutex
}

// New creates a new mock server for testing.
// The mock server will be automatically cleaned up when the test completes.
func New(t testing.TB) *MockServer {
	t.Helper()
	return &MockServer{
		t:          t,
		mocks:      make([]*config.MockConfiguration, 0),
		timesLimit: make(map[string]int),
	}
}

// Start starts the mock server and returns the base URL.
// This must be called after configuring mocks.
func (m *MockServer) Start() string {
	m.t.Helper()

	if m.started {
		return m.baseURL
	}

	// Create server configuration for testing
	cfg := &config.ServerConfiguration{
		HTTPPort:       0, // Use random port
		ManagementPort: 0, // Use random port for management API
		LogRequests:    true,
		MaxLogEntries:  1000,
	}

	// Create the mock server with configured mocks
	m.mocksMu.RLock()
	mocks := make([]*config.MockConfiguration, len(m.mocks))
	copy(mocks, m.mocks)
	m.mocksMu.RUnlock()

	m.server = engine.NewServerWithMocks(cfg, mocks)

	// Create an httptest server that wraps the engine handler
	m.httpSrv = httptest.NewServer(m.wrapHandler(m.server.Handler()))
	m.baseURL = m.httpSrv.URL

	// Start the actual engine to enable the control API
	if err := m.server.Start(); err != nil {
		m.t.Fatalf("failed to start engine: %v", err)
	}

	// Create HTTP client for engine management API
	managementPort := m.server.ManagementPort()
	m.controlURL = fmt.Sprintf("http://localhost:%d", managementPort)
	m.engineClient = engineclient.New(m.controlURL)

	m.started = true

	return m.baseURL
}

// wrapHandler wraps the engine handler with times limit checking
func (m *MockServer) wrapHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check times limits before forwarding
		m.timesMu.Lock()
		for id, remaining := range m.timesLimit {
			if remaining <= 0 && m.engineClient != nil {
				// Find and disable the mock via HTTP
				mockCfg, err := m.engineClient.GetMock(context.Background(), id)
				if err == nil && mockCfg != nil && (mockCfg.Enabled == nil || *mockCfg.Enabled) {
					disabled := false
					mockCfg.Enabled = &disabled
					_, _ = m.engineClient.UpdateMock(context.Background(), id, mockCfg)
				}
			}
		}
		m.timesMu.Unlock()

		// Forward to engine handler
		h.ServeHTTP(w, r)

		// Decrement times counter for matched mock
		logs := m.server.GetRequestLogs(nil)
		if len(logs) > 0 {
			lastLog := logs[0]
			if lastLog.MatchedMockID != "" {
				m.timesMu.Lock()
				if _, ok := m.timesLimit[lastLog.MatchedMockID]; ok {
					m.timesLimit[lastLog.MatchedMockID]--
				}
				m.timesMu.Unlock()
			}
		}
	})
}

// Stop stops the mock server.
// This should be called with defer after New().
func (m *MockServer) Stop() {
	m.t.Helper()

	if m.httpSrv != nil {
		m.httpSrv.Close()
	}
	if m.server != nil {
		_ = m.server.Stop()
	}
	m.started = false
}

// URL returns the base URL of the mock server.
// Returns empty string if the server is not started.
func (m *MockServer) URL() string {
	return m.baseURL
}

// Mock adds a mock endpoint and returns a builder for configuration.
// Use the builder's fluent methods to configure the mock response.
//
// Example:
//
//	mock.Mock("GET", "/users/123").
//	    WithStatus(200).
//	    WithBody(`{"id": "123"}`).
//	    Reply()
func (m *MockServer) Mock(method, path string) *MockBuilder {
	m.t.Helper()

	enabled := true
	mockCfg := &config.MockConfiguration{
		Type:    mock.TypeHTTP,
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: method,
				Path:   path,
			},
			Response: &mock.HTTPResponse{
				StatusCode: http.StatusOK,
			},
		},
	}

	return &MockBuilder{
		server: m,
		mock:   mockCfg,
	}
}

// Reset clears all mocks and request logs.
// Use this between test cases to start fresh.
func (m *MockServer) Reset() {
	m.t.Helper()

	m.mocksMu.Lock()
	m.mocks = make([]*config.MockConfiguration, 0)
	m.mocksMu.Unlock()

	m.timesMu.Lock()
	m.timesLimit = make(map[string]int)
	m.timesMu.Unlock()

	if m.engineClient != nil {
		// Clear existing mocks from the server via HTTP
		ctx := context.Background()
		mocks, err := m.engineClient.ListMocks(ctx)
		if err == nil {
			for _, mockCfg := range mocks {
				_ = m.engineClient.DeleteMock(ctx, mockCfg.ID)
			}
		}
		// Clear request logs via HTTP
		_, _ = m.engineClient.ClearRequests(ctx)
	}
}

// Requests returns all logged requests for assertions.
// Requests are returned in reverse chronological order (newest first).
func (m *MockServer) Requests() []RequestLog {
	m.t.Helper()

	if m.server == nil {
		return nil
	}

	logs := m.server.GetRequestLogs(nil)
	result := make([]RequestLog, len(logs))

	for i, log := range logs {
		headers := make(map[string]string)
		for k, v := range log.Headers {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		result[i] = RequestLog{
			Method:      log.Method,
			Path:        log.Path,
			Headers:     headers,
			Body:        log.Body,
			QueryString: log.QueryString,
			MatchedID:   log.MatchedMockID,
		}
	}

	return result
}

// AssertCalled asserts that an endpoint was called at least once.
func (m *MockServer) AssertCalled(t testing.TB, method, path string) {
	t.Helper()

	count := m.countCalls(method, path)
	if count == 0 {
		t.Errorf("expected %s %s to be called, but it was not called", method, path)
	}
}

// AssertCalledTimes asserts that an endpoint was called exactly n times.
func (m *MockServer) AssertCalledTimes(t testing.TB, method, path string, times int) {
	t.Helper()

	count := m.countCalls(method, path)
	if count != times {
		t.Errorf("expected %s %s to be called %d times, but was called %d times",
			method, path, times, count)
	}
}

// AssertNotCalled asserts that an endpoint was not called.
func (m *MockServer) AssertNotCalled(t testing.TB, method, path string) {
	t.Helper()

	count := m.countCalls(method, path)
	if count > 0 {
		t.Errorf("expected %s %s to not be called, but it was called %d times",
			method, path, count)
	}
}

// countCalls counts how many times a method/path combination was called.
func (m *MockServer) countCalls(method, path string) int {
	if m.server == nil {
		return 0
	}

	logs := m.server.GetRequestLogs(&engine.RequestLogFilter{
		Method: method,
	})

	count := 0
	for _, log := range logs {
		if matchesPath(log.Path, path) {
			count++
		}
	}
	return count
}

// matchesPath checks if a request path matches the expected path pattern.
// Supports exact matching and path parameters ({id} patterns).
func matchesPath(actual, expected string) bool {
	if actual == expected {
		return true
	}

	// Handle path parameters
	actualParts := strings.Split(actual, "/")
	expectedParts := strings.Split(expected, "/")

	if len(actualParts) != len(expectedParts) {
		return false
	}

	for i := range expectedParts {
		exp := expectedParts[i]
		act := actualParts[i]

		// Check for path parameter pattern {name}
		if strings.HasPrefix(exp, "{") && strings.HasSuffix(exp, "}") {
			continue // Any value matches a path parameter
		}

		if exp != act {
			return false
		}
	}

	return true
}

// addMock adds a mock configuration to the server.
// Called internally by MockBuilder.Reply().
func (m *MockServer) addMock(mockCfg *config.MockConfiguration) {
	m.mocksMu.Lock()
	m.mocks = append(m.mocks, mockCfg)
	m.mocksMu.Unlock()

	// If server is already started, add mock dynamically via HTTP
	if m.engineClient != nil {
		_, _ = m.engineClient.CreateMock(context.Background(), mockCfg)
	}
}

// setTimesLimit sets the times limit for a mock.
func (m *MockServer) setTimesLimit(mockID string, times int) {
	m.timesMu.Lock()
	m.timesLimit[mockID] = times
	m.timesMu.Unlock()
}

// Client returns an http.Client configured to work with the mock server.
// This is a convenience method for tests.
func (m *MockServer) Client() *http.Client {
	if m.httpSrv != nil {
		return m.httpSrv.Client()
	}
	return http.DefaultClient
}

// Server returns the underlying engine.Server for advanced use cases.
// Most tests should not need this.
func (m *MockServer) Server() *engine.Server {
	return m.server
}
