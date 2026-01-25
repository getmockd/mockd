package validation

import (
	"log/slog"
	"reflect"
	"regexp"
	"strings"
)

// InferValidation automatically generates validation rules from seed data.
// It analyzes the seed data to determine:
// - Required fields (present in all items)
// - Field types (from Go type detection)
// - String formats (email, uuid, date, etc.)
// - Path parameter patterns (from basePath)
func InferValidation(seedData []map[string]interface{}, basePath string, logger *slog.Logger) *StatefulValidation {
	if len(seedData) == 0 {
		return nil
	}

	if logger == nil {
		logger = slog.Default()
	}

	validation := &StatefulValidation{
		Auto:       true,
		Required:   []string{},
		Fields:     make(map[string]*FieldValidator),
		PathParams: make(map[string]*FieldValidator),
	}

	// Analyze fields across all seed data items
	fieldInfo := analyzeFields(seedData)

	// Determine required fields and infer validators
	for fieldName, info := range fieldInfo {
		// Skip the ID field - it's auto-generated on create
		if fieldName == "id" {
			continue
		}

		// Field is required if present in all items
		if info.presentCount == len(seedData) {
			validation.Required = append(validation.Required, fieldName)
		}

		// Create field validator
		validator := &FieldValidator{
			Type:     info.inferredType,
			Required: info.presentCount == len(seedData),
		}

		// Add format if detected
		if info.format != "" {
			validator.Format = info.format
		}

		// Add string constraints if applicable
		if info.inferredType == "string" {
			if info.minLength > 0 {
				minLen := info.minLength
				validator.MinLength = &minLen
			}
			if info.maxLength > 0 && info.maxLength < 10000 {
				maxLen := info.maxLength
				validator.MaxLength = &maxLen
			}
		}

		// Add number constraints if applicable
		if info.inferredType == "number" || info.inferredType == "integer" {
			if info.minValue != nil {
				validator.Min = info.minValue
			}
			if info.maxValue != nil {
				validator.Max = info.maxValue
			}
		}

		validation.Fields[fieldName] = validator
	}

	// Infer path parameter validation from basePath
	pathParams := extractPathParams(basePath)
	for _, param := range pathParams {
		// Try to infer pattern from seed data if the field exists
		pattern := inferPathParamPattern(param, seedData)
		validation.PathParams[param] = &FieldValidator{
			Type:     "string",
			Required: true,
			Pattern:  pattern,
			Message:  formatPatternMessage(param, pattern),
		}
	}

	logger.Info("inferred validation from seed data",
		"required", validation.Required,
		"fields", len(validation.Fields),
		"pathParams", len(validation.PathParams),
	)

	return validation
}

// fieldAnalysis holds analysis results for a single field
type fieldAnalysis struct {
	presentCount int
	inferredType string
	format       string
	minLength    int
	maxLength    int
	minValue     *float64
	maxValue     *float64
}

// analyzeFields analyzes all fields across seed data items
func analyzeFields(seedData []map[string]interface{}) map[string]*fieldAnalysis {
	result := make(map[string]*fieldAnalysis)

	for _, item := range seedData {
		for fieldName, value := range item {
			if value == nil {
				continue
			}

			info, exists := result[fieldName]
			if !exists {
				info = &fieldAnalysis{}
				result[fieldName] = info
			}

			info.presentCount++

			// Infer type
			valueType := inferType(value)
			if info.inferredType == "" {
				info.inferredType = valueType
			} else if info.inferredType != valueType {
				// Type inconsistency - default to most general
				info.inferredType = "string"
			}

			// Type-specific analysis
			switch v := value.(type) {
			case string:
				analyzeString(v, info)
			case float64:
				analyzeNumber(v, info)
			case int:
				analyzeNumber(float64(v), info)
			case int64:
				analyzeNumber(float64(v), info)
			}
		}
	}

	return result
}

// inferType determines the JSON type of a value
func inferType(value interface{}) string {
	if value == nil {
		return ""
	}

	switch value.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case int, int64, int32:
		return "integer"
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
		case reflect.Float32, reflect.Float64:
			return "number"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return "integer"
		case reflect.Bool:
			return "boolean"
		case reflect.Slice, reflect.Array:
			return "array"
		case reflect.Map, reflect.Struct:
			return "object"
		default:
			return "string" // Default to string
		}
	}
}

