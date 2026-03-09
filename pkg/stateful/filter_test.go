package stateful

import (
	"testing"
	"time"
)

// ── ApplyFilters ─────────────────────────────────────────────────────────────

func TestApplyFilters_NoFilters(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"name": "Alice"}},
		{ID: "2", Data: map[string]interface{}{"name": "Bob"}},
	}
	filter := DefaultQueryFilter()
	result := ApplyFilters(items, filter)
	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
}

func TestApplyFilters_ExactMatch(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"status": "active", "name": "Alice"}},
		{ID: "2", Data: map[string]interface{}{"status": "inactive", "name": "Bob"}},
		{ID: "3", Data: map[string]interface{}{"status": "active", "name": "Charlie"}},
	}
	filter := DefaultQueryFilter()
	filter.Filters["status"] = "active"
	result := ApplyFilters(items, filter)
	if len(result) != 2 {
		t.Errorf("expected 2 active items, got %d", len(result))
	}
}

func TestApplyFilters_IDFilter(t *testing.T) {
	items := []*ResourceItem{
		{ID: "user-1", Data: map[string]interface{}{"name": "Alice"}},
		{ID: "user-2", Data: map[string]interface{}{"name": "Bob"}},
	}
	filter := DefaultQueryFilter()
	filter.Filters["id"] = "user-1"
	result := ApplyFilters(items, filter)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	if result[0].ID != "user-1" {
		t.Errorf("expected user-1, got %s", result[0].ID)
	}
}

func TestApplyFilters_ParentField(t *testing.T) {
	items := []*ResourceItem{
		{ID: "c1", Data: map[string]interface{}{"postId": "post-1", "body": "comment 1"}},
		{ID: "c2", Data: map[string]interface{}{"postId": "post-2", "body": "comment 2"}},
		{ID: "c3", Data: map[string]interface{}{"postId": "post-1", "body": "comment 3"}},
	}
	filter := DefaultQueryFilter()
	filter.ParentField = "postId"
	filter.ParentID = "post-1"
	result := ApplyFilters(items, filter)
	if len(result) != 2 {
		t.Errorf("expected 2 items with postId=post-1, got %d", len(result))
	}
}

func TestApplyFilters_ParentFieldMissing(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"name": "Alice"}}, // no postId field
	}
	filter := DefaultQueryFilter()
	filter.ParentField = "postId"
	filter.ParentID = "post-1"
	result := ApplyFilters(items, filter)
	if len(result) != 0 {
		t.Errorf("expected 0 items (parent field missing), got %d", len(result))
	}
}

func TestApplyFilters_MultipleFilters(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"status": "active", "role": "admin"}},
		{ID: "2", Data: map[string]interface{}{"status": "active", "role": "user"}},
		{ID: "3", Data: map[string]interface{}{"status": "inactive", "role": "admin"}},
	}
	filter := DefaultQueryFilter()
	filter.Filters["status"] = "active"
	filter.Filters["role"] = "admin"
	result := ApplyFilters(items, filter)
	if len(result) != 1 {
		t.Fatalf("expected 1 item matching both filters, got %d", len(result))
	}
	if result[0].ID != "1" {
		t.Errorf("expected item 1, got %s", result[0].ID)
	}
}

// ── SortItems ────────────────────────────────────────────────────────────────

func TestSortItems_ByCreatedAtDesc(t *testing.T) {
	now := time.Now()
	items := []*ResourceItem{
		{ID: "1", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "2", CreatedAt: now},
		{ID: "3", CreatedAt: now.Add(-1 * time.Hour)},
	}
	SortItems(items, "createdAt", "desc")
	if items[0].ID != "2" {
		t.Errorf("expected newest first (ID=2), got %s", items[0].ID)
	}
	if items[2].ID != "1" {
		t.Errorf("expected oldest last (ID=1), got %s", items[2].ID)
	}
}

