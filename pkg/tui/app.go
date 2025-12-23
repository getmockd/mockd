package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/tui/client"
)

// Run starts the TUI application with default admin URL
func Run() error {
	return RunWithAdminURL("http://localhost:9090")
}

// RunWithAdminURL starts the TUI application with a custom admin URL
func RunWithAdminURL(adminURL string) error {
	// Create the program with alt screen (full-screen mode)
	p := tea.NewProgram(
		newModelWithClient(client.NewClient(adminURL)),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse support (optional)
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
