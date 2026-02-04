package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getmockd/mockd/pkg/ai"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/google/uuid"
)

// Generator provides high-level AI-powered mock generation utilities.
type Generator struct {
	provider ai.Provider
}

// New creates a new Generator with the specified AI provider.
func New(provider ai.Provider) *Generator {
	return &Generator{
		provider: provider,
	}
}

// EnhanceOpenAPISchema generates a realistic value for an OpenAPI schema.
func (g *Generator) EnhanceOpenAPISchema(ctx context.Context, schema *openapi3.Schema, fieldName string) (interface{}, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	// If there's an example, use it
	if schema.Example != nil {
		return schema.Example, nil
	}

	// If there's a default, use it
	if schema.Default != nil {
		return schema.Default, nil
	}

	// If there are enum values, pick the first one
	if len(schema.Enum) > 0 {
		return schema.Enum[0], nil
	}

	// Build the request for AI generation
	req := g.schemaToRequest(schema, fieldName)

	resp, err := g.provider.Generate(ctx, req)
	if err != nil {
		// nolint:nilerr // intentionally returning fallback value when AI generation fails
		return g.generateFallbackValue(schema), nil
	}

	return resp.Value, nil
}

// EnhanceOpenAPISchemaRecursive generates realistic values for a complex schema.
func (g *Generator) EnhanceOpenAPISchemaRecursive(ctx context.Context, schema *openapi3.Schema, fieldName string) (interface{}, error) {
	if schema == nil {
		return nil, nil
	}

	switch schema.Type.Slice()[0] {
	case "object":
		return g.enhanceObject(ctx, schema, fieldName)
	case "array":
		return g.enhanceArray(ctx, schema, fieldName)
	default:
		return g.EnhanceOpenAPISchema(ctx, schema, fieldName)
	}
}

//nolint:unparam // error return kept for API consistency with other enhance methods
func (g *Generator) enhanceObject(ctx context.Context, schema *openapi3.Schema, _ string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for propName, propRef := range schema.Properties {
		if propRef.Value == nil {
			continue
		}

		value, err := g.EnhanceOpenAPISchemaRecursive(ctx, propRef.Value, propName)
		if err != nil {
			// Use fallback for this property
			value = g.generateFallbackValue(propRef.Value)
		}
		result[propName] = value
	}

	return result, nil
}

//nolint:unparam // error is always nil but kept for consistency with interface
func (g *Generator) enhanceArray(ctx context.Context, schema *openapi3.Schema, fieldName string) ([]interface{}, error) {
	if schema.Items == nil || schema.Items.Value == nil {
		return []interface{}{}, nil
	}

	// Generate 1-3 items
	count := 2
	if schema.MinItems > 0 && uint64(count) < schema.MinItems {
		count = int(schema.MinItems)
	}
	if schema.MaxItems != nil && uint64(count) > *schema.MaxItems {
		count = int(*schema.MaxItems)
	}

	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		value, err := g.EnhanceOpenAPISchemaRecursive(ctx, schema.Items.Value, fieldName)
		if err != nil {
			value = g.generateFallbackValue(schema.Items.Value)
		}
		result[i] = value
	}

	return result, nil
}

func (g *Generator) schemaToRequest(schema *openapi3.Schema, fieldName string) *ai.GenerateRequest {
	req := &ai.GenerateRequest{
		FieldName:   fieldName,
		FieldType:   getSchemaType(schema),
		Format:      schema.Format,
		Description: schema.Description,
		Schema:      schema,
	}

	// Extract examples
	if schema.Example != nil {
		req.Examples = []string{fmt.Sprintf("%v", schema.Example)}
	}

	// Extract constraints
	constraints := &ai.FieldConstraints{}
	hasConstraints := false

	if schema.MinLength > 0 {
		minLen := int(schema.MinLength)
		constraints.MinLength = &minLen
		hasConstraints = true
	}
	if schema.MaxLength != nil {
		maxLen := int(*schema.MaxLength)
		constraints.MaxLength = &maxLen
		hasConstraints = true
	}
	if schema.Min != nil {
		constraints.Minimum = schema.Min
		hasConstraints = true
	}
	if schema.Max != nil {
		constraints.Maximum = schema.Max
		hasConstraints = true
	}
	if schema.Pattern != "" {
		constraints.Pattern = schema.Pattern
		hasConstraints = true
	}
	if len(schema.Enum) > 0 {
		constraints.Enum = schema.Enum
		hasConstraints = true
	}

	if hasConstraints {
		req.Constraints = constraints
	}

	return req
}

func getSchemaType(schema *openapi3.Schema) string {
	types := schema.Type.Slice()
	if len(types) > 0 {
		return types[0]
	}
	return "string"
}

func (g *Generator) generateFallbackValue(schema *openapi3.Schema) interface{} {
	if schema == nil {
		return nil
	}

	schemaType := getSchemaType(schema)

	switch schemaType {
	case "string":
		return generateFallbackString(schema)
	case "integer":
		return 1
	case "number":
		return 1.0
	case "boolean":
		return true
	case "array":
		return []interface{}{}
	case "object":
		return map[string]interface{}{}
	default:
		return ""
	}
}

func generateFallbackString(schema *openapi3.Schema) string {
	switch schema.Format {
	case "email":
		return "user@example.com"
	case "uri", "url":
		return "https://example.com"
	case "uuid":
		return uuid.New().String()
	case "date":
		return time.Now().Format("2006-01-02")
	case "date-time":
		return time.Now().Format(time.RFC3339)
	case "hostname":
		return "example.com"
	case "ipv4":
		return "192.168.1.1"
	case "ipv6":
		return "::1"
	default:
		return "string"
	}
}

