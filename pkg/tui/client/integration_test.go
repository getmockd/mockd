//go:build integration
// +build integration

package client_test

import (
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
)

// TestIntegration_HealthCheck tests a real connection to Admin API.
// Run with: go test -tags=integration -v ./pkg/tui/client/...
// Requires a running mockd instance on localhost:9090
func TestIntegration_HealthCheck(t *testing.T) {
	c := client.NewDefaultClient()

	health, err := c.GetHealth()
	if err != nil {
		t.Skipf("Skipping integration test: Admin API not available: %v", err)
		return
	}

	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", health.Status)
	}

	t.Logf("Server is healthy, uptime: %d seconds", health.Uptime)
}

// TestIntegration_MockOperations tests full mock CRUD cycle.
func TestIntegration_MockOperations(t *testing.T) {
	c := client.NewDefaultClient()

	// Verify server is available
	if err := c.Ping(); err != nil {
		t.Skipf("Skipping integration test: Admin API not available: %v", err)
		return
	}

	// Create a test mock
	testMock := &config.MockConfiguration{
		Name: "Integration Test Mock",
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/test/integration",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       `{"test": "integration"}`,
		},
		Enabled: true,
	}

	// Create
	created, err := c.CreateMock(testMock)
	if err != nil {
		t.Fatalf("CreateMock failed: %v", err)
	}
	t.Logf("Created mock with ID: %s", created.ID)

	// Verify it appears in the list
	mocks, err := c.ListMocks()
	if err != nil {
		t.Fatalf("ListMocks failed: %v", err)
	}

	found := false
	for _, m := range mocks {
		if m.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Created mock not found in list")
	}

	// Get specific mock
	retrieved, err := c.GetMock(created.ID)
	if err != nil {
		t.Fatalf("GetMock failed: %v", err)
	}
	if retrieved.ID != created.ID {
		t.Errorf("Retrieved mock ID mismatch")
	}

	// Toggle
	toggled, err := c.ToggleMock(created.ID, false)
	if err != nil {
		t.Fatalf("ToggleMock failed: %v", err)
	}
	if toggled.Enabled {
		t.Errorf("Expected mock to be disabled")
	}

	// Update
	created.Name = "Updated Integration Test Mock"
	updated, err := c.UpdateMock(created.ID, created)
	if err != nil {
		t.Fatalf("UpdateMock failed: %v", err)
	}
	if updated.Name != created.Name {
		t.Errorf("Mock name not updated")
	}

	// Cleanup - delete the test mock
	if err := c.DeleteMock(created.ID); err != nil {
		t.Fatalf("DeleteMock failed: %v", err)
	}
	t.Logf("Cleaned up test mock")

	// Verify it's gone
	time.Sleep(100 * time.Millisecond)
	_, err = c.GetMock(created.ID)
	if err == nil {
		t.Errorf("Expected error when getting deleted mock")
	}
}

// TestIntegration_TrafficOperations tests traffic/request log operations.
func TestIntegration_TrafficOperations(t *testing.T) {
	c := client.NewDefaultClient()

	if err := c.Ping(); err != nil {
		t.Skipf("Skipping integration test: Admin API not available: %v", err)
		return
	}

	// Get traffic
	entries, err := c.GetTraffic(nil)
	if err != nil {
		t.Fatalf("GetTraffic failed: %v", err)
	}
	t.Logf("Found %d traffic entries", len(entries))

	// Test with filter
	filter := &client.RequestLogFilter{
		Limit: 10,
	}
	filtered, err := c.GetTraffic(filter)
	if err != nil {
		t.Fatalf("GetTraffic with filter failed: %v", err)
	}
	if len(filtered) > 10 {
		t.Errorf("Expected max 10 entries, got %d", len(filtered))
	}
}

// TestIntegration_ProxyStatus tests proxy status query.
func TestIntegration_ProxyStatus(t *testing.T) {
	c := client.NewDefaultClient()

	if err := c.Ping(); err != nil {
		t.Skipf("Skipping integration test: Admin API not available: %v", err)
		return
	}

	status, err := c.GetProxyStatus()
	if err != nil {
		t.Fatalf("GetProxyStatus failed: %v", err)
	}

	t.Logf("Proxy running: %t", status.Running)
	if status.Running {
		t.Logf("Proxy port: %d, mode: %s", status.Port, status.Mode)
	}
}

// TestIntegration_StreamRecordings tests stream recording operations.
func TestIntegration_StreamRecordings(t *testing.T) {
	c := client.NewDefaultClient()

	if err := c.Ping(); err != nil {
		t.Skipf("Skipping integration test: Admin API not available: %v", err)
		return
	}

	// List stream recordings
	recordings, err := c.ListStreamRecordings(nil)
	if err != nil {
		t.Fatalf("ListStreamRecordings failed: %v", err)
	}
	t.Logf("Found %d stream recordings", len(recordings))

	// Test with filter
	filter := &client.StreamRecordingFilter{
		Protocol: "websocket",
		Limit:    50,
	}
	filtered, err := c.ListStreamRecordings(filter)
	if err != nil {
		t.Fatalf("ListStreamRecordings with filter failed: %v", err)
	}
	t.Logf("Found %d WebSocket recordings", len(filtered))

	// Get stats
	stats, err := c.GetStreamRecordingStats()
	if err != nil {
		t.Fatalf("GetStreamRecordingStats failed: %v", err)
	}
	t.Logf("Storage stats: %+v", stats)
}
