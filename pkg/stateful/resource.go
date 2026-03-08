package stateful

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/validation"
)

// ID strategy constants for StatefulResourceConfig.IDStrategy.
const (
	IDStrategyUUID     = "uuid"
	IDStrategyPrefix   = "prefix"
	IDStrategyULID     = "ulid"
	IDStrategySequence = "sequence"
	IDStrategyShort    = "short"
)

// DefaultIDField is the default field name used for item IDs.
const DefaultIDField = "id"

// StatefulResource represents a named collection that maintains state.
type StatefulResource struct {
	mu               sync.RWMutex
	name             string
	basePath         string
	idField          string
	idStrategy       string // uuid (default), prefix, ulid, sequence, short
	idPrefix         string // for "prefix" strategy (e.g., "cus_")
	sequenceCounter  int    // for "sequence" strategy
	parentField      string
	maxItems         int
	items            map[string]*ResourceItem
	seedData         []map[string]interface{}
	pathRegex        *regexp.Regexp
	pathParams       []string
	validator        *validation.StatefulValidator
	validationConfig *validation.StatefulValidation
	responseCfg      *config.ResponseTransform
}

// NewStatefulResource creates a new StatefulResource from config.
func NewStatefulResource(config *ResourceConfig) *StatefulResource {
	return NewStatefulResourceWithLogger(config, nil)
}

// NewStatefulResourceWithLogger creates a new StatefulResource with a custom logger.
func NewStatefulResourceWithLogger(config *ResourceConfig, logger *slog.Logger) *StatefulResource {
	idField := config.IDField
	if idField == "" {
		idField = DefaultIDField
	}

	r := &StatefulResource{
		name:             config.Name,
		basePath:         config.BasePath,
		idField:          idField,
		idStrategy:       config.IDStrategy,
		idPrefix:         config.IDPrefix,
		parentField:      config.ParentField,
		maxItems:         config.MaxItems,
		items:            make(map[string]*ResourceItem),
		seedData:         config.SeedData,
		validationConfig: config.Validation,
		responseCfg:      config.Response,
	}

	// Build path regex for HTTP matching (skip if no basePath — bridge-only resource)
	if r.basePath != "" {
		r.buildPathMatcher()
	}

	// Initialize validation
	r.initValidation(logger)

	return r
}

// initValidation sets up the validator based on configuration
func (r *StatefulResource) initValidation(logger *slog.Logger) {
	if r.validationConfig == nil {
		return
	}

	// Auto-infer validation from seed data if enabled
	if r.validationConfig.Auto && len(r.seedData) > 0 {
		inferred := validation.InferValidation(r.seedData, r.basePath, logger)
		if inferred != nil {
			// Merge inferred with explicit config (explicit takes precedence)
			r.validationConfig = mergeStatefulValidation(inferred, r.validationConfig)
		}
	}

	// Create the validator
	r.validator = validation.NewStatefulValidator(r.validationConfig)
}

// mergeStatefulValidation merges two StatefulValidation configs
func mergeStatefulValidation(base, override *validation.StatefulValidation) *validation.StatefulValidation {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	result := &validation.StatefulValidation{
		Auto:       override.Auto,
		Required:   append([]string{}, base.Required...),
		Fields:     make(map[string]*validation.FieldValidator),
		PathParams: make(map[string]*validation.FieldValidator),
		Mode:       base.Mode,
	}

	// Copy base fields
	for k, v := range base.Fields {
		result.Fields[k] = v
	}
	for k, v := range base.PathParams {
		result.PathParams[k] = v
	}

	// Override with explicit config
	result.Required = append(result.Required, override.Required...)
	for k, v := range override.Fields {
		result.Fields[k] = v
	}
	for k, v := range override.PathParams {
		result.PathParams[k] = v
	}
	if override.OnCreate != nil {
		result.OnCreate = override.OnCreate
	}
	if override.OnUpdate != nil {
		result.OnUpdate = override.OnUpdate
	}
	if override.Schema != nil {
		result.Schema = override.Schema
	}
	if override.SchemaRef != "" {
		result.SchemaRef = override.SchemaRef
	}
	if override.Mode != "" {
		result.Mode = override.Mode
	}

	return result
}

// generateID creates a new ID based on the resource's ID strategy.
// Must be called while holding the resource mutex (or from a locked context).
func (r *StatefulResource) generateID() string {
	switch r.idStrategy {
	case IDStrategyPrefix:
		return r.idPrefix + id.Short()
	case IDStrategyULID:
		return id.ULID()
	case IDStrategySequence:
		r.sequenceCounter++
		return strconv.Itoa(r.sequenceCounter)
	case IDStrategyShort:
		return id.Short()
	default:
		return id.UUID()
	}
}

