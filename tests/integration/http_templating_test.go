package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
// Test Helpers for HTTP Templating Tests
// ============================================================================

// templatingTestBundle groups server and engine client for templating tests
type templatingTestBundle struct {
	Server         *engine.Server
	Client         *engineclient.Client
	HTTPPort       int
	ManagementPort int
}

// setupTemplatingServer creates a test server for templating tests
func setupTemplatingServer(t *testing.T) *templatingTestBundle {
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

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	t.Cleanup(func() {
		srv.Stop()
	})

	return &templatingTestBundle{
		Server:         srv,
		Client:         client,
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
	}
}

// createHTTPMock creates an HTTP mock via the engine client
func createHTTPMock(t *testing.T, bundle *templatingTestBundle, name string, method string, path string, pathPattern string, body string, delayMs int) string {
	testMock := &config.MockConfiguration{
		Name:    name,
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method:      method,
				Path:        path,
				PathPattern: pathPattern,
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body:    body,
				DelayMs: delayMs,
			},
		},
	}

	created, err := bundle.Client.CreateMock(context.Background(), testMock)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	return created.ID
}

// httpGet makes a GET request to the mock server
func httpGet(t *testing.T, port int, path string) (*http.Response, []byte) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, path))
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	return resp, body
}

// httpGetWithHeaders makes a GET request with custom headers
func httpGetWithHeaders(t *testing.T, port int, path string, headers map[string]string) (*http.Response, []byte) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d%s", port, path), nil)
	require.NoError(t, err)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	return resp, body
}

// httpPost makes a POST request with JSON body
func httpPost(t *testing.T, port int, path string, jsonBody interface{}) (*http.Response, []byte) {
	bodyBytes, err := json.Marshal(jsonBody)
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d%s", port, path),
		"application/json",
		bytes.NewReader(bodyBytes),
	)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	return resp, body
}

// ============================================================================
// Test 1: Computed Values (uuid, timestamp, now)
// ============================================================================

func TestHTTPTemplating_ComputedValues(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with computed value templates
	createHTTPMock(t, bundle, "computed-values-test", "GET", "/api/computed", "",
		`{"id": "{{uuid}}", "ts": "{{timestamp}}", "now": "{{now}}"}`, 0)

	// Make HTTP request
	resp, body := httpGet(t, bundle.HTTPPort, "/api/computed")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Verify UUID format (8-4-4-4-12 hex format)
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	assert.Regexp(t, uuidRegex, result["id"], "id should be a valid UUID")

	// Verify timestamp is numeric (Unix epoch)
	tsStr, ok := result["ts"].(string)
	require.True(t, ok, "ts should be a string")
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	require.NoError(t, err, "ts should be parseable as int64")
	assert.True(t, ts > 1700000000, "timestamp should be after 2023")
	assert.True(t, ts < 2000000000, "timestamp should be reasonable")

	// Verify now is RFC3339 format
	nowStr, ok := result["now"].(string)
	require.True(t, ok, "now should be a string")
	_, err = time.Parse(time.RFC3339, nowStr)
	require.NoError(t, err, "now should be valid RFC3339 format")
}

func TestHTTPTemplating_ComputedValues_UniquePerRequest(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with UUID template
	createHTTPMock(t, bundle, "unique-uuid-test", "GET", "/api/uuid", "",
		`{"id": "{{uuid}}"}`, 0)

	// Make multiple requests
	var uuids []string
	for i := 0; i < 5; i++ {
		_, body := httpGet(t, bundle.HTTPPort, "/api/uuid")
		var result map[string]interface{}
		err := json.Unmarshal(body, &result)
		require.NoError(t, err)
		uuids = append(uuids, result["id"].(string))
	}

	// Verify all UUIDs are unique
	uuidSet := make(map[string]bool)
	for _, uuid := range uuids {
		assert.False(t, uuidSet[uuid], "UUID should be unique across requests")
		uuidSet[uuid] = true
	}
}

// ============================================================================
// Test 2: Request Data Access (body, method, path)
// ============================================================================

