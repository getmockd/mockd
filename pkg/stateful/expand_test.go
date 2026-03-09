package stateful

import (
	"testing"
)

// =============================================================================
// ExpandRelationships Tests
// =============================================================================

func TestExpandRelationships_Basic(t *testing.T) {
	// Set up a "customers" resource with one item
	store := NewStateStore()
	_ = store.Register(&ResourceConfig{
		Name: "customers",
		SeedData: []map[string]interface{}{
			{"id": "cus_123", "name": "Alice", "email": "alice@example.com"},
		},
	})

	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	resolver := func(tableName string) *StatefulResource {
		return store.Get(tableName)
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_123",
		"status":   "active",
	}

	result := ExpandRelationships(item, []string{"customer"}, rels, resolver)

	// The "customer" field should now be a map (expanded object)
	expanded, ok := result["customer"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected customer to be a map, got %T", result["customer"])
	}
	if expanded["id"] != "cus_123" {
		t.Errorf("expected customer.id = cus_123, got %v", expanded["id"])
	}
	if expanded["name"] != "Alice" {
		t.Errorf("expected customer.name = Alice, got %v", expanded["name"])
	}
	if expanded["email"] != "alice@example.com" {
		t.Errorf("expected customer.email = alice@example.com, got %v", expanded["email"])
	}

	// Other fields should be unchanged
	if result["id"] != "sub_1" {
		t.Errorf("expected id = sub_1, got %v", result["id"])
	}
	if result["status"] != "active" {
		t.Errorf("expected status = active, got %v", result["status"])
	}
}

func TestExpandRelationships_Multiple(t *testing.T) {
	store := NewStateStore()
	_ = store.Register(&ResourceConfig{
		Name: "customers",
		SeedData: []map[string]interface{}{
			{"id": "cus_1", "name": "Alice"},
		},
	})
	_ = store.Register(&ResourceConfig{
		Name: "invoices",
		SeedData: []map[string]interface{}{
			{"id": "inv_1", "amount": 5000},
		},
	})

	rels := map[string]*RelationshipInfo{
		"customer":       {Table: "customers"},
		"latest_invoice": {Table: "invoices"},
	}

	resolver := func(tableName string) *StatefulResource {
		return store.Get(tableName)
	}

	item := map[string]interface{}{
		"id":             "sub_1",
		"customer":       "cus_1",
		"latest_invoice": "inv_1",
	}

	result := ExpandRelationships(item, []string{"customer", "latest_invoice"}, rels, resolver)

	// Both fields should be expanded
	cust, ok := result["customer"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected customer to be a map, got %T", result["customer"])
	}
	if cust["name"] != "Alice" {
		t.Errorf("expected customer.name = Alice, got %v", cust["name"])
	}

	inv, ok := result["latest_invoice"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected latest_invoice to be a map, got %T", result["latest_invoice"])
	}
	if inv["amount"] != 5000 {
		t.Errorf("expected latest_invoice.amount = 5000, got %v", inv["amount"])
	}
}

func TestExpandRelationships_MissingRelationship(t *testing.T) {
	store := NewStateStore()
	_ = store.Register(&ResourceConfig{
		Name: "customers",
		SeedData: []map[string]interface{}{
			{"id": "cus_1", "name": "Alice"},
		},
	})

	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	resolver := func(tableName string) *StatefulResource {
		return store.Get(tableName)
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_1",
		"plan":     "plan_gold",
	}

	// Request expansion of a field with no relationship defined
	result := ExpandRelationships(item, []string{"plan"}, rels, resolver)

	// "plan" should be unchanged
	if result["plan"] != "plan_gold" {
		t.Errorf("expected plan = plan_gold, got %v", result["plan"])
	}

	// "customer" should also be unchanged (not requested for expansion)
	if result["customer"] != "cus_1" {
		t.Errorf("expected customer = cus_1, got %v", result["customer"])
	}
}

func TestExpandRelationships_MissingRelatedItem(t *testing.T) {
	store := NewStateStore()
	_ = store.Register(&ResourceConfig{
		Name: "customers",
		SeedData: []map[string]interface{}{
			{"id": "cus_1", "name": "Alice"},
		},
	})

	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	resolver := func(tableName string) *StatefulResource {
		return store.Get(tableName)
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_nonexistent",
	}

	result := ExpandRelationships(item, []string{"customer"}, rels, resolver)

	// Should remain as string since the related item doesn't exist
	if result["customer"] != "cus_nonexistent" {
		t.Errorf("expected customer = cus_nonexistent, got %v", result["customer"])
	}
}

