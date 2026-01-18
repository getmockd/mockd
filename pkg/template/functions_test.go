package template

import (
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestFuncUUIDShort(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	result, err := engine.Process("{{uuid.short}}", ctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(result) != 8 {
		t.Errorf("uuid.short should return 8 characters, got %d: %q", len(result), result)
	}

	// Should be hex characters only
	matched, _ := regexp.MatchString("^[0-9a-f]{8}$", result)
	if !matched {
		t.Errorf("uuid.short should be hex characters, got %q", result)
	}
}

func TestFuncRandomInt(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	tests := []struct {
		name     string
		template string
		min      int
		max      int
		wantErr  bool
	}{
		{
			name:     "basic range",
			template: "{{random.int 1 100}}",
			min:      1,
			max:      100,
		},
		{
			name:     "single value range",
			template: "{{random.int 5 5}}",
			min:      5,
			max:      5,
		},
		{
			name:     "negative range",
			template: "{{random.int -10 10}}",
			min:      -10,
			max:      10,
		},
		{
			name:     "missing args returns empty",
			template: "{{random.int 1}}",
			wantErr:  true,
		},
		{
			name:     "no args returns empty",
			template: "{{random.int}}",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			if tt.wantErr {
				if result != "" {
					t.Errorf("expected empty result for invalid args, got %q", result)
				}
				return
			}

			n, err := strconv.Atoi(result)
			if err != nil {
				t.Fatalf("result should be a valid integer, got %q", result)
			}

			if n < tt.min || n > tt.max {
				t.Errorf("result %d should be between %d and %d", n, tt.min, tt.max)
			}
		})
	}
}

func TestFuncRandomFloat(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	result, err := engine.Process("{{random.float}}", ctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	f, err := strconv.ParseFloat(result, 64)
	if err != nil {
		t.Fatalf("result should be a valid float, got %q: %v", result, err)
	}

	if f < 0 || f >= 1 {
		t.Errorf("random.float should return value between 0 and 1, got %f", f)
	}
}

func TestFuncUpper(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test?name=hello", nil)
	ctx := NewContext(r, nil)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "upper with context value",
			template: "{{upper request.query.name}}",
			expected: "HELLO",
		},
		{
			name:     "upper with literal",
			template: "{{upper world}}",
			expected: "WORLD",
		},
		{
			name:     "upper with quoted literal",
			template: `{{upper "test"}}`,
			expected: "TEST",
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

func TestFuncLower(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test?name=HELLO", nil)
	ctx := NewContext(r, nil)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "lower with context value",
			template: "{{lower request.query.name}}",
			expected: "hello",
		},
		{
			name:     "lower with literal",
			template: "{{lower WORLD}}",
			expected: "world",
		},
		{
			name:     "lower with quoted literal",
			template: `{{lower "TEST"}}`,
			expected: "test",
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

func TestFuncDefault(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test?exists=value", nil)
	ctx := NewContext(r, nil)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "default with existing value",
			template: `{{default request.query.exists "fallback"}}`,
			expected: "value",
		},
		{
			name:     "default with missing value",
			template: `{{default request.query.missing "fallback"}}`,
			expected: "fallback",
		},
		{
			name:     "default with unquoted fallback",
			template: "{{default request.query.missing fallback}}",
			expected: "fallback",
		},
		{
			name:     "default with empty existing value uses fallback",
			template: `{{default request.query.empty "fallback"}}`,
			expected: "fallback",
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

func TestCombinedNewFunctions(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test?name=world", nil)
	ctx := NewContext(r, nil)

	// Test combining multiple new functions in one template
	template := `{"id": "{{uuid.short}}", "greeting": "{{upper request.query.name}}", "random": {{random.int 1 10}}}`
	result, err := engine.Process(template, ctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify it contains expected patterns
	if !strings.Contains(result, `"greeting": "WORLD"`) {
		t.Errorf("result should contain uppercase WORLD, got %s", result)
	}
	if !strings.Contains(result, `"id": "`) {
		t.Errorf("result should contain id field, got %s", result)
	}
	if !strings.Contains(result, `"random": `) {
		t.Errorf("result should contain random field, got %s", result)
	}
}

func TestFunctionsWithNilContext(t *testing.T) {
	engine := New()

	// Functions that don't need context should still work
	tests := []struct {
		name     string
		template string
		checkFn  func(result string) bool
	}{
		{
			name:     "uuid.short works without context",
			template: "{{uuid.short}}",
			checkFn:  func(r string) bool { return len(r) == 8 },
		},
		{
			name:     "random.float works without context",
			template: "{{random.float}}",
			checkFn:  func(r string) bool { _, err := strconv.ParseFloat(r, 64); return err == nil },
		},
		{
			name:     "random.int works without context",
			template: "{{random.int 1 10}}",
			checkFn:  func(r string) bool { n, err := strconv.Atoi(r); return err == nil && n >= 1 && n <= 10 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if !tt.checkFn(result) {
				t.Errorf("check failed for result %q", result)
			}
		})
	}
}

func TestRandomIntInvalidRange(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	// min > max should return empty
	result, err := engine.Process("{{random.int 100 1}}", ctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result != "" {
		t.Errorf("random.int with min > max should return empty, got %q", result)
	}
}

func TestRandomIntNonNumericArgs(t *testing.T) {
	engine := New()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	tests := []struct {
		name     string
		template string
	}{
		{"non-numeric min", "{{random.int abc 10}}"},
		{"non-numeric max", "{{random.int 1 xyz}}"},
		{"both non-numeric", "{{random.int abc xyz}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != "" {
				t.Errorf("expected empty result for invalid args, got %q", result)
			}
		})
	}
}
