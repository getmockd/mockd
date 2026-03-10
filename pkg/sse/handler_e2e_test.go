package sse

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// sseEvent is a parsed SSE event for test assertions.
type sseEvent struct {
	Type    string
	ID      string
	Data    string
	Retry   string
	Comment string
}

// parseSSEEvents parses raw SSE text into structured events.
// It handles the retry directive, comments, and multi-line data fields.
func parseSSEEvents(body string) []sseEvent {
	var events []sseEvent
	var current sseEvent
	hasFields := false

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Blank line dispatches the event
			if hasFields {
				events = append(events, current)
				current = sseEvent{}
				hasFields = false
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			current.Type = line[len("event:"):]
			hasFields = true
		} else if strings.HasPrefix(line, "id:") {
			current.ID = line[len("id:"):]
			hasFields = true
		} else if strings.HasPrefix(line, "data:") {
			if current.Data != "" {
				current.Data += "\n"
			}
			current.Data += line[len("data:"):]
			hasFields = true
		} else if strings.HasPrefix(line, "retry:") {
			current.Retry = line[len("retry:"):]
			hasFields = true
		} else if strings.HasPrefix(line, ":") {
			// SSE comment
			current.Comment = line[1:] // strip the leading colon
			hasFields = true
		}
	}

	// Handle trailing event without final blank line
	if hasFields {
		events = append(events, current)
	}

	return events
}

// newTestMockConfig creates a MockConfiguration for SSE testing with sensible defaults.
func newTestMockConfig(id string, events []mock.SSEEventDef, lifecycle mock.SSELifecycleConfig) *config.MockConfiguration {
	return &config.MockConfiguration{
		ID:   id,
		Type: mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/events"},
			SSE: &mock.SSEConfig{
				Events:    events,
				Lifecycle: lifecycle,
			},
		},
	}
}

// startTestServer creates an httptest.Server that wraps the SSEHandler with a mock config.
func startTestServer(handler *SSEHandler, mockCfg *config.MockConfiguration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r, mockCfg)
	}))
}

// TestHandlerE2E_BasicSSEStream verifies that a basic SSE stream sends
// correct headers and events over a real HTTP connection.
func TestHandlerE2E_BasicSSEStream(t *testing.T) {
	handler := NewSSEHandler(100)
	mockCfg := newTestMockConfig("e2e-basic", []mock.SSEEventDef{
		{Data: "hello"},
		{Data: "world"},
		{Data: "done"},
	}, mock.SSELifecycleConfig{
		MaxEvents: 3,
	})

	ts := startTestServer(handler, mockCfg)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response headers.
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	raw := string(body)
	events := parseSSEEvents(raw)

	// First parsed event should be the retry directive.
	if len(events) == 0 {
		t.Fatal("expected at least one parsed event (retry directive)")
	}
	if events[0].Retry != "3000" {
		t.Errorf("first event retry = %q, want %q", events[0].Retry, "3000")
	}

	// After the retry directive, expect 3 data events.
	dataEvents := events[1:]
	if len(dataEvents) != 3 {
		t.Fatalf("got %d data events, want 3; raw body:\n%s", len(dataEvents), raw)
	}

	wantData := []string{"hello", "world", "done"}
	for i, want := range wantData {
		if dataEvents[i].Data != want {
			t.Errorf("event[%d].Data = %q, want %q", i, dataEvents[i].Data, want)
		}
	}
}

// TestHandlerE2E_EventTypeAndID verifies that event type and id fields
// appear in the SSE wire format.
func TestHandlerE2E_EventTypeAndID(t *testing.T) {
	handler := NewSSEHandler(100)
	mockCfg := newTestMockConfig("e2e-type-id", []mock.SSEEventDef{
		{Type: "update", ID: "evt-1", Data: "first"},
		{Type: "notification", ID: "evt-2", Data: "second"},
	}, mock.SSELifecycleConfig{
		MaxEvents: 2,
	})

	ts := startTestServer(handler, mockCfg)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	events := parseSSEEvents(string(body))
	// Skip retry directive.
	dataEvents := events[1:]
	if len(dataEvents) != 2 {
		t.Fatalf("got %d data events, want 2; raw:\n%s", len(dataEvents), string(body))
	}

	// Event 1
	if dataEvents[0].Type != "update" {
		t.Errorf("event[0].Type = %q, want %q", dataEvents[0].Type, "update")
	}
	if dataEvents[0].ID != "evt-1" {
		t.Errorf("event[0].ID = %q, want %q", dataEvents[0].ID, "evt-1")
	}
	if dataEvents[0].Data != "first" {
		t.Errorf("event[0].Data = %q, want %q", dataEvents[0].Data, "first")
	}

	// Event 2
	if dataEvents[1].Type != "notification" {
		t.Errorf("event[1].Type = %q, want %q", dataEvents[1].Type, "notification")
	}
	if dataEvents[1].ID != "evt-2" {
		t.Errorf("event[1].ID = %q, want %q", dataEvents[1].ID, "evt-2")
	}
	if dataEvents[1].Data != "second" {
		t.Errorf("event[1].Data = %q, want %q", dataEvents[1].Data, "second")
	}
}

