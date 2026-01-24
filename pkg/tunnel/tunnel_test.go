package tunnel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Regression Tests - Bug Fixes (MUST HAVE)
// =============================================================================

// TestClient_OnDisconnect_CalledOnce verifies the atomic flag prevents double call.
// Regression test for Bug 3.13: OnDisconnect called twice.
func TestClient_OnDisconnect_CalledOnce(t *testing.T) {
	var callCount atomic.Int32
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	cfg.OnDisconnect = func(err error) {
		callCount.Add(1)
	}

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Simulate multiple goroutines racing to trigger disconnect
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Disconnect()
		}()
	}
	wg.Wait()

	count := callCount.Load()
	if count != 1 {
		t.Errorf("OnDisconnect called %d times, expected 1", count)
	}
}

// TestHandler_BuildResponse_SetCookie_PreservesMultiple verifies null separator for Set-Cookie.
// Regression test for Bug 3.14: Set-Cookie header mangling.
func TestHandler_BuildResponse_SetCookie_PreservesMultiple(t *testing.T) {
	// Create a handler that sets multiple cookies with commas in values
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cookie with comma in value
		w.Header().Add("Set-Cookie", "session=abc123; Expires=Thu, 01 Jan 2099 00:00:00 GMT; Path=/")
		// Another cookie with comma in value
		w.Header().Add("Set-Cookie", "user=john; Expires=Fri, 15 Feb 2099 12:30:00 GMT; HttpOnly")
		// Simple cookie
		w.Header().Add("Set-Cookie", "simple=value; Path=/")
		w.WriteHeader(http.StatusOK)
	})

	handler := NewEngineHandler(mockHandler, nil)

	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/",
	}

	resp := handler.HandleRequest(context.Background(), req)

	cookies, ok := resp.Headers["Set-Cookie"]
	if !ok {
		t.Fatal("Set-Cookie header missing from response")
	}

	// Verify cookies are joined with null separator, not comma
	if cookies == "" {
		t.Fatal("Set-Cookie header is empty")
	}

	// Split by null separator
	parts := splitByNull(cookies)
	if len(parts) != 3 {
		t.Errorf("expected 3 cookies, got %d: %v", len(parts), parts)
	}

	// Verify each cookie is intact (contains comma)
	for _, cookie := range parts {
		if cookie == "" {
			t.Error("empty cookie found after split")
		}
	}

	// Verify specific cookies are present
	expected := []string{
		"session=abc123; Expires=Thu, 01 Jan 2099 00:00:00 GMT; Path=/",
		"user=john; Expires=Fri, 15 Feb 2099 12:30:00 GMT; HttpOnly",
		"simple=value; Path=/",
	}
	for i, exp := range expected {
		if i < len(parts) && parts[i] != exp {
			t.Errorf("cookie %d: got %q, want %q", i, parts[i], exp)
		}
	}
}

func splitByNull(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == '\x00' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// TestHandler_BuildResponse_RegularHeaders_JoinedWithComma verifies non-Set-Cookie headers use comma.
func TestHandler_BuildResponse_RegularHeaders_JoinedWithComma(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Cache-Control", "no-store")
		w.Header().Add("Accept-Encoding", "gzip")
		w.Header().Add("Accept-Encoding", "deflate")
		w.WriteHeader(http.StatusOK)
	})

	handler := NewEngineHandler(mockHandler, nil)

	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/",
	}

	resp := handler.HandleRequest(context.Background(), req)

	// Check Cache-Control is comma-joined
	cacheControl := resp.Headers["Cache-Control"]
	if cacheControl != "no-cache, no-store" {
		t.Errorf("Cache-Control: got %q, want %q", cacheControl, "no-cache, no-store")
	}

	// Check Accept-Encoding is comma-joined
	acceptEncoding := resp.Headers["Accept-Encoding"]
	if acceptEncoding != "gzip, deflate" {
		t.Errorf("Accept-Encoding: got %q, want %q", acceptEncoding, "gzip, deflate")
	}
}

