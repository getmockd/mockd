package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Regression Tests for Bug 3.6: Localhost Auth Bypass Variants
// Tests that isLocalhost correctly identifies localhost connections
// ============================================================================

func TestIsLocalhost_127_0_0_1_ReturnsTrue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	assert.True(t, isLocalhost(req), "127.0.0.1 should be identified as localhost")
}

func TestIsLocalhost_IPv6Loopback_ReturnsTrue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:12345"

	assert.True(t, isLocalhost(req), "::1 (IPv6 loopback) should be identified as localhost")
}

func TestIsLocalhost_127_0_0_2_ReturnsTrue(t *testing.T) {
	// The entire 127.0.0.0/8 range is loopback
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.2:12345"

	assert.True(t, isLocalhost(req), "127.0.0.2 (full loopback range) should be identified as localhost")
}

func TestIsLocalhost_FullLoopbackRange(t *testing.T) {
	// Table-driven test for various loopback addresses
	tests := []struct {
		name       string
		remoteAddr string
		expected   bool
	}{
		{"127.0.0.1 standard", "127.0.0.1:8080", true},
		{"127.0.0.2", "127.0.0.2:8080", true},
		{"127.0.1.1", "127.0.1.1:8080", true},
		{"127.255.255.254", "127.255.255.254:8080", true},
		{"::1 IPv6", "[::1]:8080", true},
		{"127.0.0.1 no port", "127.0.0.1", true},
		{"::1 no port", "::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			assert.Equal(t, tt.expected, isLocalhost(req))
		})
	}
}

func TestIsLocalhost_ExternalIP_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
	}{
		{"external IPv4", "192.168.1.100:12345"},
		{"public IPv4", "8.8.8.8:12345"},
		{"external IPv6", "[2001:db8::1]:12345"},
		{"private network", "10.0.0.1:12345"},
		{"link-local", "169.254.1.1:12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			assert.False(t, isLocalhost(req), "%s should NOT be identified as localhost", tt.remoteAddr)
		})
	}
}

func TestIsLocalhost_EmptyHost_ReturnsFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ""

	assert.False(t, isLocalhost(req), "empty RemoteAddr should return false")
}

func TestIsLocalhost_InvalidIP_ReturnsFalse(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
	}{
		{"invalid format", "not-an-ip:8080"},
		{"partial IP", "127.0.0:8080"},
		{"letters in IP", "127.a.0.1:8080"},
		{"out of range", "256.0.0.1:8080"},
		{"empty after split", ":8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			assert.False(t, isLocalhost(req), "%s should return false", tt.remoteAddr)
		})
	}
}

// ============================================================================
// Regression Tests for Bug 3.6: Localhost Bypass - Opt-in Behavior
// ============================================================================

func TestLocalhostBypass_Disabled_RequiresAuth(t *testing.T) {
	// Create API with localhost bypass DISABLED (default)
	api := NewAdminAPI(0,
		WithAPIKey("test-api-key"),
		WithAllowLocalhostBypass(false),
	)
	defer api.Stop()

	// Make request from localhost WITHOUT auth header
	// Note: /health is exempt from auth, so we use /status instead
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	// Serve through the middleware chain
	api.httpServer.Handler.ServeHTTP(rec, req)

	// Should require authentication (401 Unauthorized)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"localhost requests should require auth when bypass is disabled")
}

func TestLocalhostBypass_Enabled_AllowsLocalhost(t *testing.T) {
	// Create API with localhost bypass ENABLED
	api := NewAdminAPI(0,
		WithAPIKey("test-api-key"),
		WithAPIKeyAllowLocalhost(true),
	)
	defer api.Stop()

	// Make request from localhost WITHOUT auth header
	// Using /status which requires auth (unlike /health which is exempt)
	// Note: /status returns 503 without engine, but that's OK - we just need to verify
	// auth was bypassed (not 401)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	// Serve through the middleware chain
	api.httpServer.Handler.ServeHTTP(rec, req)

	// Should not be 401 (auth bypassed) - may be 503 (no engine) or 200
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
		"localhost requests should bypass auth when enabled")
}

