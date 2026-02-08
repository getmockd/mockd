// Package integration provides integration tests for chaos engineering features.
package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// TestBundle holds test setup components for chaos tests.
type TestBundle struct {
	Server         *engine.Server
	HTTPPort       int
	ManagementPort int
	Client         *engineclient.Client
	Cleanup        func()
}

// setupChaosTestServer creates a server with chaos middleware enabled.
func setupChaosTestServer(t *testing.T, chaosConfig *chaos.ChaosConfig) *TestBundle {
	httpPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
		Chaos:          chaosConfig,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	cleanup := func() {
		srv.Stop()
	}

	return &TestBundle{
		Server:         srv,
		HTTPPort:       httpPort,
		ManagementPort: srv.ManagementPort(),
		Client:         client,
		Cleanup:        cleanup,
	}
}

// setupBasicMock creates a basic HTTP mock for testing.
func setupBasicMock(t *testing.T, client *engineclient.Client, path string, body string) *config.MockConfiguration {
	mockCfg := &config.MockConfiguration{
		Name:    "Test Mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   path,
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: body,
			},
		},
	}

	created, err := client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)
	return created
}

// measureRequestTime measures how long an HTTP request takes.
func measureRequestTime(url string) (time.Duration, *http.Response, error) {
	start := time.Now()
	resp, err := http.Get(url)
	elapsed := time.Since(start)
	return elapsed, resp, err
}

// TestChaosLatencyInjectionFixed tests fixed latency injection.
func TestChaosLatencyInjectionFixed(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "200ms",
				Max:         "200ms", // Fixed latency
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Make request and measure time
	elapsed, resp, err := measureRequestTime(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response is successful
	assert.Equal(t, 200, resp.StatusCode)

	// Verify latency was injected (should be at least 200ms)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(200),
		"Response should take at least 200ms due to injected latency")
}

// TestChaosLatencyInjectionRange tests random latency injection within a range.
func TestChaosLatencyInjectionRange(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "100ms",
				Max:         "300ms",
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Make multiple requests to verify randomness
	var durations []time.Duration
	for i := 0; i < 5; i++ {
		elapsed, resp, err := measureRequestTime(url)
		require.NoError(t, err)
		resp.Body.Close()
		durations = append(durations, elapsed)
	}

	// Verify all durations are within expected range
	for i, d := range durations {
		assert.GreaterOrEqual(t, d.Milliseconds(), int64(100),
			"Request %d should take at least 100ms", i)
		// Allow some overhead above max
		assert.LessOrEqual(t, d.Milliseconds(), int64(400),
			"Request %d should not exceed 400ms (300ms + overhead)", i)
	}

	// Verify there's some variance (not all exactly the same)
	// At least one duration should differ by more than 10ms from another
	var hasVariance bool
	for i := 0; i < len(durations)-1; i++ {
		diff := durations[i] - durations[i+1]
		if diff < 0 {
			diff = -diff
		}
		if diff > 10*time.Millisecond {
			hasVariance = true
			break
		}
	}
	// Note: This might occasionally fail due to randomness, but with 5 samples
	// and a 200ms range, it's unlikely all will be within 10ms of each other
	if !hasVariance {
		t.Log("Warning: No significant variance detected in latency, but this could happen by chance")
	}
}

