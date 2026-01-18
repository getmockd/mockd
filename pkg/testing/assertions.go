package testing

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// RequestLog represents a logged HTTP request for assertions.
type RequestLog struct {
	// Method is the HTTP method (GET, POST, etc.)
	Method string
	// Path is the request URL path
	Path string
	// Headers are the request headers (single value per key)
	Headers map[string]string
	// Body is the request body content
	Body string
	// QueryString is the raw query string
	QueryString string
	// MatchedID is the ID of the mock that matched this request
	MatchedID string
}

// AssertJSONBody asserts that the request body matches the expected JSON.
// The expected value can be a string, []byte, or any struct/map that will be JSON encoded.
func (r *RequestLog) AssertJSONBody(t testing.TB, expected any) {
	t.Helper()

	var expectedJSON any
	var actualJSON any

	// Parse expected
	switch v := expected.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &expectedJSON); err != nil {
			t.Errorf("failed to parse expected JSON: %v", err)
			return
		}
	case []byte:
		if err := json.Unmarshal(v, &expectedJSON); err != nil {
			t.Errorf("failed to parse expected JSON: %v", err)
			return
		}
	default:
		// Marshal and unmarshal to normalize
		data, err := json.Marshal(v)
		if err != nil {
			t.Errorf("failed to marshal expected value: %v", err)
			return
		}
		if err := json.Unmarshal(data, &expectedJSON); err != nil {
			t.Errorf("failed to parse expected JSON: %v", err)
			return
		}
	}

	// Parse actual
	if err := json.Unmarshal([]byte(r.Body), &actualJSON); err != nil {
		t.Errorf("request body is not valid JSON: %v\nbody: %s", err, r.Body)
		return
	}

	// Compare
	if !reflect.DeepEqual(actualJSON, expectedJSON) {
		expectedBytes, _ := json.MarshalIndent(expectedJSON, "", "  ")
		actualBytes, _ := json.MarshalIndent(actualJSON, "", "  ")
		t.Errorf("request body does not match expected JSON\nexpected:\n%s\nactual:\n%s",
			string(expectedBytes), string(actualBytes))
	}
}

// AssertBody asserts that the request body exactly matches the expected string.
func (r *RequestLog) AssertBody(t testing.TB, expected string) {
	t.Helper()

	if r.Body != expected {
		t.Errorf("request body does not match\nexpected: %q\nactual: %q", expected, r.Body)
	}
}

// AssertBodyContains asserts that the request body contains the expected substring.
func (r *RequestLog) AssertBodyContains(t testing.TB, substr string) {
	t.Helper()

	if !strings.Contains(r.Body, substr) {
		t.Errorf("request body does not contain %q\nbody: %s", substr, r.Body)
	}
}

// AssertHeader asserts that the request had the specified header with the expected value.
func (r *RequestLog) AssertHeader(t testing.TB, key, expected string) {
	t.Helper()

	actual, ok := r.Headers[key]
	if !ok {
		// Try case-insensitive match
		for k, v := range r.Headers {
			if strings.EqualFold(k, key) {
				actual = v
				ok = true
				break
			}
		}
	}

	if !ok {
		t.Errorf("request does not have header %q", key)
		return
	}

	if actual != expected {
		t.Errorf("header %q value mismatch\nexpected: %q\nactual: %q", key, expected, actual)
	}
}

// AssertHeaderExists asserts that the request had the specified header (any value).
func (r *RequestLog) AssertHeaderExists(t testing.TB, key string) {
	t.Helper()

	_, ok := r.Headers[key]
	if !ok {
		// Try case-insensitive match
		for k := range r.Headers {
			if strings.EqualFold(k, key) {
				ok = true
				break
			}
		}
	}

	if !ok {
		t.Errorf("request does not have header %q", key)
	}
}

// AssertHeaderContains asserts that the header value contains the expected substring.
func (r *RequestLog) AssertHeaderContains(t testing.TB, key, substr string) {
	t.Helper()

	var actual string
	var ok bool

	actual, ok = r.Headers[key]
	if !ok {
		// Try case-insensitive match
		for k, v := range r.Headers {
			if strings.EqualFold(k, key) {
				actual = v
				ok = true
				break
			}
		}
	}

	if !ok {
		t.Errorf("request does not have header %q", key)
		return
	}

	if !strings.Contains(actual, substr) {
		t.Errorf("header %q value does not contain %q\nvalue: %q", key, substr, actual)
	}
}

// AssertQueryParam asserts that the request had the specified query parameter.
func (r *RequestLog) AssertQueryParam(t testing.TB, key, expected string) {
	t.Helper()

	// Parse query string manually
	params := parseQueryString(r.QueryString)

	actual, ok := params[key]
	if !ok {
		t.Errorf("request does not have query parameter %q", key)
		return
	}

	if actual != expected {
		t.Errorf("query parameter %q value mismatch\nexpected: %q\nactual: %q", key, expected, actual)
	}
}

// AssertQueryParamExists asserts that the request had the specified query parameter (any value).
func (r *RequestLog) AssertQueryParamExists(t testing.TB, key string) {
	t.Helper()

	params := parseQueryString(r.QueryString)
	if _, ok := params[key]; !ok {
		t.Errorf("request does not have query parameter %q", key)
	}
}

// AssertMethod asserts that the request used the expected HTTP method.
func (r *RequestLog) AssertMethod(t testing.TB, expected string) {
	t.Helper()

	if !strings.EqualFold(r.Method, expected) {
		t.Errorf("request method mismatch\nexpected: %q\nactual: %q", expected, r.Method)
	}
}

// AssertPath asserts that the request path matches.
func (r *RequestLog) AssertPath(t testing.TB, expected string) {
	t.Helper()

	if r.Path != expected {
		t.Errorf("request path mismatch\nexpected: %q\nactual: %q", expected, r.Path)
	}
}

// parseQueryString parses a query string into a map.
func parseQueryString(qs string) map[string]string {
	result := make(map[string]string)
	if qs == "" {
		return result
	}

	pairs := strings.Split(qs, "&")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else if len(parts) == 1 {
			result[parts[0]] = ""
		}
	}
	return result
}

// JSONField extracts a field from the request body JSON.
// Returns nil if the body is not valid JSON or the field doesn't exist.
func (r *RequestLog) JSONField(field string) any {
	var data map[string]any
	if err := json.Unmarshal([]byte(r.Body), &data); err != nil {
		return nil
	}

	// Support nested fields with dot notation
	parts := strings.Split(field, ".")
	var current any = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

// AssertJSONField asserts that a JSON field in the request body has the expected value.
func (r *RequestLog) AssertJSONField(t testing.TB, field string, expected any) {
	t.Helper()

	actual := r.JSONField(field)
	if actual == nil {
		t.Errorf("JSON field %q not found in request body: %s", field, r.Body)
		return
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("JSON field %q mismatch\nexpected: %v (%T)\nactual: %v (%T)",
			field, expected, expected, actual, actual)
	}
}