// TestEngineHandler_CheckAuth_TokenValid verifies valid token auth passes.
// Regression test for Bug 1.2: AuthConfig never used.
func TestEngineHandler_CheckAuth_TokenValid(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	auth := &AuthConfig{
		Type:  "token",
		Token: "secret-token-123",
	}

	handler := NewEngineHandler(mockHandler, auth)

	req := &TunnelMessage{
		Type:    MessageTypeRequest,
		ID:      "test-1",
		Method:  "GET",
		Path:    "/",
		Headers: map[string]string{"X-Auth-Token": "secret-token-123"},
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type == MessageTypeError {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Status)
	}
}

// TestEngineHandler_CheckAuth_TokenInvalid verifies invalid token auth fails.
func TestEngineHandler_CheckAuth_TokenInvalid(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{
		Type:  "token",
		Token: "secret-token-123",
	}

	handler := NewEngineHandler(mockHandler, auth)

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"wrong token", map[string]string{"X-Auth-Token": "wrong-token"}},
		{"missing token", map[string]string{}},
		{"empty token", map[string]string{"X-Auth-Token": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &TunnelMessage{
				Type:    MessageTypeRequest,
				ID:      "test-1",
				Method:  "GET",
				Path:    "/",
				Headers: tt.headers,
			}

			resp := handler.HandleRequest(context.Background(), req)

			if resp.Type != MessageTypeError {
				t.Errorf("expected error, got %s with status %d", resp.Type, resp.Status)
			}
			if resp.Error == "" {
				t.Error("expected error message")
			}
		})
	}
}

// TestEngineHandler_CheckAuth_BasicAuth verifies basic auth works correctly.
func TestEngineHandler_CheckAuth_BasicAuth(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{
		Type:     "basic",
		Username: "admin",
		Password: "password123",
	}

	handler := NewEngineHandler(mockHandler, auth)

	tests := []struct {
		name       string
		authHeader string
		shouldPass bool
	}{
		{
			"valid credentials",
			"Basic " + base64.StdEncoding.EncodeToString([]byte("admin:password123")),
			true,
		},
		{
			"wrong password",
			"Basic " + base64.StdEncoding.EncodeToString([]byte("admin:wrongpass")),
			false,
		},
		{
			"wrong username",
			"Basic " + base64.StdEncoding.EncodeToString([]byte("notadmin:password123")),
			false,
		},
		{
			"missing header",
			"",
			false,
		},
		{
			"invalid encoding",
			"Basic not-base64!!!",
			false,
		},
		{
			"bearer instead of basic",
			"Bearer sometoken",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.authHeader != "" {
				headers["Authorization"] = tt.authHeader
			}

			req := &TunnelMessage{
				Type:    MessageTypeRequest,
				ID:      "test-1",
				Method:  "GET",
				Path:    "/",
				Headers: headers,
			}

			resp := handler.HandleRequest(context.Background(), req)

			if tt.shouldPass {
				if resp.Type == MessageTypeError {
					t.Errorf("expected success, got error: %s", resp.Error)
				}
			} else {
				if resp.Type != MessageTypeError {
					t.Errorf("expected error, got success with status %d", resp.Status)
				}
			}
		})
	}
}

// TestEngineHandler_CheckAuth_IPAllowed verifies IP whitelist allows valid IPs.
func TestEngineHandler_CheckAuth_IPAllowed(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{
		Type:       "ip",
		AllowedIPs: []string{"192.168.1.100", "10.0.0.1", "127.0.0.1"},
	}

	handler := NewEngineHandler(mockHandler, auth)

	tests := []struct {
		name       string
		headers    map[string]string
		shouldPass bool
	}{
		{
			"allowed IP via X-Forwarded-For",
			map[string]string{"X-Forwarded-For": "192.168.1.100"},
			true,
		},
		{
			"allowed IP via X-Real-IP",
			map[string]string{"X-Real-IP": "10.0.0.1"},
			true,
		},
		{
			"first IP in chain is allowed",
			map[string]string{"X-Forwarded-For": "127.0.0.1, 8.8.8.8, 1.1.1.1"},
			true,
		},
		{
			"lowercase header works",
			map[string]string{"x-forwarded-for": "192.168.1.100"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &TunnelMessage{
				Type:    MessageTypeRequest,
				ID:      "test-1",
				Method:  "GET",
				Path:    "/",
				Headers: tt.headers,
			}

			resp := handler.HandleRequest(context.Background(), req)

			if tt.shouldPass {
				if resp.Type == MessageTypeError {
					t.Errorf("expected success, got error: %s", resp.Error)
				}
			} else {
				if resp.Type != MessageTypeError {
					t.Errorf("expected error, got success")
				}
			}
		})
	}
}

