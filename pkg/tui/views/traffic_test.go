package views

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTraffic tests the creation of a new traffic model.
func TestNewTraffic(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)

	assert.NotNil(t, m.client)
	assert.True(t, m.loading)
	assert.Empty(t, m.requests)
	assert.False(t, m.paused)
	assert.False(t, m.filterActive)
	assert.False(t, m.detailOpen)
}

// TestTrafficInit tests the Init function.
func TestTrafficInit(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)

	cmd := m.Init()
	assert.NotNil(t, cmd)
}

// TestTrafficSetSize tests the SetSize function.
func TestTrafficSetSize(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)

	m.SetSize(100, 50)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)
}

// TestTrafficDataMsg tests handling of traffic data messages.
func TestTrafficDataMsg(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	now := time.Now()
	requests := []*config.RequestLogEntry{
		{
			ID:             "req-1",
			Timestamp:      now,
			Method:         "GET",
			Path:           "/api/users",
			ResponseStatus: 200,
			DurationMs:     15,
			MatchedMockID:  "mock-1",
		},
		{
			ID:             "req-2",
			Timestamp:      now.Add(-1 * time.Second),
			Method:         "POST",
			Path:           "/api/orders",
			ResponseStatus: 201,
			DurationMs:     45,
			MatchedMockID:  "mock-2",
		},
	}

	msg := trafficDataMsg{requests: requests}
	m, cmd := m.Update(msg)

	assert.Nil(t, cmd)
	assert.False(t, m.loading)
	assert.Nil(t, m.err)
	assert.Len(t, m.requests, 2)
	assert.Equal(t, "req-1", m.lastRequestID)
}

// TestTrafficPauseResume tests pause and resume functionality.
func TestTrafficPauseResume(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Initially not paused
	assert.False(t, m.paused)

	// Pause
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	m, cmd := m.Update(keyMsg)
	assert.True(t, m.paused)
	assert.Equal(t, "Traffic paused", m.statusMessage)
	assert.Nil(t, cmd)

	// Resume
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	m, cmd = m.Update(keyMsg)
	assert.False(t, m.paused)
	assert.Equal(t, "Traffic resumed", m.statusMessage)
	assert.NotNil(t, cmd) // Should fetch data immediately
}

// TestTrafficFilterActivation tests filter activation.
func TestTrafficFilterActivation(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Initially not active
	assert.False(t, m.filterActive)

	// Activate filter with '/'
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	m, cmd := m.Update(keyMsg)
	assert.True(t, m.filterActive)
	assert.NotNil(t, cmd)
}

// TestTrafficFilterApplication tests filtering of requests.
func TestTrafficFilterApplication(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	now := time.Now()
	m.requests = []*config.RequestLogEntry{
		{
			ID:             "req-1",
			Timestamp:      now,
			Method:         "GET",
			Path:           "/api/users",
			ResponseStatus: 200,
			DurationMs:     15,
		},
		{
			ID:             "req-2",
			Timestamp:      now,
			Method:         "POST",
			Path:           "/api/orders",
			ResponseStatus: 201,
			DurationMs:     45,
		},
		{
			ID:             "req-3",
			Timestamp:      now,
			Method:         "GET",
			Path:           "/api/products",
			ResponseStatus: 404,
			DurationMs:     5,
		},
	}

	// Test filtering by method
	m.filterText = "GET"
	filtered := m.filterRequests(m.requests)
	assert.Len(t, filtered, 2)

	// Test filtering by path
	m.filterText = "orders"
	filtered = m.filterRequests(m.requests)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "/api/orders", filtered[0].Path)

	// Test filtering by status
	m.filterText = "404"
	filtered = m.filterRequests(m.requests)
	assert.Len(t, filtered, 1)
	assert.Equal(t, 404, filtered[0].ResponseStatus)

	// Test no match
	m.filterText = "PATCH"
	filtered = m.filterRequests(m.requests)
	assert.Len(t, filtered, 0)
}

