package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChaosEngineServer creates a test server that simulates chaos-related engine endpoints.
type mockChaosEngineServer struct {
	*httptest.Server
	chaosConfig *engineclient.ChaosConfig
}

func newMockChaosEngineServer() *mockChaosEngineServer {
	mces := &mockChaosEngineServer{
		chaosConfig: &engineclient.ChaosConfig{
			Enabled: false,
		},
	}

	mux := http.NewServeMux()

	// Get chaos config
	mux.HandleFunc("GET /chaos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mces.chaosConfig)
	})

	// Set chaos config
	mux.HandleFunc("PUT /chaos", func(w http.ResponseWriter, r *http.Request) {
		var cfg engineclient.ChaosConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid JSON"}`))
			return
		}
		mces.chaosConfig = &cfg
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mces.Server = httptest.NewServer(mux)
	return mces
}

func (mces *mockChaosEngineServer) client() *engineclient.Client {
	return engineclient.New(mces.URL)
}

// TestHandleGetChaos tests the GET /chaos handler.
func TestHandleGetChaos(t *testing.T) {
	t.Run("returns chaos config when disabled", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.handleGetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var resp engineclient.ChaosConfig
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.False(t, resp.Enabled)
	})

	t.Run("returns chaos config when enabled with latency", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		// Configure chaos with latency
		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "100ms",
				Max:         "500ms",
				Probability: 0.5,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.handleGetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp engineclient.ChaosConfig
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Enabled)
		require.NotNil(t, resp.Latency)
		assert.Equal(t, "100ms", resp.Latency.Min)
		assert.Equal(t, "500ms", resp.Latency.Max)
		assert.Equal(t, 0.5, resp.Latency.Probability)
	})

	t.Run("returns chaos config when enabled with error rate", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		// Configure chaos with error rate
		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			ErrorRate: &engineclient.ErrorRateConfig{
				Probability: 0.25,
				DefaultCode: 500,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.handleGetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp engineclient.ChaosConfig
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Enabled)
		require.NotNil(t, resp.ErrorRate)
		assert.Equal(t, 0.25, resp.ErrorRate.Probability)
		assert.Equal(t, 500, resp.ErrorRate.DefaultCode)
	})

	t.Run("returns chaos config with both latency and error rate", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		// Configure chaos with both
		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "50ms",
				Max:         "200ms",
				Probability: 0.8,
			},
			ErrorRate: &engineclient.ErrorRateConfig{
				Probability: 0.1,
				DefaultCode: 503,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.handleGetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp engineclient.ChaosConfig
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Enabled)
		require.NotNil(t, resp.Latency)
		require.NotNil(t, resp.ErrorRate)
		assert.Equal(t, "50ms", resp.Latency.Min)
		assert.Equal(t, 503, resp.ErrorRate.DefaultCode)
	})

	t.Run("returns chaos config with bandwidth", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Bandwidth: &engineclient.BandwidthConfig{
				BytesPerSecond: 1024,
				Probability:    0.3,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.handleGetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp engineclient.ChaosConfig
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Enabled)
		require.NotNil(t, resp.Bandwidth)
		assert.Equal(t, 1024, resp.Bandwidth.BytesPerSecond)
		assert.Equal(t, 0.3, resp.Bandwidth.Probability)
	})

	t.Run("returns chaos config with rules", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Rules: []engineclient.ChaosRuleConfig{
				{
					PathPattern: "/api/v1/*",
					Methods:     []string{"GET", "POST"},
					Probability: 0.5,
				},
				{
					PathPattern: "/health",
					Probability: 0.0,
				},
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.handleGetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp engineclient.ChaosConfig
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Enabled)
		require.Len(t, resp.Rules, 2)
		assert.Equal(t, "/api/v1/*", resp.Rules[0].PathPattern)
		assert.Equal(t, []string{"GET", "POST"}, resp.Rules[0].Methods)
		assert.Equal(t, 0.5, resp.Rules[0].Probability)
		assert.Equal(t, "/health", resp.Rules[1].PathPattern)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAdminAPI(0) // No engine

		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleGetChaos)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})
}

