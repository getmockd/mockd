package testing

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	stdtesting "testing"
	"time"
)

func TestNew(t *stdtesting.T) {
	mock := New(t)
	if mock == nil {
		t.Fatal("New() returned nil")
	}
	if mock.t != t {
		t.Error("New() did not set testing.TB")
	}
}

func TestStartAndStop(t *stdtesting.T) {
	mock := New(t)

	mock.Mock("GET", "/test").
		WithStatus(200).
		WithBody("hello").
		Reply()

	url := mock.Start()
	if url == "" {
		t.Fatal("Start() returned empty URL")
	}
	if !strings.HasPrefix(url, "http://") {
		t.Errorf("Expected URL to start with http://, got %s", url)
	}

	// Verify server is running
	resp, err := http.Get(url + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("Expected body 'hello', got %q", string(body))
	}

	mock.Stop()

	// Verify URL() returns the correct URL
	if mock.URL() != url {
		t.Errorf("URL() mismatch: expected %s, got %s", url, mock.URL())
	}
}

func TestMockWithJSON(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	type User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	mock.Mock("GET", "/users/123").
		WithStatus(200).
		WithJSON(User{ID: "123", Name: "Test User"}).
		Reply()

	url := mock.Start()

	resp, err := http.Get(url + "/users/123")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if user.ID != "123" || user.Name != "Test User" {
		t.Errorf("Unexpected user: %+v", user)
	}
}

func TestMockWithHeaders(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/api").
		WithStatus(200).
		WithHeader("X-Custom-Header", "custom-value").
		WithHeader("X-Another", "another-value").
		WithBody("ok").
		Reply()

	url := mock.Start()

	resp, err := http.Get(url + "/api")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if v := resp.Header.Get("X-Custom-Header"); v != "custom-value" {
		t.Errorf("Expected X-Custom-Header 'custom-value', got %q", v)
	}
	if v := resp.Header.Get("X-Another"); v != "another-value" {
		t.Errorf("Expected X-Another 'another-value', got %q", v)
	}
}

func TestMockWithDelay(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/slow").
		WithStatus(200).
		WithDelay("100ms").
		WithBody("delayed").
		Reply()

	url := mock.Start()

	start := time.Now()
	resp, err := http.Get(url + "/slow")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected delay of at least 100ms, got %v", elapsed)
	}
}

func TestMockWithQueryParams(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/search").
		WithQueryParam("q", "test").
		WithQueryParam("page", "1").
		WithStatus(200).
		WithBody(`{"results": []}`).
		Reply()

	url := mock.Start()

	// Request with matching query params
	resp, err := http.Get(url + "/search?q=test&page=1")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Request without matching query params should 404
	resp2, err := http.Get(url + "/search?q=other")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 404 {
		t.Errorf("Expected status 404 for non-matching query, got %d", resp2.StatusCode)
	}
}

func TestMockWithRequestHeaders(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/secure").
		WithRequestHeader("Authorization", "Bearer token123").
		WithStatus(200).
		WithBody("authorized").
		Reply()

	url := mock.Start()

	// Request with matching header
	req, _ := http.NewRequest("GET", url+"/secure", nil)
	req.Header.Set("Authorization", "Bearer token123")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Request without matching header should 404
	resp2, err := http.Get(url + "/secure")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 404 {
		t.Errorf("Expected status 404 for non-matching header, got %d", resp2.StatusCode)
	}
}

func TestAssertCalled(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/api/users").WithStatus(200).Reply()
	mock.Mock("POST", "/api/users").WithStatus(201).Reply()

	url := mock.Start()

	// Make some requests
	http.Get(url + "/api/users")
	http.Post(url+"/api/users", "application/json", strings.NewReader("{}"))

	// These should pass
	mock.AssertCalled(t, "GET", "/api/users")
	mock.AssertCalled(t, "POST", "/api/users")
}

func TestAssertCalledTimes(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/api/endpoint").WithStatus(200).Reply()

	url := mock.Start()

	// Make 3 requests
	http.Get(url + "/api/endpoint")
	http.Get(url + "/api/endpoint")
	http.Get(url + "/api/endpoint")

	mock.AssertCalledTimes(t, "GET", "/api/endpoint", 3)
}

func TestAssertNotCalled(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/called").WithStatus(200).Reply()
	mock.Mock("DELETE", "/never-called").WithStatus(204).Reply()

	url := mock.Start()

	// Only call one endpoint
	http.Get(url + "/called")

	// This should pass
	mock.AssertNotCalled(t, "DELETE", "/never-called")
}

