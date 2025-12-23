package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/tui/client"
)

func TestNewLogs(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	if model.client == nil {
		t.Error("client should not be nil")
	}

	if model.loading {
		t.Error("logs view should not start in loading state")
	}

	if model.paused {
		t.Error("logs should not be paused initially")
	}

	if model.levelFilter != LogLevelDebug {
		t.Errorf("expected default level filter Debug, got %v", model.levelFilter)
	}
}

func TestLogsUpdate_WindowSize(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := model.Update(msg)

	if updated.width != 100 {
		t.Errorf("expected width 100, got %d", updated.width)
	}

	if updated.height != 50 {
		t.Errorf("expected height 50, got %d", updated.height)
	}

	if updated.viewport.Width != 96 {
		t.Errorf("expected viewport width 96, got %d", updated.viewport.Width)
	}
}

func TestLogsPauseResume(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	// Press 'p' to pause
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	updated, _ := model.handleKey(msg)

	if !updated.paused {
		t.Error("logs should be paused")
	}

	// Press 'p' again to resume
	updated, _ = updated.handleKey(msg)

	if updated.paused {
		t.Error("logs should be resumed")
	}
}

func TestLogsClear(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	// Add some logs
	model.logs = []LogEntry{
		{Level: LogLevelInfo, Message: "test"},
		{Level: LogLevelWarn, Message: "test"},
	}
	model.filtered = model.logs

	// Press 'c' to clear
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	updated, _ := model.handleKey(msg)

	if len(updated.logs) != 0 {
		t.Errorf("expected 0 logs after clear, got %d", len(updated.logs))
	}

	if len(updated.filtered) != 0 {
		t.Errorf("expected 0 filtered logs after clear, got %d", len(updated.filtered))
	}
}

func TestLogsLevelFiltering(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	// Add logs of different levels
	model.logs = []LogEntry{
		{Level: LogLevelDebug, Message: "debug"},
		{Level: LogLevelInfo, Message: "info"},
		{Level: LogLevelWarn, Message: "warn"},
		{Level: LogLevelError, Message: "error"},
	}

	tests := []struct {
		key           string
		expectedLevel LogLevel
		expectedCount int
	}{
		{"1", LogLevelDebug, 4}, // All
		{"2", LogLevelInfo, 3},  // Info, Warn, Error
		{"3", LogLevelWarn, 2},  // Warn, Error
		{"4", LogLevelError, 1}, // Error only
	}

	for _, tt := range tests {
		t.Run("level_"+tt.key, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(tt.key[0])}}
			updated, _ := model.handleKey(msg)

			if updated.levelFilter != tt.expectedLevel {
				t.Errorf("expected level filter %v, got %v", tt.expectedLevel, updated.levelFilter)
			}

			if len(updated.filtered) != tt.expectedCount {
				t.Errorf("expected %d filtered logs, got %d", tt.expectedCount, len(updated.filtered))
			}
		})
	}
}

func TestLogsSearchToggle(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	// Press '/' to show search
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	updated, _ := model.handleKey(msg)

	if !updated.showSearch {
		t.Error("search should be shown")
	}

	// Press 'esc' to hide it
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = updated.handleKey(msg)

	if updated.showSearch {
		t.Error("search should be hidden")
	}
}

func TestLogsSearchFiltering(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewLogs(c)

	// Add some logs
	model.logs = []LogEntry{
		{Level: LogLevelInfo, Message: "User login successful", Source: "auth"},
		{Level: LogLevelInfo, Message: "Request processed", Source: "engine"},
		{Level: LogLevelError, Message: "User not found", Source: "auth"},
	}

	// Set search term
	model.searchTerm = "user"
	model.filterLogs()

	if len(model.filtered) != 2 {
		t.Errorf("expected 2 logs matching 'user', got %d", len(model.filtered))
	}
}