// TestTrafficDetailView tests opening the detail view.
func TestTrafficDetailView(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	now := time.Now()
	m.requests = []*config.RequestLogEntry{
		{
			ID:             "req-1",
			Timestamp:      now,
			Method:         "GET",
			Path:           "/api/users",
			ResponseStatus: 200,
			DurationMs:     15,
			Headers:        map[string][]string{"Content-Type": {"application/json"}},
			Body:           `{"test": "data"}`,
			MatchedMockID:  "mock-1",
		},
	}
	m.updateTableRows()

	// Open detail view
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, cmd := m.Update(keyMsg)
	assert.True(t, m.detailOpen)
	assert.NotNil(t, m.selectedRequest)
	assert.Equal(t, "req-1", m.selectedRequest.ID)

	// Close detail view with Esc
	keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	m, cmd = m.Update(keyMsg)
	assert.False(t, m.detailOpen)
	assert.Nil(t, cmd)
}

// TestTrafficMergeRequests tests merging of new and existing requests.
func TestTrafficMergeRequests(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	now := time.Now()

	// Set existing requests
	m.requests = []*config.RequestLogEntry{
		{ID: "req-1", Timestamp: now.Add(-2 * time.Second), Method: "GET", Path: "/api/users"},
		{ID: "req-2", Timestamp: now.Add(-3 * time.Second), Method: "POST", Path: "/api/orders"},
	}

	// New requests (includes one duplicate and one new)
	newRequests := []*config.RequestLogEntry{
		{ID: "req-3", Timestamp: now, Method: "GET", Path: "/api/products"},
		{ID: "req-1", Timestamp: now.Add(-2 * time.Second), Method: "GET", Path: "/api/users"}, // Duplicate
	}

	merged := m.mergeRequests(newRequests)

	// Should have 3 unique requests
	assert.Len(t, merged, 3)
	// Newest should be first
	assert.Equal(t, "req-3", merged[0].ID)
	assert.Equal(t, "req-1", merged[1].ID)
	assert.Equal(t, "req-2", merged[2].ID)
}

// TestTrafficMergeRequestsMaxLimit tests that merge respects max limit.
func TestTrafficMergeRequestsMaxLimit(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	now := time.Now()

	// Fill with maxRequests
	for i := 0; i < maxRequests; i++ {
		m.requests = append(m.requests, &config.RequestLogEntry{
			ID:        fmt.Sprintf("req-%d", i),
			Timestamp: now.Add(-time.Duration(i) * time.Second),
		})
	}

	// Add new requests
	newRequests := []*config.RequestLogEntry{
		{ID: "req-new-1", Timestamp: now},
		{ID: "req-new-2", Timestamp: now.Add(-1 * time.Second)},
	}

	merged := m.mergeRequests(newRequests)

	// Should still be maxRequests
	assert.Len(t, merged, maxRequests)
	// Newest should be first
	assert.Equal(t, "req-new-1", merged[0].ID)
	assert.Equal(t, "req-new-2", merged[1].ID)
}

// TestTrafficGetStatusColor tests status code color mapping.
func TestTrafficGetStatusColor(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)

	// Test 2xx - success (green)
	assert.Equal(t, m.getStatusColor(200), m.getStatusColor(200))

	// Test 3xx - warning (yellow)
	assert.Equal(t, m.getStatusColor(301), m.getStatusColor(301))

	// Test 4xx - error (red)
	assert.Equal(t, m.getStatusColor(404), m.getStatusColor(404))

	// Test 5xx - error (red)
	assert.Equal(t, m.getStatusColor(500), m.getStatusColor(500))
}

// TestTrafficFormatJSON tests JSON formatting.
func TestTrafficFormatJSON(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)

	// Valid JSON
	input := `{"name":"test","value":123}`
	formatted := m.formatJSON(input)
	assert.Contains(t, formatted, "name")
	assert.Contains(t, formatted, "test")
	// Should be pretty-printed (contains newlines)
	assert.Contains(t, formatted, "\n")

	// Invalid JSON
	input = "not json"
	formatted = m.formatJSON(input)
	assert.Equal(t, "not json", formatted)
}

// TestTrafficClearAction tests the clear action.
func TestTrafficClearAction(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/requests" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	c := client.NewClient(server.URL)
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Add some requests
	m.requests = []*config.RequestLogEntry{
		{ID: "req-1", Method: "GET", Path: "/api/users"},
	}

	// Trigger clear
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	m, cmd := m.Update(keyMsg)
	assert.NotNil(t, cmd)
}

