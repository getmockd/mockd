// Package stateful provides the response transform gateway for stateful CRUD resources.
// TransformItem is the single funnel point — called by HTTP handlers, SOAP adapters,
// and future GraphQL/gRPC adapters. Protocol-agnostic: operates on map[string]interface{}.
package stateful

import (
	"fmt"
	"regexp"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// Timestamp format constants for ResponseTransform.Timestamps.Format.
const (
	TimestampFormatUnix    = "unix"
	TimestampFormatISO8601 = "iso8601"
	TimestampFormatRFC3339 = "rfc3339"
	TimestampFormatNone    = "none"
)

// Default field names and status codes.
const (
	DefaultDataField    = "data"
	DefaultDeleteStatus = 204
	DefaultCreateStatus = 201
	FieldCreatedAt      = "createdAt"
	FieldUpdatedAt      = "updatedAt"
)

// templateVarPattern matches {{item.fieldName}} placeholders in delete body templates.
var templateVarPattern = regexp.MustCompile(`\{\{item\.(\w+)\}\}`)

// TransformItem applies response transforms to a single item's JSON representation.
// If cfg is nil, returns input unchanged (fully backward compatible).
// Transform order: rename → hide → timestamps → inject (inject last so injected fields
// are always present and can't be accidentally hidden or renamed).
func TransformItem(data map[string]interface{}, cfg *config.ResponseTransform) map[string]interface{} {
	if cfg == nil || data == nil {
		return data
	}

	// 1. Rename fields
	if cfg.Fields != nil && len(cfg.Fields.Rename) > 0 {
		for oldKey, newKey := range cfg.Fields.Rename {
			if val, ok := data[oldKey]; ok {
				data[newKey] = val
				delete(data, oldKey)
			}
		}
	}

	// 2. Hide fields
	if cfg.Fields != nil && len(cfg.Fields.Hide) > 0 {
		for _, field := range cfg.Fields.Hide {
			delete(data, field)
		}
	}

	// 3. Timestamps — format conversion and/or key rename
	if cfg.Timestamps != nil {
		transformTimestamps(data, cfg.Timestamps)
	}

	// 4. Inject fields (last — always present, can't be hidden/renamed accidentally)
	if cfg.Fields != nil && len(cfg.Fields.Inject) > 0 {
		for key, val := range cfg.Fields.Inject {
			data[key] = val
		}
	}

	return data
}

// TransformList applies item transforms to each item, then reshapes the list envelope.
// If cfg is nil, returns the standard PaginatedResponse (backward compatible).
// Item transforms are protocol-agnostic. List envelope is HTTP-specific.
func TransformList(items []map[string]interface{}, meta PaginationMeta, cfg *config.ResponseTransform) interface{} {
	// Transform each item
	for i, item := range items {
		items[i] = TransformItem(item, cfg)
	}

	// If no list config, return standard PaginatedResponse shape
	if cfg == nil || cfg.List == nil {
		return &PaginatedResponse{
			Data: items,
			Meta: meta,
		}
	}

	listCfg := cfg.List

	// Build custom envelope
	result := make(map[string]interface{})

	// Extra fields at envelope level (e.g., object: "list", url: "/v1/customers")
	for key, val := range listCfg.ExtraFields {
		result[key] = val
	}

	dataField := listCfg.DataField
	if dataField == "" {
		dataField = DefaultDataField
	}
	result[dataField] = items

	// Meta fields (unless hidden)
	if !listCfg.HideMeta {
		metaMap := buildMetaMap(meta, listCfg.MetaFields)
		// Merge meta fields directly into envelope (not nested under "meta")
		for k, v := range metaMap {
			result[k] = v
		}
	}

	return result
}

// TransformDeleteResponse returns the HTTP status code and response body for a delete.
// If cfg is nil or has no delete override, returns (204, nil) — current behavior.
// The item parameter is the item being deleted (read before delete) — used for
// {{item.fieldName}} template substitution in the delete body.
func TransformDeleteResponse(item *ResourceItem, cfg *config.ResponseTransform) (int, interface{}) {
	if cfg == nil || cfg.Delete == nil {
		return DefaultDeleteStatus, nil
	}

	status := cfg.Delete.Status
	if status == 0 {
		status = DefaultDeleteStatus
	}

	if cfg.Delete.Body == nil {
		return status, nil
	}

	// Build response body with {{item.*}} template substitution
	body := make(map[string]interface{})
	var itemData map[string]interface{}
	if item != nil {
		itemData = item.ToJSON()
	}

	for key, val := range cfg.Delete.Body {
		body[key] = resolveTemplateValue(val, itemData)
	}

	return status, body
}

// TransformCreateStatus returns the HTTP status code for create operations.
// If cfg is nil or has no create override, returns 201 — current behavior.
func TransformCreateStatus(cfg *config.ResponseTransform) int {
	if cfg == nil || cfg.Create == nil || cfg.Create.Status == 0 {
		return DefaultCreateStatus
	}
	return cfg.Create.Status
}

// transformTimestamps handles timestamp format conversion and key renaming.
func transformTimestamps(data map[string]interface{}, cfg *config.TimestampTransform) {
	// Process both standard timestamp fields
	for _, tsField := range []string{FieldCreatedAt, FieldUpdatedAt} {
		val, ok := data[tsField]
		if !ok {
			continue
		}

		// Determine output key (rename or keep original)
		outputKey := tsField
		if newName, hasRename := cfg.Fields[tsField]; hasRename && newName != "" {
			outputKey = newName
		}

		// Format conversion
		switch cfg.Format {
		case TimestampFormatNone:
			delete(data, tsField)
			continue
		case TimestampFormatUnix:
			data[outputKey] = convertTimestamp(val, func(t time.Time) interface{} { return t.Unix() })
		case TimestampFormatISO8601:
			data[outputKey] = convertTimestamp(val, func(t time.Time) interface{} { return t.Format(time.RFC3339) })
		default: // rfc3339, empty, or unknown — keep as-is, apply rename
			if outputKey != tsField {
				data[outputKey] = val
			}
		}

		// Remove old key if renamed
		if outputKey != tsField {
			delete(data, tsField)
		}
	}
}

// convertTimestamp parses a timestamp value and applies a format function.
// Handles string (RFC3339Nano) and time.Time inputs; returns val unchanged on parse error.
func convertTimestamp(val interface{}, formatFn func(time.Time) interface{}) interface{} {
	switch v := val.(type) {
	case string:
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return val
		}
		return formatFn(t)
	case time.Time:
		return formatFn(v)
	default:
		return val
	}
}

