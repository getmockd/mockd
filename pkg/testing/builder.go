package testing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// MockBuilder builds mock configurations using a fluent API.
type MockBuilder struct {
	server *MockServer
	mock   *config.MockConfiguration
	times  int   // 0 means unlimited
	err    error // First error encountered during building
}

// setError records the first error encountered during building.
// Subsequent errors are ignored (first error wins pattern).
func (b *MockBuilder) setError(err error) {
	if b.err == nil {
		b.err = err
	}
}

// Err returns any error encountered during building.
func (b *MockBuilder) Err() error {
	return b.err
}

// ensureHTTP ensures HTTP spec is initialized
func (b *MockBuilder) ensureHTTP() {
	if b.mock.HTTP == nil {
		b.mock.HTTP = &mock.HTTPSpec{}
	}
	if b.mock.HTTP.Response == nil {
		b.mock.HTTP.Response = &mock.HTTPResponse{}
	}
	if b.mock.HTTP.Matcher == nil {
		b.mock.HTTP.Matcher = &mock.HTTPMatcher{}
	}
}

// WithStatus sets the HTTP response status code.
// Default is 200 (OK).
func (b *MockBuilder) WithStatus(status int) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Response.StatusCode = status
	return b
}

// WithBody sets the response body.
// For structs/maps, use WithJSON instead for automatic JSON encoding.
func (b *MockBuilder) WithBody(body interface{}) *MockBuilder {
	b.ensureHTTP()
	switch v := body.(type) {
	case string:
		b.mock.HTTP.Response.Body = v
	case []byte:
		b.mock.HTTP.Response.Body = string(v)
	default:
		// Try to JSON encode
		data, err := json.Marshal(v)
		if err != nil {
			b.setError(fmt.Errorf("WithBody: failed to marshal body: %w", err))
			b.mock.HTTP.Response.Body = ""
		} else {
			b.mock.HTTP.Response.Body = string(data)
			// Set Content-Type if not already set
			if b.mock.HTTP.Response.Headers == nil {
				b.mock.HTTP.Response.Headers = make(map[string]string)
			}
			if _, ok := b.mock.HTTP.Response.Headers["Content-Type"]; !ok {
				b.mock.HTTP.Response.Headers["Content-Type"] = "application/json"
			}
		}
	}
	return b
}

// WithJSON sets the response body as JSON.
// Automatically sets Content-Type to application/json.
func (b *MockBuilder) WithJSON(body interface{}) *MockBuilder {
	b.ensureHTTP()
	data, err := json.Marshal(body)
	if err != nil {
		b.setError(fmt.Errorf("WithJSON: failed to marshal body: %w", err))
		b.mock.HTTP.Response.Body = ""
	} else {
		b.mock.HTTP.Response.Body = string(data)
	}

	if b.mock.HTTP.Response.Headers == nil {
		b.mock.HTTP.Response.Headers = make(map[string]string)
	}
	b.mock.HTTP.Response.Headers["Content-Type"] = "application/json"

	return b
}

// WithHeader adds a response header.
func (b *MockBuilder) WithHeader(key, value string) *MockBuilder {
	b.ensureHTTP()
	if b.mock.HTTP.Response.Headers == nil {
		b.mock.HTTP.Response.Headers = make(map[string]string)
	}
	b.mock.HTTP.Response.Headers[key] = value
	return b
}

// WithHeaders sets multiple response headers at once.
func (b *MockBuilder) WithHeaders(headers map[string]string) *MockBuilder {
	b.ensureHTTP()
	if b.mock.HTTP.Response.Headers == nil {
		b.mock.HTTP.Response.Headers = make(map[string]string)
	}
	for k, v := range headers {
		b.mock.HTTP.Response.Headers[k] = v
	}
	return b
}

// WithDelay adds a response delay.
// Accepts duration strings like "100ms", "1s", "500ms".
func (b *MockBuilder) WithDelay(delay string) *MockBuilder {
	b.ensureHTTP()
	d, err := time.ParseDuration(delay)
	if err != nil {
		b.setError(fmt.Errorf("WithDelay: invalid duration %q: %w", delay, err))
	} else {
		b.mock.HTTP.Response.DelayMs = int(d.Milliseconds())
	}
	return b
}

// WithDelayMs adds a response delay in milliseconds.
func (b *MockBuilder) WithDelayMs(delayMs int) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Response.DelayMs = delayMs
	return b
}

// WithBodyContains matches requests containing the given substring in the body.
func (b *MockBuilder) WithBodyContains(substr string) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Matcher.BodyContains = substr
	return b
}

// WithBodyEquals matches requests with exactly matching body.
func (b *MockBuilder) WithBodyEquals(body string) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Matcher.BodyEquals = body
	return b
}

// WithBodyPattern matches requests with body matching the regex pattern.
func (b *MockBuilder) WithBodyPattern(pattern string) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Matcher.BodyPattern = pattern
	return b
}

// WithQueryParam matches requests with a specific query parameter.
func (b *MockBuilder) WithQueryParam(key, value string) *MockBuilder {
	b.ensureHTTP()
	if b.mock.HTTP.Matcher.QueryParams == nil {
		b.mock.HTTP.Matcher.QueryParams = make(map[string]string)
	}
	b.mock.HTTP.Matcher.QueryParams[key] = value
	return b
}