func TestSortItems_ByNameAsc(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"name": "Charlie"}},
		{ID: "2", Data: map[string]interface{}{"name": "Alice"}},
		{ID: "3", Data: map[string]interface{}{"name": "Bob"}},
	}
	SortItems(items, "name", "asc")
	if items[0].Data["name"] != "Alice" {
		t.Errorf("expected Alice first, got %v", items[0].Data["name"])
	}
	if items[2].Data["name"] != "Charlie" {
		t.Errorf("expected Charlie last, got %v", items[2].Data["name"])
	}
}

func TestSortItems_DefaultField(t *testing.T) {
	now := time.Now()
	items := []*ResourceItem{
		{ID: "1", CreatedAt: now.Add(-1 * time.Hour)},
		{ID: "2", CreatedAt: now},
	}
	SortItems(items, "", "asc") // empty sort field defaults to createdAt
	if items[0].ID != "1" {
		t.Errorf("expected oldest first with asc order, got %s", items[0].ID)
	}
}

func TestSortItems_ByID(t *testing.T) {
	items := []*ResourceItem{
		{ID: "c"},
		{ID: "a"},
		{ID: "b"},
	}
	SortItems(items, "id", "asc")
	if items[0].ID != "a" {
		t.Errorf("expected a first, got %s", items[0].ID)
	}
}

// ── CompareValues ────────────────────────────────────────────────────────────

func TestCompareValues_Strings(t *testing.T) {
	if !CompareValues("alpha", "beta") {
		t.Error("expected alpha < beta")
	}
	if CompareValues("beta", "alpha") {
		t.Error("expected beta not < alpha")
	}
}

func TestCompareValues_Ints(t *testing.T) {
	if !CompareValues(1, 2) {
		t.Error("expected 1 < 2")
	}
}

func TestCompareValues_Float64(t *testing.T) {
	if !CompareValues(1.5, 2.5) {
		t.Error("expected 1.5 < 2.5")
	}
}

func TestCompareValues_Time(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !CompareValues(t1, t2) {
		t.Error("expected t1 before t2")
	}
}

func TestCompareValues_MixedTypes(t *testing.T) {
	// Fallback to string comparison for mismatched types
	result := CompareValues("10", 20)
	_ = result // just verify no panic
}

// ── Paginate ─────────────────────────────────────────────────────────────────

func TestPaginate_Basic(t *testing.T) {
	items := make([]*ResourceItem, 10)
	for i := range items {
		items[i] = &ResourceItem{ID: string(rune('a' + i))}
	}

	page, total := Paginate(items, 0, 3)
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
	if len(page) != 3 {
		t.Errorf("expected 3 items, got %d", len(page))
	}
}

func TestPaginate_OffsetBeyondTotal(t *testing.T) {
	items := make([]*ResourceItem, 5)
	for i := range items {
		items[i] = &ResourceItem{ID: string(rune('a' + i))}
	}

	page, total := Paginate(items, 100, 10)
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(page) != 0 {
		t.Errorf("expected 0 items (offset beyond total), got %d", len(page))
	}
}

func TestPaginate_NegativeOffset(t *testing.T) {
	items := make([]*ResourceItem, 5)
	for i := range items {
		items[i] = &ResourceItem{ID: string(rune('a' + i))}
	}

	page, _ := Paginate(items, -5, 3)
	if len(page) != 3 {
		t.Errorf("negative offset should be treated as 0, got %d items", len(page))
	}
}

func TestPaginate_ZeroLimit(t *testing.T) {
	items := make([]*ResourceItem, 5)
	for i := range items {
		items[i] = &ResourceItem{ID: string(rune('a' + i))}
	}

	page, _ := Paginate(items, 0, 0)
	if len(page) != 5 {
		t.Errorf("zero limit should use default (100), got %d items", len(page))
	}
}

func TestPaginate_LastPage(t *testing.T) {
	items := make([]*ResourceItem, 5)
	for i := range items {
		items[i] = &ResourceItem{ID: string(rune('a' + i))}
	}

	page, _ := Paginate(items, 3, 10)
	if len(page) != 2 {
		t.Errorf("expected last 2 items, got %d", len(page))
	}
}

// ── CursorPaginate ───────────────────────────────────────────────────────────

