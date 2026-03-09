package stateful

import (
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

func TestTransformItem_NilConfig(t *testing.T) {
	data := map[string]interface{}{"id": "123", "name": "Alice"}
	result := TransformItem(data, nil)
	if result["id"] != "123" || result["name"] != "Alice" {
		t.Errorf("expected unchanged data, got %v", result)
	}
}

func TestTransformItem_NilData(t *testing.T) {
	cfg := &config.ResponseTransform{}
	result := TransformItem(nil, cfg)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTransformItem_Rename(t *testing.T) {
	data := map[string]interface{}{"firstName": "Alice", "lastName": "Johnson"}
	cfg := &config.ResponseTransform{
		Fields: &config.FieldTransform{
			Rename: map[string]string{"firstName": "first_name", "lastName": "last_name"},
		},
	}
	result := TransformItem(data, cfg)
	if result["first_name"] != "Alice" {
		t.Errorf("expected first_name=Alice, got %v", result["first_name"])
	}
	if result["last_name"] != "Johnson" {
		t.Errorf("expected last_name=Johnson, got %v", result["last_name"])
	}
	if _, ok := result["firstName"]; ok {
		t.Error("expected firstName to be removed after rename")
	}
}

func TestTransformItem_Hide(t *testing.T) {
	data := map[string]interface{}{"id": "1", "name": "Alice", "secret": "shhh", "internal": "meta"}
	cfg := &config.ResponseTransform{
		Fields: &config.FieldTransform{
			Hide: []string{"secret", "internal"},
		},
	}
	result := TransformItem(data, cfg)
	if _, ok := result["secret"]; ok {
		t.Error("expected secret to be hidden")
	}
	if _, ok := result["internal"]; ok {
		t.Error("expected internal to be hidden")
	}
	if result["name"] != "Alice" {
		t.Error("expected name to remain")
	}
}

func TestTransformItem_Inject(t *testing.T) {
	data := map[string]interface{}{"id": "1", "name": "Alice"}
	cfg := &config.ResponseTransform{
		Fields: &config.FieldTransform{
			Inject: map[string]interface{}{
				"object":   "customer",
				"livemode": false,
			},
		},
	}
	result := TransformItem(data, cfg)
	if result["object"] != "customer" {
		t.Errorf("expected object=customer, got %v", result["object"])
	}
	if result["livemode"] != false {
		t.Errorf("expected livemode=false, got %v", result["livemode"])
	}
}

func TestTransformItem_TimestampsUnix(t *testing.T) {
	ts := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	data := map[string]interface{}{
		"id":        "1",
		"createdAt": ts.Format(time.RFC3339Nano),
		"updatedAt": ts.Format(time.RFC3339Nano),
	}
	cfg := &config.ResponseTransform{
		Timestamps: &config.TimestampTransform{
			Format: "unix",
		},
	}
	result := TransformItem(data, cfg)
	if result["createdAt"] != int64(1772971200) {
		t.Errorf("expected unix timestamp 1772971200, got %v (type %T)", result["createdAt"], result["createdAt"])
	}
}

func TestTransformItem_TimestampsRename(t *testing.T) {
	ts := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	data := map[string]interface{}{
		"id":        "1",
		"createdAt": ts.Format(time.RFC3339Nano),
		"updatedAt": ts.Format(time.RFC3339Nano),
	}
	cfg := &config.ResponseTransform{
		Timestamps: &config.TimestampTransform{
			Format: "unix",
			Fields: map[string]string{
				"createdAt": "created",
				"updatedAt": "updated",
			},
		},
	}
	result := TransformItem(data, cfg)
	if _, ok := result["createdAt"]; ok {
		t.Error("expected createdAt to be renamed")
	}
	if _, ok := result["updatedAt"]; ok {
		t.Error("expected updatedAt to be renamed")
	}
	if result["created"] != int64(1772971200) {
		t.Errorf("expected created=1772971200, got %v", result["created"])
	}
	if result["updated"] != int64(1772971200) {
		t.Errorf("expected updated=1772971200, got %v", result["updated"])
	}
}

func TestTransformItem_TimestampsNone(t *testing.T) {
	ts := time.Now().Format(time.RFC3339Nano)
	data := map[string]interface{}{
		"id": "1", "name": "Alice",
		"createdAt": ts, "updatedAt": ts,
	}
	cfg := &config.ResponseTransform{
		Timestamps: &config.TimestampTransform{Format: "none"},
	}
	result := TransformItem(data, cfg)
	if _, ok := result["createdAt"]; ok {
		t.Error("expected createdAt to be removed with format=none")
	}
	if _, ok := result["updatedAt"]; ok {
		t.Error("expected updatedAt to be removed with format=none")
	}
	if result["name"] != "Alice" {
		t.Error("expected other fields to remain")
	}
}

func TestTransformItem_TimestampsISO8601(t *testing.T) {
	ts := time.Date(2026, 3, 8, 12, 30, 45, 123456789, time.UTC)
	data := map[string]interface{}{
		"id":        "1",
		"createdAt": ts.Format(time.RFC3339Nano),
	}
	cfg := &config.ResponseTransform{
		Timestamps: &config.TimestampTransform{Format: "iso8601"},
	}
	result := TransformItem(data, cfg)
	expected := "2026-03-08T12:30:45Z"
	if result["createdAt"] != expected {
		t.Errorf("expected %q, got %v", expected, result["createdAt"])
	}
}

func TestTransformItem_FullStripeStyle(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	data := map[string]interface{}{
		"id":            "cus_abc123",
		"name":          "Alice Johnson",
		"email":         "alice@example.com",
		"internalNotes": "VIP customer",
		"createdAt":     ts.Format(time.RFC3339Nano),
		"updatedAt":     ts.Format(time.RFC3339Nano),
	}
	cfg := &config.ResponseTransform{
		Timestamps: &config.TimestampTransform{
			Format: "unix",
			Fields: map[string]string{
				"createdAt": "created",
				"updatedAt": "updated",
			},
		},
		Fields: &config.FieldTransform{
			Inject: map[string]interface{}{
				"object":   "customer",
				"livemode": false,
			},
			Hide:   []string{"internalNotes"},
			Rename: map[string]string{},
		},
	}
	result := TransformItem(data, cfg)

	// Check injected fields
	if result["object"] != "customer" {
		t.Errorf("expected object=customer, got %v", result["object"])
	}
	if result["livemode"] != false {
		t.Errorf("expected livemode=false, got %v", result["livemode"])
	}
	// Check hidden fields
	if _, ok := result["internalNotes"]; ok {
		t.Error("expected internalNotes to be hidden")
	}
	// Check timestamp conversion + rename
	if result["created"] != int64(1768471200) {
		t.Errorf("expected created=1768471200, got %v", result["created"])
	}
	// Check original data preserved
	if result["name"] != "Alice Johnson" {
		t.Errorf("expected name=Alice Johnson, got %v", result["name"])
	}
}

func TestTransformItem_TransformOrder(t *testing.T) {
	// Verify: rename happens before hide, inject happens last
	data := map[string]interface{}{
		"id": "1", "oldName": "Alice",
		"createdAt": time.Now().Format(time.RFC3339Nano),
		"updatedAt": time.Now().Format(time.RFC3339Nano),
	}
	cfg := &config.ResponseTransform{
		Fields: &config.FieldTransform{
			Rename: map[string]string{"oldName": "name"},
			Hide:   []string{"oldName"}, // should be a no-op since oldName was renamed
			Inject: map[string]interface{}{"type": "user"},
		},
	}
	result := TransformItem(data, cfg)
	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice (renamed from oldName), got %v", result["name"])
	}
	if result["type"] != "user" {
		t.Errorf("expected type=user (injected), got %v", result["type"])
	}
}

// --- List transform tests ---

func TestTransformList_NilConfig(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}
	meta := PaginationMeta{Total: 2, Limit: 100, Offset: 0, Count: 2}
	result := TransformList(items, meta, nil)

	resp, ok := result.(*PaginatedResponse)
	if !ok {
		t.Fatalf("expected *PaginatedResponse, got %T", result)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Meta.Total)
	}
}

