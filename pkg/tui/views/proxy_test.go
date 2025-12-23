package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/tui/client"
)

func TestNewProxy(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewProxy(c)

	if model.client == nil {
		t.Error("client should not be nil")
	}

	if !model.loading {
		t.Error("should start in loading state")
	}

	if model.showTargetInput {
		t.Error("target input should not be shown initially")
	}
}

func TestProxyUpdate_WindowSize(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewProxy(c)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := model.Update(msg)

	if updated.width != 100 {
		t.Errorf("expected width 100, got %d", updated.width)
	}

	if updated.height != 50 {
		t.Errorf("expected height 50, got %d", updated.height)
	}
}

func TestProxyUpdate_StatusLoaded(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewProxy(c)
	model.loading = true

	status := &admin.ProxyStatusResponse{
		Running: true,
		Port:    8443,
		Mode:    "record",
	}

	msg := proxyStatusLoadedMsg{status: status}
	updated, _ := model.Update(msg)

	if updated.loading {
		t.Error("loading should be false after status loaded")
	}

	if updated.status == nil {
		t.Error("status should not be nil")
	}

	if !updated.status.Running {
		t.Error("proxy should be running")
	}
}

func TestProxyTargetInputToggle(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewProxy(c)
	model.status = &admin.ProxyStatusResponse{Running: true}

	// Press 't' to show target input
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	updated, _ := model.handleKey(msg)

	if !updated.showTargetInput {
		t.Error("target input should be shown")
	}

	// Press 'esc' to hide it
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = updated.handleKey(msg)

	if updated.showTargetInput {
		t.Error("target input should be hidden")
	}
}

func TestProxyView(t *testing.T) {
	c := client.NewDefaultClient()
	model := NewProxy(c)
	model.status = &admin.ProxyStatusResponse{
		Running: true,
		Port:    8443,
		Mode:    "pass-through",
	}
	model.loading = false

	view := model.View()
	if view == "" {
		t.Error("view should not be empty")
	}
}
