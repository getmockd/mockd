package components

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewForm(t *testing.T) {
	form := NewForm("Test Form")

	if form.title != "Test Form" {
		t.Errorf("Expected title 'Test Form', got '%s'", form.title)
	}

	if len(form.fields) != 0 {
		t.Error("NewForm() should create form with no fields")
	}

	if form.focusIndex != 0 {
		t.Error("NewForm() should initialize with focusIndex 0")
	}

	if form.submitted {
		t.Error("NewForm() should initialize with submitted = false")
	}

	if form.cancelled {
		t.Error("NewForm() should initialize with cancelled = false")
	}
}

func TestFormAddField(t *testing.T) {
	form := NewForm("Test Form")

	form.AddField("Name", "Enter name", "default", true, nil)

	if len(form.fields) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(form.fields))
	}

	field := form.fields[0]
	if field.Label != "Name" {
		t.Errorf("Expected label 'Name', got '%s'", field.Label)
	}

	if field.Placeholder != "Enter name" {
		t.Errorf("Expected placeholder 'Enter name', got '%s'", field.Placeholder)
	}

	if field.Value != "default" {
		t.Errorf("Expected value 'default', got '%s'", field.Value)
	}

	if !field.Required {
		t.Error("Expected field to be required")
	}

	// First field should be focused
	if !field.Input.Focused() {
		t.Error("First field should be focused")
	}
}

func TestFormAddMultipleFields(t *testing.T) {
	form := NewForm("Test Form")

	form.AddField("Field1", "Placeholder1", "Value1", true, nil)
	form.AddField("Field2", "Placeholder2", "Value2", false, nil)
	form.AddField("Field3", "Placeholder3", "Value3", true, nil)

	if len(form.fields) != 3 {
		t.Fatalf("Expected 3 fields, got %d", len(form.fields))
	}

	// Only first field should be focused
	if !form.fields[0].Input.Focused() {
		t.Error("First field should be focused")
	}

	if form.fields[1].Input.Focused() {
		t.Error("Second field should not be focused")
	}

	if form.fields[2].Input.Focused() {
		t.Error("Third field should not be focused")
	}
}

func TestFormSetSize(t *testing.T) {
	form := NewForm("Test Form")
	form.SetSize(100, 50)

	if form.width != 100 {
		t.Errorf("Expected width 100, got %d", form.width)
	}

	if form.height != 50 {
		t.Errorf("Expected height 50, got %d", form.height)
	}
}

func TestFormReset(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField("Name", "Enter name", "initial", true, nil)
	form.AddField("Email", "Enter email", "test@example.com", false, nil)

	// Modify state
	form.submitted = true
	form.cancelled = true
	form.focusIndex = 1
	form.fields[0].Input.SetValue("modified")

	// Reset
	form.Reset()

	if form.submitted {
		t.Error("Reset() should set submitted to false")
	}

	if form.cancelled {
		t.Error("Reset() should set cancelled to false")
	}

	if form.focusIndex != 0 {
		t.Error("Reset() should reset focusIndex to 0")
	}

	if form.fields[0].Input.Value() != "" {
		t.Error("Reset() should clear field values")
	}

	if !form.fields[0].Input.Focused() {
		t.Error("Reset() should focus first field")
	}

	if form.fields[1].Input.Focused() {
		t.Error("Reset() should blur other fields")
	}
}

func TestFormSetValues(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField("Name", "Enter name", "", true, nil)
	form.AddField("Email", "Enter email", "", false, nil)
	form.AddField("Age", "Enter age", "", false, nil)

	values := map[string]string{
		"Name":  "John Doe",
		"Email": "john@example.com",
		"Age":   "30",
	}

	form.SetValues(values)

	if form.fields[0].Input.Value() != "John Doe" {
		t.Errorf("Expected Name to be 'John Doe', got '%s'", form.fields[0].Input.Value())
	}

	if form.fields[1].Input.Value() != "john@example.com" {
		t.Errorf("Expected Email to be 'john@example.com', got '%s'", form.fields[1].Input.Value())
	}

	if form.fields[2].Input.Value() != "30" {
		t.Errorf("Expected Age to be '30', got '%s'", form.fields[2].Input.Value())
	}
}

func TestFormGetValues(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField("Name", "Enter name", "John", true, nil)
	form.AddField("Email", "Enter email", "john@example.com", false, nil)

	values := form.GetValues()

	if values["Name"] != "John" {
		t.Errorf("Expected Name value 'John', got '%s'", values["Name"])
	}

	if values["Email"] != "john@example.com" {
		t.Errorf("Expected Email value 'john@example.com', got '%s'", values["Email"])
	}
}

