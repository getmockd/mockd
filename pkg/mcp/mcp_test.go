package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// REGRESSION TESTS - Bug Fixes
// =============================================================================

// TestSession_SetClientData_Concurrent verifies the fix for Bug 3.10:
// Data race on session fields when calling SetClientData concurrently.
func TestSession_SetClientData_Concurrent(t *testing.T) {
	t.Parallel()

	session := NewSession()
	var wg sync.WaitGroup
	iterations := 100

	// Spawn multiple goroutines that read and write session fields
	for i := 0; i < iterations; i++ {
		wg.Add(3)

		// Writer goroutine
		go func(n int) {
			defer wg.Done()
			session.SetClientData(
				"2025-06-18",
				ClientInfo{Name: "client", Version: "1.0"},
				ClientCapabilities{},
			)
		}(i)

		// Reader goroutine for state
		go func() {
			defer wg.Done()
			_ = session.GetState()
		}()

		// Another writer for state
		go func(n int) {
			defer wg.Done()
			session.SetState(SessionState(n % 4))
		}(i)
	}

	wg.Wait()
	// If we get here without race detector panic, the fix works
}

// TestFormatString_BasicPlaceholders verifies %s, %d, %v work correctly.
func TestFormatString_BasicPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "string placeholder",
			format:   "hello %s",
			args:     []interface{}{"world"},
			expected: "hello world",
		},
		{
			name:     "int placeholder with %d",
			format:   "count: %d",
			args:     []interface{}{42},
			expected: "count: 42",
		},
		{
			name:     "int placeholder with %v",
			format:   "value: %v",
			args:     []interface{}{123},
			expected: "value: 123",
		},
		{
			name:     "multiple placeholders",
			format:   "%s has %d items",
			args:     []interface{}{"cart", 5},
			expected: "cart has 5 items",
		},
		{
			name:     "error type",
			format:   "error: %v",
			args:     []interface{}{errors.New("something failed")},
			expected: "error: something failed",
		},
		{
			name:     "bool true",
			format:   "enabled: %v",
			args:     []interface{}{true},
			expected: "enabled: true",
		},
		{
			name:     "bool false",
			format:   "enabled: %v",
			args:     []interface{}{false},
			expected: "enabled: false",
		},
		{
			name:     "int64 type",
			format:   "big: %d",
			args:     []interface{}{int64(9223372036854775807)},
			expected: "big: 9223372036854775807",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatString(tt.format, tt.args...)
			if result != tt.expected {
				t.Errorf("formatString(%q, %v) = %q, want %q",
					tt.format, tt.args, result, tt.expected)
			}
		})
	}
}

// TestFormatString_EscapedPercent verifies %% produces single %.
// Note: formatString only processes format specifiers when args are provided.
func TestFormatString_EscapedPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "escaped percent with placeholder",
			format:   "%d%% complete",
			args:     []interface{}{50},
			expected: "50% complete",
		},
		{
			name:     "escaped percent at end",
			format:   "value: %s%%",
			args:     []interface{}{"100"},
			expected: "value: 100%",
		},
		{
			name:     "multiple escaped percents with args",
			format:   "%%foo%% bar %s%%",
			args:     []interface{}{"test"},
			expected: "%foo% bar test%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatString(tt.format, tt.args...)
			if result != tt.expected {
				t.Errorf("formatString(%q, %v) = %q, want %q",
					tt.format, tt.args, result, tt.expected)
			}
		})
	}
}

// TestFormatString_MissingArgs handles fewer args than placeholders.
// With fmt.Sprintf, missing args produce %!s(MISSING) etc.
func TestFormatString_MissingArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "one placeholder no args",
			format:   "hello %s",
			args:     nil,
			expected: "hello %s", // no args triggers early return
		},
		{
			name:     "two placeholders one arg",
			format:   "%s and %s",
			args:     []interface{}{"foo"},
			expected: "foo and %!s(MISSING)",
		},
		{
			name:     "three placeholders one arg",
			format:   "%s %d %v",
			args:     []interface{}{"only"},
			expected: "only %!d(MISSING) %!v(MISSING)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatString(tt.format, tt.args...)
			if result != tt.expected {
				t.Errorf("formatString(%q, %v) = %q, want %q",
					tt.format, tt.args, result, tt.expected)
			}
		})
	}
}