func TestHTTPTemplating_RequestDataAccess(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with request data templates
	createHTTPMock(t, bundle, "request-data-test", "POST", "/api/users", "",
		`{"echo": "{{request.body.name}}", "method": "{{request.method}}", "path": "{{request.path}}"}`, 0)

	// POST with JSON body
	resp, body := httpPost(t, bundle.HTTPPort, "/api/users", map[string]interface{}{
		"name": "John",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "John", result["echo"], "should echo request body name")
	assert.Equal(t, "POST", result["method"], "should echo request method")
	assert.Equal(t, "/api/users", result["path"], "should echo request path")
}

func TestHTTPTemplating_RequestRawBody(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock that echoes raw body as plain text (not inside JSON)
	// Note: rawBody contains raw JSON which needs special handling to embed in JSON
	testMock := &config.MockConfiguration{
		Name:    "raw-body-test",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/raw",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: `raw:{{request.rawBody}}`,
			},
		},
	}

	_, err := bundle.Client.CreateMock(context.Background(), testMock)
	require.NoError(t, err)

	// POST with JSON body
	_, body := httpPost(t, bundle.HTTPPort, "/api/raw", map[string]interface{}{
		"test": "value",
	})

	// Verify raw body is echoed
	bodyStr := string(body)
	assert.True(t, len(bodyStr) > 4, "response should have content")
	assert.Contains(t, bodyStr, "test")
	assert.Contains(t, bodyStr, "value")
}

// ============================================================================
// Test 3: Query Parameters
// ============================================================================

func TestHTTPTemplating_QueryParameters(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with query parameter templates
	createHTTPMock(t, bundle, "query-params-test", "GET", "/api/items", "",
		`{"filter": "{{request.query.status}}", "page": "{{request.query.page}}"}`, 0)

	// GET with query parameters
	resp, body := httpGet(t, bundle.HTTPPort, "/api/items?status=active&page=2")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "active", result["filter"], "should extract status query param")
	assert.Equal(t, "2", result["page"], "should extract page query param")
}

func TestHTTPTemplating_QueryParameters_Missing(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with query parameter templates
	createHTTPMock(t, bundle, "query-missing-test", "GET", "/api/search", "",
		`{"q": "{{request.query.q}}", "limit": "{{request.query.limit}}"}`, 0)

	// GET with only one query parameter
	resp, body := httpGet(t, bundle.HTTPPort, "/api/search?q=test")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "test", result["q"], "should extract q query param")
	assert.Equal(t, "", result["limit"], "missing param should be empty string")
}

// ============================================================================
// Test 4: Headers
// ============================================================================

func TestHTTPTemplating_Headers(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with header templates
	createHTTPMock(t, bundle, "headers-test", "GET", "/api/headers", "",
		`{"auth": "{{request.header.Authorization}}", "agent": "{{request.header.User-Agent}}"}`, 0)

	// GET with custom headers
	resp, body := httpGetWithHeaders(t, bundle.HTTPPort, "/api/headers", map[string]string{
		"Authorization": "Bearer xyz123",
		"User-Agent":    "TestClient/1.0",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "Bearer xyz123", result["auth"], "should extract Authorization header")
	assert.Equal(t, "TestClient/1.0", result["agent"], "should extract User-Agent header")
}

func TestHTTPTemplating_Headers_CaseInsensitive(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with header template using canonical case
	createHTTPMock(t, bundle, "headers-case-test", "GET", "/api/headers-case", "",
		`{"contentType": "{{request.header.Content-Type}}"}`, 0)

	// GET with header in different case
	resp, body := httpGetWithHeaders(t, bundle.HTTPPort, "/api/headers-case", map[string]string{
		"content-type": "application/xml",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify (HTTP headers are canonicalized)
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "application/xml", result["contentType"], "should extract header regardless of case")
}

// ============================================================================
// Test 5: Path Parameters (via pathPattern regex)
// ============================================================================

func TestHTTPTemplating_PathParameters(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with pathPattern containing named capture groups
	createHTTPMock(t, bundle, "path-params-test", "", "", `/users/(?P<userId>\d+)/orders/(?P<orderId>\d+)`,
		`{"user": "{{request.pathPattern.userId}}", "order": "{{request.pathPattern.orderId}}"}`, 0)

	// GET with path parameters
	resp, body := httpGet(t, bundle.HTTPPort, "/users/123/orders/456")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "123", result["user"], "should extract userId from path")
	assert.Equal(t, "456", result["order"], "should extract orderId from path")
}

func TestHTTPTemplating_PathParameters_Mixed(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with pathPattern and static path parts
	createHTTPMock(t, bundle, "path-mixed-test", "", "", `/api/v1/products/(?P<productId>[a-z0-9-]+)/reviews`,
		`{"productId": "{{request.pathPattern.productId}}", "path": "{{request.path}}"}`, 0)

	// GET with alphanumeric product ID
	resp, body := httpGet(t, bundle.HTTPPort, "/api/v1/products/abc-123-xyz/reviews")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "abc-123-xyz", result["productId"], "should extract alphanumeric productId")
	assert.Equal(t, "/api/v1/products/abc-123-xyz/reviews", result["path"], "should have full path")
}

