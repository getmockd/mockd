package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/sse"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/websocket"
)

func TestNewConnections(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewConnections(c)

	if model.client == nil {
		t.Error("client should not be nil")
	}

	if !model.loading {
		t.Error("should start in loading state")
	}

	if model.showMessageInput {
		t.Error("message input should not be shown initially")
	}
}

func TestConnectionsUpdate_WindowSize(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewConnections(c)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := model.Update(msg)

	if updated.width != 100 {
		t.Errorf("expected width 100, got %d", updated.width)
	}

	if updated.height != 50 {
		t.Errorf("expected height 50, got %d", updated.height)
	}
}

func TestConnectionsUpdate_ConnectionsLoaded(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewConnections(c)
	model.loading = true

	wsConns := []websocket.ConnectionInfo{
		{
			ID:               "ws123",
			EndpointPath:     "/ws/test",
			ConnectedAt:      time.Now().Add(-5 * time.Minute),
			MessagesSent:     10,
			MessagesReceived: 5,
		},
	}

	sseConns := []sse.SSEStreamInfo{
		{
			ID:         "sse123",
			ClientIP:   "192.168.1.1",
			StartTime:  time.Now().Add(-2 * time.Minute),
			EventsSent: 20,
			Status:     "active",
		},
	}

	msg := connectionsLoadedMsg{
		wsConnections:  wsConns,
		sseConnections: sseConns,
	}
	updated, _ := model.Update(msg)

	if updated.loading {
		t.Error("loading should be false after connections loaded")
	}

	if len(updated.connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(updated.connections))
	}
}

func TestConnectionsFilterHandling(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewConnections(c)
	model.wsConnections = []websocket.ConnectionInfo{
		{ID: "ws1", EndpointPath: "/ws/test", ConnectedAt: time.Now()},
	}
	model.sseConnections = []sse.SSEStreamInfo{
		{ID: "sse1", ClientIP: "127.0.0.1", StartTime: time.Now()},
	}
	model.mergeConnections()

	tests := []struct {
		key            string
		expectedFilter string
		expectedCount  int
	}{
		{"1", "", 2},
		{"2", "ws", 1},
		{"3", "sse", 1},
	}

	for _, tt := range tests {
		t.Run("filter_"+tt.key, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(tt.key[0])}}
			updated, _ := model.handleKey(msg)

			if updated.typeFilter != tt.expectedFilter {
				t.Errorf("expected filter %q, got %q", tt.expectedFilter, updated.typeFilter)
			}

			if len(updated.connections) != tt.expectedCount {
				t.Errorf("expected %d connections, got %d", tt.expectedCount, len(updated.connections))
			}
		})
	}
}

func TestConnectionsMessageInput(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewConnections(c)
	model.connections = []Connection{
		{ID: "ws1", Type: "ws"},
	}

	// Press 'm' to show message input
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}
	updated, _ := model.handleKey(msg)

	if !updated.showMessageInput {
		t.Error("message input should be shown")
	}

	// Press 'esc' to hide it
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = updated.handleKey(msg)

	if updated.showMessageInput {
		t.Error("message input should be hidden")
	}
}
