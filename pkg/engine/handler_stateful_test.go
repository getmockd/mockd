package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/stateful"
)

func TestParseIntParam(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue int
		wantErr   bool
	}{
		{
			name:      "valid positive integer",
			input:     "42",
			wantValue: 42,
			wantErr:   false,
		},
		{
			name:      "zero",
			input:     "0",
			wantValue: 0,
			wantErr:   false,
		},
		{
			name:      "large number",
			input:     "999999",
			wantValue: 999999,
			wantErr:   false,
		},
		{
			name:      "negative number",
			input:     "-10",
			wantValue: -10,
			wantErr:   false,
		},
		{
			name:      "empty string",
			input:     "",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "non-numeric string",
			input:     "abc",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "mixed alphanumeric",
			input:     "123abc",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "decimal number",
			input:     "3.14",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "whitespace",
			input:     " 10 ",
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v int
			got, err := parseIntParam(tt.input, &v)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIntParam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got != tt.wantValue {
					t.Errorf("parseIntParam() returned = %v, want %v", got, tt.wantValue)
				}
				if v != tt.wantValue {
					t.Errorf("parseIntParam() set v = %v, want %v", v, tt.wantValue)
				}
			}
		})
	}
}

// ── Error response format ────────────────────────────────────────────────────

func TestWriteStatefulError_ResponseBody(t *testing.T) {
	h := &Handler{log: slog.Default()}

	w := httptest.NewRecorder()
	status := h.writeStatefulError(w, http.StatusNotFound, "resource not found", "users", "user-42")

	if status != http.StatusNotFound {
		t.Errorf("Expected %d, got %d", http.StatusNotFound, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Error != "resource not found" {
		t.Errorf("Error: got %q", resp.Error)
	}
	if resp.Resource != "users" {
		t.Errorf("Resource: got %q", resp.Resource)
	}
	if resp.ID != "user-42" {
		t.Errorf("ID: got %q", resp.ID)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode: got %d", resp.StatusCode)
	}
}

func TestWriteStatefulErrorWithHint_ResponseBody(t *testing.T) {
	h := &Handler{log: slog.Default()}

	w := httptest.NewRecorder()
	status := h.writeStatefulErrorWithHint(w, http.StatusInsufficientStorage, "capacity exceeded", "products", "", "Delete some items")

	if status != http.StatusInsufficientStorage {
		t.Errorf("Expected %d, got %d", http.StatusInsufficientStorage, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Hint != "Delete some items" {
		t.Errorf("Hint: got %q", resp.Hint)
	}
}

// ── Query filter parsing ─────────────────────────────────────────────────────

func TestParseQueryFilter_Defaults(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if filter.Limit != 100 {
		t.Errorf("Default limit: expected 100, got %d", filter.Limit)
	}
	if filter.Offset != 0 {
		t.Errorf("Default offset: expected 0, got %d", filter.Offset)
	}
}

func TestParseQueryFilter_WithParams(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=25&offset=10&sort=name&order=asc&status=active", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if filter.Limit != 25 {
		t.Errorf("Limit: expected 25, got %d", filter.Limit)
	}
	if filter.Offset != 10 {
		t.Errorf("Offset: expected 10, got %d", filter.Offset)
	}
	if filter.Sort != "name" {
		t.Errorf("Sort: expected 'name', got %q", filter.Sort)
	}
	if filter.Order != "asc" {
		t.Errorf("Order: expected 'asc', got %q", filter.Order)
	}
	if filter.Filters["status"] != "active" {
		t.Errorf("Custom filter 'status': expected 'active', got %q", filter.Filters["status"])
	}
}

func TestParseQueryFilter_InvalidLimitIgnored(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=abc&offset=-5", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	// Invalid limit should use default
	if filter.Limit != 100 {
		t.Errorf("Invalid limit should default to 100, got %d", filter.Limit)
	}
}

func TestParseQueryFilter_ZeroLimitIgnored(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=0", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	// limit=0 is not > 0, so default applies
	if filter.Limit != 100 {
		t.Errorf("Zero limit should default to 100, got %d", filter.Limit)
	}
}

func TestParseQueryFilter_WithParentField(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:        "comments",
		ParentField: "postId",
	}
	resource := stateful.NewStatefulResource(cfg)

	pathParams := map[string]string{"postId": "post-99"}
	req := httptest.NewRequest(http.MethodGet, "/api/posts/post-99/comments", nil)
	filter := h.parseQueryFilter(req, resource, pathParams)

	if filter.ParentField != "postId" {
		t.Errorf("ParentField: expected 'postId', got %q", filter.ParentField)
	}
	if filter.ParentID != "post-99" {
		t.Errorf("ParentID: expected 'post-99', got %q", filter.ParentID)
	}
}

func TestParseQueryFilter_ReservedKeysExcludedFromFilters(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=10&offset=5&sort=name&order=asc&category=books", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	// Reserved keys should not appear in Filters map
	for _, reserved := range []string{"limit", "offset", "sort", "order"} {
		if _, ok := filter.Filters[reserved]; ok {
			t.Errorf("Reserved key %q should not be in Filters map", reserved)
		}
	}

	// Custom query params should be in Filters
	if filter.Filters["category"] != "books" {
		t.Errorf("Custom filter 'category': expected 'books', got %q", filter.Filters["category"])
	}
}

// ── handleCustomOperation tests ──────────────────────────────────────────────

func TestHandleCustomOperation_BodyTooLarge(t *testing.T) {
	h := &Handler{
		log:            slog.Default(),
		statefulBridge: stateful.NewBridge(stateful.NewStateStore()),
	}

	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/transfer", bytes.NewReader(largeBody))

	status := h.handleCustomOperation(w, req, "", "TransferFunds", largeBody)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected %d, got %d", http.StatusRequestEntityTooLarge, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("response statusCode = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
	if resp.Error != "request body too large" {
		t.Fatalf("response error = %q, want request body too large", resp.Error)
	}
}

// ── parseStatefulBody tests ──────────────────────────────────────────────────

func TestParseStatefulBody_JSON(t *testing.T) {
	body := []byte(`{"name":"Alice","age":30}`)
	data, err := parseStatefulBody(body, "application/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", data["name"])
	}
	if data["age"] != float64(30) {
		t.Errorf("expected age=30, got %v (type %T)", data["age"], data["age"])
	}
}

func TestParseStatefulBody_JSONWithCharset(t *testing.T) {
	body := []byte(`{"name":"Bob"}`)
	data, err := parseStatefulBody(body, "application/json; charset=utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", data["name"])
	}
}

