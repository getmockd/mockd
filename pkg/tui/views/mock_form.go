package views

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/components"
)

// MockFormMode represents whether we're creating or editing a mock.
type MockFormMode int

const (
	MockFormModeCreate MockFormMode = iota
	MockFormModeEdit
)

// MockFormModel represents the create/edit mock form.
type MockFormModel struct {
	client *client.Client
	form   components.FormModel
	mode   MockFormMode
	mockID string // For edit mode

	width  int
	height int
}

// NewMockForm creates a new mock form.
func NewMockForm(adminClient *client.Client) MockFormModel {
	form := components.NewForm("Create Mock")

	// Add form fields with default values
	form.AddField("Name", "My API Mock", "", false, nil)
	form.AddField("Method", "GET", "GET", true, validateMethod)
	form.AddField("Path", "/api/users", "/api/users", true, validatePath)
	form.AddField("Status", "200", "200", true, validateStatus)
	form.AddField("Headers", "{}", "{}", false, validateJSON)
	form.AddField("Body", "{}", "{}", false, nil)
	form.AddField("Delay (ms)", "0", "0", false, validateDelay)

	return MockFormModel{
		client: adminClient,
		form:   form,
		mode:   MockFormModeCreate,
	}
}

// SetMode sets the form mode (create or edit).
func (m *MockFormModel) SetMode(mode MockFormMode, mock *config.MockConfiguration) {
	m.mode = mode

	if mode == MockFormModeEdit && mock != nil {
		m.mockID = mock.ID
		m.form = components.NewForm("Edit Mock")

		// Prepare values
		values := make(map[string]string)
		values["Name"] = mock.Name
		values["Method"] = mock.Matcher.Method
		values["Path"] = mock.Matcher.Path

		if mock.Response != nil {
			values["Status"] = strconv.Itoa(mock.Response.StatusCode)
			values["Body"] = mock.Response.Body
			values["Delay (ms)"] = strconv.Itoa(mock.Response.DelayMs)

			// Serialize headers to JSON
			if len(mock.Response.Headers) > 0 {
				headersJSON, _ := json.MarshalIndent(mock.Response.Headers, "", "  ")
				values["Headers"] = string(headersJSON)
			} else {
				values["Headers"] = "{}"
			}
		} else {
			values["Status"] = "200"
			values["Body"] = "{}"
			values["Headers"] = "{}"
			values["Delay (ms)"] = "0"
		}

		// Re-add fields with validators
		m.form.AddField("Name", "My API Mock", values["Name"], false, nil)
		m.form.AddField("Method", "GET", values["Method"], true, validateMethod)
		m.form.AddField("Path", "/api/users", values["Path"], true, validatePath)
		m.form.AddField("Status", "200", values["Status"], true, validateStatus)
		m.form.AddField("Headers", "{}", values["Headers"], false, validateJSON)
		m.form.AddField("Body", "{}", values["Body"], false, nil)
		m.form.AddField("Delay (ms)", "0", values["Delay (ms)"], false, validateDelay)
	} else {
		// Create mode - reset form
		m.form = components.NewForm("Create Mock")
		m.form.AddField("Name", "My API Mock", "", false, nil)
		m.form.AddField("Method", "GET", "GET", true, validateMethod)
		m.form.AddField("Path", "/api/users", "/api/users", true, validatePath)
		m.form.AddField("Status", "200", "200", true, validateStatus)
		m.form.AddField("Headers", "{}", "{}", false, validateJSON)
		m.form.AddField("Body", "{}", "{}", false, nil)
		m.form.AddField("Delay (ms)", "0", "0", false, validateDelay)
		m.mockID = ""
	}
}

// Reset resets the form.
func (m *MockFormModel) Reset() {
	m.form.Reset()
	m.mode = MockFormModeCreate
	m.mockID = ""
}

// SetSize sets the form dimensions.
func (m *MockFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.form.SetSize(width, height)
}

// Init initializes the form.
func (m MockFormModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages.
func (m MockFormModel) Update(msg tea.Msg) (MockFormModel, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)

	// Check if form was submitted
	if m.form.IsSubmitted() {
		// Validate
		if err := m.form.Validate(); err != nil {
			return m, func() tea.Msg {
				return mockFormErrorMsg{err: err}
			}
		}

		// Submit the mock
		return m, m.submitMock()
	}

	// Check if form was cancelled
	if m.form.IsCancelled() {
		return m, func() tea.Msg {
			return mockFormCancelledMsg{}
		}
	}

	return m, cmd
}

// View renders the form.
func (m MockFormModel) View() string {
	return m.form.View()
}

// submitMock submits the form data to create or update a mock.
func (m MockFormModel) submitMock() tea.Cmd {
	return func() tea.Msg {
		values := m.form.GetValues()

		// Parse headers from JSON
		var headers map[string]string
		headersStr := strings.TrimSpace(values["Headers"])
		if headersStr == "" || headersStr == "{}" {
			headers = make(map[string]string)
		} else {
			if err := json.Unmarshal([]byte(headersStr), &headers); err != nil {
				return mockFormErrorMsg{err: fmt.Errorf("invalid headers JSON: %w", err)}
			}
		}

		// Parse status code
		status, err := strconv.Atoi(values["Status"])
		if err != nil {
			return mockFormErrorMsg{err: fmt.Errorf("invalid status code: %w", err)}
		}

		// Parse delay
		delay := 0
		delayStr := strings.TrimSpace(values["Delay (ms)"])
		if delayStr != "" && delayStr != "0" {
			delay, err = strconv.Atoi(delayStr)
			if err != nil {
				return mockFormErrorMsg{err: fmt.Errorf("invalid delay: %w", err)}
			}
		}

		// Build mock configuration
		mock := &config.MockConfiguration{
			Name:    values["Name"],
			Enabled: true,
			Matcher: &config.RequestMatcher{
				Method: strings.ToUpper(values["Method"]),
				Path:   values["Path"],
			},
			Response: &config.ResponseDefinition{
				StatusCode: status,
				Headers:    headers,
				Body:       values["Body"],
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

// Validators

func validateMethod(value string) error {
	value = strings.ToUpper(strings.TrimSpace(value))
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range validMethods {
		if value == m {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(validMethods, ", "))
}

func validatePath(value string) error {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("must start with /")
	}
	return nil
}

func validateStatus(value string) error {
	status, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if status < 100 || status > 599 {
		return fmt.Errorf("must be between 100 and 599")
	}
	return nil
}

func validateJSON(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "{}" {
		return nil
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return fmt.Errorf("must be valid JSON object")
	}
	return nil
}

func validateDelay(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return nil
	}

	delay, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if delay < 0 {
		return fmt.Errorf("must be >= 0")
	}
	return nil
}

// Messages

type mockFormSubmittedMsg struct {
	mock *config.MockConfiguration
}

type mockFormCancelledMsg struct{}

type mockFormErrorMsg struct {
	err error
}
