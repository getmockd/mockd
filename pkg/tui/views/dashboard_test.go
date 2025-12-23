package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
)

// TestNewDashboard tests dashboard creation.
func TestNewDashboard(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)

	if dashboard.client == nil {
		t.Error("Expected client to be set")
	}

	if !dashboard.loading {
		t.Error("Expected dashboard to start in loading state")
	}
}

// TestDashboardInit tests dashboard initialization.
func TestDashboardInit(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)

	cmd := dashboard.Init()
	if cmd == nil {
		t.Error("Expected Init to return a command")
	}
}

// TestDashboardSetSize tests setting dashboard dimensions.
func TestDashboardSetSize(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)

	dashboard.SetSize(100, 50)

	if dashboard.width != 100 {
		t.Errorf("Expected width 100, got %d", dashboard.width)
	}

	if dashboard.height != 50 {
		t.Errorf("Expected height 50, got %d", dashboard.height)
	}
}

// TestDashboardUpdate tests dashboard update with window size message.
func TestDashboardUpdate(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 60,
	}

	updatedDashboard, _ := dashboard.Update(msg)

	if updatedDashboard.width != 120 {
		t.Errorf("Expected width 120, got %d", updatedDashboard.width)
	}

	if updatedDashboard.height != 60 {
		t.Errorf("Expected height 60, got %d", updatedDashboard.height)
	}
}

// TestDashboardUpdateWithData tests dashboard update with dashboard data.
func TestDashboardUpdateWithData(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)
	dashboard.loading = true

	// Create test data
	health := &admin.HealthResponse{
		Status: "ok",
		Uptime: 3600,
	}

	proxyStatus := &admin.ProxyStatusResponse{
		Running: true,
		Port:    8080,
		Mode:    "record",
	}

	mocks := []*config.MockConfiguration{
		{
			ID:      "mock1",
			Enabled: true,
		},
		{
			ID:      "mock2",
			Enabled: false,
		},
	}

	traffic := []*config.RequestLogEntry{
		{
			ID:             "req1",
			Timestamp:      time.Now(),
			Method:         "GET",
			Path:           "/api/users",
			ResponseStatus: 200,
			DurationMs:     15,
		},
	}

	msg := dashboardDataMsg{
		health:        health,
		proxyStatus:   proxyStatus,
		mocks:         mocks,
		recentTraffic: traffic,
	}

	updatedDashboard, _ := dashboard.Update(msg)

	if updatedDashboard.loading {
		t.Error("Expected loading to be false after data received")
	}

	if updatedDashboard.health == nil {
		t.Error("Expected health to be set")
	}

	if updatedDashboard.activeMocks != 1 {
		t.Errorf("Expected 1 active mock, got %d", updatedDashboard.activeMocks)
	}

	if updatedDashboard.disabledMocks != 1 {
		t.Errorf("Expected 1 disabled mock, got %d", updatedDashboard.disabledMocks)
	}

	if len(updatedDashboard.recentTraffic) != 1 {
		t.Errorf("Expected 1 traffic entry, got %d", len(updatedDashboard.recentTraffic))
	}
}

// TestDashboardUpdateWithError tests dashboard update with error.
func TestDashboardUpdateWithError(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)
	dashboard.loading = true

	msg := errMsg{
		err: &testError{"test error"},
	}

	updatedDashboard, _ := dashboard.Update(msg)

	if updatedDashboard.loading {
		t.Error("Expected loading to be false after error")
	}

	if updatedDashboard.err == nil {
		t.Error("Expected error to be set")
	}
}

// TestDashboardViewLoading tests rendering while loading.
func TestDashboardViewLoading(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)
	dashboard.loading = true

	view := dashboard.View()

	if view == "" {
		t.Error("Expected view to render loading state")
	}
}

// TestDashboardViewError tests rendering error state.
func TestDashboardViewError(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)
	dashboard.loading = false
	dashboard.err = &testError{"connection failed"}

	view := dashboard.View()

	if view == "" {
		t.Error("Expected view to render error state")
	}
}

// TestDashboardViewWithData tests rendering with actual data.
func TestDashboardViewWithData(t *testing.T) {
	client := client.NewClient("http://localhost:9090")
	dashboard := NewDashboard(client)
	dashboard.loading = false

	dashboard.health = &admin.HealthResponse{
		Status: "ok",
		Uptime: 3600,
	}

	dashboard.proxyStatus = &admin.ProxyStatusResponse{
		Running: true,
		Port:    8080,
	}

	dashboard.mocks = []*config.MockConfiguration{
		{ID: "mock1", Enabled: true},
		{ID: "mock2", Enabled: true},
		{ID: "mock3", Enabled: false},
	}

	dashboard.activeMocks = 2
	dashboard.disabledMocks = 1

	view := dashboard.View()

	if view == "" {
		t.Error("Expected view to render dashboard")
	}

	// Check for expected content
	if !contains(view, "Server Status") {
		t.Error("Expected view to contain 'Server Status'")
	}

	if !contains(view, "Quick Stats") {
		t.Error("Expected view to contain 'Quick Stats'")
	}

	if !contains(view, "Recent Activity") {
		t.Error("Expected view to contain 'Recent Activity'")
	}
}

// TestFormatDuration tests the duration formatting utility.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Nanosecond, "500ns"},
		{500 * time.Microsecond, "500µs"},
		{15 * time.Millisecond, "15ms"},
		{2 * time.Second, "2.0s"},
		{90 * time.Second, "1.5m"},
		{2 * time.Hour, "2.0h"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}

// TestTruncate tests the string truncation utility.
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten!", 12, "exactly ten!"},
		{"this is a very long string", 10, "this is..."},
		{"truncate me", 8, "trunc..."},
		{"abc", 2, "ab"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// Helper types and functions

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findInString(s, substr)
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
