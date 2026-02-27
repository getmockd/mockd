package matching

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
)

// Helper to create a test HTTP request.
func newTestRequest(method, path string, headers map[string]string, body string) *http.Request {
	r := &http.Request{
		Method: method,
		URL:    &url.URL{Path: path, RawQuery: ""},
		Header: http.Header{},
		Body:   http.NoBody,
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	if body != "" {
		r.Body = newReadCloser(body)
	}
	return r
}

// Helper to create a request with query params.
func newTestRequestWithQuery(method, path, rawQuery string) *http.Request {
	return &http.Request{
		Method: method,
		URL:    &url.URL{Path: path, RawQuery: rawQuery},
		Header: http.Header{},
		Body:   http.NoBody,
	}
}

type readCloser struct {
	*strings.Reader
}

func (rc readCloser) Close() error { return nil }

func newReadCloser(s string) readCloser {
	return readCloser{strings.NewReader(s)}
}

// --- MatchBreakdown Tests ---

func TestMatchBreakdown_NilMatcher(t *testing.T) {
	r := newTestRequest("GET", "/test", nil, "")
	nm := MatchBreakdown(nil, r, nil)
	if nm.Score != 0 {
		t.Errorf("expected score 0 for nil matcher, got %d", nm.Score)
	}
}

func TestMatchBreakdown_AllFieldsMatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "GET",
		Path:   "/api/users",
	}
	r := newTestRequest("GET", "/api/users", nil, "")
	nm := MatchBreakdown(matcher, r, nil)

	if !nm.Fields[0].Matched || !nm.Fields[1].Matched {
		t.Error("expected both method and path to match")
	}
	if nm.Score != ScoreMethod+ScorePathExact {
		t.Errorf("expected score %d, got %d", ScoreMethod+ScorePathExact, nm.Score)
	}
	if nm.MatchPercentage != 100 {
		t.Errorf("expected 100%% match, got %d%%", nm.MatchPercentage)
	}
}

func TestMatchBreakdown_MethodMismatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "POST",
		Path:   "/api/users",
	}
	r := newTestRequest("GET", "/api/users", nil, "")
	nm := MatchBreakdown(matcher, r, nil)

	// Method should NOT match
	if nm.Fields[0].Matched {
		t.Error("expected method to not match")
	}
	if nm.Fields[0].Expected != "POST" || nm.Fields[0].Actual != "GET" {
		t.Errorf("expected POST/GET, got %v/%v", nm.Fields[0].Expected, nm.Fields[0].Actual)
	}

	// Path should still match (no short-circuit)
	if !nm.Fields[1].Matched {
		t.Error("expected path to match despite method mismatch")
	}

	// Score should be path only
	if nm.Score != ScorePathExact {
		t.Errorf("expected score %d (path only), got %d", ScorePathExact, nm.Score)
	}
}

func TestMatchBreakdown_PathMismatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "GET",
		Path:   "/api/users",
	}
	r := newTestRequest("GET", "/api/products", nil, "")
	nm := MatchBreakdown(matcher, r, nil)

	if !nm.Fields[0].Matched {
		t.Error("expected method to match")
	}
	if nm.Fields[1].Matched {
		t.Error("expected path to not match")
	}
	if nm.Score != ScoreMethod {
		t.Errorf("expected score %d (method only), got %d", ScoreMethod, nm.Score)
	}
}

func TestMatchBreakdown_HeaderMismatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "POST",
		Path:   "/api/users",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer *",
		},
	}
	r := newTestRequest("POST", "/api/users", map[string]string{
		"Content-Type": "text/plain",
		// Authorization missing
	}, "")
	nm := MatchBreakdown(matcher, r, nil)

	// Method and path match
	if !nm.Fields[0].Matched || !nm.Fields[1].Matched {
		t.Error("expected method and path to match")
	}

	// Headers should not all match
	headersField := nm.Fields[2]
	if headersField.Matched {
		t.Error("expected headers to not match overall")
	}

	// Check individual header details
	details, ok := headersField.Details.([]HeaderDetail)
	if !ok {
		t.Fatal("expected HeaderDetail slice in details")
	}

	// At least one header should not match
	mismatched := 0
	for _, d := range details {
		if !d.Matched {
			mismatched++
		}
	}
	if mismatched == 0 {
		t.Error("expected at least one header mismatch")
	}
}

func TestMatchBreakdown_QueryParamMismatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "GET",
		Path:   "/api/search",
		QueryParams: map[string]string{
			"q":    "mockd",
			"page": "1",
		},
	}
	r := newTestRequestWithQuery("GET", "/api/search", "q=mockd&page=2")
	nm := MatchBreakdown(matcher, r, nil)

	// Method and path match, query params partially match
	qpField := nm.Fields[2]
	if qpField.Field != "queryParams" {
		t.Fatalf("expected queryParams field, got %s", qpField.Field)
	}
	if qpField.Matched {
		t.Error("expected queryParams to not match (page=2 vs page=1)")
	}
	// q=mockd matched, so partial score
	if qpField.Score != ScoreQueryParam {
		t.Errorf("expected partial query score %d, got %d", ScoreQueryParam, qpField.Score)
	}
}

