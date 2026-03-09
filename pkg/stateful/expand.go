package stateful

import (
	"fmt"
	"strings"
)

// ExpandRelationships inlines related objects for the requested expand fields.
// For each requested field, if the table has a relationship defined for that field,
// the field's string value is used as an ID to look up the related item from the
// target table. The string ID is replaced with the full object map.
//
// If the related item is not found, the field is left as-is (graceful degradation).
// The resolver function looks up a resource by table name from the store.
func ExpandRelationships(
	item map[string]interface{},
	expandFields []string,
	relationships map[string]*RelationshipInfo,
	resolver func(tableName string) *StatefulResource,
) map[string]interface{} {
	if len(expandFields) == 0 || len(relationships) == 0 {
		return item
	}

	for _, field := range expandFields {
		rel, ok := relationships[field]
		if !ok {
			continue // no relationship defined for this field
		}

		// Get the current value (should be a string ID)
		idVal, ok := item[field]
		if !ok {
			continue
		}

		idStr := fmt.Sprintf("%v", idVal)
		if idStr == "" || idStr == "<nil>" {
			continue
		}

		// Look up the related resource
		resource := resolver(rel.Table)
		if resource == nil {
			continue
		}

		// Fetch the related item
		related := resource.Get(idStr)
		if related == nil {
			continue // item not found — leave as string ID
		}

		// Build the expanded object: start with the Data map, add the ID
		expanded := make(map[string]interface{}, len(related.Data)+1)
		for k, v := range related.Data {
			expanded[k] = v
		}
		// Ensure the ID field is present
		idField := resource.IDField()
		if idField == "" {
			idField = "id"
		}
		if _, hasID := expanded[idField]; !hasID {
			expanded[idField] = related.ID
		}

		// Replace the string ID with the full object
		item[field] = expanded
	}

	return item
}

// ParseExpandParam extracts expand field names from the query string.
// Supports both ?expand[]=customer&expand[]=invoice (array style)
// and ?expand=customer,invoice (comma-separated style).
func ParseExpandParam(queryValues []string) []string {
	if len(queryValues) == 0 {
		return nil
	}

	var fields []string
	for _, v := range queryValues {
		// Handle comma-separated values
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				fields = append(fields, part)
			}
		}
	}
	return fields
}