// TestFormatString_ExtraArgs handles more args than placeholders.
// With fmt.Sprintf, extra args are appended with %!(EXTRA ...).
func TestFormatString_ExtraArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "one placeholder two args",
			format:   "hello %s",
			args:     []interface{}{"world", "extra"},
			expected: "hello world%!(EXTRA string=extra)",
		},
		{
			name:     "no placeholders with args",
			format:   "static text",
			args:     []interface{}{"ignored", 123},
			expected: "static text%!(EXTRA string=ignored, int=123)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatString(tt.format, tt.args...)
			if result != tt.expected {
				t.Errorf("formatString(%q, %v) = %q, want %q",
					tt.format, tt.args, result, tt.expected)
			}
		})
	}
}

// TestFormatString_NoPlaceholders returns format unchanged.
func TestFormatString_NoPlaceholders(t *testing.T) {
	t.Parallel()

	const format = "this is just plain text"
	result := formatString(format)

	if result != format {
		t.Errorf("formatString(%q) = %q, want %q", format, result, format)
	}
}

// TestSessionManager_Delete_ClosesSession verifies the fix for Bug 1.4:
// Session EventChannel should be closed when session is deleted.
func TestSessionManager_Delete_ClosesSession(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	session, err := manager.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	sessionID := session.ID
	eventChan := session.EventChannel

	// Delete the session
	manager.Delete(sessionID)

	// Verify session is removed from manager
	if got := manager.Get(sessionID); got != nil {
		t.Error("Get() after Delete should return nil")
	}

	// Verify EventChannel is closed by trying to receive
	select {
	case _, ok := <-eventChan:
		if ok {
			t.Error("EventChannel should be closed after Delete")
		}
		// Channel is closed as expected
	default:
		t.Error("EventChannel should be closed and readable, not blocking")
	}
}

// TestServer_DoubleStop_NoPanic verifies the fix for Bug 1.5:
// Calling Stop() twice should not panic due to sync.Once.
func TestServer_DoubleStop_NoPanic(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Port = 19091 // Use a specific port that's likely available

	server := NewServer(cfg, nil, nil)

	// Start the server
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// First stop should succeed
	if err := server.Stop(); err != nil {
		t.Fatalf("First Stop() error = %v", err)
	}

	// Second stop should not panic and should return nil
	// (since running is already false)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Second Stop() caused panic: %v", r)
		}
	}()

	if err := server.Stop(); err != nil {
		t.Errorf("Second Stop() error = %v, want nil", err)
	}
}

// =============================================================================
// SESSION LIFECYCLE TESTS
// =============================================================================

func TestSession_Touch_UpdatesLastActive(t *testing.T) {
	t.Parallel()

	session := NewSession()
	originalTime := session.LastActiveAt

	// Wait briefly to ensure time difference
	time.Sleep(5 * time.Millisecond)

	session.Touch()

	if !session.LastActiveAt.After(originalTime) {
		t.Error("Touch() should update LastActiveAt to a later time")
	}
}

func TestSession_IsExpired_BeforeTimeout(t *testing.T) {
	t.Parallel()

	session := NewSession()
	timeout := time.Hour

	if session.IsExpired(timeout) {
		t.Error("IsExpired() should return false for fresh session")
	}
}

func TestSession_IsExpired_AfterTimeout(t *testing.T) {
	t.Parallel()

	session := NewSession()
	// Use very short timeout for fast test
	timeout := 5 * time.Millisecond

	// Wait for timeout to pass
	time.Sleep(10 * time.Millisecond)

	if !session.IsExpired(timeout) {
		t.Error("IsExpired() should return true after timeout")
	}
}