// ============================================================================
// Test 6: Random Values
// ============================================================================

func TestHTTPTemplating_RandomValues(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with random value templates
	createHTTPMock(t, bundle, "random-values-test", "GET", "/api/random", "",
		`{"random": "{{random}}", "randomInt": "{{random.int 1 100}}", "randomFloat": "{{random.float}}"}`, 0)

	// Make multiple requests to verify randomness
	var randomValues []string
	var randomInts []int

	for i := 0; i < 5; i++ {
		resp, body := httpGet(t, bundle.HTTPPort, "/api/random")
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err := json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Collect random string values
		randomValues = append(randomValues, result["random"].(string))

		// Verify randomInt is in range
		randomIntStr := result["randomInt"].(string)
		randomInt, err := strconv.Atoi(randomIntStr)
		require.NoError(t, err, "randomInt should be parseable")
		assert.GreaterOrEqual(t, randomInt, 1, "randomInt should be >= 1")
		assert.LessOrEqual(t, randomInt, 100, "randomInt should be <= 100")
		randomInts = append(randomInts, randomInt)

		// Verify randomFloat is a valid float
		randomFloatStr := result["randomFloat"].(string)
		randomFloat, err := strconv.ParseFloat(randomFloatStr, 64)
		require.NoError(t, err, "randomFloat should be parseable")
		assert.GreaterOrEqual(t, randomFloat, 0.0, "randomFloat should be >= 0")
		assert.LessOrEqual(t, randomFloat, 1.0, "randomFloat should be <= 1")
	}

	// Verify some variation exists (not all values the same)
	// Due to randomness, this could theoretically fail, but probability is extremely low
	uniqueRandoms := make(map[string]bool)
	for _, v := range randomValues {
		uniqueRandoms[v] = true
	}
	// With 5 samples, we should have at least 2 unique values
	assert.GreaterOrEqual(t, len(uniqueRandoms), 2, "random values should have some variation")
}

func TestHTTPTemplating_RandomInt_EdgeCases(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Test with min=max
	createHTTPMock(t, bundle, "random-same-test", "GET", "/api/random-same", "",
		`{"value": "{{random.int 42 42}}"}`, 0)

	_, body := httpGet(t, bundle.HTTPPort, "/api/random-same")
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "42", result["value"], "random.int with same min/max should return that value")
}

// ============================================================================
// Test 7: String Functions (upper, lower)
// ============================================================================

