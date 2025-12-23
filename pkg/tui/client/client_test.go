package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/recording"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:9090")
	if c.baseURL != "http://localhost:9090" {
		t.Errorf("expected baseURL to be http://localhost:9090, got %s", c.baseURL)
	}
}

func TestNewDefaultClient(t *testing.T) {
	c := NewDefaultClient()
	if c.baseURL != "http://localhost:9090" {
		t.Errorf("expected baseURL to be http://localhost:9090, got %s", c.baseURL)
	}
}

func TestGetHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected path /health, got %s", r.URL.Path)
		}
		resp := admin.HealthResponse{Status: "ok", Uptime: 100}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	health, err := c.GetHealth()
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", health.Status)
	}
	if health.Uptime != 100 {
		t.Errorf("expected uptime 100, got %d", health.Uptime)
	}
}

func TestListMocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mocks" {
			t.Errorf("expected path /mocks, got %s", r.URL.Path)
		}
		resp := admin.MockListResponse{
			Mocks: []*config.MockConfiguration{
				{
					ID:      "mock1",
					Matcher: &config.RequestMatcher{Method: "GET", Path: "/test"},
				},
			},
			Count: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	mocks, err := c.ListMocks()
	if err != nil {
		t.Fatalf("ListMocks failed: %v", err)
	}
	if len(mocks) != 1 {
		t.Errorf("expected 1 mock, got %d", len(mocks))
	}
	if mocks[0].ID != "mock1" {
		t.Errorf("expected mock ID 'mock1', got %s", mocks[0].ID)
	}
}

func TestGetMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mocks/mock1" {
			t.Errorf("expected path /mocks/mock1, got %s", r.URL.Path)
		}
		mock := config.MockConfiguration{
			ID:      "mock1",
			Matcher: &config.RequestMatcher{Method: "GET", Path: "/test"},
		}
		json.NewEncoder(w).Encode(mock)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	mock, err := c.GetMock("mock1")
	if err != nil {
		t.Fatalf("GetMock failed: %v", err)
	}
	if mock.ID != "mock1" {
		t.Errorf("expected mock ID 'mock1', got %s", mock.ID)
	}
}

func TestCreateMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/mocks" {
			t.Errorf("expected path /mocks, got %s", r.URL.Path)
		}
		var mock config.MockConfiguration
		json.NewDecoder(r.Body).Decode(&mock)
		mock.ID = "new-mock"
		json.NewEncoder(w).Encode(mock)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	newMock := &config.MockConfiguration{
		Matcher: &config.RequestMatcher{Method: "POST", Path: "/create"},
	}
	result, err := c.CreateMock(newMock)
	if err != nil {
		t.Fatalf("CreateMock failed: %v", err)
	}
	if result.ID != "new-mock" {
		t.Errorf("expected mock ID 'new-mock', got %s", result.ID)
	}
}

func TestDeleteMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/mocks/mock1" {
			t.Errorf("expected path /mocks/mock1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.DeleteMock("mock1")
	if err != nil {
		t.Fatalf("DeleteMock failed: %v", err)
	}
}

func TestToggleMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/mocks/mock1/toggle" {
			t.Errorf("expected path /mocks/mock1/toggle, got %s", r.URL.Path)
		}
		var req admin.ToggleRequest
		json.NewDecoder(r.Body).Decode(&req)
		mock := config.MockConfiguration{ID: "mock1", Enabled: req.Enabled}
		json.NewEncoder(w).Encode(mock)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	result, err := c.ToggleMock("mock1", true)
	if err != nil {
		t.Fatalf("ToggleMock failed: %v", err)
	}
	if !result.Enabled {
		t.Errorf("expected mock to be enabled")
	}
}

func TestGetTraffic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/requests" {
			t.Errorf("expected path /requests, got %s", r.URL.Path)
		}
		resp := admin.RequestLogListResponse{
			Requests: []*config.RequestLogEntry{
				{ID: "req1", Method: "GET", Path: "/test"},
			},
			Count: 1,
			Total: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	entries, err := c.GetTraffic(nil)
	if err != nil {
		t.Fatalf("GetTraffic failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestListStreamRecordings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stream-recordings" {
			t.Errorf("expected path /stream-recordings, got %s", r.URL.Path)
		}
		resp := admin.StreamRecordingListResponse{
			Recordings: []*recording.RecordingSummary{
				{ID: "rec1", Protocol: "websocket"},
			},
			Total: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	recordings, err := c.ListStreamRecordings(nil)
	if err != nil {
		t.Fatalf("ListStreamRecordings failed: %v", err)
	}
	if len(recordings) != 1 {
		t.Errorf("expected 1 recording, got %d", len(recordings))
	}
}

func TestGetProxyStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/status" {
			t.Errorf("expected path /proxy/status, got %s", r.URL.Path)
		}
		resp := admin.ProxyStatusResponse{
			Running: true,
			Port:    8888,
			Mode:    "record",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	status, err := c.GetProxyStatus()
	if err != nil {
		t.Fatalf("GetProxyStatus failed: %v", err)
	}
	if !status.Running {
		t.Errorf("expected proxy to be running")
	}
	if status.Port != 8888 {
		t.Errorf("expected port 8888, got %d", status.Port)
	}
}

func TestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := admin.ErrorResponse{
			Error:   "not_found",
			Message: "Mock not found",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	_, err := c.GetMock("nonexistent")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "not_found: Mock not found" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := admin.HealthResponse{Status: "ok", Uptime: 0}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}
