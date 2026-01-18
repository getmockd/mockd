package performance

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
)

// T135: Admin API latency benchmark verifying <100ms
// Uses CLI-started server for realistic benchmarks.
func TestAdminAPILatency(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	require.NoError(t, err, "Failed to start test server")
	defer ts.Stop()

	client := &http.Client{Timeout: 5 * time.Second}

	// Test health endpoint latency
	t.Run("Health endpoint", func(t *testing.T) {
		start := time.Now()
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", adminPort))
		latency := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		t.Logf("Health endpoint latency: %v", latency)
		assert.Less(t, latency, 100*time.Millisecond, "Health endpoint should respond in <100ms")
	})

	// Test list mocks latency
	t.Run("List mocks endpoint", func(t *testing.T) {
		start := time.Now()
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/mocks", adminPort))
		latency := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		t.Logf("List mocks latency: %v", latency)
		assert.Less(t, latency, 100*time.Millisecond, "List mocks should respond in <100ms")
	})

	// Test create mock latency (via Admin API)
	t.Run("Create mock endpoint", func(t *testing.T) {
		latencyMockID := fmt.Sprintf("latency-test-%d", time.Now().UnixNano())
		mockConfig := map[string]interface{}{
			"id":      latencyMockID,
			"name":    "Latency Test Mock",
			"enabled": true,
			"type":    "http",
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/latency",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       "ok",
				},
			},
		}
		body, _ := json.Marshal(mockConfig)

		start := time.Now()
		resp, err := client.Post(
			fmt.Sprintf("http://localhost:%d/mocks", adminPort),
			"application/json",
			bytes.NewReader(body),
		)
		latency := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		t.Logf("Create mock latency: %v", latency)
		assert.Less(t, latency, 100*time.Millisecond, "Create mock should respond in <100ms")
	})

	// Test get mock latency
	t.Run("Get mock endpoint", func(t *testing.T) {
		start := time.Now()
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/mocks/latency-test", adminPort))
		latency := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		t.Logf("Get mock latency: %v", latency)
		assert.Less(t, latency, 100*time.Millisecond, "Get mock should respond in <100ms")
	})

	// Test delete mock latency
	t.Run("Delete mock endpoint", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:%d/mocks/latency-test", adminPort), nil)

		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		t.Logf("Delete mock latency: %v", latency)
		assert.Less(t, latency, 100*time.Millisecond, "Delete mock should respond in <100ms")
	})
}

func BenchmarkAdminAPIHealth(b *testing.B) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	if err != nil {
		b.Fatalf("Failed to start test server: %v", err)
	}
	defer ts.Stop()

	client := &http.Client{}
	url := fmt.Sprintf("http://localhost:%d/health", adminPort)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
}

func BenchmarkAdminAPIListMocks(b *testing.B) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	if err != nil {
		b.Fatalf("Failed to start test server: %v", err)
	}
	defer ts.Stop()

	// Add some mocks via Admin API (use unique IDs to avoid conflicts)
	runID := time.Now().UnixNano()
	for i := 0; i < 50; i++ {
		mockConfig := map[string]interface{}{
			"id":      fmt.Sprintf("mock-%d-%d", runID, i),
			"name":    fmt.Sprintf("Mock %d", i),
			"enabled": true,
			"type":    "http",
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"path": fmt.Sprintf("/api/%d", i),
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       "ok",
				},
			},
		}
		if err := ts.CreateMock(mockConfig); err != nil {
			b.Fatalf("Failed to create mock %d: %v", i, err)
		}
	}

	client := &http.Client{}
	url := fmt.Sprintf("http://localhost:%d/mocks", adminPort)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
}

// Test average latency over multiple requests
func TestAdminAPIAverageLatency(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	require.NoError(t, err, "Failed to start test server")
	defer ts.Stop()

	client := &http.Client{Timeout: 5 * time.Second}
	numRequests := 100

	var totalLatency time.Duration

	for i := 0; i < numRequests; i++ {
		start := time.Now()
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", adminPort))
		latency := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()
		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(numRequests)
	t.Logf("Average latency over %d requests: %v", numRequests, avgLatency)

	assert.Less(t, avgLatency, 100*time.Millisecond, "Average latency should be <100ms")
}
