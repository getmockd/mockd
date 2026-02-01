package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// =============================================================================
// REGRESSION TESTS - Bug 3.8 & 3.9 fixes
// =============================================================================

// TestMiddleware_NilLogger_NoPanic ensures passing nil logger doesn't panic (Bug 3.9)
func TestMiddleware_NilLogger_NoPanic(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// This should not panic - nil logger should be replaced with NoOpLogger
	middleware := NewMiddleware(handler, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// TestMiddleware_LargeRequestBody_BoundedMemory verifies body preview is limited (Bug 3.8)
func TestMiddleware_LargeRequestBody_BoundedMemory(t *testing.T) {
	t.Parallel()

	const maxPreview = 256
	const bodySize = 10 * 1024 * 1024 // 10MB

	captured := &capturingLogger{}
	config := &AuditConfig{
		Enabled:            true,
		MaxBodyPreviewSize: maxPreview,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the body to ensure it was reconstructed properly
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		if n != bodySize {
			t.Errorf("expected to read %d bytes, got %d", bodySize, n)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewMiddleware(handler, captured, config)

	// Create large body
	largeBody := make([]byte, bodySize)
	for i := range largeBody {
		largeBody[i] = byte('A' + (i % 26))
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(largeBody))
	req.ContentLength = int64(bodySize)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify captured preview is bounded
	entries := captured.Entries()
	if len(entries) < 1 {
		t.Fatal("expected at least 1 audit entry")
	}

	requestEntry := entries[0]
	if requestEntry.Request == nil {
		t.Fatal("expected request info in audit entry")
	}

	if len(requestEntry.Request.BodyPreview) > maxPreview {
		t.Errorf("body preview exceeded max: got %d, max %d",
			len(requestEntry.Request.BodyPreview), maxPreview)
	}
}

// TestMiddleware_LargeResponseBody_BoundedCapture verifies response capture is limited (Bug 3.8)
func TestMiddleware_LargeResponseBody_BoundedCapture(t *testing.T) {
	t.Parallel()

	const maxPreview = 512
	const responseSize = 5 * 1024 * 1024 // 5MB

	captured := &capturingLogger{}
	config := &AuditConfig{
		Enabled:            true,
		MaxBodyPreviewSize: maxPreview,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		// Write large response in chunks
		chunk := make([]byte, 4096)
		for i := range chunk {
			chunk[i] = byte('X')
		}
		written := 0
		for written < responseSize {
			toWrite := 4096
			if written+toWrite > responseSize {
				toWrite = responseSize - written
			}
			w.Write(chunk[:toWrite])
			written += toWrite
		}
	})

	middleware := NewMiddleware(handler, captured, config)

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Verify response was written in full to client
	if rec.Body.Len() != responseSize {
		t.Errorf("expected response size %d, got %d", responseSize, rec.Body.Len())
	}

	// Verify captured response preview is bounded
	entries := captured.Entries()
	if len(entries) < 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}

	responseEntry := entries[1] // Second entry is response
	if responseEntry.Response == nil {
		t.Fatal("expected response info in audit entry")
	}

	if len(responseEntry.Response.BodyPreview) > maxPreview {
		t.Errorf("response body preview exceeded max: got %d, max %d",
			len(responseEntry.Response.BodyPreview), maxPreview)
	}

	// Verify BodySize tracks full size, not preview
	if responseEntry.Response.BodySize != int64(responseSize) {
		t.Errorf("expected BodySize %d, got %d", responseSize, responseEntry.Response.BodySize)
	}
}

// =============================================================================
// Registry Tests - Race condition fixes
// =============================================================================

// TestRegistry_ConcurrentRegisterWriter tests concurrent writer registration
func TestRegistry_ConcurrentRegisterWriter(t *testing.T) {
	// Note: Cannot use t.Parallel() as this modifies global registry

	// Clean up before and after
	registryMu.Lock()
	originalWriters := make(map[string]WriterFactory)
	for k, v := range registeredWriters {
		originalWriters[k] = v
	}
	registeredWriters = make(map[string]WriterFactory)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registeredWriters = originalWriters
		registryMu.Unlock()
	}()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "writer-" + string(rune('a'+idx%26))
			RegisterWriter(name, func(config map[string]interface{}) (AuditLogger, error) {
				return &NoOpLogger{}, nil
			})
		}(i)
	}

	// Also do concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "writer-" + string(rune('a'+idx%26))
			GetRegisteredWriter(name)
		}(i)
	}

	wg.Wait()
	// No race detected = pass (use -race flag)
}

