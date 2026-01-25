package validation

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// ValidateField validates a value against a FieldValidator
func ValidateField(field, location string, value interface{}, validator *FieldValidator) *Result {
	result := &Result{Valid: true}

	if validator == nil {
		return result
	}

	// Handle nil/null values
	if value == nil {
		if validator.Required {
			result.AddError(NewRequiredError(field, location))
		} else if !validator.Nullable && validator.Type != "" {
			// If type is specified and value is null, that's an error (unless nullable)
			result.AddError(NewTypeError(field, location, validator.Type, nil))
		}
		return result
	}

	// Type validation
	if validator.Type != "" {
		typeResult := validateType(field, location, value, validator.Type)
		result.Merge(typeResult)
		if !typeResult.Valid {
			return result // Stop on type mismatch
		}
	}

	// Type-specific validations
	switch v := value.(type) {
	case string:
		validateString(field, location, v, validator, result)
	case float64:
		validateNumber(field, location, v, validator, result)
	case int:
		validateNumber(field, location, float64(v), validator, result)
	case int64:
		validateNumber(field, location, float64(v), validator, result)
	case bool:
		// No additional validation for booleans beyond type check
	case []interface{}:
		validateArray(field, location, v, validator, result)
	case map[string]interface{}:
		validateObject(field, location, v, validator, result)
	}

	// Enum validation (applies to any type)
	if len(validator.Enum) > 0 {
		validateEnum(field, location, value, validator.Enum, result)
	}

	return result
}

// validateType checks if a value matches the expected JSON type
func validateType(field, location string, value interface{}, expectedType string) *Result {
	result := &Result{Valid: true}

	actualType := getJSONType(value)
	expected := strings.ToLower(expectedType)

	// Handle integer as a special case of number
	if expected == "integer" {
		if actualType != "number" {
			result.AddError(NewTypeError(field, location, expectedType, value))
			return result
		}
		// Check if it's a whole number
		if num, ok := value.(float64); ok {
			if num != float64(int64(num)) {
				result.AddError(NewTypeError(field, location, "integer", value))
			}
		}
		return result
	}

	if actualType != expected {
		result.AddError(NewTypeError(field, location, expectedType, value))
	}

	return result
}

// getJSONType returns the JSON type name for a value
func getJSONType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case float64, int, int64, float32, int32:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		// Use reflection for other types
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.String:
			return "string"
		case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return "number"
		case reflect.Bool:
			return "boolean"
		case reflect.Slice, reflect.Array:
			return "array"
		case reflect.Map, reflect.Struct:
			return "object"
		default:
			return "unknown"
		}
	}
}

// validateString validates string-specific constraints
func validateString(field, location string, value string, validator *FieldValidator, result *Result) {
	// MinLength
	if validator.MinLength != nil && len(value) < *validator.MinLength {
		result.AddError(NewMinLengthError(field, location, *validator.MinLength, len(value)))
	}

	// MaxLength
	if validator.MaxLength != nil && len(value) > *validator.MaxLength {
		result.AddError(NewMaxLengthError(field, location, *validator.MaxLength, len(value)))
	}

	// Pattern (regex)
	if validator.Pattern != "" {
		matched, err := regexp.MatchString(validator.Pattern, value)
		if err != nil || !matched {
			err := NewPatternError(field, location, validator.Pattern, value)
			if validator.Message != "" {
				err.Message = validator.Message
			}
			result.AddError(err)
		}
	}

	// Format validation
	if validator.Format != "" {
		if !ValidateFormat(validator.Format, value) {
			err := NewFormatError(field, location, validator.Format, value)
			if validator.Message != "" {
				err.Message = validator.Message
			}
			result.AddError(err)
		}
	}
}

// validateNumber validates number-specific constraints
func validateNumber(field, location string, value float64, validator *FieldValidator, result *Result) {
	// Min
	if validator.Min != nil && value < *validator.Min {
		result.AddError(NewMinError(field, location, *validator.Min, value))
	}

	// Max
	if validator.Max != nil && value > *validator.Max {
		result.AddError(NewMaxError(field, location, *validator.Max, value))
	}

	// ExclusiveMin
	if validator.ExclusiveMin != nil && value <= *validator.ExclusiveMin {
		err := &FieldError{
			Field:    field,
			Location: location,
			Code:     ErrCodeExclusiveMin,
			Message:  fmt.Sprintf("must be > %v", *validator.ExclusiveMin),
			Received: value,
			Expected: fmt.Sprintf("> %v", *validator.ExclusiveMin),
		}
		result.AddError(err)
	}

	// ExclusiveMax
	if validator.ExclusiveMax != nil && value >= *validator.ExclusiveMax {
		err := &FieldError{
			Field:    field,
			Location: location,
			Code:     ErrCodeExclusiveMax,
			Message:  fmt.Sprintf("must be < %v", *validator.ExclusiveMax),
			Received: value,
			Expected: fmt.Sprintf("< %v", *validator.ExclusiveMax),
		}
		result.AddError(err)
	}
}