func TestParseStatefulBody_EmptyContentType(t *testing.T) {
	body := []byte(`{"name":"Charlie"}`)
	data, err := parseStatefulBody(body, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Charlie" {
		t.Errorf("expected name=Charlie, got %v", data["name"])
	}
}

func TestParseStatefulBody_FormEncoded(t *testing.T) {
	body := []byte("name=Alice&email=alice%40example.com")
	data, err := parseStatefulBody(body, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", data["name"])
	}
	if data["email"] != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %v", data["email"])
	}
}

func TestParseStatefulBody_FormEncodedNested(t *testing.T) {
	body := []byte("name=Alice&metadata[tier]=premium&metadata[source]=api")
	data, err := parseStatefulBody(body, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", data["name"])
	}
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata as map, got %T", data["metadata"])
	}
	if meta["tier"] != "premium" {
		t.Errorf("expected metadata.tier=premium, got %v", meta["tier"])
	}
	if meta["source"] != "api" {
		t.Errorf("expected metadata.source=api, got %v", meta["source"])
	}
}

func TestParseStatefulBody_FormEncodedDeepNested(t *testing.T) {
	body := []byte("items[0][price]=price_123&items[0][quantity]=1")
	data, err := parseStatefulBody(body, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Numeric-keyed maps are converted to arrays
	items, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("expected items as []any (array), got %T", data["items"])
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item0, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected items[0] as map, got %T", items[0])
	}
	if item0["price"] != "price_123" {
		t.Errorf("expected price=price_123, got %v", item0["price"])
	}
	// quantity is coerced to int64 from form value
	if item0["quantity"] != int64(1) {
		t.Errorf("expected quantity=1 (int64), got %v (%T)", item0["quantity"], item0["quantity"])
	}
}

func TestParseStatefulBody_InvalidJSON(t *testing.T) {
	body := []byte(`{not valid json}`)
	_, err := parseStatefulBody(body, "application/json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseStatefulBody_UnknownContentTypeFallsBackToJSON(t *testing.T) {
	body := []byte(`{"name":"Dave"}`)
	data, err := parseStatefulBody(body, "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Dave" {
		t.Errorf("expected name=Dave, got %v", data["name"])
	}
}

func TestParseStatefulBody_UnknownContentTypeInvalidJSON(t *testing.T) {
	body := []byte(`not json at all`)
	_, err := parseStatefulBody(body, "text/plain")
	if err == nil {
		t.Fatal("expected error for unknown content type with invalid JSON")
	}
}

// ── formToMap tests ──────────────────────────────────────────────────────────

func TestFormToMap_Simple(t *testing.T) {
	values := map[string][]string{
		"name":  {"Alice"},
		"email": {"alice@example.com"},
	}
	result := formToMap(values)
	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", result["name"])
	}
	if result["email"] != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %v", result["email"])
	}
}

func TestFormToMap_BracketNested(t *testing.T) {
	values := map[string][]string{
		"metadata[tier]":   {"premium"},
		"metadata[source]": {"api"},
	}
	result := formToMap(values)
	meta, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata as map, got %T", result["metadata"])
	}
	if meta["tier"] != "premium" {
		t.Errorf("expected tier=premium, got %v", meta["tier"])
	}
	if meta["source"] != "api" {
		t.Errorf("expected source=api, got %v", meta["source"])
	}
}

func TestFormToMap_EmptyValues(t *testing.T) {
	values := map[string][]string{
		"empty": {},
	}
	result := formToMap(values)
	if _, ok := result["empty"]; ok {
		t.Error("expected empty values to be skipped")
	}
}

func TestFormToMap_MultipleValues_UsesFirst(t *testing.T) {
	values := map[string][]string{
		"name": {"Alice", "Bob"},
	}
	result := formToMap(values)
	if result["name"] != "Alice" {
		t.Errorf("expected first value Alice, got %v", result["name"])
	}
}

// ── setNested tests ──────────────────────────────────────────────────────────

func TestSetNested_SingleLevel(t *testing.T) {
	result := make(map[string]any)
	setNested(result, "metadata", "[tier]", "premium")
	meta, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result["metadata"])
	}
	if meta["tier"] != "premium" {
		t.Errorf("expected tier=premium, got %v", meta["tier"])
	}
}

func TestSetNested_MultiLevel(t *testing.T) {
	result := make(map[string]any)
	setNested(result, "a", "[b][c]", "deep")
	a, ok := result["a"].(map[string]any)
	if !ok {
		t.Fatalf("expected map at a, got %T", result["a"])
	}
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatalf("expected map at a.b, got %T", a["b"])
	}
	if b["c"] != "deep" {
		t.Errorf("expected c=deep, got %v", b["c"])
	}
}

func TestSetNested_ExistingMap(t *testing.T) {
	result := map[string]any{
		"metadata": map[string]any{"existing": "value"},
	}
	setNested(result, "metadata", "[new]", "added")
	meta := result["metadata"].(map[string]any)
	if meta["existing"] != "value" {
		t.Error("expected existing value to be preserved")
	}
	if meta["new"] != "added" {
		t.Errorf("expected new=added, got %v", meta["new"])
	}
}

func TestSetNested_ConflictBaseNotMap(t *testing.T) {
	result := map[string]any{
		"metadata": "not a map",
	}
	setNested(result, "metadata", "[key]", "value")
	// Should not overwrite the string — setNested returns early on conflict
	if result["metadata"] != "not a map" {
		t.Errorf("expected conflict to be skipped, got %v", result["metadata"])
	}
}

// ── httpStatusToErrorCode tests ──────────────────────────────────────────────