// ResponseConfig returns the response transform configuration, if any.
func (r *StatefulResource) ResponseConfig() *config.ResponseTransform {
	return r.responseCfg
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
// Returns false for bridge-only resources (no basePath / no HTTP routing).
func (r *StatefulResource) MatchPath(path string) (string, map[string]string, bool) {
	if r.pathRegex == nil {
		return "", nil, false
	}
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

// loadSeed populates the resource with seed data on first initialization.
// Unlike Reset, this also persists generated IDs back into seedData for deterministic resets,
// and returns an error on duplicate IDs.
func (r *StatefulResource) loadSeed() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items = make(map[string]*ResourceItem)

	for i, data := range r.seedData {
		item := FromJSON(data, r.idField)

		if item.ID == "" {
			item.ID = r.generateID()
			// Persist generated ID so Reset() reuses it (deterministic across resets)
			r.seedData[i][r.idField] = item.ID
		} else {
			r.trackSequenceID(item.ID)
		}

		if _, exists := r.items[item.ID]; exists {
			return fmt.Errorf("duplicate ID %q in seed data at index %d", item.ID, i)
		}

		r.stampAndStore(item)
	}

	return nil
}

// Create adds a new item to the resource.
func (r *StatefulResource) Create(data map[string]interface{}, pathParams map[string]string) (*ResourceItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Enforce memory limit
	if r.maxItems > 0 && len(r.items) >= r.maxItems {
		return nil, &CapacityError{Resource: r.name, MaxItems: r.maxItems}
	}

	item := FromJSON(data, r.idField)

	// Generate ID if not provided
	if item.ID == "" {
		item.ID = r.generateID()
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

// Patch partially updates an existing item by merging the provided fields
// into the existing data. Fields not present in the patch are preserved.
func (r *StatefulResource) Patch(id string, data map[string]interface{}) (*ResourceItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.items[id]
	if !ok {
		return nil, &NotFoundError{Resource: r.name, ID: id}
	}

	// Merge patch data into existing data
	merged := make(map[string]interface{})
	for k, v := range existing.Data {
		merged[k] = v
	}
	for k, v := range data {
		// Skip system fields
		if k == "createdAt" || k == "updatedAt" || k == r.idField {
			continue
		}
		merged[k] = v
	}

	// Build updated item preserving system fields
	item := &ResourceItem{
		ID:        id,
		Data:      merged,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: time.Now(),
	}

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
	if r.idStrategy == IDStrategySequence {
		r.sequenceCounter = 0
	}

	for _, data := range r.seedData {
		item := FromJSON(data, r.idField)
		if item.ID == "" {
			item.ID = r.generateID()
		} else {
			r.trackSequenceID(item.ID)
		}
		r.stampAndStore(item)
	}
}

// trackSequenceID updates the sequence counter if this ID is a higher numeric value.
// No-op for non-sequence strategies.
func (r *StatefulResource) trackSequenceID(itemID string) {
	if r.idStrategy == IDStrategySequence {
		if n, err := strconv.Atoi(itemID); err == nil && n > r.sequenceCounter {
			r.sequenceCounter = n
		}
	}
}

// stampAndStore sets timestamps and stores the item. Must be called under lock.
func (r *StatefulResource) stampAndStore(item *ResourceItem) {
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now
	r.items[item.ID] = item
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
		MaxItems:    r.maxItems,
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

// HasValidation returns true if validation is configured for this resource.
func (r *StatefulResource) HasValidation() bool {
	return r.validator != nil
}

// ValidateCreate validates data for a create operation.
// Returns nil if validation passes or is not configured.
func (r *StatefulResource) ValidateCreate(ctx context.Context, data map[string]interface{}, pathParams map[string]string) *validation.Result {
	if r.validator == nil {
		return &validation.Result{Valid: true}
	}
	return r.validator.ValidateCreate(ctx, data, pathParams)
}

// ValidateUpdate validates data for an update operation.
// Returns nil if validation passes or is not configured.
func (r *StatefulResource) ValidateUpdate(ctx context.Context, data map[string]interface{}, pathParams map[string]string) *validation.Result {
	if r.validator == nil {
		return &validation.Result{Valid: true}
	}
	return r.validator.ValidateUpdate(ctx, data, pathParams)
}

// ValidatePathParams validates only the path parameters.
// Returns nil if validation passes or is not configured.
func (r *StatefulResource) ValidatePathParams(ctx context.Context, pathParams map[string]string) *validation.Result {
	if r.validator == nil {
		return &validation.Result{Valid: true}
	}
	return r.validator.ValidatePathParams(ctx, pathParams)
}

// GetValidationMode returns the validation mode (strict, warn, permissive).
func (r *StatefulResource) GetValidationMode() string {
	if r.validator == nil {
		return validation.ModeStrict
	}
	return r.validator.GetMode()
}

// Config reconstructs the ResourceConfig from the resource's current settings.
// This is used by Export to serialize the resource definition back to config format.
// Note: seed data reflects the original config, not current runtime state.
func (r *StatefulResource) Config() *ResourceConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := &ResourceConfig{
		Name:     r.name,
		BasePath: r.basePath,
		MaxItems: r.maxItems,
	}
	if r.idField != DefaultIDField {
		cfg.IDField = r.idField
	}
	if r.idStrategy != "" && r.idStrategy != IDStrategyUUID {
		cfg.IDStrategy = r.idStrategy
	}
	if r.idPrefix != "" {
		cfg.IDPrefix = r.idPrefix
	}
	if r.parentField != "" {
		cfg.ParentField = r.parentField
	}
	if len(r.seedData) > 0 {
		cfg.SeedData = r.seedData
	}
	if r.validationConfig != nil {
		cfg.Validation = r.validationConfig
	}
	if r.responseCfg != nil {
		cfg.Response = r.responseCfg
	}
	return cfg
}