// analyzeString analyzes a string value for format and constraints
func analyzeString(value string, info *fieldAnalysis) {
	// Track string length
	strLen := len(value)
	if info.minLength == 0 || strLen < info.minLength {
		info.minLength = strLen
	}
	if strLen > info.maxLength {
		info.maxLength = strLen
	}

	// Detect format (only if not already set or matches)
	detectedFormat := DetectFormat(value)
	if detectedFormat != "" {
		if info.format == "" {
			info.format = detectedFormat
		} else if info.format != detectedFormat {
			// Inconsistent format - clear it
			info.format = ""
		}
	}
}

// analyzeNumber analyzes a numeric value for constraints
func analyzeNumber(value float64, info *fieldAnalysis) {
	if info.minValue == nil || value < *info.minValue {
		v := value
		info.minValue = &v
	}
	if info.maxValue == nil || value > *info.maxValue {
		v := value
		info.maxValue = &v
	}
}

// extractPathParams extracts parameter names from a URL path pattern
// e.g., "/api/posts/:postId/comments" -> ["postId"]
func extractPathParams(path string) []string {
	pattern := regexp.MustCompile(`:(\w+)`)
	matches := pattern.FindAllStringSubmatch(path, -1)

	params := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}

	return params
}

// inferPathParamPattern tries to infer a regex pattern for a path parameter
// based on values found in seed data
func inferPathParamPattern(paramName string, seedData []map[string]interface{}) string {
	// Collect sample values from seed data
	var samples []string
	for _, item := range seedData {
		if value, ok := item[paramName]; ok {
			if strVal, ok := value.(string); ok {
				samples = append(samples, strVal)
			}
		}
	}

	if len(samples) == 0 {
		// No samples - use a general pattern
		return `^[a-zA-Z0-9_-]+$`
	}

	// Analyze samples to determine pattern
	return inferPatternFromSamples(samples, paramName)
}

// inferPatternFromSamples determines a regex pattern from sample values
func inferPatternFromSamples(samples []string, fieldName string) string {
	if len(samples) == 0 {
		return `^[a-zA-Z0-9_-]+$`
	}

	// Check if all samples follow a common pattern
	// Pattern 1: prefix-number (e.g., "post-1", "user-123")
	prefixPattern := regexp.MustCompile(`^([a-z]+)-(\d+)$`)
	allMatchPrefix := true
	var prefix string
	for _, sample := range samples {
		if matches := prefixPattern.FindStringSubmatch(sample); matches != nil {
			if prefix == "" {
				prefix = matches[1]
			} else if prefix != matches[1] {
				allMatchPrefix = false
				break
			}
		} else {
			allMatchPrefix = false
			break
		}
	}
	if allMatchPrefix && prefix != "" {
		return `^` + prefix + `-\d+$`
	}

	// Pattern 2: UUID
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	allUUID := true
	for _, sample := range samples {
		if !uuidPattern.MatchString(sample) {
			allUUID = false
			break
		}
	}
	if allUUID {
		return uuidPattern.String()
	}

	// Pattern 3: All numeric
	numericPattern := regexp.MustCompile(`^\d+$`)
	allNumeric := true
	for _, sample := range samples {
		if !numericPattern.MatchString(sample) {
			allNumeric = false
			break
		}
	}
	if allNumeric {
		return `^\d+$`
	}

	// Pattern 4: Alphanumeric with hyphens/underscores (default)
	return `^[a-zA-Z0-9_-]+$`
}

// formatPatternMessage creates a user-friendly message for a pattern
func formatPatternMessage(paramName, pattern string) string {
	// Translate common patterns to readable messages
	switch pattern {
	case `^\d+$`:
		return paramName + " must be a number"
	case `^[a-zA-Z0-9_-]+$`:
		return paramName + " must be alphanumeric (hyphens and underscores allowed)"
	case `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`:
		return paramName + " must be a valid UUID"
	default:
		// Check for prefix-number pattern
		if strings.HasPrefix(pattern, "^") && strings.HasSuffix(pattern, `-\d+$`) {
			prefix := pattern[1 : len(pattern)-5]
			return paramName + " must be in format '" + prefix + "-123'"
		}
		return paramName + " must match required format"
	}
}