func TestHttpStatusToErrorCode(t *testing.T) {
	tests := []struct {
		status   int
		expected stateful.ErrorCode
	}{
		{http.StatusNotFound, stateful.ErrCodeNotFound},
		{http.StatusConflict, stateful.ErrCodeConflict},
		{http.StatusBadRequest, stateful.ErrCodeValidation},
		{http.StatusUnprocessableEntity, stateful.ErrCodeValidation},
		{http.StatusRequestEntityTooLarge, stateful.ErrCodePayloadTooLarge},
		{http.StatusInsufficientStorage, stateful.ErrCodeCapacityExceeded},
		{http.StatusInternalServerError, stateful.ErrCodeInternal},
		{http.StatusServiceUnavailable, stateful.ErrCodeInternal}, // unmapped → internal
		{http.StatusForbidden, stateful.ErrCodeInternal},          // unmapped → internal
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := httpStatusToErrorCode(tt.status)
			if got != tt.expected {
				t.Errorf("httpStatusToErrorCode(%d) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

// ── parseQueryFilter cursor params tests ─────────────────────────────────────

func TestParseQueryFilter_CursorParams(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?starting_after=item-5&limit=10", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if filter.StartingAfter != "item-5" {
		t.Errorf("expected starting_after=item-5, got %q", filter.StartingAfter)
	}
	if filter.Limit != 10 {
		t.Errorf("expected limit=10, got %d", filter.Limit)
	}
}

func TestParseQueryFilter_EndingBeforeParam(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?ending_before=item-10", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if filter.EndingBefore != "item-10" {
		t.Errorf("expected ending_before=item-10, got %q", filter.EndingBefore)
	}
}

func TestParseQueryFilter_CursorParamsExcludedFromFilters(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name: "items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?starting_after=abc&ending_before=xyz&custom=value", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if _, ok := filter.Filters["starting_after"]; ok {
		t.Error("starting_after should be excluded from Filters map")
	}
	if _, ok := filter.Filters["ending_before"]; ok {
		t.Error("ending_before should be excluded from Filters map")
	}
	if filter.Filters["custom"] != "value" {
		t.Errorf("expected custom=value in Filters, got %q", filter.Filters["custom"])
	}
}

// ── coerceFormValue tests ────────────────────────────────────────────────────

func TestCoerceFormValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  any
	}{
		{name: "true bool", input: "true", want: true},
		{name: "false bool", input: "false", want: false},
		{name: "positive int", input: "42", want: int64(42)},
		{name: "zero int", input: "0", want: int64(0)},
		{name: "negative int", input: "-5", want: int64(-5)},
		{name: "positive float", input: "3.14", want: float64(3.14)},
		{name: "zero float", input: "0.0", want: float64(0.0)},
		{name: "negative float", input: "-1.5", want: float64(-1.5)},
		{name: "plain string", input: "hello", want: "hello"},
		{name: "empty string", input: "", want: ""},
		{name: "inf becomes nil", input: "inf", want: nil},
		{name: "infinity stays string", input: "infinity", want: "infinity"},
		{name: "info stays string", input: "info", want: "info"},
		{name: "Inf stays string (case sensitive)", input: "Inf", want: "Inf"},
		{name: "mixed alphanumeric stays string", input: "123abc", want: "123abc"},
		{name: "large int", input: "1000000000000", want: int64(1000000000000)},
		{name: "phone number with + stays string", input: "+15551234567", want: "+15551234567"},
		{name: "positive float with + stays string", input: "+3.14", want: "+3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceFormValue(tt.input)
			if got != tt.want {
				t.Errorf("coerceFormValue(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

// ── convertNumericKeysToArrays tests ─────────────────────────────────────────

func TestConvertNumericKeysToArrays(t *testing.T) {
	t.Run("simple numeric-keyed map becomes array", func(t *testing.T) {
		m := map[string]any{
			"items": map[string]any{"0": "a", "1": "b", "2": "c"},
		}
		convertNumericKeysToArrays(m)
		arr, ok := m["items"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", m["items"])
		}
		if len(arr) != 3 {
			t.Fatalf("expected len 3, got %d", len(arr))
		}
		if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
			t.Errorf("expected [a b c], got %v", arr)
		}
	})

	t.Run("array of objects", func(t *testing.T) {
		m := map[string]any{
			"items": map[string]any{
				"0": map[string]any{"x": "1"},
				"1": map[string]any{"x": "2"},
			},
		}
		convertNumericKeysToArrays(m)
		arr, ok := m["items"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", m["items"])
		}
		if len(arr) != 2 {
			t.Fatalf("expected len 2, got %d", len(arr))
		}
		obj0, ok := arr[0].(map[string]any)
		if !ok {
			t.Fatalf("expected map at [0], got %T", arr[0])
		}
		if obj0["x"] != "1" {
			t.Errorf("expected x=1, got %v", obj0["x"])
		}
	})

	t.Run("mixed keys — nested numeric converted", func(t *testing.T) {
		m := map[string]any{
			"name": "Alice",
			"tags": map[string]any{"0": "vip", "1": "new"},
		}
		convertNumericKeysToArrays(m)
		if m["name"] != "Alice" {
			t.Errorf("expected name=Alice, got %v", m["name"])
		}
		arr, ok := m["tags"].([]any)
		if !ok {
			t.Fatalf("expected tags as []any, got %T", m["tags"])
		}
		if len(arr) != 2 || arr[0] != "vip" || arr[1] != "new" {
			t.Errorf("expected [vip new], got %v", arr)
		}
	})

	t.Run("non-numeric keys — no conversion", func(t *testing.T) {
		m := map[string]any{
			"data": map[string]any{"tier": "premium", "source": "api"},
		}
		convertNumericKeysToArrays(m)
		sub, ok := m["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", m["data"])
		}
		if sub["tier"] != "premium" || sub["source"] != "api" {
			t.Errorf("expected unchanged map, got %v", sub)
		}
	})

	t.Run("non-consecutive keys — no conversion", func(t *testing.T) {
		m := map[string]any{
			"items": map[string]any{"1": "a", "3": "c"},
		}
		convertNumericKeysToArrays(m)
		sub, ok := m["items"].(map[string]any)
		if !ok {
			t.Fatalf("expected map (unconverted), got %T", m["items"])
		}
		if sub["1"] != "a" || sub["3"] != "c" {
			t.Errorf("expected unchanged map, got %v", sub)
		}
	})

	t.Run("single element array", func(t *testing.T) {
		m := map[string]any{
			"items": map[string]any{"0": "only"},
		}
		convertNumericKeysToArrays(m)
		arr, ok := m["items"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", m["items"])
		}
		if len(arr) != 1 || arr[0] != "only" {
			t.Errorf("expected [only], got %v", arr)
		}
	})

	t.Run("empty map stays empty", func(t *testing.T) {
		m := map[string]any{
			"data": map[string]any{},
		}
		convertNumericKeysToArrays(m)
		sub, ok := m["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected empty map, got %T", m["data"])
		}
		if len(sub) != 0 {
			t.Errorf("expected empty map, got %v", sub)
		}
	})

	t.Run("deeply nested numeric keys converted", func(t *testing.T) {
		m := map[string]any{
			"items": map[string]any{
				"0": map[string]any{
					"tags": map[string]any{"0": "a", "1": "b"},
				},
			},
		}
		convertNumericKeysToArrays(m)
		items, ok := m["items"].([]any)
		if !ok {
			t.Fatalf("expected items as []any, got %T", m["items"])
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		item0, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("expected item[0] as map, got %T", items[0])
		}
		tags, ok := item0["tags"].([]any)
		if !ok {
			t.Fatalf("expected tags as []any, got %T", item0["tags"])
		}
		if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
			t.Errorf("expected [a b], got %v", tags)
		}
	})
}

// ── Integration: form-encoded type coercion ──────────────────────────────────

func TestParseStatefulBody_FormEncodedTypeCoercion(t *testing.T) {
	body := []byte("amount=2000&active=true&rate=3.14&name=Test")
	data, err := parseStatefulBody(body, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data["amount"] != int64(2000) {
		t.Errorf("expected amount=int64(2000), got %v (%T)", data["amount"], data["amount"])
	}
	if data["active"] != true {
		t.Errorf("expected active=true (bool), got %v (%T)", data["active"], data["active"])
	}
	if data["rate"] != float64(3.14) {
		t.Errorf("expected rate=float64(3.14), got %v (%T)", data["rate"], data["rate"])
	}
	if data["name"] != "Test" {
		t.Errorf("expected name=Test (string), got %v (%T)", data["name"], data["name"])
	}
}

// ── Integration: form-encoded arrays ─────────────────────────────────────────

func TestParseStatefulBody_FormEncodedArrays(t *testing.T) {
	body := []byte("items[0]=card&items[1]=bank")
	data, err := parseStatefulBody(body, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("expected items as []any, got %T", data["items"])
	}
	expected := []any{"card", "bank"}
	if len(items) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(items))
	}
	for i, want := range expected {
		if items[i] != want {
			t.Errorf("items[%d] = %v, want %v", i, items[i], want)
		}
	}
}

