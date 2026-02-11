package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/getmockd/mockd/pkg/util"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Validator validates requests against configured rules.
type Validator struct {
	config      *RequestValidation
	schema      *jsonschema.Schema
	schemaError error
	once        sync.Once
}

// NewValidator creates a new Validator from RequestValidation config.
func NewValidator(config *RequestValidation) *Validator {
	return &Validator{
		config: config,
	}
}

// Validate validates request data against all configured rules.
// It returns a Result with validation errors/warnings.
func (v *Validator) Validate(ctx context.Context, body map[string]interface{}, pathParams, queryParams map[string]string, headers map[string]string) *Result {
	if v.config == nil || v.config.IsEmpty() {
		return &Result{Valid: true}
	}

	result := &Result{Valid: true}

	// Validate path parameters
	if len(v.config.PathParams) > 0 && pathParams != nil {
		pathResult := ValidateMap(LocationPath, pathParams, v.config.PathParams)
		result.Merge(pathResult)
	}

	// Validate query parameters
	if len(v.config.QueryParams) > 0 && queryParams != nil {
		queryResult := ValidateMap(LocationQuery, queryParams, v.config.QueryParams)
		result.Merge(queryResult)
	}

	// Validate headers
	if len(v.config.Headers) > 0 && headers != nil {
		headerResult := ValidateMap(LocationHeader, headers, v.config.Headers)
		result.Merge(headerResult)
	}

	// Validate body
	if body != nil {
		bodyResult := v.validateBody(body)
		result.Merge(bodyResult)
	}

	return result
}

// ValidateBody validates just the request body.
func (v *Validator) ValidateBody(body map[string]interface{}) *Result {
	if v.config == nil {
		return &Result{Valid: true}
	}
	return v.validateBody(body)
}

// validateBody performs body validation
func (v *Validator) validateBody(body map[string]interface{}) *Result {
	result := &Result{Valid: true}

	// JSON Schema validation (takes precedence if configured)
	if v.config.Schema != nil || v.config.SchemaRef != "" {
		schemaResult := v.validateJSONSchema(body)
		result.Merge(schemaResult)
		// If schema validation is configured, skip field-level validation
		// (they would be redundant)
		return result
	}

	// Required fields validation
	if len(v.config.Required) > 0 {
		requiredResult := ValidateRequired(LocationBody, body, v.config.Required)
		result.Merge(requiredResult)
	}

	// Field-level validation
	if len(v.config.Fields) > 0 {
		fieldsResult := ValidateFields(LocationBody, body, v.config.Fields)
		result.Merge(fieldsResult)
	}

	return result
}

// validateJSONSchema validates body against JSON Schema
func (v *Validator) validateJSONSchema(body map[string]interface{}) *Result {
	result := &Result{Valid: true}

	// Compile schema on first use
	v.once.Do(func() {
		v.schema, v.schemaError = v.compileSchema()
	})

	if v.schemaError != nil {
		result.AddError(&FieldError{
			Location: LocationBody,
			Code:     ErrCodeSchema,
			Message:  fmt.Sprintf("schema compilation error: %v", v.schemaError),
		})
		return result
	}

	if v.schema == nil {
		return result
	}

	// Validate against schema
	err := v.schema.Validate(body)
	if err != nil {
		// Parse schema validation errors
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			parseSchemaErrors(validationErr, result)
		} else {
			result.AddError(&FieldError{
				Location: LocationBody,
				Code:     ErrCodeSchema,
				Message:  err.Error(),
			})
		}
	}

	return result
}

// compileSchema compiles the JSON Schema
func (v *Validator) compileSchema() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	var schemaData interface{}

	// Load from reference or use inline
	if v.config.SchemaRef != "" {
		cleanPath, safe := util.SafeFilePathAllowAbsolute(v.config.SchemaRef)
		if !safe {
			return nil, fmt.Errorf("unsafe schema file path: %s", v.config.SchemaRef)
		}
		data, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read schema file: %w", err)
		}
		if err := json.Unmarshal(data, &schemaData); err != nil {
			return nil, fmt.Errorf("failed to parse schema file: %w", err)
		}
	} else {
		schemaData = v.config.Schema
	}

	// Convert to JSON and back to ensure consistent types
	schemaBytes, err := json.Marshal(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	if err := compiler.AddResource("schema.json", strings.NewReader(string(schemaBytes))); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	return compiler.Compile("schema.json")
}

