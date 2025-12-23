package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/tui/client"
)

func TestNewStreams(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewStreams(c)

	if model.client == nil {
		t.Error("client should not be nil")
	}

	if !model.loading {
		t.Error("should start in loading state")
	}

	if model.replayMode != "pure" {
		t.Errorf("expected default replay mode 'pure', got %q", model.replayMode)
	}

	if model.replayScale != 1.0 {
		t.Errorf("expected default replay scale 1.0, got %f", model.replayScale)
	}
}

func TestStreamsUpdate_WindowSize(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewStreams(c)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := model.Update(msg)

	if updated.width != 100 {
		t.Errorf("expected width 100, got %d", updated.width)
	}

	if updated.height != 50 {
		t.Errorf("expected height 50, got %d", updated.height)
	}
}

func TestStreamsUpdate_RecordingsLoaded(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewStreams(c)
	model.loading = true

	recordings := []*recording.RecordingSummary{
		{
			ID:         "test123",
			Protocol:   "websocket",
			Path:       "/ws/test",
			FrameCount: 10,
			Duration:   5000,
			FileSize:   1024,
			StartTime:  time.Now(),
		},
	}

	msg := streamRecordingsLoadedMsg{recordings: recordings}
	updated, _ := model.Update(msg)

	if updated.loading {
		t.Error("loading should be false after recordings loaded")
	}

	if len(updated.recordings) != 1 {
		t.Errorf("expected 1 recording, got %d", len(updated.recordings))
	}
}

func TestStreamsFilterHandling(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewStreams(c)

	tests := []struct {
		key            string
		expectedFilter string
	}{
		{"1", ""},
		{"2", "websocket"},
		{"3", "sse"},
	}

	for _, tt := range tests {
		t.Run("filter_"+tt.key, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(tt.key[0])}}
			updated, _ := model.handleKey(msg)

			if updated.protocolFilter != tt.expectedFilter {
				t.Errorf("expected filter %q, got %q", tt.expectedFilter, updated.protocolFilter)
			}
		})
	}
}

func TestStreamsReplayModal(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewStreams(c)
	model.recordings = []*recording.RecordingSummary{
		{ID: "test123", Protocol: "websocket"},
	}

	// Open modal with 'r' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	updated, _ := model.handleKey(msg)

	if !updated.showReplayModal {
		t.Error("replay modal should be shown")
	}

	// Change mode with '2' key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	updated, _ = updated.handleReplayModalKey(msg)

	if updated.replayMode != "synchronized" {
		t.Errorf("expected replay mode 'synchronized', got %q", updated.replayMode)
	}

	// Close modal with esc
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = updated.handleReplayModalKey(msg)

	if updated.showReplayModal {
		t.Error("replay modal should be closed")
	}
}

func TestStreamsReplayScale(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewStreams(c)
	model.showReplayModal = true
	model.replayScale = 1.0

	// Increase scale with '+'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	updated, _ := model.handleReplayModalKey(msg)

	if updated.replayScale <= 1.0 {
		t.Error("replay scale should have increased")
	}

	// Decrease scale with '-'
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	updated, _ = updated.handleReplayModalKey(msg)

	if updated.replayScale >= 1.1 {
		t.Error("replay scale should have decreased")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
		}
	}
}

func TestMinMax(t *testing.T) {
	if min(5.0, 3.0) != 3.0 {
		t.Error("min(5.0, 3.0) should be 3.0")
	}

	if max(5.0, 3.0) != 5.0 {
		t.Error("max(5.0, 3.0) should be 5.0")
	}
}