// ── handleBindingDelete preserve tests ───────────────────────────────────────

func TestHandleBindingDelete_PreserveKeepsItem(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	// Create an item first
	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_123", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	responseCfg := &config.ResponseTransform{
		Delete: &config.VerbOverride{
			Status:   200,
			Preserve: true,
			Body: map[string]interface{}{
				"id":      "{{item.id}}",
				"deleted": true,
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/customers/cus_123", nil)

	status := h.handleBindingDelete(w, req, "", "customers", "cus_123", responseCfg)

	// Should return configured status
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}

	// Should return configured body
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] != "cus_123" {
		t.Errorf("expected id=cus_123, got %v", body["id"])
	}
	if body["deleted"] != true {
		t.Errorf("expected deleted=true, got %v", body["deleted"])
	}

	// Item should STILL exist in the store
	getResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource:   "customers",
		Action:     stateful.ActionGet,
		ResourceID: "cus_123",
	})
	if getResult.Error != nil {
		t.Fatalf("item should still exist after preserve-delete, got error: %v", getResult.Error)
	}
	if getResult.Item == nil {
		t.Fatal("item should still exist after preserve-delete, got nil item")
	}
}

func TestHandleBindingDelete_DefaultRemovesItem(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	// Create an item first
	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_456", "name": "Bob"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/customers/cus_456", nil)

	// No preserve — default behavior
	status := h.handleBindingDelete(w, req, "", "customers", "cus_456", nil)

	// Should return default 204
	if status != 204 {
		t.Errorf("expected status 204, got %d", status)
	}

	// Item should be GONE from the store
	getResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource:   "customers",
		Action:     stateful.ActionGet,
		ResourceID: "cus_456",
	})
	if getResult.Error == nil {
		t.Fatal("item should be deleted, but GET succeeded")
	}
}

// ── extractLastPathParam tests ───────────────────────────────────────────────

func TestExtractLastPathParam_SingleParam(t *testing.T) {
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/v1/customers/{customer}",
			},
		},
	}
	pathParams := map[string]string{"customer": "cus_1"}
	got := extractLastPathParam(m, pathParams)
	if got != "cus_1" {
		t.Errorf("extractLastPathParam() = %q, want %q", got, "cus_1")
	}
}

func TestExtractLastPathParam_MultipleParams(t *testing.T) {
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/v1/customers/{customer}/subscriptions/{subscription}",
			},
		},
	}
	pathParams := map[string]string{"customer": "cus_1", "subscription": "sub_2"}
	got := extractLastPathParam(m, pathParams)
	if got != "sub_2" {
		t.Errorf("extractLastPathParam() = %q, want %q (last param)", got, "sub_2")
	}
}

func TestExtractLastPathParam_NoParams(t *testing.T) {
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/v1/customers",
			},
		},
	}
	pathParams := map[string]string{"customer": "cus_1"}
	got := extractLastPathParam(m, pathParams)
	if got != "" {
		t.Errorf("extractLastPathParam() = %q, want empty (no {param} in path)", got)
	}
}

func TestExtractLastPathParam_NilHTTP(t *testing.T) {
	m := &mock.Mock{}
	pathParams := map[string]string{"customer": "cus_1"}
	got := extractLastPathParam(m, pathParams)
	if got != "" {
		t.Errorf("extractLastPathParam() = %q, want empty (nil HTTP)", got)
	}
}

func TestExtractLastPathParam_EmptyPathParams(t *testing.T) {
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/v1/customers/{customer}",
			},
		},
	}
	pathParams := map[string]string{}
	got := extractLastPathParam(m, pathParams)
	if got != "" {
		t.Errorf("extractLastPathParam() = %q, want empty (param in path but not in pathParams)", got)
	}
}

func TestExtractLastPathParam_ParamWithSuffix(t *testing.T) {
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/2010-04-01/Accounts/{AccountSid}/Messages/{Sid}.json",
			},
		},
	}
	pathParams := map[string]string{"AccountSid": "AC_test", "Sid": "SM123abc"}
	got := extractLastPathParam(m, pathParams)
	if got != "SM123abc" {
		t.Errorf("extractLastPathParam() = %q, want %q (param with .json suffix)", got, "SM123abc")
	}
}

func TestExtractLastPathParam_ParamWithPrefix(t *testing.T) {
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/api/v{version}/items",
			},
		},
	}
	pathParams := map[string]string{"version": "3"}
	got := extractLastPathParam(m, pathParams)
	if got != "3" {
		t.Errorf("extractLastPathParam() = %q, want %q (param with prefix)", got, "3")
	}
}

// ── parseExpandFields tests ──────────────────────────────────────────────────

func TestParseExpandFields_BracketNotation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/charges?expand[]=data.source&expand[]=customer", nil)
	got := parseExpandFields(req)

	if len(got) != 2 {
		t.Fatalf("parseExpandFields() returned %d fields, want 2", len(got))
	}
	want := map[string]bool{"data.source": true, "customer": true}
	for _, field := range got {
		if !want[field] {
			t.Errorf("unexpected expand field %q", field)
		}
	}
}

