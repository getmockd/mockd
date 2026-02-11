package engine

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
)

// helper to create an HTTP mock configuration
func newHTTPMock(id string, enabled bool, matcher *mock.HTTPMatcher, response *mock.HTTPResponse, priority int) *config.MockConfiguration {
	return &config.MockConfiguration{
		ID:      id,
		Enabled: &enabled,
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: priority,
			Matcher:  matcher,
			Response: response,
		},
	}
}

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
			mockCfg := newHTTPMock("test", true,
				&mock.HTTPMatcher{Method: tt.method},
				&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
				0,
			)
			mocks := []*config.MockConfiguration{mockCfg}

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
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{Path: "/api/users"},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

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
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{Path: "/api/users/*"},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

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
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{
			Path: "/api/test",
			Headers: map[string]string{
				"X-API-Key": "secret123",
			},
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

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
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{
			Path: "/api/search",
			QueryParams: map[string]string{
				"q": "test",
			},
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

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
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{
			Method:       "POST",
			Path:         "/api/data",
			BodyContains: "email",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

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
	genericMock := newHTTPMock("generic", true,
		&mock.HTTPMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "generic"},
		0,
	)

	specificMock := newHTTPMock("specific", true,
		&mock.HTTPMatcher{
			Method: "GET",
			Path:   "/api/users",
			Headers: map[string]string{
				"X-Version": "v2",
			},
			QueryParams: map[string]string{
				"include": "profile",
			},
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "specific"},
		0,
	)

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
	lowPriority := newHTTPMock("low", true,
		&mock.HTTPMatcher{
			Method: "GET",
			Path:   "/api/test",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "low"},
		1,
	)

	highPriority := newHTTPMock("high", true,
		&mock.HTTPMatcher{
			Method: "GET",
			Path:   "/api/test",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "high"},
		10,
	)

	// Add in low-high order
	mocks := []*config.MockConfiguration{lowPriority, highPriority}

	req := httptest.NewRequest("GET", "/api/test", nil)
	result := SelectBestMatch(mocks, req)

	assert.NotNil(t, result)
	assert.Equal(t, "high", result.ID)
}

// Test PathPattern regex matching
func TestMatchPathPattern(t *testing.T) {
	tests := []struct {
		name        string
		pathPattern string
		path        string
		want        bool
	}{
		{"regex match numeric id", `^/api/users/\d+$`, "/api/users/123", true},
		{"regex no match", `^/api/users/\d+$`, "/api/users/abc", false},
		{"regex match with alternation", `^/api/(users|products)/\d+$`, "/api/products/456", true},
		{"regex match partial", `/users/\d+`, "/api/users/789/profile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCfg := newHTTPMock("test", true,
				&mock.HTTPMatcher{PathPattern: tt.pathPattern},
				&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
				0,
			)
			mocks := []*config.MockConfiguration{mockCfg}

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

// Test PathPattern with named capture groups
func TestMatchPathPatternWithCaptures(t *testing.T) {
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{
			PathPattern: `^/api/users/(?P<userId>\d+)/posts/(?P<postId>\d+)$`,
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

	req := httptest.NewRequest("GET", "/api/users/42/posts/99", nil)
	result := SelectBestMatchWithCaptures(mocks, req)

	assert.NotNil(t, result)
	assert.NotNil(t, result.PathPatternCaptures)
	assert.Equal(t, "42", result.PathPatternCaptures["userId"])
	assert.Equal(t, "99", result.PathPatternCaptures["postId"])
}

// Test Path and PathPattern mutual exclusivity
func TestPathAndPathPatternMutualExclusive(t *testing.T) {
	// When both Path and PathPattern are set, the matcher should not match
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{
			Path:        "/api/users",
			PathPattern: `^/api/users$`,
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

	req := httptest.NewRequest("GET", "/api/users", nil)
	result := SelectBestMatch(mocks, req)

	assert.Nil(t, result, "should not match when both Path and PathPattern are set")
}

// Test BodyPattern regex matching
func TestMatchBodyPattern(t *testing.T) {
	tests := []struct {
		name        string
		bodyPattern string
		body        string
		want        bool
	}{
		{"regex match email field", `"email":\s*"[^"]+"`, `{"email": "test@example.com"}`, true},
		{"regex no match", `"email":\s*"[^"]+"`, `{"name": "John"}`, false},
		{"regex match status", `"status":\s*"(pending|approved)"`, `{"status": "approved"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCfg := newHTTPMock("test", true,
				&mock.HTTPMatcher{
					Method:      "POST",
					Path:        "/api/data",
					BodyPattern: tt.bodyPattern,
				},
				&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
				0,
			)
			mocks := []*config.MockConfiguration{mockCfg}

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

// Test BodyPattern combined with BodyContains (AND logic)
func TestBodyPatternAndBodyContainsCombined(t *testing.T) {
	mockCfg := newHTTPMock("test", true,
		&mock.HTTPMatcher{
			Method:       "POST",
			Path:         "/api/data",
			BodyContains: "email",
			BodyPattern:  `"status":\s*"active"`,
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "ok"},
		0,
	)
	mocks := []*config.MockConfiguration{mockCfg}

	// Should match - has both email and status: active
	req1 := httptest.NewRequest("POST", "/api/data", strings.NewReader(`{"email": "test@example.com", "status": "active"}`))
	result1 := SelectBestMatch(mocks, req1)
	assert.NotNil(t, result1, "should match when both conditions are satisfied")

	// Should not match - has email but wrong status
	req2 := httptest.NewRequest("POST", "/api/data", strings.NewReader(`{"email": "test@example.com", "status": "inactive"}`))
	result2 := SelectBestMatch(mocks, req2)
	assert.Nil(t, result2, "should not match when BodyPattern fails")

	// Should not match - has correct status but no email
	req3 := httptest.NewRequest("POST", "/api/data", strings.NewReader(`{"name": "John", "status": "active"}`))
	result3 := SelectBestMatch(mocks, req3)
	assert.Nil(t, result3, "should not match when BodyContains fails")
}

// Test scoring: PathPattern should score 14 (between exact 15 and named params 12)
func TestPathPatternScoring(t *testing.T) {
	// Create mocks with different matching methods
	exactMock := newHTTPMock("exact", true,
		&mock.HTTPMatcher{
			Path: "/api/users/123",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "exact"},
		0,
	)

	regexMock := newHTTPMock("regex", true,
		&mock.HTTPMatcher{
			PathPattern: `^/api/users/\d+$`,
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "regex"},
		0,
	)

	namedMock := newHTTPMock("named", true,
		&mock.HTTPMatcher{
			Path: "/api/users/{id}",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "named"},
		0,
	)

	// All three match /api/users/123, but exact should win (score 15 > 14 > 12)
	mocks := []*config.MockConfiguration{namedMock, regexMock, exactMock}

	req := httptest.NewRequest("GET", "/api/users/123", nil)
	result := SelectBestMatch(mocks, req)

	assert.NotNil(t, result)
	assert.Equal(t, "exact", result.ID, "exact match should win over regex and named params")

	// Test when exact match is not available
	mocks2 := []*config.MockConfiguration{namedMock, regexMock}

	req2 := httptest.NewRequest("GET", "/api/users/456", nil)
	result2 := SelectBestMatch(mocks2, req2)

	assert.NotNil(t, result2)
	assert.Equal(t, "regex", result2.ID, "regex should win over named params (14 > 12)")
}

// Test BodyPattern scoring: should score 22 (between contains 20 and equals 25)
func TestBodyPatternScoring(t *testing.T) {
	// Create mocks with different body matching methods
	equalsMock := newHTTPMock("equals", true,
		&mock.HTTPMatcher{
			Method:     "POST",
			Path:       "/api/data",
			BodyEquals: `{"status": "active"}`,
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "equals"},
		0,
	)

	patternMock := newHTTPMock("pattern", true,
		&mock.HTTPMatcher{
			Method:      "POST",
			Path:        "/api/data",
			BodyPattern: `"status":\s*"active"`,
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "pattern"},
		0,
	)

	containsMock := newHTTPMock("contains", true,
		&mock.HTTPMatcher{
			Method:       "POST",
			Path:         "/api/data",
			BodyContains: "status",
		},
		&mock.HTTPResponse{StatusCode: 200, Body: "contains"},
		0,
	)

	// All three match the body, but equals should win (score 25 > 22 > 20)
	mocks := []*config.MockConfiguration{containsMock, patternMock, equalsMock}

	req := httptest.NewRequest("POST", "/api/data", strings.NewReader(`{"status": "active"}`))
	result := SelectBestMatch(mocks, req)

	assert.NotNil(t, result)
	assert.Equal(t, "equals", result.ID, "equals should win over pattern and contains")

	// Test when equals match is not available
	mocks2 := []*config.MockConfiguration{containsMock, patternMock}

	req2 := httptest.NewRequest("POST", "/api/data", strings.NewReader(`{ "status": "active" }`))
	result2 := SelectBestMatch(mocks2, req2)

	assert.NotNil(t, result2)
	assert.Equal(t, "pattern", result2.ID, "pattern should win over contains (22 > 20)")
}
