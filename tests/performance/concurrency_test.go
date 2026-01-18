package performance

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T134: Concurrent request benchmark verifying 1000+ req/s
// Uses CLI-started server for realistic benchmarks.
func TestConcurrentRequests(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	require.NoError(t, err, "Failed to start test server")
	defer ts.Stop()

	// Create mock via Admin API (use unique ID to avoid conflicts)
	mockID := fmt.Sprintf("perf-test-%d", time.Now().UnixNano())
	mockConfig := map[string]interface{}{
		"id":      mockID,
		"name":    "Performance Test Mock",
		"enabled": true,
		"type":    "http",
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/test",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "ok",
			},
		},
	}
	require.NoError(t, ts.CreateMock(mockConfig))

	// Run concurrent requests
	numRequests := 1000
	numWorkers := 50

	var successCount int64
	var errorCount int64
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numRequests/numWorkers; j++ {
				resp, err := client.Get(fmt.Sprintf("http://localhost:%d/api/test", httpPort))
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == 200 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	reqPerSec := float64(successCount) / duration.Seconds()
	t.Logf("Completed %d requests in %v (%d errors)", successCount, duration, errorCount)
	t.Logf("Requests per second: %.2f", reqPerSec)

	// Should handle at least 1000 requests per second
	assert.GreaterOrEqual(t, reqPerSec, float64(1000), "Should handle >=1000 req/s, got %.2f", reqPerSec)
	assert.Zero(t, errorCount, "Should have no errors")
}

func BenchmarkConcurrentRequests(b *testing.B) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	if err != nil {
		b.Fatalf("Failed to start test server: %v", err)
	}
	defer ts.Stop()

	// Create mock via Admin API (use unique ID to avoid conflicts)
	benchMockID := fmt.Sprintf("bench-test-%d", time.Now().UnixNano())
	mockConfig := map[string]interface{}{
		"id":      benchMockID,
		"name":    "Benchmark Test Mock",
		"enabled": true,
		"type":    "http",
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/bench",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "benchmark response",
			},
		},
	}
	if err := ts.CreateMock(mockConfig); err != nil {
		b.Fatalf("Failed to create mock: %v", err)
	}

	client := &http.Client{}
	url := fmt.Sprintf("http://localhost:%d/api/bench", httpPort)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(url)
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}
	})
}

// Test handling of many concurrent connections
func TestManyConnections(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	require.NoError(t, err, "Failed to start test server")
	defer ts.Stop()

	// Create mock via Admin API (use unique ID to avoid conflicts)
	connMockID := fmt.Sprintf("conn-test-%d", time.Now().UnixNano())
	mockConfig := map[string]interface{}{
		"id":      connMockID,
		"name":    "Connection Test Mock",
		"enabled": true,
		"type":    "http",
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"path": "/",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "ok",
			},
		},
	}
	require.NoError(t, ts.CreateMock(mockConfig))

	// Open 100 connections simultaneously
	var wg sync.WaitGroup
	var successCount int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", httpPort))
			if err == nil {
				resp.Body.Close()
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(100), successCount, "All 100 connections should succeed")
}