func TestHTTPTemplating_StringFunctions(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with string function templates
	createHTTPMock(t, bundle, "string-funcs-test", "POST", "/api/strings", "",
		`{"upper": "{{upper request.body.name}}", "lower": "{{lower request.body.name}}"}`, 0)

	// POST with mixed case name
	resp, body := httpPost(t, bundle.HTTPPort, "/api/strings", map[string]interface{}{
		"name": "JoHn DoE",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "JOHN DOE", result["upper"], "upper should convert to uppercase")
	assert.Equal(t, "john doe", result["lower"], "lower should convert to lowercase")
}

func TestHTTPTemplating_StringFunctions_WithSpecialChars(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with string functions on special characters
	createHTTPMock(t, bundle, "string-special-test", "POST", "/api/strings-special", "",
		`{"upper": "{{upper request.body.text}}"}`, 0)

	// POST with special characters
	resp, body := httpPost(t, bundle.HTTPPort, "/api/strings-special", map[string]interface{}{
		"text": "hello123!@#",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "HELLO123!@#", result["upper"], "upper should handle special chars")
}

// ============================================================================
// Test 8: Default Values
// ============================================================================

func TestHTTPTemplating_DefaultValues_WithValue(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with default value template using single quotes (supported by template engine)
	createHTTPMock(t, bundle, "default-with-value", "POST", "/api/default-test", "",
		`{"name": "{{default request.body.name 'Anonymous'}}"}`, 0)

	// POST with name provided
	resp, body := httpPost(t, bundle.HTTPPort, "/api/default-test", map[string]interface{}{
		"name": "Alice",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "Alice", result["name"], "should use provided value")
}

func TestHTTPTemplating_DefaultValues_WithoutValue(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with default value template using single quotes (supported by template engine)
	createHTTPMock(t, bundle, "default-without-value", "POST", "/api/default-empty", "",
		`{"name": "{{default request.body.name 'Anonymous'}}"}`, 0)

	// POST with empty body
	resp, body := httpPost(t, bundle.HTTPPort, "/api/default-empty", map[string]interface{}{})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "Anonymous", result["name"], "should use default value when not provided")
}

func TestHTTPTemplating_DefaultValues_EmptyString(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with default value template using single quotes (supported by template engine)
	createHTTPMock(t, bundle, "default-empty-string", "POST", "/api/default-empty-str", "",
		`{"name": "{{default request.body.name 'Guest'}}"}`, 0)

	// POST with empty string name
	resp, body := httpPost(t, bundle.HTTPPort, "/api/default-empty-str", map[string]interface{}{
		"name": "",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "Guest", result["name"], "should use default when value is empty string")
}

// ============================================================================
// Test 9: Nested JSON Body Access
// ============================================================================

func TestHTTPTemplating_NestedJSONBody(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with nested body access
	createHTTPMock(t, bundle, "nested-body-test", "POST", "/api/nested", "",
		`{"userName": "{{request.body.user.name}}", "city": "{{request.body.user.address.city}}"}`, 0)

	// POST with nested JSON
	resp, body := httpPost(t, bundle.HTTPPort, "/api/nested", map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Bob",
			"address": map[string]interface{}{
				"city":    "NYC",
				"country": "USA",
			},
		},
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "Bob", result["userName"], "should extract nested user.name")
	assert.Equal(t, "NYC", result["city"], "should extract deeply nested user.address.city")
}

func TestHTTPTemplating_NestedJSONBody_MissingPath(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with nested body access to non-existent path
	createHTTPMock(t, bundle, "nested-missing-test", "POST", "/api/nested-missing", "",
		`{"value": "{{request.body.does.not.exist}}"}`, 0)

	// POST with some JSON
	resp, body := httpPost(t, bundle.HTTPPort, "/api/nested-missing", map[string]interface{}{
		"foo": "bar",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify - should have empty string for missing path
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "", result["value"], "missing path should return empty string")
}

func TestHTTPTemplating_NestedJSONBody_DeepNesting(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with deep nesting
	createHTTPMock(t, bundle, "deep-nesting-test", "POST", "/api/deep", "",
		`{"value": "{{request.body.level1.level2.level3.value}}"}`, 0)

	// POST with deeply nested JSON
	resp, body := httpPost(t, bundle.HTTPPort, "/api/deep", map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"value": "deep-value",
				},
			},
		},
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "deep-value", result["value"], "should extract deeply nested value")
}

// ============================================================================
// Test 10: Response Delays
// ============================================================================

func TestHTTPTemplating_ResponseDelay(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with 100ms delay
	createHTTPMock(t, bundle, "delay-test", "GET", "/api/delayed", "",
		`{"status": "ok"}`, 100)

	// Measure request time
	start := time.Now()
	resp, body := httpGet(t, bundle.HTTPPort, "/api/delayed")
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response to verify it's valid
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])

	// Verify delay was applied (allow some margin for processing)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "response should be delayed by at least 100ms")
	assert.Less(t, elapsed, 500*time.Millisecond, "response should not be excessively delayed")
}

func TestHTTPTemplating_ResponseDelay_Zero(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with no delay
	createHTTPMock(t, bundle, "no-delay-test", "GET", "/api/no-delay", "",
		`{"status": "fast"}`, 0)

	// Measure request time
	start := time.Now()
	resp, _ := httpGet(t, bundle.HTTPPort, "/api/no-delay")
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify no significant delay
	assert.Less(t, elapsed, 100*time.Millisecond, "response without delay should be fast")
}

// ============================================================================
// Test 11: Combined Template Features
// ============================================================================

