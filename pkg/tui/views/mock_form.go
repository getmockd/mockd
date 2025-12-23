package views

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// MockFormMode represents whether we're creating or editing a mock.
type MockFormMode int

const (
	MockFormModeCreate MockFormMode = iota
	MockFormModeEdit
)

// MockFormModel represents the create/edit mock form using Huh.
type MockFormModel struct {
	client *client.Client
	form   *huh.Form
	mode   MockFormMode
	mockID string // For edit mode

	// Form field values
	name    string
	method  string
	path    string
	status  string
	headers string
	body    string
	delay   string

	width  int
	height int

	// State
	completed bool
	cancelled bool
	err       error
}

// NewMockForm creates a new mock form using Huh.
func NewMockForm(adminClient *client.Client) MockFormModel {
	m := MockFormModel{
		client: adminClient,
		mode:   MockFormModeCreate,
		// Set default values
		method:  "GET",
		path:    "/api/users",
		status:  "200",
		headers: "{}",
		body:    "{}",
		delay:   "0",
	}

	m.buildForm()
	return m
}

// buildForm constructs the Huh form with all fields.
func (m *MockFormModel) buildForm() {
	// Use Huh's form builder with groups
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Placeholder("My API Mock").
				Value(&m.name).
				Description("Optional: A friendly name for this mock"),

			huh.NewSelect[string]().
				Title("Method").
				Options(
					huh.NewOption("GET", "GET"),
					huh.NewOption("POST", "POST"),
					huh.NewOption("PUT", "PUT"),
					huh.NewOption("DELETE", "DELETE"),
					huh.NewOption("PATCH", "PATCH"),
					huh.NewOption("HEAD", "HEAD"),
					huh.NewOption("OPTIONS", "OPTIONS"),
				).
				Value(&m.method).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("method is required")
					}
					return nil
				}),

			huh.NewInput().
				Title("Path").
				Placeholder("/api/users").
				Value(&m.path).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("path is required")
					}
					if !strings.HasPrefix(s, "/") {
						return fmt.Errorf("path must start with /")
					}
					return nil
				}),

			huh.NewInput().
				Title("Status Code").
				Placeholder("200").
				Value(&m.status).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("status is required")
					}
					status, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if status < 100 || status > 599 {
						return fmt.Errorf("must be between 100 and 599")
					}
					return nil
				}),

			huh.NewText().
				Title("Headers (JSON)").
				Placeholder("{}").
				Value(&m.headers).
				CharLimit(1000).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" || s == "{}" {
						return nil
					}
					var data map[string]string
					if err := json.Unmarshal([]byte(s), &data); err != nil {
						return fmt.Errorf("must be valid JSON object")
					}
					return nil
				}),

			huh.NewText().
				Title("Body").
				Placeholder("{}").
				Value(&m.body).
				CharLimit(10000).
				Lines(3),

			huh.NewInput().
				Title("Delay (ms)").
				Placeholder("0").
				Value(&m.delay).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" || s == "0" {
						return nil
					}
					delay, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if delay < 0 {
						return fmt.Errorf("must be >= 0")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCharm()).
		WithShowHelp(true).
		WithShowErrors(true)
}

// SetMode sets the form mode (create or edit).
func (m *MockFormModel) SetMode(mode MockFormMode, mock *config.MockConfiguration) {
	m.mode = mode

	if mode == MockFormModeEdit && mock != nil {
		m.mockID = mock.ID
		m.name = mock.Name
		m.method = mock.Matcher.Method
		m.path = mock.Matcher.Path

		if mock.Response != nil {
			m.status = strconv.Itoa(mock.Response.StatusCode)
			m.body = mock.Response.Body
			m.delay = strconv.Itoa(mock.Response.DelayMs)

			// Serialize headers to JSON
			if len(mock.Response.Headers) > 0 {
				headersJSON, _ := json.MarshalIndent(mock.Response.Headers, "", "  ")
				m.headers = string(headersJSON)
			} else {
				m.headers = "{}"
			}
		} else {
			m.status = "200"
			m.body = "{}"
			m.headers = "{}"
			m.delay = "0"
		}
	} else {
		// Create mode - reset to defaults
		m.mockID = ""
		m.name = ""
		m.method = "GET"
		m.path = "/api/users"
		m.status = "200"
		m.headers = "{}"
		m.body = "{}"
		m.delay = "0"
	}

	m.completed = false
	m.cancelled = false
	m.err = nil

	// Rebuild form with new values
	m.buildForm()
}

// Reset resets the form to create mode.
func (m *MockFormModel) Reset() {
	m.SetMode(MockFormModeCreate, nil)
}

// SetSize sets the form dimensions.
func (m *MockFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	if m.form != nil {
		m.form = m.form.WithWidth(width - 4) // Account for padding
	}
}

// Init initializes the form.
func (m MockFormModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages.
func (m MockFormModel) Update(msg tea.Msg) (MockFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			// Cancel form
			m.cancelled = true
			return m, func() tea.Msg {
				return mockFormCancelledMsg{}
			}
		}
	}

	// Update the form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	// Check if form is complete
	if m.form.State == huh.StateCompleted {
		m.completed = true
		return m, m.submitMock()
	}

	return m, cmd
}

// View renders the form.
func (m MockFormModel) View() string {
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(styles.ColorError).
			Bold(true).
			MarginBottom(1)
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + m.form.View()
	}

	return m.form.View()
}

// submitMock submits the form data to create or update a mock.
func (m MockFormModel) submitMock() tea.Cmd {
	return func() tea.Msg {
		// Parse headers from JSON
		var headers map[string]string
		headersStr := strings.TrimSpace(m.headers)
		if headersStr == "" || headersStr == "{}" {
			headers = make(map[string]string)
		} else {
			if err := json.Unmarshal([]byte(headersStr), &headers); err != nil {
				return mockFormErrorMsg{err: fmt.Errorf("invalid headers JSON: %w", err)}
			}
		}

		// Parse status code
		status, err := strconv.Atoi(m.status)
		if err != nil {
			return mockFormErrorMsg{err: fmt.Errorf("invalid status code: %w", err)}
		}

		// Parse delay
		delay := 0
		delayStr := strings.TrimSpace(m.delay)
		if delayStr != "" && delayStr != "0" {
			delay, err = strconv.Atoi(delayStr)
			if err != nil {
				return mockFormErrorMsg{err: fmt.Errorf("invalid delay: %w", err)}
			}
		}

		// Build mock configuration
		mock := &config.MockConfiguration{
			Name:    m.name,
			Enabled: true,
			Matcher: &config.RequestMatcher{
				Method: strings.ToUpper(m.method),
				Path:   m.path,
			},
			Response: &config.ResponseDefinition{
				StatusCode: status,
				Headers:    headers,
				Body:       m.body,
				DelayMs:    delay,
			},
		}

		// Create or update
		var result *config.MockConfiguration
		if m.mode == MockFormModeEdit && m.mockID != "" {
			result, err = m.client.UpdateMock(m.mockID, mock)
		} else {
			result, err = m.client.CreateMock(mock)
		}

		if err != nil {
			return mockFormErrorMsg{err: err}
		}

		return mockFormSubmittedMsg{mock: result}
	}
}

// Messages

type mockFormSubmittedMsg struct {
	mock *config.MockConfiguration
}

type mockFormCancelledMsg struct{}

type mockFormErrorMsg struct {
	err error
}