// GenerateFromDescription creates mock configurations from a natural language description.
func (g *Generator) GenerateFromDescription(ctx context.Context, description string) ([]*config.MockConfiguration, error) {
	prompt := fmt.Sprintf(`Generate a JSON array of mock API endpoint configurations based on this description: %s

Each mock should have this structure:
{
  "name": "endpoint name",
  "method": "GET|POST|PUT|DELETE|PATCH",
  "path": "/api/path/{param}",
  "statusCode": 200,
  "responseBody": { ... realistic response data ... }
}

IMPORTANT: Use {param} syntax for path parameters (NOT :param). For example: /api/users/{id}, /api/posts/{postId}.
Return ONLY the raw JSON array. Do NOT wrap in markdown code fences. No explanation.`, description)

	resp, err := g.provider.Generate(ctx, &ai.GenerateRequest{
		FieldName: "mocks",
		FieldType: "array",
		Context:   prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate mocks: %w", err)
	}

	return g.parseMockResponse(resp.RawResponse)
}

type mockEndpoint struct {
	Name         string      `json:"name"`
	Method       string      `json:"method"`
	Path         string      `json:"path"`
	StatusCode   int         `json:"statusCode"`
	ResponseBody interface{} `json:"responseBody"`
}

func (g *Generator) parseMockResponse(response string) ([]*config.MockConfiguration, error) {
	response = strings.TrimSpace(response)

	// Strip markdown code fences — LLMs frequently wrap JSON in ```json ... ```
	response = stripCodeFences(response)

	// Try to parse as JSON array
	var endpoints []mockEndpoint
	if err := json.Unmarshal([]byte(response), &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse generated mocks: %w", err)
	}

	mocks := make([]*config.MockConfiguration, len(endpoints))
	now := time.Now()

	for i, ep := range endpoints {
		body := ""
		if ep.ResponseBody != nil {
			bodyBytes, _ := json.MarshalIndent(ep.ResponseBody, "", "  ")
			body = string(bodyBytes)
		}

		statusCode := ep.StatusCode
		if statusCode == 0 {
			statusCode = 200
		}

		// Normalize path: convert Express-style :param to mockd-style {param}
		path := normalizePathParams(ep.Path)

		enabled := true
		mocks[i] = &config.MockConfiguration{
			ID:        fmt.Sprintf("ai-generated-%d", i+1),
			Type:      mock.MockTypeHTTP,
			Name:      ep.Name,
			Enabled:   &enabled,
			CreatedAt: now,
			UpdatedAt: now,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: strings.ToUpper(ep.Method),
					Path:   path,
				},
				Response: &mock.HTTPResponse{
					StatusCode: statusCode,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: body,
				},
			},
		}
	}

	return mocks, nil
}

// EnhanceMock improves an existing mock with AI-generated response data.
func (g *Generator) EnhanceMock(ctx context.Context, m *config.MockConfiguration) error {
	if m == nil || m.HTTP == nil || m.HTTP.Response == nil {
		return nil
	}

	// Skip if the response body is not JSON or is already non-trivial
	body := strings.TrimSpace(m.HTTP.Response.Body)
	if body != "" && body != "{}" && body != `{"status": "ok"}` {
		// Already has a meaningful body
		return nil
	}

	// Generate based on the mock context
	method, path := "", ""
	if m.HTTP.Matcher != nil {
		method = m.HTTP.Matcher.Method
		path = m.HTTP.Matcher.Path
	}
	ctxStr := fmt.Sprintf("API endpoint: %s %s", method, path)
	if m.Name != "" {
		ctxStr += fmt.Sprintf(", Name: %s", m.Name)
	}
	if m.Description != "" {
		ctxStr += fmt.Sprintf(", Description: %s", m.Description)
	}

	resp, err := g.provider.Generate(ctx, &ai.GenerateRequest{
		FieldName:   "response",
		FieldType:   "object",
		Context:     ctxStr,
		Description: "Generate a realistic JSON response for this API endpoint",
	})
	if err != nil {
		return fmt.Errorf("failed to enhance mock: %w", err)
	}

	// Format as JSON
	switch v := resp.Value.(type) {
	case map[string]interface{}:
		bodyBytes, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		m.HTTP.Response.Body = string(bodyBytes)
	case string:
		m.HTTP.Response.Body = v
	default:
		bodyBytes, err := json.MarshalIndent(resp.Value, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		m.HTTP.Response.Body = string(bodyBytes)
	}

	return nil
}

// stripCodeFences removes markdown code fences from an LLM response.
// LLMs frequently wrap JSON in ```json ... ``` despite being told not to.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)

	// Check for opening fence (```json, ```JSON, or just ```)
	if strings.HasPrefix(s, "```") {
		// Find end of first line (the opening fence)
		idx := strings.Index(s, "\n")
		if idx >= 0 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if strings.HasSuffix(strings.TrimSpace(s), "```") {
			s = strings.TrimSpace(s)
			s = s[:len(s)-3]
			s = strings.TrimSpace(s)
		}
	}
	return s
}

// normalizePathParams converts Express-style :param path segments to mockd-style {param}.
// For example, "/users/:id/posts/:postId" becomes "/users/{id}/posts/{postId}".
// LLMs frequently generate Express/Rails-style params since they're common in training data.
func normalizePathParams(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") && len(seg) > 1 {
			segments[i] = "{" + seg[1:] + "}"
		}
	}
	return strings.Join(segments, "/")
}