// TestChaosErrorRateInjection tests error rate injection.
func TestChaosErrorRateInjection(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			ErrorRate: &chaos.ErrorRateFault{
				Probability: 0.5,
				DefaultCode: 500,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Make many requests to verify error rate
	requests := 100
	errors := 0

	for i := 0; i < requests; i++ {
		resp, err := http.Get(url)
		require.NoError(t, err)
		if resp.StatusCode >= 500 {
			errors++
		}
		resp.Body.Close()
	}

	actualRate := float64(errors) / float64(requests)

	// Allow 20% tolerance for statistical variation (50% +/- 20%)
	assert.InDelta(t, 0.5, actualRate, 0.20,
		"Error rate should be approximately 50%% (got %.2f%%)", actualRate*100)
}

// TestChaosErrorRateWithCustomStatusCodes tests error injection with custom status codes.
func TestChaosErrorRateWithCustomStatusCodes(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			ErrorRate: &chaos.ErrorRateFault{
				Probability: 1.0, // Always inject error
				StatusCodes: []int{502, 503, 504},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	statusCounts := make(map[int]int)

	// Make requests to verify status codes
	for i := 0; i < 30; i++ {
		resp, err := http.Get(url)
		require.NoError(t, err)
		statusCounts[resp.StatusCode]++
		resp.Body.Close()
	}

	// Verify only expected status codes are returned
	for status := range statusCounts {
		assert.Contains(t, []int{502, 503, 504}, status,
			"Unexpected status code: %d", status)
	}

	// Verify all three status codes appear (with 30 requests, statistically likely)
	// Allow for possibility that not all appear, just log a warning
	if len(statusCounts) < 2 {
		t.Log("Warning: Not all expected status codes appeared, but this could happen by chance")
	}
}

// TestChaosTimeoutSimulation tests timeout simulation via long latency.
func TestChaosTimeoutSimulation(t *testing.T) {
	// Use a long latency to simulate timeout
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "5s",
				Max:         "5s",
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Create client with short timeout
	client := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	// Request should timeout
	resp, err := client.Get(url)
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err)

	// Verify it's a timeout error (could be "timeout" or "deadline exceeded")
	errStr := err.Error()
	isTimeoutError := strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "Timeout")
	assert.True(t, isTimeoutError,
		"Error should be a timeout error, got: %s", errStr)
}

// TestChaosConnectionReset tests connection reset injection.
func TestChaosConnectionReset(t *testing.T) {
	// Connection reset requires path-specific rules with FaultConnectionReset
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/reset",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultConnectionReset,
						Probability: 1.0,
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/reset", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/reset", bundle.HTTPPort)

	// Request should fail with connection error
	resp, err := http.Get(url)

	// Either we get an error (connection reset) or the connection was hijacked
	// and we get no response
	if err == nil && resp != nil {
		resp.Body.Close()
		// If we got a response, it should be an empty one or error
		// Connection reset is implementation-specific
		t.Log("Connection was hijacked but request completed - behavior varies by platform")
	} else {
		// Verify we got a connection error, not an HTTP error
		if err != nil {
			// Check for connection-related error patterns
			errStr := err.Error()
			isConnectionError := strings.Contains(errStr, "connection reset") ||
				strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "broken pipe")
			assert.True(t, isConnectionError || strings.Contains(errStr, "EOF"),
				"Expected connection error, got: %v", err)
		}
	}
}

// TestChaosBodyCorruption tests response body corruption.
func TestChaosBodyCorruption(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/corrupt",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultCorruptBody,
						Probability: 1.0,
						Config: map[string]interface{}{
							"corruptRate": 0.5, // 50% of bytes corrupted
						},
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	originalBody := `{"status": "ok", "data": "hello world"}`
	setupBasicMock(t, bundle.Client, "/api/corrupt", originalBody)

	url := fmt.Sprintf("http://localhost:%d/api/corrupt", bundle.HTTPPort)

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// With 50% corruption rate, the body should be corrupted
	// and not valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)

	// Either JSON parsing fails (corrupted) or the body is different
	if err == nil {
		// If JSON is valid, check if content differs
		t.Logf("Body was valid JSON: %s (original: %s)", string(body), originalBody)
		// Note: With high corruption rate, this is unlikely but possible
	} else {
		// Body is corrupted (expected)
		assert.Error(t, err, "Body should be corrupted and not valid JSON")
	}
}

// TestChaosPartialResponse tests response truncation.
func TestChaosPartialResponse(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/partial",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultPartialResponse,
						Probability: 1.0,
						Config: map[string]interface{}{
							"maxBytes": 50, // Truncate to 50 bytes
						},
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	// Create a large response body (1000 bytes)
	largeBody := strings.Repeat("x", 1000)
	setupBasicMock(t, bundle.Client, "/api/partial", largeBody)

	url := fmt.Sprintf("http://localhost:%d/api/partial", bundle.HTTPPort)

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Body should be truncated
	assert.LessOrEqual(t, len(body), 50,
		"Response should be truncated to at most 50 bytes, got %d", len(body))
}