// TestHandlerE2E_EventsArriveInOrder verifies that 5 numbered events
// arrive in the correct order.
func TestHandlerE2E_EventsArriveInOrder(t *testing.T) {
	handler := NewSSEHandler(100)

	eventDefs := make([]mock.SSEEventDef, 5)
	for i := range eventDefs {
		eventDefs[i] = mock.SSEEventDef{Data: strings.Repeat("", 0) + string(rune('1'+i))}
	}
	// Use explicit string data for clarity.
	eventDefs[0].Data = "1"
	eventDefs[1].Data = "2"
	eventDefs[2].Data = "3"
	eventDefs[3].Data = "4"
	eventDefs[4].Data = "5"

	mockCfg := newTestMockConfig("e2e-order", eventDefs, mock.SSELifecycleConfig{
		MaxEvents: 5,
	})

	ts := startTestServer(handler, mockCfg)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	events := parseSSEEvents(string(body))
	dataEvents := events[1:] // skip retry directive
	if len(dataEvents) != 5 {
		t.Fatalf("got %d data events, want 5; raw:\n%s", len(dataEvents), string(body))
	}

	for i, ev := range dataEvents {
		want := string(rune('1' + i))
		if ev.Data != want {
			t.Errorf("event[%d].Data = %q, want %q", i, ev.Data, want)
		}
	}
}

// TestHandlerE2E_GracefulTerminationWithFinalEvent verifies that the stream
// sends a final event when configured with graceful termination, then closes.
func TestHandlerE2E_GracefulTerminationWithFinalEvent(t *testing.T) {
	handler := NewSSEHandler(100)
	mockCfg := newTestMockConfig("e2e-graceful", []mock.SSEEventDef{
		{Data: "event-1"},
		{Data: "event-2"},
	}, mock.SSELifecycleConfig{
		MaxEvents: 2,
		Termination: mock.SSETerminationConfig{
			Type:       "graceful",
			FinalEvent: &mock.SSEEventDef{Data: "stream-complete"},
		},
	})

	ts := startTestServer(handler, mockCfg)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	events := parseSSEEvents(string(body))
	dataEvents := events[1:] // skip retry directive

	// Expect 2 regular events + 1 final event = 3 total.
	if len(dataEvents) != 3 {
		t.Fatalf("got %d data events, want 3 (2 regular + 1 final); raw:\n%s", len(dataEvents), string(body))
	}

	if dataEvents[0].Data != "event-1" {
		t.Errorf("event[0].Data = %q, want %q", dataEvents[0].Data, "event-1")
	}
	if dataEvents[1].Data != "event-2" {
		t.Errorf("event[1].Data = %q, want %q", dataEvents[1].Data, "event-2")
	}
	if dataEvents[2].Data != "stream-complete" {
		t.Errorf("final event Data = %q, want %q", dataEvents[2].Data, "stream-complete")
	}
}

// TestHandlerE2E_NonFlusherReturnsError verifies that calling ServeHTTP with a
// ResponseWriter that does not implement http.Flusher returns an HTTP 500.
func TestHandlerE2E_NonFlusherReturnsError(t *testing.T) {
	handler := NewSSEHandler(100)
	mockCfg := newTestMockConfig("e2e-noflusher", []mock.SSEEventDef{
		{Data: "test"},
	}, mock.SSELifecycleConfig{
		MaxEvents: 1,
	})

	w := &e2eNonFlushWriter{header: make(http.Header)}
	r := httptest.NewRequest(http.MethodGet, "/events", nil)

	handler.ServeHTTP(w, r, mockCfg)

	if w.code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.code, http.StatusInternalServerError)
	}

	if !strings.Contains(string(w.body), "Streaming not supported") {
		t.Errorf("body = %q, want it to contain %q", string(w.body), "Streaming not supported")
	}
}

// e2eNonFlushWriter is an http.ResponseWriter that does NOT implement
// http.Flusher. It captures status code and body for assertions.
type e2eNonFlushWriter struct {
	header http.Header
	body   []byte
	code   int
}

func (w *e2eNonFlushWriter) Header() http.Header { return w.header }
func (w *e2eNonFlushWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}
func (w *e2eNonFlushWriter) WriteHeader(code int) { w.code = code }