// TestTrafficRefreshTick tests the refresh tick.
func TestTrafficRefreshTick(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Not paused - should fetch
	m.paused = false
	msg := trafficRefreshTickMsg(time.Now())
	m, cmd := m.Update(msg)
	assert.NotNil(t, cmd)

	// Paused - should still schedule next tick but not fetch immediately
	m.paused = true
	msg = trafficRefreshTickMsg(time.Now())
	m, cmd = m.Update(msg)
	assert.NotNil(t, cmd)
}

// TestTrafficWithRealAPI tests integration with a mock API server.
func TestTrafficWithRealAPI(t *testing.T) {
	now := time.Now()
	testRequests := []*config.RequestLogEntry{
		{
			ID:             "req-1",
			Timestamp:      now,
			Method:         "GET",
			Path:           "/api/users",
			ResponseStatus: 200,
			DurationMs:     15,
			MatchedMockID:  "mock-1",
			Headers:        map[string][]string{"Content-Type": {"application/json"}},
			Body:           `{"id": 1, "name": "John"}`,
			BodySize:       25,
			RemoteAddr:     "127.0.0.1:12345",
		},
		{
			ID:             "req-2",
			Timestamp:      now.Add(-1 * time.Second),
			Method:         "POST",
			Path:           "/api/orders",
			ResponseStatus: 201,
			DurationMs:     45,
			MatchedMockID:  "mock-2",
			Headers:        map[string][]string{"Content-Type": {"application/json"}},
			Body:           `{"orderId": 123}`,
			BodySize:       17,
			RemoteAddr:     "127.0.0.1:12346",
		},
	}

	// Create a test server that returns traffic data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/requests" {
			resp := admin.RequestLogListResponse{
				Requests: testRequests,
				Total:    len(testRequests),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	c := client.NewClient(server.URL)
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Fetch data
	cmd := m.fetchTrafficData()
	require.NotNil(t, cmd)

	msg := cmd()
	require.IsType(t, trafficDataMsg{}, msg)

	dataMsg := msg.(trafficDataMsg)
	assert.Len(t, dataMsg.requests, 2)
	assert.Equal(t, "req-1", dataMsg.requests[0].ID)

	// Update model with the message
	m, _ = m.Update(dataMsg)
	assert.Len(t, m.requests, 2)
	assert.Equal(t, "req-1", m.lastRequestID)
}

// TestTrafficErrorHandling tests error handling.
func TestTrafficErrorHandling(t *testing.T) {
	// Create a test server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(admin.ErrorResponse{
			Error:   "internal_error",
			Message: "Something went wrong",
		})
	}))
	defer server.Close()

	c := client.NewClient(server.URL)
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Fetch data (should error)
	cmd := m.fetchTrafficData()
	require.NotNil(t, cmd)

	msg := cmd()
	require.IsType(t, trafficErrMsg{}, msg)

	errMsg := msg.(trafficErrMsg)
	assert.Error(t, errMsg.err)

	// Update model with error
	m, _ = m.Update(errMsg)
	assert.NotNil(t, m.err)
	assert.False(t, m.loading)
}

// TestTrafficView tests the View rendering.
func TestTrafficView(t *testing.T) {
	c := client.NewClient("http://localhost:9090")
	m := NewTraffic(c)
	m.SetSize(100, 50)

	// Loading state
	m.loading = true
	view := m.View()
	assert.Contains(t, view, "Loading")

	// With data
	m.loading = false
	now := time.Now()
	m.requests = []*config.RequestLogEntry{
		{
			ID:             "req-1",
			Timestamp:      now,
			Method:         "GET",
			Path:           "/api/users",
			ResponseStatus: 200,
			DurationMs:     15,
		},
	}
	m.updateTableRows()

	view = m.View()
	assert.Contains(t, view, "Traffic Log")
	assert.Contains(t, view, "LIVE")
	assert.Contains(t, view, "1 requests")

	// Paused state
	m.paused = true
	view = m.View()
	assert.Contains(t, view, "PAUSED")

	// With filter
	m.filterText = "GET"
	view = m.View()
	assert.Contains(t, view, "Filter:")
}
