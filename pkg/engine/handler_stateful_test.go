package engine

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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

	status := h.handleCustomOperation(w, req, "TransferFunds", largeBody)
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
		{name: "inf stays string", input: "inf", want: "inf"},
		{name: "mixed alphanumeric stays string", input: "123abc", want: "123abc"},
		{name: "large int", input: "1000000000000", want: int64(1000000000000)},
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
