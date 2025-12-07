package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// T077: Multiple mocks, correct selection based on scoring
func TestMatchingMultipleMocksCorrectSelection(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add generic mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "generic-users",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "generic users list",
		},
	})

	// Add specific mock with headers
	srv.AddMock(&config.MockConfiguration{
		ID:       "v2-users",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
			Headers: map[string]string{
				"X-API-Version": "v2",
			},
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "v2 users list",
		},
	})

	// Add specific mock with query params
	srv.AddMock(&config.MockConfiguration{
		ID:       "filtered-users",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
			QueryParams: map[string]string{
				"status": "active",
			},
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "active users list",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Test 1: Generic request matches generic mock
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "generic users list", string(body))

	// Test 2: Request with header matches v2 mock (higher score)
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/users", httpPort), nil)
	req.Header.Set("X-API-Version", "v2")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "v2 users list", string(body))

	// Test 3: Request with query param matches filtered mock
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/users?status=active", httpPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "active users list", string(body))
}

// T078: Priority tie-breaking
func TestMatchingPriorityTieBreaking(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Both mocks have same matching criteria, different priorities
	srv.AddMock(&config.MockConfiguration{
		ID:       "low-priority",
		Priority: 1,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/priority-test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "low priority",
		},
	})

	srv.AddMock(&config.MockConfiguration{
		ID:       "high-priority",
		Priority: 100,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/priority-test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "high priority",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should match high priority
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/priority-test", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "high priority", string(body))
}

// Test body matching
func TestMatchingWithBody(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add mock that requires specific body content (higher priority due to more specific matching)
	srv.AddMock(&config.MockConfiguration{
		ID:       "create-user",
		Priority: 10,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method:       "POST",
			Path:         "/api/users",
			BodyContains: "email",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 201,
			Body:       "user created",
		},
	})

	// Add fallback mock (lower priority)
	srv.AddMock(&config.MockConfiguration{
		ID:       "post-users-fallback",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 400,
			Body:       "email required",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Request with email in body should match first mock
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		bytes.NewReader([]byte(`{"email": "test@example.com", "name": "Test"}`)),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 201, resp.StatusCode)
	assert.Equal(t, "user created", string(body))

	// Request without email should match fallback
	resp, err = http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		bytes.NewReader([]byte(`{"name": "Test"}`)),
	)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, "email required", string(body))
}

// Test response delay
func TestResponseDelay(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add mock with delay
	srv.AddMock(&config.MockConfiguration{
		ID:      "delayed",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/slow",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "delayed response",
			DelayMs:    200,
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Measure request time
	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/slow", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	duration := time.Since(start)

	// Should take at least 200ms
	assert.GreaterOrEqual(t, duration.Milliseconds(), int64(200))
	assert.Equal(t, 200, resp.StatusCode)
}
