package integration

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

// T129: Requests are logged
func TestLoggingRequestsAreLogged(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "log-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/logged",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "logged response",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make a request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/logged", httpPort))
	require.NoError(t, err)
	resp.Body.Close()

	// Check logs
	logs := srv.GetRequestLogs(nil)
	require.Len(t, logs, 1)
	assert.Equal(t, "GET", logs[0].Method)
	assert.Equal(t, "/api/logged", logs[0].Path)
	assert.Equal(t, "log-test", logs[0].MatchedMockID)
	assert.Equal(t, 200, logs[0].ResponseStatus)
}

// T130: Retrieve logs via API
func TestLoggingRetrieveViaAPI(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "api-log-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/api/data",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "data",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make some requests
	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/data", httpPort))
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Get logs via admin API
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", adminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(3), result["count"])
	assert.Equal(t, float64(3), result["total"])
}

// T131: Filter logs by criteria
func TestLoggingFilterByCriteria(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "users-mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "users",
		},
	})
	srv.AddMock(&config.MockConfiguration{
		ID:      "orders-mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/api/orders",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 201,
			Body:       "order created",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make mixed requests
	http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	http.Post(fmt.Sprintf("http://localhost:%d/api/orders", httpPort), "application/json", nil)

	// Filter by method
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests?method=GET", adminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, float64(2), result["count"])

	// Filter by path
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests?path=/api/orders", adminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(1), result["count"])

	// Filter by matched mock
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests?matched=users-mock", adminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(2), result["count"])
}

// T132: Clear logs
func TestLoggingClearLogs(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "clear-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/api/test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "ok",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make some requests
	for i := 0; i < 5; i++ {
		http.Get(fmt.Sprintf("http://localhost:%d/api/test", httpPort))
	}

	assert.Equal(t, 5, srv.RequestLogCount())

	// Clear logs via admin API
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:%d/requests", adminPort), nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, float64(5), result["cleared"])

	// Verify cleared
	assert.Equal(t, 0, srv.RequestLogCount())
}

// Test get single request log
func TestLoggingGetSingleRequest(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "single-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/api/single",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "single",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make a request
	http.Get(fmt.Sprintf("http://localhost:%d/api/single", httpPort))

	// Get the log entry ID
	logs := srv.GetRequestLogs(nil)
	require.Len(t, logs, 1)
	logID := logs[0].ID

	// Get single log via API
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests/%s", adminPort, logID))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var entry config.RequestLogEntry
	err = json.Unmarshal(body, &entry)
	require.NoError(t, err)

	assert.Equal(t, logID, entry.ID)
	assert.Equal(t, "/api/single", entry.Path)
}

// Test logging POST request with body
func TestLoggingPostRequestWithBody(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "post-body-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/api/data",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 201,
			Body:       "created",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make POST request with body
	reqBody := `{"name": "test", "value": 123}`
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/data", httpPort),
		"application/json",
		bytes.NewReader([]byte(reqBody)),
	)
	require.NoError(t, err)
	resp.Body.Close()

	// Check logs
	logs := srv.GetRequestLogs(nil)
	require.Len(t, logs, 1)
	assert.Equal(t, "POST", logs[0].Method)
	assert.Equal(t, reqBody, logs[0].Body)
	assert.Contains(t, logs[0].Headers["Content-Type"], "application/json")
}

// Test logs with no match
func TestLoggingNoMatch(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	// No mocks added

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make request with no matching mock
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/unknown", httpPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)

	// Check logs - should still be logged
	logs := srv.GetRequestLogs(nil)
	require.Len(t, logs, 1)
	assert.Equal(t, "", logs[0].MatchedMockID)
	assert.Equal(t, 404, logs[0].ResponseStatus)
}

// Test log limit and offset
func TestLoggingLimitAndOffset(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:      httpPort,
		AdminPort:     adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 100,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "pagination-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/api/test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "ok",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make several requests
	for i := 0; i < 10; i++ {
		http.Get(fmt.Sprintf("http://localhost:%d/api/test", httpPort))
	}

	// Get with limit
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests?limit=3", adminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, float64(3), result["count"])
	assert.Equal(t, float64(10), result["total"])

	// Get with offset
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests?offset=5", adminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(5), result["count"])
}