// TestHandleSetChaos tests the PUT /chaos handler.
func TestHandleSetChaos(t *testing.T) {
	t.Run("enables chaos with latency", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		chaosConfig := engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "100ms",
				Max:         "500ms",
				Probability: 0.5,
			},
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "ok")

		// Verify the chaos config was set
		assert.True(t, server.chaosConfig.Enabled)
		require.NotNil(t, server.chaosConfig.Latency)
		assert.Equal(t, "100ms", server.chaosConfig.Latency.Min)
		assert.Equal(t, "500ms", server.chaosConfig.Latency.Max)
		assert.Equal(t, 0.5, server.chaosConfig.Latency.Probability)
	})

	t.Run("enables chaos with error rate", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		chaosConfig := engineclient.ChaosConfig{
			Enabled: true,
			ErrorRate: &engineclient.ErrorRateConfig{
				Probability: 0.2,
				DefaultCode: 500,
			},
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify the chaos config was set
		assert.True(t, server.chaosConfig.Enabled)
		require.NotNil(t, server.chaosConfig.ErrorRate)
		assert.Equal(t, 0.2, server.chaosConfig.ErrorRate.Probability)
		assert.Equal(t, 500, server.chaosConfig.ErrorRate.DefaultCode)
	})

	t.Run("enables chaos with both latency and error rate", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		chaosConfig := engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "50ms",
				Max:         "150ms",
				Probability: 0.75,
			},
			ErrorRate: &engineclient.ErrorRateConfig{
				Probability: 0.1,
				DefaultCode: 503,
			},
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify the chaos config was set
		assert.True(t, server.chaosConfig.Enabled)
		require.NotNil(t, server.chaosConfig.Latency)
		require.NotNil(t, server.chaosConfig.ErrorRate)
	})

	t.Run("disables chaos", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		// Start with chaos enabled
		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "100ms",
				Max:         "500ms",
				Probability: 0.5,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		chaosConfig := engineclient.ChaosConfig{
			Enabled: false,
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify chaos was disabled
		assert.False(t, server.chaosConfig.Enabled)
	})

	t.Run("updates existing chaos config", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		// Start with initial config
		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "100ms",
				Max:         "500ms",
				Probability: 0.5,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		// Update to different config
		newConfig := engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "200ms",
				Max:         "1000ms",
				Probability: 0.9,
			},
			ErrorRate: &engineclient.ErrorRateConfig{
				Probability: 0.05,
				DefaultCode: 502,
			},
		}
		body, _ := json.Marshal(newConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify the config was updated
		assert.True(t, server.chaosConfig.Enabled)
		require.NotNil(t, server.chaosConfig.Latency)
		assert.Equal(t, "200ms", server.chaosConfig.Latency.Min)
		assert.Equal(t, "1000ms", server.chaosConfig.Latency.Max)
		assert.Equal(t, 0.9, server.chaosConfig.Latency.Probability)
		require.NotNil(t, server.chaosConfig.ErrorRate)
		assert.Equal(t, 0.05, server.chaosConfig.ErrorRate.Probability)
		assert.Equal(t, 502, server.chaosConfig.ErrorRate.DefaultCode)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid JSON")
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAdminAPI(0) // No engine

		chaosConfig := engineclient.ChaosConfig{
			Enabled: true,
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleSetChaos)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})

	t.Run("handles empty body gracefully", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader([]byte("")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		// Should return bad request for empty body (invalid JSON)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("handles null values in config", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		// Send config with explicit null values
		body := []byte(`{"enabled": true, "latency": null, "errorRate": null}`)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, server.chaosConfig.Enabled)
		assert.Nil(t, server.chaosConfig.Latency)
		assert.Nil(t, server.chaosConfig.ErrorRate)
	})
}