func TestReset(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/first").WithStatus(200).Reply()
	url := mock.Start()

	http.Get(url + "/first")
	mock.AssertCalled(t, "GET", "/first")

	// Reset and add new mock
	mock.Reset()

	mock.Mock("GET", "/second").WithStatus(200).Reply()

	// First endpoint should now 404
	resp, _ := http.Get(url + "/first")
	if resp.StatusCode != 404 {
		t.Errorf("Expected 404 after reset, got %d", resp.StatusCode)
	}

	// Second endpoint should work
	resp2, _ := http.Get(url + "/second")
	if resp2.StatusCode != 200 {
		t.Errorf("Expected 200 for new mock, got %d", resp2.StatusCode)
	}

	// Verify request log was cleared - we should only see requests made after reset
	// (the 404 request to /first and the 200 request to /second)
	requests := mock.Requests()
	foundOldFirst := false
	for _, r := range requests {
		// Check if we have a request that matched a mock (not 404)
		if r.Path == "/first" && r.MatchedID != "" {
			foundOldFirst = true
		}
	}
	if foundOldFirst {
		t.Error("Expected old /first request with mock match to be cleared after reset")
	}
}

func TestRequests(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("POST", "/api/data").WithStatus(201).Reply()

	url := mock.Start()

	// Make a POST request with body and headers
	req, _ := http.NewRequest("POST", url+"/api/data", strings.NewReader(`{"name": "test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "abc123")

	client := &http.Client{}
	client.Do(req)

	requests := mock.Requests()
	if len(requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requests))
	}

	r := requests[0]
	if r.Method != "POST" {
		t.Errorf("Expected method POST, got %s", r.Method)
	}
	if r.Path != "/api/data" {
		t.Errorf("Expected path /api/data, got %s", r.Path)
	}
	if r.Body != `{"name": "test"}` {
		t.Errorf("Expected body, got %q", r.Body)
	}
}

func TestRequestLogAssertions(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("POST", "/api/items").WithStatus(201).Reply()

	url := mock.Start()

	req, _ := http.NewRequest("POST", url+"/api/items?source=test", strings.NewReader(`{"id": "123", "name": "Item"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")

	client := &http.Client{}
	client.Do(req)

	requests := mock.Requests()
	if len(requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requests))
	}

	r := requests[0]

	// Test header assertions
	r.AssertHeader(t, "Content-Type", "application/json")
	r.AssertHeaderExists(t, "Authorization")
	r.AssertHeaderContains(t, "Authorization", "Bearer")

	// Test body assertions
	r.AssertBodyContains(t, `"id"`)
	r.AssertJSONBody(t, map[string]interface{}{"id": "123", "name": "Item"})
	r.AssertJSONField(t, "id", "123")
	r.AssertJSONField(t, "name", "Item")

	// Test query param assertion
	r.AssertQueryParam(t, "source", "test")
	r.AssertQueryParamExists(t, "source")

	// Test method and path assertions
	r.AssertMethod(t, "POST")
	r.AssertPath(t, "/api/items")
}

func TestMultipleMocks(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/api/success").
		WithStatus(200).
		WithBody(`{"status": "ok"}`).
		Reply()

	mock.Mock("GET", "/api/error").
		WithStatus(500).
		WithBody(`{"error": "internal error"}`).
		Reply()

	mock.Mock("GET", "/api/notfound").
		WithStatus(404).
		WithBody(`{"error": "not found"}`).
		Reply()

	url := mock.Start()

	// Test success endpoint
	resp1, _ := http.Get(url + "/api/success")
	if resp1.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp1.StatusCode)
	}

	// Test error endpoint
	resp2, _ := http.Get(url + "/api/error")
	if resp2.StatusCode != 500 {
		t.Errorf("Expected 500, got %d", resp2.StatusCode)
	}

	// Test notfound endpoint
	resp3, _ := http.Get(url + "/api/notfound")
	if resp3.StatusCode != 404 {
		t.Errorf("Expected 404, got %d", resp3.StatusCode)
	}
}

