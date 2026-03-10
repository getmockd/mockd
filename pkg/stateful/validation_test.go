package stateful

import (
	"context"
	"testing"

	"github.com/getmockd/mockd/pkg/validation"
)

// =============================================================================
// initValidation Tests
// =============================================================================

func TestInitValidation_NilConfig(t *testing.T) {
	// When validationConfig is nil, initValidation should be a no-op
	r := &StatefulResource{
		validationConfig: nil,
		seedData:         []map[string]interface{}{{"name": "Alice"}},
	}
	r.initValidation(nil)

	if r.validator != nil {
		t.Error("validator should be nil when validationConfig is nil")
	}
}

func TestInitValidation_AutoInferFromSeedData(t *testing.T) {
	// Auto=true with seed data should infer validation rules
	cfg := &ResourceConfig{
		Name: "users",
		SeedData: []map[string]interface{}{
			{"id": "u1", "name": "Alice", "email": "alice@example.com"},
			{"id": "u2", "name": "Bob", "email": "bob@example.com"},
		},
		Validation: &validation.StatefulValidation{
			Auto: true,
		},
	}
	r := NewStatefulResource(cfg)

	if r.validator == nil {
		t.Fatal("validator should be non-nil after auto-infer with seed data")
	}
	if !r.HasValidation() {
		t.Error("HasValidation() should return true")
	}
	// Inferred config should include "name" and "email" as required (present in all seed items)
	if r.validationConfig == nil {
		t.Fatal("validationConfig should not be nil after initValidation")
	}
	if len(r.validationConfig.Required) == 0 {
		t.Error("expected inferred required fields from seed data")
	}
	if len(r.validationConfig.Fields) == 0 {
		t.Error("expected inferred field validators from seed data")
	}
}

func TestInitValidation_AutoWithoutSeedData(t *testing.T) {
	// Auto=true but no seed data — no inference possible, but explicit config should still work
	cfg := &ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Auto:     true,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	}
	r := NewStatefulResource(cfg)

	if r.validator == nil {
		t.Fatal("validator should be non-nil with explicit required/fields config")
	}
}

func TestInitValidation_ExplicitOnly(t *testing.T) {
	// Auto=false with explicit fields — should use explicit config directly
	cfg := &ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Required: []string{"title"},
			Fields: map[string]*validation.FieldValidator{
				"title": {Type: "string", Required: true},
			},
		},
	}
	r := NewStatefulResource(cfg)

	if r.validator == nil {
		t.Fatal("validator should be non-nil with explicit config")
	}
}

func TestInitValidation_EmptyValidation(t *testing.T) {
	// Empty StatefulValidation (no Auto, no fields, no required) — should result in nil validator
	cfg := &ResourceConfig{
		Name:       "items",
		Validation: &validation.StatefulValidation{},
	}
	r := NewStatefulResource(cfg)

	if r.validator != nil {
		t.Error("validator should be nil for empty validation config")
	}
	if r.HasValidation() {
		t.Error("HasValidation() should return false for empty validation config")
	}
}

func TestInitValidation_AutoMergesWithExplicit(t *testing.T) {
	// Auto=true with seed data AND explicit overrides — explicit should take precedence
	minLen := 1
	cfg := &ResourceConfig{
		Name: "products",
		SeedData: []map[string]interface{}{
			{"id": "p1", "name": "Widget", "price": 9.99},
			{"id": "p2", "name": "Gadget", "price": 19.99},
		},
		Validation: &validation.StatefulValidation{
			Auto: true,
			Fields: map[string]*validation.FieldValidator{
				// Override the inferred "name" field with an explicit constraint
				"name": {Type: "string", MinLength: &minLen},
			},
		},
	}
	r := NewStatefulResource(cfg)

	if r.validator == nil {
		t.Fatal("validator should be non-nil")
	}
	// The explicit "name" field validator should override the inferred one
	nameField := r.validationConfig.Fields["name"]
	if nameField == nil {
		t.Fatal("expected 'name' field validator after merge")
	}
	if nameField.MinLength == nil || *nameField.MinLength != 1 {
		t.Error("explicit MinLength override should take precedence")
	}
	// price should still be inferred
	priceField := r.validationConfig.Fields["price"]
	if priceField == nil {
		t.Error("expected inferred 'price' field validator")
	}
}

