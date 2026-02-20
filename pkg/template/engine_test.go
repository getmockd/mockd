package template

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Parenthesized Syntax Tests
// =============================================================================

func TestRandomIntParenthesized(t *testing.T) {
	engine := New()

	tests := []struct {
		name     string
		template string
		min      int
		max      int
	}{
		{"basic range", "{{random.int(1, 100)}}", 1, 100},
		{"tight range", "{{random.int(5, 5)}}", 5, 5},
		{"zero range", "{{random.int(0, 0)}}", 0, 0},
		{"large range", "{{random.int(0, 1000000)}}", 0, 1000000},
		{"with spaces", "{{ random.int(1, 50) }}", 1, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			n, err := strconv.Atoi(result)
			if err != nil {
				t.Fatalf("result should be integer, got %q: %v", result, err)
			}
			if n < tt.min || n > tt.max {
				t.Errorf("result %d not in range [%d, %d]", n, tt.min, tt.max)
			}
		})
	}
}

func TestRandomFloatParenthesized(t *testing.T) {
	engine := New()

	t.Run("basic range", func(t *testing.T) {
		result, err := engine.Process("{{random.float(1.0, 10.0)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		f, err := strconv.ParseFloat(result, 64)
		if err != nil {
			t.Fatalf("result should be float, got %q: %v", result, err)
		}
		if f < 1.0 || f > 10.0 {
			t.Errorf("result %f not in range [1.0, 10.0]", f)
		}
	})

	t.Run("with precision", func(t *testing.T) {
		result, err := engine.Process("{{random.float(0.0, 100.0, 2)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		f, err := strconv.ParseFloat(result, 64)
		if err != nil {
			t.Fatalf("result should be float, got %q: %v", result, err)
		}
		if f < 0.0 || f > 100.0 {
			t.Errorf("result %f not in range [0.0, 100.0]", f)
		}
		// Check precision: should have at most 2 decimal places
		parts := strings.Split(result, ".")
		if len(parts) == 2 && len(parts[1]) > 2 {
			t.Errorf("expected at most 2 decimal places, got %q", result)
		}
	})

	t.Run("no args returns 0-1", func(t *testing.T) {
		result, err := engine.Process("{{random.float}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		f, err := strconv.ParseFloat(result, 64)
		if err != nil {
			t.Fatalf("result should be float, got %q: %v", result, err)
		}
		if f < 0.0 || f >= 1.0 {
			t.Errorf("result %f not in range [0.0, 1.0)", f)
		}
	})
}

func TestParenthesizedUpperLower(t *testing.T) {
	engine := New()

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"upper parenthesized", `{{upper("hello")}}`, "HELLO"},
		{"lower parenthesized", `{{lower("WORLD")}}`, "world"},
		{"upper unquoted", `{{upper(hello)}}`, "HELLO"},
		{"lower unquoted", `{{lower(WORLD)}}`, "world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Process() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParenthesizedDefault(t *testing.T) {
	engine := New()

	t.Run("default with empty value", func(t *testing.T) {
		// Without context, request.query.missing resolves to ""
		result, err := engine.Process(`{{default(request.query.missing, "fallback")}}`, nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "fallback" {
			t.Errorf("Process() = %q, want %q", result, "fallback")
		}
	})
}

// =============================================================================
// Random String Tests
// =============================================================================

func TestRandomStringParenthesized(t *testing.T) {
	engine := New()

	t.Run("default length 10", func(t *testing.T) {
		result, err := engine.Process("{{random.string}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 10 {
			t.Errorf("random.string should return 10 chars, got %d: %q", len(result), result)
		}
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, result)
		if !matched {
			t.Errorf("random.string should be alphanumeric, got %q", result)
		}
	})

	t.Run("custom length", func(t *testing.T) {
		result, err := engine.Process("{{random.string(20)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 20 {
			t.Errorf("random.string(20) should return 20 chars, got %d: %q", len(result), result)
		}
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, result)
		if !matched {
			t.Errorf("random.string(20) should be alphanumeric, got %q", result)
		}
	})

	t.Run("length 1", func(t *testing.T) {
		result, err := engine.Process("{{random.string(1)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 1 {
			t.Errorf("random.string(1) should return 1 char, got %d: %q", len(result), result)
		}
	})

	t.Run("large length", func(t *testing.T) {
		result, err := engine.Process("{{random.string(100)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 100 {
			t.Errorf("random.string(100) should return 100 chars, got %d", len(result))
		}
	})

	t.Run("with spaces", func(t *testing.T) {
		result, err := engine.Process("{{ random.string(5) }}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 5 {
			t.Errorf("random.string(5) with spaces should return 5 chars, got %d: %q", len(result), result)
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{random.string(10)}}", nil)
			results[result] = true
		}
		// With 62^10 possibilities, 20 calls should produce at least 2 unique values
		if len(results) < 2 {
			t.Error("random.string should produce different values across calls")
		}
	})
}

// =============================================================================
// Default Function Extended Tests
// =============================================================================

func TestDefaultWithPayloadFallback(t *testing.T) {
	engine := New()

	t.Run("default with missing payload uses fallback", func(t *testing.T) {
		ctx := NewMQTTContext("test/topic", "client-1", nil, nil)
		result, err := engine.Process(`{{default(payload.missing, "N/A")}}`, ctx)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "N/A" {
			t.Errorf("Process() = %q, want %q", result, "N/A")
		}
	})

	t.Run("default with present payload uses value", func(t *testing.T) {
		payload := map[string]any{"temp": "72.5"}
		ctx := NewMQTTContext("test/topic", "client-1", payload, nil)
		result, err := engine.Process(`{{default(payload.temp, "N/A")}}`, ctx)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "72.5" {
			t.Errorf("Process() = %q, want %q", result, "72.5")
		}
	})

	t.Run("default with missing header uses fallback parenthesized", func(t *testing.T) {
		result, err := engine.Process(`{{default(request.header.X-Custom, "fallback-value")}}`, nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "fallback-value" {
			t.Errorf("Process() = %q, want %q", result, "fallback-value")
		}
	})

	t.Run("default resolves topic as value", func(t *testing.T) {
		ctx := NewMQTTContext("sensors/temp", "client-1", nil, nil)
		result, err := engine.Process(`{{default(topic, "no-topic")}}`, ctx)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "sensors/temp" {
			t.Errorf("Process() = %q, want %q", result, "sensors/temp")
		}
	})
}

// =============================================================================
// Sequence with Default Engine Tests
// =============================================================================

func TestSequenceWithDefaultEngine(t *testing.T) {
	t.Run("sequence works in HTTP context", func(t *testing.T) {
		engine := New()
		result1, _ := engine.Process(`{{sequence("http_counter")}}`, nil)
		result2, _ := engine.Process(`{{sequence("http_counter")}}`, nil)
		result3, _ := engine.Process(`{{sequence("http_counter")}}`, nil)
		if result1 != "1" || result2 != "2" || result3 != "3" {
			t.Errorf("sequence should auto-increment: got %q, %q, %q", result1, result2, result3)
		}
	})

	t.Run("sequence with custom start in HTTP context", func(t *testing.T) {
		engine := New()
		result1, _ := engine.Process(`{{sequence("counter", 100)}}`, nil)
		result2, _ := engine.Process(`{{sequence("counter", 100)}}`, nil)
		if result1 != "100" || result2 != "101" {
			t.Errorf("sequence with start=100 should give 100, 101: got %q, %q", result1, result2)
		}
	})
}

// =============================================================================
// Sequence Tests
// =============================================================================

func TestSequenceBasic(t *testing.T) {
	store := NewSequenceStore()
	engine := NewWithSequences(store)

	t.Run("auto-increment from 1", func(t *testing.T) {
		for i := int64(1); i <= 5; i++ {
			result, err := engine.Process(`{{sequence("counter")}}`, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			expected := strconv.FormatInt(i, 10)
			if result != expected {
				t.Errorf("iteration %d: got %q, want %q", i, result, expected)
			}
		}
	})

	t.Run("custom start value", func(t *testing.T) {
		result, err := engine.Process(`{{sequence("custom", 100)}}`, nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "100" {
			t.Errorf("first value should be 100, got %q", result)
		}

		result, err = engine.Process(`{{sequence("custom", 100)}}`, nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "101" {
			t.Errorf("second value should be 101, got %q", result)
		}
	})

	t.Run("independent sequences", func(t *testing.T) {
		store2 := NewSequenceStore()
		eng2 := NewWithSequences(store2)

		eng2.Process(`{{sequence("a")}}`, nil) // a=1
		eng2.Process(`{{sequence("a")}}`, nil) // a=2
		eng2.Process(`{{sequence("b")}}`, nil) // b=1

		resultA, _ := eng2.Process(`{{sequence("a")}}`, nil)
		resultB, _ := eng2.Process(`{{sequence("b")}}`, nil)

		if resultA != "3" {
			t.Errorf("sequence 'a' should be 3, got %q", resultA)
		}
		if resultB != "2" {
			t.Errorf("sequence 'b' should be 2, got %q", resultB)
		}
	})

	t.Run("default engine has sequences", func(t *testing.T) {
		eng := New()
		result, err := eng.Process(`{{sequence("test")}}`, nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "1" {
			t.Errorf("sequence with default engine should start at 1, got %q", result)
		}
		result, err = eng.Process(`{{sequence("test")}}`, nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "2" {
			t.Errorf("second call should return 2, got %q", result)
		}
	})
}

// =============================================================================
// SequenceStore Tests
// =============================================================================

func TestSequenceStoreNext(t *testing.T) {
	store := NewSequenceStore()

	// First call starts at the given start value
	if v := store.Next("test", 1); v != 1 {
		t.Errorf("first Next = %d, want 1", v)
	}
	if v := store.Next("test", 1); v != 2 {
		t.Errorf("second Next = %d, want 2", v)
	}
	if v := store.Next("test", 1); v != 3 {
		t.Errorf("third Next = %d, want 3", v)
	}
}

func TestSequenceStoreReset(t *testing.T) {
	store := NewSequenceStore()

	store.Next("reset-test", 10) // 10
	store.Next("reset-test", 10) // 11
	store.Reset("reset-test")

	// After reset, should start from the given start value again
	if v := store.Next("reset-test", 10); v != 10 {
		t.Errorf("after reset Next = %d, want 10", v)
	}
}

func TestSequenceStoreCurrent(t *testing.T) {
	store := NewSequenceStore()

	// Non-existent sequence returns 0
	if v := store.Current("missing"); v != 0 {
		t.Errorf("Current of missing = %d, want 0", v)
	}

	store.Next("exist", 5)
	// After Next(5), current should be 6 (value was 5, then incremented)
	if v := store.Current("exist"); v != 6 {
		t.Errorf("Current after Next(5) = %d, want 6", v)
	}
}

func TestSequenceStoreConcurrency(t *testing.T) {
	store := NewSequenceStore()
	var wg sync.WaitGroup
	goroutines := 100
	iterations := 100

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				store.Next("concurrent", 1)
			}
		}()
	}
	wg.Wait()

	// After 100 goroutines × 100 iterations, current should be 10001 (started at 1, incremented 10000 times)
	expected := int64(goroutines*iterations) + 1
	if v := store.Current("concurrent"); v != expected {
		t.Errorf("concurrent Current = %d, want %d", v, expected)
	}
}

// =============================================================================
// Faker Tests
// =============================================================================

func TestFakerVariables(t *testing.T) {
	engine := New()

	fakerTypes := []struct {
		name    string
		pattern string // regex to validate output format
	}{
		{"uuid", `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`},
		{"boolean", `^(true|false)$`},
		{"name", `.+`},
		{"firstName", `.+`},
		{"lastName", `.+`},
		{"email", `.+@.+\..+`},
		{"address", `.+`},
		{"phone", `^\+1-\d{3}-\d{3}-\d{4}$`},
		{"company", `.+`},
		{"word", `^\w+$`},
		{"sentence", `.+\.$`},
	}

	for _, ft := range fakerTypes {
		t.Run("faker."+ft.name, func(t *testing.T) {
			result, err := engine.Process("{{faker."+ft.name+"}}", nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result == "" {
				t.Error("faker should produce non-empty output")
			}
			if matched, _ := regexp.MatchString(ft.pattern, result); !matched {
				t.Errorf("faker.%s = %q doesn't match pattern %q", ft.name, result, ft.pattern)
			}
		})
	}

	t.Run("faker.unknown returns empty", func(t *testing.T) {
		result, err := engine.Process("{{faker.nonexistent}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "" {
			t.Errorf("unknown faker type should return empty, got %q", result)
		}
	})
}

// =============================================================================
// MQTT Context Tests
// =============================================================================

func TestMQTTContextVariables(t *testing.T) {
	engine := New()
	ctx := NewMQTTContext("sensors/temperature", "device-001", nil, nil)
	ctx.MQTT.DeviceID = "dev-42"

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"topic", "{{topic}}", "sensors/temperature"},
		{"clientId", "{{clientId}}", "device-001"},
		{"device_id", "{{device_id}}", "dev-42"},
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

	t.Run("nil context returns empty", func(t *testing.T) {
		result, err := engine.Process("{{topic}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "" {
			t.Errorf("topic with nil context should be empty, got %q", result)
		}
	})
}

// =============================================================================
// Wildcard Substitution Tests
// =============================================================================

// Note: MQTT wildcard substitution like {1}, {2} is handled by
// mqtt.RenderTopicTemplate (simple string replacement) for topic names.
// The template engine's resolveWildcard code path requires the expression
// to be exactly "{1}" but the {{ }} regex prevents } inside expressions,
// so the triple-brace syntax {{{1}}} doesn't parse correctly.
// This test verifies the resolveWildcard function works directly.

func TestResolveWildcard(t *testing.T) {
	engine := New()
	ctx := NewMQTTContext("sensors/floor1/temp", "", nil, []string{"floor1", "temp"})

	tests := []struct {
		name     string
		matches  []string // simulates regex match groups
		expected string
	}{
		{"first wildcard", []string{"{1}", "1"}, "floor1"},
		{"second wildcard", []string{"{2}", "2"}, "temp"},
		{"out of range", []string{"{3}", "3"}, ""},
		{"zero index", []string{"{0}", "0"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.resolveWildcard(tt.matches, ctx)
			if result != tt.expected {
				t.Errorf("resolveWildcard() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("nil context returns empty", func(t *testing.T) {
		result := engine.resolveWildcard([]string{"{1}", "1"}, nil)
		if result != "" {
			t.Errorf("resolveWildcard with nil context should be empty, got %q", result)
		}
	})
}

// =============================================================================
// Payload Access Tests
// =============================================================================

func TestPayloadAccess(t *testing.T) {
	engine := New()
	payload := map[string]any{
		"temperature": 72.5,
		"unit":        "F",
		"location": map[string]any{
			"floor":  "1",
			"room":   "A",
			"sensor": map[string]any{"id": "s-001"},
		},
		"active": true,
		"count":  42,
	}
	ctx := NewMQTTContext("sensors/temp", "client-1", payload, nil)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"string field", "{{payload.unit}}", "F"},
		{"float field", "{{payload.temperature}}", "72.5"},
		{"bool field", "{{payload.active}}", "true"},
		{"int field", "{{payload.count}}", "42"},
		{"nested field", "{{payload.location.floor}}", "1"},
		{"deeply nested", "{{payload.location.sensor.id}}", "s-001"},
		{"missing field", "{{payload.missing}}", ""},
		{"missing nested", "{{payload.location.missing}}", ""},
		{"invalid path", "{{payload.temperature.nested}}", ""},
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

	t.Run("nil payload returns empty", func(t *testing.T) {
		ctx2 := NewMQTTContext("t", "c", nil, nil)
		result, err := engine.Process("{{payload.field}}", ctx2)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "" {
			t.Errorf("payload with nil data should be empty, got %q", result)
		}
	})

	t.Run("nil context returns empty", func(t *testing.T) {
		result, err := engine.Process("{{payload.field}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result != "" {
			t.Errorf("payload with nil context should be empty, got %q", result)
		}
	})
}

// =============================================================================
// Timestamp Variant Tests
// =============================================================================

func TestTimestampVariants(t *testing.T) {
	engine := New()

	t.Run("timestamp.iso", func(t *testing.T) {
		result, err := engine.Process("{{timestamp.iso}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		// Should be an RFC3339Nano string
		if !strings.Contains(result, "T") || !strings.Contains(result, "Z") {
			t.Errorf("timestamp.iso should be ISO format, got %q", result)
		}
	})

	t.Run("timestamp.unix", func(t *testing.T) {
		result, err := engine.Process("{{timestamp.unix}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		n, err := strconv.ParseInt(result, 10, 64)
		if err != nil {
			t.Fatalf("timestamp.unix should be integer, got %q: %v", result, err)
		}
		// Should be a reasonable Unix timestamp (after 2020)
		if n < 1577836800 {
			t.Errorf("timestamp.unix %d seems too low", n)
		}
	})

	t.Run("timestamp.unix_ms", func(t *testing.T) {
		result, err := engine.Process("{{timestamp.unix_ms}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		n, err := strconv.ParseInt(result, 10, 64)
		if err != nil {
			t.Fatalf("timestamp.unix_ms should be integer, got %q: %v", result, err)
		}
		// Milliseconds should be much larger than seconds
		if n < 1577836800000 {
			t.Errorf("timestamp.unix_ms %d seems too low", n)
		}
	})

	t.Run("timestamp equals timestamp.unix", func(t *testing.T) {
		// Both should produce Unix timestamps
		r1, _ := engine.Process("{{timestamp}}", nil)
		r2, _ := engine.Process("{{timestamp.unix}}", nil)

		n1, err1 := strconv.ParseInt(r1, 10, 64)
		n2, err2 := strconv.ParseInt(r2, 10, 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("both should be integers: %q, %q", r1, r2)
		}
		// Should be within 1 second of each other
		diff := n1 - n2
		if diff < -1 || diff > 1 {
			t.Errorf("timestamp and timestamp.unix differ by %d", diff)
		}
	})
}

// =============================================================================
// ProcessInterface Tests
// =============================================================================

func TestProcessInterface(t *testing.T) {
	engine := New()
	ctx := NewMQTTContext("sensors/temp", "device-1", nil, nil)

	t.Run("string values are processed", func(t *testing.T) {
		data := "Topic is {{topic}}"
		result := engine.ProcessInterface(data, ctx)
		if result != "Topic is sensors/temp" {
			t.Errorf("ProcessInterface() = %q, want %q", result, "Topic is sensors/temp")
		}
	})

	t.Run("map values are recursively processed", func(t *testing.T) {
		data := map[string]interface{}{
			"topic":  "{{topic}}",
			"client": "{{clientId}}",
			"num":    42,
		}
		result := engine.ProcessInterface(data, ctx).(map[string]interface{})
		if result["topic"] != "sensors/temp" {
			t.Errorf("topic = %q, want %q", result["topic"], "sensors/temp")
		}
		if result["client"] != "device-1" {
			t.Errorf("client = %q, want %q", result["client"], "device-1")
		}
		if result["num"] != 42 {
			t.Errorf("num = %v, want 42", result["num"])
		}
	})

	t.Run("slice values are recursively processed", func(t *testing.T) {
		data := []interface{}{"{{topic}}", "literal", 123}
		result := engine.ProcessInterface(data, ctx).([]interface{})
		if result[0] != "sensors/temp" {
			t.Errorf("[0] = %q, want %q", result[0], "sensors/temp")
		}
		if result[1] != "literal" {
			t.Errorf("[1] = %q, want %q", result[1], "literal")
		}
		if result[2] != 123 {
			t.Errorf("[2] = %v, want 123", result[2])
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		result := engine.ProcessInterface(nil, ctx)
		if result != nil {
			t.Errorf("nil should return nil, got %v", result)
		}
	})

	t.Run("non-string non-collection returned unchanged", func(t *testing.T) {
		result := engine.ProcessInterface(42, ctx)
		if result != 42 {
			t.Errorf("int should be unchanged, got %v", result)
		}
	})
}

// =============================================================================
// formatValue Tests
// =============================================================================

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"float64", 3.14, "3.14"},
		{"float64 whole", 42.0, "42"},
		{"int", 42, "42"},
		{"int64", int64(123), "123"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"other", []int{1, 2}, "[1 2]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// splitFuncArgs Tests
// =============================================================================

func TestSplitFuncArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple", "a, b, c", []string{"a", "b", "c"}},
		{"quoted commas", `"hello, world", b`, []string{`"hello, world"`, "b"}},
		{"single arg", "value", []string{"value"}},
		{"quoted strings", `"first", "second"`, []string{`"first"`, `"second"`}},
		{"mixed", `request.query.x, "fallback"`, []string{"request.query.x", `"fallback"`}},
		{"single quotes", `'hello, world', b`, []string{`'hello, world'`, "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitFuncArgs(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("splitFuncArgs(%q) = %v (len %d), want %v (len %d)",
					tt.input, result, len(result), tt.expected, len(tt.expected))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitFuncArgs(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// =============================================================================
// Combined / Integration Tests
// =============================================================================

func TestMQTTTemplateIntegration(t *testing.T) {
	store := NewSequenceStore()
	engine := NewWithSequences(store)

	payload := map[string]any{
		"temp": 72.5,
		"unit": "F",
	}
	ctx := NewMQTTContext("sensors/floor1/temp", "device-001", payload, []string{"floor1", "temp"})
	ctx.MQTT.DeviceID = "dev-42"

	tmpl := `{"id": {{sequence("msg_id")}}, "topic": "{{topic}}", "device": "{{device_id}}", "temp": {{payload.temp}}, "name": "{{faker.firstName}}"}`

	result, err := engine.Process(tmpl, ctx)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify specific substitutions
	if !strings.Contains(result, `"id": 1`) {
		t.Errorf("should contain sequence id 1, got %s", result)
	}
	if !strings.Contains(result, `"topic": "sensors/floor1/temp"`) {
		t.Errorf("should contain topic, got %s", result)
	}
	if !strings.Contains(result, `"device": "dev-42"`) {
		t.Errorf("should contain device_id, got %s", result)
	}
	if !strings.Contains(result, `"temp": 72.5`) {
		t.Errorf("should contain payload.temp, got %s", result)
	}

	// Process again — sequence should increment
	result2, _ := engine.Process(`{{sequence("msg_id")}}`, ctx)
	if result2 != "2" {
		t.Errorf("second sequence call should return 2, got %q", result2)
	}
}

func TestBuiltinVariables(t *testing.T) {
	engine := New()

	builtins := []struct {
		name     string
		template string
		checkFn  func(string) bool
	}{
		{"now", "{{now}}", func(s string) bool { return strings.Contains(s, "T") }},
		{"uuid", "{{uuid}}", func(s string) bool { return len(s) == 36 && strings.Count(s, "-") == 4 }},
		{"uuid.short", "{{uuid.short}}", func(s string) bool { return len(s) == 8 }},
		{"timestamp", "{{timestamp}}", func(s string) bool { _, err := strconv.ParseInt(s, 10, 64); return err == nil }},
		{"random", "{{random}}", func(s string) bool { return len(s) == 8 }}, // hex encoded 4 bytes
	}

	for _, b := range builtins {
		t.Run(b.name, func(t *testing.T) {
			result, err := engine.Process(b.template, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if !b.checkFn(result) {
				t.Errorf("check failed for %q, got %q", b.name, result)
			}
		})
	}
}

func TestUnknownExpression(t *testing.T) {
	engine := New()

	result, err := engine.Process("{{unknown.expression}}", nil)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result != "" {
		t.Errorf("unknown expression should return empty, got %q", result)
	}
}

func TestMixedTemplate(t *testing.T) {
	engine := New()

	// Template with literal text + multiple expressions
	result, err := engine.Process("Hello {{uuid.short}}, today is {{now}}", nil)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !strings.HasPrefix(result, "Hello ") {
		t.Errorf("result should start with 'Hello ', got %q", result)
	}
	if !strings.Contains(result, ", today is ") {
		t.Errorf("result should contain ', today is ', got %q", result)
	}
}

func TestEmptyTemplate(t *testing.T) {
	engine := New()

	result, err := engine.Process("", nil)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result != "" {
		t.Errorf("empty template should return empty, got %q", result)
	}
}

func TestTemplateWithNoExpressions(t *testing.T) {
	engine := New()

	result, err := engine.Process("plain text with no expressions", nil)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result != "plain text with no expressions" {
		t.Errorf("plain text should be unchanged, got %q", result)
	}
}

func TestWhitespaceInExpressions(t *testing.T) {
	engine := New()

	// Extra whitespace inside braces should be handled
	tests := []struct {
		name     string
		template string
	}{
		{"leading space", "{{ uuid.short }}"},
		{"extra spaces", "{{  uuid.short  }}"},
		{"tab", "{{\tuuid.short\t}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if len(result) != 8 {
				t.Errorf("should still resolve uuid.short, got %q (len=%d)", result, len(result))
			}
		})
	}
}

func TestParseStringArg(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`"  spaced  "`, "  spaced  "},
		{`""`, ""},
		{`''`, ""},
		{`a`, "a"},
		{`  "trimmed"  `, "trimmed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseStringArg(tt.input)
			if result != tt.expected {
				t.Errorf("parseStringArg(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
