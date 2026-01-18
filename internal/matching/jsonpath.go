// Package matching provides request matching algorithms.
package matching

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ohler55/ojg/jp"
)

// JSONPathResult contains the results of JSONPath matching.
type JSONPathResult struct {
	// Score is the total match score (ScoreJSONPathCondition per matched condition)
	Score int
	// Matched contains the values extracted by each JSONPath expression
	// Keys are sanitized versions of the JSONPath (e.g., "$.user.name" -> "user_name")
	Matched map[string]interface{}
}

// MatchJSONPath evaluates JSONPath conditions against a JSON body.
// Returns a score of +ScoreJSONPathCondition per matched condition and the matched values.
// Returns (0, nil) if body is not valid JSON or any condition fails.
func MatchJSONPath(conditions map[string]interface{}, body []byte) JSONPathResult {
	if len(conditions) == 0 {
		return JSONPathResult{Score: 0, Matched: nil}
	}

	// Parse the body as JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Not valid JSON - return no match (not an error, just doesn't match)
		return JSONPathResult{Score: 0, Matched: nil}
	}

	result := JSONPathResult{
		Score:   0,
		Matched: make(map[string]interface{}),
	}

	for path, expected := range conditions {
		matched, value := matchSingleJSONPath(path, expected, data)
		if !matched {
			// All conditions must match - return 0 if any fails
			return JSONPathResult{Score: 0, Matched: nil}
		}
		result.Score += ScoreJSONPathCondition
		if value != nil {
			key := sanitizeJSONPathKey(path)
			result.Matched[key] = value
		}
	}

	return result
}

// matchSingleJSONPath evaluates a single JSONPath condition.
// Returns (true, extractedValue) if matched, (false, nil) if not.
func matchSingleJSONPath(path string, expected interface{}, data interface{}) (bool, interface{}) {
	// Parse the JSONPath expression
	expr, err := jp.ParseString(path)
	if err != nil {
		// Invalid JSONPath expression - treat as no match
		return false, nil
	}

	// Get all values matching the path
	results := expr.Get(data)

	// No results found
	if len(results) == 0 {
		// Check if this is an existence check for non-existence
		if isExistenceCheck(expected) {
			exists := getExistsValue(expected)
			// If checking for non-existence (exists: false), and nothing found, it's a match
			if !exists {
				return true, nil
			}
		}
		return false, nil
	}

	// Handle existence check
	if isExistenceCheck(expected) {
		exists := getExistsValue(expected)
		// If checking for existence (exists: true), and we found something, it's a match
		// If checking for non-existence (exists: false), and we found something, it's not a match
		if exists {
			return true, results[0]
		}
		return false, nil
	}

	// For wildcard paths that return multiple results, check if any match
	for _, result := range results {
		if valuesEqual(result, expected) {
			return true, result
		}
	}

	return false, nil
}

// isExistenceCheck determines if the expected value is an existence check object.
// An existence check is a map with an "exists" key containing a boolean.
func isExistenceCheck(expected interface{}) bool {
	m, ok := expected.(map[string]interface{})
	if !ok {
		return false
	}
	_, hasExists := m["exists"]
	return hasExists && len(m) == 1
}

// getExistsValue extracts the boolean value from an existence check.
func getExistsValue(expected interface{}) bool {
	m, ok := expected.(map[string]interface{})
	if !ok {
		return false
	}
	exists, ok := m["exists"]
	if !ok {
		return false
	}
	b, ok := exists.(bool)
	return ok && b
}

// valuesEqual compares two values for equality, handling type coercion.
// Supports comparing:
//   - strings
//   - numbers (float64, int, etc.)
//   - booleans
//   - null
func valuesEqual(actual, expected interface{}) bool {
	// Handle nil/null
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Try direct equality first
	if reflect.DeepEqual(actual, expected) {
		return true
	}

	// Handle numeric comparison (JSON numbers are float64)
	actualNum, actualIsNum := toFloat64(actual)
	expectedNum, expectedIsNum := toFloat64(expected)
	if actualIsNum && expectedIsNum {
		return actualNum == expectedNum
	}

	// Handle string comparison
	actualStr, actualIsStr := actual.(string)
	expectedStr, expectedIsStr := expected.(string)
	if actualIsStr && expectedIsStr {
		return actualStr == expectedStr
	}

	// Handle boolean comparison
	actualBool, actualIsBool := actual.(bool)
	expectedBool, expectedIsBool := expected.(bool)
	if actualIsBool && expectedIsBool {
		return actualBool == expectedBool
	}

	return false
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case int16:
		return float64(n), true
	case int8:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint8:
		return float64(n), true
	default:
		return 0, false
	}
}

// sanitizeJSONPathKey converts a JSONPath expression to a valid key name.
// Example: "$.user.name" -> "user_name", "$.items[0].id" -> "items_0_id"
func sanitizeJSONPathKey(path string) string {
	// Remove leading "$." or "$"
	if len(path) > 0 && path[0] == '$' {
		path = path[1:]
	}
	if len(path) > 0 && path[0] == '.' {
		path = path[1:]
	}

	// Replace special characters with underscores
	result := make([]byte, 0, len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch c {
		case '.', '[', ']', '*', '@', '?', '(', ')', ',', ' ':
			// Skip consecutive underscores
			if len(result) > 0 && result[len(result)-1] != '_' {
				result = append(result, '_')
			}
		default:
			result = append(result, c)
		}
	}

	// Trim trailing underscores
	for len(result) > 0 && result[len(result)-1] == '_' {
		result = result[:len(result)-1]
	}

	return string(result)
}

// ValidateJSONPathExpression validates a JSONPath expression at load time.
// Returns an error if the expression is invalid.
func ValidateJSONPathExpression(path string) error {
	_, err := jp.ParseString(path)
	if err != nil {
		return fmt.Errorf("invalid JSONPath expression %q: %w", path, err)
	}
	return nil
}
