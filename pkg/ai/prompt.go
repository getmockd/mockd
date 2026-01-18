package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// System prompt for AI providers
const systemPrompt = `You are a mock data generator for API testing. Your task is to generate realistic, contextually appropriate test data.

Rules:
1. Return ONLY the generated value, no explanations or formatting
2. For JSON objects, return valid JSON
3. For arrays, return valid JSON arrays
4. For strings, return the string without quotes unless it's JSON
5. For numbers, return just the number
6. For booleans, return "true" or "false"
7. Make data realistic and contextually appropriate based on field names and descriptions
8. Use common patterns (e.g., realistic emails, phone formats, addresses)
9. Never include placeholder text like "example" or "test" unless contextually appropriate`

// buildPrompt creates a prompt for a single field generation.
func buildPrompt(req *GenerateRequest) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Generate a realistic value for a field named '%s' of type '%s'.\n", req.FieldName, req.FieldType))

	if req.Format != "" {
		b.WriteString(fmt.Sprintf("Format: %s\n", req.Format))
	}

	if req.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", req.Description))
	}

	if req.Context != "" {
		b.WriteString(fmt.Sprintf("Context: %s\n", req.Context))
	}

	if len(req.Examples) > 0 {
		b.WriteString(fmt.Sprintf("Examples: %s\n", strings.Join(req.Examples, ", ")))
	}

	if req.Constraints != nil {
		if req.Constraints.MinLength != nil {
			b.WriteString(fmt.Sprintf("Minimum length: %d\n", *req.Constraints.MinLength))
		}
		if req.Constraints.MaxLength != nil {
			b.WriteString(fmt.Sprintf("Maximum length: %d\n", *req.Constraints.MaxLength))
		}
		if req.Constraints.Minimum != nil {
			b.WriteString(fmt.Sprintf("Minimum value: %g\n", *req.Constraints.Minimum))
		}
		if req.Constraints.Maximum != nil {
			b.WriteString(fmt.Sprintf("Maximum value: %g\n", *req.Constraints.Maximum))
		}
		if len(req.Constraints.Enum) > 0 {
			enumStrs := make([]string, len(req.Constraints.Enum))
			for i, e := range req.Constraints.Enum {
				enumStrs[i] = fmt.Sprintf("%v", e)
			}
			b.WriteString(fmt.Sprintf("Allowed values: %s\n", strings.Join(enumStrs, ", ")))
		}
	}

	if req.Schema != nil {
		schemaJSON, err := json.Marshal(req.Schema)
		if err == nil {
			b.WriteString(fmt.Sprintf("Schema: %s\n", string(schemaJSON)))
		}
	}

	b.WriteString("\nReturn ONLY the generated value, nothing else.")

	return b.String()
}

// buildBatchPrompt creates a prompt for generating multiple fields at once.
func buildBatchPrompt(reqs []*GenerateRequest) string {
	var b strings.Builder

	b.WriteString("Generate realistic values for the following fields. Return a JSON object with field names as keys.\n\n")

	for i, req := range reqs {
		b.WriteString(fmt.Sprintf("%d. Field: %s (type: %s)", i+1, req.FieldName, req.FieldType))
		if req.Format != "" {
			b.WriteString(fmt.Sprintf(", format: %s", req.Format))
		}
		if req.Description != "" {
			b.WriteString(fmt.Sprintf(", description: %s", req.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString("\nReturn ONLY the JSON object with generated values, no additional text.")

	return b.String()
}

// parseGeneratedValue parses the AI response into the appropriate type.
func parseGeneratedValue(response string, fieldType string) (interface{}, error) {
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	response = stripCodeBlocks(response)

	switch fieldType {
	case "string":
		// Remove surrounding quotes if present
		if len(response) >= 2 && response[0] == '"' && response[len(response)-1] == '"' {
			var s string
			if err := json.Unmarshal([]byte(response), &s); err == nil {
				return s, nil
			}
		}
		return response, nil

	case "integer", "int", "int32", "int64":
		// Try to parse as integer
		if i, err := strconv.ParseInt(response, 10, 64); err == nil {
			return i, nil
		}
		// Try as float and convert
		if f, err := strconv.ParseFloat(response, 64); err == nil {
			return int64(f), nil
		}
		return nil, fmt.Errorf("cannot parse %q as integer", response)

	case "number", "float", "float32", "float64", "double":
		if f, err := strconv.ParseFloat(response, 64); err == nil {
			return f, nil
		}
		return nil, fmt.Errorf("cannot parse %q as number", response)

	case "boolean", "bool":
		response = strings.ToLower(response)
		if response == "true" || response == "1" || response == "yes" {
			return true, nil
		}
		if response == "false" || response == "0" || response == "no" {
			return false, nil
		}
		return nil, fmt.Errorf("cannot parse %q as boolean", response)

	case "object":
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(response), &obj); err != nil {
			return nil, fmt.Errorf("cannot parse as object: %w", err)
		}
		return obj, nil

	case "array":
		var arr []interface{}
		if err := json.Unmarshal([]byte(response), &arr); err != nil {
			return nil, fmt.Errorf("cannot parse as array: %w", err)
		}
		return arr, nil

	default:
		// Try JSON first
		var result interface{}
		if err := json.Unmarshal([]byte(response), &result); err == nil {
			return result, nil
		}
		// Return as string if all else fails
		return response, nil
	}
}

// parseBatchResponse parses a batch response containing multiple field values.
func parseBatchResponse(response string, reqs []*GenerateRequest) ([]interface{}, error) {
	response = strings.TrimSpace(response)
	response = stripCodeBlocks(response)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch response as JSON: %w", err)
	}

	values := make([]interface{}, len(reqs))
	for i, req := range reqs {
		if v, ok := result[req.FieldName]; ok {
			values[i] = v
		} else {
			// Try to generate a default value
			values[i] = generateDefaultValue(req.FieldType)
		}
	}

	return values, nil
}

// stripCodeBlocks removes markdown code blocks from the response.
func stripCodeBlocks(s string) string {
	// Remove ```json ... ``` blocks
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) > 2 {
			// Remove first and last lines
			lines = lines[1 : len(lines)-1]
			// Check if last line is just ```
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			s = strings.Join(lines, "\n")
		}
	}
	return strings.TrimSpace(s)
}

// generateDefaultValue generates a default value for a type when AI fails.
func generateDefaultValue(fieldType string) interface{} {
	switch fieldType {
	case "string":
		return ""
	case "integer", "int", "int32", "int64":
		return int64(0)
	case "number", "float", "float32", "float64", "double":
		return float64(0)
	case "boolean", "bool":
		return false
	case "object":
		return map[string]interface{}{}
	case "array":
		return []interface{}{}
	default:
		return nil
	}
}
