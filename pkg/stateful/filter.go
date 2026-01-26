package stateful

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ApplyFilters filters items based on the query filter.
// Supports parent filtering for nested resources and exact match filtering on any field.
func ApplyFilters(items []*ResourceItem, filter *QueryFilter) []*ResourceItem {
	result := make([]*ResourceItem, 0, len(items))

	for _, item := range items {
		// Check parent field filter (for nested resources like /users/:userId/orders)
		if filter.ParentField != "" && filter.ParentID != "" {
			if parentVal, ok := item.Data[filter.ParentField]; ok {
				if fmt.Sprintf("%v", parentVal) != filter.ParentID {
					continue
				}
			} else {
				continue
			}
		}

		// Check exact match filters from query params
		matched := true
		for field, value := range filter.Filters {
			var itemValue interface{}

			// Check system fields first
			switch field {
			case "id":
				itemValue = item.ID
			default:
				itemValue = item.Data[field]
			}

			if fmt.Sprintf("%v", itemValue) != value {
				matched = false
				break
			}
		}

		if matched {
			result = append(result, item)
		}
	}

	return result
}

// SortItems sorts items by the specified field and order.
// Supported fields: id, createdAt, updatedAt, or any user-defined field in Data.
// Order: "asc" for ascending, "desc" for descending (default: createdAt desc).
func SortItems(items []*ResourceItem, sortField, order string) {
	if sortField == "" {
		sortField = "createdAt"
	}

	sort.Slice(items, func(i, j int) bool {
		var vi, vj interface{}

		switch sortField {
		case "id":
			vi, vj = items[i].ID, items[j].ID
		case "createdAt":
			vi, vj = items[i].CreatedAt, items[j].CreatedAt
		case "updatedAt":
			vi, vj = items[i].UpdatedAt, items[j].UpdatedAt
		default:
			vi, vj = items[i].Data[sortField], items[j].Data[sortField]
		}

		less := CompareValues(vi, vj)

		if strings.ToLower(order) == "desc" {
			return !less
		}
		return less
	})
}

// CompareValues compares two interface values for sorting.
// Handles string, int, int64, float64, and time.Time types.
// Falls back to string comparison for unknown types.
func CompareValues(a, b interface{}) bool {
	switch va := a.(type) {
	case string:
		if vb, ok := b.(string); ok {
			return va < vb
		}
	case int:
		if vb, ok := b.(int); ok {
			return va < vb
		}
	case int64:
		if vb, ok := b.(int64); ok {
			return va < vb
		}
	case float64:
		if vb, ok := b.(float64); ok {
			return va < vb
		}
	case time.Time:
		if vb, ok := b.(time.Time); ok {
			return va.Before(vb)
		}
	}

	// Fallback to string comparison
	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
}

// Paginate applies offset and limit to a slice of items.
// Returns the paginated slice and the total count before pagination.
// Handles edge cases: negative offset is treated as 0, zero/negative limit uses default (100).
func Paginate(items []*ResourceItem, offset, limit int) ([]*ResourceItem, int) {
	total := len(items)

	// Handle negative offset - treat as 0
	start := offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	// Handle zero/negative limit - use default
	if limit <= 0 {
		limit = 100 // Default limit
	}

	end := start + limit
	if end > total {
		end = total
	}

	return items[start:end], total
}
