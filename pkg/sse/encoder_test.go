package sse

import (
	"strings"
	"testing"
)

func TestEncoder_FormatEvent_SingleLineData(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Data: "Hello, World!",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:Hello, World!\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_MultiLineData(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Data: "Line 1\nLine 2\nLine 3",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:Line 1\ndata:Line 2\ndata:Line 3\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_WithEventType(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Type: "message",
		Data: "Hello",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "event:message\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_WithID(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Data: "Hello",
		ID:   "123",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "id:123\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_WithRetry(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Data:  "Hello",
		Retry: 3000,
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "retry:3000\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_WithComment(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Comment: "This is a comment",
		Data:    "Hello",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := ":This is a comment\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_FullEvent(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Comment: "Test event",
		Type:    "update",
		ID:      "456",
		Retry:   5000,
		Data:    "Hello",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields are present
	if !strings.Contains(result, ":Test event\n") {
		t.Error("expected comment to be present")
	}
	if !strings.Contains(result, "event:update\n") {
		t.Error("expected event type to be present")
	}
	if !strings.Contains(result, "id:456\n") {
		t.Error("expected id to be present")
	}
	if !strings.Contains(result, "retry:5000\n") {
		t.Error("expected retry to be present")
	}
	if !strings.Contains(result, "data:Hello\n") {
		t.Error("expected data to be present")
	}
	if !strings.HasSuffix(result, "\n\n") {
		t.Error("expected double newline at end")
	}
}

func TestEncoder_FormatEvent_JSONData(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Data: map[string]interface{}{
			"message": "Hello",
			"count":   42,
		},
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain JSON-encoded data
	if !strings.Contains(result, "data:") {
		t.Error("expected data field")
	}
	if !strings.Contains(result, "message") {
		t.Error("expected JSON to contain 'message'")
	}
	if !strings.Contains(result, "count") {
		t.Error("expected JSON to contain 'count'")
	}
}

func TestEncoder_FormatEvent_UTF8Data(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Data: "Hello 世界 🌍",
	}

	result, err := encoder.FormatEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:Hello 世界 🌍\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEvent_InvalidEventType(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		Type: "invalid\ntype",
		Data: "Hello",
	}

	_, err := encoder.FormatEvent(event)
	if err == nil {
		t.Error("expected error for event type with newline")
	}
}

func TestEncoder_FormatEvent_InvalidID(t *testing.T) {
	encoder := NewEncoder()

	event := &SSEEventDef{
		ID:   "invalid\nid",
		Data: "Hello",
	}

	_, err := encoder.FormatEvent(event)
	if err == nil {
		t.Error("expected error for ID with newline")
	}
}

func TestEncoder_FormatEvent_NilEvent(t *testing.T) {
	encoder := NewEncoder()

	_, err := encoder.FormatEvent(nil)
	if err != ErrInvalidConfig {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestEncoder_FormatComment(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatComment("keepalive")
	expected := ":keepalive\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatComment_MultiLine(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatComment("line1\nline2")
	expected := ":line1\n:line2\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatKeepalive(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatKeepalive()
	expected := ": keepalive\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatRetry(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatRetry(3000)
	expected := "retry:3000\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEventSimple(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatEventSimple("Hello")
	expected := "data:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEventWithID(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatEventWithID("Hello", "123")
	expected := "id:123\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEventWithType(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatEventWithType("message", "Hello")
	expected := "event:message\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEventFull(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatEventFull("message", "Hello", "123", 3000)
	expected := "event:message\nid:123\nretry:3000\ndata:Hello\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEncoder_FormatEventFull_MultilineData(t *testing.T) {
	encoder := NewEncoder()

	result := encoder.FormatEventFull("message", "Line1\nLine2", "123", 0)
	expected := "event:message\nid:123\ndata:Line1\ndata:Line2\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
