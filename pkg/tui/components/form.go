package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// FormField represents a single form input field.
type FormField struct {
	Label       string
	Placeholder string
	Value       string
	Input       textinput.Model
	Multiline   bool // For future textarea support
	Required    bool
	Validator   func(string) error
}

// FormModel represents a form with multiple input fields.
type FormModel struct {
	title      string
	fields     []FormField
	focusIndex int
	width      int
	height     int
	submitted  bool
	cancelled  bool
}

// NewForm creates a new form with the given title.
func NewForm(title string) FormModel {
	return FormModel{
		title:      title,
		fields:     []FormField{},
		focusIndex: 0,
		submitted:  false,
		cancelled:  false,
	}
}

// AddField adds a field to the form.
func (m *FormModel) AddField(label, placeholder, value string, required bool, validator func(string) error) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	// Initial width - will be updated by SetSize
	ti.Width = 60

	field := FormField{
		Label:       label,
		Placeholder: placeholder,
		Value:       value,
		Input:       ti,
		Required:    required,
		Validator:   validator,
	}

	m.fields = append(m.fields, field)

	// Focus the first field
	if len(m.fields) == 1 {
		m.fields[0].Input.Focus()
	}
}

// SetSize sets the form dimensions.
func (m *FormModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate responsive input width
	// Account for label width (15), borders/padding (6), and margins
	inputWidth := width - 25

	// Set minimum and maximum widths
	if inputWidth < 40 {
		inputWidth = 40
	}
	if inputWidth > 80 {
		inputWidth = 80
	}

	// Update all input field widths
	for i := range m.fields {
		m.fields[i].Input.Width = inputWidth
	}
}

// Reset resets the form state.
func (m *FormModel) Reset() {
	m.submitted = false
	m.cancelled = false
	m.focusIndex = 0

	// Clear all fields
	for i := range m.fields {
		m.fields[i].Input.SetValue("")
		m.fields[i].Value = ""
	}

	// Focus first field
	if len(m.fields) > 0 {
		m.fields[0].Input.Focus()
		for i := 1; i < len(m.fields); i++ {
			m.fields[i].Input.Blur()
		}
	}
}

// SetValues sets the form field values (for editing).
func (m *FormModel) SetValues(values map[string]string) {
	for i := range m.fields {
		if val, ok := values[m.fields[i].Label]; ok {
			m.fields[i].Input.SetValue(val)
			m.fields[i].Value = val
		}
	}
}

// GetValues returns the current form values.
func (m FormModel) GetValues() map[string]string {
	values := make(map[string]string)
	for _, field := range m.fields {
		values[field.Label] = field.Input.Value()
	}
	return values
}

// Validate validates all form fields.
func (m FormModel) Validate() error {
	for _, field := range m.fields {
		value := field.Input.Value()

		// Check required fields
		if field.Required && strings.TrimSpace(value) == "" {
			return &ValidationError{Field: field.Label, Message: "This field is required"}
		}

		// Run custom validator
		if field.Validator != nil {
			if err := field.Validator(value); err != nil {
				return &ValidationError{Field: field.Label, Message: err.Error()}
			}
		}
	}
	return nil
}

// ValidationError represents a form validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// IsSubmitted returns whether the form was submitted.
func (m FormModel) IsSubmitted() bool {
	return m.submitted
}

// IsCancelled returns whether the form was cancelled.
func (m FormModel) IsCancelled() bool {
	return m.cancelled
}

// Init initializes the form.
func (m FormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m FormModel) Update(msg tea.Msg) (FormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, nil

		case "tab", "down", "j":
			// Move to next field
			if len(m.fields) > 0 {
				m.fields[m.focusIndex].Input.Blur()
				m.focusIndex = (m.focusIndex + 1) % len(m.fields)
				m.fields[m.focusIndex].Input.Focus()
			}
			return m, nil

		case "shift+tab", "up", "k":
			// Move to previous field
			if len(m.fields) > 0 {
				m.fields[m.focusIndex].Input.Blur()
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.fields) - 1
				}
				m.fields[m.focusIndex].Input.Focus()
			}
			return m, nil

		case "ctrl+d", "ctrl+enter":
			// Submit form (Ctrl+D instead of Ctrl+D which conflicts with terminal XOFF)
			m.submitted = true
			return m, nil

		case "enter":
			// Enter on last field submits
			if m.focusIndex == len(m.fields)-1 {
				m.submitted = true
				return m, nil
			}
			// Otherwise move to next field
			if len(m.fields) > 0 {
				m.fields[m.focusIndex].Input.Blur()
				m.focusIndex = (m.focusIndex + 1) % len(m.fields)
				m.fields[m.focusIndex].Input.Focus()
			}
			return m, nil
		}

	case tea.MouseMsg:
		// Handle mouse clicks on fields
		if msg.Type == tea.MouseLeft {
			// Calculate which field was clicked based on Y position
			// Title takes 2 lines, each field takes 1 line
			clickedLine := msg.Y - 2
			if clickedLine >= 0 && clickedLine < len(m.fields) {
				// Switch focus to clicked field
				if m.focusIndex != clickedLine {
					m.fields[m.focusIndex].Input.Blur()
					m.focusIndex = clickedLine
					m.fields[m.focusIndex].Input.Focus()
				}
			}
		}
	}

	// Update focused field
	if len(m.fields) > 0 && m.focusIndex < len(m.fields) {
		var cmd tea.Cmd
		m.fields[m.focusIndex].Input, cmd = m.fields[m.focusIndex].Input.Update(msg)
		// Update the value
		m.fields[m.focusIndex].Value = m.fields[m.focusIndex].Input.Value()
		return m, cmd
	}

	return m, nil
}

// View renders the form.
func (m FormModel) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary).
		MarginBottom(0)
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	// Render fields
	for i, field := range m.fields {
		// Label
		labelStyle := styles.FormLabelStyle.Copy().Width(15)
		if field.Required {
			labelStyle = labelStyle.Copy().Foreground(styles.ColorWarning)
		}

		label := field.Label
		if field.Required {
			label += " *"
		}

		b.WriteString(labelStyle.Render(label))
		b.WriteString(" ")

		// Input
		inputStyle := styles.FormInputStyle
		if i == m.focusIndex {
			inputStyle = styles.FormInputFocusedStyle
		}

		b.WriteString(inputStyle.Render(field.Input.View()))
		b.WriteString("\n")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(styles.ColorMuted).
		MarginTop(1)

	help := "Tab: next field • Shift+Tab: previous field • Ctrl+D: submit • Esc: cancel"
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(help))

	return b.String()
}