// TestRegistry_GetRegisteredWriter_NotFound returns false for unknown writer
func TestRegistry_GetRegisteredWriter_NotFound(t *testing.T) {
	t.Parallel()

	_, ok := GetRegisteredWriter("nonexistent-writer-xyz")
	if ok {
		t.Error("expected GetRegisteredWriter to return false for unknown writer")
	}
}

// TestRegistry_RegisterRedactor tests redactor registration
func TestRegistry_RegisterRedactor(t *testing.T) {
	// Note: Cannot use t.Parallel() as this modifies global registry

	registryMu.Lock()
	originalRedactor := registeredRedactor
	registeredRedactor = nil
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registeredRedactor = originalRedactor
		registryMu.Unlock()
	}()

	// Initially nil
	if fn := GetRegisteredRedactor(); fn != nil {
		t.Error("expected nil redactor initially")
	}

	// Register a redactor
	called := false
	RegisterRedactor(func(entry *AuditEntry) *AuditEntry {
		called = true
		entry.TraceID = "redacted"
		return entry
	})

	// Should be registered now
	fn := GetRegisteredRedactor()
	if fn == nil {
		t.Fatal("expected redactor to be registered")
	}

	// Test it works
	entry := NewAuditEntry("test", "original-trace")
	result := fn(entry)

	if !called {
		t.Error("redactor was not called")
	}
	if result.TraceID != "redacted" {
		t.Errorf("expected redacted trace ID, got %s", result.TraceID)
	}
}

// =============================================================================
// FileLogger Tests
// =============================================================================

// TestFileLogger_WriteAndClose tests basic write then close
func TestFileLogger_WriteAndClose(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	entry := NewAuditEntry(EventRequestReceived, "trace-123")
	entry.Request = &RequestInfo{
		Method: "GET",
		Path:   "/api/test",
	}

	if err := logger.Log(*entry); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	// Verify file contents
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var logged AuditEntry
	if err := json.Unmarshal(data, &logged); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if logged.TraceID != "trace-123" {
		t.Errorf("expected trace ID 'trace-123', got '%s'", logged.TraceID)
	}
	if logged.Event != EventRequestReceived {
		t.Errorf("expected event '%s', got '%s'", EventRequestReceived, logged.Event)
	}
	if logged.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", logged.Sequence)
	}
}

// TestFileLogger_LogAfterClose_ReturnsError ensures logging after close returns error
func TestFileLogger_LogAfterClose_ReturnsError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	// Close first
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	// Now try to log
	entry := NewAuditEntry(EventRequestReceived, "trace-after-close")
	err = logger.Log(*entry)

	if err == nil {
		t.Error("expected error when logging after close, got nil")
	}

	if !strings.Contains(err.Error(), "logger is closed") {
		t.Errorf("expected 'logger is closed' error, got: %v", err)
	}
}

// TestFileLogger_ConcurrentWrites tests concurrent write safety
func TestFileLogger_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "concurrent.log")

	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}
	defer logger.Close()

	const numWriters = 50
	const entriesPerWriter = 20

	var wg sync.WaitGroup
	var errCount int64

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < entriesPerWriter; j++ {
				entry := NewAuditEntry(EventRequestReceived, "trace-concurrent")
				entry.Request = &RequestInfo{
					Method: "GET",
					Path:   "/concurrent",
				}
				if err := logger.Log(*entry); err != nil {
					atomic.AddInt64(&errCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if errCount > 0 {
		t.Errorf("got %d errors during concurrent writes", errCount)
	}

	// Verify all entries were written
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	expectedLines := numWriters * entriesPerWriter

	if len(lines) != expectedLines {
		t.Errorf("expected %d log lines, got %d", expectedLines, len(lines))
	}

	// Verify each line is valid JSON and sequence numbers are unique
	sequences := make(map[int64]bool)
	for i, line := range lines {
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}
		if sequences[entry.Sequence] {
			t.Errorf("duplicate sequence number: %d", entry.Sequence)
		}
		sequences[entry.Sequence] = true
	}
}

// TestFileLogger_DoubleClose tests closing twice doesn't error
func TestFileLogger_DoubleClose(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "double-close.log")

	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create file logger: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}

	// Second close should be safe
	if err := logger.Close(); err != nil {
		t.Errorf("second close should not error, got: %v", err)
	}
}

// =============================================================================
// NoOpLogger Tests
// =============================================================================