func TestTransformList_CustomEnvelope(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "1", "name": "Alice"},
	}
	meta := PaginationMeta{Total: 1, Limit: 10, Offset: 0, Count: 1}
	cfg := &config.ResponseTransform{
		List: &config.ListTransform{
			DataField: "results",
			ExtraFields: map[string]interface{}{
				"object": "list",
				"url":    "/v1/customers",
			},
			MetaFields: map[string]string{
				"total": "total_count",
			},
		},
	}
	result := TransformList(items, meta, cfg)

	envelope, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if envelope["object"] != "list" {
		t.Errorf("expected object=list, got %v", envelope["object"])
	}
	if envelope["url"] != "/v1/customers" {
		t.Errorf("expected url=/v1/customers, got %v", envelope["url"])
	}
	if envelope["total_count"] != 1 {
		t.Errorf("expected total_count=1, got %v", envelope["total_count"])
	}
	itemsArr, ok := envelope["results"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected results as []map, got %T", envelope["results"])
	}
	if len(itemsArr) != 1 {
		t.Errorf("expected 1 item, got %d", len(itemsArr))
	}
}

func TestTransformList_HideMeta(t *testing.T) {
	items := []map[string]interface{}{{"id": "1"}}
	meta := PaginationMeta{Total: 1, Limit: 100, Offset: 0, Count: 1}
	cfg := &config.ResponseTransform{
		List: &config.ListTransform{
			HideMeta: true,
		},
	}
	result := TransformList(items, meta, cfg)
	envelope, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	// Should have data but no meta fields
	if _, ok := envelope["total"]; ok {
		t.Error("expected meta to be hidden")
	}
	if _, ok := envelope["data"]; !ok {
		t.Error("expected data field to be present")
	}
}