func TestParseExpandFields_PlainNotation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/charges?expand=customer&expand=invoice", nil)
	got := parseExpandFields(req)

	if len(got) != 2 {
		t.Fatalf("parseExpandFields() returned %d fields, want 2", len(got))
	}
	want := map[string]bool{"customer": true, "invoice": true}
	for _, field := range got {
		if !want[field] {
			t.Errorf("unexpected expand field %q", field)
		}
	}
}

func TestParseExpandFields_NoExpand(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/charges?limit=10", nil)
	got := parseExpandFields(req)

	if len(got) != 0 {
		t.Errorf("parseExpandFields() returned %v, want empty slice", got)
	}
}

// ── bridgeStatusToHTTP tests ─────────────────────────────────────────────────

func TestBridgeStatusToHTTP(t *testing.T) {
	tests := []struct {
		name   string
		status stateful.ResultStatus
		want   int
	}{
		{"StatusNotFound → 404", stateful.StatusNotFound, http.StatusNotFound},
		{"StatusConflict → 409", stateful.StatusConflict, http.StatusConflict},
		{"StatusValidationError → 400", stateful.StatusValidationError, http.StatusBadRequest},
		{"StatusCapacityExceeded → 507", stateful.StatusCapacityExceeded, http.StatusInsufficientStorage},
		{"StatusError → 500", stateful.StatusError, http.StatusInternalServerError},
		{"StatusSuccess → 500 (default)", stateful.StatusSuccess, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bridgeStatusToHTTP(tt.status)
			if got != tt.want {
				t.Errorf("bridgeStatusToHTTP(%v) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}

// ── handleStatefulBinding dispatch tests ─────────────────────────────────────

func TestHandleStatefulBinding_Dispatch(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "customers", IDField: "id"})
	br := stateful.NewBridge(store)

	// Seed a test item
	br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/customers/{customer}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "customers",
				Action: "get",
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/customers/cus_1", nil)
	pathParams := map[string]string{"customer": "cus_1"}

	status := h.handleStatefulBinding(w, req, m, nil, pathParams)

	if status != http.StatusOK {
		t.Fatalf("handleStatefulBinding() status = %d, want %d", status, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] != "cus_1" {
		t.Errorf("response id = %v, want cus_1", body["id"])
	}
	if body["name"] != "Alice" {
		t.Errorf("response name = %v, want Alice", body["name"])
	}
}

func TestHandleStatefulBinding_NilBridge(t *testing.T) {
	h := &Handler{log: slog.Default(), statefulBridge: nil}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/customers/{customer}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "customers",
				Action: "get",
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/customers/cus_1", nil)
	pathParams := map[string]string{"customer": "cus_1"}

	status := h.handleStatefulBinding(w, req, m, nil, pathParams)

	if status != http.StatusServiceUnavailable {
		t.Fatalf("handleStatefulBinding() with nil bridge = %d, want %d", status, http.StatusServiceUnavailable)
	}
}

// ── handleBindingGet tests ───────────────────────────────────────────────────

func TestHandleBindingGet_Success(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/customers/cus_1", nil)

	status := h.handleBindingGet(w, req, "", "customers", "cus_1", nil)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] != "cus_1" {
		t.Errorf("expected id=cus_1, got %v", body["id"])
	}
	if body["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", body["name"])
	}
}

func TestHandleBindingGet_NotFound(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/customers/nonexistent", nil)

	status := h.handleBindingGet(w, req, "", "customers", "nonexistent", nil)

	if status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", status)
	}
}

func TestHandleBindingGet_WithResponseTransform(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	responseCfg := &config.ResponseTransform{
		Fields: &config.FieldTransform{
			Rename: map[string]string{"id": "customer_id"},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/customers/cus_1", nil)

	status := h.handleBindingGet(w, req, "", "customers", "cus_1", responseCfg)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["customer_id"] != "cus_1" {
		t.Errorf("expected customer_id=cus_1, got %v", body["customer_id"])
	}
	if _, exists := body["id"]; exists {
		t.Errorf("expected 'id' to be renamed away, but it still exists")
	}
	if body["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", body["name"])
	}
}

// ── handleBindingList tests ──────────────────────────────────────────────────

func TestHandleBindingList_Success(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	for _, d := range []map[string]interface{}{
		{"id": "cus_1", "name": "Alice"},
		{"id": "cus_2", "name": "Bob"},
	} {
		r := br.Execute(context.Background(), &stateful.OperationRequest{
			Resource: "customers",
			Action:   stateful.ActionCreate,
			Data:     d,
		})
		if r.Error != nil {
			t.Fatalf("failed to create item: %v", r.Error)
		}
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/customers", nil)

	status := h.handleBindingList(w, req, "", "customers", nil, nil)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data as array, got %T", body["data"])
	}
	if len(data) != 2 {
		t.Errorf("expected 2 items, got %d", len(data))
	}

	meta, ok := body["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta as object, got %T", body["meta"])
	}
	if total, ok := meta["total"].(float64); !ok || int(total) != 2 {
		t.Errorf("expected meta.total=2, got %v", meta["total"])
	}
}

func TestHandleBindingList_TableNotFound(t *testing.T) {
	store := stateful.NewStateStore()
	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bogus", nil)

	status := h.handleBindingList(w, req, "", "bogus", nil, nil)

	if status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", status)
	}
}

func TestHandleBindingList_WithPagination(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("cus_%d", i)
		r := br.Execute(context.Background(), &stateful.OperationRequest{
			Resource: "customers",
			Action:   stateful.ActionCreate,
			Data:     map[string]interface{}{"id": id, "name": "User"},
		})
		if r.Error != nil {
			t.Fatalf("failed to create item %d: %v", i, r.Error)
		}
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/customers?limit=2", nil)

	status := h.handleBindingList(w, req, "", "customers", nil, nil)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data as array, got %T", body["data"])
	}
	if len(data) != 2 {
		t.Errorf("expected 2 items (limit=2), got %d", len(data))
	}
}

// ── handleBindingCreate tests ────────────────────────────────────────────────

func TestHandleBindingCreate_Success(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{"name":"Bob"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingCreate(w, req, "", "customers", nil, bodyBytes, nil)

	if status != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", body["name"])
	}
	if body["id"] == nil || body["id"] == "" {
		t.Errorf("expected a generated id, got %v", body["id"])
	}
}

func TestHandleBindingCreate_InvalidBody(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{not valid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingCreate(w, req, "", "customers", nil, bodyBytes, nil)

	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", status)
	}
}