// parseSchemaErrors extracts detailed errors from JSON Schema validation
func parseSchemaErrors(err *jsonschema.ValidationError, result *Result) {
	// Handle basic validation errors
	if len(err.Causes) == 0 {
		field := extractFieldFromPath(err.InstanceLocation)
		result.AddError(&FieldError{
			Field:    field,
			Location: LocationBody,
			Code:     ErrCodeSchema,
			Message:  err.Message,
		})
		return
	}

	// Recursively process causes
	for _, cause := range err.Causes {
		parseSchemaErrors(cause, result)
	}
}

// extractFieldFromPath extracts field name from JSON Pointer path
func extractFieldFromPath(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	// Remove leading slash and convert JSON Pointer to dot notation
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "/", ".")
	return path
}

// StatefulValidator validates requests for stateful resources.
type StatefulValidator struct {
	config          *StatefulValidation
	createValidator *Validator
	updateValidator *Validator
	pathValidator   *Validator
}

// NewStatefulValidator creates a validator for stateful resources.
func NewStatefulValidator(config *StatefulValidation) *StatefulValidator {
	if config == nil || config.IsEmpty() {
		return nil
	}

	sv := &StatefulValidator{
		config: config,
	}

	// Create validators for different operations
	createConfig := config.GetCreateValidation()
	if createConfig != nil && !createConfig.IsEmpty() {
		sv.createValidator = NewValidator(createConfig)
	}

	updateConfig := config.GetUpdateValidation()
	if updateConfig != nil && !updateConfig.IsEmpty() {
		sv.updateValidator = NewValidator(updateConfig)
	}

	// Path params validator (shared)
	if len(config.PathParams) > 0 {
		sv.pathValidator = NewValidator(&RequestValidation{
			PathParams: config.PathParams,
		})
	}

	return sv
}

// ValidateCreate validates a create (POST) request.
func (sv *StatefulValidator) ValidateCreate(ctx context.Context, body map[string]interface{}, pathParams map[string]string) *Result {
	result := &Result{Valid: true}

	// Validate path params first
	if sv.pathValidator != nil {
		pathResult := sv.pathValidator.Validate(ctx, nil, pathParams, nil, nil)
		result.Merge(pathResult)
	}

	// Validate body
	if sv.createValidator != nil {
		bodyResult := sv.createValidator.ValidateBody(body)
		result.Merge(bodyResult)
	}

	return result
}

// ValidateUpdate validates an update (PUT/PATCH) request.
func (sv *StatefulValidator) ValidateUpdate(ctx context.Context, body map[string]interface{}, pathParams map[string]string) *Result {
	result := &Result{Valid: true}

	// Validate path params first
	if sv.pathValidator != nil {
		pathResult := sv.pathValidator.Validate(ctx, nil, pathParams, nil, nil)
		result.Merge(pathResult)
	}

	// Validate body
	if sv.updateValidator != nil {
		bodyResult := sv.updateValidator.ValidateBody(body)
		result.Merge(bodyResult)
	}

	return result
}

// ValidatePathParams validates only the path parameters.
func (sv *StatefulValidator) ValidatePathParams(ctx context.Context, pathParams map[string]string) *Result {
	if sv.pathValidator == nil {
		return &Result{Valid: true}
	}
	return sv.pathValidator.Validate(ctx, nil, pathParams, nil, nil)
}

// GetMode returns the validation mode.
func (sv *StatefulValidator) GetMode() string {
	if sv.config == nil {
		return ModeStrict
	}
	return sv.config.GetMode()
}

// HTTPValidator validates HTTP requests for per-mock validation.
type HTTPValidator struct {
	validator *Validator
	config    *RequestValidation
}

// NewHTTPValidator creates a validator for HTTP mocks.
func NewHTTPValidator(config *RequestValidation) *HTTPValidator {
	if config == nil || config.IsEmpty() {
		return nil
	}

	return &HTTPValidator{
		validator: NewValidator(config),
		config:    config,
	}
}

// Validate validates an HTTP request.
func (hv *HTTPValidator) Validate(ctx context.Context, body map[string]interface{}, pathParams, queryParams map[string]string, headers map[string]string) *Result {
	return hv.validator.Validate(ctx, body, pathParams, queryParams, headers)
}

// GetMode returns the validation mode.
func (hv *HTTPValidator) GetMode() string {
	return hv.config.GetMode()
}

// GetFailStatus returns the HTTP status code for failures.
func (hv *HTTPValidator) GetFailStatus() int {
	return hv.config.GetFailStatus()
}