// =============================================================================
// mergeStatefulValidation Tests
// =============================================================================

func TestMergeStatefulValidation_NilBase(t *testing.T) {
	override := &validation.StatefulValidation{
		Required: []string{"name"},
		Mode:     validation.ModeWarn,
	}
	result := mergeStatefulValidation(nil, override)
	if result != override {
		t.Error("expected override to be returned when base is nil")
	}
}

func TestMergeStatefulValidation_NilOverride(t *testing.T) {
	base := &validation.StatefulValidation{
		Required: []string{"name"},
		Mode:     validation.ModeStrict,
	}
	result := mergeStatefulValidation(base, nil)
	if result != base {
		t.Error("expected base to be returned when override is nil")
	}
}

func TestMergeStatefulValidation_BothNil(t *testing.T) {
	result := mergeStatefulValidation(nil, nil)
	if result != nil {
		t.Error("expected nil when both are nil")
	}
}

func TestMergeStatefulValidation_FieldsMerge(t *testing.T) {
	base := &validation.StatefulValidation{
		Required: []string{"name"},
		Fields: map[string]*validation.FieldValidator{
			"name":  {Type: "string"},
			"email": {Type: "string", Format: "email"},
		},
		PathParams: map[string]*validation.FieldValidator{
			"userId": {Type: "string"},
		},
		Mode: validation.ModeStrict,
	}
	minLen := 5
	override := &validation.StatefulValidation{
		Auto:     true,
		Required: []string{"email"},
		Fields: map[string]*validation.FieldValidator{
			// Override name field
			"name": {Type: "string", MinLength: &minLen},
			// Add new field
			"age": {Type: "integer"},
		},
		PathParams: map[string]*validation.FieldValidator{
			"orgId": {Type: "string"},
		},
		Mode: validation.ModeWarn,
	}

	result := mergeStatefulValidation(base, override)

	// Auto should come from override
	if !result.Auto {
		t.Error("Auto should be true from override")
	}

	// Required should have both base and override entries
	if len(result.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(result.Required))
	}
	foundName, foundEmail := false, false
	for _, r := range result.Required {
		if r == "name" {
			foundName = true
		}
		if r == "email" {
			foundEmail = true
		}
	}
	if !foundName || !foundEmail {
		t.Errorf("expected both 'name' and 'email' in required, got %v", result.Required)
	}

	// Fields should contain all three, with override taking precedence for "name"
	if len(result.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(result.Fields))
	}
	if result.Fields["name"] == nil || result.Fields["name"].MinLength == nil || *result.Fields["name"].MinLength != 5 {
		t.Error("name field should use override's MinLength")
	}
	if result.Fields["email"] == nil {
		t.Error("email field should be present from base")
	}
	if result.Fields["age"] == nil {
		t.Error("age field should be present from override")
	}

	// PathParams should have both
	if len(result.PathParams) != 2 {
		t.Errorf("expected 2 path params, got %d", len(result.PathParams))
	}

	// Mode should come from override
	if result.Mode != validation.ModeWarn {
		t.Errorf("Mode should be %q from override, got %q", validation.ModeWarn, result.Mode)
	}
}

func TestMergeStatefulValidation_OnCreateOnUpdateOverride(t *testing.T) {
	base := &validation.StatefulValidation{
		Required: []string{"name"},
		Fields:   map[string]*validation.FieldValidator{},
	}
	onCreateConfig := &validation.RequestValidation{
		Required: []string{"password"},
	}
	onUpdateConfig := &validation.RequestValidation{
		Required: []string{"reason"},
	}
	override := &validation.StatefulValidation{
		OnCreate: onCreateConfig,
		OnUpdate: onUpdateConfig,
	}

	result := mergeStatefulValidation(base, override)

	if result.OnCreate != onCreateConfig {
		t.Error("OnCreate should be set from override")
	}
	if result.OnUpdate != onUpdateConfig {
		t.Error("OnUpdate should be set from override")
	}
}