func TestTransformList_ItemTransformsApplied(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	items := []map[string]interface{}{
		{"id": "1", "name": "Alice", "createdAt": ts, "updatedAt": ts},
	}
	meta := PaginationMeta{Total: 1, Limit: 100, Offset: 0, Count: 1}
	cfg := &config.ResponseTransform{
		Timestamps: &config.TimestampTransform{Format: "unix"},
		Fields:     &config.FieldTransform{Inject: map[string]interface{}{"object": "user"}},
	}
	result := TransformList(items, meta, cfg)
	resp, ok := result.(*PaginatedResponse)
	if !ok {
		t.Fatalf("expected *PaginatedResponse, got %T", result)
	}
	item := resp.Data[0]
	if item["object"] != "user" {
		t.Errorf("expected inject to apply to list items, got %v", item["object"])
	}
	if _, isInt := item["createdAt"].(int64); !isInt {
		t.Errorf("expected unix timestamp on list items, got %T", item["createdAt"])
	}
}

// --- Delete transform tests ---

func TestTransformDeleteResponse_NilConfig(t *testing.T) {
	status, body := TransformDeleteResponse(nil, nil)
	if status != 204 {
		t.Errorf("expected 204, got %d", status)
	}
	if body != nil {
		t.Errorf("expected nil body, got %v", body)
	}
}

func TestTransformDeleteResponse_CustomStatus(t *testing.T) {
	cfg := &config.ResponseTransform{
		Delete: &config.VerbOverride{Status: 200},
	}
	status, body := TransformDeleteResponse(nil, cfg)
	if status != 200 {
		t.Errorf("expected 200, got %d", status)
	}
	if body != nil {
		t.Errorf("expected nil body, got %v", body)
	}
}

func TestTransformDeleteResponse_CustomBody(t *testing.T) {
	item := &ResourceItem{
		ID:   "cus_abc123",
		Data: map[string]interface{}{"name": "Alice"},
	}
	cfg := &config.ResponseTransform{
		Delete: &config.VerbOverride{
			Status: 200,
			Body: map[string]interface{}{
				"id":      "{{item.id}}",
				"object":  "customer",
				"deleted": true,
			},
		},
	}
	status, body := TransformDeleteResponse(item, cfg)
	if status != 200 {
		t.Errorf("expected 200, got %d", status)
	}
	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map body, got %T", body)
	}
	if bodyMap["id"] != "cus_abc123" {
		t.Errorf("expected id=cus_abc123 from template, got %v", bodyMap["id"])
	}
	if bodyMap["object"] != "customer" {
		t.Errorf("expected object=customer, got %v", bodyMap["object"])
	}
	if bodyMap["deleted"] != true {
		t.Errorf("expected deleted=true, got %v", bodyMap["deleted"])
	}
}

// --- Create status tests ---

func TestTransformCreateStatus_NilConfig(t *testing.T) {
	if s := TransformCreateStatus(nil); s != 201 {
		t.Errorf("expected 201, got %d", s)
	}
}

func TestTransformCreateStatus_Override(t *testing.T) {
	cfg := &config.ResponseTransform{
		Create: &config.VerbOverride{Status: 200},
	}
	if s := TransformCreateStatus(cfg); s != 200 {
		t.Errorf("expected 200, got %d", s)
	}
}

// --- Template variable resolution tests ---

func TestResolveTemplateValue_StringWithTemplate(t *testing.T) {
	itemData := map[string]interface{}{"id": "abc123", "name": "Alice"}
	result := resolveTemplateValue("{{item.id}}", itemData)
	if result != "abc123" {
		t.Errorf("expected abc123, got %v", result)
	}
}

func TestResolveTemplateValue_NoTemplate(t *testing.T) {
	result := resolveTemplateValue("static value", nil)
	if result != "static value" {
		t.Errorf("expected static value, got %v", result)
	}
}

