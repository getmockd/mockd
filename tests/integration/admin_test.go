package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

func boolPtr(b bool) *bool { return &b }

// setupAdminTest creates a server with admin API for testing
func setupAdminTest(t *testing.T) (*engine.Server, *admin.API, int, int, func()) {
	httpPort := getFreePort()
	adminPort := getFreePort()
	managementPort := getFreePort()

	// Use a temp directory for test isolation
	tempDir := t.TempDir()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		AdminPort:      adminPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)
	engineURL := fmt.Sprintf("http://localhost:%d", srv.ManagementPort())
	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(engineURL),
		admin.WithAPIKeyDisabled(), // Disable API key auth for tests
		admin.WithDataDir(tempDir), // Use temp dir for test isolation
	)

	err := srv.Start()
	require.NoError(t, err)

	err = adminAPI.Start()
	require.NoError(t, err)

	waitForReady(t, adminPort)

	cleanup := func() {
		adminAPI.Stop()
		srv.Stop()
		// Small delay to ensure file handles are released before TempDir cleanup
		time.Sleep(10 * time.Millisecond)
	}

	return srv, adminAPI, httpPort, adminPort, cleanup
}

// T054: Create mock via engine client and verify it works
func TestAdminAPICreateMock(t *testing.T) {
	srv, _, httpPort, _, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create engine client for direct mock management
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Create mock via engine client
	mockCfg := &config.MockConfiguration{
		Name:    "Test Mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"message": "hello"}`,
			},
		},
	}

	created, err := client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "Test Mock", created.Name)
	assert.NotNil(t, created.Enabled)
	assert.True(t, *created.Enabled)

	// Verify mock works
	mockResp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/test", httpPort))
	require.NoError(t, err)
	defer mockResp.Body.Close()

	assert.Equal(t, 200, mockResp.StatusCode)
	respBody, _ := io.ReadAll(mockResp.Body)
	assert.Equal(t, `{"message": "hello"}`, string(respBody))
}

// T055: GET /mocks lists mocks
func TestAdminAPIListMocks(t *testing.T) {
	srv, _, _, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create engine client for mock CRUD
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add some mocks via engine client
	_, err := client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "mock-1",
		Name:    "Mock 1",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/one"},
			Response: &mock.HTTPResponse{StatusCode: 200, Body: "one"},
		},
	})
	require.NoError(t, err)

	_, err = client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "mock-2",
		Name:    "Mock 2",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/two"},
			Response: &mock.HTTPResponse{StatusCode: 200, Body: "two"},
		},
	})
	require.NoError(t, err)

	// List mocks via admin API
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/mocks", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result admin.MockListResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Count)
	assert.Len(t, result.Mocks, 2)
}

// T056: PUT /mocks/{id} updates mock
func TestAdminAPIUpdateMock(t *testing.T) {
	_, _, httpPort, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create initial mock via Admin API (so it's in the store)
	createData := map[string]interface{}{
		"id":      "update-test",
		"name":    "Original",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/update-test",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "original",
			},
		},
	}
	createBody, _ := json.Marshal(createData)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", adminPort),
		"application/json",
		bytes.NewReader(createBody),
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify original response
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/update-test", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "original", string(body))

	// Update via admin API
	updateData := map[string]interface{}{
		"name": "Updated",
		"type": "http",
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/update-test",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "updated",
			},
		},
		"enabled": true,
	}

	updateBody, _ := json.Marshal(updateData)
	req, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://localhost:%d/mocks/update-test", adminPort),
		bytes.NewReader(updateBody),
	)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err = httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify updated response
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/update-test", httpPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "updated", string(body))
}

// T057: DELETE /mocks/{id} removes mock
func TestAdminAPIDeleteMock(t *testing.T) {
	_, _, httpPort, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create mock via Admin API (so it's in the store)
	createData := map[string]interface{}{
		"id":      "delete-test",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/delete-test",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "exists",
			},
		},
	}
	createBody, _ := json.Marshal(createData)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", adminPort),
		"application/json",
		bytes.NewReader(createBody),
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify mock exists
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/delete-test", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Delete via admin API
	req, _ := http.NewRequest(
		"DELETE",
		fmt.Sprintf("http://localhost:%d/mocks/delete-test", adminPort),
		nil,
	)
	httpClient := &http.Client{}
	resp, err = httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify mock is gone
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/delete-test", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// T058: Invalid JSON returns 400
func TestAdminAPIInvalidJSONReturns400(t *testing.T) {
	_, _, _, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Send invalid JSON
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", adminPort),
		"application/json",
		bytes.NewReader([]byte("not valid json")),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp admin.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_json", errResp.Error)
}

// T059: Duplicate ID returns 409
func TestAdminAPIDuplicateIDReturns409(t *testing.T) {
	srv, _, _, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Create first mock
	_, err := client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "duplicate-id",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/dup"},
			Response: &mock.HTTPResponse{StatusCode: 200, Body: "first"},
		},
	})
	require.NoError(t, err)

	// Try to create another with same ID (using unified mock format)
	mockData := map[string]interface{}{
		"id":   "duplicate-id",
		"type": "http",
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/dup2",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "second",
			},
		},
	}

	body, _ := json.Marshal(mockData)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", adminPort),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var errResp admin.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "duplicate_id", errResp.Error)
}

// Test health endpoint
func TestAdminAPIHealth(t *testing.T) {
	_, _, _, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health admin.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)
	assert.GreaterOrEqual(t, health.Uptime, 0)
}

// Test toggle mock
func TestAdminAPIToggleMock(t *testing.T) {
	srv, _, httpPort, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Create enabled mock
	_, err := client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "toggle-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/toggle"},
			Response: &mock.HTTPResponse{StatusCode: 200, Body: "enabled"},
		},
	})
	require.NoError(t, err)

	// Verify mock works
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/toggle", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Disable mock via toggle
	toggleData := map[string]interface{}{"enabled": false}
	body, _ := json.Marshal(toggleData)
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("http://localhost:%d/mocks/toggle-test/toggle", adminPort),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err = httpClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify mock is disabled (404)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/toggle", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// Test filter by enabled
func TestAdminAPIFilterByEnabled(t *testing.T) {
	srv, _, _, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add enabled mock
	_, err := client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "enabled-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/enabled"},
			Response: &mock.HTTPResponse{StatusCode: 200, Body: "enabled"},
		},
	})
	require.NoError(t, err)

	// Add a mock that will be disabled
	_, err = client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "disabled-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/disabled"},
			Response: &mock.HTTPResponse{StatusCode: 200, Body: "disabled"},
		},
	})
	require.NoError(t, err)

	// Toggle the mock to disabled
	_, err = client.ToggleMock(context.Background(), "disabled-mock", false)
	require.NoError(t, err)

	// Filter by enabled=true
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/mocks?enabled=true", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result admin.MockListResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "enabled-mock", result.Mocks[0].ID)

	// Filter by enabled=false
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/mocks?enabled=false", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "disabled-mock", result.Mocks[0].ID)
}