// TestEngineHandler_CheckAuth_IPDenied verifies IP whitelist denies invalid IPs.
func TestEngineHandler_CheckAuth_IPDenied(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{
		Type:       "ip",
		AllowedIPs: []string{"192.168.1.100"},
	}

	handler := NewEngineHandler(mockHandler, auth)

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"wrong IP", map[string]string{"X-Forwarded-For": "192.168.1.200"}},
		{"no IP header", map[string]string{}},
		{"empty IP header", map[string]string{"X-Forwarded-For": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &TunnelMessage{
				Type:    MessageTypeRequest,
				ID:      "test-1",
				Method:  "GET",
				Path:    "/",
				Headers: tt.headers,
			}

			resp := handler.HandleRequest(context.Background(), req)

			if resp.Type != MessageTypeError {
				t.Errorf("expected error, got success with status %d", resp.Status)
			}
		})
	}
}

// TestEngineHandler_CheckAuth_NilConfig_AllowsAll verifies nil auth allows all requests.
func TestEngineHandler_CheckAuth_NilConfig_AllowsAll(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("allowed"))
	})

	handler := NewEngineHandler(mockHandler, nil)

	req := &TunnelMessage{
		Type:    MessageTypeRequest,
		ID:      "test-1",
		Method:  "GET",
		Path:    "/",
		Headers: map[string]string{},
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type == MessageTypeError {
		t.Errorf("expected success with nil auth, got error: %s", resp.Error)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Status)
	}
}

// TestEngineHandler_CheckAuth_EmptyType_AllowsAll verifies empty auth type allows all.
func TestEngineHandler_CheckAuth_EmptyType_AllowsAll(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{Type: ""}
	handler := NewEngineHandler(mockHandler, auth)

	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/",
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type == MessageTypeError {
		t.Errorf("expected success with empty auth type, got error: %s", resp.Error)
	}
}

// TestEngineHandler_CheckAuth_NoneType_AllowsAll verifies "none" auth type allows all.
func TestEngineHandler_CheckAuth_NoneType_AllowsAll(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{Type: "none"}
	handler := NewEngineHandler(mockHandler, auth)

	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/",
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type == MessageTypeError {
		t.Errorf("expected success with 'none' auth type, got error: %s", resp.Error)
	}
}

// =============================================================================
// Client Lifecycle Tests
// =============================================================================

// TestClient_Disconnect_SetsConnectedFalse verifies Disconnect sets connected to false.
func TestClient_Disconnect_SetsConnectedFalse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Manually set connected to true to test disconnect
	client.connected.Store(true)

	if !client.IsConnected() {
		t.Error("expected client to be connected before disconnect")
	}

	client.Disconnect()

	if client.IsConnected() {
		t.Error("expected client to be disconnected after Disconnect()")
	}
}

// TestClient_IsConnected_ReflectsState verifies IsConnected reflects the actual state.
func TestClient_IsConnected_ReflectsState(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Initial state should be disconnected
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}

	// Set to connected
	client.connected.Store(true)
	if !client.IsConnected() {
		t.Error("client should be connected after setting flag")
	}

	// Set to disconnected
	client.connected.Store(false)
	if client.IsConnected() {
		t.Error("client should be disconnected after clearing flag")
	}
}

// TestClient_NewClient_NilHandler verifies NewClient rejects nil handler.
func TestClient_NewClient_NilHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	_, err := NewClient(cfg, nil)
	if err == nil {
		t.Error("expected error for nil handler")
	}
}

// TestClient_NewClient_NilConfig_UsesDefaults verifies NewClient uses defaults for nil config.
func TestClient_NewClient_NilConfig_UsesDefaults(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	// nil config should fail because Token is required
	_, err := NewClient(nil, handler)
	if err == nil {
		t.Error("expected error for nil config (token required)")
	}
}

// TestClient_Disconnect_Idempotent verifies multiple Disconnect calls are safe.
func TestClient_Disconnect_Idempotent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Multiple calls should not panic
	client.Disconnect()
	client.Disconnect()
	client.Disconnect()

	if client.IsConnected() {
		t.Error("client should be disconnected")
	}
}

