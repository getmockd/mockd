package integration

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// getFreePort gets a free port for testing
func getFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// T035: Create mock, send request, verify response
func TestBasicMockCreationAndResponse(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		LogRequests:  true,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add a mock
	mock := &config.MockConfiguration{
		Name:    "Get Users",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"users": ["Alice", "Bob"]}`,
		},
	}

	err := srv.AddMock(mock)
	require.NoError(t, err)
	assert.NotEmpty(t, mock.ID)

	// Start server
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Make request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"users": ["Alice", "Bob"]}`, string(body))
}

// T036: Update mock, verify new response
func TestUpdateMockAndVerifyNewResponse(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add initial mock
	mock := &config.MockConfiguration{
		ID:      "test-mock-1",
		Name:    "Initial Mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/data",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "initial response",
		},
	}

	err := srv.AddMock(mock)
	require.NoError(t, err)

	// Start server
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Verify initial response
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/data", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "initial response", string(body))

	// Update mock
	updatedMock := &config.MockConfiguration{
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/data",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "updated response",
		},
	}

	err = srv.UpdateMock("test-mock-1", updatedMock)
	require.NoError(t, err)

	// Verify updated response
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/data", httpPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "updated response", string(body))
}

// T037: Delete mock, verify 404
func TestDeleteMockReturns404(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add mock
	mock := &config.MockConfiguration{
		ID:      "delete-test-mock",
		Name:    "Delete Test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/delete-me",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "I exist",
		},
	}

	err := srv.AddMock(mock)
	require.NoError(t, err)

	// Start server
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Verify mock exists
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/delete-me", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Delete mock
	err = srv.DeleteMock("delete-test-mock")
	require.NoError(t, err)

	// Verify 404
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/delete-me", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// T038: Server restart with in-memory persistence (mocks should be lost)
func TestServerRestartInMemoryPersistence(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add mock
	mock := &config.MockConfiguration{
		ID:      "restart-test-mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/restart-test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "I exist",
		},
	}

	err := srv.AddMock(mock)
	require.NoError(t, err)

	// Start server
	err = srv.Start()
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Verify mock exists
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/restart-test", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Stop server
	err = srv.Stop()
	require.NoError(t, err)

	// Create new server instance (simulating restart)
	srv2 := engine.NewServer(cfg)
	err = srv2.Start()
	require.NoError(t, err)
	defer srv2.Stop()

	time.Sleep(50 * time.Millisecond)

	// Verify mock is gone (in-memory only)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/restart-test", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// Additional test: Multiple mocks with priority
func TestMultipleMocksWithPriority(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add lower priority mock (generic)
	lowPriority := &config.MockConfiguration{
		ID:       "low-priority",
		Priority: 1,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "generic users",
		},
	}

	// Add higher priority mock (specific)
	highPriority := &config.MockConfiguration{
		ID:       "high-priority",
		Priority: 10,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "specific users",
		},
	}

	// Add in reverse order to test sorting
	err := srv.AddMock(lowPriority)
	require.NoError(t, err)
	err = srv.AddMock(highPriority)
	require.NoError(t, err)

	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should match high priority mock
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "specific users", string(body))
}

// Test wildcard path matching
func TestWildcardPathMatching(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add wildcard mock
	mock := &config.MockConfiguration{
		ID:      "wildcard-mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users/*",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "user details",
		},
	}

	err := srv.AddMock(mock)
	require.NoError(t, err)

	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should match /api/users/123
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users/123", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "user details", string(body))

	// Should match /api/users/abc/profile
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/users/abc/profile", httpPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "user details", string(body))
}
