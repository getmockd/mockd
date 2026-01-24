package matching

import (
	"testing"
)

func TestMatchJSONPath_SimpleFieldMatching(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
		wantMatch  bool
	}{
		{
			name:       "simple string field match",
			conditions: map[string]interface{}{"$.status": "active"},
			body:       `{"status": "active", "name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "simple string field mismatch",
			conditions: map[string]interface{}{"$.status": "active"},
			body:       `{"status": "inactive", "name": "test"}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "number field match",
			conditions: map[string]interface{}{"$.count": float64(42)},
			body:       `{"count": 42, "name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "number field mismatch",
			conditions: map[string]interface{}{"$.count": float64(42)},
			body:       `{"count": 43, "name": "test"}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "boolean field match - true",
			conditions: map[string]interface{}{"$.enabled": true},
			body:       `{"enabled": true, "name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "boolean field match - false",
			conditions: map[string]interface{}{"$.enabled": false},
			body:       `{"enabled": false, "name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "null field match",
			conditions: map[string]interface{}{"$.deleted": nil},
			body:       `{"deleted": null, "name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "multiple conditions - all match",
			conditions: map[string]interface{}{"$.status": "active", "$.count": float64(10)},
			body:       `{"status": "active", "count": 10}`,
			wantScore:  30,
			wantMatch:  true,
		},
		{
			name:       "multiple conditions - one fails",
			conditions: map[string]interface{}{"$.status": "active", "$.count": float64(10)},
			body:       `{"status": "active", "count": 20}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "field does not exist",
			conditions: map[string]interface{}{"$.missing": "value"},
			body:       `{"status": "active"}`,
			wantScore:  0,
			wantMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
			gotMatch := result.Score > 0
			if gotMatch != tt.wantMatch {
				t.Errorf("MatchJSONPath() match = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestMatchJSONPath_NestedPaths(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
		wantMatch  bool
	}{
		{
			name:       "nested path - two levels",
			conditions: map[string]interface{}{"$.user.name": "John"},
			body:       `{"user": {"name": "John", "age": 30}}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "nested path - three levels",
			conditions: map[string]interface{}{"$.user.address.city": "NYC"},
			body:       `{"user": {"name": "John", "address": {"city": "NYC", "zip": "10001"}}}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "nested path mismatch",
			conditions: map[string]interface{}{"$.user.address.city": "LA"},
			body:       `{"user": {"name": "John", "address": {"city": "NYC", "zip": "10001"}}}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "nested path - intermediate missing",
			conditions: map[string]interface{}{"$.user.address.city": "NYC"},
			body:       `{"user": {"name": "John"}}`,
			wantScore:  0,
			wantMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
			gotMatch := result.Score > 0
			if gotMatch != tt.wantMatch {
				t.Errorf("MatchJSONPath() match = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestMatchJSONPath_ArrayIndexing(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
		wantMatch  bool
	}{
		{
			name:       "array index - first element",
			conditions: map[string]interface{}{"$.items[0].name": "first"},
			body:       `{"items": [{"name": "first"}, {"name": "second"}]}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "array index - second element",
			conditions: map[string]interface{}{"$.items[1].name": "second"},
			body:       `{"items": [{"name": "first"}, {"name": "second"}]}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "array index - out of bounds",
			conditions: map[string]interface{}{"$.items[5].name": "missing"},
			body:       `{"items": [{"name": "first"}, {"name": "second"}]}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "root array index",
			conditions: map[string]interface{}{"$[0].id": float64(1)},
			body:       `[{"id": 1}, {"id": 2}]`,
			wantScore:  15,
			wantMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
			gotMatch := result.Score > 0
			if gotMatch != tt.wantMatch {
				t.Errorf("MatchJSONPath() match = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestMatchJSONPath_ArrayWildcards(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
		wantMatch  bool
	}{
		{
			name:       "wildcard - any element matches",
			conditions: map[string]interface{}{"$.items[*].type": "premium"},
			body:       `{"items": [{"type": "basic"}, {"type": "premium"}, {"type": "basic"}]}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "wildcard - no element matches",
			conditions: map[string]interface{}{"$.items[*].type": "enterprise"},
			body:       `{"items": [{"type": "basic"}, {"type": "premium"}]}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "wildcard - all elements same value",
			conditions: map[string]interface{}{"$.items[*].status": "active"},
			body:       `{"items": [{"status": "active"}, {"status": "active"}]}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "nested wildcard",
			conditions: map[string]interface{}{"$.orders[*].items[*].sku": "SKU-123"},
			body:       `{"orders": [{"items": [{"sku": "SKU-123"}, {"sku": "SKU-456"}]}]}`,
			wantScore:  15,
			wantMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
			gotMatch := result.Score > 0
			if gotMatch != tt.wantMatch {
				t.Errorf("MatchJSONPath() match = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestMatchJSONPath_ExistenceChecks(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
		wantMatch  bool
	}{
		{
			name:       "exists true - field present",
			conditions: map[string]interface{}{"$.token": map[string]interface{}{"exists": true}},
			body:       `{"token": "abc123", "name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "exists true - field missing",
			conditions: map[string]interface{}{"$.token": map[string]interface{}{"exists": true}},
			body:       `{"name": "test"}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "exists false - field missing",
			conditions: map[string]interface{}{"$.deleted": map[string]interface{}{"exists": false}},
			body:       `{"name": "test"}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "exists false - field present",
			conditions: map[string]interface{}{"$.deleted": map[string]interface{}{"exists": false}},
			body:       `{"deleted": true, "name": "test"}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "exists true - field null",
			conditions: map[string]interface{}{"$.value": map[string]interface{}{"exists": true}},
			body:       `{"value": null}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "nested exists check",
			conditions: map[string]interface{}{"$.user.email": map[string]interface{}{"exists": true}},
			body:       `{"user": {"name": "John", "email": "john@example.com"}}`,
			wantScore:  15,
			wantMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
			gotMatch := result.Score > 0
			if gotMatch != tt.wantMatch {
				t.Errorf("MatchJSONPath() match = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestMatchJSONPath_NonJSONBody(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
	}{
		{
			name:       "plain text body",
			conditions: map[string]interface{}{"$.field": "value"},
			body:       "This is plain text, not JSON",
			wantScore:  0,
		},
		{
			name:       "empty body",
			conditions: map[string]interface{}{"$.field": "value"},
			body:       "",
			wantScore:  0,
		},
		{
			name:       "malformed JSON",
			conditions: map[string]interface{}{"$.field": "value"},
			body:       `{"field": "value"`,
			wantScore:  0,
		},
		{
			name:       "XML body",
			conditions: map[string]interface{}{"$.field": "value"},
			body:       `<root><field>value</field></root>`,
			wantScore:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
		})
	}
}

func TestMatchJSONPath_InvalidJSONPath(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
	}{
		{
			name:       "invalid JSONPath syntax - unclosed bracket",
			conditions: map[string]interface{}{"$[invalid": "value"},
			body:       `{"field": "value"}`,
			wantScore:  0,
		},
		{
			name:       "invalid JSONPath syntax - bad filter",
			conditions: map[string]interface{}{"$[?(": "value"},
			body:       `{"field": "value"}`,
			wantScore:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
		})
	}
}

// Note: The ojg library treats paths like "field" (without $) as valid relative paths.
// This is technically valid JSONPath behavior, so we accept it.

func TestMatchJSONPath_MatchedValues(t *testing.T) {
	tests := []struct {
		name         string
		conditions   map[string]interface{}
		body         string
		wantKey      string
		wantValue    interface{}
		shouldHaveIt bool
	}{
		{
			name:         "captures simple string value",
			conditions:   map[string]interface{}{"$.status": "active"},
			body:         `{"status": "active"}`,
			wantKey:      "status",
			wantValue:    "active",
			shouldHaveIt: true,
		},
		{
			name:         "captures nested value",
			conditions:   map[string]interface{}{"$.user.name": "John"},
			body:         `{"user": {"name": "John"}}`,
			wantKey:      "user_name",
			wantValue:    "John",
			shouldHaveIt: true,
		},
		{
			name:         "captures array indexed value",
			conditions:   map[string]interface{}{"$.items[0].id": float64(123)},
			body:         `{"items": [{"id": 123}]}`,
			wantKey:      "items_0_id",
			wantValue:    float64(123),
			shouldHaveIt: true,
		},
		{
			name:         "no capture on mismatch",
			conditions:   map[string]interface{}{"$.status": "active"},
			body:         `{"status": "inactive"}`,
			wantKey:      "status",
			shouldHaveIt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if tt.shouldHaveIt {
				if result.Matched == nil {
					t.Errorf("MatchJSONPath() Matched is nil, expected key %q", tt.wantKey)
					return
				}
				val, ok := result.Matched[tt.wantKey]
				if !ok {
					t.Errorf("MatchJSONPath() Matched missing key %q, got keys: %v", tt.wantKey, result.Matched)
					return
				}
				if val != tt.wantValue {
					t.Errorf("MatchJSONPath() Matched[%q] = %v, want %v", tt.wantKey, val, tt.wantValue)
				}
			} else {
				if len(result.Matched) > 0 {
					t.Errorf("MatchJSONPath() Matched should be empty on mismatch, got: %v", result.Matched)
				}
			}
		})
	}
}

func TestMatchJSONPath_EmptyConditions(t *testing.T) {
	result := MatchJSONPath(nil, []byte(`{"field": "value"}`))
	if result.Score != 0 {
		t.Errorf("MatchJSONPath(nil) score = %d, want 0", result.Score)
	}

	result = MatchJSONPath(map[string]interface{}{}, []byte(`{"field": "value"}`))
	if result.Score != 0 {
		t.Errorf("MatchJSONPath({}) score = %d, want 0", result.Score)
	}
}

func TestSanitizeJSONPathKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"$.status", "status"},
		{"$.user.name", "user_name"},
		{"$.items[0].id", "items_0_id"},
		{"$.items[*].type", "items_type"},
		{"$.data.user.address.city", "data_user_address_city"},
		{"$", ""},
		{"$.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeJSONPathKey(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeJSONPathKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateJSONPathExpression(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"$.field", false},
		{"$.user.name", false},
		{"$.items[0]", false},
		{"$.items[*].id", false},
		{"$..name", false},
		{"$[invalid", true},
		// Note: Empty string is treated as a valid path (the root) by ojg
		{"", false},
		{"$", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := ValidateJSONPathExpression(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSONPathExpression(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestMatchJSONPath_TypeAwareComparison(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		body       string
		wantScore  int
		wantMatch  bool
	}{
		{
			name:       "int expected matches json number",
			conditions: map[string]interface{}{"$.count": 42},
			body:       `{"count": 42}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "float expected matches json number",
			conditions: map[string]interface{}{"$.price": 19.99},
			body:       `{"price": 19.99}`,
			wantScore:  15,
			wantMatch:  true,
		},
		{
			name:       "string number does not match number",
			conditions: map[string]interface{}{"$.count": "42"},
			body:       `{"count": 42}`,
			wantScore:  0,
			wantMatch:  false,
		},
		{
			name:       "number does not match string number",
			conditions: map[string]interface{}{"$.count": float64(42)},
			body:       `{"count": "42"}`,
			wantScore:  0,
			wantMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchJSONPath(tt.conditions, []byte(tt.body))
			if result.Score != tt.wantScore {
				t.Errorf("MatchJSONPath() score = %d, want %d", result.Score, tt.wantScore)
			}
			gotMatch := result.Score > 0
			if gotMatch != tt.wantMatch {
				t.Errorf("MatchJSONPath() match = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}