func TestMergeStatefulValidation_SchemaOverride(t *testing.T) {
	base := &validation.StatefulValidation{
		Required: []string{"a"},
		Fields:   map[string]*validation.FieldValidator{},
	}
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	override := &validation.StatefulValidation{
		Schema:    schema,
		SchemaRef: "/path/to/schema.json",
	}

	result := mergeStatefulValidation(base, override)

	if result.Schema == nil {
		t.Error("Schema should be set from override")
	}
	if result.SchemaRef != "/path/to/schema.json" {
		t.Errorf("SchemaRef should be %q, got %q", "/path/to/schema.json", result.SchemaRef)
	}
}

func TestMergeStatefulValidation_ModeEmptyOverrideKeepsBase(t *testing.T) {
	base := &validation.StatefulValidation{
		Required: []string{},
		Fields:   map[string]*validation.FieldValidator{},
		Mode:     validation.ModePermissive,
	}
	override := &validation.StatefulValidation{
		// Mode is empty — base mode should be preserved
	}

	result := mergeStatefulValidation(base, override)

	if result.Mode != validation.ModePermissive {
		t.Errorf("Mode should remain %q when override is empty, got %q", validation.ModePermissive, result.Mode)
	}
}

// =============================================================================
// ValidateCreate / ValidateUpdate Tests
// =============================================================================

func TestValidateCreate_NoValidator(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{Name: "items"})

	result := r.ValidateCreate(context.Background(), map[string]interface{}{"name": "test"}, nil)

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Valid {
		t.Error("validation should pass when no validator is configured")
	}
}

func TestValidateUpdate_NoValidator(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{Name: "items"})

	result := r.ValidateUpdate(context.Background(), map[string]interface{}{"name": "test"}, nil)

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Valid {
		t.Error("validation should pass when no validator is configured")
	}
}

func TestValidateCreate_ValidData(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Required: []string{"name", "email"},
			Fields: map[string]*validation.FieldValidator{
				"name":  {Type: "string", Required: true},
				"email": {Type: "string", Required: true, Format: "email"},
			},
		},
	})

	result := r.ValidateCreate(context.Background(), map[string]interface{}{
		"name":  "Alice",
		"email": "alice@example.com",
	}, nil)

	if !result.Valid {
		t.Errorf("expected valid result, got errors: %v", result.Errors)
	}
}

func TestValidateCreate_MissingRequiredField(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Required: []string{"name", "email"},
			Fields: map[string]*validation.FieldValidator{
				"name":  {Type: "string", Required: true},
				"email": {Type: "string", Required: true},
			},
		},
	})

	result := r.ValidateCreate(context.Background(), map[string]interface{}{
		"name": "Alice",
		// email missing
	}, nil)

	if result.Valid {
		t.Error("expected validation to fail for missing required field")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error")
	}

	// Check that the error references the missing field
	foundEmailError := false
	for _, err := range result.Errors {
		if err.Field == "email" {
			foundEmailError = true
		}
	}
	if !foundEmailError {
		t.Error("expected error for missing 'email' field")
	}
}

func TestValidateCreate_WrongType(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Fields: map[string]*validation.FieldValidator{
				"count": {Type: "integer"},
			},
		},
	})

	result := r.ValidateCreate(context.Background(), map[string]interface{}{
		"count": "not-a-number",
	}, nil)

	if result.Valid {
		t.Error("expected validation to fail for wrong type")
	}
}

func TestValidateUpdate_ValidData(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string"},
			},
		},
	})

	result := r.ValidateUpdate(context.Background(), map[string]interface{}{
		"name": "Updated Name",
	}, nil)

	if !result.Valid {
		t.Errorf("expected valid result, got errors: %v", result.Errors)
	}
}

