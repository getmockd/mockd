package views

import (
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
)

func TestNewMocks(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocks := NewMocks(adminClient)

	if mocks.client == nil {
		t.Error("NewMocks() should initialize client")
	}

	if mocks.viewMode != ViewModeList {
		t.Error("NewMocks() should initialize in list view mode")
	}

	if mocks.loading != true {
		t.Error("NewMocks() should initialize with loading = true")
	}

	if len(mocks.mocks) != 0 {
		t.Error("NewMocks() should initialize with empty mocks list")
	}

	if mocks.filterActive {
		t.Error("NewMocks() should initialize with filter inactive")
	}

	if mocks.selectedMock != nil {
		t.Error("NewMocks() should initialize with no selected mock")
	}
}

func TestMocksSetSize(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocks := NewMocks(adminClient)

	mocks.SetSize(100, 50)

	if mocks.width != 100 {
		t.Errorf("Expected width 100, got %d", mocks.width)
	}

	if mocks.height != 50 {
		t.Errorf("Expected height 50, got %d", mocks.height)
	}
}

func TestMocksUpdateTable(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)

	// Create test mocks
	testMocks := []*config.MockConfiguration{
		{
			ID:      "mock1",
			Name:    "Test Mock 1",
			Enabled: true,
			Matcher: &config.RequestMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &config.ResponseDefinition{
				StatusCode: 200,
				Body:       "{}",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:      "mock2",
			Name:    "Test Mock 2",
			Enabled: false,
			Matcher: &config.RequestMatcher{
				Method: "POST",
				Path:   "/api/orders",
			},
			Response: &config.ResponseDefinition{
				StatusCode: 201,
				Body:       "{}",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	mocksView.mocks = testMocks
	mocksView.updateTable()

	// Table should have 2 rows
	rows := mocksView.table.Rows()
	if len(rows) != 2 {
		t.Errorf("Expected 2 table rows, got %d", len(rows))
	}

	// Check first row
	if rows[0][0] != "✓" {
		t.Errorf("Expected first mock to be enabled (✓), got %s", rows[0][0])
	}
	if rows[0][1] != "GET" {
		t.Errorf("Expected method GET, got %s", rows[0][1])
	}
	if rows[0][2] != "/api/users" {
		t.Errorf("Expected path /api/users, got %s", rows[0][2])
	}
	if rows[0][3] != "200" {
		t.Errorf("Expected status 200, got %s", rows[0][3])
	}

	// Check second row
	if rows[1][0] != "✗" {
		t.Errorf("Expected second mock to be disabled (✗), got %s", rows[1][0])
	}
	if rows[1][1] != "POST" {
		t.Errorf("Expected method POST, got %s", rows[1][1])
	}
}

func TestMocksUpdateSelectedMock(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)

	testMocks := []*config.MockConfiguration{
		{
			ID:   "mock1",
			Name: "Test Mock 1",
			Matcher: &config.RequestMatcher{
				Method: "GET",
				Path:   "/test1",
			},
			Response: &config.ResponseDefinition{
				StatusCode: 200,
			},
		},
		{
			ID:   "mock2",
			Name: "Test Mock 2",
			Matcher: &config.RequestMatcher{
				Method: "POST",
				Path:   "/test2",
			},
			Response: &config.ResponseDefinition{
				StatusCode: 201,
			},
		},
	}

	mocksView.mocks = testMocks
	mocksView.updateTable()

	// Initially, cursor should be at 0
	mocksView.updateSelectedMock()
	if mocksView.selectedMock == nil {
		t.Error("Expected a selected mock, got nil")
	}
	if mocksView.selectedMock.ID != "mock1" {
		t.Errorf("Expected selected mock ID 'mock1', got '%s'", mocksView.selectedMock.ID)
	}
}

func TestEnabledStatus(t *testing.T) {
	tests := []struct {
		enabled  bool
		expected string
	}{
		{true, "enabled"},
		{false, "disabled"},
	}

	for _, tt := range tests {
		result := enabledStatus(tt.enabled)
		if result != tt.expected {
			t.Errorf("enabledStatus(%v) = %s, expected %s", tt.enabled, result, tt.expected)
		}
	}
}

func TestMocksViewRender(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)
	mocksView.SetSize(100, 30)

	// Test list view rendering
	view := mocksView.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}

	// Should contain the title
	if len(view) < 10 {
		t.Error("View should render content")
	}
}

func TestMocksUpdateWithData(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)

	testMocks := []*config.MockConfiguration{
		{
			ID:      "mock1",
			Name:    "Test Mock",
			Enabled: true,
			Matcher: &config.RequestMatcher{
				Method: "GET",
				Path:   "/test",
			},
			Response: &config.ResponseDefinition{
				StatusCode: 200,
				Body:       "{}",
			},
		},
	}

	msg := mocksLoadedMsg{mocks: testMocks}
	mocksView, _ = mocksView.Update(msg)

	if mocksView.loading {
		t.Error("Loading should be false after mocksLoadedMsg")
	}

	if len(mocksView.mocks) != 1 {
		t.Errorf("Expected 1 mock, got %d", len(mocksView.mocks))
	}

	if mocksView.mocks[0].ID != "mock1" {
		t.Errorf("Expected mock ID 'mock1', got '%s'", mocksView.mocks[0].ID)
	}
}

func TestMocksUpdateWithError(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)
	mocksView.loading = true

	msg := mocksErrorMsg{err: &TestError{message: "API error"}}
	mocksView, _ = mocksView.Update(msg)

	if mocksView.loading {
		t.Error("Loading should be false after error")
	}

	if mocksView.err == nil {
		t.Error("Error should be set after mocksErrorMsg")
	}
}

// TestError is a test error implementation
type TestError struct {
	message string
}

func (e *TestError) Error() string {
	return e.message
}

func TestMocksFormSubmit(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)
	mocksView.viewMode = ViewModeForm

	testMock := &config.MockConfiguration{
		ID:   "new-mock",
		Name: "Created Mock",
	}

	msg := mockFormSubmittedMsg{mock: testMock}
	mocksView, _ = mocksView.Update(msg)

	if mocksView.viewMode != ViewModeList {
		t.Error("View mode should return to list after form submission")
	}

	if !mocksView.loading {
		t.Error("Loading should be true to fetch updated mocks list")
	}
}

func TestMocksFormCancel(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)
	mocksView.viewMode = ViewModeForm
	mocksView.loading = false // Set initial state

	msg := mockFormCancelledMsg{}
	mocksView, _ = mocksView.Update(msg)

	if mocksView.viewMode != ViewModeList {
		t.Error("View mode should return to list after form cancellation")
	}
}

func TestMocksToggle(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)

	testMock := &config.MockConfiguration{
		ID:      "mock1",
		Name:    "Test Mock",
		Enabled: true,
	}

	msg := mockToggledMsg{mock: testMock}
	mocksView, _ = mocksView.Update(msg)

	if !mocksView.loading {
		t.Error("Loading should be true after toggle to refresh list")
	}
}

func TestMocksDelete(t *testing.T) {
	adminClient := client.NewDefaultClient()
	mocksView := NewMocks(adminClient)
	mocksView.mockToDelete = &config.MockConfiguration{
		ID:   "mock1",
		Name: "Test Mock",
	}

	msg := mockDeletedMsg{}
	mocksView, _ = mocksView.Update(msg)

	if mocksView.mockToDelete != nil {
		t.Error("mockToDelete should be nil after deletion")
	}

	if !mocksView.loading {
		t.Error("Loading should be true after delete to refresh list")
	}
}