func TestCursorPaginate_NoParams(t *testing.T) {
	items := makeTestItems(5)
	page, total, hasMore := CursorPaginate(items, "", "", 3)
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(page) != 3 {
		t.Errorf("expected 3 items, got %d", len(page))
	}
	if !hasMore {
		t.Error("expected hasMore=true (3 of 5)")
	}
}

func TestCursorPaginate_NoParams_AllFit(t *testing.T) {
	items := makeTestItems(3)
	page, total, hasMore := CursorPaginate(items, "", "", 10)
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(page) != 3 {
		t.Errorf("expected 3 items, got %d", len(page))
	}
	if hasMore {
		t.Error("expected hasMore=false (all items fit)")
	}
}

func TestCursorPaginate_StartingAfter(t *testing.T) {
	items := makeTestItems(5) // IDs: item-0, item-1, item-2, item-3, item-4
	page, _, hasMore := CursorPaginate(items, "item-1", "", 2)
	if len(page) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page))
	}
	if page[0].ID != "item-2" {
		t.Errorf("expected first item to be item-2, got %s", page[0].ID)
	}
	if page[1].ID != "item-3" {
		t.Errorf("expected second item to be item-3, got %s", page[1].ID)
	}
	if !hasMore {
		t.Error("expected hasMore=true (item-4 remains)")
	}
}

func TestCursorPaginate_StartingAfter_LastPage(t *testing.T) {
	items := makeTestItems(5) // IDs: item-0, item-1, item-2, item-3, item-4
	page, _, hasMore := CursorPaginate(items, "item-3", "", 10)
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if page[0].ID != "item-4" {
		t.Errorf("expected item-4, got %s", page[0].ID)
	}
	if hasMore {
		t.Error("expected hasMore=false (last page)")
	}
}

func TestCursorPaginate_StartingAfter_LastItem(t *testing.T) {
	items := makeTestItems(3)
	page, _, hasMore := CursorPaginate(items, "item-2", "", 10)
	if len(page) != 0 {
		t.Errorf("expected 0 items after last, got %d", len(page))
	}
	if hasMore {
		t.Error("expected hasMore=false (nothing after last)")
	}
}

func TestCursorPaginate_StartingAfter_NotFound(t *testing.T) {
	items := makeTestItems(3)
	// If the cursor ID doesn't exist, start from the beginning
	page, _, hasMore := CursorPaginate(items, "nonexistent", "", 2)
	if len(page) != 2 {
		t.Errorf("expected 2 items (start from beginning), got %d", len(page))
	}
	if !hasMore {
		t.Error("expected hasMore=true")
	}
}

func TestCursorPaginate_EndingBefore(t *testing.T) {
	items := makeTestItems(5) // IDs: item-0, item-1, item-2, item-3, item-4
	page, _, hasMore := CursorPaginate(items, "", "item-3", 2)
	if len(page) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page))
	}
	if page[0].ID != "item-1" {
		t.Errorf("expected first item to be item-1, got %s", page[0].ID)
	}
	if page[1].ID != "item-2" {
		t.Errorf("expected second item to be item-2, got %s", page[1].ID)
	}
	// start = end(3) - limit(2) = 1, which is > 0, so hasMore=true (item-0 exists before)
	if !hasMore {
		t.Error("expected hasMore=true (item-0 exists before the page)")
	}
}

func TestCursorPaginate_EndingBefore_FirstItems(t *testing.T) {
	items := makeTestItems(5) // IDs: item-0, item-1, item-2, item-3, item-4
	page, _, hasMore := CursorPaginate(items, "", "item-2", 10)
	if len(page) != 2 {
		t.Fatalf("expected 2 items (item-0, item-1), got %d", len(page))
	}
	if page[0].ID != "item-0" {
		t.Errorf("expected item-0, got %s", page[0].ID)
	}
	if hasMore {
		t.Error("expected hasMore=false (start=0)")
	}
}

func TestCursorPaginate_DefaultLimit(t *testing.T) {
	items := makeTestItems(3)
	page, _, _ := CursorPaginate(items, "", "", 0)
	if len(page) != 3 {
		t.Errorf("zero limit should default to 100, got %d items", len(page))
	}
}