// TestNoOpLogger_LogReturnsNil verifies no-op behavior
func TestNoOpLogger_LogReturnsNil(t *testing.T) {
	t.Parallel()

	logger := &NoOpLogger{}

	entry := NewAuditEntry(EventRequestReceived, "trace-noop")
	err := logger.Log(*entry)

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestNoOpLogger_CloseReturnsNil verifies close returns nil
func TestNoOpLogger_CloseReturnsNil(t *testing.T) {
	t.Parallel()

	logger := &NoOpLogger{}

	err := logger.Close()

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// =============================================================================
// MultiWriter Tests
// =============================================================================

// TestMultiWriter_FanOut verifies entries are written to all writers
func TestMultiWriter_FanOut(t *testing.T) {
	t.Parallel()

	logger1 := &capturingLogger{}
	logger2 := &capturingLogger{}
	logger3 := &capturingLogger{}

	multi := NewMultiWriter(logger1, logger2, logger3)

	entry := NewAuditEntry(EventRequestReceived, "trace-multi")
	if err := multi.Log(*entry); err != nil {
		t.Fatalf("failed to log: %v", err)
	}

	for i, logger := range []*capturingLogger{logger1, logger2, logger3} {
		entries := logger.Entries()
		if len(entries) != 1 {
			t.Errorf("logger %d: expected 1 entry, got %d", i, len(entries))
		}
	}
}

// TestMultiWriter_NilWritersFiltered verifies nil writers are filtered out
func TestMultiWriter_NilWritersFiltered(t *testing.T) {
	t.Parallel()

	logger1 := &capturingLogger{}
	multi := NewMultiWriter(nil, logger1, nil, nil)

	if multi.Len() != 1 {
		t.Errorf("expected 1 writer after filtering nils, got %d", multi.Len())
	}

	entry := NewAuditEntry(EventRequestReceived, "trace-filter")
	if err := multi.Log(*entry); err != nil {
		t.Fatalf("failed to log: %v", err)
	}

	if len(logger1.Entries()) != 1 {
		t.Error("expected entry to be logged")
	}
}

// TestMultiWriter_ContinuesOnError verifies all writers get entry even if some fail
func TestMultiWriter_ContinuesOnError(t *testing.T) {
	t.Parallel()

	logger1 := &capturingLogger{}
	failingLogger := &failingLogger{}
	logger2 := &capturingLogger{}

	multi := NewMultiWriter(logger1, failingLogger, logger2)

	entry := NewAuditEntry(EventRequestReceived, "trace-error")
	err := multi.Log(*entry)

	// Should return error
	if err == nil {
		t.Error("expected error from failing logger")
	}

	// But both successful loggers should have received the entry
	if len(logger1.Entries()) != 1 {
		t.Error("logger1 should have received entry")
	}
	if len(logger2.Entries()) != 1 {
		t.Error("logger2 should have received entry")
	}
}

// TestMultiWriter_ConcurrentAddRemove tests concurrent modifications
func TestMultiWriter_ConcurrentAddRemove(t *testing.T) {
	t.Parallel()

	multi := NewMultiWriter()

	var wg sync.WaitGroup
	const iterations = 100

	// Concurrent adds
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			multi.Add(&NoOpLogger{})
		}()
	}

	// Concurrent logs
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry := NewAuditEntry(EventRequestReceived, "trace")
			multi.Log(*entry)
		}()
	}

	// Concurrent length checks
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			multi.Len()
		}()
	}

	wg.Wait()
	// No race = pass
}

// =============================================================================
// Config Tests
// =============================================================================

// TestConfig_Validate tests config validation
func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    *AuditConfig
		wantError bool
	}{
		{
			name:      "disabled config is always valid",
			config:    &AuditConfig{Enabled: false, Level: "invalid"},
			wantError: false,
		},
		{
			name:      "valid debug level",
			config:    &AuditConfig{Enabled: true, Level: LevelDebug},
			wantError: false,
		},
		{
			name:      "valid info level",
			config:    &AuditConfig{Enabled: true, Level: LevelInfo},
			wantError: false,
		},
		{
			name:      "valid empty level defaults to info",
			config:    &AuditConfig{Enabled: true, Level: ""},
			wantError: false,
		},
		{
			name:      "invalid level",
			config:    &AuditConfig{Enabled: true, Level: "invalid"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestConfig_ShouldLog tests level filtering
func TestConfig_ShouldLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configLvl string
		eventLvl  string
		wantLog   bool
	}{
		{"debug logs debug", LevelDebug, LevelDebug, true},
		{"debug logs info", LevelDebug, LevelInfo, true},
		{"debug logs error", LevelDebug, LevelError, true},
		{"info skips debug", LevelInfo, LevelDebug, false},
		{"info logs info", LevelInfo, LevelInfo, true},
		{"error skips info", LevelError, LevelInfo, false},
		{"error logs error", LevelError, LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := &AuditConfig{Enabled: true, Level: tt.configLvl}
			got := config.ShouldLog(tt.eventLvl)
			if got != tt.wantLog {
				t.Errorf("ShouldLog(%s) = %v, want %v", tt.eventLvl, got, tt.wantLog)
			}
		})
	}
}

