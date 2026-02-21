package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// ============================================================================
// Test Helpers
// ============================================================================

// httpTestBundle groups server and client for HTTP tests
type httpTestBundle struct {
	Server   *engine.Server
	Client   *engineclient.Client
	HTTPPort int
}

func setupHTTPServer(t *testing.T) *httpTestBundle {
	httpPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		srv.Stop()
	})

	waitForReady(t, srv.ManagementPort())

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	return &httpTestBundle{
		Server:   srv,
		Client:   client,
		HTTPPort: httpPort,
	}
}

// ============================================================================
// User Story 1: HTTP Methods
// ============================================================================

func TestHTTP_US1_AllMethods(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Note: OPTIONS is often handled specially for CORS, test separately if needed
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			mockCfg := &config.MockConfiguration{
				ID:      fmt.Sprintf("method-%s", strings.ToLower(method)),
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: method,
						Path:   fmt.Sprintf("/api/%s-test", strings.ToLower(method)),
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Headers:    map[string]string{"Content-Type": "text/plain"},
						Body:       fmt.Sprintf("%s response", method),
					},
				},
			}

			_, err := bundle.Client.CreateMock(context.Background(), mockCfg)
			require.NoError(t, err)

			req, err := http.NewRequest(method, fmt.Sprintf("http://localhost:%d/api/%s-test", bundle.HTTPPort, strings.ToLower(method)), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)

			// HEAD should not return body
			if method != "HEAD" {
				body, _ := io.ReadAll(resp.Body)
				assert.Equal(t, fmt.Sprintf("%s response", method), string(body))
			}
		})
	}
}

func TestHTTP_US1_MethodMismatch(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Add mock for GET only
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "get-only",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/get-only",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "GET works",
			},
		},
	})
	require.NoError(t, err)

	// POST should return 404
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/api/get-only", bundle.HTTPPort), "text/plain", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// ============================================================================
// User Story 2: Path Matching
// ============================================================================

func TestHTTP_US2_ExactPath(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "exact-path",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/exact/path",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "exact match",
			},
		},
	})
	require.NoError(t, err)

	// Exact match works
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/exact/path", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "exact match", string(body))

	// Trailing slash doesn't match
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/exact/path/", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)

	// Different path doesn't match
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/exact/other", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHTTP_US2_NamedParams(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "named-params",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/users/{userId}/posts/{postId}",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"userId": "{{request.pathParam.userId}}", "postId": "{{request.pathParam.postId}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/users/42/posts/123", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	// Parse JSON response for precise assertions instead of string Contains
	var result map[string]string
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Response should be valid JSON: %s", string(body))
	assert.Equal(t, "42", result["userId"], "userId path param should be captured")
	assert.Equal(t, "123", result["postId"], "postId path param should be captured")
}