func TestSession_Subscribe_Unsubscribe(t *testing.T) {
	t.Parallel()

	session := NewSession()
	uri := "mockd://endpoints"

	// Initially not subscribed
	if session.IsSubscribed(uri) {
		t.Error("IsSubscribed() should return false before Subscribe")
	}

	// Subscribe
	session.Subscribe(uri)
	if !session.IsSubscribed(uri) {
		t.Error("IsSubscribed() should return true after Subscribe")
	}

	// Get subscriptions
	subs := session.GetSubscriptions()
	if len(subs) != 1 || subs[0] != uri {
		t.Errorf("GetSubscriptions() = %v, want [%s]", subs, uri)
	}

	// Unsubscribe
	session.Unsubscribe(uri)
	if session.IsSubscribed(uri) {
		t.Error("IsSubscribed() should return false after Unsubscribe")
	}
}

func TestSession_GetState_SetState_ThreadSafe(t *testing.T) {
	t.Parallel()

	session := NewSession()
	var wg sync.WaitGroup
	iterations := 100

	// Test concurrent get/set of state
	for i := 0; i < iterations; i++ {
		wg.Add(2)

		go func(state SessionState) {
			defer wg.Done()
			session.SetState(state)
		}(SessionState(i % 4))

		go func() {
			defer wg.Done()
			_ = session.GetState()
		}()
	}

	wg.Wait()
	// Success if no race detected
}

func TestSession_SendNotification(t *testing.T) {
	t.Parallel()

	session := NewSession()
	notif := NewNotification("test/method", nil)

	// Should succeed with empty channel
	if !session.SendNotification(notif) {
		t.Error("SendNotification() should return true for empty channel")
	}

	// Verify notification was sent
	select {
	case received := <-session.EventChannel:
		if received.Method != notif.Method {
			t.Errorf("received method = %s, want %s", received.Method, notif.Method)
		}
	default:
		t.Error("expected notification in channel")
	}
}

func TestSession_SendNotification_FullChannel(t *testing.T) {
	t.Parallel()

	session := NewSession()

	// Fill the channel to capacity (100)
	for i := 0; i < 100; i++ {
		session.SendNotification(NewNotification("fill", nil))
	}

	// Next send should fail (channel full)
	if session.SendNotification(NewNotification("overflow", nil)) {
		t.Error("SendNotification() should return false for full channel")
	}
}

func TestSession_Close(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)

	session.Close()

	if session.GetState() != SessionStateExpired {
		t.Errorf("GetState() after Close = %v, want %v",
			session.GetState(), SessionStateExpired)
	}

	// Verify channel is closed
	select {
	case _, ok := <-session.EventChannel:
		if ok {
			t.Error("EventChannel should be closed after Close()")
		}
	default:
		t.Error("EventChannel should be closed and immediately readable")
	}
}

// =============================================================================
// SESSION MANAGER TESTS
// =============================================================================

func TestSessionManager_Create_GeneratesID(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	session1, err := manager.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session2, err := manager.Create()
	if err != nil {
		t.Fatalf("Create() second error = %v", err)
	}

	// IDs should be non-empty
	if session1.ID == "" {
		t.Error("session1.ID should not be empty")
	}
	if session2.ID == "" {
		t.Error("session2.ID should not be empty")
	}

	// IDs should be unique
	if session1.ID == session2.ID {
		t.Error("sessions should have unique IDs")
	}

	// IDs should be 32 hex characters (16 bytes)
	if len(session1.ID) != 32 {
		t.Errorf("session ID length = %d, want 32", len(session1.ID))
	}
}

func TestSessionManager_Get_ExistingSession(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	session, _ := manager.Create()

	got := manager.Get(session.ID)
	if got != session {
		t.Error("Get() should return the created session")
	}
}

func TestSessionManager_Get_NonExistent_ReturnsNil(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	got := manager.Get("nonexistent-id")
	if got != nil {
		t.Error("Get() should return nil for nonexistent session")
	}
}

