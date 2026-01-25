// Package validation provides request validation for mockd.
// It supports field-level validation, JSON Schema validation, and auto-inference
// from seed data for stateful resources.

package validation

// FieldValidator defines validation rules for a single field.
// It supports type checking, string/number constraints, array validation,
// enum values, and nested object validation.
type FieldValidator struct {
	// Type specifies the expected JSON type: string, number, integer, boolean, array, object
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	// Required indicates the field must be present (used in Fields map context)
	Required bool `json:"required,omitempty" yaml:"required,omitempty"`

	// Nullable allows null values even when type is specified
	Nullable bool `json:"nullable,omitempty" yaml:"nullable,omitempty"`

	// String validations
	MinLength *int   `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength *int   `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern   string `json:"pattern,omitempty" yaml:"pattern,omitempty"` // Regex pattern
	Format    string `json:"format,omitempty" yaml:"format,omitempty"`   // email, uuid, date, datetime, uri, ipv4, ipv6, hostname

	// Number validations (applies to number and integer types)
	Min          *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max          *float64 `json:"max,omitempty" yaml:"max,omitempty"`
	ExclusiveMin *float64 `json:"exclusiveMin,omitempty" yaml:"exclusiveMin,omitempty"`
	ExclusiveMax *float64 `json:"exclusiveMax,omitempty" yaml:"exclusiveMax,omitempty"`

	// Array validations
	MinItems    *int            `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems    *int            `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	UniqueItems bool            `json:"uniqueItems,omitempty" yaml:"uniqueItems,omitempty"`
	Items       *FieldValidator `json:"items,omitempty" yaml:"items,omitempty"` // Validation for array items

	// Enum validation - value must be one of these
	Enum []interface{} `json:"enum,omitempty" yaml:"enum,omitempty"`

	// Nested object validation
	Properties map[string]*FieldValidator `json:"properties,omitempty" yaml:"properties,omitempty"`

	// Custom error message (overrides default)
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// RequestValidation defines validation rules for HTTP requests.
// It can validate body fields, path parameters, query parameters, and headers.
type RequestValidation struct {
	// Required lists field names that must be present in the request body
	Required []string `json:"required,omitempty" yaml:"required,omitempty"`

	// Fields defines per-field validation rules for request body
	Fields map[string]*FieldValidator `json:"fields,omitempty" yaml:"fields,omitempty"`

	// Schema is an inline JSON Schema for full schema validation
	Schema interface{} `json:"schema,omitempty" yaml:"schema,omitempty"`

	// SchemaRef is a file path to an external JSON Schema
	SchemaRef string `json:"schemaRef,omitempty" yaml:"schemaRef,omitempty"`

	// PathParams defines validation rules for URL path parameters
	PathParams map[string]*FieldValidator `json:"pathParams,omitempty" yaml:"pathParams,omitempty"`

	// QueryParams defines validation rules for URL query parameters
	QueryParams map[string]*FieldValidator `json:"queryParams,omitempty" yaml:"queryParams,omitempty"`

	// Headers defines validation rules for HTTP headers
	Headers map[string]*FieldValidator `json:"headers,omitempty" yaml:"headers,omitempty"`

	// Mode controls validation behavior: strict (default), warn, permissive
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`

	// FailStatus is the HTTP status code for validation failures (default 400)
	FailStatus int `json:"failStatus,omitempty" yaml:"failStatus,omitempty"`
}

// StatefulValidation defines validation rules for stateful CRUD resources.
type StatefulValidation struct {
	// Auto enables automatic inference of validation rules from seed data
	Auto bool `json:"auto,omitempty" yaml:"auto,omitempty"`

	// OnCreate defines validation rules for POST (create) operations
	OnCreate *RequestValidation `json:"onCreate,omitempty" yaml:"onCreate,omitempty"`

	// OnUpdate defines validation rules for PUT/PATCH (update) operations
	OnUpdate *RequestValidation `json:"onUpdate,omitempty" yaml:"onUpdate,omitempty"`

	// Required lists field names required for both create and update (shared)
	Required []string `json:"required,omitempty" yaml:"required,omitempty"`

	// Fields defines per-field validation rules (shared for create and update)
	Fields map[string]*FieldValidator `json:"fields,omitempty" yaml:"fields,omitempty"`

	// PathParams defines validation rules for URL path parameters (e.g., :postId)
	PathParams map[string]*FieldValidator `json:"pathParams,omitempty" yaml:"pathParams,omitempty"`

	// Schema is an inline JSON Schema for full schema validation
	Schema interface{} `json:"schema,omitempty" yaml:"schema,omitempty"`

	// SchemaRef is a file path to an external JSON Schema
	SchemaRef string `json:"schemaRef,omitempty" yaml:"schemaRef,omitempty"`

	// Mode controls validation behavior: strict (default), warn, permissive
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

// ValidationMode constants
const (
	ModeStrict     = "strict"     // Return error on any validation failure
	ModeWarn       = "warn"       // Log warnings but allow request through
	ModePermissive = "permissive" // Only fail on critical errors (missing required fields)
)

// DefaultFailStatus is the default HTTP status code for validation failures
const DefaultFailStatus = 400

// GetMode returns the validation mode, defaulting to strict
func (v *RequestValidation) GetMode() string {
	if v == nil || v.Mode == "" {
		return ModeStrict
	}
	return v.Mode
}

// GetFailStatus returns the HTTP status code for failures, defaulting to 400
func (v *RequestValidation) GetFailStatus() int {
	if v == nil || v.FailStatus == 0 {
		return DefaultFailStatus
	}
	return v.FailStatus
}

// GetMode returns the validation mode, defaulting to strict
func (v *StatefulValidation) GetMode() string {
	if v == nil || v.Mode == "" {
		return ModeStrict
	}
	return v.Mode
}

// IsEmpty returns true if no validation rules are configured
func (v *StatefulValidation) IsEmpty() bool {
	if v == nil {
		return true
	}
	return !v.Auto &&
		v.OnCreate == nil &&
		v.OnUpdate == nil &&
		len(v.Required) == 0 &&
		len(v.Fields) == 0 &&
		len(v.PathParams) == 0 &&
		v.Schema == nil &&
		v.SchemaRef == ""
}

// IsEmpty returns true if no validation rules are configured
func (v *RequestValidation) IsEmpty() bool {
	if v == nil {
		return true
	}
	return len(v.Required) == 0 &&
		len(v.Fields) == 0 &&
		v.Schema == nil &&
		v.SchemaRef == "" &&
		len(v.PathParams) == 0 &&
		len(v.QueryParams) == 0 &&
		len(v.Headers) == 0
}

// Merge combines two RequestValidation configs, with other taking precedence
func (v *RequestValidation) Merge(other *RequestValidation) *RequestValidation {
	if v == nil {
		return other
	}
	if other == nil {
		return v
	}

	result := &RequestValidation{
		Required:    append([]string{}, v.Required...),
		Fields:      make(map[string]*FieldValidator),
		PathParams:  make(map[string]*FieldValidator),
		QueryParams: make(map[string]*FieldValidator),
		Headers:     make(map[string]*FieldValidator),
		Mode:        v.Mode,
		FailStatus:  v.FailStatus,
	}

	// Copy v's fields
	for k, fv := range v.Fields {
		result.Fields[k] = fv
	}
	for k, fv := range v.PathParams {
		result.PathParams[k] = fv
	}
	for k, fv := range v.QueryParams {
		result.QueryParams[k] = fv
	}
	for k, fv := range v.Headers {
		result.Headers[k] = fv
	}

	// Override with other's fields
	result.Required = append(result.Required, other.Required...)
	for k, fv := range other.Fields {
		result.Fields[k] = fv
	}
	for k, fv := range other.PathParams {
		result.PathParams[k] = fv
	}
	for k, fv := range other.QueryParams {
		result.QueryParams[k] = fv
	}
	for k, fv := range other.Headers {
		result.Headers[k] = fv
	}

	if other.Schema != nil {
		result.Schema = other.Schema
	}
	if other.SchemaRef != "" {
		result.SchemaRef = other.SchemaRef
	}
	if other.Mode != "" {
		result.Mode = other.Mode
	}
	if other.FailStatus != 0 {
		result.FailStatus = other.FailStatus
	}

	return result
}

// ToRequestValidation converts StatefulValidation shared rules to RequestValidation
func (v *StatefulValidation) ToRequestValidation() *RequestValidation {
	if v == nil {
		return nil
	}
	return &RequestValidation{
		Required:   v.Required,
		Fields:     v.Fields,
		PathParams: v.PathParams,
		Schema:     v.Schema,
		SchemaRef:  v.SchemaRef,
		Mode:       v.Mode,
	}
}

// GetCreateValidation returns validation rules for create operations
func (v *StatefulValidation) GetCreateValidation() *RequestValidation {
	if v == nil {
		return nil
	}
	base := v.ToRequestValidation()
	if v.OnCreate != nil {
		return base.Merge(v.OnCreate)
	}
	return base
}

// GetUpdateValidation returns validation rules for update operations
func (v *StatefulValidation) GetUpdateValidation() *RequestValidation {
	if v == nil {
		return nil
	}
	base := v.ToRequestValidation()
	if v.OnUpdate != nil {
		return base.Merge(v.OnUpdate)
	}
	return base
}