func TestValidateUpdate_InvalidData(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Fields: map[string]*validation.FieldValidator{
				"score": {Type: "number"},
			},
		},
	})

	result := r.ValidateUpdate(context.Background(), map[string]interface{}{
		"score": "not-a-number",
	}, nil)

	if result.Valid {
		t.Error("expected validation to fail for wrong type on update")
	}
}

func TestValidateCreate_OnCreateOverride(t *testing.T) {
	// OnCreate has extra required fields beyond shared fields
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
			OnCreate: &validation.RequestValidation{
				Required: []string{"password"},
				Fields: map[string]*validation.FieldValidator{
					"password": {Type: "string", Required: true},
				},
			},
		},
	})

	// Missing password — should fail
	result := r.ValidateCreate(context.Background(), map[string]interface{}{
		"name": "Alice",
	}, nil)

	if result.Valid {
		t.Error("expected validation to fail when password is missing on create")
	}
}

func TestValidateUpdate_OnUpdateOverride(t *testing.T) {
	// OnUpdate has different required fields
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string"},
			},
			OnUpdate: &validation.RequestValidation{
				Required: []string{"reason"},
				Fields: map[string]*validation.FieldValidator{
					"reason": {Type: "string", Required: true},
				},
			},
		},
	})

	// Missing reason — should fail
	result := r.ValidateUpdate(context.Background(), map[string]interface{}{
		"name": "Updated",
	}, nil)

	if result.Valid {
		t.Error("expected validation to fail when 'reason' is missing on update")
	}
}

func TestValidateCreate_WithAutoInferredValidation(t *testing.T) {
	// Auto-inferred validation from seed data should be enforced on create
	r := NewStatefulResource(&ResourceConfig{
		Name: "products",
		SeedData: []map[string]interface{}{
			{"id": "p1", "name": "Widget", "price": 9.99},
			{"id": "p2", "name": "Gadget", "price": 19.99},
		},
		Validation: &validation.StatefulValidation{
			Auto: true,
		},
	})

	if !r.HasValidation() {
		t.Fatal("expected validation to be configured via auto-infer")
	}

	// Valid data should pass — values must fit within inferred constraints
	// (name min length = 6 from "Widget"/"Gadget", price max = 19.99 from seed data)
	result := r.ValidateCreate(context.Background(), map[string]interface{}{
		"name":  "Foobar",
		"price": 14.99,
	}, nil)
	if !result.Valid {
		t.Errorf("expected valid data to pass auto-inferred validation, got errors: %v", result.Errors)
	}

	// Missing required field should fail
	result = r.ValidateCreate(context.Background(), map[string]interface{}{
		// name and price both missing
	}, nil)
	if result.Valid {
		t.Error("expected validation to fail when all auto-inferred required fields are missing")
	}
}

// =============================================================================
// ValidatePathParams Tests
// =============================================================================

func TestValidatePathParams_NoValidator(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{Name: "items"})

	result := r.ValidatePathParams(context.Background(), map[string]string{"id": "123"})

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Valid {
		t.Error("validation should pass when no validator is configured")
	}
}

func TestValidatePathParams_ValidParams(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "comments",
		Validation: &validation.StatefulValidation{
			PathParams: map[string]*validation.FieldValidator{
				"postId": {Type: "string", Required: true, Pattern: `^\d+$`},
			},
			// Need at least one field or required to make the validation non-empty
			Required: []string{},
			Fields: map[string]*validation.FieldValidator{
				"body": {Type: "string"},
			},
		},
	})

	result := r.ValidatePathParams(context.Background(), map[string]string{
		"postId": "123",
	})

	if !result.Valid {
		t.Errorf("expected valid path params, got errors: %v", result.Errors)
	}
}