func TestMatchBreakdown_BodyContainsMismatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method:       "POST",
		Path:         "/api/users",
		BodyContains: "email",
	}
	body := `{"name": "Alice"}`
	r := newTestRequest("POST", "/api/users", nil, body)
	nm := MatchBreakdown(matcher, r, []byte(body))

	// Method and path match
	if !nm.Fields[0].Matched || !nm.Fields[1].Matched {
		t.Error("expected method and path to match")
	}

	// BodyContains should not match
	bodyField := nm.Fields[2]
	if bodyField.Matched {
		t.Error("expected bodyContains to not match")
	}
	if bodyField.Score != 0 {
		t.Errorf("expected score 0 for body mismatch, got %d", bodyField.Score)
	}
}

func TestMatchBreakdown_BodyPatternMismatch(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method:      "POST",
		Path:        "/api/users",
		BodyPattern: `"email"\s*:\s*"[^"]+@[^"]+"`,
	}
	body := `{"name": "Alice"}`
	r := newTestRequest("POST", "/api/users", nil, body)
	nm := MatchBreakdown(matcher, r, []byte(body))

	bodyField := nm.Fields[2]
	if bodyField.Matched {
		t.Error("expected bodyPattern to not match")
	}
}

func TestMatchBreakdown_PathPattern(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method:      "GET",
		PathPattern: `/api/users/\d+`,
	}
	r := newTestRequest("GET", "/api/users/abc", nil, "")
	nm := MatchBreakdown(matcher, r, nil)

	if !nm.Fields[0].Matched {
		t.Error("expected method to match")
	}
	if nm.Fields[1].Matched {
		t.Error("expected pathPattern to not match for non-numeric ID")
	}
}

func TestMatchBreakdown_ScoreCalculation(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "GET",
		Path:   "/api/users",
		Headers: map[string]string{
			"Accept": "application/json",
		},
		QueryParams: map[string]string{
			"limit": "10",
		},
	}
	r := newTestRequestWithQuery("GET", "/api/users", "limit=10")
	r.Header.Set("Accept", "application/json")
	nm := MatchBreakdown(matcher, r, nil)

	expectedScore := ScoreMethod + ScorePathExact + ScoreHeader + ScoreQueryParam
	if nm.Score != expectedScore {
		t.Errorf("expected score %d, got %d", expectedScore, nm.Score)
	}
	if nm.MatchPercentage != 100 {
		t.Errorf("expected 100%%, got %d%%", nm.MatchPercentage)
	}
}

func TestMatchBreakdown_Percentage(t *testing.T) {
	matcher := &mock.HTTPMatcher{
		Method: "POST",
		Path:   "/api/users",
	}
	// Method matches, path doesn't
	r := newTestRequest("POST", "/api/products", nil, "")
	nm := MatchBreakdown(matcher, r, nil)

	// Score = method (10), Max = method (10) + path (15) = 25
	expectedPct := (ScoreMethod * 100) / (ScoreMethod + ScorePathExact)
	if nm.MatchPercentage != expectedPct {
		t.Errorf("expected %d%%, got %d%%", expectedPct, nm.MatchPercentage)
	}
}

func TestMatchBreakdown_OnlySpecifiedFields(t *testing.T) {
	// Matcher only specifies method — path, headers, body should not appear
	matcher := &mock.HTTPMatcher{
		Method: "GET",
	}
	r := newTestRequest("GET", "/anything", nil, "")
	nm := MatchBreakdown(matcher, r, nil)

	if len(nm.Fields) != 1 {
		t.Errorf("expected 1 field (method only), got %d", len(nm.Fields))
	}
	if nm.Fields[0].Field != "method" {
		t.Errorf("expected 'method' field, got %q", nm.Fields[0].Field)
	}
}

// --- CollectNearMisses Tests ---

func TestCollectNearMisses_Ordering(t *testing.T) {
	enabled := true
	mocks := []*mock.Mock{
		{ID: "mock1", Name: "Exact path", Type: mock.TypeHTTP, Enabled: &enabled,
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users"}}},
		{ID: "mock2", Name: "Wrong everything", Type: mock.TypeHTTP, Enabled: &enabled,
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "DELETE", Path: "/api/admin"}}},
		{ID: "mock3", Name: "Right method only", Type: mock.TypeHTTP, Enabled: &enabled,
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/api/products"}}},
	}

	r := newTestRequest("POST", "/api/users", nil, "")
	results := CollectNearMisses(mocks, r, nil, 3)

	if len(results) == 0 {
		t.Fatal("expected at least one near miss")
	}

	// mock1 matches path but not method (score = path 15)
	// mock3 matches method but not path (score = method 10)
	// mock2 might match nothing (DELETE != POST, /api/admin != /api/users)
	// So mock1 should be first (higher score from path)
	if results[0].MockID != "mock1" {
		t.Errorf("expected mock1 first (path match), got %s", results[0].MockID)
	}
}

