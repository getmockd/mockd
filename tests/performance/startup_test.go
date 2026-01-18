package performance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T133: Startup benchmark verifying <2s startup time
// Uses CLI binary for realistic benchmarks.
func TestStartupTime(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	start := time.Now()

	ts, err := StartTestServer(httpPort, adminPort)
	require.NoError(t, err, "Failed to start test server")

	startupTime := time.Since(start)
	ts.Stop()

	// Startup should be under 2 seconds
	assert.Less(t, startupTime, 2*time.Second, "Server startup took %v, expected <2s", startupTime)

	// Log actual startup time
	t.Logf("Server startup time: %v", startupTime)
}

// BenchmarkServerStartup measures actual server startup time via CLI.
// This is the real-world startup time users will experience.
func BenchmarkServerStartup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpPort := getFreePort()
		adminPort := getFreePort()

		ts, err := StartTestServer(httpPort, adminPort)
		if err != nil {
			b.Fatalf("Failed to start server: %v", err)
		}
		ts.Stop()
	}
}

func TestStartupWithMocks(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	ts, err := StartTestServer(httpPort, adminPort)
	require.NoError(t, err, "Failed to start test server")
	defer ts.Stop()

	// Create 100 mocks via Admin API
	start := time.Now()
	for i := 0; i < 100; i++ {
		mockConfig := map[string]interface{}{
			"id":      generateID(i),
			"name":    "Mock " + generateID(i),
			"enabled": true,
			"type":    "http",
			"http": map[string]interface{}{
				"priority": i,
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   generatePath(i),
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       "response",
				},
			},
		}
		require.NoError(t, ts.CreateMock(mockConfig), "Failed to create mock %d", i)
	}
	loadTime := time.Since(start)

	// Even with 100 mocks, loading should be reasonable
	t.Logf("Created 100 mocks in: %v", loadTime)
	assert.Less(t, loadTime, 10*time.Second, "Creating 100 mocks took %v", loadTime)
}

func generateID(n int) string {
	return "mock-" + string(rune('a'+n%26)) + string(rune('0'+n/26%10))
}

func generatePath(n int) string {
	return "/api/resource/" + string(rune('a'+n%26)) + "/" + string(rune('0'+n%10))
}
