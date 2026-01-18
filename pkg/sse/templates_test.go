package sse

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getmockd/mockd/internal/id"
)

func TestTemplateRegistry_New(t *testing.T) {
	registry := NewTemplateRegistry()

	// Should have built-in templates
	templates := registry.List()
	if len(templates) < 2 {
		t.Errorf("expected at least 2 built-in templates, got %d", len(templates))
	}
}

func TestTemplateRegistry_Register(t *testing.T) {
	registry := NewTemplateRegistry()

	called := false
	registry.Register("test", func(params map[string]interface{}) []SSEEventDef {
		called = true
		return nil
	})

	gen, ok := registry.Get("test")
	if !ok {
		t.Fatal("expected to find registered template")
	}

	gen(nil)
	if !called {
		t.Error("expected generator to be called")
	}
}

func TestTemplateRegistry_Get_NotFound(t *testing.T) {
	registry := NewTemplateRegistry()

	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent template")
	}
}

func TestTemplateRegistry_List(t *testing.T) {
	registry := NewTemplateRegistry()

	templates := registry.List()

	hasOpenAI := false
	hasNotification := false
	for _, name := range templates {
		if name == TemplateOpenAIChat {
			hasOpenAI = true
		}
		if name == TemplateNotificationStream {
			hasNotification = true
		}
	}

	if !hasOpenAI {
		t.Error("expected openai-chat template")
	}
	if !hasNotification {
		t.Error("expected notification-stream template")
	}
}

func TestGenerateOpenAIChatEvents_Default(t *testing.T) {
	events := generateOpenAIChatEvents(map[string]interface{}{})

	// Should have default tokens + [DONE]
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}

	// Last should be [DONE]
	lastEvent := events[len(events)-1]
	if lastEvent.Data != "[DONE]" {
		t.Errorf("expected last event to be [DONE], got %v", lastEvent.Data)
	}
}

func TestGenerateOpenAIChatEvents_CustomTokens(t *testing.T) {
	params := map[string]interface{}{
		"tokens":       []string{"Hello", "World"},
		"model":        "test-model",
		"finishReason": "stop",
		"includeDone":  true,
	}

	events := generateOpenAIChatEvents(params)

	// 2 tokens + [DONE]
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Check first token event
	if events[0].Data == nil {
		t.Fatal("expected data in first event")
	}

	dataMap, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if dataMap["model"] != "test-model" {
		t.Errorf("expected model 'test-model', got %v", dataMap["model"])
	}
}

func TestGenerateOpenAIChatEvents_NoDone(t *testing.T) {
	params := map[string]interface{}{
		"tokens":      []string{"test"},
		"includeDone": false,
	}

	events := generateOpenAIChatEvents(params)

	if len(events) != 1 {
		t.Errorf("expected 1 event (no [DONE]), got %d", len(events))
	}
}

func TestGenerateOpenAIChatEvents_FinishReason(t *testing.T) {
	params := map[string]interface{}{
		"tokens":       []string{"test"},
		"finishReason": "length",
		"includeDone":  false,
	}

	events := generateOpenAIChatEvents(params)

	dataMap := events[0].Data.(map[string]interface{})
	choices := dataMap["choices"].([]map[string]interface{})
	finishReason := choices[0]["finish_reason"]

	if finishReason != "length" {
		t.Errorf("expected finish_reason 'length', got %v", finishReason)
	}
}

func TestGenerateOpenAIChatEvents_DelayPerToken(t *testing.T) {
	params := map[string]interface{}{
		"tokens":        []string{"a", "b"},
		"delayPerToken": 100,
		"includeDone":   false,
	}

	events := generateOpenAIChatEvents(params)

	for i, event := range events {
		if event.Delay == nil || *event.Delay != 100 {
			t.Errorf("event %d: expected delay 100, got %v", i, event.Delay)
		}
	}
}

func TestGenerateNotificationEvents(t *testing.T) {
	params := map[string]interface{}{
		"messages": []map[string]interface{}{
			{"type": "alert", "payload": "test alert"},
			{"type": "update", "payload": map[string]interface{}{"status": "ok"}},
		},
		"interval": 500,
	}

	events := generateNotificationEvents(params)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != "alert" {
		t.Errorf("expected type 'alert', got %q", events[0].Type)
	}
	if events[0].Data != "test alert" {
		t.Errorf("expected data 'test alert', got %v", events[0].Data)
	}
	if events[0].Delay == nil || *events[0].Delay != 500 {
		t.Errorf("expected delay 500, got %v", events[0].Delay)
	}
}