func TestResolveTemplateValue_NonString(t *testing.T) {
	result := resolveTemplateValue(42, nil)
	if result != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestResolveTemplateValue_MissingField(t *testing.T) {
	itemData := map[string]interface{}{"id": "123"}
	result := resolveTemplateValue("{{item.missing}}", itemData)
	if result != "{{item.missing}}" {
		t.Errorf("expected unresolved template, got %v", result)
	}
}

// ── TransformError tests ─────────────────────────────────────────────────────

func TestTransformError_NilConfig(t *testing.T) {
	result := TransformError(ErrCodeNotFound, "not found", "users", "user-1", "", nil)
	if result != nil {
		t.Errorf("expected nil for nil config, got %v", result)
	}
}

func TestTransformError_NilErrors(t *testing.T) {
	cfg := &config.ResponseTransform{} // Errors is nil
	result := TransformError(ErrCodeNotFound, "not found", "users", "user-1", "", cfg)
	if result != nil {
		t.Errorf("expected nil when Errors is nil, got %v", result)
	}
}

func TestTransformError_DefaultFields(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{},
	}
	result := TransformError(ErrCodeNotFound, "not found", "users", "user-1", "", cfg)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["message"] != "not found" {
		t.Errorf("expected message='not found', got %v", result["message"])
	}
	if result["resource"] != "users" {
		t.Errorf("expected resource='users', got %v", result["resource"])
	}
	if result["id"] != "user-1" {
		t.Errorf("expected id='user-1', got %v", result["id"])
	}
}

func TestTransformError_EmptyFieldsOmitted(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{},
	}
	// Pass empty field and id — they should not appear in output
	result := TransformError(ErrCodeNotFound, "not found", "", "", "", cfg)
	if _, ok := result["resource"]; ok {
		t.Error("empty resource should be omitted")
	}
	if _, ok := result["id"]; ok {
		t.Error("empty id should be omitted")
	}
	if _, ok := result["field"]; ok {
		t.Error("empty field should be omitted")
	}
}

func TestTransformError_CustomFieldMapping(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Fields: map[string]string{
				"message": "msg",
				"id":      "item_id",
			},
		},
	}
	result := TransformError(ErrCodeNotFound, "not found", "users", "user-1", "", cfg)
	if result["msg"] != "not found" {
		t.Errorf("expected msg='not found', got %v", result["msg"])
	}
	if result["item_id"] != "user-1" {
		t.Errorf("expected item_id='user-1', got %v", result["item_id"])
	}
	// Old field names should not be present
	if _, ok := result["message"]; ok {
		t.Error("default 'message' key should not be present when mapped")
	}
}

func TestTransformError_Wrap(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Wrap: "error",
		},
	}
	result := TransformError(ErrCodeNotFound, "not found", "users", "user-1", "", cfg)
	errObj, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected wrapped error object, got %T", result["error"])
	}
	if errObj["message"] != "not found" {
		t.Errorf("expected message='not found', got %v", errObj["message"])
	}
}

func TestTransformError_Inject(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Inject: map[string]interface{}{
				"doc_url": "https://docs.example.com/errors",
				"status":  "error",
			},
		},
	}
	result := TransformError(ErrCodeNotFound, "not found", "", "", "", cfg)
	if result["doc_url"] != "https://docs.example.com/errors" {
		t.Errorf("expected injected doc_url, got %v", result["doc_url"])
	}
	if result["status"] != "error" {
		t.Errorf("expected injected status='error', got %v", result["status"])
	}
}

func TestTransformError_TypeMap(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Fields: map[string]string{
				"type": "type",
			},
			TypeMap: map[string]string{
				"NOT_FOUND":        "invalid_request_error",
				"CONFLICT":         "idempotency_error",
				"VALIDATION_ERROR": "invalid_request_error",
			},
		},
	}
	result := TransformError(ErrCodeNotFound, "not found", "", "", "", cfg)
	if result["type"] != "invalid_request_error" {
		t.Errorf("expected type='invalid_request_error', got %v", result["type"])
	}
}

func TestTransformError_CodeMap(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Fields: map[string]string{
				"code": "code",
			},
			CodeMap: map[string]string{
				"NOT_FOUND": "resource_missing",
				"CONFLICT":  "resource_already_exists",
			},
		},
	}
	result := TransformError(ErrCodeNotFound, "not found", "", "", "", cfg)
	if result["code"] != "resource_missing" {
		t.Errorf("expected code='resource_missing', got %v", result["code"])
	}
}