func TestHandleBindingCreate_BodyTooLarge(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingCreate(w, req, "", "customers", nil, largeBody, nil)

	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", status)
	}
}

// ── handleBindingMutate tests ────────────────────────────────────────────────

func TestHandleBindingMutate_PatchSuccess(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{"name":"Updated"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/customers/cus_1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingMutate(w, req, "", "customers", "cus_1", nil, bodyBytes, nil, stateful.ActionPatch)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["name"] != "Updated" {
		t.Errorf("expected name=Updated, got %v", body["name"])
	}
	if body["id"] != "cus_1" {
		t.Errorf("expected id=cus_1, got %v", body["id"])
	}
}

func TestHandleBindingMutate_NotFound(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{"name":"Updated"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/customers/nonexistent", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingMutate(w, req, "", "customers", "nonexistent", nil, bodyBytes, nil, stateful.ActionPatch)

	if status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", status)
	}
}

func TestHandleBindingMutate_InvalidBody(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{bad json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/customers/cus_1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingMutate(w, req, "", "customers", "cus_1", nil, bodyBytes, nil, stateful.ActionPatch)

	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", status)
	}
}

// ── handleBindingCustom tests ────────────────────────────────────────────────

func TestHandleBindingCustom_Success(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	// Create an item that the custom operation will read
	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	// Register a custom operation on the bridge
	br.RegisterCustomOperation("", "double-name", &stateful.CustomOperation{
		Name: "double-name",
		Steps: []stateful.Step{
			{Type: stateful.StepRead, Resource: "customers", ID: `input.userId`, As: "user"},
			{Type: stateful.StepSet, Var: "result", Value: `user.name + " " + user.name`},
		},
		Response: map[string]string{
			"doubled": "result",
		},
	})

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	binding := &mock.StatefulBinding{
		Table:     "customers",
		Action:    "custom",
		Operation: "double-name",
	}

	bodyBytes := []byte(`{"userId":"cus_1"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers/double-name", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingCustom(w, req, "", binding, nil, bodyBytes, nil)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["doubled"] != "Alice Alice" {
		t.Errorf("expected doubled='Alice Alice', got %v", body["doubled"])
	}
}

func TestHandleBindingCustom_MissingOperation(t *testing.T) {
	store := stateful.NewStateStore()
	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	binding := &mock.StatefulBinding{
		Table:     "customers",
		Action:    "custom",
		Operation: "", // empty — should fail
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers/action", nil)

	status := h.handleBindingCustom(w, req, "", binding, nil, nil, nil)

	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", status)
	}

	var body stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error == "" {
		t.Fatal("expected error message in response")
	}
	if !strings.Contains(body.Error, "requires an operation name") {
		t.Errorf("expected error about 'requires an operation name', got %q", body.Error)
	}
}

func TestHandleBindingCustom_EmptyBody(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	// Create an item
	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	// Register a custom operation that uses a path param instead of body input
	br.RegisterCustomOperation("", "get-by-path", &stateful.CustomOperation{
		Name: "get-by-path",
		Steps: []stateful.Step{
			{Type: stateful.StepRead, Resource: "customers", ID: `input.customerId`, As: "user"},
		},
		Response: map[string]string{
			"name": "user.name",
		},
	})

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	binding := &mock.StatefulBinding{
		Table:     "customers",
		Action:    "custom",
		Operation: "get-by-path",
	}

	// Empty bodyBytes — path params should be merged into input
	pathParams := map[string]string{"customerId": "cus_1"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers/cus_1/action", nil)
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingCustom(w, req, "", binding, pathParams, nil, nil)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", body["name"])
	}
}

// ── writeBindingError tests ──────────────────────────────────────────────────

func TestWriteBindingError_NotFound(t *testing.T) {
	h := &Handler{log: slog.Default()}

	result := &stateful.OperationResult{
		Status: stateful.StatusNotFound,
		Error:  &stateful.NotFoundError{Resource: "customers", ID: "cus_1"},
	}

	w := httptest.NewRecorder()
	status := h.writeBindingError(w, result, "customers", "cus_1", nil)

	if status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", status)
	}

	var body stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.StatusCode != http.StatusNotFound {
		t.Errorf("expected statusCode=404, got %d", body.StatusCode)
	}
}

func TestWriteBindingError_WithErrorTransform(t *testing.T) {
	h := &Handler{log: slog.Default()}

	result := &stateful.OperationResult{
		Status: stateful.StatusNotFound,
		Error:  &stateful.NotFoundError{Resource: "customers", ID: "cus_1"},
	}

	responseCfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Wrap: "error",
			Fields: map[string]string{
				"message": "message",
			},
		},
	}

	w := httptest.NewRecorder()
	status := h.writeBindingError(w, result, "customers", "cus_1", responseCfg)

	if status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	// Should be wrapped under "error" key
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error wrapped under 'error' key, got %v", body)
	}
	if errObj["message"] == nil || errObj["message"] == "" {
		t.Errorf("expected 'message' field in transformed error, got %v", errObj)
	}
}

// ── handleCustomOperation additional path tests ──────────────────────────────

func TestHandleCustomOperation_NilBridge(t *testing.T) {
	h := &Handler{
		log:            slog.Default(),
		statefulBridge: nil, // nil bridge
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/operations/test", nil)

	status := h.handleCustomOperation(w, req, "", "test-op", nil)

	if status != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", status)
	}
}

func TestHandleCustomOperation_InvalidJSON(t *testing.T) {
	store := stateful.NewStateStore()
	br := stateful.NewBridge(store)

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{not valid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/operations/test", bytes.NewReader(bodyBytes))

	status := h.handleCustomOperation(w, req, "", "test-op", bodyBytes)

	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", status)
	}
}

// ── handleStatefulBinding full CRUD integration ──────────────────────────────

func TestHandleStatefulBinding_FullCRUDFlow(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "products", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	// 1. CREATE via handleStatefulBinding
	createMock := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/products", Method: "POST"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "products",
				Action: "create",
			},
		},
	}
	createBody := []byte(`{"name":"Widget","price":1999}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/products", nil)
	req.Header.Set("Content-Type", "application/json")
	status := h.handleStatefulBinding(w, req, createMock, createBody, nil)
	if status != http.StatusCreated {
		t.Fatalf("CREATE: expected 201, got %d", status)
	}
	var created map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("CREATE decode: %v", err)
	}
	productID, ok := created["id"].(string)
	if !ok || productID == "" {
		t.Fatalf("CREATE: expected generated id, got %v", created["id"])
	}
	if created["name"] != "Widget" {
		t.Errorf("CREATE: expected name=Widget, got %v", created["name"])
	}

	// 2. LIST via handleStatefulBinding
	listMock := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/products", Method: "GET"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "products",
				Action: "list",
			},
		},
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/products", nil)
	status = h.handleStatefulBinding(w, req, listMock, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("LIST: expected 200, got %d", status)
	}
	var listBody map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&listBody); err != nil {
		t.Fatalf("LIST decode: %v", err)
	}
	data, ok := listBody["data"].([]interface{})
	if !ok {
		t.Fatalf("LIST: expected data as array, got %T", listBody["data"])
	}
	if len(data) != 1 {
		t.Errorf("LIST: expected 1 item, got %d", len(data))
	}

	// 3. GET via handleStatefulBinding
	getMock := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/products/{product}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "products",
				Action: "get",
			},
		},
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/products/"+productID, nil)
	status = h.handleStatefulBinding(w, req, getMock, nil, map[string]string{"product": productID})
	if status != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d", status)
	}
	var getBody map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&getBody); err != nil {
		t.Fatalf("GET decode: %v", err)
	}
	if getBody["id"] != productID {
		t.Errorf("GET: expected id=%s, got %v", productID, getBody["id"])
	}

	// 4. UPDATE (PUT) via handleStatefulBinding
	updateMock := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/products/{product}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "products",
				Action: "update",
			},
		},
	}
	updateBody := []byte(`{"name":"Super Widget","price":2999}`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/v1/products/"+productID, nil)
	req.Header.Set("Content-Type", "application/json")
	status = h.handleStatefulBinding(w, req, updateMock, updateBody, map[string]string{"product": productID})
	if status != http.StatusOK {
		t.Fatalf("UPDATE: expected 200, got %d", status)
	}
	var updateResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&updateResp); err != nil {
		t.Fatalf("UPDATE decode: %v", err)
	}
	if updateResp["name"] != "Super Widget" {
		t.Errorf("UPDATE: expected name=Super Widget, got %v", updateResp["name"])
	}

	// 5. DELETE via handleStatefulBinding
	deleteMock := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/products/{product}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "products",
				Action: "delete",
			},
		},
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/v1/products/"+productID, nil)
	status = h.handleStatefulBinding(w, req, deleteMock, nil, map[string]string{"product": productID})
	if status != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d", status)
	}

	// 6. Verify item is gone (GET should 404)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/products/"+productID, nil)
	status = h.handleStatefulBinding(w, req, getMock, nil, map[string]string{"product": productID})
	if status != http.StatusNotFound {
		t.Fatalf("GET after DELETE: expected 404, got %d", status)
	}
}

