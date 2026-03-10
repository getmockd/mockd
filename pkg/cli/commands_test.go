package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
)

// =============================================================================
// extractMockDetails tests
// =============================================================================

func TestExtractMockDetails_HTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mock       *mock.Mock
		wantPath   string
		wantMethod string
		wantStatus int
	}{
		{
			name: "HTTP mock with path and method",
			mock: &mock.Mock{
				Type: mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/api/users",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
					},
				},
			},
			wantPath:   "/api/users",
			wantMethod: "GET",
			wantStatus: 200,
		},
		{
			name: "HTTP mock with pathPattern fallback",
			mock: &mock.Mock{
				Type: mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method:      "POST",
						PathPattern: "^/api/v[0-9]+/.*",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 201,
					},
				},
			},
			wantPath:   "^/api/v[0-9]+/.*",
			wantMethod: "POST",
			wantStatus: 201,
		},
		{
			name: "HTTP mock with nil HTTP spec",
			mock: &mock.Mock{
				Type: mock.TypeHTTP,
				HTTP: nil,
			},
			wantPath:   "",
			wantMethod: "",
			wantStatus: 0,
		},
		{
			name: "HTTP mock with nil matcher",
			mock: &mock.Mock{
				Type: mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher:  nil,
					Response: &mock.HTTPResponse{StatusCode: 204},
				},
			},
			wantPath:   "",
			wantMethod: "",
			wantStatus: 204,
		},
		{
			name: "HTTP mock with nil response",
			mock: &mock.Mock{
				Type: mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "DELETE",
						Path:   "/api/items",
					},
					Response: nil,
				},
			},
			wantPath:   "/api/items",
			wantMethod: "DELETE",
			wantStatus: 0,
		},
		{
			name: "empty type defaults to HTTP",
			mock: &mock.Mock{
				Type: "",
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/default",
					},
					Response: &mock.HTTPResponse{StatusCode: 200},
				},
			},
			wantPath:   "/default",
			wantMethod: "GET",
			wantStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, method, status := extractMockDetails(tt.mock)
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

func TestExtractMockDetails_WebSocket(t *testing.T) {
	t.Parallel()

	m := &mock.Mock{
		Type:      mock.TypeWebSocket,
		WebSocket: &mock.WebSocketSpec{Path: "/ws/chat"},
	}

	path, method, status := extractMockDetails(m)
	if path != "/ws/chat" {
		t.Errorf("path = %q, want /ws/chat", path)
	}
	if method != "WS" {
		t.Errorf("method = %q, want WS", method)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
}

func TestExtractMockDetails_GraphQL(t *testing.T) {
	t.Parallel()

	m := &mock.Mock{
		Type:    mock.TypeGraphQL,
		GraphQL: &mock.GraphQLSpec{Path: "/graphql"},
	}

	path, method, status := extractMockDetails(m)
	if path != "/graphql" {
		t.Errorf("path = %q, want /graphql", path)
	}
	if method != "GQL" {
		t.Errorf("method = %q, want GQL", method)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
}

func TestExtractMockDetails_GRPC(t *testing.T) {
	t.Parallel()

	m := &mock.Mock{
		Type: mock.TypeGRPC,
		GRPC: &mock.GRPCSpec{Port: 50051},
	}

	path, method, status := extractMockDetails(m)
	if path != ":50051" {
		t.Errorf("path = %q, want :50051", path)
	}
	if method != "gRPC" {
		t.Errorf("method = %q, want gRPC", method)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
}

func TestExtractMockDetails_MQTT(t *testing.T) {
	t.Parallel()

	m := &mock.Mock{
		Type: mock.TypeMQTT,
		MQTT: &mock.MQTTSpec{Port: 1883},
	}

	path, method, status := extractMockDetails(m)
	if path != ":1883" {
		t.Errorf("path = %q, want :1883", path)
	}
	if method != "MQTT" {
		t.Errorf("method = %q, want MQTT", method)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
}

func TestExtractMockDetails_SOAP(t *testing.T) {
	t.Parallel()

	m := &mock.Mock{
		Type: mock.TypeSOAP,
		SOAP: &mock.SOAPSpec{Path: "/soap/service"},
	}

	path, method, status := extractMockDetails(m)
	if path != "/soap/service" {
		t.Errorf("path = %q, want /soap/service", path)
	}
	if method != "SOAP" {
		t.Errorf("method = %q, want SOAP", method)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
}

func TestExtractMockDetails_NilSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mock *mock.Mock
	}{
		{name: "nil websocket spec", mock: &mock.Mock{Type: mock.TypeWebSocket}},
		{name: "nil graphql spec", mock: &mock.Mock{Type: mock.TypeGraphQL}},
		{name: "nil grpc spec", mock: &mock.Mock{Type: mock.TypeGRPC}},
		{name: "nil soap spec", mock: &mock.Mock{Type: mock.TypeSOAP}},
		{name: "nil mqtt spec", mock: &mock.Mock{Type: mock.TypeMQTT}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, method, status := extractMockDetails(tt.mock)
			if path != "" {
				t.Errorf("path = %q, want empty", path)
			}
			if method != "" {
				t.Errorf("method = %q, want empty", method)
			}
			if status != 0 {
				t.Errorf("status = %d, want 0", status)
			}
		})
	}
}

// =============================================================================
// deleteByID tests
// =============================================================================

func TestDeleteByID_ExactMatch(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/mocks/http_abc123" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": "mock not found"})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)

	// Capture stdout to suppress printResult output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := deleteByID(client, "http_abc123")

	w.Close()
	os.Stdout = oldStdout
	// Drain the reader to avoid pipe issues
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("deleteByID() returned error: %v", err)
	}
}