func TestLocalhostBypass_Enabled_ExternalStillRequiresAuth(t *testing.T) {
	// Create API with localhost bypass ENABLED
	api := NewAdminAPI(0,
		WithAPIKey("test-api-key"),
		WithAPIKeyAllowLocalhost(true),
	)
	defer api.Stop()

	// Make request from EXTERNAL IP without auth
	// Using /status which requires auth (unlike /health which is exempt)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	api.httpServer.Handler.ServeHTTP(rec, req)

	// External IPs should still require auth
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"external requests should require auth even when localhost bypass is enabled")
}

// ============================================================================
// Regression Tests for Bug 2.8: Bulk Create Doesn't Check Duplicate IDs
// ============================================================================

func TestBulkCreate_DuplicateIDs_ReturnsError(t *testing.T) {
	// Create temp directory for test isolation
	tmpDir := t.TempDir()

	api := NewAdminAPI(0, WithDataDir(tmpDir))
	defer api.Stop()

	// Request body with duplicate IDs
	mocks := []*mock.Mock{
		{ID: "duplicate-id", Name: "Mock 1", Type: mock.MockTypeHTTP},
		{ID: "unique-id", Name: "Mock 2", Type: mock.MockTypeHTTP},
		{ID: "duplicate-id", Name: "Mock 3 - Duplicate", Type: mock.MockTypeHTTP},
	}
	body, _ := json.Marshal(mocks)

	req := httptest.NewRequest("POST", "/mocks/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleBulkCreateUnifiedMocks(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"bulk create with duplicate IDs should return 400")

	var resp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, "duplicate_id", resp["error"],
		"error code should be duplicate_id")
	assert.Contains(t, resp["message"].(string), "duplicate-id",
		"message should mention the duplicate ID")
}

func TestBulkCreate_UniqueIDs_Succeeds(t *testing.T) {
	tmpDir := t.TempDir()

	api := NewAdminAPI(0, WithDataDir(tmpDir))
	defer api.Stop()

	// Request body with unique IDs
	mocks := []*mock.Mock{
		{ID: "mock-1", Name: "Mock 1", Type: mock.MockTypeHTTP},
		{ID: "mock-2", Name: "Mock 2", Type: mock.MockTypeHTTP},
		{ID: "mock-3", Name: "Mock 3", Type: mock.MockTypeHTTP},
	}
	body, _ := json.Marshal(mocks)

	req := httptest.NewRequest("POST", "/mocks/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleBulkCreateUnifiedMocks(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code,
		"bulk create with unique IDs should succeed")

	var resp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, float64(3), resp["created"],
		"should report 3 mocks created")
}

func TestBulkCreate_PortConflictWithinBatch_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	api := NewAdminAPI(0, WithDataDir(tmpDir))
	defer api.Stop()

	// Two MQTT mocks using the same port in the same batch
	mocks := []*mock.Mock{
		{
			ID:          "mqtt-1",
			Name:        "MQTT Mock 1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		},
		{
			ID:          "mqtt-2",
			Name:        "MQTT Mock 2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883}, // Same port - conflict
		},
	}
	body, _ := json.Marshal(mocks)

	req := httptest.NewRequest("POST", "/mocks/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleBulkCreateUnifiedMocks(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code,
		"bulk create with port conflicts should return 409")

	var resp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, "port_conflict", resp["error"])
	conflicts := resp["conflicts"].([]interface{})
	assert.Len(t, conflicts, 1, "should have one conflict")
}

// ============================================================================
// Regression Tests for Bug 2.9: Incomplete Rollback on Engine Failure
// ============================================================================

// mockFailingEngineServer creates a test server that fails on certain operations.
type mockFailingEngineServer struct {
	*httptest.Server
	mocks       map[string]*config.MockConfiguration
	failOnMock  string // ID of mock to fail on
	failMessage string
}