func TestFormValidate(t *testing.T) {
	tests := []struct {
		name   string
		fields []struct {
			label     string
			value     string
			required  bool
			validator func(string) error
		}
		expectError bool
		errorField  string
	}{
		{
			name: "Valid form with required fields",
			fields: []struct {
				label     string
				value     string
				required  bool
				validator func(string) error
			}{
				{"Name", "John", true, nil},
				{"Email", "john@example.com", false, nil},
			},
			expectError: false,
		},
		{
			name: "Missing required field",
			fields: []struct {
				label     string
				value     string
				required  bool
				validator func(string) error
			}{
				{"Name", "", true, nil},
				{"Email", "john@example.com", false, nil},
			},
			expectError: true,
			errorField:  "Name",
		},
		{
			name: "Custom validator fails",
			fields: []struct {
				label     string
				value     string
				required  bool
				validator func(string) error
			}{
				{"Age", "invalid", false, func(v string) error {
					if v != "valid" {
						return fmt.Errorf("must be 'valid'")
					}
					return nil
				}},
			},
			expectError: true,
			errorField:  "Age",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := NewForm("Test Form")

			for _, f := range tt.fields {
				form.AddField(f.label, "", f.value, f.required, f.validator)
			}

			err := form.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if tt.expectError && err != nil {
				validationErr, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("Expected ValidationError, got %T", err)
				} else if validationErr.Field != tt.errorField {
					t.Errorf("Expected error for field '%s', got '%s'", tt.errorField, validationErr.Field)
				}
			}
		})
	}
}

func TestFormIsSubmitted(t *testing.T) {
	form := NewForm("Test Form")

	if form.IsSubmitted() {
		t.Error("New form should not be submitted")
	}

	form.submitted = true

	if !form.IsSubmitted() {
		t.Error("Form should be submitted after setting submitted = true")
	}
}

func TestFormIsCancelled(t *testing.T) {
	form := NewForm("Test Form")

	if form.IsCancelled() {
		t.Error("New form should not be cancelled")
	}

	form.cancelled = true

	if !form.IsCancelled() {
		t.Error("Form should be cancelled after setting cancelled = true")
	}
}

func TestFormUpdateNavigation(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField("Field1", "", "", false, nil)
	form.AddField("Field2", "", "", false, nil)
	form.AddField("Field3", "", "", false, nil)

	tests := []struct {
		name          string
		key           tea.KeyMsg
		initialFocus  int
		expectedFocus int
	}{
		{
			"Tab moves to next field",
			tea.KeyMsg{Type: tea.KeyTab},
			0,
			1,
		},
		{
			"Down moves to next field",
			tea.KeyMsg{Type: tea.KeyDown},
			0,
			1,
		},
		{
			"Up moves to previous field",
			tea.KeyMsg{Type: tea.KeyUp},
			1,
			0,
		},
		{
			"Tab wraps around to first field",
			tea.KeyMsg{Type: tea.KeyTab},
			2,
			0,
		},
		{
			"Up wraps around to last field",
			tea.KeyMsg{Type: tea.KeyUp},
			0,
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form.focusIndex = tt.initialFocus
			form, _ = form.Update(tt.key)

			if form.focusIndex != tt.expectedFocus {
				t.Errorf("Expected focus index %d, got %d", tt.expectedFocus, form.focusIndex)
			}
		})
	}
}

func TestFormUpdateSubmit(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField("Name", "", "John", false, nil)

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlS}
	form, _ = form.Update(keyMsg)

	if !form.IsSubmitted() {
		t.Error("Form should be submitted after Ctrl+S")
	}
}

func TestFormUpdateCancel(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField("Name", "", "John", false, nil)

	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	form, _ = form.Update(keyMsg)

	if !form.IsCancelled() {
		t.Error("Form should be cancelled after Esc")
	}
}

func TestFormView(t *testing.T) {
	form := NewForm("My Form")
	form.AddField("Name", "Enter name", "John", true, nil)
	form.AddField("Email", "Enter email", "", false, nil)

	view := form.View()

	if !strings.Contains(view, "My Form") {
		t.Error("View should contain form title")
	}

	if !strings.Contains(view, "Name") {
		t.Error("View should contain field labels")
	}

	if !strings.Contains(view, "Email") {
		t.Error("View should contain all field labels")
	}

	if !strings.Contains(view, "*") {
		t.Error("View should mark required fields with *")
	}

	if !strings.Contains(view, "Tab") {
		t.Error("View should contain help text")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "Email",
		Message: "Invalid email format",
	}

	expected := "Email: Invalid email format"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}