func TestDeleteByID_404FallsToPrefix(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	var deletedID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/mocks/http_ab":
			// First attempt: exact match → 404
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": "mock not found"})
		case r.Method == http.MethodGet && r.URL.Path == "/mocks":
			// ListMocks for prefix search
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":   "http_abc123",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
					{
						"id":   "http_xyz789",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "POST", "path": "/api/items"},
							"response": map[string]interface{}{"statusCode": 201},
						},
					},
				},
				"count": 2,
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/mocks/http_abc123":
			// Prefix matched → delete the single match
			deletedID = "http_abc123"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := deleteByID(client, "http_ab")

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("deleteByID() returned error: %v", err)
	}
	if deletedID != "http_abc123" {
		t.Errorf("expected mock http_abc123 to be deleted, got %q", deletedID)
	}
}

// =============================================================================
// deleteByPrefix tests
// =============================================================================

func TestDeleteByPrefix_NoMatches(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mocks": []map[string]interface{}{
				{
					"id":   "http_abc123",
					"type": "http",
					"http": map[string]interface{}{
						"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
						"response": map[string]interface{}{"statusCode": 200},
					},
				},
			},
			"count": 1,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := deleteByPrefix(client, "nomatch_")

	if err == nil {
		t.Fatal("expected error for no matches, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should contain 'not found', got: %v", err)
	}
}

func TestDeleteByPrefix_AmbiguousMatches(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mocks": []map[string]interface{}{
				{
					"id":   "http_abc123",
					"type": "http",
					"http": map[string]interface{}{
						"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
						"response": map[string]interface{}{"statusCode": 200},
					},
				},
				{
					"id":   "http_abc456",
					"type": "http",
					"http": map[string]interface{}{
						"matcher":  map[string]interface{}{"method": "POST", "path": "/api/users"},
						"response": map[string]interface{}{"statusCode": 201},
					},
				},
			},
			"count": 2,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)

	// Capture stderr since deleteByPrefix writes to stderr for ambiguous matches
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	err := deleteByPrefix(client, "http_abc")

	wErr.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected error for ambiguous matches, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should contain 'ambiguous', got: %v", err)
	}
}

func TestDeleteByPrefix_SingleMatch(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	var deletedID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mocks":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":   "http_abc123",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
					{
						"id":   "http_xyz789",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "POST", "path": "/api/items"},
							"response": map[string]interface{}{"statusCode": 201},
						},
					},
				},
				"count": 2,
			})
		case r.Method == http.MethodDelete:
			deletedID = strings.TrimPrefix(r.URL.Path, "/mocks/")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := deleteByPrefix(client, "http_abc")

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("deleteByPrefix() returned error: %v", err)
	}
	if deletedID != "http_abc123" {
		t.Errorf("expected mock http_abc123 to be deleted, got %q", deletedID)
	}
}

// =============================================================================
// deleteByPath tests
// =============================================================================