func newMockFailingEngineServer(failOnMock, failMessage string) *mockFailingEngineServer {
	mes := &mockFailingEngineServer{
		mocks:       make(map[string]*config.MockConfiguration),
		failOnMock:  failOnMock,
		failMessage: failMessage,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(engineclient.StatusResponse{
			ID:     "test-engine",
			Status: "running",
		})
	})

	mux.HandleFunc("GET /mocks", func(w http.ResponseWriter, r *http.Request) {
		mocks := make([]*config.MockConfiguration, 0, len(mes.mocks))
		for _, m := range mes.mocks {
			mocks = append(mocks, m)
		}
		json.NewEncoder(w).Encode(engineclient.MockListResponse{
			Mocks: mocks,
			Count: len(mocks),
		})
	})

	mux.HandleFunc("GET /mocks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, ok := mes.mocks[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "not_found"})
			return
		}
		json.NewEncoder(w).Encode(m)
	})

	mux.HandleFunc("POST /mocks", func(w http.ResponseWriter, r *http.Request) {
		var m config.MockConfiguration
		json.NewDecoder(r.Body).Decode(&m)

		if m.ID == mes.failOnMock {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "engine_error",
				Message: mes.failMessage,
			})
			return
		}

		mes.mocks[m.ID] = &m
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&m)
	})

	mux.HandleFunc("DELETE /mocks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		delete(mes.mocks, id)
		w.WriteHeader(http.StatusNoContent)
	})

	mes.Server = httptest.NewServer(mux)
	return mes
}

func (mes *mockFailingEngineServer) client() *engineclient.Client {
	return engineclient.New(mes.URL)
}

func TestCreateMock_EngineFailure_RollsBack(t *testing.T) {
	tmpDir := t.TempDir()

	// Engine that fails on creating this specific mock
	server := newMockFailingEngineServer("will-fail", "port 8080 is already in use")
	defer server.Close()

	api := NewAdminAPI(0,
		WithDataDir(tmpDir),
		WithLocalEngineClient(server.client()),
	)
	defer api.Stop()

	// Wait for store to initialize
	time.Sleep(50 * time.Millisecond)

	mockData := &mock.Mock{
		ID:   "will-fail",
		Name: "Mock That Fails",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/test"},
			Response: &mock.HTTPResponse{StatusCode: 200},
		},
	}
	body, _ := json.Marshal(mockData)

	req := httptest.NewRequest("POST", "/mocks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleCreateUnifiedMock(rec, req)

	// Should return error (port conflict from engine)
	assert.Equal(t, http.StatusConflict, rec.Code,
		"should return 409 on port error from engine")

	// Verify mock was NOT persisted in the store (rollback)
	mockStore := api.getMockStore()
	require.NotNil(t, mockStore)

	_, err := mockStore.Get(context.Background(), "will-fail")
	assert.Equal(t, store.ErrNotFound, err,
		"mock should be rolled back from store after engine failure")
}

// ============================================================================
// Regression Tests for Bug 1.1 & Bug 3.7: Path Traversal in CA Path
// ============================================================================

func TestProxyHandler_CAPath_RejectsTraversal(t *testing.T) {
	pm := NewProxyManager()

	tests := []struct {
		name   string
		caPath string
	}{
		{"parent directory", "../etc/certs"},
		{"deep traversal", "../../etc/certs"},
		{"hidden in path", "certs/../../../etc"},
		{"windows style", "..\\etc\\certs"},
		{"mixed slashes", "../etc/..\\certs"},
		{"multiple dots", "certs/.../.../etc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(ProxyStartRequest{
				Port:   8888,
				Mode:   "record",
				CAPath: tt.caPath,
			})

			req := httptest.NewRequest("POST", "/proxy/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			pm.handleProxyStart(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code,
				"path traversal attempt with '%s' should be rejected", tt.caPath)

			var resp ErrorResponse
			json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.Equal(t, "invalid_path", resp.Error,
				"error code should be invalid_path")
		})
	}
}