func TestSessionManager_Cleanup_RemovesExpired(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: 5 * time.Millisecond,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	// Create sessions
	session1, _ := manager.Create()
	session2, _ := manager.Create()

	if manager.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", manager.Count())
	}

	// Wait for timeout
	time.Sleep(10 * time.Millisecond)

	// Touch one session to keep it alive
	session2.Touch()

	// Run cleanup
	removed := manager.Cleanup()
	if removed != 1 {
		t.Errorf("Cleanup() removed %d, want 1", removed)
	}

	// Verify session1 is gone, session2 remains
	if manager.Get(session1.ID) != nil {
		t.Error("expired session1 should be removed")
	}
	if manager.Get(session2.ID) == nil {
		t.Error("touched session2 should remain")
	}
}

func TestSessionManager_MaxSessions(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    2,
	}
	manager := NewSessionManager(cfg)

	// Create max sessions
	_, err := manager.Create()
	if err != nil {
		t.Fatalf("Create() 1 error = %v", err)
	}
	_, err = manager.Create()
	if err != nil {
		t.Fatalf("Create() 2 error = %v", err)
	}

	// Third should fail
	_, err = manager.Create()
	if err == nil {
		t.Error("Create() should fail when max sessions reached")
	}
}

func TestSessionManager_Count_List(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	if manager.Count() != 0 {
		t.Errorf("initial Count() = %d, want 0", manager.Count())
	}

	session1, _ := manager.Create()
	session2, _ := manager.Create()

	if manager.Count() != 2 {
		t.Errorf("Count() = %d, want 2", manager.Count())
	}

	ids := manager.List()
	if len(ids) != 2 {
		t.Errorf("List() length = %d, want 2", len(ids))
	}

	// Verify both IDs are in the list
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	if !idMap[session1.ID] || !idMap[session2.ID] {
		t.Error("List() should contain all session IDs")
	}
}

func TestSessionManager_Touch(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	session, _ := manager.Create()
	originalTime := session.LastActiveAt

	time.Sleep(5 * time.Millisecond)
	manager.Touch(session.ID)

	if !session.LastActiveAt.After(originalTime) {
		t.Error("Touch() via manager should update LastActiveAt")
	}

	// Touch nonexistent should not panic
	manager.Touch("nonexistent")
}

func TestSessionManager_Close(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	session1, _ := manager.Create()
	session2, _ := manager.Create()
	ch1 := session1.EventChannel
	ch2 := session2.EventChannel

	manager.Close()

	if manager.Count() != 0 {
		t.Errorf("Count() after Close = %d, want 0", manager.Count())
	}

	// Verify channels are closed
	for _, ch := range []chan *JSONRPCNotification{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("EventChannel should be closed after manager.Close()")
			}
		default:
			t.Error("EventChannel should be closed and immediately readable")
		}
	}
}

func TestSessionManager_Broadcast(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SessionTimeout: time.Hour,
		MaxSessions:    10,
	}
	manager := NewSessionManager(cfg)

	session1, _ := manager.Create()
	session1.SetState(SessionStateReady)

	session2, _ := manager.Create()
	session2.SetState(SessionStateReady)

	// Session in non-ready state should not receive broadcast
	session3, _ := manager.Create()
	session3.SetState(SessionStateInitialized)

	notif := NewNotification("test/broadcast", nil)
	manager.Broadcast(notif)

	// Check session1 received
	select {
	case n := <-session1.EventChannel:
		if n.Method != notif.Method {
			t.Errorf("session1 received wrong method")
		}
	default:
		t.Error("session1 should have received broadcast")
	}

	// Check session2 received
	select {
	case n := <-session2.EventChannel:
		if n.Method != notif.Method {
			t.Errorf("session2 received wrong method")
		}
	default:
		t.Error("session2 should have received broadcast")
	}

	// Check session3 did NOT receive (not ready)
	select {
	case <-session3.EventChannel:
		t.Error("session3 should NOT have received broadcast (not ready)")
	default:
		// Expected
	}
}

// =============================================================================
// JSONRPC TESTS
// =============================================================================