func TestHandleStatefulBinding_UnsupportedAction(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "bogus_action",
			},
		},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
	status := h.handleStatefulBinding(w, req, m, nil, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported action, got %d", status)
	}
}

func TestHandleStatefulBinding_MissingItemIDForGet(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	// GET binding with no path params — should fail because item ID is required
	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "get",
			},
		},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
	status := h.handleStatefulBinding(w, req, m, nil, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing item ID, got %d", status)
	}
}

func TestHandleStatefulBinding_FormEncodedCreate(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "customers", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/customers", Method: "POST"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "customers",
				Action: "create",
			},
		},
	}

	bodyBytes := []byte("name=Alice&email=alice%40example.com&active=true&balance=5000")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/customers", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status := h.handleStatefulBinding(w, req, m, bodyBytes, nil)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", body["name"])
	}
	if body["email"] != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %v", body["email"])
	}
	// Form values are coerced: "true" → bool, "5000" → int64
	if body["active"] != true {
		t.Errorf("expected active=true (bool), got %v (%T)", body["active"], body["active"])
	}
	// JSON encoding turns int64 into float64
	if body["balance"] != float64(5000) {
		t.Errorf("expected balance=5000 (float64 via JSON), got %v (%T)", body["balance"], body["balance"])
	}
}

func TestHandleStatefulBinding_PatchAction(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	// Seed an item
	br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "items",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "item_1", "name": "Original", "color": "red"},
	})

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items/{item}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "patch",
			},
		},
	}

	bodyBytes := []byte(`{"name":"Patched"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/items/item_1", nil)
	req.Header.Set("Content-Type", "application/json")
	status := h.handleStatefulBinding(w, req, m, bodyBytes, map[string]string{"item": "item_1"})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["name"] != "Patched" {
		t.Errorf("expected name=Patched, got %v", body["name"])
	}
}

func TestHandleStatefulBinding_ListWithFieldFilter(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	for _, d := range []map[string]interface{}{
		{"id": "1", "status": "active", "type": "a"},
		{"id": "2", "status": "inactive", "type": "a"},
		{"id": "3", "status": "active", "type": "b"},
	} {
		br.Execute(context.Background(), &stateful.OperationRequest{
			Resource: "items", Action: stateful.ActionCreate, Data: d,
		})
	}

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "list",
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items?status=active", nil)
	status := h.handleStatefulBinding(w, req, m, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data as array, got %T", body["data"])
	}
	if len(data) != 2 {
		t.Errorf("expected 2 active items, got %d", len(data))
	}
}

func TestHandleStatefulBinding_ListWithSortAndOrder(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	for _, d := range []map[string]interface{}{
		{"id": "aaa", "name": "Charlie"},
		{"id": "bbb", "name": "Alice"},
		{"id": "ccc", "name": "Bob"},
	} {
		br.Execute(context.Background(), &stateful.OperationRequest{
			Resource: "items", Action: stateful.ActionCreate, Data: d,
		})
	}

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "list",
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items?sort=name&order=asc", nil)
	status := h.handleStatefulBinding(w, req, m, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data as array, got %T", body["data"])
	}
	if len(data) != 3 {
		t.Fatalf("expected 3 items, got %d", len(data))
	}

	// Verify sort order: Alice, Bob, Charlie
	first := data[0].(map[string]interface{})
	last := data[2].(map[string]interface{})
	if first["name"] != "Alice" {
		t.Errorf("expected first item name=Alice (asc sort), got %v", first["name"])
	}
	if last["name"] != "Charlie" {
		t.Errorf("expected last item name=Charlie (asc sort), got %v", last["name"])
	}
}

func TestHandleStatefulBinding_ListWithLimitOffset(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	for i := 0; i < 10; i++ {
		br.Execute(context.Background(), &stateful.OperationRequest{
			Resource: "items", Action: stateful.ActionCreate,
			Data: map[string]interface{}{"id": fmt.Sprintf("item_%02d", i), "seq": i},
		})
	}

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "list",
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items?limit=3&offset=2", nil)
	status := h.handleStatefulBinding(w, req, m, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data as array, got %T", body["data"])
	}
	if len(data) != 3 {
		t.Errorf("expected 3 items (limit=3), got %d", len(data))
	}

	meta, ok := body["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta as object, got %T", body["meta"])
	}
	if total, ok := meta["total"].(float64); !ok || int(total) != 10 {
		t.Errorf("expected meta.total=10, got %v", meta["total"])
	}
}

func TestHandleStatefulBinding_UpdateNotFound(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items/{item}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "update",
			},
		},
	}

	bodyBytes := []byte(`{"name":"Updated"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/items/nonexistent", nil)
	req.Header.Set("Content-Type", "application/json")
	status := h.handleStatefulBinding(w, req, m, bodyBytes, map[string]string{"item": "nonexistent"})
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", status)
	}
}