func TestGenerateNotificationEvents_Empty(t *testing.T) {
	events := generateNotificationEvents(map[string]interface{}{})

	if events != nil {
		t.Errorf("expected nil for empty messages, got %v", events)
	}
}

func TestGetString(t *testing.T) {
	params := map[string]interface{}{
		"key": "value",
	}

	result := getString(params, "key", "default")
	if result != "value" {
		t.Errorf("expected 'value', got %q", result)
	}

	result = getString(params, "missing", "default")
	if result != "default" {
		t.Errorf("expected 'default', got %q", result)
	}
}

func TestGetInt(t *testing.T) {
	params := map[string]interface{}{
		"int":     42,
		"int64":   int64(100),
		"float64": float64(200.5),
	}

	if getInt(params, "int", 0) != 42 {
		t.Error("failed to get int")
	}
	if getInt(params, "int64", 0) != 100 {
		t.Error("failed to get int64")
	}
	if getInt(params, "float64", 0) != 200 {
		t.Error("failed to get float64 as int")
	}
	if getInt(params, "missing", 99) != 99 {
		t.Error("failed to get default")
	}
}

func TestGetBool(t *testing.T) {
	params := map[string]interface{}{
		"true":  true,
		"false": false,
	}

	if !getBool(params, "true", false) {
		t.Error("expected true")
	}
	if getBool(params, "false", true) {
		t.Error("expected false")
	}
	if !getBool(params, "missing", true) {
		t.Error("expected default true")
	}
}

func TestGetStringSlice(t *testing.T) {
	params := map[string]interface{}{
		"strings":    []string{"a", "b", "c"},
		"interfaces": []interface{}{"x", "y", "z"},
	}

	result := getStringSlice(params, "strings")
	if len(result) != 3 || result[0] != "a" {
		t.Errorf("expected [a,b,c], got %v", result)
	}

	result = getStringSlice(params, "interfaces")
	if len(result) != 3 || result[0] != "x" {
		t.Errorf("expected [x,y,z], got %v", result)
	}

	result = getStringSlice(params, "missing")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestGetMapSlice(t *testing.T) {
	params := map[string]interface{}{
		"maps": []map[string]interface{}{
			{"key": "value1"},
			{"key": "value2"},
		},
		"interfaces": []interface{}{
			map[string]interface{}{"key": "value3"},
		},
	}

	result := getMapSlice(params, "maps")
	if len(result) != 2 {
		t.Errorf("expected 2 maps, got %d", len(result))
	}

	result = getMapSlice(params, "interfaces")
	if len(result) != 1 {
		t.Errorf("expected 1 map, got %d", len(result))
	}
}

func TestGenerateID(t *testing.T) {
	id1 := id.Alphanumeric(10)
	id2 := id.Alphanumeric(10)

	if len(id1) != 10 {
		t.Errorf("expected length 10, got %d", len(id1))
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
}

func TestProcessRandomPlaceholder_UUID(t *testing.T) {
	result := processRandomPlaceholder("$uuid")

	str, ok := result.(string)
	if !ok {
		t.Fatal("expected string result")
	}

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(str) != 36 {
		t.Errorf("expected UUID length 36, got %d", len(str))
	}
	if str[8] != '-' || str[13] != '-' || str[18] != '-' || str[23] != '-' {
		t.Errorf("invalid UUID format: %s", str)
	}
}

func TestProcessRandomPlaceholder_Timestamp(t *testing.T) {
	result := processRandomPlaceholder("$timestamp")

	_, ok := result.(int64)
	if !ok {
		t.Fatal("expected int64 result")
	}
}

func TestProcessRandomPlaceholder_Random(t *testing.T) {
	for i := 0; i < 100; i++ {
		result := processRandomPlaceholder("$random(10,20)")

		n, ok := result.(int)
		if !ok {
			t.Fatal("expected int result")
		}
		if n < 10 || n > 20 {
			t.Errorf("random value %d outside range [10,20]", n)
		}
	}
}

func TestProcessRandomPlaceholder_Pick(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		result := processRandomPlaceholder("$pick(red,green,blue)")

		str, ok := result.(string)
		if !ok {
			t.Fatal("expected string result")
		}
		if str != "red" && str != "green" && str != "blue" {
			t.Errorf("unexpected pick result: %s", str)
		}
		seen[str] = true
	}

	// Should have seen all options eventually
	if len(seen) < 2 {
		t.Error("expected variation in pick results")
	}
}

func TestProcessRandomPlaceholder_NotPlaceholder(t *testing.T) {
	result := processRandomPlaceholder("regular string")

	str, ok := result.(string)
	if !ok {
		t.Fatal("expected string result")
	}
	if str != "regular string" {
		t.Errorf("expected unchanged string, got %q", str)
	}
}

func TestParseIntOr(t *testing.T) {
	tests := []struct {
		input    string
		default_ int
		expected int
	}{
		{"42", 0, 42},
		{"0", 99, 0},
		{"-5", 0, -5},
		{"invalid", 99, 99},
		{"", 99, 0}, // Empty string parses as 0 with the implementation
		{"12abc", 99, 99},
	}

	for _, tc := range tests {
		result := parseIntOr(tc.input, tc.default_)
		if result != tc.expected {
			t.Errorf("parseIntOr(%q, %d) = %d, expected %d", tc.input, tc.default_, result, tc.expected)
		}
	}
}

func TestRandomInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		result := randomInt(10, 20)
		if result < 10 || result > 20 {
			t.Errorf("randomInt(10,20) = %d, outside range", result)
		}
	}

	// Edge case: min == max
	result := randomInt(5, 5)
	if result != 5 {
		t.Errorf("randomInt(5,5) = %d, expected 5", result)
	}
}