func TestDeleteByPath_SingleMatch(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	var deletedID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mocks":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":   "http_abc123",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api/hello"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
					{
						"id":   "http_xyz789",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "POST", "path": "/api/items"},
							"response": map[string]interface{}{"statusCode": 201},
						},
					},
				},
				"count": 2,
			})
		case r.Method == http.MethodDelete:
			deletedID = strings.TrimPrefix(r.URL.Path, "/mocks/")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := deleteByPath(client, "/api/hello", "", false)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("deleteByPath() returned error: %v", err)
	}
	if deletedID != "http_abc123" {
		t.Errorf("expected mock http_abc123 to be deleted, got %q", deletedID)
	}
}

func TestDeleteByPath_NoMatch(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mocks": []map[string]interface{}{
				{
					"id":   "http_abc123",
					"type": "http",
					"http": map[string]interface{}{
						"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
						"response": map[string]interface{}{"statusCode": 200},
					},
				},
			},
			"count": 1,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := deleteByPath(client, "/api/nonexistent", "", false)

	if err == nil {
		t.Fatal("expected error for no matching path, got nil")
	}
	if !strings.Contains(err.Error(), "no mocks found") {
		t.Errorf("error should contain 'no mocks found', got: %v", err)
	}
}

func TestDeleteByPath_NoMatchWithMethod(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mocks": []map[string]interface{}{
				{
					"id":   "http_abc123",
					"type": "http",
					"http": map[string]interface{}{
						"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
						"response": map[string]interface{}{"statusCode": 200},
					},
				},
			},
			"count": 1,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := deleteByPath(client, "/api/users", "POST", false)

	if err == nil {
		t.Fatal("expected error for no matching method, got nil")
	}
	if !strings.Contains(err.Error(), "POST") {
		t.Errorf("error should mention the method POST, got: %v", err)
	}
}

func TestDeleteByPath_MethodFilter(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	var deletedID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mocks":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":   "http_get1",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api/users"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
					{
						"id":   "http_post1",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "POST", "path": "/api/users"},
							"response": map[string]interface{}{"statusCode": 201},
						},
					},
				},
				"count": 2,
			})
		case r.Method == http.MethodDelete:
			deletedID = strings.TrimPrefix(r.URL.Path, "/mocks/")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)

	// Capture stdout
	oldStdout := os.Stdout
	rd, wr, _ := os.Pipe()
	os.Stdout = wr

	err := deleteByPath(client, "/api/users", "post", false)

	wr.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(rd)

	if err != nil {
		t.Fatalf("deleteByPath() returned error: %v", err)
	}
	if deletedID != "http_post1" {
		t.Errorf("expected mock http_post1 to be deleted, got %q", deletedID)
	}
}

// =============================================================================
// runList tests (via outputMocksTable)
// =============================================================================

func TestOutputMocksTable_EmptyList(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputMocksTable(nil, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No mocks configured") {
		t.Errorf("expected 'No mocks configured', got: %s", output)
	}
}