func TestHandleStatefulBinding_DeleteNotFound(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items/{item}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "delete",
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/items/nonexistent", nil)
	status := h.handleStatefulBinding(w, req, m, nil, map[string]string{"item": "nonexistent"})
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", status)
	}
}

func TestHandleStatefulBinding_CreateBodyTooLarge(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/items", Method: "POST"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "items",
				Action: "create",
			},
		},
	}

	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/items", nil)
	req.Header.Set("Content-Type", "application/json")
	status := h.handleStatefulBinding(w, req, m, largeBody, nil)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", status)
	}
}

func TestHandleStatefulBinding_WithResponseTransformOverride(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "customers", IDField: "id"})
	br := stateful.NewBridge(store)

	br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})

	h := &Handler{log: slog.Default(), statefulBridge: br}

	responseCfg := &config.ResponseTransform{
		Fields: &config.FieldTransform{
			Inject: map[string]interface{}{"object": "customer"},
		},
	}

	m := &mock.Mock{
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Path: "/v1/customers/{customer}"},
			StatefulBinding: &mock.StatefulBinding{
				Table:  "customers",
				Action: "get",
				Response: &mock.StatefulBindingResponse{
					Transform: responseCfg,
				},
			},
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/customers/cus_1", nil)
	status := h.handleStatefulBinding(w, req, m, nil, map[string]string{"customer": "cus_1"})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["object"] != "customer" {
		t.Errorf("expected injected object=customer, got %v", body["object"])
	}
	if body["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", body["name"])
	}
}

// ── handleBindingMutate PUT vs PATCH tests ───────────────────────────────────

func TestHandleBindingMutate_UpdateReplacesAllFields(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "items",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "item_1", "name": "Original", "color": "red", "size": "large"},
	})

	h := &Handler{log: slog.Default(), statefulBridge: br}

	// PUT/update replaces entire data — fields not in body are removed
	bodyBytes := []byte(`{"name":"Replaced"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/items/item_1", nil)
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingMutate(w, req, "", "items", "item_1", nil, bodyBytes, nil, stateful.ActionUpdate)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["name"] != "Replaced" {
		t.Errorf("expected name=Replaced, got %v", body["name"])
	}
	// color and size should be gone (PUT semantics)
	if _, exists := body["color"]; exists {
		t.Errorf("expected 'color' to be removed by PUT, but it still exists")
	}
}

func TestHandleBindingMutate_BodyTooLarge(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/items/item_1", nil)
	req.Header.Set("Content-Type", "application/json")

	status := h.handleBindingMutate(w, req, "", "items", "item_1", nil, largeBody, nil, stateful.ActionUpdate)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", status)
	}
}

func TestHandleBindingCreate_FormEncoded(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "customers", IDField: "id"})
	br := stateful.NewBridge(store)

	h := &Handler{log: slog.Default(), statefulBridge: br}

	bodyBytes := []byte("name=Bob&email=bob%40example.com&metadata[tier]=premium")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	status := h.handleBindingCreate(w, req, "", "customers", nil, bodyBytes, nil)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", body["name"])
	}
	if body["email"] != "bob@example.com" {
		t.Errorf("expected email=bob@example.com, got %v", body["email"])
	}
	// Check nested form data
	meta, ok := body["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata as map, got %T", body["metadata"])
	}
	if meta["tier"] != "premium" {
		t.Errorf("expected metadata.tier=premium, got %v", meta["tier"])
	}
}

// ── handleCustomOperation additional tests ───────────────────────────────────

func TestHandleCustomOperation_EmptyBody(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{Name: "items", IDField: "id"})
	br := stateful.NewBridge(store)

	// Register a simple operation with a set step (no body needed)
	br.RegisterCustomOperation("", "noop", &stateful.CustomOperation{
		Name: "noop",
		Steps: []stateful.Step{
			{Type: stateful.StepSet, Var: "msg", Value: `"hello"`},
		},
		Response: map[string]string{
			"message": "msg",
		},
	})

	h := &Handler{log: slog.Default(), statefulBridge: br}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/operations/noop", nil)

	// nil bodyBytes — operation should succeed with empty input
	status := h.handleCustomOperation(w, req, "", "noop", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["message"] != "hello" {
		t.Errorf("expected message=hello, got %v", body["message"])
	}
}

func TestHandleCustomOperation_Success(t *testing.T) {
	store := stateful.NewStateStore()
	_ = store.Register("", &stateful.ResourceConfig{
		Name:    "customers",
		IDField: "id",
	})

	br := stateful.NewBridge(store)

	// Create an item
	createResult := br.Execute(context.Background(), &stateful.OperationRequest{
		Resource: "customers",
		Action:   stateful.ActionCreate,
		Data:     map[string]interface{}{"id": "cus_1", "name": "Alice"},
	})
	if createResult.Error != nil {
		t.Fatalf("failed to create item: %v", createResult.Error)
	}

	// Register a custom operation
	br.RegisterCustomOperation("", "greet", &stateful.CustomOperation{
		Name: "greet",
		Steps: []stateful.Step{
			{Type: stateful.StepRead, Resource: "customers", ID: `input.userId`, As: "user"},
			{Type: stateful.StepSet, Var: "greeting", Value: `"Hello " + user.name`},
		},
		Response: map[string]string{
			"message": "greeting",
		},
	})

	h := &Handler{
		log:            slog.Default(),
		statefulBridge: br,
	}

	bodyBytes := []byte(`{"userId":"cus_1"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/operations/greet", bytes.NewReader(bodyBytes))

	status := h.handleCustomOperation(w, req, "", "greet", bodyBytes)

	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["message"] != "Hello Alice" {
		t.Errorf("expected message='Hello Alice', got %v", body["message"])
	}
}
