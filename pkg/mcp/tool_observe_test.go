package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	apitypes "github.com/getmockd/mockd/pkg/api/types"
	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// =============================================================================
// handleGetServerStatus Tests
// =============================================================================

func TestHandleGetServerStatus_HealthyWithStats(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		healthFn: func() error { return nil },
		getStatsFn: func() (*cli.StatsResult, error) {
			return &cli.StatsResult{
				Uptime:       3600,
				RequestCount: 42,
				MockCount:    5,
			}, nil
		},
		getPortsFn: func() ([]cli.PortInfo, error) { return nil, fmt.Errorf("no ports") },
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetServerStatus(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetServerStatus() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["version"] != ServerVersion {
		t.Errorf("version = %v, want %v", parsed["version"], ServerVersion)
	}
	if parsed["adminUrl"] != "http://localhost:4290" {
		t.Errorf("adminUrl = %v, want http://localhost:4290", parsed["adminUrl"])
	}
	if parsed["reachable"] != true {
		t.Errorf("reachable = %v, want true", parsed["reachable"])
	}
	if parsed["healthy"] != true {
		t.Errorf("healthy = %v, want true", parsed["healthy"])
	}
	if parsed["uptime"] != float64(3600) {
		t.Errorf("uptime = %v, want 3600", parsed["uptime"])
	}
	if parsed["totalRequests"] != float64(42) {
		t.Errorf("totalRequests = %v, want 42", parsed["totalRequests"])
	}
	if parsed["mockCount"] != float64(5) {
		t.Errorf("mockCount = %v, want 5", parsed["mockCount"])
	}
}

func TestHandleGetServerStatus_HealthyWithPorts(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		healthFn:   func() error { return nil },
		getStatsFn: func() (*cli.StatsResult, error) { return nil, fmt.Errorf("stats unavailable") },
		getPortsFn: func() ([]cli.PortInfo, error) {
			return []cli.PortInfo{
				{Port: 4280, Protocol: "HTTP", Component: "Mock Engine", Status: "running"},
				{Port: 4290, Protocol: "HTTP", Component: "Admin API", Status: "running"},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetServerStatus(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetServerStatus() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reachable"] != true {
		t.Errorf("reachable = %v, want true", parsed["reachable"])
	}
	if parsed["healthy"] != true {
		t.Errorf("healthy = %v, want true", parsed["healthy"])
	}

	ports, ok := parsed["ports"].([]interface{})
	if !ok {
		t.Fatalf("ports is not an array: %T", parsed["ports"])
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}

	port0 := ports[0].(map[string]interface{})
	if port0["port"] != float64(4280) {
		t.Errorf("ports[0].port = %v, want 4280", port0["port"])
	}
	if port0["component"] != "Mock Engine" {
		t.Errorf("ports[0].component = %v, want Mock Engine", port0["component"])
	}
}

func TestHandleGetServerStatus_Unreachable(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		healthFn: func() error {
			return fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetServerStatus(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetServerStatus() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result (with unreachable info), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reachable"] != false {
		t.Errorf("reachable = %v, want false", parsed["reachable"])
	}
	if parsed["healthy"] != false {
		t.Errorf("healthy = %v, want false", parsed["healthy"])
	}
	errMsg, _ := parsed["error"].(string)
	if errMsg == "" {
		t.Error("expected non-empty error field")
	}
	hint, _ := parsed["hint"].(string)
	if hint == "" {
		t.Error("expected non-empty hint field")
	}
}

func TestHandleGetServerStatus_UnhealthyNonConnection(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		healthFn: func() error {
			return &cli.APIError{StatusCode: 503, Message: "service degraded"}
		},
		getStatsFn: func() (*cli.StatsResult, error) { return nil, fmt.Errorf("unavailable") },
		getPortsFn: func() ([]cli.PortInfo, error) { return nil, fmt.Errorf("unavailable") },
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetServerStatus(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetServerStatus() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result (with unhealthy info), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reachable"] != true {
		t.Errorf("reachable = %v, want true", parsed["reachable"])
	}
	if parsed["healthy"] != false {
		t.Errorf("healthy = %v, want false", parsed["healthy"])
	}
	healthErr, _ := parsed["healthError"].(string)
	if healthErr != "service degraded" {
		t.Errorf("healthError = %q, want %q", healthErr, "service degraded")
	}
}

func TestHandleGetServerStatus_StatsFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		healthFn: func() error { return nil },
		getStatsFn: func() (*cli.StatsResult, error) {
			return nil, fmt.Errorf("stats endpoint unavailable")
		},
		getPortsFn: func() ([]cli.PortInfo, error) { return nil, fmt.Errorf("ports unavailable") },
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetServerStatus(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetServerStatus() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	// Should still be healthy even if stats fail
	if parsed["reachable"] != true {
		t.Errorf("reachable = %v, want true", parsed["reachable"])
	}
	if parsed["healthy"] != true {
		t.Errorf("healthy = %v, want true", parsed["healthy"])
	}
	// Stats fields should be absent
	if _, ok := parsed["uptime"]; ok {
		t.Error("expected no uptime field when stats fail")
	}
	if _, ok := parsed["totalRequests"]; ok {
		t.Error("expected no totalRequests field when stats fail")
	}
}

func TestHandleGetServerStatus_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	result, err := handleGetServerStatus(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetServerStatus() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// =============================================================================
// handleGetRequestLogs Tests
// =============================================================================

func TestHandleGetRequestLogs_Success(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			return &cli.LogResult{
				Requests: []*apitypes.RequestLogEntry{
					{
						ID:            "log-1",
						Method:        "GET",
						Path:          "/api/users",
						StatusCode:    200,
						DurationMs:    15,
						Timestamp:     ts,
						MatchedMockID: "http_abc123",
					},
					{
						ID:            "log-2",
						Method:        "POST",
						Path:          "/api/users",
						StatusCode:    201,
						DurationMs:    32,
						Timestamp:     ts.Add(time.Second),
						MatchedMockID: "http_def456",
					},
				},
				Count: 2,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var entries []RequestLogEntry
	resultJSON(t, result, &entries)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != "log-1" {
		t.Errorf("entries[0].ID = %s, want log-1", entries[0].ID)
	}
	if entries[0].Method != "GET" {
		t.Errorf("entries[0].Method = %s, want GET", entries[0].Method)
	}
	if entries[0].Path != "/api/users" {
		t.Errorf("entries[0].Path = %s, want /api/users", entries[0].Path)
	}
	if entries[0].Status != 200 {
		t.Errorf("entries[0].Status = %d, want 200", entries[0].Status)
	}
	if entries[0].Duration != "15ms" {
		t.Errorf("entries[0].Duration = %s, want 15ms", entries[0].Duration)
	}
	if entries[0].MockID != "http_abc123" {
		t.Errorf("entries[0].MockID = %s, want http_abc123", entries[0].MockID)
	}
	if entries[1].Status != 201 {
		t.Errorf("entries[1].Status = %d, want 201", entries[1].Status)
	}
}

func TestHandleGetRequestLogs_WithFilters(t *testing.T) {
	t.Parallel()

	var capturedFilter *cli.LogFilter
	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			capturedFilter = filter
			return &cli.LogResult{
				Requests: []*apitypes.RequestLogEntry{},
				Count:    0,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"limit":         float64(25),
		"offset":        float64(10),
		"method":        "POST",
		"pathPrefix":    "/api/orders",
		"mockId":        "http_xyz",
		"protocol":      "http",
		"unmatchedOnly": true,
	}
	result, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if capturedFilter == nil {
		t.Fatal("expected filter to be captured")
	}
	if capturedFilter.Limit != 25 {
		t.Errorf("filter.Limit = %d, want 25", capturedFilter.Limit)
	}
	if capturedFilter.Offset != 10 {
		t.Errorf("filter.Offset = %d, want 10", capturedFilter.Offset)
	}
	if capturedFilter.Method != "POST" {
		t.Errorf("filter.Method = %s, want POST", capturedFilter.Method)
	}
	if capturedFilter.Path != "/api/orders" {
		t.Errorf("filter.Path = %s, want /api/orders", capturedFilter.Path)
	}
	if capturedFilter.MatchedID != "http_xyz" {
		t.Errorf("filter.MatchedID = %s, want http_xyz", capturedFilter.MatchedID)
	}
	if capturedFilter.Protocol != "http" {
		t.Errorf("filter.Protocol = %s, want http", capturedFilter.Protocol)
	}
	if !capturedFilter.UnmatchedOnly {
		t.Error("filter.UnmatchedOnly = false, want true")
	}
}

func TestHandleGetRequestLogs_WithNearMisses(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			return &cli.LogResult{
				Requests: []*apitypes.RequestLogEntry{
					{
						ID:         "log-unmatched",
						Method:     "GET",
						Path:       "/api/unknown",
						StatusCode: 404,
						DurationMs: 2,
						Timestamp:  ts,
						NearMisses: []requestlog.NearMissInfo{
							{
								MockID:          "http_abc",
								MockName:        "Get Users",
								MatchPercentage: 75,
								Reason:          "path mismatch: expected /api/users",
							},
							{
								MockID:          "http_def",
								MockName:        "Get Orders",
								MatchPercentage: 50,
								Reason:          "path mismatch: expected /api/orders",
							},
						},
					},
				},
				Count: 1,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"unmatchedOnly": true}
	result, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var entries []RequestLogEntry
	resultJSON(t, result, &entries)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].MockID != "" {
		t.Errorf("MockID = %q, want empty (unmatched)", entries[0].MockID)
	}
	if len(entries[0].NearMisses) != 2 {
		t.Fatalf("expected 2 near misses, got %d", len(entries[0].NearMisses))
	}
	nm := entries[0].NearMisses[0]
	if nm.MockID != "http_abc" {
		t.Errorf("NearMisses[0].MockID = %s, want http_abc", nm.MockID)
	}
	if nm.MockName != "Get Users" {
		t.Errorf("NearMisses[0].MockName = %s, want Get Users", nm.MockName)
	}
	if nm.MatchPercentage != 75 {
		t.Errorf("NearMisses[0].MatchPercentage = %d, want 75", nm.MatchPercentage)
	}
	if nm.Reason != "path mismatch: expected /api/users" {
		t.Errorf("NearMisses[0].Reason = %s, want path mismatch: expected /api/users", nm.Reason)
	}
}

func TestHandleGetRequestLogs_Empty(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			return &cli.LogResult{
				Requests: []*apitypes.RequestLogEntry{},
				Count:    0,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var entries []RequestLogEntry
	resultJSON(t, result, &entries)

	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestHandleGetRequestLogs_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			return nil, &cli.APIError{StatusCode: 500, Message: "internal error"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to get logs") {
		t.Errorf("error text = %q, want to contain 'failed to get logs'", text)
	}
}

func TestHandleGetRequestLogs_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{}
	result, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleGetRequestLogs_DefaultLimitOffset(t *testing.T) {
	t.Parallel()

	var capturedFilter *cli.LogFilter
	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			capturedFilter = filter
			return &cli.LogResult{Requests: []*apitypes.RequestLogEntry{}, Count: 0}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	// No limit or offset specified — should use defaults
	args := map[string]interface{}{}
	_, err := handleGetRequestLogs(args, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}

	if capturedFilter.Limit != 100 {
		t.Errorf("default limit = %d, want 100", capturedFilter.Limit)
	}
	if capturedFilter.Offset != 0 {
		t.Errorf("default offset = %d, want 0", capturedFilter.Offset)
	}
}

func TestHandleGetRequestLogs_TimestampPreserved(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	client := &mockAdminClient{
		getLogsFn: func(filter *cli.LogFilter) (*cli.LogResult, error) {
			return &cli.LogResult{
				Requests: []*apitypes.RequestLogEntry{
					{
						ID:         "log-ts",
						Method:     "GET",
						Path:       "/",
						StatusCode: 200,
						DurationMs: 1,
						Timestamp:  ts,
					},
				},
				Count: 1,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetRequestLogs(map[string]interface{}{}, session, server)
	if err != nil {
		t.Fatalf("handleGetRequestLogs() error = %v", err)
	}

	// Parse the raw JSON to check the timestamp format
	text := resultText(t, result)
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("failed to parse raw JSON: %v", err)
	}

	var entry RequestLogEntry
	if err := json.Unmarshal(raw[0], &entry); err != nil {
		t.Fatalf("failed to parse entry: %v", err)
	}
	if !entry.Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", entry.Timestamp, ts)
	}
}

// =============================================================================
// handleClearRequestLogs Tests
// =============================================================================

func TestHandleClearRequestLogs_Success(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		clearLogsFn: func() (int, error) {
			return 17, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleClearRequestLogs(nil, session, server)
	if err != nil {
		t.Fatalf("handleClearRequestLogs() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["cleared"] != float64(17) {
		t.Errorf("cleared = %v, want 17", parsed["cleared"])
	}
}

func TestHandleClearRequestLogs_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		clearLogsFn: func() (int, error) {
			return 0, &cli.APIError{StatusCode: 500, Message: "clear failed"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleClearRequestLogs(nil, session, server)
	if err != nil {
		t.Fatalf("handleClearRequestLogs() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to clear logs") {
		t.Errorf("error text = %q, want to contain 'failed to clear logs'", text)
	}
}

func TestHandleClearRequestLogs_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	result, err := handleClearRequestLogs(nil, session, server)
	if err != nil {
		t.Fatalf("handleClearRequestLogs() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}