func TestValidatePathParams_InvalidPattern(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "comments",
		Validation: &validation.StatefulValidation{
			PathParams: map[string]*validation.FieldValidator{
				"postId": {Type: "string", Required: true, Pattern: `^\d+$`},
			},
			Fields: map[string]*validation.FieldValidator{
				"body": {Type: "string"},
			},
		},
	})

	result := r.ValidatePathParams(context.Background(), map[string]string{
		"postId": "not-a-number",
	})

	if result.Valid {
		t.Error("expected validation to fail for invalid path param pattern")
	}
}

// =============================================================================
// shouldRejectValidation Tests
// =============================================================================

func TestShouldRejectValidation_StrictMode(t *testing.T) {
	// Strict mode: any error should reject
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "name",
		Code:    validation.ErrCodeType,
		Message: "wrong type",
	})

	if !shouldRejectValidation(result, validation.ModeStrict) {
		t.Error("strict mode should reject on any validation error")
	}
}

func TestShouldRejectValidation_StrictMode_RequiredError(t *testing.T) {
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "email",
		Code:    validation.ErrCodeRequired,
		Message: "field 'email' is required",
	})

	if !shouldRejectValidation(result, validation.ModeStrict) {
		t.Error("strict mode should reject on required error")
	}
}

func TestShouldRejectValidation_WarnMode_NeverRejects(t *testing.T) {
	// Warn mode: never reject, regardless of error type
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "email",
		Code:    validation.ErrCodeRequired,
		Message: "field 'email' is required",
	})

	if shouldRejectValidation(result, validation.ModeWarn) {
		t.Error("warn mode should never reject")
	}
}

func TestShouldRejectValidation_WarnMode_TypeError(t *testing.T) {
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "score",
		Code:    validation.ErrCodeType,
		Message: "wrong type",
	})

	if shouldRejectValidation(result, validation.ModeWarn) {
		t.Error("warn mode should never reject, even for type errors")
	}
}

func TestShouldRejectValidation_PermissiveMode_RequiredError(t *testing.T) {
	// Permissive mode: only reject on required field errors
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "name",
		Code:    validation.ErrCodeRequired,
		Message: "field 'name' is required",
	})

	if !shouldRejectValidation(result, validation.ModePermissive) {
		t.Error("permissive mode should reject on required field errors")
	}
}

func TestShouldRejectValidation_PermissiveMode_TypeError(t *testing.T) {
	// Permissive mode: should NOT reject on non-required errors
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "score",
		Code:    validation.ErrCodeType,
		Message: "wrong type",
	})

	if shouldRejectValidation(result, validation.ModePermissive) {
		t.Error("permissive mode should not reject on type errors")
	}
}

func TestShouldRejectValidation_PermissiveMode_MixedErrors(t *testing.T) {
	// Permissive mode with a mix of required and non-required errors — should reject
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "score",
		Code:    validation.ErrCodeType,
		Message: "wrong type",
	})
	result.AddError(&validation.FieldError{
		Field:   "name",
		Code:    validation.ErrCodeRequired,
		Message: "field 'name' is required",
	})

	if !shouldRejectValidation(result, validation.ModePermissive) {
		t.Error("permissive mode should reject when required errors are present")
	}
}

func TestShouldRejectValidation_PermissiveMode_OnlyNonRequiredErrors(t *testing.T) {
	// Multiple non-required errors — should NOT reject
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "email",
		Code:    validation.ErrCodeFormat,
		Message: "invalid email format",
	})
	result.AddError(&validation.FieldError{
		Field:   "score",
		Code:    validation.ErrCodeMin,
		Message: "must be >= 0",
	})

	if shouldRejectValidation(result, validation.ModePermissive) {
		t.Error("permissive mode should not reject when no required errors present")
	}
}

func TestShouldRejectValidation_DefaultMode(t *testing.T) {
	// Empty string mode should behave like strict (default)
	result := &validation.Result{Valid: false}
	result.AddError(&validation.FieldError{
		Field:   "name",
		Code:    validation.ErrCodeType,
		Message: "wrong type",
	})

	if !shouldRejectValidation(result, "") {
		t.Error("empty/default mode should behave like strict and reject")
	}
}