func TestParseRequest_ValidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		method string
		hasID  bool
	}{
		{
			name:   "basic request",
			input:  `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
			method: "ping",
			hasID:  true,
		},
		{
			name:   "string id",
			input:  `{"jsonrpc":"2.0","id":"abc123","method":"initialize"}`,
			method: "initialize",
			hasID:  true,
		},
		{
			name:   "notification (no id)",
			input:  `{"jsonrpc":"2.0","method":"initialized"}`,
			method: "initialized",
			hasID:  false,
		},
		{
			name:   "with params",
			input:  `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test"}}`,
			method: "tools/call",
			hasID:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			req, err := ParseRequest(reader)

			if err != nil {
				t.Fatalf("ParseRequest() error = %v", err)
			}
			if req.Method != tt.method {
				t.Errorf("Method = %s, want %s", req.Method, tt.method)
			}
			if tt.hasID && req.IsNotification() {
				t.Error("expected request with ID, got notification")
			}
			if !tt.hasID && !req.IsNotification() {
				t.Error("expected notification, got request with ID")
			}
		})
	}
}

func TestParseRequest_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "malformed json",
			input: `{"jsonrpc":"2.0"`,
		},
		{
			name:  "not json at all",
			input: `this is not json`,
		},
		{
			name:  "empty input",
			input: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			_, err := ParseRequest(reader)

			if err == nil {
				t.Fatal("ParseRequest() should return error for invalid JSON")
				return
			}
			if err.Code != ErrCodeParseError {
				t.Errorf("error code = %d, want %d (ParseError)",
					err.Code, ErrCodeParseError)
			}
		})
	}
}

func TestValidateRequest_InvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  JSONRPCRequest
	}{
		{
			name: "wrong jsonrpc version",
			req:  JSONRPCRequest{JSONRPC: "1.0", Method: "test"},
		},
		{
			name: "empty method",
			req:  JSONRPCRequest{JSONRPC: "2.0", Method: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(&tt.req)
			if err == nil {
				t.Fatal("ValidateRequest() should return error")
				return
			}
			if err.Code != ErrCodeInvalidRequest {
				t.Errorf("error code = %d, want %d (InvalidRequest)",
					err.Code, ErrCodeInvalidRequest)
			}
		})
	}
}

func TestNewErrorResponse_FormatsCorrectly(t *testing.T) {
	t.Parallel()

	err := MethodNotFoundError("unknown/method")
	resp := ErrorResponse(1, err)

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("ID = %v, want 1", resp.ID)
	}
	if resp.Result != nil {
		t.Error("Result should be nil for error response")
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, ErrCodeMethodNotFound)
	}
}

func TestSuccessResponse_FormatsCorrectly(t *testing.T) {
	t.Parallel()

	result := map[string]string{"status": "ok"}
	resp := SuccessResponse("req-123", result)

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", resp.JSONRPC)
	}
	if resp.ID != "req-123" {
		t.Errorf("ID = %v, want req-123", resp.ID)
	}
	if resp.Error != nil {
		t.Error("Error should be nil for success response")
	}
	if resp.Result == nil {
		t.Error("Result should not be nil")
	}
}