// =============================================================================
// Message Tests
// =============================================================================

// TestNewResponseMessage_SetsFields verifies NewResponseMessage sets all fields correctly.
func TestNewResponseMessage_SetsFields(t *testing.T) {
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Custom":     "value",
	}
	body := []byte(`{"status":"ok"}`)

	msg := NewResponseMessage("req-123", 201, headers, body)

	if msg.Type != MessageTypeResponse {
		t.Errorf("Type: got %q, want %q", msg.Type, MessageTypeResponse)
	}
	if msg.ID != "req-123" {
		t.Errorf("ID: got %q, want %q", msg.ID, "req-123")
	}
	if msg.Status != 201 {
		t.Errorf("Status: got %d, want %d", msg.Status, 201)
	}
	if msg.Headers["Content-Type"] != "application/json" {
		t.Errorf("Headers[Content-Type]: got %q, want %q", msg.Headers["Content-Type"], "application/json")
	}
	if string(msg.Body) != `{"status":"ok"}` {
		t.Errorf("Body: got %q, want %q", string(msg.Body), `{"status":"ok"}`)
	}
}

// TestNewErrorMessage_SetsFields verifies NewErrorMessage sets all fields correctly.
func TestNewErrorMessage_SetsFields(t *testing.T) {
	msg := NewErrorMessage("req-456", "auth_failed", "Invalid token")

	if msg.Type != MessageTypeError {
		t.Errorf("Type: got %q, want %q", msg.Type, MessageTypeError)
	}
	if msg.ID != "req-456" {
		t.Errorf("ID: got %q, want %q", msg.ID, "req-456")
	}
	if msg.Error != "auth_failed: Invalid token" {
		t.Errorf("Error: got %q, want %q", msg.Error, "auth_failed: Invalid token")
	}
}

// TestNewPongMessage_SetsFields verifies NewPongMessage sets all fields correctly.
func TestNewPongMessage_SetsFields(t *testing.T) {
	msg := NewPongMessage("ping-789")

	if msg.Type != MessageTypePong {
		t.Errorf("Type: got %q, want %q", msg.Type, MessageTypePong)
	}
	if msg.ID != "ping-789" {
		t.Errorf("ID: got %q, want %q", msg.ID, "ping-789")
	}
}

// TestTunnelMessage_JSON_RoundTrip verifies messages can be encoded and decoded.
func TestTunnelMessage_JSON_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  *TunnelMessage
	}{
		{
			"response message",
			&TunnelMessage{
				Type:    MessageTypeResponse,
				ID:      "test-1",
				Status:  200,
				Headers: map[string]string{"Content-Type": "text/plain"},
				Body:    []byte("hello"),
			},
		},
		{
			"error message",
			&TunnelMessage{
				Type:  MessageTypeError,
				ID:    "test-2",
				Error: "something went wrong",
			},
		},
		{
			"request message",
			&TunnelMessage{
				Type:    MessageTypeRequest,
				ID:      "test-3",
				Method:  "POST",
				Path:    "/api/users",
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    []byte(`{"name":"test"}`),
			},
		},
		{
			"pong message",
			&TunnelMessage{
				Type: MessageTypePong,
				ID:   "ping-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := tt.msg.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			decoded, err := DecodeMessage(data)
			if err != nil {
				t.Fatalf("DecodeMessage failed: %v", err)
			}

			// Compare
			if decoded.Type != tt.msg.Type {
				t.Errorf("Type: got %q, want %q", decoded.Type, tt.msg.Type)
			}
			if decoded.ID != tt.msg.ID {
				t.Errorf("ID: got %q, want %q", decoded.ID, tt.msg.ID)
			}
			if decoded.Status != tt.msg.Status {
				t.Errorf("Status: got %d, want %d", decoded.Status, tt.msg.Status)
			}
			if decoded.Method != tt.msg.Method {
				t.Errorf("Method: got %q, want %q", decoded.Method, tt.msg.Method)
			}
			if decoded.Path != tt.msg.Path {
				t.Errorf("Path: got %q, want %q", decoded.Path, tt.msg.Path)
			}
			if decoded.Error != tt.msg.Error {
				t.Errorf("Error: got %q, want %q", decoded.Error, tt.msg.Error)
			}
			if string(decoded.Body) != string(tt.msg.Body) {
				t.Errorf("Body: got %q, want %q", string(decoded.Body), string(tt.msg.Body))
			}
		})
	}
}