func TestCursorPaginate_EmptyItems(t *testing.T) {
	var items []*ResourceItem
	page, total, hasMore := CursorPaginate(items, "", "", 10)
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
	if len(page) != 0 {
		t.Errorf("expected 0 items, got %d", len(page))
	}
	if hasMore {
		t.Error("expected hasMore=false for empty list")
	}
}

// ── resolveNestedField ───────────────────────────────────────────────────────

func TestResolveNestedField(t *testing.T) {
	data := map[string]interface{}{
		"status": "active",
		"metadata": map[string]interface{}{
			"tier": "gold",
			"nested": map[string]interface{}{
				"deep": "value",
			},
		},
		"flat_number": 42,
		"tags":        []interface{}{"alpha", "beta", "gamma"},
	}

	tests := []struct {
		name   string
		key    string
		want   interface{}
		wantOK bool
	}{
		{"plain key", "status", "active", true},
		{"single bracket", "metadata[tier]", "gold", true},
		{"double bracket", "metadata[nested][deep]", "value", true},
		{"missing base key", "nonexistent[foo]", nil, false},
		{"missing nested key", "metadata[missing]", nil, false},
		{"non-map intermediate", "flat_number[sub]", nil, false},
		{"plain key fast path", "flat_number", 42, true},
		{"missing plain key", "nope", nil, false},
		{"slice index 0", "tags[0]", "alpha", true},
		{"slice index 1", "tags[1]", "beta", true},
		{"slice out of bounds", "tags[5]", nil, false},
		{"slice negative index", "tags[-1]", nil, false},
		{"slice non-numeric index", "tags[foo]", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveNestedField(data, tt.key)
			if ok != tt.wantOK {
				t.Errorf("resolveNestedField(%q) ok = %v, want %v", tt.key, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("resolveNestedField(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// ── ApplyFilters with nested fields ─────────────────────────────────────────

func TestApplyFilters_NestedFields(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{
			"name":     "Alice",
			"metadata": map[string]interface{}{"tier": "gold", "region": "us"},
		}},
		{ID: "2", Data: map[string]interface{}{
			"name":     "Bob",
			"metadata": map[string]interface{}{"tier": "silver", "region": "eu"},
		}},
		{ID: "3", Data: map[string]interface{}{
			"name":     "Charlie",
			"metadata": map[string]interface{}{"tier": "gold", "region": "eu"},
		}},
	}

	t.Run("filter by nested field", func(t *testing.T) {
		filter := DefaultQueryFilter()
		filter.Filters["metadata[tier]"] = "gold"
		result := ApplyFilters(items, filter)
		if len(result) != 2 {
			t.Fatalf("expected 2 gold-tier items, got %d", len(result))
		}
		if result[0].ID != "1" || result[1].ID != "3" {
			t.Errorf("expected IDs 1 and 3, got %s and %s", result[0].ID, result[1].ID)
		}
	})

	t.Run("filter by non-existent nested path", func(t *testing.T) {
		filter := DefaultQueryFilter()
		filter.Filters["metadata[nonexistent]"] = "anything"
		result := ApplyFilters(items, filter)
		if len(result) != 0 {
			t.Errorf("expected 0 items for non-existent path, got %d", len(result))
		}
	})

	t.Run("combined nested and flat filter", func(t *testing.T) {
		filter := DefaultQueryFilter()
		filter.Filters["metadata[tier]"] = "gold"
		filter.Filters["metadata[region]"] = "eu"
		result := ApplyFilters(items, filter)
		if len(result) != 1 {
			t.Fatalf("expected 1 item (gold+eu), got %d", len(result))
		}
		if result[0].ID != "3" {
			t.Errorf("expected ID 3, got %s", result[0].ID)
		}
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func makeTestItems(n int) []*ResourceItem {
	items := make([]*ResourceItem, n)
	for i := 0; i < n; i++ {
		items[i] = &ResourceItem{
			ID:   "item-" + string(rune('0'+i)),
			Data: map[string]interface{}{"index": i},
		}
	}
	return items
}