func TestRandomPick(t *testing.T) {
	options := []string{"a", "b", "c"}
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		result := randomPick(options)
		if result != "a" && result != "b" && result != "c" {
			t.Errorf("unexpected pick: %s", result)
		}
		seen[result] = true
	}

	if len(seen) < 2 {
		t.Error("expected variation in picks")
	}

	// Empty slice
	result := randomPick([]string{})
	if result != "" {
		t.Errorf("expected empty string for empty slice, got %q", result)
	}
}

func TestGenerateUUID(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		uuid := generateUUID()

		// Check format
		if len(uuid) != 36 {
			t.Errorf("invalid UUID length: %d", len(uuid))
		}

		parts := strings.Split(uuid, "-")
		if len(parts) != 5 {
			t.Errorf("invalid UUID parts: %d", len(parts))
		}

		// Check version 4 indicator
		if uuid[14] != '4' {
			t.Errorf("expected version 4, got %c", uuid[14])
		}

		// Check uniqueness
		if seen[uuid] {
			t.Error("duplicate UUID generated")
		}
		seen[uuid] = true
	}
}

func TestFormatOpenAIChatChunk(t *testing.T) {
	chunk := FormatOpenAIChatChunk("chat-123", "gpt-4", "Hello", 1234567890, nil)

	var response OpenAIChatResponse
	err := json.Unmarshal([]byte(chunk), &response)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if response.ID != "chat-123" {
		t.Errorf("expected ID 'chat-123', got %q", response.ID)
	}
	if response.Object != "chat.completion.chunk" {
		t.Errorf("expected object 'chat.completion.chunk', got %q", response.Object)
	}
	if response.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %q", response.Model)
	}
	if response.Created != 1234567890 {
		t.Errorf("expected created 1234567890, got %d", response.Created)
	}
	if len(response.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Delta.Content != "Hello" {
		t.Errorf("expected content 'Hello', got %q", response.Choices[0].Delta.Content)
	}
}

func TestFormatOpenAIChatChunk_WithFinishReason(t *testing.T) {
	chunk := FormatOpenAIChatChunk("chat-123", "gpt-4", "", 1234567890, "stop")

	var response OpenAIChatResponse
	json.Unmarshal([]byte(chunk), &response)

	if response.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %v", response.Choices[0].FinishReason)
	}
}