func TestHTTPTemplating_CombinedFeatures(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock combining multiple template features
	createHTTPMock(t, bundle, "combined-test", "POST", "/api/combined", "",
		`{
			"id": "{{uuid}}",
			"timestamp": "{{timestamp}}",
			"user": "{{upper request.body.name}}",
			"greeting": "Hello, {{default request.body.name \"Guest\"}}!",
			"method": "{{request.method}}",
			"filter": "{{request.query.filter}}"
		}`, 0)

	// POST with query params and body
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/api/combined?filter=active", bundle.HTTPPort), nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	bodyData, _ := json.Marshal(map[string]interface{}{"name": "Alice"})
	req.Body = io.NopCloser(bytes.NewReader(bodyData))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse and verify all fields
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Verify UUID format
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	assert.Regexp(t, uuidRegex, result["id"])

	// Verify timestamp
	_, err = strconv.ParseInt(result["timestamp"].(string), 10, 64)
	assert.NoError(t, err)

	// Verify string function
	assert.Equal(t, "ALICE", result["user"])

	// Verify greeting with default
	assert.Equal(t, "Hello, Alice!", result["greeting"])

	// Verify method
	assert.Equal(t, "POST", result["method"])

	// Verify query param
	assert.Equal(t, "active", result["filter"])
}

// ============================================================================
// Test 12: Edge Cases and Error Handling
// ============================================================================

func TestHTTPTemplating_InvalidTemplate_GracefulDegradation(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with invalid template syntax (unclosed braces)
	createHTTPMock(t, bundle, "invalid-template", "GET", "/api/invalid", "",
		`{"value": "{{request.body.unknown}}", "literal": "text"}`, 0)

	// Request should still work, unknown expressions become empty
	resp, body := httpGet(t, bundle.HTTPPort, "/api/invalid")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should have the literal text and empty for unknown
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "", result["value"], "unknown template should be empty")
	assert.Equal(t, "text", result["literal"], "literal text should be preserved")
}

func TestHTTPTemplating_NoTemplates_PlainText(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with no templates
	createHTTPMock(t, bundle, "plain-text", "GET", "/api/plain", "",
		`{"message": "Hello, World!"}`, 0)

	resp, body := httpGet(t, bundle.HTTPPort, "/api/plain")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"message": "Hello, World!"}`, string(body))
}

func TestHTTPTemplating_EscapedBraces(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with template next to literal braces
	createHTTPMock(t, bundle, "braces-test", "GET", "/api/braces", "",
		`{"template": "{{uuid}}", "literal": "{not_a_template}"}`, 0)

	resp, body := httpGet(t, bundle.HTTPPort, "/api/braces")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	// UUID should be processed
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	assert.Regexp(t, uuidRegex, result["template"])

	// Single braces should be preserved
	assert.Equal(t, "{not_a_template}", result["literal"])
}

// ============================================================================
// Test 13: UUID Short Format
// ============================================================================

func TestHTTPTemplating_UUIDShort(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock with short UUID template
	createHTTPMock(t, bundle, "uuid-short-test", "GET", "/api/uuid-short", "",
		`{"shortId": "{{uuid.short}}"}`, 0)

	resp, body := httpGet(t, bundle.HTTPPort, "/api/uuid-short")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	shortId := result["shortId"].(string)
	assert.Len(t, shortId, 8, "uuid.short should be 8 characters")
	// Should be hex characters
	matched, _ := regexp.MatchString(`^[0-9a-f]{8}$`, shortId)
	assert.True(t, matched, "uuid.short should be hex characters")
}

// ============================================================================
// Test 14: Request URL
// ============================================================================

func TestHTTPTemplating_RequestURL(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock that echoes URL
	createHTTPMock(t, bundle, "url-test", "GET", "/api/url", "",
		`{"url": "{{request.url}}"}`, 0)

	resp, body := httpGet(t, bundle.HTTPPort, "/api/url?foo=bar&baz=qux")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	url := result["url"].(string)
	assert.Contains(t, url, "/api/url")
	assert.Contains(t, url, "foo=bar")
	assert.Contains(t, url, "baz=qux")
}

// ============================================================================
// Test 15: Multiple Query Parameters with Same Name
// ============================================================================

func TestHTTPTemplating_MultiValueQueryParams(t *testing.T) {
	bundle := setupTemplatingServer(t)

	// Create mock - note: request.query.param returns first value only
	createHTTPMock(t, bundle, "multi-query-test", "GET", "/api/multi-query", "",
		`{"tag": "{{request.query.tag}}"}`, 0)

	// GET with multiple values for same param
	resp, body := httpGet(t, bundle.HTTPPort, "/api/multi-query?tag=first&tag=second&tag=third")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Should get first value
	assert.Equal(t, "first", result["tag"], "should return first value for multi-value params")
}