func TestProxyHandler_CAPath_RejectsAbsolute(t *testing.T) {
	pm := NewProxyManager()

	tests := []struct {
		name   string
		caPath string
	}{
		{"unix absolute", "/etc/ssl/certs"},
		{"unix root", "/tmp/ca"},
		{"unix home", "/home/user/.ssl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(ProxyStartRequest{
				Port:   8888,
				Mode:   "record",
				CAPath: tt.caPath,
			})

			req := httptest.NewRequest("POST", "/proxy/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			pm.handleProxyStart(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code,
				"absolute path '%s' should be rejected", tt.caPath)

			var resp ErrorResponse
			json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.Equal(t, "invalid_path", resp.Error)
		})
	}
}

func TestProxyHandler_CAPath_AcceptsValidRelative(t *testing.T) {
	pm := NewProxyManager()

	// Create a valid temp directory for CA files
	tmpDir := t.TempDir()
	caDir := filepath.Join(tmpDir, "certs")
	os.MkdirAll(caDir, 0755)

	// Use relative path from temp dir
	body, _ := json.Marshal(ProxyStartRequest{
		Port:   0, // Auto port
		Mode:   "record",
		CAPath: "certs",
	})

	req := httptest.NewRequest("POST", "/proxy/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Change to temp dir so relative path works
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	pm.handleProxyStart(rec, req)

	// Should either succeed or fail for reasons other than invalid_path
	if rec.Code == http.StatusBadRequest {
		var resp ErrorResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.NotEqual(t, "invalid_path", resp.Error,
			"valid relative path should not be rejected as invalid_path")
	}
}

// ============================================================================
// Regression Test for Bug 3.7: Missing Content-Type before WriteHeader
// ============================================================================

func TestStatefulHandler_GetResource_SetsContentType(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAdminAPI(0, WithLocalEngineClient(server.client()))
	defer api.Stop()

	req := httptest.NewRequest("GET", "/state/resources/users", nil)
	req.SetPathValue("name", "users")
	rec := httptest.NewRecorder()

	api.handleGetStateResource(rec, req)

	// The Content-Type header should be set BEFORE WriteHeader is called
	contentType := rec.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType,
		"Content-Type should be set for GET /state/resources/{name}")
}

func TestStatefulHandler_ClearResource_SetsContentType(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAdminAPI(0, WithLocalEngineClient(server.client()))
	defer api.Stop()

	req := httptest.NewRequest("DELETE", "/state/resources/users/clear", nil)
	req.SetPathValue("name", "users")
	rec := httptest.NewRecorder()

	api.handleClearStateResource(rec, req)

	// The Content-Type header should be set BEFORE WriteHeader is called
	contentType := rec.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType,
		"Content-Type should be set for DELETE /state/resources/{name}/clear")
}

// ============================================================================
// Handler Tests: Health Endpoint
// ============================================================================

func TestHealthHandler_ReturnsOK(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	api.handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp HealthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

func TestHealthHandler_ReturnsUptime(t *testing.T) {
	api := NewAdminAPI(0)
	api.startTime = time.Now().Add(-10 * time.Second) // Simulate 10 seconds uptime
	defer api.Stop()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	api.handleHealth(rec, req)

	var resp HealthResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	assert.GreaterOrEqual(t, resp.Uptime, 10,
		"uptime should be at least 10 seconds")
}

// ============================================================================
// Token Tests
// ============================================================================

func TestGenerateToken_ReturnsNonEmpty(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	token, err := api.GenerateRegistrationToken()

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	// Token is 32 hex characters (16 bytes) - generateRandomHex(32) generates 32/2=16 bytes
	assert.GreaterOrEqual(t, len(token), 32, "token should be at least 32 hex characters")
}

func TestGenerateToken_ReturnsUnique(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := api.GenerateRegistrationToken()
		require.NoError(t, err)
		assert.False(t, tokens[token], "tokens should be unique")
		tokens[token] = true
	}
}

func TestValidateToken_ValidToken_ReturnsTrue(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	token, err := api.GenerateRegistrationToken()
	require.NoError(t, err)

	// Token should be valid immediately after generation
	isValid := api.ValidateRegistrationToken(token)
	assert.True(t, isValid, "freshly generated token should be valid")
}

