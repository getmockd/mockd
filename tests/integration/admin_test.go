package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// setupAdminTest creates a server with admin API for testing
func setupAdminTest(t *testing.T) (*engine.Server, *admin.AdminAPI, int, int, func()) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	adminAPI := admin.NewAdminAPI(srv, adminPort)

	err := srv.Start()
	require.NoError(t, err)

	err = adminAPI.Start()
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	cleanup := func() {
		adminAPI.Stop()
		srv.Stop()
	}

	return srv, adminAPI, httpPort, adminPort, cleanup
}

// T054: POST /mocks creates mock
func TestAdminAPICreateMock(t *testing.T) {
	_, _, httpPort, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create mock via admin API
	mockData := map[string]interface{}{
		"name": "Test Mock",
		"matcher": map[string]interface{}{
			"method": "GET",
			"path":   "/api/test",
		},
		"response": map[string]interface{}{
			"statusCode": 200,
			"body":       `{"message": "hello"}`,
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

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created config.MockConfiguration
	err = json.NewDecoder(resp.Body).Decode(&created)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "Test Mock", created.Name)
	assert.True(t, created.Enabled)

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

	// Add some mocks directly
	srv.AddMock(&config.MockConfiguration{
		ID:       "mock-1",
		Name:     "Mock 1",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/one"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "one"},
	})
	srv.AddMock(&config.MockConfiguration{
		ID:       "mock-2",
		Name:     "Mock 2",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/two"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "two"},
	})

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
	srv, _, httpPort, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create initial mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "update-test",
		Name:     "Original",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/update-test"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "original"},
	})

	// Verify original response
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/update-test", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "original", string(body))

	// Update via admin API
	updateData := map[string]interface{}{
		"name": "Updated",
		"matcher": map[string]interface{}{
			"method": "GET",
			"path":   "/update-test",
		},
		"response": map[string]interface{}{
			"statusCode": 200,
			"body":       "updated",
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

	client := &http.Client{}
	resp, err = client.Do(req)
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
	srv, _, httpPort, adminPort, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "delete-test",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/delete-test"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "exists"},
	})

	// Verify mock exists
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/delete-test", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Delete via admin API
	req, _ := http.NewRequest(
		"DELETE",
		fmt.Sprintf("http://localhost:%d/mocks/delete-test", adminPort),
		nil,
	)
	client := &http.Client{}
	resp, err = client.Do(req)
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

	// Create first mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "duplicate-id",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/dup"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "first"},
	})

	// Try to create another with same ID
	mockData := map[string]interface{}{
		"id": "duplicate-id",
		"matcher": map[string]interface{}{
			"method": "GET",
			"path":   "/dup2",
		},
		"response": map[string]interface{}{
			"statusCode": 200,
			"body":       "second",
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

	// Create enabled mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "toggle-test",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/toggle"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "enabled"},
	})

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

	client := &http.Client{}
	resp, err = client.Do(req)
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

	// Add enabled and disabled mocks
	srv.AddMock(&config.MockConfiguration{
		ID:       "enabled-mock",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/enabled"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "enabled"},
	})

	disabledMock := &config.MockConfiguration{
		ID:       "disabled-mock",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Method: "GET", Path: "/disabled"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "disabled"},
	}
	srv.AddMock(disabledMock)
	disabledMock.Enabled = false
	srv.Store().Set(disabledMock)

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