// buildMetaMap creates the pagination meta map with optional field renaming.
func buildMetaMap(meta PaginationMeta, renames map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	// Default key names
	totalKey := "total"
	limitKey := "limit"
	offsetKey := "offset"
	countKey := "count"

	// Apply renames
	if len(renames) > 0 {
		if v, ok := renames["total"]; ok && v != "" {
			totalKey = v
		}
		if v, ok := renames["limit"]; ok && v != "" {
			limitKey = v
		}
		if v, ok := renames["offset"]; ok && v != "" {
			offsetKey = v
		}
		if v, ok := renames["count"]; ok && v != "" {
			countKey = v
		}
	}

	result[totalKey] = meta.Total
	result[limitKey] = meta.Limit
	result[offsetKey] = meta.Offset
	result[countKey] = meta.Count

	return result
}

// resolveTemplateValue replaces {{item.fieldName}} patterns in a value.
// Only processes string values; non-strings are returned as-is.
func resolveTemplateValue(val interface{}, itemData map[string]interface{}) interface{} {
	str, ok := val.(string)
	if !ok {
		return val
	}

	result := templateVarPattern.ReplaceAllStringFunc(str, func(match string) string {
		fieldName := templateVarPattern.FindStringSubmatch(match)[1]
		if itemData == nil {
			return match
		}
		if fieldVal, ok := itemData[fieldName]; ok {
			return fmt.Sprintf("%v", fieldVal)
		}
		return match
	})

	return result
}
