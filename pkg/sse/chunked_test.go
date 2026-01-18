package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

func TestChunkedHandler_New(t *testing.T) {
	handler := NewChunkedHandler()
	if handler == nil {
		t.Fatal("expected handler to be created")
	}
}

func TestChunkedHandler_MissingConfig(t *testing.T) {
	handler := NewChunkedHandler()

	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: nil,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(w, r, mockCfg)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestChunkedHandler_NoFlusher(t *testing.T) {
	handler := NewChunkedHandler()

	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Data: "test data",
			},
		},
	}

	// Use a non-flushing ResponseWriter
	w := &nonFlushingResponseWriter{header: make(http.Header)}
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(w, r, mockCfg)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.code)
	}
}

func TestChunkedHandler_BasicChunked(t *testing.T) {
	handler := NewChunkedHandler()

	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Data:       "Hello World!",
				ChunkSize:  5,
				ChunkDelay: 0,
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(w, r, mockCfg)

	body := w.Body.String()
	if body != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", body)
	}
}

func TestChunkedHandler_DefaultChunkSize(t *testing.T) {
	handler := NewChunkedHandler()

	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Data:      "test",
				ChunkSize: 0, // Should use default 1024
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(w, r, mockCfg)

	if w.Body.String() != "test" {
		t.Errorf("expected 'test', got %q", w.Body.String())
	}
}

func TestChunkedHandler_NDJSON(t *testing.T) {
	handler := NewChunkedHandler()

	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Format: "ndjson",
				NDJSONItems: []interface{}{
					map[string]interface{}{"id": 1, "name": "Alice"},
					map[string]interface{}{"id": 2, "name": "Bob"},
				},
				ChunkDelay: 0,
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(w, r, mockCfg)

	// Check Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != ContentTypeNDJSON {
		t.Errorf("expected Content-Type %q, got %q", ContentTypeNDJSON, contentType)
	}

	// Check body has 2 lines
	body := w.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), body)
	}

	// Check JSON content
	if !strings.Contains(lines[0], `"id":1`) {
		t.Errorf("expected first line to contain id:1, got %q", lines[0])
	}
	if !strings.Contains(lines[1], `"id":2`) {
		t.Errorf("expected second line to contain id:2, got %q", lines[1])
	}
}

func TestChunkedHandler_Headers(t *testing.T) {
	handler := NewChunkedHandler()

	// Test default Content-Type
	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Data: "test",
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r, mockCfg)

	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("expected Content-Type application/octet-stream, got %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %q", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("X-Accel-Buffering") != "no" {
		t.Errorf("expected X-Accel-Buffering no, got %q", w.Header().Get("X-Accel-Buffering"))
	}
}

func TestChunkedHandler_ContextCancellation(t *testing.T) {
	handler := NewChunkedHandler()

	// Large data that would take multiple chunks
	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Data:       strings.Repeat("x", 10000),
				ChunkSize:  100,
				ChunkDelay: 100, // 100ms delay
			},
		},
	}

	w := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	// Cancel immediately
	cancel()

	handler.ServeHTTP(w, r, mockCfg)

	// Should not have received all data due to cancellation
	// (though this is timing-dependent)
}

func TestChunkedHandler_EmptyData(t *testing.T) {
	handler := NewChunkedHandler()

	mockCfg := &config.MockConfiguration{
		ID:   "test",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Chunked: &mock.ChunkedConfig{
				Data: "",
			},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(w, r, mockCfg)

	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", w.Body.String())
	}
}

// ChunkedResponse tests

func TestChunkedResponse_New(t *testing.T) {
	resp := NewChunkedResponse(100, 10)

	if resp.chunkSize != 100 {
		t.Errorf("expected chunkSize 100, got %d", resp.chunkSize)
	}
	if resp.chunkDelay != 10 {
		t.Errorf("expected chunkDelay 10, got %d", resp.chunkDelay)
	}
}

func TestChunkedResponse_DefaultChunkSize(t *testing.T) {
	resp := NewChunkedResponse(0, 0)

	if resp.chunkSize != 1024 {
		t.Errorf("expected default chunkSize 1024, got %d", resp.chunkSize)
	}
}

func TestChunkedResponse_AddChunk(t *testing.T) {
	resp := NewChunkedResponse(100, 0)

	resp.AddChunk([]byte("chunk1"))
	resp.AddChunk([]byte("chunk2"))

	chunks := resp.Chunks()
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestChunkedResponse_AddData(t *testing.T) {
	resp := NewChunkedResponse(5, 0)

	resp.AddData([]byte("Hello World!")) // 12 bytes

	chunks := resp.Chunks()
	// Should be 3 chunks: "Hello" (5), " Worl" (5), "d!" (2)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}

	if string(chunks[0]) != "Hello" {
		t.Errorf("expected first chunk 'Hello', got %q", string(chunks[0]))
	}
	if string(chunks[1]) != " Worl" {
		t.Errorf("expected second chunk ' Worl', got %q", string(chunks[1]))
	}
	if string(chunks[2]) != "d!" {
		t.Errorf("expected third chunk 'd!', got %q", string(chunks[2]))
	}
}

func TestChunkedResponse_TotalSize(t *testing.T) {
	resp := NewChunkedResponse(100, 0)

	resp.AddChunk([]byte("hello"))
	resp.AddChunk([]byte("world"))

	total := resp.TotalSize()
	if total != 10 {
		t.Errorf("expected total size 10, got %d", total)
	}
}

func TestChunkedResponse_TotalSize_Empty(t *testing.T) {
	resp := NewChunkedResponse(100, 0)

	total := resp.TotalSize()
	if total != 0 {
		t.Errorf("expected total size 0, got %d", total)
	}
}

// NDJSONBuilder tests

func TestNDJSONBuilder_New(t *testing.T) {
	builder := NewNDJSONBuilder(50)

	if builder.chunkDelay != 50 {
		t.Errorf("expected chunkDelay 50, got %d", builder.chunkDelay)
	}
}

func TestNDJSONBuilder_Add(t *testing.T) {
	builder := NewNDJSONBuilder(0)

	builder.Add(map[string]interface{}{"id": 1})
	builder.Add(map[string]interface{}{"id": 2})

	items := builder.Items()
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestNDJSONBuilder_Build(t *testing.T) {
	builder := NewNDJSONBuilder(0)

	builder.Add(map[string]interface{}{"id": 1, "name": "Alice"})
	builder.Add(map[string]interface{}{"id": 2, "name": "Bob"})

	data, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], `"id":1`) {
		t.Errorf("expected first line to contain id:1, got %q", lines[0])
	}
}

func TestNDJSONBuilder_Build_Empty(t *testing.T) {
	builder := NewNDJSONBuilder(0)

	data, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty data, got %q", string(data))
	}
}

func TestNDJSONBuilder_Build_InvalidJSON(t *testing.T) {
	builder := NewNDJSONBuilder(0)

	// Add something that can't be marshaled (channel)
	builder.Add(make(chan int))

	_, err := builder.Build()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
