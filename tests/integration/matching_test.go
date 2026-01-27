package integration

import (
	"bytes"
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

// matchingTestBundle groups server and client for matching tests
type matchingTestBundle struct {
	Server   *engine.Server
	Client   *engineclient.Client
	HTTPPort int
}

func setupMatchingServer(t *testing.T) *matchingTestBundle {
	httpPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		ManagementPort:  managementPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		srv.Stop()
	})

	time.Sleep(50 * time.Millisecond)

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	return &matchingTestBundle{
		Server:   srv,
		Client:   client,
		HTTPPort: httpPort,
	}
}

// T077: Multiple mocks, correct selection based on scoring
func TestMatchingMultipleMocksCorrectSelection(t *testing.T) {
	bundle := setupMatchingServer(t)

	// Add generic mock
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "generic-users",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "generic users list",
			},
		},
	})
	require.NoError(t, err)

	// Add specific mock with headers
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "v2-users",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
				Headers: map[string]string{
					"X-API-Version": "v2",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "v2 users list",
			},
		},
	})
	require.NoError(t, err)

	// Add specific mock with query params
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "filtered-users",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
				QueryParams: map[string]string{
					"status": "active",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "active users list",
			},
		},
	})
	require.NoError(t, err)

	// Test 1: Generic request matches generic mock
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "generic users list", string(body))

	// Test 2: Request with header matches v2 mock (higher score)
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort), nil)
	req.Header.Set("X-API-Version", "v2")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "v2 users list", string(body))

	// Test 3: Request with query param matches filtered mock
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/users?status=active", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "active users list", string(body))
}

// T078: Priority tie-breaking
func TestMatchingPriorityTieBreaking(t *testing.T) {
	bundle := setupMatchingServer(t)

	// Both mocks have same matching criteria, different priorities
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "low-priority",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 1,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/priority-test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "low priority",
			},
		},
	})
	require.NoError(t, err)

	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "high-priority",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 100,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/priority-test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "high priority",
			},
		},
	})
	require.NoError(t, err)

	// Should match high priority
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/priority-test", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "high priority", string(body))
}

// Test body matching
func TestMatchingWithBody(t *testing.T) {
	bundle := setupMatchingServer(t)

	// Add mock that requires specific body content (higher priority due to more specific matching)
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "create-user",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 10,
			Matcher: &mock.HTTPMatcher{
				Method:       "POST",
				Path:         "/api/users",
				BodyContains: "email",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 201,
				Body:       "user created",
			},
		},
	})
	require.NoError(t, err)

	// Add fallback mock (lower priority)
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "post-users-fallback",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 400,
				Body:       "email required",
			},
		},
	})
	require.NoError(t, err)

	// Request with email in body should match first mock
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort),
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
		fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort),
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
	bundle := setupMatchingServer(t)

	// Add mock with delay
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "delayed",
		Enabled: boolPtr(true),
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/slow",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "delayed response",
				DelayMs:    200,
			},
		},
	})
	require.NoError(t, err)

	// Measure request time
	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/slow", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	duration := time.Since(start)

	// Should take at least 200ms
	assert.GreaterOrEqual(t, duration.Milliseconds(), int64(200))
	assert.Equal(t, 200, resp.StatusCode)
}