func TestTransformError_StripeStyle(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Wrap: "error",
			Fields: map[string]string{
				"message": "message",
				"type":    "type",
				"code":    "code",
			},
			TypeMap: map[string]string{
				"NOT_FOUND": "invalid_request_error",
			},
			CodeMap: map[string]string{
				"NOT_FOUND": "resource_missing",
			},
			Inject: map[string]interface{}{
				"doc_url": "https://stripe.com/docs/error-codes/resource-missing",
			},
		},
	}
	result := TransformError(ErrCodeNotFound, "No such customer: cus_xxx", "customers", "cus_xxx", "", cfg)
	errObj, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected wrapped error, got %T", result["error"])
	}
	if errObj["message"] != "No such customer: cus_xxx" {
		t.Errorf("expected message, got %v", errObj["message"])
	}
	if errObj["type"] != "invalid_request_error" {
		t.Errorf("expected type='invalid_request_error', got %v", errObj["type"])
	}
	if errObj["code"] != "resource_missing" {
		t.Errorf("expected code='resource_missing', got %v", errObj["code"])
	}
	if errObj["doc_url"] != "https://stripe.com/docs/error-codes/resource-missing" {
		t.Errorf("expected doc_url injected, got %v", errObj["doc_url"])
	}
}

func TestTransformError_FieldValidationError(t *testing.T) {
	cfg := &config.ResponseTransform{
		Errors: &config.ErrorTransform{
			Fields: map[string]string{
				"message": "message",
				"field":   "param",
			},
		},
	}
	result := TransformError(ErrCodeValidation, "required", "", "", "email", cfg)
	if result["message"] != "required" {
		t.Errorf("expected message='required', got %v", result["message"])
	}
	if result["param"] != "email" {
		t.Errorf("expected param='email', got %v", result["param"])
	}
}

// ── TransformList computed has_more tests ─────────────────────────────────────

func TestTransformList_ComputedHasMore_True(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "1", "name": "Alice"},
	}
	meta := PaginationMeta{Total: 5, Limit: 1, Offset: 0, Count: 1, HasMore: true}
	cfg := &config.ResponseTransform{
		List: &config.ListTransform{
			ExtraFields: map[string]interface{}{
				"object":   "list",
				"has_more": false, // Static value — should be overridden by computed
			},
			HideMeta: true,
		},
	}
	result := TransformList(items, meta, cfg)
	envelope, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if envelope["has_more"] != true {
		t.Errorf("expected has_more=true (computed from meta), got %v", envelope["has_more"])
	}
}

func TestTransformList_ComputedHasMore_False(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "1", "name": "Alice"},
	}
	meta := PaginationMeta{Total: 1, Limit: 10, Offset: 0, Count: 1, HasMore: false}
	cfg := &config.ResponseTransform{
		List: &config.ListTransform{
			ExtraFields: map[string]interface{}{
				"has_more": true, // Static value — should be overridden
			},
			HideMeta: true,
		},
	}
	result := TransformList(items, meta, cfg)
	envelope, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if envelope["has_more"] != false {
		t.Errorf("expected has_more=false (computed), got %v", envelope["has_more"])
	}
}

func TestTransformList_ComputedHasMore_CamelCase(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "1"},
	}
	meta := PaginationMeta{Total: 10, Limit: 1, Offset: 0, Count: 1, HasMore: true}
	cfg := &config.ResponseTransform{
		List: &config.ListTransform{
			ExtraFields: map[string]interface{}{
				"hasMore": false,
			},
			HideMeta: true,
		},
	}
	result := TransformList(items, meta, cfg)
	envelope := result.(map[string]interface{})
	if envelope["hasMore"] != true {
		t.Errorf("expected hasMore=true (camelCase key), got %v", envelope["hasMore"])
	}
}

func TestTransformList_NonHasMoreExtraFields(t *testing.T) {
	items := []map[string]interface{}{{"id": "1"}}
	meta := PaginationMeta{Total: 1, Limit: 10, Offset: 0, Count: 1}
	cfg := &config.ResponseTransform{
		List: &config.ListTransform{
			ExtraFields: map[string]interface{}{
				"object": "list",
				"url":    "/v1/items",
			},
			HideMeta: true,
		},
	}
	result := TransformList(items, meta, cfg)
	envelope := result.(map[string]interface{})
	if envelope["object"] != "list" {
		t.Errorf("expected object='list', got %v", envelope["object"])
	}
	if envelope["url"] != "/v1/items" {
		t.Errorf("expected url='/v1/items', got %v", envelope["url"])
	}
}