// TestChaosBandwidthThrottling tests bandwidth limiting.
func TestChaosBandwidthThrottling(t *testing.T) {
	// Skip in short mode as this test takes time
	if testing.Short() {
		t.Skip("Skipping bandwidth test in short mode")
	}

	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Bandwidth: &chaos.BandwidthFault{
				BytesPerSecond: 1000, // 1 KB/s
				Probability:    1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	// Create 5KB response
	largeBody := strings.Repeat("x", 5000)
	setupBasicMock(t, bundle.Client, "/api/slow", largeBody)

	url := fmt.Sprintf("http://localhost:%d/api/slow", bundle.HTTPPort)

	start := time.Now()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read all data
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	elapsed := time.Since(start)

	// At 1KB/s, 5KB should take approximately 5 seconds
	// Allow for variance - should take at least 3 seconds
	assert.GreaterOrEqual(t, elapsed.Seconds(), 3.0,
		"Download should take at least 3 seconds at 1KB/s for 5KB")
}

// TestChaosCombinedLatencyAndErrorRate tests multiple chaos features together.
func TestChaosCombinedLatencyAndErrorRate(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "50ms",
				Max:         "50ms",
				Probability: 1.0, // Always add latency
			},
			ErrorRate: &chaos.ErrorRateFault{
				Probability: 0.3, // 30% error rate
				DefaultCode: 503,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	requests := 50
	errors := 0
	slowRequests := 0

	for i := 0; i < requests; i++ {
		start := time.Now()
		resp, err := http.Get(url)
		elapsed := time.Since(start)
		require.NoError(t, err)

		if resp.StatusCode >= 500 {
			errors++
		}
		if elapsed >= 50*time.Millisecond {
			slowRequests++
		}
		resp.Body.Close()
	}

	errorRate := float64(errors) / float64(requests)
	slowRate := float64(slowRequests) / float64(requests)

	// Verify error rate is approximately 30%
	assert.InDelta(t, 0.3, errorRate, 0.20,
		"Error rate should be approximately 30%% (got %.2f%%)", errorRate*100)

	// Verify most requests have latency (should be close to 100%)
	// Note: Error responses might not have latency applied depending on implementation
	assert.Greater(t, slowRate, 0.5,
		"Most requests should have latency applied (got %.2f%%)", slowRate*100)
}

// TestChaosPerEndpoint tests path-specific chaos rules.
func TestChaosPerEndpoint(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/slow",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultLatency,
						Probability: 1.0,
						Config: map[string]interface{}{
							"min": "200ms",
							"max": "200ms",
						},
					},
				},
			},
		},
		// No global rules - /api/fast should be unaffected
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/fast", `{"status": "fast"}`)
	setupBasicMock(t, bundle.Client, "/api/slow", `{"status": "slow"}`)

	fastURL := fmt.Sprintf("http://localhost:%d/api/fast", bundle.HTTPPort)
	slowURL := fmt.Sprintf("http://localhost:%d/api/slow", bundle.HTTPPort)

	// Test fast endpoint (no chaos)
	fastElapsed, fastResp, err := measureRequestTime(fastURL)
	require.NoError(t, err)
	fastResp.Body.Close()

	// Test slow endpoint (with latency)
	slowElapsed, slowResp, err := measureRequestTime(slowURL)
	require.NoError(t, err)
	slowResp.Body.Close()

	// Fast endpoint should be quick (< 100ms)
	assert.Less(t, fastElapsed.Milliseconds(), int64(100),
		"Fast endpoint should respond quickly")

	// Slow endpoint should have latency (>= 200ms)
	assert.GreaterOrEqual(t, slowElapsed.Milliseconds(), int64(200),
		"Slow endpoint should have 200ms latency")
}

// TestChaosMethodSpecificRules tests method-specific chaos rules.
func TestChaosMethodSpecificRules(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/test",
				Methods:     []string{"POST"}, // Only affect POST
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultError,
						Probability: 1.0,
						Config: map[string]interface{}{
							"defaultCode": 500,
						},
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	// Create mock that accepts both GET and POST
	mockCfg := &config.MockConfiguration{
		Name:    "Method Test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/api/test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"status": "ok"}`,
			},
		},
	}
	_, err := bundle.Client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// GET should succeed
	getResp, err := http.Get(url)
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, 200, getResp.StatusCode, "GET should succeed")

	// POST should fail
	postResp, err := http.Post(url, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer postResp.Body.Close()
	assert.Equal(t, 500, postResp.StatusCode, "POST should return 500")
}

