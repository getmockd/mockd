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

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// T134: Concurrent request benchmark verifying 1000+ req/s
func TestConcurrentRequests(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "perf-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "ok",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

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

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "bench-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/bench",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "benchmark response",
		},
	})

	srv.Start()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

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

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "conn-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "ok",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

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