func TestValidateToken_InvalidToken_ReturnsFalse(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	// Try to validate a token that was never generated
	isValid := api.ValidateRegistrationToken("invalid-token-12345678901234567890")
	assert.False(t, isValid, "non-existent token should be invalid")
}

func TestValidateToken_ExpiredToken_ReturnsFalse(t *testing.T) {
	// Create API with very short token expiration
	api := NewAdminAPI(0,
		WithRegistrationTokenExpiration(50*time.Millisecond),
	)
	defer api.Stop()

	token, err := api.GenerateRegistrationToken()
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	isValid := api.ValidateRegistrationToken(token)
	assert.False(t, isValid, "expired token should be invalid")
}

func TestValidateToken_ConsumedOnUse(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	token, err := api.GenerateRegistrationToken()
	require.NoError(t, err)

	// First use should succeed
	isValid1 := api.ValidateRegistrationToken(token)
	assert.True(t, isValid1, "first use should succeed")

	// Second use should fail (token consumed)
	isValid2 := api.ValidateRegistrationToken(token)
	assert.False(t, isValid2, "second use should fail (token is one-time use)")
}

// ============================================================================
// Engine Token Tests
// ============================================================================

func TestEngineToken_ValidToken_ReturnsTrue(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	engineID := "test-engine-1"
	token, err := api.generateEngineToken(engineID)
	require.NoError(t, err)

	isValid := api.ValidateEngineToken(engineID, token)
	assert.True(t, isValid, "valid engine token should validate")
}

func TestEngineToken_WrongEngineID_ReturnsFalse(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	engineID := "test-engine-1"
	token, err := api.generateEngineToken(engineID)
	require.NoError(t, err)

	// Try to use token with different engine ID
	isValid := api.ValidateEngineToken("different-engine", token)
	assert.False(t, isValid, "token should not validate for different engine ID")
}

func TestEngineToken_WrongToken_ReturnsFalse(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	engineID := "test-engine-1"
	_, err := api.generateEngineToken(engineID)
	require.NoError(t, err)

	// Try with wrong token value
	isValid := api.ValidateEngineToken(engineID, "wrong-token")
	assert.False(t, isValid, "wrong token should not validate")
}

func TestEngineToken_NonExistentEngine_ReturnsFalse(t *testing.T) {
	api := NewAdminAPI(0)
	defer api.Stop()

	isValid := api.ValidateEngineToken("non-existent", "any-token")
	assert.False(t, isValid, "non-existent engine should not validate")
}

// ============================================================================
// Middleware Tests
// ============================================================================

func TestSecurityHeaders_AreSet(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeadersMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for preflight")
	})

	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}
	wrapped := NewCORSMiddlewareWithConfig(handler, config)

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	config := CORSConfig{
		AllowedOrigins: []string{"https://allowed.com"},
	}
	wrapped := NewCORSMiddlewareWithConfig(handler, config)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Request should still be processed, but no CORS headers
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"disallowed origin should not get CORS headers")
}

// ============================================================================
// Rate Limiter Tests
// ============================================================================

func TestRateLimiter_AllowsBurstRequests(t *testing.T) {
	rl := NewRateLimiter(100, 10) // 100 req/s, burst of 10
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// Burst of 10 requests should all succeed
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code,
			"request %d in burst should succeed", i+1)
	}
}

// ============================================================================
// getBearerToken Helper Tests
// ============================================================================

func TestGetBearerToken_ValidHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer my-secret-token")

	token := getBearerToken(req)
	assert.Equal(t, "my-secret-token", token)
}

func TestGetBearerToken_MissingHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	token := getBearerToken(req)
	assert.Empty(t, token)
}

func TestGetBearerToken_WrongScheme(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	token := getBearerToken(req)
	assert.Empty(t, token, "Basic auth should not extract as bearer token")
}

func TestGetBearerToken_EmptyValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")

	token := getBearerToken(req)
	assert.Empty(t, token)
}
