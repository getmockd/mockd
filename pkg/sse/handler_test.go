package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

func TestSSEHandler_SetSSEHeaders(t *testing.T) {
	handler := NewSSEHandler(100)
	w := httptest.NewRecorder()

	handler.setSSEHeaders(w)

	// Check Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != ContentTypeEventStream {
		t.Errorf("expected Content-Type %q, got %q", ContentTypeEventStream, contentType)
	}

	// Check Cache-Control
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got %q", cacheControl)
	}

	// Check Connection
	connection := w.Header().Get("Connection")
	if connection != "keep-alive" {
		t.Errorf("expected Connection 'keep-alive', got %q", connection)
	}

	// Check X-Accel-Buffering (nginx)
	accelBuffering := w.Header().Get("X-Accel-Buffering")
	if accelBuffering != "no" {
		t.Errorf("expected X-Accel-Buffering 'no', got %q", accelBuffering)
	}
}

func TestSSEHandler_FlusherDetection(t *testing.T) {
	handler := NewSSEHandler(100)

	// Create a mock configuration
	mockCfg := &config.MockConfiguration{
		ID:   "test-sse",
		Name: "Test SSE",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			SSE: &mock.SSEConfig{
				Events: []mock.SSEEventDef{
					{Data: "test"},
				},
			},
		},
	}

	// Test with non-flushing response writer
	t.Run("non-flushing writer", func(t *testing.T) {
		w := &nonFlushingResponseWriter{header: make(http.Header)}
		r := httptest.NewRequest(http.MethodGet, "/events", nil)

		handler.ServeHTTP(w, r, mockCfg)

		// Should return error about streaming not supported
		if w.code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.code)
		}
	})
}

// nonFlushingResponseWriter is a ResponseWriter that doesn't implement Flusher
type nonFlushingResponseWriter struct {
	header http.Header
	body   []byte
	code   int
}

func (w *nonFlushingResponseWriter) Header() http.Header {
	return w.header
}

func (w *nonFlushingResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *nonFlushingResponseWriter) WriteHeader(code int) {
	w.code = code
}

func TestSSEHandler_GenerateStreamID(t *testing.T) {
	handler := NewSSEHandler(100)

	id1 := handler.generateStreamID()
	id2 := handler.generateStreamID()

	if id1 == id2 {
		t.Error("expected unique stream IDs")
	}

	if !strings.HasPrefix(id1, "sse-") {
		t.Errorf("expected ID to start with 'sse-', got %q", id1)
	}
}

func TestSSEHandler_ConfigFromMock(t *testing.T) {
	handler := NewSSEHandler(100)

	delay := 100
	mockCfg := &mock.SSEConfig{
		Events: []mock.SSEEventDef{
			{Type: "message", Data: "Hello", ID: "1"},
		},
		Timing: mock.SSETimingConfig{
			FixedDelay:   &delay,
			InitialDelay: 50,
		},
		Lifecycle: mock.SSELifecycleConfig{
			KeepaliveInterval: 15,
			MaxEvents:         100,
		},
		Resume: mock.SSEResumeConfig{
			Enabled:    true,
			BufferSize: 50,
		},
	}

	sseConfig := handler.configFromMock(mockCfg)

	// Verify events
	if len(sseConfig.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(sseConfig.Events))
	}
	if sseConfig.Events[0].Type != "message" {
		t.Errorf("expected event type 'message', got %q", sseConfig.Events[0].Type)
	}

	// Verify timing
	if sseConfig.Timing.InitialDelay != 50 {
		t.Errorf("expected initial delay 50, got %d", sseConfig.Timing.InitialDelay)
	}
	if sseConfig.Timing.FixedDelay == nil || *sseConfig.Timing.FixedDelay != 100 {
		t.Error("expected fixed delay 100")
	}

	// Verify lifecycle
	if sseConfig.Lifecycle.KeepaliveInterval != 15 {
		t.Errorf("expected keepalive 15, got %d", sseConfig.Lifecycle.KeepaliveInterval)
	}
	if sseConfig.Lifecycle.MaxEvents != 100 {
		t.Errorf("expected max events 100, got %d", sseConfig.Lifecycle.MaxEvents)
	}

	// Verify resume
	if !sseConfig.Resume.Enabled {
		t.Error("expected resume to be enabled")
	}
	if sseConfig.Resume.BufferSize != 50 {
		t.Errorf("expected buffer size 50, got %d", sseConfig.Resume.BufferSize)
	}
}

func TestSSEHandler_GetManager(t *testing.T) {
	handler := NewSSEHandler(100)
	manager := handler.GetManager()

	if manager == nil {
		t.Error("expected manager to not be nil")
	}
}

func TestSSEHandler_GetTemplates(t *testing.T) {
	handler := NewSSEHandler(100)
	templates := handler.GetTemplates()

	if templates == nil {
		t.Error("expected templates to not be nil")
	}

	// Check built-in templates
	if _, ok := templates.Get(TemplateOpenAIChat); !ok {
		t.Error("expected openai-chat template to be registered")
	}
}

func TestSSEHandler_Buffer(t *testing.T) {
	handler := NewSSEHandler(100)

	// Initially no buffer
	buffer := handler.GetBuffer("test-mock")
	if buffer != nil {
		t.Error("expected no buffer initially")
	}

	// Buffer an event
	handler.bufferEvent("test-mock", &SSEEventDef{Data: "test", ID: "1"}, 0)

	// Now buffer should exist
	buffer = handler.GetBuffer("test-mock")
	if buffer == nil {
		t.Error("expected buffer to exist")
	}
	if buffer.Size() != 1 {
		t.Errorf("expected buffer size 1, got %d", buffer.Size())
	}

	// Clear buffer
	handler.ClearBuffer("test-mock")
	buffer = handler.GetBuffer("test-mock")
	if buffer != nil {
		t.Error("expected buffer to be cleared")
	}
}

func TestFormatStreamID(t *testing.T) {
	tests := []struct {
		id       int64
		expected string
	}{
		{1, "sse-1"},
		{42, "sse-42"},
		{100, "sse-100"},
		{0, "sse-0"},
	}

	for _, tc := range tests {
		result := formatStreamID(tc.id)
		if result != tc.expected {
			t.Errorf("formatStreamID(%d) = %q, expected %q", tc.id, result, tc.expected)
		}
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123456789, "123456789"},
	}

	for _, tc := range tests {
		result := formatInt64(tc.n)
		if result != tc.expected {
			t.Errorf("formatInt64(%d) = %q, expected %q", tc.n, result, tc.expected)
		}
	}
}

func TestSSEStream_Properties(t *testing.T) {
	now := time.Now()
	stream := &SSEStream{
		ID:         "test-1",
		MockID:     "mock-1",
		ClientIP:   "127.0.0.1:1234",
		UserAgent:  "test-agent",
		StartTime:  now,
		EventsSent: 10,
		BytesSent:  1024,
		Status:     StreamStatusActive,
	}

	if stream.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got %q", stream.ID)
	}
	if stream.MockID != "mock-1" {
		t.Errorf("expected MockID 'mock-1', got %q", stream.MockID)
	}
	if stream.Status != StreamStatusActive {
		t.Errorf("expected status Active, got %v", stream.Status)
	}
}
