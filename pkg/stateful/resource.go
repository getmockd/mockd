package stateful

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/id"
)

// StatefulResource represents a named collection that maintains state.
type StatefulResource struct {
	mu          sync.RWMutex
	name        string
	basePath    string
	idField     string
	parentField string
	items       map[string]*ResourceItem
	seedData    []map[string]interface{}
	pathRegex   *regexp.Regexp
	pathParams  []string
}

// NewStatefulResource creates a new StatefulResource from config.
func NewStatefulResource(config *ResourceConfig) *StatefulResource {
	idField := config.IDField
	if idField == "" {
		idField = "id"
	}

	r := &StatefulResource{
		name:        config.Name,
		basePath:    config.BasePath,
		idField:     idField,
		parentField: config.ParentField,
		items:       make(map[string]*ResourceItem),
		seedData:    config.SeedData,
	}

	// Build path regex for matching
	r.buildPathMatcher()

	return r
}

// buildPathMatcher creates a regex for matching incoming request paths.
func (r *StatefulResource) buildPathMatcher() {
	// Convert path params like :userId to regex groups
	paramPattern := regexp.MustCompile(`:(\w+)`)
	matches := paramPattern.FindAllStringSubmatch(r.basePath, -1)

	r.pathParams = make([]string, 0)
	for _, match := range matches {
		r.pathParams = append(r.pathParams, match[1])
	}

	// Build regex pattern
	pattern := "^" + paramPattern.ReplaceAllString(regexp.QuoteMeta(r.basePath), `([^/]+)`)

	// Allow optional trailing ID segment
	pattern += "(?:/([^/]+))?$"

	r.pathRegex = regexp.MustCompile(pattern)
}

// MatchPath checks if the given path matches this resource.
// Returns: itemID (if present), path params, and whether it matched.
func (r *StatefulResource) MatchPath(path string) (string, map[string]string, bool) {
	matches := r.pathRegex.FindStringSubmatch(path)
	if matches == nil {
		return "", nil, false
	}

	params := make(map[string]string)

	// Extract path parameters (e.g., :userId)
	for i, paramName := range r.pathParams {
		if i+1 < len(matches) {
			params[paramName] = matches[i+1]
		}
	}

	// The last capture group is the optional item ID
	itemID := ""
	lastIdx := len(r.pathParams) + 1
	if lastIdx < len(matches) && matches[lastIdx] != "" {
		itemID = matches[lastIdx]
	}

	return itemID, params, true
}

// loadSeed populates the resource with seed data.
func (r *StatefulResource) loadSeed() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items = make(map[string]*ResourceItem)

	for i, data := range r.seedData {
		item := FromJSON(data, r.idField)

		// Generate ID if not provided
		if item.ID == "" {
			item.ID = id.UUID()
		}

		// Check for duplicate ID
		if _, exists := r.items[item.ID]; exists {
			return fmt.Errorf("duplicate ID %q in seed data at index %d", item.ID, i)
		}

		// Set timestamps
		now := time.Now()
		item.CreatedAt = now
		item.UpdatedAt = now

		r.items[item.ID] = item
	}

	return nil
}

// Create adds a new item to the resource.
func (r *StatefulResource) Create(data map[string]interface{}, pathParams map[string]string) (*ResourceItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item := FromJSON(data, r.idField)

	// Generate ID if not provided
	if item.ID == "" {
		item.ID = id.UUID()
	}

	// Check for duplicate ID
	if _, exists := r.items[item.ID]; exists {
		return nil, &ConflictError{Resource: r.name, ID: item.ID}
	}

	// Set parent field from path params if nested resource
	if r.parentField != "" && pathParams != nil {
		if parentID, ok := pathParams[r.parentField]; ok {
			item.Data[r.parentField] = parentID
		}
	}

	// Set timestamps
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	r.items[item.ID] = item
	return item, nil
}

// Get retrieves a single item by ID.
func (r *StatefulResource) Get(id string) *ResourceItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.items[id]
}

// List returns items matching the filter.
func (r *StatefulResource) List(filter *QueryFilter) *PaginatedResponse {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if filter == nil {
		filter = DefaultQueryFilter()
	}

	// Collect all items into a slice
	allItems := make([]*ResourceItem, 0, len(r.items))
	for _, item := range r.items {
		allItems = append(allItems, item)
	}

	// Apply filters using exported function
	filtered := ApplyFilters(allItems, filter)

	// Sort using exported function
	SortItems(filtered, filter.Sort, filter.Order)

	// Apply pagination using exported function
	page, total := Paginate(filtered, filter.Offset, filter.Limit)

	// Convert to JSON format
	data := make([]map[string]interface{}, len(page))
	for i, item := range page {
		data[i] = item.ToJSON()
	}

	return &PaginatedResponse{
		Data: data,
		Meta: PaginationMeta{
			Total:  total,
			Limit:  filter.Limit,
			Offset: filter.Offset,
			Count:  len(data),
		},
	}
}

// Update modifies an existing item.
func (r *StatefulResource) Update(id string, data map[string]interface{}) (*ResourceItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.items[id]
	if !ok {
		return nil, &NotFoundError{Resource: r.name, ID: id}
	}

	// Create updated item preserving system fields
	item := FromJSON(data, r.idField)
	item.ID = id
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now()

	r.items[id] = item
	return item, nil
}

// Delete removes an item by ID.
func (r *StatefulResource) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return &NotFoundError{Resource: r.name, ID: id}
	}

	delete(r.items, id)
	return nil
}

// Reset restores the resource to its seed data state.
func (r *StatefulResource) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items = make(map[string]*ResourceItem)

	for _, data := range r.seedData {
		item := FromJSON(data, r.idField)

		if item.ID == "" {
			item.ID = id.UUID()
		}

		now := time.Now()
		item.CreatedAt = now
		item.UpdatedAt = now

		r.items[item.ID] = item
	}
}

// Clear removes all items but keeps the resource registered (does not restore seed data).
func (r *StatefulResource) Clear() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := len(r.items)
	r.items = make(map[string]*ResourceItem)
	return count
}

// Count returns the number of items in the resource.
func (r *StatefulResource) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// Info returns information about this resource.
func (r *StatefulResource) Info() *ResourceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &ResourceInfo{
		Name:        r.name,
		BasePath:    r.basePath,
		ItemCount:   len(r.items),
		SeedCount:   len(r.seedData),
		IDField:     r.idField,
		ParentField: r.parentField,
	}
}

// Name returns the resource name.
func (r *StatefulResource) Name() string {
	return r.name
}

// BasePath returns the resource base path.
func (r *StatefulResource) BasePath() string {
	return r.basePath
}

// ParentField returns the parent field name (for nested resources).
func (r *StatefulResource) ParentField() string {
	return r.parentField
}