func TestCollectNearMisses_FilterZeroScore(t *testing.T) {
	enabled := true
	mocks := []*mock.Mock{
		{ID: "mock1", Type: mock.TypeHTTP, Enabled: &enabled,
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "DELETE", Path: "/completely/different"}}},
	}

	r := newTestRequest("GET", "/api/users", nil, "")
	results := CollectNearMisses(mocks, r, nil, 3)

	// Nothing should match at all
	if len(results) != 0 {
		t.Errorf("expected 0 near misses, got %d", len(results))
	}
}

func TestCollectNearMisses_Empty(t *testing.T) {
	r := newTestRequest("GET", "/api/users", nil, "")
	results := CollectNearMisses(nil, r, nil, 3)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil mocks, got %d", len(results))
	}
}

func TestCollectNearMisses_SkipsNonHTTP(t *testing.T) {
	enabled := true
	mocks := []*mock.Mock{
		{ID: "ws1", Type: mock.TypeWebSocket, Enabled: &enabled},
		{ID: "grpc1", Type: mock.TypeGRPC, Enabled: &enabled},
	}

	r := newTestRequest("GET", "/api/users", nil, "")
	results := CollectNearMisses(mocks, r, nil, 3)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-HTTP mocks, got %d", len(results))
	}
}

func TestCollectNearMisses_SkipsDisabled(t *testing.T) {
	disabled := false
	mocks := []*mock.Mock{
		{ID: "mock1", Type: mock.TypeHTTP, Enabled: &disabled,
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users"}}},
	}

	r := newTestRequest("GET", "/api/users", nil, "")
	results := CollectNearMisses(mocks, r, nil, 3)
	if len(results) != 0 {
		t.Errorf("expected 0 results for disabled mocks, got %d", len(results))
	}
}

func TestCollectNearMisses_TopN(t *testing.T) {
	enabled := true
	var mocks []*mock.Mock
	for i := 0; i < 10; i++ {
		mocks = append(mocks, &mock.Mock{
			ID: fmt.Sprintf("mock%d", i), Type: mock.TypeHTTP, Enabled: &enabled,
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: fmt.Sprintf("/api/path%d", i)}},
		})
	}

	// Request with GET method — all mocks match on method (score 10 each)
	r := newTestRequest("GET", "/api/other", nil, "")
	results := CollectNearMisses(mocks, r, nil, 3)

	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

// --- GenerateReason Tests ---

func TestGenerateReason_AllMatched(t *testing.T) {
	fields := []FieldResult{
		{Field: "method", Matched: true},
		{Field: "path", Matched: true},
	}
	reason := GenerateReason(fields)
	if reason != "all specified fields matched" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestGenerateReason_MethodMismatch(t *testing.T) {
	fields := []FieldResult{
		{Field: "method", Matched: false, Expected: "POST", Actual: "GET"},
	}
	reason := GenerateReason(fields)
	if !strings.Contains(reason, "method expected") {
		t.Errorf("expected method mismatch in reason, got: %s", reason)
	}
}

func TestGenerateReason_PartialMatch(t *testing.T) {
	fields := []FieldResult{
		{Field: "method", Matched: true},
		{Field: "path", Matched: false, Expected: "/api/users", Actual: "/api/products"},
	}
	reason := GenerateReason(fields)
	if !strings.Contains(reason, "method matched") {
		t.Errorf("expected 'method matched' in reason, got: %s", reason)
	}
	if !strings.Contains(reason, "path expected") {
		t.Errorf("expected path mismatch in reason, got: %s", reason)
	}
}

func TestGenerateReason_MultipleMatched(t *testing.T) {
	fields := []FieldResult{
		{Field: "method", Matched: true},
		{Field: "path", Matched: true},
		{Field: "headers", Matched: false, Details: []HeaderDetail{
			{Key: "Content-Type", Expected: "application/json", Actual: "text/plain", Matched: false},
		}},
	}
	reason := GenerateReason(fields)
	if !strings.Contains(reason, "method and path matched") {
		t.Errorf("expected 'method and path matched' in reason, got: %s", reason)
	}
	if !strings.Contains(reason, "header Content-Type") {
		t.Errorf("expected header detail in reason, got: %s", reason)
	}
}

func TestGenerateReason_Empty(t *testing.T) {
	reason := GenerateReason(nil)
	if reason != "no fields to compare" {
		t.Errorf("unexpected reason for empty fields: %s", reason)
	}
}

func TestJoinFields(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, ""},
		{[]string{"method"}, "method"},
		{[]string{"method", "path"}, "method and path"},
		{[]string{"method", "path", "headers"}, "method, path, and headers"},
	}
	for _, tt := range tests {
		got := joinFields(tt.input)
		if got != tt.expected {
			t.Errorf("joinFields(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