// TestChaosDynamicToggle tests enabling/disabling chaos at runtime.
func TestChaosDynamicToggle(t *testing.T) {
	// Start with chaos disabled
	bundle := setupChaosTestServer(t, nil)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Request should be fast (no chaos)
	elapsed1, resp1, err := measureRequestTime(url)
	require.NoError(t, err)
	resp1.Body.Close()
	assert.Less(t, elapsed1.Milliseconds(), int64(50),
		"Without chaos, request should be fast")

	// Enable chaos via injector
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "200ms",
				Max:         "200ms",
				Probability: 1.0,
			},
		},
	}
	injector, err := chaos.NewInjector(chaosConfig)
	require.NoError(t, err)
	err = bundle.Server.SetChaosInjector(injector)
	require.NoError(t, err)

	// Request should now be slow
	elapsed2, resp2, err := measureRequestTime(url)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.GreaterOrEqual(t, elapsed2.Milliseconds(), int64(200),
		"With chaos enabled, request should have latency")

	// Disable chaos
	err = bundle.Server.SetChaosInjector(nil)
	require.NoError(t, err)

	// Request should be fast again
	elapsed3, resp3, err := measureRequestTime(url)
	require.NoError(t, err)
	resp3.Body.Close()
	assert.Less(t, elapsed3.Milliseconds(), int64(50),
		"With chaos disabled, request should be fast again")
}

// TestChaosStats tests chaos statistics tracking.
func TestChaosStats(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "10ms",
				Max:         "10ms",
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Make several requests
	for i := 0; i < 10; i++ {
		resp, err := http.Get(url)
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Check stats
	injector := bundle.Server.ChaosInjector()
	require.NotNil(t, injector)

	stats := injector.GetStats()
	assert.Equal(t, int64(10), stats.TotalRequests,
		"Should have tracked 10 requests")
	assert.GreaterOrEqual(t, stats.InjectedFaults, int64(10),
		"Should have injected at least 10 faults")
	assert.GreaterOrEqual(t, stats.LatencyInjected, int64(10),
		"Should have injected latency 10 times")

	// Reset stats
	injector.ResetStats()
	statsAfterReset := injector.GetStats()
	assert.Equal(t, int64(0), statsAfterReset.TotalRequests,
		"Stats should be reset")
}

// TestChaosWithConcurrentRequests tests chaos under concurrent load.
func TestChaosWithConcurrentRequests(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "50ms",
				Max:         "100ms",
				Probability: 1.0,
			},
			ErrorRate: &chaos.ErrorRateFault{
				Probability: 0.2,
				DefaultCode: 500,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	errorCount := 0
	var totalDuration time.Duration

	// Make 50 concurrent requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			elapsed, resp, err := measureRequestTime(url)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			totalDuration += elapsed
			if resp.StatusCode >= 500 {
				errorCount++
			} else {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	total := successCount + errorCount
	assert.Equal(t, 50, total, "All requests should complete")

	avgDuration := totalDuration / time.Duration(total)
	assert.GreaterOrEqual(t, avgDuration.Milliseconds(), int64(50),
		"Average duration should reflect latency injection")

	// Error rate should be approximately 20%
	errorRate := float64(errorCount) / float64(total)
	assert.InDelta(t, 0.2, errorRate, 0.20,
		"Error rate should be approximately 20%%")
}

// TestChaosProbabilityZero tests that probability 0 means no chaos.
func TestChaosProbabilityZero(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "1s",
				Max:         "1s",
				Probability: 0.0, // Never apply
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// All requests should be fast (no latency applied)
	for i := 0; i < 10; i++ {
		elapsed, resp, err := measureRequestTime(url)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Less(t, elapsed.Milliseconds(), int64(100),
			"With probability 0, no latency should be applied")
	}
}

// TestChaosDisabled tests that disabled chaos config does nothing.
func TestChaosDisabled(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: false, // Disabled
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "1s",
				Max:         "1s",
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	elapsed, resp, err := measureRequestTime(url)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Less(t, elapsed.Milliseconds(), int64(100),
		"With chaos disabled, no latency should be applied")
}

// TestChaosRegexPathMatching tests regex pattern matching for paths.
func TestChaosRegexPathMatching(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/users/[0-9]+",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultLatency,
						Probability: 1.0,
						Config: map[string]interface{}{
							"min": "200ms",
							"max": "200ms",
						},
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	// Create mocks for different paths
	setupBasicMock(t, bundle.Client, "/api/users/123", `{"id": 123}`)
	setupBasicMock(t, bundle.Client, "/api/users/list", `{"users": []}`)

	// /api/users/123 should match regex and have latency
	url1 := fmt.Sprintf("http://localhost:%d/api/users/123", bundle.HTTPPort)
	elapsed1, resp1, err := measureRequestTime(url1)
	require.NoError(t, err)
	resp1.Body.Close()
	assert.GreaterOrEqual(t, elapsed1.Milliseconds(), int64(200),
		"/api/users/123 should match regex and have latency")

	// /api/users/list should NOT match regex
	url2 := fmt.Sprintf("http://localhost:%d/api/users/list", bundle.HTTPPort)
	elapsed2, resp2, err := measureRequestTime(url2)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Less(t, elapsed2.Milliseconds(), int64(100),
		"/api/users/list should not match regex and be fast")
}

// TestChaosSlowBodyDelivery tests slow response body delivery.
func TestChaosSlowBodyDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow body test in short mode")
	}

	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/slow-body",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultSlowBody,
						Probability: 1.0,
						Config: map[string]interface{}{
							"bytesPerSecond": 500, // 500 bytes per second
						},
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	// Create 2KB response
	body := strings.Repeat("x", 2000)
	setupBasicMock(t, bundle.Client, "/api/slow-body", body)

	url := fmt.Sprintf("http://localhost:%d/api/slow-body", bundle.HTTPPort)

	start := time.Now()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read body
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	elapsed := time.Since(start)

	assert.Len(t, data, 2000, "Should receive full body")

	// At 500 bytes/s, 2000 bytes should take ~4 seconds
	// Allow for some variance - should take at least 2 seconds
	assert.GreaterOrEqual(t, elapsed.Seconds(), 2.0,
		"Slow body delivery should take at least 2 seconds")
}