func TestExpandRelationships_NilExpandList(t *testing.T) {
	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_1",
	}

	result := ExpandRelationships(item, nil, rels, nil)

	if result["customer"] != "cus_1" {
		t.Errorf("expected customer = cus_1, got %v", result["customer"])
	}
}

func TestExpandRelationships_EmptyExpandList(t *testing.T) {
	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_1",
	}

	result := ExpandRelationships(item, []string{}, rels, nil)

	if result["customer"] != "cus_1" {
		t.Errorf("expected customer = cus_1, got %v", result["customer"])
	}
}

func TestExpandRelationships_EmptyRelationshipsMap(t *testing.T) {
	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_1",
	}

	result := ExpandRelationships(item, []string{"customer"}, nil, nil)

	if result["customer"] != "cus_1" {
		t.Errorf("expected customer = cus_1, got %v", result["customer"])
	}
}

func TestExpandRelationships_MissingTargetResource(t *testing.T) {
	store := NewStateStore()
	// Don't register the "customers" resource

	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	resolver := func(tableName string) *StatefulResource {
		return store.Get(tableName)
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": "cus_1",
	}

	result := ExpandRelationships(item, []string{"customer"}, rels, resolver)

	// Should stay as string since the target resource doesn't exist
	if result["customer"] != "cus_1" {
		t.Errorf("expected customer = cus_1, got %v", result["customer"])
	}
}

func TestExpandRelationships_NilFieldValue(t *testing.T) {
	store := NewStateStore()
	_ = store.Register(&ResourceConfig{
		Name: "customers",
		SeedData: []map[string]interface{}{
			{"id": "cus_1", "name": "Alice"},
		},
	})

	rels := map[string]*RelationshipInfo{
		"customer": {Table: "customers"},
	}

	resolver := func(tableName string) *StatefulResource {
		return store.Get(tableName)
	}

	item := map[string]interface{}{
		"id":       "sub_1",
		"customer": nil,
	}

	result := ExpandRelationships(item, []string{"customer"}, rels, resolver)

	// nil → "<nil>" string, should be skipped gracefully
	if result["customer"] != nil {
		t.Errorf("expected customer = nil, got %v", result["customer"])
	}
}

// =============================================================================
// ParseExpandParam Tests
// =============================================================================

func TestParseExpandParam_ArrayStyle(t *testing.T) {
	result := ParseExpandParam([]string{"customer", "invoice"})
	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result))
	}
	if result[0] != "customer" {
		t.Errorf("expected result[0] = customer, got %s", result[0])
	}
	if result[1] != "invoice" {
		t.Errorf("expected result[1] = invoice, got %s", result[1])
	}
}

func TestParseExpandParam_CommaSeparated(t *testing.T) {
	result := ParseExpandParam([]string{"customer,invoice"})
	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result))
	}
	if result[0] != "customer" {
		t.Errorf("expected result[0] = customer, got %s", result[0])
	}
	if result[1] != "invoice" {
		t.Errorf("expected result[1] = invoice, got %s", result[1])
	}
}

func TestParseExpandParam_Empty(t *testing.T) {
	result := ParseExpandParam(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}

	result = ParseExpandParam([]string{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}

func TestParseExpandParam_MixedWhitespace(t *testing.T) {
	result := ParseExpandParam([]string{" customer , invoice "})
	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result))
	}
	if result[0] != "customer" {
		t.Errorf("expected result[0] = customer, got %q", result[0])
	}
	if result[1] != "invoice" {
		t.Errorf("expected result[1] = invoice, got %q", result[1])
	}
}

func TestParseExpandParam_EmptyValues(t *testing.T) {
	result := ParseExpandParam([]string{""})
	if len(result) != 0 {
		t.Errorf("expected 0 fields for empty string, got %d", len(result))
	}
}

func TestParseExpandParam_Mixed(t *testing.T) {
	// Combines array style with comma-separated in one value
	result := ParseExpandParam([]string{"customer", "invoice,plan"})
	if len(result) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(result))
	}
	if result[0] != "customer" {
		t.Errorf("expected result[0] = customer, got %s", result[0])
	}
	if result[1] != "invoice" {
		t.Errorf("expected result[1] = invoice, got %s", result[1])
	}
	if result[2] != "plan" {
		t.Errorf("expected result[2] = plan, got %s", result[2])
	}
}
