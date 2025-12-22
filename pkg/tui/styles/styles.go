// Package styles provides styling for the mockd TUI
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#7D56F4")
	ColorSecondary = lipgloss.Color("#56B6C2")
	ColorSuccess   = lipgloss.Color("#98C379")
	ColorWarning   = lipgloss.Color("#E5C07B")
	ColorError     = lipgloss.Color("#E06C75")
	ColorInfo      = lipgloss.Color("#61AFEF")

	// UI colors
	ColorForeground = lipgloss.Color("#FAFAFA")
	ColorBackground = lipgloss.Color("#282C34")
	ColorMuted      = lipgloss.Color("#5C6370")
	ColorBorder     = lipgloss.Color("#3E4451")
	ColorHighlight  = lipgloss.Color("#528BFF")

	// Status colors
	ColorActive   = lipgloss.Color("#98C379")
	ColorInactive = lipgloss.Color("#5C6370")
	ColorEnabled  = lipgloss.Color("#98C379")
	ColorDisabled = lipgloss.Color("#E06C75")
)

// Base styles
var (
	// Base style for the entire application
	BaseStyle = lipgloss.NewStyle().
			Foreground(ColorForeground).
			Background(ColorBackground)

	// Border styles
	RoundedBorder = lipgloss.RoundedBorder()
	NormalBorder  = lipgloss.NormalBorder()
	ThickBorder   = lipgloss.ThickBorder()
	DoubleBorder  = lipgloss.DoubleBorder()
)

// Component styles
var (
	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorForeground).
			Background(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginLeft(1)

	// Sidebar styles
	SidebarStyle = lipgloss.NewStyle().
			BorderStyle(RoundedBorder).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Width(20)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(ColorForeground)

	SidebarItemActiveStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Background(lipgloss.Color("#3E4451")).
				Bold(true)

	// Content area styles
	ContentStyle = lipgloss.NewStyle().
			BorderStyle(RoundedBorder).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(lipgloss.Color("#21252B")).
			Padding(0, 1)

	StatusBarKeyStyle = lipgloss.NewStyle().
				Foreground(ColorHighlight).
				Bold(true)

	StatusBarValueStyle = lipgloss.NewStyle().
				Foreground(ColorForeground)

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(ColorBorder)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(ColorForeground)

	TableRowSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Background(lipgloss.Color("#3E4451")).
				Bold(true)

	// Help styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Bold(true)

	// Modal/Dialog styles
	ModalStyle = lipgloss.NewStyle().
			BorderStyle(RoundedBorder).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Background(ColorBackground)

	ModalTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Align(lipgloss.Center)

	// Form styles
	FormLabelStyle = lipgloss.NewStyle().
			Foreground(ColorInfo).
			Bold(true).
			Width(15)

	FormInputStyle = lipgloss.NewStyle().
			Foreground(ColorForeground).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	FormInputFocusedStyle = lipgloss.NewStyle().
				Foreground(ColorForeground).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	// Badge styles
	BadgeSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(ColorSuccess).
				Bold(true).
				Padding(0, 1)

	BadgeWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(ColorWarning).
				Bold(true).
				Padding(0, 1)

	BadgeErrorStyle = lipgloss.NewStyle().
			Foreground(ColorForeground).
			Background(ColorError).
			Bold(true).
			Padding(0, 1)

	BadgeInfoStyle = lipgloss.NewStyle().
			Foreground(ColorForeground).
			Background(ColorInfo).
			Bold(true).
			Padding(0, 1)

	// Loading/Spinner styles
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// Error message style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	// Success message style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)
)

// GetFocusedStyle returns focused variant of a style
func GetFocusedStyle(base lipgloss.Style) lipgloss.Style {
	return base.Copy().BorderForeground(ColorPrimary)
}

// GetUnfocusedStyle returns unfocused variant of a style
func GetUnfocusedStyle(base lipgloss.Style) lipgloss.Style {
	return base.Copy().BorderForeground(ColorBorder)
}
