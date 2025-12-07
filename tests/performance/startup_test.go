package performance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// T133: Startup benchmark verifying <2s startup time
func TestStartupTime(t *testing.T) {
	cfg := &config.ServerConfiguration{
		HTTPPort:     0, // Disabled
		AdminPort:    getFreePort(),
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	start := time.Now()

	srv := engine.NewServer(cfg)
	require.NotNil(t, srv)

	err := srv.Start()
	require.NoError(t, err)

	startupTime := time.Since(start)
	srv.Stop()

	// Startup should be under 2 seconds
	assert.Less(t, startupTime, 2*time.Second, "Server startup took %v, expected <2s", startupTime)

	// Log actual startup time
	t.Logf("Server startup time: %v", startupTime)
}

func BenchmarkServerStartup(b *testing.B) {
	cfg := &config.ServerConfiguration{
		HTTPPort:     0,
		AdminPort:    0,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv := engine.NewServer(cfg)
		if srv != nil {
			// Don't actually start to avoid port exhaustion
		}
	}
}

func TestStartupWithMocks(t *testing.T) {
	cfg := &config.ServerConfiguration{
		HTTPPort:     getFreePort(),
		AdminPort:    getFreePort(),
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	// Create 100 mocks
	mocks := make([]*config.MockConfiguration, 100)
	for i := 0; i < 100; i++ {
		mocks[i] = &config.MockConfiguration{
			ID:       generateID(i),
			Priority: i,
			Enabled:  true,
			Matcher: &config.RequestMatcher{
				Method: "GET",
				Path:   generatePath(i),
			},
			Response: &config.ResponseDefinition{
				StatusCode: 200,
				Body:       "response",
			},
		}
	}

	start := time.Now()
	srv := engine.NewServerWithMocks(cfg, mocks)
	err := srv.Start()
	require.NoError(t, err)
	startupTime := time.Since(start)
	srv.Stop()

	// Even with 100 mocks, startup should be under 2 seconds
	assert.Less(t, startupTime, 2*time.Second, "Server with 100 mocks startup took %v", startupTime)
	t.Logf("Server with 100 mocks startup time: %v", startupTime)
}

func generateID(n int) string {
	return "mock-" + string(rune('a'+n%26)) + string(rune('0'+n/26%10))
}

func generatePath(n int) string {
	return "/api/resource/" + string(rune('a'+n%26)) + "/" + string(rune('0'+n%10))
}