// =============================================================================
// GetValidationMode Tests
// =============================================================================

func TestGetValidationMode_NoValidator(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{Name: "items"})

	mode := r.GetValidationMode()
	if mode != validation.ModeStrict {
		t.Errorf("expected default mode %q when no validator, got %q", validation.ModeStrict, mode)
	}
}

func TestGetValidationMode_StrictExplicit(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Mode:     validation.ModeStrict,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})

	mode := r.GetValidationMode()
	if mode != validation.ModeStrict {
		t.Errorf("expected %q, got %q", validation.ModeStrict, mode)
	}
}

func TestGetValidationMode_Warn(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Mode:     validation.ModeWarn,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})

	mode := r.GetValidationMode()
	if mode != validation.ModeWarn {
		t.Errorf("expected %q, got %q", validation.ModeWarn, mode)
	}
}

func TestGetValidationMode_Permissive(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Mode:     validation.ModePermissive,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})

	mode := r.GetValidationMode()
	if mode != validation.ModePermissive {
		t.Errorf("expected %q, got %q", validation.ModePermissive, mode)
	}
}

func TestGetValidationMode_DefaultsToStrict(t *testing.T) {
	// Mode unset in config — should default to strict
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})

	mode := r.GetValidationMode()
	if mode != validation.ModeStrict {
		t.Errorf("expected default %q, got %q", validation.ModeStrict, mode)
	}
}

// =============================================================================
// Integration: Validation Mode affects Create/Update behavior
// =============================================================================

func TestValidation_StrictMode_RejectsInvalidCreate(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Mode:     validation.ModeStrict,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})

	result := r.ValidateCreate(context.Background(), map[string]interface{}{}, nil)
	if result.Valid {
		t.Error("strict mode should detect missing required field")
	}
	if !shouldRejectValidation(result, r.GetValidationMode()) {
		t.Error("strict mode should reject the request")
	}
}

func TestValidation_WarnMode_DoesNotRejectInvalidCreate(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Mode:     validation.ModeWarn,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})

	result := r.ValidateCreate(context.Background(), map[string]interface{}{}, nil)
	if result.Valid {
		t.Error("warn mode should still detect validation errors")
	}
	if shouldRejectValidation(result, r.GetValidationMode()) {
		t.Error("warn mode should NOT reject the request")
	}
}

func TestValidation_PermissiveMode_RejectsOnlyRequiredErrors(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "users",
		Validation: &validation.StatefulValidation{
			Mode:     validation.ModePermissive,
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name":  {Type: "string", Required: true},
				"score": {Type: "number"},
			},
		},
	})

	// Missing required field — should reject in permissive mode
	result := r.ValidateCreate(context.Background(), map[string]interface{}{
		"score": 42.0,
	}, nil)
	if result.Valid {
		t.Error("should detect missing required field")
	}
	if !shouldRejectValidation(result, r.GetValidationMode()) {
		t.Error("permissive mode should reject when required field is missing")
	}

	// Wrong type on non-required field — should NOT reject in permissive mode
	result = r.ValidateCreate(context.Background(), map[string]interface{}{
		"name":  "Alice",
		"score": "not-a-number",
	}, nil)
	if result.Valid {
		t.Error("should detect type error")
	}
	if shouldRejectValidation(result, r.GetValidationMode()) {
		t.Error("permissive mode should NOT reject when only non-required errors present")
	}
}

// =============================================================================
// HasValidation Tests
// =============================================================================

func TestHasValidation_False(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{Name: "items"})
	if r.HasValidation() {
		t.Error("HasValidation should be false with no validation config")
	}
}

func TestHasValidation_True(t *testing.T) {
	r := NewStatefulResource(&ResourceConfig{
		Name: "items",
		Validation: &validation.StatefulValidation{
			Required: []string{"name"},
			Fields: map[string]*validation.FieldValidator{
				"name": {Type: "string", Required: true},
			},
		},
	})
	if !r.HasValidation() {
		t.Error("HasValidation should be true with validation configured")
	}
}