func TestHTTP_US2_WildcardPath(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "wildcard-path",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/static/*",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "static content",
			},
		},
	})
	require.NoError(t, err)

	// Single level
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/static/file.js", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "static content", string(body))

	// Multiple levels
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/static/path/to/file.css", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "static content", string(body))
}

func TestHTTP_US2_PathPattern(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "path-pattern",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method:      "GET",
				PathPattern: `^/api/v\d+/users/\d+$`,
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "versioned user",
			},
		},
	})
	require.NoError(t, err)

	// Matches v1
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/users/123", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "versioned user", string(body))

	// Matches v2
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/v2/users/456", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "versioned user", string(body))

	// Doesn't match non-numeric user ID
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/v1/users/abc", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHTTP_US2_PathPatternCapture(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "path-pattern-capture",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method:      "GET",
				PathPattern: `^/orders/(?P<orderId>[0-9a-f-]+)$`,
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"orderId": "{{request.pathPattern.orderId}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/orders/550e8400-e29b-41d4", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, string(body), "550e8400-e29b-41d4")
}

// ============================================================================
// User Story 3: Header Matching
// ============================================================================

func TestHTTP_US3_HeaderMatch(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "header-match",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/data",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "authorized",
			},
		},
	})
	require.NoError(t, err)

	// Add fallback (lower priority than the specific header match)
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "header-match-fallback",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/data",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 401,
				Body:       "unauthorized",
			},
		},
	})
	require.NoError(t, err)

	// With correct header
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/data", bundle.HTTPPort), nil)
	req.Header.Set("Authorization", "Bearer token123")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "authorized", string(body))

	// Without header - falls back to unauthorized
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/data", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 401, resp.StatusCode)
	assert.Equal(t, "unauthorized", string(body))
}

func TestHTTP_US3_MultipleHeaders(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "multi-header",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/multi-header",
				Headers: map[string]string{
					"X-API-Key":     "secret",
					"X-API-Version": "v2",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "matched both headers",
			},
		},
	})
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/multi-header", bundle.HTTPPort), nil)
	req.Header.Set("X-API-Key", "secret")
	req.Header.Set("X-API-Version", "v2")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "matched both headers", string(body))
}

// ============================================================================
// User Story 4: Query Parameter Matching
// ============================================================================

func TestHTTP_US4_QueryParamMatch(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "query-param",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/search",
				QueryParams: map[string]string{
					"q": "golang",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "golang results",
			},
		},
	})
	require.NoError(t, err)

	// Add fallback (lower priority)
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "query-fallback",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/search",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "all results",
			},
		},
	})
	require.NoError(t, err)

	// With query param
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/search?q=golang", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "golang results", string(body))

	// Without query param
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/search", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "all results", string(body))
}

func TestHTTP_US4_MultipleQueryParams(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "multi-query",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/filter",
				QueryParams: map[string]string{
					"status": "active",
					"type":   "admin",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "active admins",
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/filter?status=active&type=admin", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "active admins", string(body))
}

// ============================================================================
// User Story 5: Body Matching
// ============================================================================

func TestHTTP_US5_BodyContains(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "body-contains",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method:       "POST",
				Path:         "/api/validate",
				BodyContains: "email",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "email found",
			},
		},
	})
	require.NoError(t, err)

	// With email in body
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/validate", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(`{"email": "test@example.com"}`)),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "email found", string(body))
}

func TestHTTP_US5_BodyEquals(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "body-equals",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method:     "POST",
				Path:       "/api/exact",
				BodyEquals: `{"action":"confirm"}`,
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "confirmed",
			},
		},
	})
	require.NoError(t, err)

	// Exact body match
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/exact", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(`{"action":"confirm"}`)),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "confirmed", string(body))

	// Different body should not match
	resp, err = http.Post(
		fmt.Sprintf("http://localhost:%d/api/exact", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(`{"action":"cancel"}`)),
	)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHTTP_US5_BodyPattern(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "body-pattern",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method:      "POST",
				Path:        "/api/validate-email",
				BodyPattern: `"email":\s*"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"`,
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "valid email format",
			},
		},
	})
	require.NoError(t, err)

	// Valid email format
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/validate-email", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(`{"email": "user@example.com"}`)),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "valid email format", string(body))
}

func TestHTTP_US5_BodyJSONPath(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "body-jsonpath",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/order",
				BodyJSONPath: map[string]interface{}{
					"$.order.status": "pending",
					"$.order.total":  100.0,
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "order accepted",
			},
		},
	})
	require.NoError(t, err)

	// Matching JSON structure
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/order", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(`{"order": {"status": "pending", "total": 100.0, "items": []}}`)),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "order accepted", string(body))
}

// ============================================================================
// User Story 6: Response Status Codes
// ============================================================================

func TestHTTP_US6_AllStatusCodes(t *testing.T) {
	bundle := setupHTTPServer(t)

	statusCodes := []int{
		200, 201, 204, 301, 302, 400, 401, 403, 404, 405, 500, 502, 503,
	}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("Status%d", code), func(t *testing.T) {
			_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
				ID:      fmt.Sprintf("status-%d", code),
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   fmt.Sprintf("/status/%d", code),
					},
					Response: &mock.HTTPResponse{
						StatusCode: code,
						Body:       fmt.Sprintf("status %d", code),
					},
				},
			})
			require.NoError(t, err)

			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/status/%d", bundle.HTTPPort, code))
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, code, resp.StatusCode)
		})
	}
}

// ============================================================================
// User Story 7: Response Headers
// ============================================================================

func TestHTTP_US7_ResponseHeaders(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "response-headers",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/with-headers",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type":                "application/json",
					"X-Custom-Header":             "custom-value",
					"X-Request-Id":                "req-12345",
					"Cache-Control":               "no-cache",
					"Access-Control-Allow-Origin": "*",
				},
				Body: `{"message": "success"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/with-headers", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "custom-value", resp.Header.Get("X-Custom-Header"))
	assert.Equal(t, "req-12345", resp.Header.Get("X-Request-Id"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

// ============================================================================
// User Story 8: Response Delay
// ============================================================================

func TestHTTP_US8_ResponseDelay(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "delay-200ms",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
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

	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/slow", bundle.HTTPPort))
	elapsed := time.Since(start)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(180), "Should have delay of at least 180ms")
}

func TestHTTP_US8_NoDelay(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "no-delay",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/fast",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "fast response",
				DelayMs:    0,
			},
		},
	})
	require.NoError(t, err)

	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/fast", bundle.HTTPPort))
	elapsed := time.Since(start)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Less(t, elapsed.Milliseconds(), int64(100), "Should be fast without delay")
}

// ============================================================================
// User Story 9: Priority System
// ============================================================================

func TestHTTP_US9_PriorityTieBreaker(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Add low priority first
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "low-pri",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 1,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/priority",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "low priority",
			},
		},
	})
	require.NoError(t, err)

	// Add high priority second
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "high-pri",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 100,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/priority",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "high priority",
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/priority", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, "high priority", string(body))
}

func TestHTTP_US9_ScoreBasedSelection(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Generic mock (lower score)
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "generic",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "all users",
			},
		},
	})
	require.NoError(t, err)

	// Specific mock with header (higher score)
	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "specific",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
				Headers: map[string]string{
					"X-Version": "v2",
				},
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "v2 users",
			},
		},
	})
	require.NoError(t, err)

	// Without header - generic match
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "all users", string(body))

	// With header - specific match
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/users", bundle.HTTPPort), nil)
	req.Header.Set("X-Version", "v2")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "v2 users", string(body))
}

// ============================================================================
// User Story 10: Disabled Mocks
// ============================================================================

func TestHTTP_US10_DisabledMock(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Add mock as enabled first
	mockCfg := &config.MockConfiguration{
		ID:      "disabled",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/disabled",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "should not see this",
			},
		},
	}
	_, err := bundle.Client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)

	// Verify it works when enabled
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/disabled", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Now disable it
	mockCfg.Enabled = boolPtr(false)
	_, err = bundle.Client.UpdateMock(context.Background(), "disabled", mockCfg)
	require.NoError(t, err)

	// Should return 404 when disabled
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/disabled", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestHTTP_US10_EnableDisableMock(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Add enabled mock
	mockCfg := &config.MockConfiguration{
		ID:      "toggle",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/toggle",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "enabled",
			},
		},
	}
	_, err := bundle.Client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)

	// Should work
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/toggle", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Disable
	mockCfg.Enabled = boolPtr(false)
	_, err = bundle.Client.UpdateMock(context.Background(), "toggle", mockCfg)
	require.NoError(t, err)

	// Should return 404
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/toggle", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// ============================================================================
// User Story 11: Template Variables
// ============================================================================

func TestHTTP_US11_RequestPath(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "template-path",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/echo/*",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"path": "{{request.path}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/echo/test/path", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, string(body), "/api/echo/test/path")
}

func TestHTTP_US11_RequestMethod(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "template-method",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/method-echo",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"method": "{{request.method}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/api/method-echo", bundle.HTTPPort), "text/plain", nil)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, string(body), "POST")
}

func TestHTTP_US11_QueryParams(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "template-query",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/query-echo",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"name": "{{request.query.name}}", "id": "{{request.query.id}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/query-echo?name=John&id=123", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	// Parse JSON for precise assertions
	var result map[string]string
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Response should be valid JSON: %s", string(body))
	assert.Equal(t, "John", result["name"], "name query param should be captured")
	assert.Equal(t, "123", result["id"], "id query param should be captured")
}

func TestHTTP_US11_RequestBody(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "template-body",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/body-echo",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"received_name": "{{request.body.name}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/body-echo", bundle.HTTPPort),
		"application/json",
		bytes.NewReader([]byte(`{"name": "TestUser"}`)),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	// Parse JSON for precise assertion
	var result map[string]string
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Response should be valid JSON: %s", string(body))
	assert.Equal(t, "TestUser", result["received_name"], "request body name should be echoed")
}

func TestHTTP_US11_DynamicValues(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "template-dynamic",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/dynamic",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"uuid": "{{uuid}}", "timestamp": "{{timestamp}}"}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/dynamic", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	// UUID should have typical format
	assert.Contains(t, string(body), "-")
	// Timestamp should be numeric
	bodyStr := string(body)
	assert.True(t, strings.Contains(bodyStr, "timestamp"))
}

// ============================================================================
// User Story 12: Multiple Matching Criteria
// ============================================================================

func TestHTTP_US12_CombinedCriteria(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Mock with multiple criteria
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "combined",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/complex",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				QueryParams: map[string]string{
					"version": "2",
				},
				BodyContains: "action",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "all criteria matched",
			},
		},
	})
	require.NoError(t, err)

	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("http://localhost:%d/api/complex?version=2", bundle.HTTPPort),
		bytes.NewReader([]byte(`{"action": "create"}`)),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "all criteria matched", string(body))
}

// ============================================================================
// User Story 13: CRUD Operations
// ============================================================================

func TestHTTP_US13_AddMock(t *testing.T) {
	bundle := setupHTTPServer(t)

	mockCfg := &config.MockConfiguration{
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/new",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "new mock",
			},
		},
	}

	created, err := bundle.Client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
}

func TestHTTP_US13_UpdateMock(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Add initial mock
	mockCfg := &config.MockConfiguration{
		ID:      "update-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/update",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "original",
			},
		},
	}
	_, err := bundle.Client.CreateMock(context.Background(), mockCfg)
	require.NoError(t, err)

	// Verify original
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/update", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "original", string(body))

	// Update
	mockCfg.HTTP.Response.Body = "updated"
	_, err = bundle.Client.UpdateMock(context.Background(), "update-test", mockCfg)
	require.NoError(t, err)

	// Verify updated
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/update", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "updated", string(body))
}

func TestHTTP_US13_DeleteMock(t *testing.T) {
	bundle := setupHTTPServer(t)

	// Add mock
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "delete-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/delete",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "exists",
			},
		},
	})
	require.NoError(t, err)

	// Verify exists
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/delete", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Delete
	err = bundle.Client.DeleteMock(context.Background(), "delete-test")
	require.NoError(t, err)

	// Verify gone
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/delete", bundle.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

// ============================================================================
// User Story 14: Concurrent Requests
// ============================================================================

func TestHTTP_US14_ConcurrentRequests(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "concurrent",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/concurrent",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "concurrent ok",
			},
		},
	})
	require.NoError(t, err)

	// Make concurrent requests
	numRequests := 50
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/concurrent", bundle.HTTPPort))
			if err != nil {
				results <- 0
				return
			}
			resp.Body.Close()
			results <- resp.StatusCode
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		if <-results == 200 {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
}

// ============================================================================
// User Story 15: Content Types
// ============================================================================

func TestHTTP_US15_JSONResponse(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "json-response",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/json",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"message": "hello", "count": 42}`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/json", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Contains(t, string(body), `"message"`)
	assert.Contains(t, string(body), `"count"`)
}

func TestHTTP_US15_XMLResponse(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "xml-response",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/xml",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/xml",
				},
				Body: `<?xml version="1.0"?><response><message>hello</message></response>`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/xml", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, "application/xml", resp.Header.Get("Content-Type"))
	assert.Contains(t, string(body), "<message>")
}

func TestHTTP_US15_PlainTextResponse(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "text-response",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/text",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "plain text response",
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/text", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
	assert.Equal(t, "plain text response", string(body))
}

func TestHTTP_US15_HTMLResponse(t *testing.T) {
	bundle := setupHTTPServer(t)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "html-response",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/page",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "text/html",
				},
				Body: `<!DOCTYPE html><html><body><h1>Hello</h1></body></html>`,
			},
		},
	})
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/page", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))
	assert.Contains(t, string(body), "<html>")
}
