package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModal(t *testing.T) {
	modal := NewModal()

	if modal.visible {
		t.Error("NewModal() should create a hidden modal")
	}

	if modal.selectedButton != 0 {
		t.Error("NewModal() should initialize with first button selected")
	}
}

func TestModalShow(t *testing.T) {
	modal := NewModal()

	onConfirm := func() tea.Msg {
		return nil
	}

	onCancel := func() tea.Msg {
		return nil
	}

	modal.Show("Test Title", "Test Message", onConfirm, onCancel)

	if !modal.visible {
		t.Error("Show() should make modal visible")
	}

	if modal.title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", modal.title)
	}

	if modal.message != "Test Message" {
		t.Errorf("Expected message 'Test Message', got '%s'", modal.message)
	}

	if modal.selectedButton != 0 {
		t.Error("Show() should reset selectedButton to 0")
	}
}

func TestModalHide(t *testing.T) {
	modal := NewModal()
	modal.Show("Title", "Message", nil, nil)

	if !modal.visible {
		t.Error("Modal should be visible after Show()")
	}

	modal.Hide()

	if modal.visible {
		t.Error("Hide() should make modal invisible")
	}
}

func TestModalIsVisible(t *testing.T) {
	modal := NewModal()

	if modal.IsVisible() {
		t.Error("IsVisible() should return false for new modal")
	}

	modal.Show("Title", "Message", nil, nil)

	if !modal.IsVisible() {
		t.Error("IsVisible() should return true after Show()")
	}

	modal.Hide()

	if modal.IsVisible() {
		t.Error("IsVisible() should return false after Hide()")
	}
}

func TestModalSetSize(t *testing.T) {
	modal := NewModal()
	modal.SetSize(100, 50)

	if modal.width != 100 {
		t.Errorf("Expected width 100, got %d", modal.width)
	}

	if modal.height != 50 {
		t.Errorf("Expected height 50, got %d", modal.height)
	}
}

func TestModalUpdateNavigation(t *testing.T) {
	modal := NewModal()
	modal.Show("Title", "Message", nil, nil)

	tests := []struct {
		name           string
		key            string
		expectedButton int
	}{
		{"Right arrow moves to No", "right", 1},
		{"Left arrow moves to Yes", "left", 0},
		{"Tab moves to No", "tab", 1},
		{"Shift+Tab moves to Yes", "shift+tab", 0},
		{"l moves to No", "l", 1},
		{"h moves to Yes", "h", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modal.selectedButton = 0 // Reset to first button
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}

			// Create appropriate key message
			switch tt.key {
			case "right":
				keyMsg = tea.KeyMsg{Type: tea.KeyRight}
			case "left":
				keyMsg = tea.KeyMsg{Type: tea.KeyLeft}
			case "tab":
				keyMsg = tea.KeyMsg{Type: tea.KeyTab}
			case "shift+tab":
				keyMsg = tea.KeyMsg{Type: tea.KeyShiftTab}
			default:
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			modal, _ = modal.Update(keyMsg)

			if modal.selectedButton != tt.expectedButton {
				t.Errorf("Expected button %d, got %d", tt.expectedButton, modal.selectedButton)
			}
		})
	}
}

func TestModalUpdateConfirm(t *testing.T) {
	modal := NewModal()
	modal.Show("Title", "Message", func() tea.Msg {
		return tea.Msg("confirmed")
	}, nil)

	modal.selectedButton = 0 // Select Yes button
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}

	modal, cmd := modal.Update(keyMsg)

	if modal.visible {
		t.Error("Modal should be hidden after confirming")
	}

	if cmd == nil {
		t.Error("Update should return command when confirming")
	}

	// Execute the command to trigger the callback
	if cmd != nil {
		msg := cmd()
		if msg == nil {
			t.Error("Command should return non-nil message")
		}
	}
}

func TestModalUpdateCancel(t *testing.T) {
	modal := NewModal()
	modal.Show("Title", "Message", nil, func() tea.Msg {
		return tea.Msg("cancelled")
	})

	modal.selectedButton = 1 // Select No button
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}

	modal, cmd := modal.Update(keyMsg)

	if modal.visible {
		t.Error("Modal should be hidden after cancelling")
	}

	if cmd == nil {
		t.Error("Update should return command when cancelling")
	}
}

func TestModalUpdateEscape(t *testing.T) {
	modal := NewModal()
	modal.Show("Title", "Message", nil, func() tea.Msg {
		return tea.Msg("cancelled")
	})

	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}

	modal, cmd := modal.Update(keyMsg)

	if modal.visible {
		t.Error("Modal should be hidden after escape")
	}

	if cmd == nil {
		t.Error("Update should return command when pressing escape")
	}
}

func TestModalView(t *testing.T) {
	modal := NewModal()
	modal.SetSize(80, 24)

	// Hidden modal should return empty string
	view := modal.View()
	if view != "" {
		t.Error("View() should return empty string when modal is hidden")
	}

	// Visible modal should return content
	modal.Show("Delete Item", "Are you sure?", nil, nil)
	view = modal.View()

	if view == "" {
		t.Error("View() should return non-empty string when modal is visible")
	}

	// Check if title and message are in the view
	if !strings.Contains(view, "Delete Item") {
		t.Error("View() should contain the title")
	}

	if !strings.Contains(view, "Are you sure?") {
		t.Error("View() should contain the message")
	}

	// Check if buttons are present
	if !strings.Contains(view, "Yes") {
		t.Error("View() should contain Yes button")
	}

	if !strings.Contains(view, "No") {
		t.Error("View() should contain No button")
	}
}

func TestModalUpdateWhenHidden(t *testing.T) {
	modal := NewModal()
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}

	newModal, cmd := modal.Update(keyMsg)

	if !modal.IsVisible() && newModal.IsVisible() {
		t.Error("Hidden modal should remain hidden after Update()")
	}

	if cmd != nil {
		t.Error("Hidden modal should not return commands")
	}
}
