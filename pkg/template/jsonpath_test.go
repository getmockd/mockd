package template

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONPathTemplateVariables(t *testing.T) {
	engine := New()

	// Create a context with JSON body
	body := `{"user": {"name": "John", "email": "john@example.com"}, "status": "active", "count": 42}`
	r := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := NewContext(r, []byte(body))

	// Simulate matched JSONPath values (as would be set by the matcher)
	ctx.SetJSONPathMatches(map[string]interface{}{
		"user_name":  "John",
		"user_email": "john@example.com",
		"status":     "active",
		"count":      float64(42),
	})

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "simple jsonPath field",
			template: "{{request.jsonPath.status}}",
			expected: "active",
		},
		{
			name:     "nested jsonPath field",
			template: "{{request.jsonPath.user_name}}",
			expected: "John",
		},
		{
			name:     "numeric jsonPath field",
			template: "Count: {{request.jsonPath.count}}",
			expected: "Count: 42",
		},
		{
			name:     "combined template",
			template: "Hello {{request.jsonPath.user_name}}, your status is {{request.jsonPath.status}}",
			expected: "Hello John, your status is active",
		},
		{
			name:     "missing jsonPath field returns empty",
			template: "{{request.jsonPath.missing}}",
			expected: "",
		},
		{
			name:     "jsonPath with other template vars",
			template: "{{request.method}} request from {{request.jsonPath.user_name}}",
			expected: "POST request from John",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Process() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestJSONPathTemplateVariables_NoMatches(t *testing.T) {
	engine := New()

	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)
	// Don't set any JSONPath matches

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "returns empty when no matches set",
			template: "{{request.jsonPath.field}}",
			expected: "",
		},
		{
			name:     "template with unmatched jsonPath continues",
			template: "User: {{request.jsonPath.name}}, Path: {{request.path}}",
			expected: "User: , Path: /test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Process() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSetJSONPathMatches(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	// Test setting matches
	matches := map[string]interface{}{
		"field1": "value1",
		"field2": float64(123),
		"field3": true,
	}
	ctx.SetJSONPathMatches(matches)

	if ctx.Request.JSONPath["field1"] != "value1" {
		t.Errorf("JSONPath[field1] = %v, want value1", ctx.Request.JSONPath["field1"])
	}
	if ctx.Request.JSONPath["field2"] != float64(123) {
		t.Errorf("JSONPath[field2] = %v, want 123", ctx.Request.JSONPath["field2"])
	}
	if ctx.Request.JSONPath["field3"] != true {
		t.Errorf("JSONPath[field3] = %v, want true", ctx.Request.JSONPath["field3"])
	}

	// Test nil matches doesn't panic
	ctx.SetJSONPathMatches(nil)
}

func TestSetJSONPathMatches_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	// Verify initially empty
	if len(ctx.Request.JSONPath) != 0 {
		t.Errorf("Initial JSONPath should be empty, got %v", ctx.Request.JSONPath)
	}

	// Set empty map
	ctx.SetJSONPathMatches(map[string]interface{}{})
	if len(ctx.Request.JSONPath) != 0 {
		t.Errorf("JSONPath should still be empty after setting empty map, got %v", ctx.Request.JSONPath)
	}
}
