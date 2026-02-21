package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// loggingTestBundle groups server and client for logging tests
type loggingTestBundle struct {
	Server         *engine.Server
	Client         *engineclient.Client
	AdminAPI       *admin.API
	HTTPPort       int
	AdminPort      int
	ManagementPort int
}

func setupLoggingServer(t *testing.T) *loggingTestBundle {
	httpPort := getFreePort()
	adminPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		AdminPort:      adminPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
		MaxLogEntries:  100,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	tempDir := t.TempDir() // Use temp dir for test isolation
	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(tempDir),
	)
	err = adminAPI.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		adminAPI.Stop()
		srv.Stop()
		time.Sleep(10 * time.Millisecond) // Allow file handles to release
	})

	time.Sleep(50 * time.Millisecond)

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	return &loggingTestBundle{
		Server:         srv,
		Client:         client,
		AdminAPI:       adminAPI,
		HTTPPort:       httpPort,
		AdminPort:      adminPort,
		ManagementPort: managementPort,
	}
}

// T129: Requests are logged
func TestLoggingRequestsAreLogged(t *testing.T) {
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "log-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/logged",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "logged response",
			},
		},
	})
	require.NoError(t, err)

	// Make a request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/logged", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()

	// Check logs via admin API
	logResp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(logResp.Body)
	logResp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(1), result["count"])
	requests := result["requests"].([]interface{})
	require.Len(t, requests, 1)

	log := requests[0].(map[string]interface{})
	assert.Equal(t, "GET", log["method"])
	assert.Equal(t, "/api/logged", log["path"])
	assert.Equal(t, "log-test", log["matchedMockId"])
	assert.Equal(t, float64(200), log["statusCode"])
}

// T130: Retrieve logs via API
func TestLoggingRetrieveViaAPI(t *testing.T) {
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "api-log-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/api/data",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "data",
			},
		},
	})
	require.NoError(t, err)

	// Make some requests
	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/data", bundle.HTTPPort))
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Get logs via admin API
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
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
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "users-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "users",
			},
		},
	})
	require.NoError(t, err)

	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "orders-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/orders",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 201,
				Body:       "order created",
			},
		},
	})
	require.NoError(t, err)

	// Make mixed requests
	resp1, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort))
	require.NoError(t, err)
	if resp1 != nil && resp1.Body != nil {
		resp1.Body.Close()
	}
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort))
	require.NoError(t, err)
	if resp2 != nil && resp2.Body != nil {
		resp2.Body.Close()
	}
	resp3, err := http.Post(fmt.Sprintf("http://localhost:%d/api/orders", bundle.HTTPPort), "application/json", nil)
	require.NoError(t, err)
	if resp3 != nil && resp3.Body != nil {
		resp3.Body.Close()
	}

	// Filter by method
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests?method=GET", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, float64(2), result["count"])

	// Filter by path
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests?path=/api/orders", bundle.AdminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(1), result["count"])

	// Filter by matched mock
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests?matched=users-mock", bundle.AdminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(2), result["count"])
}

// T132: Clear logs
func TestLoggingClearLogs(t *testing.T) {
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "clear-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/api/test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "ok",
			},
		},
	})
	require.NoError(t, err)

	// Make some requests
	for i := 0; i < 5; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort))
		require.NoError(t, err)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}

	// Verify 5 logs exist
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, float64(5), result["count"])

	// Clear logs via admin API
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort), nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(5), result["cleared"])

	// Verify cleared
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(0), result["count"])
}

// Test get single request log
func TestLoggingGetSingleRequest(t *testing.T) {
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "single-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/api/single",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "single",
			},
		},
	})
	require.NoError(t, err)

	// Make a request
	resp0, err := http.Get(fmt.Sprintf("http://localhost:%d/api/single", bundle.HTTPPort))
	require.NoError(t, err)
	if resp0 != nil && resp0.Body != nil {
		resp0.Body.Close()
	}

	// Get all logs to find the log ID
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var listResult map[string]interface{}
	json.Unmarshal(body, &listResult)

	logs := listResult["requests"].([]interface{})
	require.Len(t, logs, 1)
	logID := logs[0].(map[string]interface{})["id"].(string)

	// Get single log via API
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests/%s", bundle.AdminPort, logID))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	var entry struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	err = json.Unmarshal(body, &entry)
	require.NoError(t, err)

	assert.Equal(t, logID, entry.ID)
	assert.Equal(t, "/api/single", entry.Path)
}

// Test logging POST request with body
func TestLoggingPostRequestWithBody(t *testing.T) {
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "post-body-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/data",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 201,
				Body:       "created",
			},
		},
	})
	require.NoError(t, err)

	// Make POST request with body
	reqBody := `{"name": "test", "value": 123}`
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/data", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(reqBody)),
	)
	require.NoError(t, err)
	resp.Body.Close()

	// Get logs list to find the request ID
	logResp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(logResp.Body)
	logResp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	logs := result["requests"].([]interface{})
	require.Len(t, logs, 1)

	log := logs[0].(map[string]interface{})
	assert.Equal(t, "POST", log["method"])
	logID := log["id"].(string)

	// Fetch single request to get body and headers details
	detailResp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests/%s", bundle.AdminPort, logID))
	require.NoError(t, err)
	detailBody, _ := io.ReadAll(detailResp.Body)
	detailResp.Body.Close()

	var detail map[string]interface{}
	err = json.Unmarshal(detailBody, &detail)
	require.NoError(t, err)

	assert.Equal(t, reqBody, detail["body"])
	headers := detail["headers"].(map[string]interface{})
	assert.Contains(t, headers["Content-Type"], "application/json")
}

// Test logs with no match
func TestLoggingNoMatch(t *testing.T) {
	bundle := setupLoggingServer(t)
	// No mocks added

	// Make request with no matching mock
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/unknown", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)

	// Check logs - should still be logged
	logResp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(logResp.Body)
	logResp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	logs := result["requests"].([]interface{})
	require.Len(t, logs, 1)

	log := logs[0].(map[string]interface{})
	// matchedMockId is omitted when empty (omitempty), so check it's either nil or empty string
	matchedMockID, _ := log["matchedMockId"].(string)
	assert.Equal(t, "", matchedMockID)
	assert.Equal(t, float64(404), log["statusCode"])
}

// Test log limit and offset
func TestLoggingLimitAndOffset(t *testing.T) {
	bundle := setupLoggingServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "pagination-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/api/test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "ok",
			},
		},
	})
	require.NoError(t, err)

	// Make several requests
	for i := 0; i < 10; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/test", bundle.HTTPPort))
		require.NoError(t, err)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}

	// Get with limit
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/requests?limit=3", bundle.AdminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, float64(3), result["count"])
	assert.Equal(t, float64(10), result["total"])

	// Get with offset
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/requests?offset=5", bundle.AdminPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &result)
	assert.Equal(t, float64(5), result["count"])
}