func TestOutputMocksTable_WithMocks(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	enabled := true
	mocks := []*mock.Mock{
		{
			ID:      "http_abc123def456",
			Type:    mock.TypeHTTP,
			Enabled: &enabled,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/api/users"},
				Response: &mock.HTTPResponse{StatusCode: 200},
			},
		},
		{
			ID:      "ws_xyz789",
			Type:    mock.TypeWebSocket,
			Enabled: &enabled,
			WebSocket: &mock.WebSocketSpec{
				Path: "/ws/events",
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputMocksTable(mocks, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain table header
	if !strings.Contains(output, "ID") || !strings.Contains(output, "TYPE") {
		t.Error("output should contain table headers ID and TYPE")
	}

	// Should show the HTTP mock
	if !strings.Contains(output, "http") {
		t.Errorf("output should contain http type, got: %s", output)
	}

	// Should show the WebSocket mock
	if !strings.Contains(output, "websocket") {
		t.Errorf("output should contain websocket type, got: %s", output)
	}

	// Status for WS should be "-"
	if !strings.Contains(output, "WS") {
		t.Errorf("output should contain WS method for websocket, got: %s", output)
	}
}

func TestOutputMocksTable_Truncation(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	enabled := true
	longID := "http_abcdefghijklmnopqrstuvwxyz"       // 30 chars, > 20
	longPath := "/api/very/long/path/that/exceeds/25" // > 25 chars

	mocks := []*mock.Mock{
		{
			ID:      longID,
			Type:    mock.TypeHTTP,
			Enabled: &enabled,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: longPath},
				Response: &mock.HTTPResponse{StatusCode: 200},
			},
		},
	}

	// Test with truncation enabled (noTruncate=false)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputMocksTable(mocks, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// ID should be truncated (17 chars + "...")
	if strings.Contains(output, longID) {
		t.Errorf("long ID should be truncated when noTruncate=false, got: %s", output)
	}
	expectedTruncatedID := longID[:17] + "..."
	if !strings.Contains(output, expectedTruncatedID) {
		t.Errorf("output should contain truncated ID %q, got: %s", expectedTruncatedID, output)
	}

	// Test with truncation disabled (noTruncate=true)
	oldStdout = os.Stdout
	r, w, _ = os.Pipe()
	os.Stdout = w

	outputMocksTable(mocks, true)

	w.Close()
	os.Stdout = oldStdout

	buf.Reset()
	buf.ReadFrom(r)
	outputFull := buf.String()

	// Full ID should be present when noTruncate=true
	if !strings.Contains(outputFull, longID) {
		t.Errorf("full ID should be shown when noTruncate=true, got: %s", outputFull)
	}
}

func TestOutputMocksTable_EnabledNil(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	// Mock with nil Enabled (defaults to true per the code)
	mocks := []*mock.Mock{
		{
			ID:   "http_nil_enabled",
			Type: mock.TypeHTTP,
			// Enabled is nil
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/api/test"},
				Response: &mock.HTTPResponse{StatusCode: 200},
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputMocksTable(mocks, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// nil Enabled should default to "true"
	if !strings.Contains(output, "true") {
		t.Errorf("nil Enabled should display as true, got: %s", output)
	}
}

// =============================================================================
// runList integration test with mock server
// =============================================================================

func TestRunList_Success(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mocks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":   "http_123",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api/data"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
				},
				"count": 1,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Set global adminURL for runList
	oldAdminURL := adminURL
	adminURL = ts.URL
	defer func() { adminURL = oldAdminURL }()

	// Ensure text output mode
	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	// Reset list-specific flags
	oldConfigFile := listConfigFile
	oldMockType := listMockType
	oldNoTruncate := listNoTruncate
	listConfigFile = ""
	listMockType = ""
	listNoTruncate = false
	defer func() {
		listConfigFile = oldConfigFile
		listMockType = oldMockType
		listNoTruncate = oldNoTruncate
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runList(nil, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runList() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "http_123") {
		t.Errorf("output should contain mock ID, got: %s", output)
	}
}

// =============================================================================
// runGet tests
// =============================================================================

func TestRunGet_Success(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mocks/http_123" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "http_123",
				"type": "http",
				"http": map[string]interface{}{
					"matcher":  map[string]interface{}{"method": "GET", "path": "/api/data"},
					"response": map[string]interface{}{"statusCode": 200, "body": `{"ok":true}`},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": "mock not found"})
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	adminURL = ts.URL
	defer func() { adminURL = oldAdminURL }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGet(nil, []string{"http_123"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGet() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "http_123") {
		t.Errorf("output should contain mock ID, got: %s", output)
	}
}

func TestRunGet_MissingID(t *testing.T) {
	// No globals modified — safe for parallel

	err := runGet(nil, []string{})
	if err == nil {
		t.Fatal("expected error when no mock ID provided")
	}
	if !strings.Contains(err.Error(), "mock ID is required") {
		t.Errorf("error should mention 'mock ID is required', got: %v", err)
	}
}

func TestRunGet_NotFound(t *testing.T) {
	// Cannot use t.Parallel() — modifies global: adminURL

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": "mock not found: nonexistent"})
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	adminURL = ts.URL
	defer func() { adminURL = oldAdminURL }()

	err := runGet(nil, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent mock")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// =============================================================================
// printChaosProfileConfig tests
// =============================================================================

func TestPrintChaosProfileConfig_Latency(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	profile := &ChaosProfileInfo{
		Name:        "slow-api",
		Description: "Adds 200-800ms latency",
		Config: map[string]interface{}{
			"latency": map[string]interface{}{
				"min": "200ms",
				"max": "800ms",
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printChaosProfileConfig(profile)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Latency") {
		t.Errorf("output should contain 'Latency', got: %s", output)
	}
	if !strings.Contains(output, "200ms") {
		t.Errorf("output should contain '200ms', got: %s", output)
	}
	if !strings.Contains(output, "800ms") {
		t.Errorf("output should contain '800ms', got: %s", output)
	}
}

func TestPrintChaosProfileConfig_ErrorRate(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	profile := &ChaosProfileInfo{
		Name:        "flaky",
		Description: "30% error rate",
		Config: map[string]interface{}{
			"errorRate": map[string]interface{}{
				"probability": 0.3,
				"statusCodes": []interface{}{float64(500), float64(502), float64(503)},
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printChaosProfileConfig(profile)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Error rate: 30%") {
		t.Errorf("output should contain 'Error rate: 30%%', got: %s", output)
	}
	if !strings.Contains(output, "500") {
		t.Errorf("output should contain error code 500, got: %s", output)
	}
	if !strings.Contains(output, "502") {
		t.Errorf("output should contain error code 502, got: %s", output)
	}
	if !strings.Contains(output, "503") {
		t.Errorf("output should contain error code 503, got: %s", output)
	}
}

func TestPrintChaosProfileConfig_Bandwidth(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	tests := []struct {
		name    string
		bps     float64
		wantStr string
	}{
		{
			name:    "KB/s display for >= 1024",
			bps:     51200,
			wantStr: "50 KB/s",
		},
		{
			name:    "B/s display for < 1024",
			bps:     512,
			wantStr: "512 B/s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &ChaosProfileInfo{
				Name: "throttled",
				Config: map[string]interface{}{
					"bandwidth": map[string]interface{}{
						"bytesPerSecond": tt.bps,
					},
				},
			}

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printChaosProfileConfig(profile)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			if !strings.Contains(output, tt.wantStr) {
				t.Errorf("output should contain %q, got: %s", tt.wantStr, output)
			}
		})
	}
}

func TestPrintChaosProfileConfig_Combined(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	profile := &ChaosProfileInfo{
		Name:        "degraded",
		Description: "Slow + flaky + throttled",
		Config: map[string]interface{}{
			"latency": map[string]interface{}{
				"min": "100ms",
				"max": "500ms",
			},
			"errorRate": map[string]interface{}{
				"probability": 0.1,
			},
			"bandwidth": map[string]interface{}{
				"bytesPerSecond": float64(10240),
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printChaosProfileConfig(profile)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Latency") {
		t.Errorf("should show latency, got: %s", output)
	}
	if !strings.Contains(output, "Error rate") {
		t.Errorf("should show error rate, got: %s", output)
	}
	if !strings.Contains(output, "Bandwidth") {
		t.Errorf("should show bandwidth, got: %s", output)
	}
}

func TestPrintChaosProfileConfig_EmptyConfig(t *testing.T) {
	// Cannot use t.Parallel() — captures os.Stdout

	profile := &ChaosProfileInfo{
		Name:   "empty",
		Config: map[string]interface{}{},
	}

	// Should not panic with empty config
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printChaosProfileConfig(profile)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Should produce no output for empty config (no panic)
	output := buf.String()
	if strings.Contains(output, "Latency") || strings.Contains(output, "Error") || strings.Contains(output, "Bandwidth") {
		t.Errorf("empty config should produce no chaos detail output, got: %s", output)
	}
}

// =============================================================================
// Chaos enable validation test
// =============================================================================

func TestChaosEnable_RequiresLatencyOrErrorRate(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: chaosEnableLatency, chaosEnableErrorRate, adminURL

	// The validation in chaosEnableCmd.RunE checks:
	// if chaosEnableLatency == "" && chaosEnableErrorRate == 0 → error

	// Save and restore globals
	oldLatency := chaosEnableLatency
	oldErrorRate := chaosEnableErrorRate
	defer func() {
		chaosEnableLatency = oldLatency
		chaosEnableErrorRate = oldErrorRate
	}()

	chaosEnableLatency = ""
	chaosEnableErrorRate = 0

	// Create a fake server (won't be called because validation fails first)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when validation fails")
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	adminURL = ts.URL
	defer func() { adminURL = oldAdminURL }()

	err := chaosEnableCmd.RunE(chaosEnableCmd, nil)
	if err == nil {
		t.Fatal("expected error when neither --latency nor --error-rate is set")
	}
	if !strings.Contains(err.Error(), "--latency") || !strings.Contains(err.Error(), "--error-rate") {
		t.Errorf("error should mention --latency and --error-rate, got: %v", err)
	}
}

func TestChaosEnable_WithLatency(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: chaosEnableLatency, chaosEnableErrorRate, adminURL, os.Stdout

	var receivedConfig map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/chaos" {
			json.NewDecoder(r.Body).Decode(&receivedConfig)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"enabled":true}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Save and restore globals
	oldLatency := chaosEnableLatency
	oldErrorRate := chaosEnableErrorRate
	oldProbability := chaosEnableProbability
	oldPath := chaosEnablePath
	oldErrorCode := chaosEnableErrorCode
	oldAdminURL := adminURL
	oldJSON := jsonOutput
	defer func() {
		chaosEnableLatency = oldLatency
		chaosEnableErrorRate = oldErrorRate
		chaosEnableProbability = oldProbability
		chaosEnablePath = oldPath
		chaosEnableErrorCode = oldErrorCode
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	chaosEnableLatency = "100ms-500ms"
	chaosEnableErrorRate = 0
	chaosEnableProbability = 1.0
	chaosEnablePath = ""
	chaosEnableErrorCode = 500
	adminURL = ts.URL
	jsonOutput = false

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := chaosEnableCmd.RunE(chaosEnableCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("chaosEnableCmd.RunE() returned error: %v", err)
	}

	if receivedConfig == nil {
		t.Fatal("server should have received chaos config")
	}

	enabled, ok := receivedConfig["enabled"].(bool)
	if !ok || !enabled {
		t.Errorf("config should have enabled=true, got: %v", receivedConfig["enabled"])
	}
}

// =============================================================================
// runDelete tests
// =============================================================================

func TestRunDelete_NoArgNoPath(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: deletePath, deleteMethod, deleteYes

	// Save and restore
	oldPath := deletePath
	oldMethod := deleteMethod
	oldYes := deleteYes
	defer func() {
		deletePath = oldPath
		deleteMethod = oldMethod
		deleteYes = oldYes
	}()

	deletePath = ""
	deleteMethod = ""
	deleteYes = false

	err := runDelete(nil, []string{})
	if err == nil {
		t.Fatal("expected error when no ID or --path provided")
	}
	if !strings.Contains(err.Error(), "mock ID or --path is required") {
		t.Errorf("error should mention 'mock ID or --path is required', got: %v", err)
	}
}

func TestRunDelete_ByPath(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: deletePath, deleteMethod, deleteYes, adminURL, os.Stdout

	var deletedID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mocks":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":   "http_target",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api/target"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
				},
				"count": 1,
			})
		case r.Method == http.MethodDelete:
			deletedID = strings.TrimPrefix(r.URL.Path, "/mocks/")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Save and restore globals
	oldPath := deletePath
	oldMethod := deleteMethod
	oldYes := deleteYes
	oldAdminURL := adminURL
	oldJSON := jsonOutput
	defer func() {
		deletePath = oldPath
		deleteMethod = oldMethod
		deleteYes = oldYes
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	deletePath = "/api/target"
	deleteMethod = ""
	deleteYes = false
	adminURL = ts.URL
	jsonOutput = false

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDelete(nil, nil)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runDelete() returned error: %v", err)
	}
	if deletedID != "http_target" {
		t.Errorf("expected mock http_target to be deleted, got %q", deletedID)
	}
}

// =============================================================================
// runGet JSON output test
// =============================================================================

func TestRunGet_JSONOutput(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mocks/http_json1" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "http_json1",
				"type": "http",
				"http": map[string]interface{}{
					"matcher":  map[string]interface{}{"method": "GET", "path": "/api/json"},
					"response": map[string]interface{}{"statusCode": 200, "body": `{"data":"test"}`},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": "not found"})
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = true
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGet(nil, []string{"http_json1"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGet() returned error: %v", err)
	}

	output := buf.String()
	// Verify output is valid JSON
	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(output), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput: %s", jsonErr, output)
	}

	if result["id"] != "http_json1" {
		t.Errorf("JSON output id = %v, want http_json1", result["id"])
	}
}
