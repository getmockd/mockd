// Package testing provides a testing SDK for using mockd in Go tests.
//
// This package makes it easy to create mock HTTP servers in your tests with a
// fluent builder API for configuring mock responses.
//
// # Basic Usage
//
// Create a mock server, configure mock endpoints, and make requests:
//
//	func TestMyAPI(t *testing.T) {
//	    mock := testing.New(t)
//	    defer mock.Stop()
//
//	    mock.Mock("GET", "/users/123").
//	        WithStatus(200).
//	        WithBody(map[string]string{"id": "123", "name": "Test User"}).
//	        Reply()
//
//	    url := mock.Start()
//
//	    // Test your code against the mock server
//	    resp, err := http.Get(url + "/users/123")
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    defer resp.Body.Close()
//
//	    // Verify the mock was called
//	    mock.AssertCalled(t, "GET", "/users/123")
//	}
//
// # Fluent Builder API
//
// The MockBuilder provides a fluent interface for configuring mock responses:
//
//	mock.Mock("POST", "/api/items").
//	    WithStatus(201).
//	    WithHeader("Content-Type", "application/json").
//	    WithBody(`{"id": "new-item", "created": true}`).
//	    WithDelay("100ms").
//	    Reply()
//
// # Request Matching
//
// Match requests by various criteria:
//
//	// Match by query parameter
//	mock.Mock("GET", "/search").
//	    WithQueryParam("q", "test").
//	    WithStatus(200).
//	    WithBody(`{"results": []}`).
//	    Reply()
//
//	// Match by header
//	mock.Mock("GET", "/api/secure").
//	    WithRequestHeader("Authorization", "Bearer token123").
//	    WithStatus(200).
//	    Reply()
//
//	// Match by body content
//	mock.Mock("POST", "/api/data").
//	    WithBodyContains("important").
//	    WithStatus(201).
//	    Reply()
//
// # Response Types
//
// Return various response types:
//
//	// JSON response (automatically sets Content-Type)
//	mock.Mock("GET", "/api/user").
//	    WithJSON(User{ID: "123", Name: "Test"}).
//	    Reply()
//
//	// String body
//	mock.Mock("GET", "/api/text").
//	    WithBody("Hello, World!").
//	    Reply()
//
//	// Custom headers
//	mock.Mock("GET", "/api/custom").
//	    WithHeader("X-Custom", "value").
//	    WithBody("custom response").
//	    Reply()
//
// # Limited Responses
//
// Configure mocks that only respond a limited number of times:
//
//	mock.Mock("GET", "/api/once").
//	    WithStatus(200).
//	    Times(1). // Only match once, then 404
//	    Reply()
//
// # Assertions
//
// Verify that endpoints were called correctly:
//
//	// Assert endpoint was called
//	mock.AssertCalled(t, "GET", "/api/endpoint")
//
//	// Assert specific call count
//	mock.AssertCalledTimes(t, "POST", "/api/create", 3)
//
//	// Assert endpoint was NOT called
//	mock.AssertNotCalled(t, "DELETE", "/api/item")
//
//	// Get request logs for custom assertions
//	requests := mock.Requests()
//	for _, req := range requests {
//	    req.AssertHeader(t, "Content-Type", "application/json")
//	    req.AssertJSONBody(t, expectedBody)
//	}
//
// # Path Parameters
//
// Use path patterns with placeholders:
//
//	mock.Mock("GET", "/users/{id}").
//	    WithStatus(200).
//	    WithBody(`{"id": "{{.Request.PathParams.id}}"}`).
//	    Reply()
//
// # Multiple Mocks
//
// Register multiple mocks for different scenarios:
//
//	func TestAPIScenarios(t *testing.T) {
//	    mock := testing.New(t)
//	    defer mock.Stop()
//
//	    // Success case
//	    mock.Mock("GET", "/api/success").
//	        WithStatus(200).
//	        WithBody(`{"status": "ok"}`).
//	        Reply()
//
//	    // Error case
//	    mock.Mock("GET", "/api/error").
//	        WithStatus(500).
//	        WithBody(`{"error": "internal error"}`).
//	        Reply()
//
//	    // Not found case
//	    mock.Mock("GET", "/api/missing").
//	        WithStatus(404).
//	        Reply()
//
//	    url := mock.Start()
//
//	    // Run your tests...
//	}
//
// # Resetting Between Tests
//
// Reset all mocks between test cases:
//
//	func TestWithReset(t *testing.T) {
//	    mock := testing.New(t)
//	    defer mock.Stop()
//
//	    url := mock.Start()
//
//	    // First scenario
//	    mock.Mock("GET", "/api").WithStatus(200).Reply()
//	    // ... test ...
//
//	    mock.Reset() // Clear all mocks and request logs
//
//	    // Second scenario
//	    mock.Mock("GET", "/api").WithStatus(500).Reply()
//	    // ... test ...
//	}
package testing