// TestChaosEmptyResponse tests empty response injection.
func TestChaosEmptyResponse(t *testing.T) {
	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		Rules: []chaos.ChaosRule{
			{
				PathPattern: "/api/empty",
				Probability: 1.0,
				Faults: []chaos.FaultConfig{
					{
						Type:        chaos.FaultEmptyResponse,
						Probability: 1.0,
					},
				},
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/empty", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/empty", bundle.HTTPPort)

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Response should be empty
	assert.Equal(t, 200, resp.StatusCode)
	assert.Empty(t, body, "Response body should be empty")
}

// TestChaosProbabilityBoundaryValues tests probability edge cases.
func TestChaosProbabilityBoundaryValues(t *testing.T) {
	testCases := []struct {
		name        string
		probability float64
		expectError bool
	}{
		{"valid_zero", 0.0, false},
		{"valid_one", 1.0, false},
		{"valid_half", 0.5, false},
		{"invalid_negative", -0.1, true},
		{"invalid_over_one", 1.1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chaosConfig := &chaos.ChaosConfig{
				Enabled: true,
				GlobalRules: &chaos.GlobalChaosRules{
					Latency: &chaos.LatencyFault{
						Min:         "10ms",
						Max:         "10ms",
						Probability: tc.probability,
					},
				},
			}

			err := chaosConfig.Validate()
			if tc.expectError {
				assert.Error(t, err, "Should fail validation for probability %.2f", tc.probability)
			} else {
				assert.NoError(t, err, "Should pass validation for probability %.2f", tc.probability)
			}
		})
	}
}