func TestConvenienceMethods(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/not-found").RespondNotFound().Reply()
	mock.Mock("POST", "/bad-request").RespondBadRequest("invalid input").Reply()
	mock.Mock("GET", "/server-error").RespondServerError("something broke").Reply()
	mock.Mock("GET", "/unauthorized").RespondUnauthorized().Reply()
	mock.Mock("GET", "/forbidden").RespondForbidden().Reply()
	mock.Mock("POST", "/created").RespondCreated(map[string]string{"id": "new"}).Reply()
	mock.Mock("DELETE", "/no-content").RespondNoContent().Reply()
	mock.Mock("POST", "/accepted").RespondAccepted().Reply()

	url := mock.Start()

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/not-found", 404},
		{"POST", "/bad-request", 400},
		{"GET", "/server-error", 500},
		{"GET", "/unauthorized", 401},
		{"GET", "/forbidden", 403},
		{"POST", "/created", 201},
		{"DELETE", "/no-content", 204},
		{"POST", "/accepted", 202},
	}

	client := &http.Client{}
	for _, tt := range tests {
		req, _ := http.NewRequest(tt.method, url+tt.path, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("%s %s: %v", tt.method, tt.path, err)
			continue
		}
		if resp.StatusCode != tt.want {
			t.Errorf("%s %s: expected %d, got %d", tt.method, tt.path, tt.want, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestBuilderRespondWith(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/custom").
		RespondWith(201, `{"created": true}`).
		Reply()

	url := mock.Start()

	resp, _ := http.Get(url + "/custom")
	if resp.StatusCode != 201 {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestBuilderRespondJSON(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/json").
		RespondJSON(map[string]interface{}{"key": "value", "num": 42}).
		Reply()

	url := mock.Start()

	resp, _ := http.Get(url + "/json")
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if data["key"] != "value" {
		t.Errorf("Expected key='value', got %v", data["key"])
	}
}

func TestJSONField(t *stdtesting.T) {
	r := &RequestLog{
		Body: `{"user": {"name": "John", "email": "john@example.com"}, "count": 5}`,
	}

	if v := r.JSONField("user.name"); v != "John" {
		t.Errorf("Expected 'John', got %v", v)
	}

	if v := r.JSONField("user.email"); v != "john@example.com" {
		t.Errorf("Expected 'john@example.com', got %v", v)
	}

	if v := r.JSONField("count"); v != float64(5) {
		t.Errorf("Expected 5, got %v (%T)", v, v)
	}

	if v := r.JSONField("nonexistent"); v != nil {
		t.Errorf("Expected nil for nonexistent field, got %v", v)
	}
}

func TestMatchesPath(t *stdtesting.T) {
	tests := []struct {
		actual   string
		expected string
		want     bool
	}{
		{"/users/123", "/users/123", true},
		{"/users/123", "/users/{id}", true},
		{"/api/v1/users/456/posts", "/api/v1/users/{userId}/posts", true},
		{"/users/123", "/users/456", false},
		{"/users/123/extra", "/users/{id}", false},
		{"/users", "/users/{id}", false},
	}

	for _, tt := range tests {
		got := matchesPath(tt.actual, tt.expected)
		if got != tt.want {
			t.Errorf("matchesPath(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
		}
	}
}

func TestClient(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/test").WithStatus(200).Reply()

	url := mock.Start()

	client := mock.Client()
	resp, err := client.Get(url + "/test")
	if err != nil {
		t.Fatalf("Client request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestDynamicMockAddition(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	// Start with one mock
	mock.Mock("GET", "/initial").WithStatus(200).Reply()

	url := mock.Start()

	// Initial mock should work
	resp1, _ := http.Get(url + "/initial")
	if resp1.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp1.StatusCode)
	}

	// Add a new mock after start
	mock.Mock("GET", "/dynamic").WithStatus(201).Reply()

	// New mock should also work
	resp2, _ := http.Get(url + "/dynamic")
	if resp2.StatusCode != 201 {
		t.Errorf("Expected 201, got %d", resp2.StatusCode)
	}
}

func TestWithPriority(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	// Lower priority mock (default)
	mock.Mock("GET", "/api/data").
		WithStatus(200).
		WithBody("default").
		Reply()

	// Higher priority mock with header requirement
	mock.Mock("GET", "/api/data").
		WithRequestHeader("X-Special", "true").
		WithStatus(200).
		WithBody("special").
		WithPriority(10).
		Reply()

	url := mock.Start()

	// Request without header should match default
	resp1, _ := http.Get(url + "/api/data")
	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != "default" {
		t.Errorf("Expected 'default', got %q", string(body1))
	}

	// Request with header should match special
	req, _ := http.NewRequest("GET", url+"/api/data", nil)
	req.Header.Set("X-Special", "true")
	client := &http.Client{}
	resp2, _ := client.Do(req)
	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "special" {
		t.Errorf("Expected 'special', got %q", string(body2))
	}
}

func TestWithName(t *stdtesting.T) {
	mock := New(t)
	defer mock.Stop()

	mock.Mock("GET", "/named").
		WithName("My Named Mock").
		WithDescription("This is a test mock").
		WithStatus(200).
		Reply()

	_ = mock.Start()

	// Just verify it compiles and doesn't panic
	// The name and description are for debugging purposes
}
