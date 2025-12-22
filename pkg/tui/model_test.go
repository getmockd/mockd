package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInitialization(t *testing.T) {
	m := newModel()

	if m.currentView != dashboardView {
		t.Errorf("Expected initial view to be dashboard, got %v", m.currentView)
	}

	if m.ready {
		t.Error("Model should not be ready before window size message")
	}
}

func TestModelWindowResize(t *testing.T) {
	m := newModel()

	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updated, _ := m.Update(msg)
	m = updated.(model)

	if !m.ready {
		t.Error("Model should be ready after window size message")
	}

	if m.width != 100 || m.height != 30 {
		t.Errorf("Expected dimensions 100x30, got %dx%d", m.width, m.height)
	}
}

func TestModelViewSwitching(t *testing.T) {
	m := newModel()
	m.ready = true
	m.width = 100
	m.height = 30

	// Test switching to mocks view
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	updated, _ := m.Update(msg)
	m = updated.(model)

	if m.currentView != mocksView {
		t.Errorf("Expected mocks view, got %v", m.currentView)
	}

	// Verify sidebar active item updated
	// (would need to expose sidebar state or test via rendering)
}

func TestModelQuit(t *testing.T) {
	m := newModel()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	// tea.Quit returns a specific command, we can't easily test it here
	// but we can verify it doesn't panic
	if cmd == nil {
		t.Error("Expected quit command to be returned")
	}
}

func TestModelRendering(t *testing.T) {
	m := newModel()

	// Before ready, should show initialization message
	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Error("Expected initialization message before ready")
	}

	// After ready, should show layout
	m.ready = true
	m.width = 100
	m.height = 30
	m.header.SetWidth(100)
	m.sidebar.SetHeight(26)
	m.statusBar.SetWidth(100)

	view = m.View()
	if strings.Contains(view, "Initializing") {
		t.Error("Should not show initialization message after ready")
	}

	// Should contain some dashboard content
	if !strings.Contains(view, "Dashboard") {
		t.Error("Expected dashboard content in view")
	}
}

func TestHelpToggle(t *testing.T) {
	m := newModel()
	m.ready = true

	// Initially help should not be visible
	if m.help.IsVisible() {
		t.Error("Help should not be visible initially")
	}

	// Press '?' to toggle help
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	updated, _ := m.Update(msg)
	m = updated.(model)

	if !m.help.IsVisible() {
		t.Error("Help should be visible after pressing '?'")
	}

	// Press '?' again to hide
	updated, _ = m.Update(msg)
	m = updated.(model)

	if m.help.IsVisible() {
		t.Error("Help should be hidden after pressing '?' again")
	}
}
