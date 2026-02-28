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

// --- ChaosFaultConfig UnmarshalJSON Tests ---

func TestChaosFaultConfig_UnmarshalJSON_Canonical(t *testing.T) {
	t.Parallel()

	input := `{"type":"circuit_breaker","probability":1.0,"config":{"tripAfter":3,"openDuration":"10s"}}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "circuit_breaker", f.Type)
	assert.Equal(t, 1.0, f.Probability)
	assert.Equal(t, float64(3), f.Config["tripAfter"])
	assert.Equal(t, "10s", f.Config["openDuration"])
}

func TestChaosFaultConfig_UnmarshalJSON_Flat(t *testing.T) {
	t.Parallel()

	// User sends config params at top level — the common mistake / natural format.
	input := `{"type":"circuit_breaker","probability":1.0,"tripAfter":3,"openDuration":"10s"}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "circuit_breaker", f.Type)
	assert.Equal(t, 1.0, f.Probability)
	assert.Equal(t, float64(3), f.Config["tripAfter"])
	assert.Equal(t, "10s", f.Config["openDuration"])
}

func TestChaosFaultConfig_UnmarshalJSON_BothFlatAndNested(t *testing.T) {
	t.Parallel()

	// Explicit "config" value wins when both are present for the same key.
	input := `{
		"type": "circuit_breaker",
		"probability": 1.0,
		"tripAfter": 5,
		"openStatusCode": 502,
		"config": {
			"tripAfter": 3,
			"openDuration": "10s"
		}
	}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "circuit_breaker", f.Type)
	assert.Equal(t, 1.0, f.Probability)
	// Explicit config wins for tripAfter
	assert.Equal(t, float64(3), f.Config["tripAfter"])
	// openDuration comes from config
	assert.Equal(t, "10s", f.Config["openDuration"])
	// openStatusCode comes from flat (not in config)
	assert.Equal(t, float64(502), f.Config["openStatusCode"])
}

func TestChaosFaultConfig_UnmarshalJSON_NoExtras(t *testing.T) {
	t.Parallel()

	// Basic fault with no config params at all — Config stays nil.
	input := `{"type":"timeout","probability":0.5}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "timeout", f.Type)
	assert.Equal(t, 0.5, f.Probability)
	assert.Nil(t, f.Config)
}

func TestChaosFaultConfig_UnmarshalJSON_EmptyConfig(t *testing.T) {
	t.Parallel()

	input := `{"type":"latency","probability":0.3,"config":{}}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "latency", f.Type)
	assert.Equal(t, 0.3, f.Probability)
	// Empty config stays as empty map (not nil) since it was explicitly provided.
	assert.NotNil(t, f.Config)
	assert.Empty(t, f.Config)
}

func TestChaosFaultConfig_UnmarshalJSON_RetryAfterFlat(t *testing.T) {
	t.Parallel()

	input := `{"type":"retry_after","probability":1.0,"statusCode":429,"retryAfter":"30s","body":"rate limited"}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "retry_after", f.Type)
	assert.Equal(t, float64(429), f.Config["statusCode"])
	assert.Equal(t, "30s", f.Config["retryAfter"])
	assert.Equal(t, "rate limited", f.Config["body"])
}

func TestChaosFaultConfig_UnmarshalJSON_ProgressiveDegradationFlat(t *testing.T) {
	t.Parallel()

	input := `{
		"type": "progressive_degradation",
		"probability": 1.0,
		"initialDelay": "20ms",
		"delayIncrement": "5ms",
		"maxDelay": "5s",
		"errorAfter": 100
	}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	require.NoError(t, err)

	assert.Equal(t, "progressive_degradation", f.Type)
	assert.Equal(t, "20ms", f.Config["initialDelay"])
	assert.Equal(t, "5ms", f.Config["delayIncrement"])
	assert.Equal(t, "5s", f.Config["maxDelay"])
	assert.Equal(t, float64(100), f.Config["errorAfter"])
}

func TestChaosFaultConfig_UnmarshalJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	input := `{"type":"circuit_breaker","probability":}`

	var f ChaosFaultConfig
	err := json.Unmarshal([]byte(input), &f)
	assert.Error(t, err)
}

func TestChaosFaultConfig_UnmarshalJSON_NestedArray(t *testing.T) {
	t.Parallel()

	// ChaosRuleConfig.Faults is a slice of ChaosFaultConfig — ensure the
	// custom UnmarshalJSON works correctly when deserialized as part of a parent struct.
	input := `{
		"pathPattern": "/api/.*",
		"faults": [
			{"type":"circuit_breaker","probability":1.0,"tripAfter":3},
			{"type":"latency","probability":0.5,"config":{"min":"100ms","max":"500ms"}}
		]
	}`

	var rule ChaosRuleConfig
	err := json.Unmarshal([]byte(input), &rule)
	require.NoError(t, err)

	require.Len(t, rule.Faults, 2)

	// First fault: flat format
	assert.Equal(t, "circuit_breaker", rule.Faults[0].Type)
	assert.Equal(t, float64(3), rule.Faults[0].Config["tripAfter"])

	// Second fault: canonical format
	assert.Equal(t, "latency", rule.Faults[1].Type)
	assert.Equal(t, "100ms", rule.Faults[1].Config["min"])
	assert.Equal(t, "500ms", rule.Faults[1].Config["max"])
}

func TestChaosFaultConfig_UnmarshalJSON_FullChaosConfig(t *testing.T) {
	t.Parallel()

	// End-to-end: full ChaosConfig with flat fault params, as a user would send it.
	input := `{
		"enabled": true,
		"rules": [
			{
				"pathPattern": "/api/orders.*",
				"faults": [
					{
						"type": "circuit_breaker",
						"probability": 1.0,
						"tripAfter": 3,
						"openDuration": "10s",
						"openStatusCode": 503
					}
				]
			}
		]
	}`

	var cfg ChaosConfig
	err := json.Unmarshal([]byte(input), &cfg)
	require.NoError(t, err)

	require.True(t, cfg.Enabled)
	require.Len(t, cfg.Rules, 1)
	require.Len(t, cfg.Rules[0].Faults, 1)

	fault := cfg.Rules[0].Faults[0]
	assert.Equal(t, "circuit_breaker", fault.Type)
	assert.Equal(t, 1.0, fault.Probability)
	assert.Equal(t, float64(3), fault.Config["tripAfter"])
	assert.Equal(t, "10s", fault.Config["openDuration"])
	assert.Equal(t, float64(503), fault.Config["openStatusCode"])
}

func TestChaosFaultConfig_MarshalJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	// Marshal always produces the canonical format with nested "config".
	original := ChaosFaultConfig{
		Type:        "circuit_breaker",
		Probability: 1.0,
		Config:      map[string]any{"tripAfter": float64(3), "openDuration": "10s"},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var roundTripped ChaosFaultConfig
	err = json.Unmarshal(data, &roundTripped)
	require.NoError(t, err)

	assert.Equal(t, original.Type, roundTripped.Type)
	assert.Equal(t, original.Probability, roundTripped.Probability)
	assert.Equal(t, original.Config["tripAfter"], roundTripped.Config["tripAfter"])
	assert.Equal(t, original.Config["openDuration"], roundTripped.Config["openDuration"])
}