// TestChaosConfigValidation tests various chaos configuration scenarios.
func TestChaosConfigValidation(t *testing.T) {
	t.Run("accepts valid latency config with zero probability", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		chaosConfig := engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "0ms",
				Max:         "100ms",
				Probability: 0,
			},
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(0), server.chaosConfig.Latency.Probability)
	})

	t.Run("accepts valid latency config with max probability", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		chaosConfig := engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "100ms",
				Max:         "200ms",
				Probability: 1.0,
			},
		}
		body, _ := json.Marshal(chaosConfig)

		req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleSetChaos(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 1.0, server.chaosConfig.Latency.Probability)
	})

	t.Run("accepts various status codes for error rate", func(t *testing.T) {
		testCases := []int{400, 401, 403, 404, 500, 502, 503, 504}

		for _, statusCode := range testCases {
			t.Run(string(rune(statusCode)), func(t *testing.T) {
				server := newMockChaosEngineServer()
				defer server.Close()

				api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

				chaosConfig := engineclient.ChaosConfig{
					Enabled: true,
					ErrorRate: &engineclient.ErrorRateConfig{
						Probability: 0.1,
						DefaultCode: statusCode,
					},
				}
				body, _ := json.Marshal(chaosConfig)

				req := httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				api.handleSetChaos(rec, req, server.client())

				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, statusCode, server.chaosConfig.ErrorRate.DefaultCode)
			})
		}
	})
}

// TestChaosHandlerRoundTrip tests getting and setting chaos config in sequence.
func TestChaosHandlerRoundTrip(t *testing.T) {
	t.Run("get-set-get roundtrip", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		// Get initial config
		req := httptest.NewRequest("GET", "/chaos", nil)
		rec := httptest.NewRecorder()
		api.handleGetChaos(rec, req, server.client())
		assert.Equal(t, http.StatusOK, rec.Code)

		var initialConfig engineclient.ChaosConfig
		json.Unmarshal(rec.Body.Bytes(), &initialConfig)
		assert.False(t, initialConfig.Enabled)

		// Set new config
		newConfig := engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "50ms",
				Max:         "100ms",
				Probability: 0.75,
			},
		}
		body, _ := json.Marshal(newConfig)

		req = httptest.NewRequest("PUT", "/chaos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		api.handleSetChaos(rec, req, server.client())
		assert.Equal(t, http.StatusOK, rec.Code)

		// Get updated config
		req = httptest.NewRequest("GET", "/chaos", nil)
		rec = httptest.NewRecorder()
		api.handleGetChaos(rec, req, server.client())
		assert.Equal(t, http.StatusOK, rec.Code)

		var updatedConfig engineclient.ChaosConfig
		json.Unmarshal(rec.Body.Bytes(), &updatedConfig)
		assert.True(t, updatedConfig.Enabled)
		require.NotNil(t, updatedConfig.Latency)
		assert.Equal(t, "50ms", updatedConfig.Latency.Min)
		assert.Equal(t, "100ms", updatedConfig.Latency.Max)
		assert.Equal(t, 0.75, updatedConfig.Latency.Probability)
	})
}

// TestChaosHandlerConcurrency tests that chaos handlers are safe for concurrent access.
func TestChaosHandlerConcurrency(t *testing.T) {
	t.Run("handles concurrent get requests", func(t *testing.T) {
		server := newMockChaosEngineServer()
		defer server.Close()

		server.chaosConfig = &engineclient.ChaosConfig{
			Enabled: true,
			Latency: &engineclient.LatencyConfig{
				Min:         "100ms",
				Max:         "200ms",
				Probability: 0.5,
			},
		}

		api := NewAdminAPI(0, WithLocalEngineClient(server.client()))

		// Make multiple concurrent GET requests
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				req := httptest.NewRequest("GET", "/chaos", nil)
				rec := httptest.NewRecorder()
				api.handleGetChaos(rec, req, server.client())
				assert.Equal(t, http.StatusOK, rec.Code)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