// TestConfig_ShouldLog_Disabled verifies disabled config never logs
func TestConfig_ShouldLog_Disabled(t *testing.T) {
	t.Parallel()

	config := &AuditConfig{Enabled: false, Level: LevelDebug}

	if config.ShouldLog(LevelError) {
		t.Error("disabled config should not log anything")
	}
}

// =============================================================================
// Middleware Additional Tests
// =============================================================================

// TestMiddleware_NilConfig_UsesDefaults verifies nil config uses defaults
func TestMiddleware_NilConfig_UsesDefaults(t *testing.T) {
	t.Parallel()

	captured := &capturingLogger{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewMiddleware(handler, captured, nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// TestMiddleware_RequestBodyReconstructed verifies body is still readable by handler
func TestMiddleware_RequestBodyReconstructed(t *testing.T) {
	t.Parallel()

	const requestBody = "test request body content"
	var handlerReceivedBody string

	captured := &capturingLogger{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		handlerReceivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewMiddleware(handler, captured, nil)

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(requestBody))
	req.ContentLength = int64(len(requestBody))
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if handlerReceivedBody != requestBody {
		t.Errorf("handler received body %q, expected %q", handlerReceivedBody, requestBody)
	}
}

// TestMiddleware_CapturesStatusCode verifies status code is captured
func TestMiddleware_CapturesStatusCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			captured := &capturingLogger{}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			middleware := NewMiddleware(handler, captured, nil)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			entries := captured.Entries()
			if len(entries) < 2 {
				t.Fatalf("expected 2 entries, got %d", len(entries))
			}

			responseEntry := entries[1]
			if responseEntry.Response == nil {
				t.Fatal("expected response info")
			}
			if responseEntry.Response.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, responseEntry.Response.StatusCode)
			}
		})
	}
}

// TestMiddleware_NoWriteHeader_Defaults200 verifies default status when WriteHeader not called
func TestMiddleware_NoWriteHeader_Defaults200(t *testing.T) {
	t.Parallel()

	captured := &capturingLogger{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't call WriteHeader, just Write
		w.Write([]byte("response without explicit status"))
	})

	middleware := NewMiddleware(handler, captured, nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	entries := captured.Entries()
	if len(entries) < 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	responseEntry := entries[1]
	if responseEntry.Response.StatusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", responseEntry.Response.StatusCode)
	}
}

// =============================================================================
// AuditEntry Builder Tests
// =============================================================================

// TestAuditEntry_BuilderChain verifies fluent builder pattern
func TestAuditEntry_BuilderChain(t *testing.T) {
	t.Parallel()

	entry := NewAuditEntry(EventRequestReceived, "trace-123").
		WithRequest(&RequestInfo{Method: "GET", Path: "/api"}).
		WithResponse(&ResponseInfo{StatusCode: 200}).
		WithClient(&ClientInfo{RemoteAddr: "127.0.0.1"}).
		WithMock(&MockInfo{ID: "mock-1"}).
		WithMetadata(&EntryMetadata{ServerID: "server-1"})

	if entry.TraceID != "trace-123" {
		t.Errorf("expected trace ID 'trace-123', got '%s'", entry.TraceID)
	}
	if entry.Request == nil || entry.Request.Method != "GET" {
		t.Error("request not set correctly")
	}
	if entry.Response == nil || entry.Response.StatusCode != 200 {
		t.Error("response not set correctly")
	}
	if entry.Client == nil || entry.Client.RemoteAddr != "127.0.0.1" {
		t.Error("client not set correctly")
	}
	if entry.Mock == nil || entry.Mock.ID != "mock-1" {
		t.Error("mock not set correctly")
	}
	if entry.Metadata == nil || entry.Metadata.ServerID != "server-1" {
		t.Error("metadata not set correctly")
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// capturingLogger captures all logged entries for test verification
type capturingLogger struct {
	mu      sync.Mutex
	entries []AuditEntry
}

func (l *capturingLogger) Log(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
	return nil
}

func (l *capturingLogger) Close() error {
	return nil
}

func (l *capturingLogger) Entries() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]AuditEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// failingLogger always returns an error
type failingLogger struct{}

func (l *failingLogger) Log(entry AuditEntry) error {
	return &testError{msg: "intentional failure"}
}

func (l *failingLogger) Close() error {
	return nil
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