func TestUnmarshalParams(t *testing.T) {
	t.Parallel()

	t.Run("valid params", func(t *testing.T) {
		params := json.RawMessage(`{"uri":"mockd://test"}`)
		result, err := UnmarshalParams[ResourceReadParams](params)

		if err != nil {
			t.Fatalf("UnmarshalParams() error = %v", err)
		}
		if result.URI != "mockd://test" {
			t.Errorf("URI = %s, want mockd://test", result.URI)
		}
	})

	t.Run("empty params returns zero value", func(t *testing.T) {
		var params json.RawMessage
		result, err := UnmarshalParams[ResourceReadParams](params)

		if err != nil {
			t.Fatalf("UnmarshalParams() error = %v", err)
		}
		if result.URI != "" {
			t.Errorf("URI = %s, want empty", result.URI)
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		params := json.RawMessage(`{invalid}`)
		_, err := UnmarshalParams[ResourceReadParams](params)

		if err == nil {
			t.Error("UnmarshalParams() should return error for invalid JSON")
		}
	})
}

func TestUnmarshalParamsRequired(t *testing.T) {
	t.Parallel()

	t.Run("empty params returns error", func(t *testing.T) {
		var params json.RawMessage
		_, err := UnmarshalParamsRequired[ResourceReadParams](params)

		if err == nil {
			t.Fatal("UnmarshalParamsRequired() should return error for empty params")
			return
		}
		if err.Code != ErrCodeInvalidParams {
			t.Errorf("error code = %d, want %d", err.Code, ErrCodeInvalidParams)
		}
	})
}

// =============================================================================
// TOOL RESULT HELPERS
// =============================================================================

func TestToolResultText(t *testing.T) {
	t.Parallel()

	result := ToolResultText("hello world")

	if result.IsError {
		t.Error("IsError should be false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Content[0].Type = %s, want text", result.Content[0].Type)
	}
	if result.Content[0].Text != "hello world" {
		t.Errorf("Content[0].Text = %s, want hello world", result.Content[0].Text)
	}
}

func TestToolResultError(t *testing.T) {
	t.Parallel()

	result := ToolResultError("something went wrong")

	if !result.IsError {
		t.Error("IsError should be true")
	}
	if result.Content[0].Text != "something went wrong" {
		t.Errorf("Content[0].Text = %s, want something went wrong",
			result.Content[0].Text)
	}
}

func TestToolResultErrorf(t *testing.T) {
	t.Parallel()

	result := ToolResultErrorf("failed to process %s: %d errors", "file.txt", 3)

	if !result.IsError {
		t.Error("IsError should be true")
	}
	expected := "failed to process file.txt: 3 errors"
	if result.Content[0].Text != expected {
		t.Errorf("Content[0].Text = %s, want %s",
			result.Content[0].Text, expected)
	}
}

func TestToolResultJSON(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"name":  "test",
		"count": float64(42),
	}
	result, err := ToolResultJSON(data)

	if err != nil {
		t.Fatalf("ToolResultJSON() error = %v", err)
	}
	if result.IsError {
		t.Error("IsError should be false")
	}

	// Parse the JSON to verify
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	if parsed["name"] != "test" {
		t.Errorf("parsed name = %v, want test", parsed["name"])
	}
}

// =============================================================================
// NOTIFICATION HELPERS
// =============================================================================

func TestNewNotification(t *testing.T) {
	t.Parallel()

	notif := NewNotification("test/method", map[string]string{"key": "value"})

	if notif.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", notif.JSONRPC)
	}
	if notif.Method != "test/method" {
		t.Errorf("Method = %s, want test/method", notif.Method)
	}
	if notif.Params == nil {
		t.Error("Params should not be nil")
	}
}

func TestResourceListChangedNotification(t *testing.T) {
	t.Parallel()

	notif := ResourceListChangedNotification()

	if notif.Method != "notifications/resources/list_changed" {
		t.Errorf("Method = %s, want notifications/resources/list_changed",
			notif.Method)
	}
}

func TestResourceUpdatedNotification(t *testing.T) {
	t.Parallel()

	notif := ResourceUpdatedNotification("mockd://endpoints")

	if notif.Method != "notifications/resources/updated" {
		t.Errorf("Method = %s, want notifications/resources/updated",
			notif.Method)
	}

	params, ok := notif.Params.(*ResourceUpdatedParams)
	if !ok {
		t.Fatal("Params should be *ResourceUpdatedParams")
	}
	if params.URI != "mockd://endpoints" {
		t.Errorf("URI = %s, want mockd://endpoints", params.URI)
	}
}

// =============================================================================
// ERROR HELPERS
// =============================================================================