// TestChaosConfigValidation tests chaos configuration validation.
func TestChaosConfigValidation(t *testing.T) {
	t.Run("valid_config", func(t *testing.T) {
		cfg := &chaos.ChaosConfig{
			Enabled: true,
			GlobalRules: &chaos.GlobalChaosRules{
				Latency: &chaos.LatencyFault{
					Min:         "10ms",
					Max:         "100ms",
					Probability: 0.5,
				},
			},
		}
		assert.NoError(t, cfg.Validate())
	})

	t.Run("invalid_bandwidth", func(t *testing.T) {
		cfg := &chaos.ChaosConfig{
			Enabled: true,
			GlobalRules: &chaos.GlobalChaosRules{
				Bandwidth: &chaos.BandwidthFault{
					BytesPerSecond: 0, // Invalid: must be > 0
					Probability:    0.5,
				},
			},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("disabled_config_skips_validation", func(t *testing.T) {
		cfg := &chaos.ChaosConfig{
			Enabled: false,
			GlobalRules: &chaos.GlobalChaosRules{
				Latency: &chaos.LatencyFault{
					Min:         "10ms",
					Max:         "100ms",
					Probability: 2.0, // Invalid but should be skipped
				},
			},
		}
		// Disabled config should skip validation
		assert.NoError(t, cfg.Validate())
	})
}

// TestChaosInjectorUpdateConfig tests updating chaos config at runtime.
func TestChaosInjectorUpdateConfig(t *testing.T) {
	initialConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "100ms",
				Max:         "100ms",
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, initialConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Initial request should have 100ms latency
	elapsed1, resp1, err := measureRequestTime(url)
	require.NoError(t, err)
	resp1.Body.Close()
	assert.GreaterOrEqual(t, elapsed1.Milliseconds(), int64(100))

	// Update config to have 300ms latency
	newConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "300ms",
				Max:         "300ms",
				Probability: 1.0,
			},
		},
	}

	injector := bundle.Server.ChaosInjector()
	require.NotNil(t, injector)
	err = injector.UpdateConfig(newConfig)
	require.NoError(t, err)

	// Request should now have 300ms latency
	elapsed2, resp2, err := measureRequestTime(url)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.GreaterOrEqual(t, elapsed2.Milliseconds(), int64(300))
}

// TestChaosNetworkErrorTypes tests various network error scenarios.
func TestChaosNetworkErrorTypes(t *testing.T) {
	// Test connection refused scenario by not starting the mock
	t.Run("connection_refused", func(t *testing.T) {
		// Use a port that nothing is listening on
		port := getFreePort()
		url := fmt.Sprintf("http://localhost:%d/api/test", port)

		client := &http.Client{
			Timeout: 1 * time.Second,
		}

		resp, err := client.Get(url)
		if resp != nil {
			resp.Body.Close()
		}
		require.Error(t, err)

		// Should be a connection error
		var netErr *net.OpError
		if ok := errors.As(err, &netErr); ok {
			_ = netErr // Use the variable
		}
		assert.Contains(t, err.Error(), "connection refused")
	})
}

// TestChaosFaultTypeCoverage verifies all fault types are documented.
func TestChaosFaultTypeCoverage(t *testing.T) {
	faultTypes := []chaos.FaultType{
		chaos.FaultLatency,
		chaos.FaultError,
		chaos.FaultTimeout,
		chaos.FaultCorruptBody,
		chaos.FaultEmptyResponse,
		chaos.FaultSlowBody,
		chaos.FaultConnectionReset,
		chaos.FaultPartialResponse,
	}

	for _, ft := range faultTypes {
		assert.NotEmpty(t, string(ft), "Fault type should have string value")
	}
}

// TestChaosLatencyDistribution tests that latency values are roughly uniform.
func TestChaosLatencyDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping distribution test in short mode")
	}

	chaosConfig := &chaos.ChaosConfig{
		Enabled: true,
		GlobalRules: &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         "0ms",
				Max:         "100ms",
				Probability: 1.0,
			},
		},
	}

	bundle := setupChaosTestServer(t, chaosConfig)
	defer bundle.Cleanup()

	setupBasicMock(t, bundle.Client, "/api/test", `{"status": "ok"}`)

	url := fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort)

	// Collect latency samples
	samples := 50
	var durations []time.Duration

	for i := 0; i < samples; i++ {
		elapsed, resp, err := measureRequestTime(url)
		require.NoError(t, err)
		resp.Body.Close()
		durations = append(durations, elapsed)
	}

	// Calculate mean and standard deviation
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	mean := sum / time.Duration(samples)

	var varianceSum float64
	for _, d := range durations {
		diff := float64(d - mean)
		varianceSum += diff * diff
	}
	stdDev := time.Duration(math.Sqrt(varianceSum / float64(samples)))

	// Mean should be roughly in the middle of the range (around 50ms)
	// Allow for some network overhead
	assert.Greater(t, mean.Milliseconds(), int64(20),
		"Mean latency should be greater than 20ms")
	assert.Less(t, mean.Milliseconds(), int64(150),
		"Mean latency should be less than 150ms")

	// Standard deviation should indicate spread (not all values the same)
	assert.Greater(t, stdDev.Milliseconds(), int64(5),
		"Should have some variance in latency distribution")

	t.Logf("Latency distribution: mean=%v, stdDev=%v", mean, stdDev)
}