// WithQueryParams matches requests with multiple query parameters.
func (b *MockBuilder) WithQueryParams(params map[string]string) *MockBuilder {
	b.ensureHTTP()
	if b.mock.HTTP.Matcher.QueryParams == nil {
		b.mock.HTTP.Matcher.QueryParams = make(map[string]string)
	}
	for k, v := range params {
		b.mock.HTTP.Matcher.QueryParams[k] = v
	}
	return b
}

// WithRequestHeader matches requests with a specific header.
func (b *MockBuilder) WithRequestHeader(key, value string) *MockBuilder {
	b.ensureHTTP()
	if b.mock.HTTP.Matcher.Headers == nil {
		b.mock.HTTP.Matcher.Headers = make(map[string]string)
	}
	b.mock.HTTP.Matcher.Headers[key] = value
	return b
}

// WithRequestHeaders matches requests with multiple headers.
func (b *MockBuilder) WithRequestHeaders(headers map[string]string) *MockBuilder {
	b.ensureHTTP()
	if b.mock.HTTP.Matcher.Headers == nil {
		b.mock.HTTP.Matcher.Headers = make(map[string]string)
	}
	for k, v := range headers {
		b.mock.HTTP.Matcher.Headers[k] = v
	}
	return b
}

// WithPathPattern sets a regex pattern for path matching.
func (b *MockBuilder) WithPathPattern(pattern string) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Matcher.PathPattern = pattern
	return b
}

// WithPriority sets the mock priority.
// Higher priority mocks are matched first when multiple mocks could match.
func (b *MockBuilder) WithPriority(priority int) *MockBuilder {
	b.ensureHTTP()
	b.mock.HTTP.Priority = priority
	return b
}

// WithName sets a human-readable name for the mock.
// Useful for debugging and identification.
func (b *MockBuilder) WithName(name string) *MockBuilder {
	b.mock.Name = name
	return b
}

// WithDescription sets a description for the mock.
func (b *MockBuilder) WithDescription(description string) *MockBuilder {
	b.mock.Description = description
	return b
}

// Times sets how many times this mock should match.
// After matching n times, subsequent requests will get 404.
// Use 0 for unlimited matches (default).
func (b *MockBuilder) Times(n int) *MockBuilder {
	b.times = n
	return b
}

// Once is a convenience method for Times(1).
func (b *MockBuilder) Once() *MockBuilder {
	return b.Times(1)
}

// Twice is a convenience method for Times(2).
func (b *MockBuilder) Twice() *MockBuilder {
	return b.Times(2)
}

// Build finalizes and registers the mock.
// Returns the MockServer for method chaining if needed.
func (b *MockBuilder) Build() *MockServer {
	b.server.addMock(b.mock)

	// Set times limit if configured
	if b.times > 0 {
		b.server.setTimesLimit(b.mock.ID, b.times)
	}

	return b.server
}

// Reply is an alias for Build.
// More readable in fluent chains:
//
//	mock.Mock("GET", "/api").WithStatus(200).Reply()
func (b *MockBuilder) Reply() {
	b.Build()
}

// RespondWith is a shorthand for setting status and body together.
func (b *MockBuilder) RespondWith(status int, body interface{}) *MockBuilder {
	return b.WithStatus(status).WithBody(body)
}

// RespondJSON is a shorthand for JSON response with status 200.
func (b *MockBuilder) RespondJSON(body interface{}) *MockBuilder {
	return b.WithStatus(http.StatusOK).WithJSON(body)
}

// RespondNotFound configures a 404 Not Found response.
func (b *MockBuilder) RespondNotFound() *MockBuilder {
	return b.WithStatus(http.StatusNotFound).WithJSON(map[string]string{
		"error": "not_found",
	})
}

// RespondBadRequest configures a 400 Bad Request response.
func (b *MockBuilder) RespondBadRequest(message string) *MockBuilder {
	return b.WithStatus(http.StatusBadRequest).WithJSON(map[string]string{
		"error": message,
	})
}

// RespondServerError configures a 500 Internal Server Error response.
func (b *MockBuilder) RespondServerError(message string) *MockBuilder {
	return b.WithStatus(http.StatusInternalServerError).WithJSON(map[string]string{
		"error": message,
	})
}

// RespondUnauthorized configures a 401 Unauthorized response.
func (b *MockBuilder) RespondUnauthorized() *MockBuilder {
	return b.WithStatus(http.StatusUnauthorized).WithJSON(map[string]string{
		"error": "unauthorized",
	})
}

// RespondForbidden configures a 403 Forbidden response.
func (b *MockBuilder) RespondForbidden() *MockBuilder {
	return b.WithStatus(http.StatusForbidden).WithJSON(map[string]string{
		"error": "forbidden",
	})
}

// RespondCreated configures a 201 Created response.
func (b *MockBuilder) RespondCreated(body interface{}) *MockBuilder {
	return b.WithStatus(http.StatusCreated).WithJSON(body)
}

// RespondNoContent configures a 204 No Content response.
func (b *MockBuilder) RespondNoContent() *MockBuilder {
	return b.WithStatus(http.StatusNoContent)
}

// RespondAccepted configures a 202 Accepted response.
func (b *MockBuilder) RespondAccepted() *MockBuilder {
	return b.WithStatus(http.StatusAccepted)
}
