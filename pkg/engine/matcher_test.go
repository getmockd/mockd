package engine

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/getmockd/mockd/pkg/config"
)

// T070: Method matching
func TestMatchMethod(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		reqMethod string
		want      bool
	}{
		{"exact match GET", "GET", "GET", true},
		{"exact match POST", "POST", "POST", true},
		{"case insensitive", "get", "GET", true},
		{"no match", "GET", "POST", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &config.MockConfiguration{
				ID:       "test",
				Enabled:  true,
				Matcher:  &config.RequestMatcher{Method: tt.method},
				Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
			}
			mocks := []*config.MockConfiguration{mock}

			req := httptest.NewRequest(tt.reqMethod, "/test", nil)
			result := SelectBestMatch(mocks, req)

			if tt.want {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// T071: Path exact matching
func TestMatchPathExact(t *testing.T) {
	mock := &config.MockConfiguration{
		ID:       "test",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Path: "/api/users"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
	}
	mocks := []*config.MockConfiguration{mock}

	tests := []struct {
		path string
		want bool
	}{
		{"/api/users", true},
		{"/api/users/", false},
		{"/api/user", false},
		{"/api", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := SelectBestMatch(mocks, req)

			if tt.want {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// T072: Path wildcard matching
func TestMatchPathWildcard(t *testing.T) {
	mock := &config.MockConfiguration{
		ID:       "test",
		Enabled:  true,
		Matcher:  &config.RequestMatcher{Path: "/api/users/*"},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
	}
	mocks := []*config.MockConfiguration{mock}

	tests := []struct {
		path string
		want bool
	}{
		{"/api/users/123", true},
		{"/api/users/abc", true},
		{"/api/users/abc/profile", true},
		{"/api/users", true},
		{"/api/other", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := SelectBestMatch(mocks, req)

			if tt.want {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// T073: Header matching
func TestMatchHeaders(t *testing.T) {
	mock := &config.MockConfiguration{
		ID:      "test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/api/test",
			Headers: map[string]string{
				"X-API-Key": "secret123",
			},
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
	}
	mocks := []*config.MockConfiguration{mock}

	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"matching header", map[string]string{"X-API-Key": "secret123"}, true},
		{"wrong value", map[string]string{"X-API-Key": "wrong"}, false},
		{"missing header", map[string]string{}, false},
		{"extra headers ok", map[string]string{"X-API-Key": "secret123", "Other": "value"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			result := SelectBestMatch(mocks, req)

			if tt.want {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// T074: Query param matching
func TestMatchQueryParams(t *testing.T) {
	mock := &config.MockConfiguration{
		ID:      "test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/api/search",
			QueryParams: map[string]string{
				"q": "test",
			},
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
	}
	mocks := []*config.MockConfiguration{mock}

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"matching param", "?q=test", true},
		{"wrong value", "?q=other", false},
		{"missing param", "", false},
		{"extra params ok", "?q=test&page=1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/search"+tt.query, nil)
			result := SelectBestMatch(mocks, req)

			if tt.want {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// T075: Body contains matching
func TestMatchBodyContains(t *testing.T) {
	mock := &config.MockConfiguration{
		ID:      "test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method:       "POST",
			Path:         "/api/data",
			BodyContains: "email",
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
	}
	mocks := []*config.MockConfiguration{mock}

	tests := []struct {
		name string
		body string
		want bool
	}{
		{"contains email", `{"email": "test@example.com"}`, true},
		{"contains email substring", `email: test`, true},
		{"no email", `{"name": "test"}`, false},
		{"empty body", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/data", strings.NewReader(tt.body))
			result := SelectBestMatch(mocks, req)

			if tt.want {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// T076: Multiple criteria matching with scoring
func TestMultipleCriteriaScoring(t *testing.T) {
	// Create two mocks - one more specific than the other
	genericMock := &config.MockConfiguration{
		ID:       "generic",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "generic"},
	}

	specificMock := &config.MockConfiguration{
		ID:       "specific",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
			Headers: map[string]string{
				"X-Version": "v2",
			},
			QueryParams: map[string]string{
				"include": "profile",
			},
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "specific"},
	}

	mocks := []*config.MockConfiguration{genericMock, specificMock}

	// Request that matches both - should match specific (higher score)
	req := httptest.NewRequest("GET", "/api/users?include=profile", nil)
	req.Header.Set("X-Version", "v2")
	result := SelectBestMatch(mocks, req)

	assert.NotNil(t, result)
	assert.Equal(t, "specific", result.ID)

	// Request that only matches generic
	req2 := httptest.NewRequest("GET", "/api/users", nil)
	result2 := SelectBestMatch(mocks, req2)

	assert.NotNil(t, result2)
	assert.Equal(t, "generic", result2.ID)
}

// T078: Priority tie-breaking
func TestPriorityTieBreaking(t *testing.T) {
	lowPriority := &config.MockConfiguration{
		ID:       "low",
		Priority: 1,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/test",
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "low"},
	}

	highPriority := &config.MockConfiguration{
		ID:       "high",
		Priority: 10,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/test",
		},
		Response: &config.ResponseDefinition{StatusCode: 200, Body: "high"},
	}

	// Add in low-high order
	mocks := []*config.MockConfiguration{lowPriority, highPriority}

	req := httptest.NewRequest("GET", "/api/test", nil)
	result := SelectBestMatch(mocks, req)

	assert.NotNil(t, result)
	assert.Equal(t, "high", result.ID)
}
