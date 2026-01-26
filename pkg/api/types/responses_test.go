package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorResponse(t *testing.T) {
	t.Parallel()

	t.Run("serializes without details", func(t *testing.T) {
		t.Parallel()
		resp := ErrorResponse{
			Error:   "not_found",
			Message: "Resource not found",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var result map[string]any
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "not_found", result["error"])
		assert.Equal(t, "Resource not found", result["message"])
		assert.Nil(t, result["details"]) // omitted when nil
	})

	t.Run("serializes with details", func(t *testing.T) {
		t.Parallel()
		resp := ErrorResponse{
			Error:   "validation_error",
			Message: "Validation failed",
			Details: map[string]string{"field": "email"},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var result map[string]any
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "validation_error", result["error"])
		assert.NotNil(t, result["details"])
	})
}

func TestHealthResponse(t *testing.T) {
	t.Parallel()

	resp := HealthResponse{
		Status:    "ok",
		Uptime:    3600,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result HealthResponse
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, 3600, result.Uptime)
}

func TestMockListResponse(t *testing.T) {
	t.Parallel()

	resp := MockListResponse{
		Mocks: nil,
		Count: 0,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(0), result["count"])
}

func TestProtocolStatus(t *testing.T) {
	t.Parallel()

	status := ProtocolStatus{
		Enabled:     true,
		Port:        8080,
		Connections: 5,
		Status:      "running",
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)

	var result ProtocolStatus
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.True(t, result.Enabled)
	assert.Equal(t, 8080, result.Port)
	assert.Equal(t, 5, result.Connections)
	assert.Equal(t, "running", result.Status)
}

func TestServerStatus(t *testing.T) {
	t.Parallel()

	status := ServerStatus{
		Status:       "running",
		ID:           "engine-1",
		HTTPPort:     4280,
		Uptime:       3600,
		MockCount:    10,
		ActiveMocks:  8,
		RequestCount: 1000,
		Version:      "1.0.0",
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)

	var result ServerStatus
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "running", result.Status)
	assert.Equal(t, "engine-1", result.ID)
	assert.Equal(t, 4280, result.HTTPPort)
	assert.Equal(t, int64(3600), result.Uptime)
}

func TestRequestLogEntry(t *testing.T) {
	t.Parallel()

	entry := RequestLogEntry{
		ID:            "req-123",
		Timestamp:     time.Now(),
		Protocol:      "http",
		Method:        "GET",
		Path:          "/api/users",
		MatchedMockID: "mock-456",
		StatusCode:    200,
		DurationMs:    15,
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var result RequestLogEntry
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "req-123", result.ID)
	assert.Equal(t, "http", result.Protocol)
	assert.Equal(t, "GET", result.Method)
	assert.Equal(t, "/api/users", result.Path)
	assert.Equal(t, 200, result.StatusCode)
}

func TestToggleRequest(t *testing.T) {
	t.Parallel()

	t.Run("enabled true", func(t *testing.T) {
		t.Parallel()
		req := ToggleRequest{Enabled: true}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var result ToggleRequest
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.True(t, result.Enabled)
	})

	t.Run("enabled false", func(t *testing.T) {
		t.Parallel()
		req := ToggleRequest{Enabled: false}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var result ToggleRequest
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.False(t, result.Enabled)
	})
}

func TestPaginatedResponse(t *testing.T) {
	t.Parallel()

	resp := PaginatedResponse[string]{
		Items:  []string{"a", "b", "c"},
		Count:  3,
		Total:  100,
		Offset: 0,
		Limit:  10,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result PaginatedResponse[string]
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, 3, result.Count)
	assert.Equal(t, 100, result.Total)
	assert.Equal(t, []string{"a", "b", "c"}, result.Items)
}
