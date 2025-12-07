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

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// T135: Admin API latency benchmark verifying <100ms
func TestAdminAPILatency(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

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

	// Test create mock latency
	t.Run("Create mock endpoint", func(t *testing.T) {
		mock := config.MockConfiguration{
			ID:      "latency-test",
			Enabled: true,
			Matcher: &config.RequestMatcher{
				Method: "GET",
				Path:   "/latency",
			},
			Response: &config.ResponseDefinition{
				StatusCode: 200,
				Body:       "ok",
			},
		}
		body, _ := json.Marshal(mock)

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

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	srv.Start()
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	adminAPI.Start()
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

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

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add some mocks
	for i := 0; i < 50; i++ {
		srv.AddMock(&config.MockConfiguration{
			ID:      fmt.Sprintf("mock-%d", i),
			Enabled: true,
			Matcher: &config.RequestMatcher{
				Path: fmt.Sprintf("/api/%d", i),
			},
			Response: &config.ResponseDefinition{
				StatusCode: 200,
				Body:       "ok",
			},
		})
	}

	srv.Start()
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	adminAPI.Start()
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

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

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

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
