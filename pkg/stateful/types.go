package stateful

import (
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// ResourceItem represents a single record within a stateful resource.
type ResourceItem struct {
	// ID is the unique identifier (UUID v4 if not provided)
	ID string `json:"id"`
	// Data contains user-defined fields (arbitrary JSON)
	Data map[string]interface{} `json:"-"`
	// CreatedAt is when the item was created
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the item was last modified
	UpdatedAt time.Time `json:"updatedAt"`
}

// QueryFilter contains parameters for filtering and paginating collection queries.
type QueryFilter struct {
	// Limit is the maximum items to return (default: 100)
	Limit int
	// Offset is the number of items to skip (default: 0)
	Offset int
	// Sort is the field name to sort by (default: "createdAt")
	Sort string
	// Order is the sort direction: "asc" or "desc" (default: "desc")
	Order string
	// Filters contains exact match filters by field name
	Filters map[string]string
	// ParentID is used for nested resource filtering
	ParentID string
	// ParentField is the field name for parent FK
	ParentField string
}

// PaginationMeta contains pagination metadata for collection responses.
type PaginationMeta struct {
	// Total is the total number of items matching filters (before pagination)
	Total int `json:"total"`
	// Limit is the maximum items per page
	Limit int `json:"limit"`
	// Offset is the number of items skipped
	Offset int `json:"offset"`
	// Count is the number of items in current page
	Count int `json:"count"`
}

// PaginatedResponse is the response envelope for collection queries.
type PaginatedResponse struct {
	// Data contains the page of items
	Data []map[string]interface{} `json:"data"`
	// Meta contains pagination metadata
	Meta PaginationMeta `json:"meta"`
}

// ResourceConfig is an alias for config.StatefulResourceConfig.
// This is the single canonical type for stateful resource configuration.
type ResourceConfig = config.StatefulResourceConfig

// StateOverview provides information about all registered stateful resources.
type StateOverview struct {
	// Resources is the number of registered stateful resources
	Resources int `json:"resources"`
	// TotalItems is the total items across all resources
	TotalItems int `json:"totalItems"`
	// ResourceList contains names of registered resources
	ResourceList []string `json:"resourceList"`
}

// ResourceInfo provides details about a specific stateful resource.
type ResourceInfo struct {
	// Name is the resource name
	Name string `json:"name"`
	// BasePath is the URL path prefix
	BasePath string `json:"basePath"`
	// ItemCount is the current number of items
	ItemCount int `json:"itemCount"`
	// SeedCount is the number of seed data items
	SeedCount int `json:"seedCount"`
	// IDField is the field used for item ID
	IDField string `json:"idField"`
	// ParentField is the field used for parent FK (if nested)
	ParentField string `json:"parentField,omitempty"`
}

// ResetResponse is returned after a state reset operation.
type ResetResponse struct {
	// Reset indicates success
	Reset bool `json:"reset"`
	// Resources lists the resources that were reset
	Resources []string `json:"resources"`
	// Message is a human-readable status message
	Message string `json:"message"`
}

// ErrorResponse represents an error returned from stateful operations.
type ErrorResponse struct {
	// Error is the error message
	Error string `json:"error"`
	// Resource is the resource name (if applicable)
	Resource string `json:"resource,omitempty"`
	// ID is the item ID (if applicable)
	ID string `json:"id,omitempty"`
	// Detail provides additional error context
	Detail string `json:"detail,omitempty"`
	// StatusCode is the HTTP status code
	StatusCode int `json:"statusCode,omitempty"`
	// Hint provides a user-friendly suggestion for resolving the error
	Hint string `json:"hint,omitempty"`
	// Field is the specific field that caused a validation error
	Field string `json:"field,omitempty"`
}

// DefaultQueryFilter returns a QueryFilter with sensible defaults.
func DefaultQueryFilter() *QueryFilter {
	return &QueryFilter{
		Limit:   100,
		Offset:  0,
		Sort:    "createdAt",
		Order:   "desc",
		Filters: make(map[string]string),
	}
}

// ToJSON converts a ResourceItem to a flattened JSON-compatible map.
// User-defined fields from Data are merged at the root level with id and timestamps.
func (item *ResourceItem) ToJSON() map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all data fields to root level
	for k, v := range item.Data {
		result[k] = v
	}

	// Set/override system fields
	result["id"] = item.ID
	result["createdAt"] = item.CreatedAt.Format(time.RFC3339)
	result["updatedAt"] = item.UpdatedAt.Format(time.RFC3339)

	return result
}

// FromJSON creates a ResourceItem from a JSON map, extracting system fields.
func FromJSON(data map[string]interface{}, idField string) *ResourceItem {
	if idField == "" {
		idField = "id"
	}

	item := &ResourceItem{
		Data: make(map[string]interface{}),
	}

	// Extract ID if present
	if id, ok := data[idField]; ok {
		if idStr, ok := id.(string); ok {
			item.ID = idStr
		}
	}

	// Copy all non-system fields to Data
	for k, v := range data {
		if k == "createdAt" || k == "updatedAt" {
			continue
		}
		if k == idField {
			continue
		}
		item.Data[k] = v
	}

	return item
}
