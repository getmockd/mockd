package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// HeaderModel represents the header component
type HeaderModel struct {
	width      int
	recording  bool
	serverPort int
}

// NewHeader creates a new header component
func NewHeader() HeaderModel {
	return HeaderModel{
		width:      80,
		recording:  false,
		serverPort: 8080,
	}
}

// SetWidth updates the header width
func (h *HeaderModel) SetWidth(width int) {
	h.width = width
}

// SetRecording updates the recording status
func (h *HeaderModel) SetRecording(recording bool) {
	h.recording = recording
}

// SetServerPort updates the server port
func (h *HeaderModel) SetServerPort(port int) {
	h.serverPort = port
}

// View renders the header
func (h HeaderModel) View() string {
	// Left: Logo and title
	logo := styles.HeaderStyle.Render("mockd")

	// Center: Server status (if space allows)
	var status string
	if h.width > 60 {
		status = fmt.Sprintf(":%d", h.serverPort)
	}

	// Right: Recording indicator and time
	var right string
	if h.recording {
		recordingBadge := styles.BadgeErrorStyle.Render("● REC")
		right = recordingBadge + " "
	}
	currentTime := time.Now().Format("15:04")
	right += currentTime

	// Calculate spacing
	statusWidth := lipgloss.Width(status)
	rightWidth := lipgloss.Width(right)
	leftWidth := lipgloss.Width(logo)

	// Available space for padding
	totalContent := leftWidth + statusWidth + rightWidth
	if totalContent >= h.width {
		// If we don't have enough space, just show logo and time
		spacer := h.width - leftWidth - rightWidth
		if spacer < 0 {
			spacer = 0
		}
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			logo,
			styles.HeaderStyle.Render(lipgloss.PlaceHorizontal(spacer, lipgloss.Right, "")),
			styles.HeaderStyle.Render(right),
		)
	}

	// Calculate padding to distribute content
	leftPadding := (h.width - totalContent) / 2
	rightPadding := h.width - totalContent - leftPadding

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		logo,
		styles.HeaderStyle.Render(lipgloss.PlaceHorizontal(leftPadding, lipgloss.Right, status)),
		styles.HeaderStyle.Render(lipgloss.PlaceHorizontal(rightPadding, lipgloss.Right, right)),
	)
}
