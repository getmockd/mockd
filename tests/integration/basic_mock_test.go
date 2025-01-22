package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// getFreePort gets a free port for testing - wrapper around shared helper
func getFreePort() int {
	return GetFreePortSafe()
}

// T035: Create mock, send request, verify response
func TestBasicMockCreationAndResponse(t *testing.T) {
	httpPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		LogRequests:    true,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)

	// Start server first to get control API running
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Create engine client for mock CRUD operations
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add a mock via HTTP client
	testMock := &config.MockConfiguration{
		Name:    "Get Users",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"users": ["Alice", "Bob"]}`,
			},
		},
	}

	created, err := client.CreateMock(context.Background(), testMock)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)

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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)

	// Start server first
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add initial mock
	initialMock := &config.MockConfiguration{
		ID:      "test-mock-1",
		Name:    "Initial Mock",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/data",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "initial response",
			},
		},
	}

	_, err = client.CreateMock(context.Background(), initialMock)
	require.NoError(t, err)

	// Verify initial response
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/data", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "initial response", string(body))

	// Update mock
	updatedMock := &config.MockConfiguration{
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/data",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "updated response",
			},
		},
	}

	_, err = client.UpdateMock(context.Background(), "test-mock-1", updatedMock)
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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)

	// Start server first
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add mock
	deleteMock := &config.MockConfiguration{
		ID:      "delete-test-mock",
		Name:    "Delete Test",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/delete-me",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "I exist",
			},
		},
	}

	_, err = client.CreateMock(context.Background(), deleteMock)
	require.NoError(t, err)

	// Verify mock exists
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/delete-me", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Delete mock
	err = client.DeleteMock(context.Background(), "delete-test-mock")
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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)

	// Start server first
	err := srv.Start()
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add mock
	restartMock := &config.MockConfiguration{
		ID:      "restart-test-mock",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/restart-test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "I exist",
			},
		},
	}

	_, err = client.CreateMock(context.Background(), restartMock)
	require.NoError(t, err)

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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)

	// Start server first
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add lower priority mock (generic)
	lowPriority := &config.MockConfiguration{
		ID:      "low-priority",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 1,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "generic users",
			},
		},
	}

	// Add higher priority mock (specific)
	highPriority := &config.MockConfiguration{
		ID:      "high-priority",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 10,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "specific users",
			},
		},
	}

	// Add in reverse order to test sorting
	_, err = client.CreateMock(context.Background(), lowPriority)
	require.NoError(t, err)
	_, err = client.CreateMock(context.Background(), highPriority)
	require.NoError(t, err)

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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)

	// Start server first
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create engine client
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	// Add wildcard mock
	wildcardMock := &config.MockConfiguration{
		ID:      "wildcard-mock",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users/*",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "user details",
			},
		},
	}

	_, err = client.CreateMock(context.Background(), wildcardMock)
	require.NoError(t, err)

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