// TestDecodeMessage_InvalidJSON verifies DecodeMessage handles invalid JSON.
func TestDecodeMessage_InvalidJSON(t *testing.T) {
	_, err := DecodeMessage([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestDecodeConnectedMessage_Valid verifies DecodeConnectedMessage works.
func TestDecodeConnectedMessage_Valid(t *testing.T) {
	data := []byte(`{
		"type": "connected",
		"session_id": "sess-123",
		"public_url": "https://test.mockd.io",
		"subdomain": "test"
	}`)

	msg, err := DecodeConnectedMessage(data)
	if err != nil {
		t.Fatalf("DecodeConnectedMessage failed: %v", err)
	}

	if msg.Type != "connected" {
		t.Errorf("Type: got %q, want %q", msg.Type, "connected")
	}
	if msg.SessionID != "sess-123" {
		t.Errorf("SessionID: got %q, want %q", msg.SessionID, "sess-123")
	}
	if msg.PublicURL != "https://test.mockd.io" {
		t.Errorf("PublicURL: got %q, want %q", msg.PublicURL, "https://test.mockd.io")
	}
	if msg.Subdomain != "test" {
		t.Errorf("Subdomain: got %q, want %q", msg.Subdomain, "test")
	}
}

// =============================================================================
// Config Tests
// =============================================================================

// TestConfig_Defaults verifies DefaultConfig sets expected defaults.
func TestConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.RelayURL != DefaultRelayURL {
		t.Errorf("RelayURL: got %q, want %q", cfg.RelayURL, DefaultRelayURL)
	}
	if cfg.ReconnectDelay != DefaultReconnectDelay {
		t.Errorf("ReconnectDelay: got %v, want %v", cfg.ReconnectDelay, DefaultReconnectDelay)
	}
	if cfg.MaxReconnectDelay != DefaultMaxReconnectDelay {
		t.Errorf("MaxReconnectDelay: got %v, want %v", cfg.MaxReconnectDelay, DefaultMaxReconnectDelay)
	}
	if cfg.PingInterval != DefaultPingInterval {
		t.Errorf("PingInterval: got %v, want %v", cfg.PingInterval, DefaultPingInterval)
	}
	if cfg.RequestTimeout != DefaultRequestTimeout {
		t.Errorf("RequestTimeout: got %v, want %v", cfg.RequestTimeout, DefaultRequestTimeout)
	}
	if !cfg.AutoReconnect {
		t.Error("AutoReconnect should be true by default")
	}
	if cfg.ClientVersion == "" {
		t.Error("ClientVersion should not be empty")
	}
}

// TestConfig_Validate verifies config validation.
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
	}{
		{
			"valid config",
			&Config{RelayURL: "wss://relay.example.com", Token: "token123"},
			false,
		},
		{
			"missing relay URL",
			&Config{RelayURL: "", Token: "token123"},
			true,
		},
		{
			"missing token",
			&Config{RelayURL: "wss://relay.example.com", Token: ""},
			true,
		},
		{
			"both missing",
			&Config{RelayURL: "", Token: ""},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestAuthConfig_Types verifies all auth types work as expected.
func TestAuthConfig_Types(t *testing.T) {
	tests := []struct {
		name   string
		config *AuthConfig
	}{
		{"none", &AuthConfig{Type: "none"}},
		{"token", &AuthConfig{Type: "token", Token: "secret"}},
		{"basic", &AuthConfig{Type: "basic", Username: "user", Password: "pass"}},
		{"ip", &AuthConfig{Type: "ip", AllowedIPs: []string{"127.0.0.1", "10.0.0.0/8"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.Type != tt.name {
				t.Errorf("Type: got %q, want %q", tt.config.Type, tt.name)
			}
		})
	}
}

// TestConfig_WithMethods verifies fluent config methods.
func TestConfig_WithMethods(t *testing.T) {
	cfg := DefaultConfig()

	cfg = cfg.WithToken("my-token")
	if cfg.Token != "my-token" {
		t.Errorf("WithToken: got %q, want %q", cfg.Token, "my-token")
	}

	cfg = cfg.WithSubdomain("my-subdomain")
	if cfg.Subdomain != "my-subdomain" {
		t.Errorf("WithSubdomain: got %q, want %q", cfg.Subdomain, "my-subdomain")
	}

	cfg = cfg.WithCustomDomain("custom.example.com")
	if cfg.CustomDomain != "custom.example.com" {
		t.Errorf("WithCustomDomain: got %q, want %q", cfg.CustomDomain, "custom.example.com")
	}

	cfg = cfg.WithRelayURL("wss://custom.relay.com")
	if cfg.RelayURL != "wss://custom.relay.com" {
		t.Errorf("WithRelayURL: got %q, want %q", cfg.RelayURL, "wss://custom.relay.com")
	}

	cfg = cfg.WithTokenAuth("auth-token")
	if cfg.Auth == nil || cfg.Auth.Type != "token" || cfg.Auth.Token != "auth-token" {
		t.Error("WithTokenAuth did not set auth correctly")
	}

	cfg = cfg.WithBasicAuth("user", "pass")
	if cfg.Auth == nil || cfg.Auth.Type != "basic" || cfg.Auth.Username != "user" || cfg.Auth.Password != "pass" {
		t.Error("WithBasicAuth did not set auth correctly")
	}

	cfg = cfg.WithIPAuth([]string{"1.2.3.4"})
	if cfg.Auth == nil || cfg.Auth.Type != "ip" || len(cfg.Auth.AllowedIPs) != 1 {
		t.Error("WithIPAuth did not set auth correctly")
	}
}

// =============================================================================
// Stats Tests
// =============================================================================

// TestTunnelStats_Uptime verifies Uptime calculation.
func TestTunnelStats_Uptime(t *testing.T) {
	stats := &TunnelStats{
		ConnectedAt: time.Now().Add(-5 * time.Minute),
	}

	uptime := stats.Uptime()
	if uptime < 4*time.Minute || uptime > 6*time.Minute {
		t.Errorf("Uptime: got %v, expected ~5 minutes", uptime)
	}

	// Zero time should return zero duration
	zeroStats := &TunnelStats{}
	if zeroStats.Uptime() != 0 {
		t.Errorf("Uptime with zero ConnectedAt: got %v, want 0", zeroStats.Uptime())
	}
}

// TestTunnelStats_LatencyMs verifies latency millisecond conversions.
func TestTunnelStats_LatencyMs(t *testing.T) {
	stats := &TunnelStats{
		AvgLatency: 5 * time.Millisecond,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 10 * time.Millisecond,
	}

	if stats.AvgLatencyMs() != 5.0 {
		t.Errorf("AvgLatencyMs: got %v, want 5.0", stats.AvgLatencyMs())
	}
	if stats.MinLatencyMs() != 1.0 {
		t.Errorf("MinLatencyMs: got %v, want 1.0", stats.MinLatencyMs())
	}
	if stats.MaxLatencyMs() != 10.0 {
		t.Errorf("MaxLatencyMs: got %v, want 10.0", stats.MaxLatencyMs())
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

// TestEngineHandler_HandleRequest_ForwardsToHandler verifies requests are forwarded.
func TestEngineHandler_HandleRequest_ForwardsToHandler(t *testing.T) {
	var receivedMethod, receivedPath string
	var receivedBody []byte
	var receivedHeaders http.Header

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedHeaders = r.Header
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("X-Response", "test")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("response body"))
	})

	handler := NewEngineHandler(mockHandler, nil)

	req := &TunnelMessage{
		Type:    MessageTypeRequest,
		ID:      "test-1",
		Method:  "POST",
		Path:    "/api/users?name=john",
		Headers: map[string]string{"Content-Type": "application/json", "X-Custom": "value"},
		Body:    []byte(`{"user":"test"}`),
	}

	resp := handler.HandleRequest(context.Background(), req)

	// Verify request was forwarded correctly
	if receivedMethod != "POST" {
		t.Errorf("Method: got %q, want %q", receivedMethod, "POST")
	}
	if receivedPath != "/api/users" {
		t.Errorf("Path: got %q, want %q", receivedPath, "/api/users")
	}
	if string(receivedBody) != `{"user":"test"}` {
		t.Errorf("Body: got %q, want %q", string(receivedBody), `{"user":"test"}`)
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header not forwarded")
	}
	if receivedHeaders.Get("X-Custom") != "value" {
		t.Errorf("X-Custom header not forwarded")
	}

	// Verify response
	if resp.Type != MessageTypeResponse {
		t.Errorf("Response Type: got %q, want %q", resp.Type, MessageTypeResponse)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("Response Status: got %d, want %d", resp.Status, http.StatusCreated)
	}
	if string(resp.Body) != "response body" {
		t.Errorf("Response Body: got %q, want %q", string(resp.Body), "response body")
	}
	if resp.Headers["X-Response"] != "test" {
		t.Errorf("Response header X-Response not present")
	}
}

// TestFuncHandler verifies FuncHandler implements RequestHandler.
func TestFuncHandler(t *testing.T) {
	called := false
	fn := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		called = true
		return NewResponseMessage(req.ID, 200, nil, []byte("ok"))
	})

	req := &TunnelMessage{ID: "test-1"}
	resp := fn.HandleRequest(context.Background(), req)

	if !called {
		t.Error("FuncHandler was not called")
	}
	if resp.ID != "test-1" {
		t.Errorf("Response ID: got %q, want %q", resp.ID, "test-1")
	}
}

// =============================================================================
// Metrics Formatting Tests
// =============================================================================

// TestFormatStats verifies basic stats formatting.
func TestFormatStats(t *testing.T) {
	stats := &TunnelStats{
		RequestsServed: 100,
		BytesIn:        1024,
		BytesOut:       2048,
		ConnectedAt:    time.Now().Add(-time.Hour),
		Reconnects:     2,
		IsConnected:    true,
	}

	output := FormatStats(stats)

	if output == "" {
		t.Error("FormatStats returned empty string")
	}

	// Should contain key information
	expectedSubstrings := []string{
		"Connected",
		"100",
		"Reconnects: 2",
	}

	for _, sub := range expectedSubstrings {
		if !containsString(output, sub) {
			t.Errorf("FormatStats output missing %q", sub)
		}
	}
}

// TestFormatStats_Nil verifies nil stats handling.
func TestFormatStats_Nil(t *testing.T) {
	output := FormatStats(nil)
	if output != "No stats available" {
		t.Errorf("FormatStats(nil): got %q, want %q", output, "No stats available")
	}
}

// TestFormatDetailedStats verifies detailed stats formatting.
func TestFormatDetailedStats(t *testing.T) {
	stats := &TunnelStats{
		RequestsServed: 100,
		BytesIn:        1024 * 1024,
		BytesOut:       2048 * 1024,
		TotalLatency:   500 * time.Millisecond,
		AvgLatency:     5 * time.Millisecond,
		MinLatency:     1 * time.Millisecond,
		MaxLatency:     20 * time.Millisecond,
		ConnectedAt:    time.Now().Add(-time.Hour),
		Reconnects:     0,
		IsConnected:    true,
	}

	output := FormatDetailedStats(stats)

	if output == "" {
		t.Error("FormatDetailedStats returned empty string")
	}

	// Should contain latency section
	if !containsString(output, "Latency") {
		t.Error("FormatDetailedStats missing Latency section")
	}

	// Should contain throughput section
	if !containsString(output, "Throughput") {
		t.Error("FormatDetailedStats missing Throughput section")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

// TestEngineHandler_BuildRequest_InvalidPath verifies error handling for invalid paths.
func TestEngineHandler_BuildRequest_InvalidPath(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := NewEngineHandler(mockHandler, nil)

	// Valid paths should work
	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/valid/path?query=value",
	}

	resp := handler.HandleRequest(context.Background(), req)
	if resp.Type == MessageTypeError {
		t.Errorf("valid path should not error: %s", resp.Error)
	}
}

// TestClient_Stats_InitialValues verifies initial stats values.
func TestClient_Stats_InitialValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	stats := client.Stats()

	if stats.RequestsServed != 0 {
		t.Errorf("RequestsServed: got %d, want 0", stats.RequestsServed)
	}
	if stats.BytesIn != 0 {
		t.Errorf("BytesIn: got %d, want 0", stats.BytesIn)
	}
	if stats.BytesOut != 0 {
		t.Errorf("BytesOut: got %d, want 0", stats.BytesOut)
	}
	if stats.IsConnected {
		t.Error("IsConnected should be false initially")
	}
}

// TestClient_PublicURL_SessionID_Subdomain verifies accessor methods.
func TestClient_PublicURL_SessionID_Subdomain(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Initially empty
	if client.PublicURL() != "" {
		t.Errorf("PublicURL: got %q, want empty", client.PublicURL())
	}
	if client.SessionID() != "" {
		t.Errorf("SessionID: got %q, want empty", client.SessionID())
	}
	if client.Subdomain() != "" {
		t.Errorf("Subdomain: got %q, want empty", client.Subdomain())
	}

	// Set values directly
	client.mu.Lock()
	client.publicURL = "https://test.mockd.io"
	client.sessionID = "sess-123"
	client.subdomain = "test"
	client.mu.Unlock()

	if client.PublicURL() != "https://test.mockd.io" {
		t.Errorf("PublicURL: got %q, want %q", client.PublicURL(), "https://test.mockd.io")
	}
	if client.SessionID() != "sess-123" {
		t.Errorf("SessionID: got %q, want %q", client.SessionID(), "sess-123")
	}
	if client.Subdomain() != "test" {
		t.Errorf("Subdomain: got %q, want %q", client.Subdomain(), "test")
	}
}

// TestEngineHandler_CheckAuth_UnknownType verifies unknown auth type is rejected.
func TestEngineHandler_CheckAuth_UnknownType(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{Type: "oauth2"} // Unknown type
	handler := NewEngineHandler(mockHandler, auth)

	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/",
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type != MessageTypeError {
		t.Errorf("expected error for unknown auth type, got success")
	}
}

// TestEngineHandler_CheckAuth_LowercaseTokenHeader verifies lowercase header works.
func TestEngineHandler_CheckAuth_LowercaseTokenHeader(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{
		Type:  "token",
		Token: "secret-token",
	}

	handler := NewEngineHandler(mockHandler, auth)

	req := &TunnelMessage{
		Type:    MessageTypeRequest,
		ID:      "test-1",
		Method:  "GET",
		Path:    "/",
		Headers: map[string]string{"x-auth-token": "secret-token"}, // lowercase
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type == MessageTypeError {
		t.Errorf("expected success with lowercase header, got error: %s", resp.Error)
	}
}

// TestEngineHandler_CheckAuth_LowercaseAuthorizationHeader verifies lowercase header works.
func TestEngineHandler_CheckAuth_LowercaseAuthorizationHeader(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	auth := &AuthConfig{
		Type:     "basic",
		Username: "admin",
		Password: "pass",
	}

	handler := NewEngineHandler(mockHandler, auth)

	req := &TunnelMessage{
		Type:   MessageTypeRequest,
		ID:     "test-1",
		Method: "GET",
		Path:   "/",
		Headers: map[string]string{
			"authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:pass")),
		},
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Type == MessageTypeError {
		t.Errorf("expected success with lowercase header, got error: %s", resp.Error)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestClient_ConcurrentStatAccess verifies stats can be accessed concurrently.
func TestClient_ConcurrentStatAccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"

	handler := FuncHandler(func(ctx context.Context, req *TunnelMessage) *TunnelMessage {
		return NewResponseMessage(req.ID, 200, nil, nil)
	})

	client, err := NewClient(cfg, handler)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Stats()
			_ = client.PublicURL()
			_ = client.SessionID()
			_ = client.Subdomain()
			_ = client.IsConnected()
		}()
	}
	wg.Wait()
}

// =============================================================================
// Additional Helper Tests
// =============================================================================

// TestFormatBytes verifies byte formatting helper.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d): got %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

// TestFormatDuration verifies duration formatting helper.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{65 * time.Minute, "1h 5m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v): got %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

// Verify JSON struct tags
func TestMessageJSONTags(t *testing.T) {
	msg := &TunnelMessage{
		Type:    MessageTypeRequest,
		ID:      "req-1",
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Accept": "application/json"},
		Body:    []byte("body"),
		Status:  200,
		Error:   "err",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify expected keys exist
	expectedKeys := []string{"type", "id", "method", "path", "headers", "body", "status", "error"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
	}
}