// validateArray validates array-specific constraints
func validateArray(field, location string, value []interface{}, validator *FieldValidator, result *Result) {
	// MinItems
	if validator.MinItems != nil && len(value) < *validator.MinItems {
		result.AddError(NewMinItemsError(field, location, *validator.MinItems, len(value)))
	}

	// MaxItems
	if validator.MaxItems != nil && len(value) > *validator.MaxItems {
		result.AddError(NewMaxItemsError(field, location, *validator.MaxItems, len(value)))
	}

	// UniqueItems
	if validator.UniqueItems && len(value) > 1 {
		seen := make(map[string]bool)
		for _, item := range value {
			// Convert to JSON string for comparison
			key, _ := json.Marshal(item)
			keyStr := string(key)
			if seen[keyStr] {
				result.AddError(NewUniqueItemsError(field, location, item))
				break
			}
			seen[keyStr] = true
		}
	}

	// Items validation
	if validator.Items != nil {
		for i, item := range value {
			itemField := fmt.Sprintf("%s[%d]", field, i)
			itemResult := ValidateField(itemField, location, item, validator.Items)
			result.Merge(itemResult)
		}
	}
}

// validateObject validates object-specific constraints
func validateObject(field, location string, value map[string]interface{}, validator *FieldValidator, result *Result) {
	if validator.Properties == nil {
		return
	}

	// Check required fields in properties
	for propName, propValidator := range validator.Properties {
		propField := field
		if propField != "" {
			propField = field + "." + propName
		} else {
			propField = propName
		}

		propValue, exists := value[propName]
		if !exists {
			if propValidator.Required {
				result.AddError(NewRequiredError(propField, location))
			}
			continue
		}

		propResult := ValidateField(propField, location, propValue, propValidator)
		result.Merge(propResult)
	}
}

// validateEnum checks if value is one of the allowed enum values
func validateEnum(field, location string, value interface{}, enum []interface{}, result *Result) {
	for _, allowed := range enum {
		if valuesEqual(value, allowed) {
			return
		}
	}
	result.AddError(NewEnumError(field, location, enum, value))
}

// valuesEqual compares two values for equality (handles type coercion for numbers)
func valuesEqual(a, b interface{}) bool {
	// Direct equality
	if a == b {
		return true
	}

	// Handle number comparisons (int vs float64)
	aNum, aIsNum := toFloat64(a)
	bNum, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return aNum == bNum
	}

	// Use JSON encoding for complex types
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// toFloat64 attempts to convert a value to float64
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// ValidateRequired checks that all required fields are present
func ValidateRequired(location string, data map[string]interface{}, required []string) *Result {
	result := &Result{Valid: true}

	for _, field := range required {
		if _, exists := data[field]; !exists {
			result.AddError(NewRequiredError(field, location))
		}
	}

	return result
}

// ValidateFields validates all fields in data against field validators
func ValidateFields(location string, data map[string]interface{}, fields map[string]*FieldValidator) *Result {
	result := &Result{Valid: true}

	for fieldName, validator := range fields {
		value, exists := data[fieldName]
		if !exists {
			if validator.Required {
				result.AddError(NewRequiredError(fieldName, location))
			}
			continue
		}

		fieldResult := ValidateField(fieldName, location, value, validator)
		result.Merge(fieldResult)
	}

	return result
}

// ValidateMap validates a map of string values (for path params, query params, headers)
func ValidateMap(location string, data map[string]string, validators map[string]*FieldValidator) *Result {
	result := &Result{Valid: true}

	for fieldName, validator := range validators {
		value, exists := data[fieldName]
		if !exists {
			if validator.Required {
				result.AddError(NewRequiredError(fieldName, location))
			}
			continue
		}

		// For string maps, validate as string
		fieldResult := ValidateField(fieldName, location, value, validator)
		result.Merge(fieldResult)
	}

	return result
}