func TestJSONRPCError_Error(t *testing.T) {
	t.Parallel()

	t.Run("without data", func(t *testing.T) {
		err := &JSONRPCError{Code: -32600, Message: "Invalid Request"}
		s := err.Error()

		if !strings.Contains(s, "Invalid Request") {
			t.Errorf("Error() = %s, should contain 'Invalid Request'", s)
		}
		if !strings.Contains(s, "-32600") {
			t.Errorf("Error() = %s, should contain '-32600'", s)
		}
	})

	t.Run("with data", func(t *testing.T) {
		err := &JSONRPCError{
			Code:    -32601,
			Message: "Method not found",
			Data:    map[string]string{"method": "unknown"},
		}
		s := err.Error()

		if !strings.Contains(s, "Method not found") {
			t.Errorf("Error() = %s, should contain 'Method not found'", s)
		}
	})
}

// =============================================================================
// CONFIG TESTS
// =============================================================================

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Port:           9091,
				Path:           "/mcp",
				MaxSessions:    100,
				SessionTimeout: time.Minute,
			},
			wantErr: false,
		},
		{
			name: "invalid port zero",
			cfg: Config{
				Port:           0,
				Path:           "/mcp",
				MaxSessions:    100,
				SessionTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			cfg: Config{
				Port:           70000,
				Path:           "/mcp",
				MaxSessions:    100,
				SessionTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "empty path",
			cfg: Config{
				Port:           9091,
				Path:           "",
				MaxSessions:    100,
				SessionTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "path without leading slash",
			cfg: Config{
				Port:           9091,
				Path:           "mcp",
				MaxSessions:    100,
				SessionTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "zero max sessions",
			cfg: Config{
				Port:           9091,
				Path:           "/mcp",
				MaxSessions:    0,
				SessionTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "session timeout too short",
			cfg: Config{
				Port:           9091,
				Path:           "/mcp",
				MaxSessions:    100,
				SessionTimeout: time.Millisecond,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Address(t *testing.T) {
	t.Parallel()

	t.Run("localhost only", func(t *testing.T) {
		cfg := &Config{Port: 9091, AllowRemote: false}
		addr := cfg.Address()
		if addr != "127.0.0.1:9091" {
			t.Errorf("Address() = %s, want 127.0.0.1:9091", addr)
		}
	})

	t.Run("allow remote", func(t *testing.T) {
		cfg := &Config{Port: 9091, AllowRemote: true}
		addr := cfg.Address()
		if addr != ":9091" {
			t.Errorf("Address() = %s, want :9091", addr)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Port != 9091 {
		t.Errorf("Port = %d, want 9091", cfg.Port)
	}
	if cfg.Path != "/mcp" {
		t.Errorf("Path = %s, want /mcp", cfg.Path)
	}
	if cfg.MaxSessions != 100 {
		t.Errorf("MaxSessions = %d, want 100", cfg.MaxSessions)
	}
	if cfg.SessionTimeout != 30*time.Minute {
		t.Errorf("SessionTimeout = %v, want 30m", cfg.SessionTimeout)
	}
}

// =============================================================================
// TOOL ARGUMENT HELPERS
// =============================================================================

func TestGetString(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"name": "test",
		"num":  42,
	}

	if got := getString(args, "name", "default"); got != "test" {
		t.Errorf("getString(name) = %s, want test", got)
	}
	if got := getString(args, "missing", "default"); got != "default" {
		t.Errorf("getString(missing) = %s, want default", got)
	}
	if got := getString(args, "num", "default"); got != "default" {
		t.Errorf("getString(num) = %s, want default (wrong type)", got)
	}
}

func TestGetInt(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"count":   float64(42), // JSON numbers are float64
		"int_val": 100,
		"str":     "not a number",
	}

	if got := getInt(args, "count", 0); got != 42 {
		t.Errorf("getInt(count) = %d, want 42", got)
	}
	if got := getInt(args, "int_val", 0); got != 100 {
		t.Errorf("getInt(int_val) = %d, want 100", got)
	}
	if got := getInt(args, "missing", 99); got != 99 {
		t.Errorf("getInt(missing) = %d, want 99", got)
	}
	if got := getInt(args, "str", 99); got != 99 {
		t.Errorf("getInt(str) = %d, want 99 (wrong type)", got)
	}
}

func TestGetBool(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"enabled":  true,
		"disabled": false,
		"str":      "true",
	}

	if got := getBool(args, "enabled", false); got != true {
		t.Errorf("getBool(enabled) = %v, want true", got)
	}
	if got := getBool(args, "disabled", true); got != false {
		t.Errorf("getBool(disabled) = %v, want false", got)
	}
	if got := getBool(args, "missing", true); got != true {
		t.Errorf("getBool(missing) = %v, want true", got)
	}
	if got := getBool(args, "str", true); got != true {
		t.Errorf("getBool(str) = %v, want true (wrong type)", got)
	}
}

func TestGetBoolPtr(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"enabled": true,
	}

	if got := getBoolPtr(args, "enabled"); got == nil || *got != true {
		t.Errorf("getBoolPtr(enabled) = %v, want true", got)
	}
	if got := getBoolPtr(args, "missing"); got != nil {
		t.Errorf("getBoolPtr(missing) = %v, want nil", got)
	}
}

func TestGetStringMap(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
			"Accept":       "text/plain",
		},
	}

	got := getStringMap(args, "headers")
	if got == nil {
		t.Fatal("getStringMap(headers) = nil, want map")
	}
	if got["Content-Type"] != "application/json" {
		t.Errorf("headers[Content-Type] = %s, want application/json",
			got["Content-Type"])
	}

	if got := getStringMap(args, "missing"); got != nil {
		t.Errorf("getStringMap(missing) = %v, want nil", got)
	}
}

func TestGetMap(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"data": map[string]interface{}{
			"key": "value",
			"num": 123,
		},
	}

	got := getMap(args, "data")
	if got == nil {
		t.Fatal("getMap(data) = nil, want map")
	}
	if got["key"] != "value" {
		t.Errorf("data[key] = %v, want value", got["key"])
	}

	if got := getMap(args, "missing"); got != nil {
		t.Errorf("getMap(missing) = %v, want nil", got)
	}
}

// =============================================================================
// SESSION STATE STRING
// =============================================================================

func TestSessionState_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state    SessionState
		expected string
	}{
		{SessionStateNew, "new"},
		{SessionStateInitialized, "initialized"},
		{SessionStateReady, "ready"},
		{SessionStateExpired, "expired"},
		{SessionState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// ADMIN ERROR HELPERS
// =============================================================================

func TestIsConnectionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("dial tcp 127.0.0.1:4290: connection refused"), true},
		{"no such host", errors.New("dial tcp: lookup badhost: no such host"), true},
		{"dial tcp generic", errors.New("dial tcp 10.0.0.1:4290: i/o timeout"), true},
		{"context deadline", errors.New("Post http://localhost:4290/health: context deadline exceeded"), true},
		{"network unreachable", errors.New("connect: network is unreachable"), true},
		{"404 not found", errors.New("404 Not Found"), false},
		{"API error", errors.New("mock not found: http_abc123"), false},
		{"empty error", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConnectionError(tt.err); got != tt.expected {
				t.Errorf("isConnectionError(%q) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestAdminError(t *testing.T) {
	t.Parallel()

	t.Run("connection error wraps with actionable message", func(t *testing.T) {
		err := errors.New("dial tcp 127.0.0.1:4290: connection refused")
		got := adminError(err, "http://localhost:4290")

		if !strings.Contains(got, "unreachable") {
			t.Errorf("adminError should mention unreachable, got: %s", got)
		}
		if !strings.Contains(got, "mockd serve") {
			t.Errorf("adminError should suggest 'mockd serve', got: %s", got)
		}
		if !strings.Contains(got, "http://localhost:4290") {
			t.Errorf("adminError should include the admin URL, got: %s", got)
		}
	})

	t.Run("non-connection error passes through", func(t *testing.T) {
		err := errors.New("mock not found: http_abc123")
		got := adminError(err, "http://localhost:4290")

		if got != "mock not found: http_abc123" {
			t.Errorf("adminError should pass through non-connection errors, got: %s", got)
		}
	})
}
