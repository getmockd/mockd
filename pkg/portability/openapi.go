package portability

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"gopkg.in/yaml.v3"
)

// OpenAPI 3.x types

// OpenAPI represents an OpenAPI 3.x specification.
type OpenAPI struct {
	OpenAPI    string              `json:"openapi" yaml:"openapi"`
	Info       OpenAPIInfo         `json:"info" yaml:"info"`
	Servers    []OpenAPIServer     `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]PathItem `json:"paths" yaml:"paths"`
	Components *OpenAPIComponents  `json:"components,omitempty" yaml:"components,omitempty"`
}

// OpenAPIInfo contains API metadata.
type OpenAPIInfo struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

// OpenAPIServer represents a server entry.
type OpenAPIServer struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// PathItem represents operations on a path.
type PathItem struct {
	Get     *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post    *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Head    *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Options *Operation `json:"options,omitempty" yaml:"options,omitempty"`
}

// Operation represents an API operation.
type Operation struct {
	Summary     string              `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Tags        []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses" yaml:"responses"`
}

// Parameter represents an API parameter.
type Parameter struct {
	Name        string      `json:"name" yaml:"name"`
	In          string      `json:"in" yaml:"in"` // query, header, path, cookie
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool        `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      *Schema     `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// RequestBody represents a request body.
type RequestBody struct {
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]MediaType `json:"content" yaml:"content"`
}

// Response represents an API response.
type Response struct {
	Description string               `json:"description" yaml:"description"`
	Headers     map[string]Header    `json:"headers,omitempty" yaml:"headers,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty" yaml:"content,omitempty"`
}

// Header represents a response header.
type Header struct {
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// MediaType represents a media type in request/response.
type MediaType struct {
	Schema  *Schema     `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// Schema represents a JSON Schema subset used in OpenAPI.
type Schema struct {
	Type       string             `json:"type,omitempty" yaml:"type,omitempty"`
	Format     string             `json:"format,omitempty" yaml:"format,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty" yaml:"items,omitempty"`
	Example    interface{}        `json:"example,omitempty" yaml:"example,omitempty"`
	Ref        string             `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// OpenAPIComponents contains reusable components.
type OpenAPIComponents struct {
	Schemas map[string]*Schema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

// Swagger 2.0 types

// Swagger represents a Swagger 2.0 specification.
type Swagger struct {
	Swagger     string                 `json:"swagger" yaml:"swagger"`
	Info        SwaggerInfo            `json:"info" yaml:"info"`
	Host        string                 `json:"host,omitempty" yaml:"host,omitempty"`
	BasePath    string                 `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	Schemes     []string               `json:"schemes,omitempty" yaml:"schemes,omitempty"`
	Paths       map[string]SwaggerPath `json:"paths" yaml:"paths"`
	Definitions map[string]*Schema     `json:"definitions,omitempty" yaml:"definitions,omitempty"`
}

// SwaggerInfo contains Swagger metadata.
type SwaggerInfo struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

// SwaggerPath represents operations on a path in Swagger 2.0.
type SwaggerPath struct {
	Get     *SwaggerOperation `json:"get,omitempty" yaml:"get,omitempty"`
	Post    *SwaggerOperation `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *SwaggerOperation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete  *SwaggerOperation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch   *SwaggerOperation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Head    *SwaggerOperation `json:"head,omitempty" yaml:"head,omitempty"`
	Options *SwaggerOperation `json:"options,omitempty" yaml:"options,omitempty"`
}

// SwaggerOperation represents an operation in Swagger 2.0.
type SwaggerOperation struct {
	Summary     string                     `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string                     `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Tags        []string                   `json:"tags,omitempty" yaml:"tags,omitempty"`
	Consumes    []string                   `json:"consumes,omitempty" yaml:"consumes,omitempty"`
	Produces    []string                   `json:"produces,omitempty" yaml:"produces,omitempty"`
	Parameters  []SwaggerParameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Responses   map[string]SwaggerResponse `json:"responses" yaml:"responses"`
}

// SwaggerParameter represents a parameter in Swagger 2.0.
type SwaggerParameter struct {
	Name        string      `json:"name" yaml:"name"`
	In          string      `json:"in" yaml:"in"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool        `json:"required,omitempty" yaml:"required,omitempty"`
	Type        string      `json:"type,omitempty" yaml:"type,omitempty"`
	Schema      *Schema     `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty" yaml:"example,omitempty"`
}

// SwaggerResponse represents a response in Swagger 2.0.
type SwaggerResponse struct {
	Description string                 `json:"description" yaml:"description"`
	Schema      *Schema                `json:"schema,omitempty" yaml:"schema,omitempty"`
	Headers     map[string]*Schema     `json:"headers,omitempty" yaml:"headers,omitempty"`
	Examples    map[string]interface{} `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// OpenAPIImporter imports OpenAPI 3.x and Swagger 2.0 specifications.
type OpenAPIImporter struct{}

// Import parses an OpenAPI/Swagger specification and returns a MockCollection.
func (i *OpenAPIImporter) Import(data []byte) (*config.MockCollection, error) {
	// Try to detect version
	var versionCheck struct {
		OpenAPI string `json:"openapi" yaml:"openapi"`
		Swagger string `json:"swagger" yaml:"swagger"`
	}

	// Try YAML first (handles both YAML and JSON)
	if err := yaml.Unmarshal(data, &versionCheck); err != nil {
		return nil, &ImportError{
			Format:  FormatOpenAPI,
			Message: "failed to parse specification",
			Cause:   err,
		}
	}

	if versionCheck.OpenAPI != "" {
		return i.importOpenAPI3(data)
	}

	if versionCheck.Swagger != "" {
		return i.importSwagger2(data)
	}

	return nil, &ImportError{
		Format:  FormatOpenAPI,
		Message: "not a valid OpenAPI 3.x or Swagger 2.0 specification",
	}
}

// importOpenAPI3 imports an OpenAPI 3.x specification.
func (i *OpenAPIImporter) importOpenAPI3(data []byte) (*config.MockCollection, error) {
	var spec OpenAPI
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, &ImportError{
			Format:  FormatOpenAPI,
			Message: "failed to parse OpenAPI 3.x specification",
			Cause:   err,
		}
	}

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    spec.Info.Title,
		Mocks:   make([]*config.MockConfiguration, 0),
	}

	now := time.Now()
	mockID := 1

	// Sort paths for deterministic output
	paths := make([]string, 0, len(spec.Paths))
	for path := range spec.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem := spec.Paths[path]
		operations := []struct {
			method string
			op     *Operation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"DELETE", pathItem.Delete},
			{"PATCH", pathItem.Patch},
			{"HEAD", pathItem.Head},
			{"OPTIONS", pathItem.Options},
		}

		for _, opEntry := range operations {
			if opEntry.op == nil {
				continue
			}

			mock := i.operationToMock(path, opEntry.method, opEntry.op, mockID, now)
			collection.Mocks = append(collection.Mocks, mock)
			mockID++
		}
	}

	return collection, nil
}

// operationToMock converts an OpenAPI operation to a MockConfiguration.
func (i *OpenAPIImporter) operationToMock(path, method string, op *Operation, id int, now time.Time) *config.MockConfiguration {
	// Convert OpenAPI path params {param} to :param for mockd
	mockPath := convertOpenAPIPath(path)

	matcher := &mock.HTTPMatcher{
		Method: method,
		Path:   mockPath,
	}

	name := op.Summary
	if name == "" {
		name = fmt.Sprintf("%s %s", method, path)
	}

	// Extract query parameters
	queryParams := make(map[string]string)
	for _, param := range op.Parameters {
		if param.In == "query" && param.Example != nil {
			queryParams[param.Name] = fmt.Sprintf("%v", param.Example)
		}
	}
	if len(queryParams) > 0 {
		matcher.QueryParams = queryParams
	}

	// Find the best response (prefer 200, then 201, then first success, then first)
	statusCode, response := findBestResponse(op.Responses)

	respDef := &mock.HTTPResponse{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
	}

	// Set content type header
	if response != nil {
		for contentType, mediaType := range response.Content {
			respDef.Headers["Content-Type"] = contentType
			if mediaType.Example != nil {
				bodyBytes, _ := json.MarshalIndent(mediaType.Example, "", "  ")
				respDef.Body = string(bodyBytes)
			} else if mediaType.Schema != nil && mediaType.Schema.Example != nil {
				bodyBytes, _ := json.MarshalIndent(mediaType.Schema.Example, "", "  ")
				respDef.Body = string(bodyBytes)
			}
			break // Use first content type
		}
	}

	// Generate default body if none found
	if respDef.Body == "" {
		respDef.Body = generateDefaultBody(statusCode)
	}

	enabled := true
	return &config.MockConfiguration{
		ID:        fmt.Sprintf("imported-%d", id),
		Type:      mock.MockTypeHTTP,
		Name:      name,
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher:  matcher,
			Response: respDef,
		},
	}
}

// importSwagger2 imports a Swagger 2.0 specification.
func (i *OpenAPIImporter) importSwagger2(data []byte) (*config.MockCollection, error) {
	var spec Swagger
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, &ImportError{
			Format:  FormatOpenAPI,
			Message: "failed to parse Swagger 2.0 specification",
			Cause:   err,
		}
	}

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    spec.Info.Title,
		Mocks:   make([]*config.MockConfiguration, 0),
	}

	now := time.Now()
	mockID := 1

	// Sort paths for deterministic output
	paths := make([]string, 0, len(spec.Paths))
	for path := range spec.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem := spec.Paths[path]

		// Prepend basePath if present
		fullPath := path
		if spec.BasePath != "" && spec.BasePath != "/" {
			fullPath = strings.TrimSuffix(spec.BasePath, "/") + path
		}

		operations := []struct {
			method string
			op     *SwaggerOperation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"DELETE", pathItem.Delete},
			{"PATCH", pathItem.Patch},
			{"HEAD", pathItem.Head},
			{"OPTIONS", pathItem.Options},
		}

		for _, opEntry := range operations {
			if opEntry.op == nil {
				continue
			}

			mock := i.swaggerOperationToMock(fullPath, opEntry.method, opEntry.op, mockID, now)
			collection.Mocks = append(collection.Mocks, mock)
			mockID++
		}
	}

	return collection, nil
}

// swaggerOperationToMock converts a Swagger 2.0 operation to a MockConfiguration.
func (i *OpenAPIImporter) swaggerOperationToMock(path, method string, op *SwaggerOperation, id int, now time.Time) *config.MockConfiguration {
	mockPath := convertOpenAPIPath(path)

	name := op.Summary
	if name == "" {
		name = fmt.Sprintf("%s %s", method, path)
	}

	// Find best response
	statusCode, response := findBestSwaggerResponse(op.Responses)

	respHeaders := make(map[string]string)
	// Set content type from produces
	if len(op.Produces) > 0 {
		respHeaders["Content-Type"] = op.Produces[0]
	}

	respBody := ""
	// Extract example from response
	if response != nil {
		if len(response.Examples) > 0 {
			for _, example := range response.Examples {
				bodyBytes, _ := json.MarshalIndent(example, "", "  ")
				respBody = string(bodyBytes)
				break
			}
		} else if response.Schema != nil && response.Schema.Example != nil {
			bodyBytes, _ := json.MarshalIndent(response.Schema.Example, "", "  ")
			respBody = string(bodyBytes)
		}
	}

	if respBody == "" {
		respBody = generateDefaultBody(statusCode)
	}

	enabled2 := true
	return &config.MockConfiguration{
		ID:        fmt.Sprintf("imported-%d", id),
		Type:      mock.MockTypeHTTP,
		Name:      name,
		Enabled:   &enabled2,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: method,
				Path:   mockPath,
			},
			Response: &mock.HTTPResponse{
				StatusCode: statusCode,
				Headers:    respHeaders,
				Body:       respBody,
			},
		},
	}
}

// Format returns FormatOpenAPI.
func (i *OpenAPIImporter) Format() Format {
	return FormatOpenAPI
}

// OpenAPIExporter exports to OpenAPI 3.x format.
type OpenAPIExporter struct {
	// AsYAML if true, outputs YAML instead of JSON
	AsYAML bool
}

// Export converts a MockCollection to an OpenAPI 3.x specification.
func (e *OpenAPIExporter) Export(collection *config.MockCollection) ([]byte, error) {
	if collection == nil {
		return nil, &ExportError{
			Format:  FormatOpenAPI,
			Message: "collection cannot be nil",
		}
	}

	spec := &OpenAPI{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       collection.Name,
			Description: "Exported from mockd",
			Version:     "1.0.0",
		},
		Paths: make(map[string]PathItem),
	}

	// Group mocks by path
	pathMocks := make(map[string][]*config.MockConfiguration)
	for _, m := range collection.Mocks {
		if m.HTTP == nil || m.HTTP.Matcher == nil {
			continue
		}
		path := convertMockdPath(m.HTTP.Matcher.Path)
		pathMocks[path] = append(pathMocks[path], m)
	}

	// Convert each path
	for path, mocks := range pathMocks {
		pathItem := PathItem{}
		for _, m := range mocks {
			op := e.mockToOperation(m)
			method := ""
			if m.HTTP != nil && m.HTTP.Matcher != nil {
				method = m.HTTP.Matcher.Method
			}
			switch strings.ToUpper(method) {
			case "GET":
				pathItem.Get = op
			case "POST":
				pathItem.Post = op
			case "PUT":
				pathItem.Put = op
			case "DELETE":
				pathItem.Delete = op
			case "PATCH":
				pathItem.Patch = op
			case "HEAD":
				pathItem.Head = op
			case "OPTIONS":
				pathItem.Options = op
			}
		}
		spec.Paths[path] = pathItem
	}

	var data []byte
	var err error

	if e.AsYAML {
		data, err = yaml.Marshal(spec)
	} else {
		data, err = json.MarshalIndent(spec, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
	}

	if err != nil {
		return nil, &ExportError{
			Format:  FormatOpenAPI,
			Message: "failed to marshal OpenAPI specification",
			Cause:   err,
		}
	}

	return data, nil
}

// mockToOperation converts a MockConfiguration to an OpenAPI Operation.
func (e *OpenAPIExporter) mockToOperation(m *config.MockConfiguration) *Operation {
	op := &Operation{
		Summary:   m.Name,
		Responses: make(map[string]Response),
	}

	// Add query parameters
	if m.HTTP != nil && m.HTTP.Matcher != nil && len(m.HTTP.Matcher.QueryParams) > 0 {
		for name, value := range m.HTTP.Matcher.QueryParams {
			op.Parameters = append(op.Parameters, Parameter{
				Name:    name,
				In:      "query",
				Example: value,
				Schema:  &Schema{Type: "string"},
			})
		}
	}

	// Add response
	if m.HTTP != nil && m.HTTP.Response != nil {
		statusStr := fmt.Sprintf("%d", m.HTTP.Response.StatusCode)
		response := Response{
			Description: fmt.Sprintf("%d response", m.HTTP.Response.StatusCode),
		}

		if m.HTTP.Response.Body != "" {
			contentType := "application/json"
			if ct, ok := m.HTTP.Response.Headers["Content-Type"]; ok {
				contentType = ct
			}

			var example interface{}
			if err := json.Unmarshal([]byte(m.HTTP.Response.Body), &example); err == nil {
				response.Content = map[string]MediaType{
					contentType: {Example: example},
				}
			} else {
				response.Content = map[string]MediaType{
					contentType: {Example: m.HTTP.Response.Body},
				}
			}
		}

		op.Responses[statusStr] = response
	}

	return op
}

// Format returns FormatOpenAPI.
func (e *OpenAPIExporter) Format() Format {
	return FormatOpenAPI
}

// Helper functions

// convertOpenAPIPath converts OpenAPI path params {param} to mockd format :param.
func convertOpenAPIPath(path string) string {
	result := path
	for strings.Contains(result, "{") {
		start := strings.Index(result, "{")
		end := strings.Index(result, "}")
		if start >= 0 && end > start {
			paramName := result[start+1 : end]
			result = result[:start] + ":" + paramName + result[end+1:]
		} else {
			break
		}
	}
	return result
}

// convertMockdPath converts mockd path params :param to OpenAPI format {param}.
func convertMockdPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// findBestResponse finds the best response to use from OpenAPI responses.
func findBestResponse(responses map[string]Response) (int, *Response) {
	if len(responses) == 0 {
		return 200, nil
	}

	// Prefer 200, then 201, then first 2xx, then first
	preferredOrder := []string{"200", "201", "202", "204"}
	for _, status := range preferredOrder {
		if resp, ok := responses[status]; ok {
			code := parseStatusCode(status)
			return code, &resp
		}
	}

	// Find first 2xx
	for status, resp := range responses {
		if strings.HasPrefix(status, "2") {
			code := parseStatusCode(status)
			return code, &resp
		}
	}

	// Return first response
	for status, resp := range responses {
		code := parseStatusCode(status)
		return code, &resp
	}

	return 200, nil
}

// findBestSwaggerResponse finds the best response from Swagger 2.0 responses.
func findBestSwaggerResponse(responses map[string]SwaggerResponse) (int, *SwaggerResponse) {
	if len(responses) == 0 {
		return 200, nil
	}

	preferredOrder := []string{"200", "201", "202", "204"}
	for _, status := range preferredOrder {
		if resp, ok := responses[status]; ok {
			code := parseStatusCode(status)
			return code, &resp
		}
	}

	for status, resp := range responses {
		if strings.HasPrefix(status, "2") {
			code := parseStatusCode(status)
			return code, &resp
		}
	}

	for status, resp := range responses {
		code := parseStatusCode(status)
		return code, &resp
	}

	return 200, nil
}

// parseStatusCode parses a status code string to int.
func parseStatusCode(s string) int {
	if s == "" {
		return 200
	}
	code := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			code = code*10 + int(c-'0')
		} else {
			return 200
		}
	}
	if code >= 100 && code < 600 {
		return code
	}
	return 200
}

// generateDefaultBody generates a default response body for a status code.
func generateDefaultBody(statusCode int) string {
	switch statusCode {
	case 200:
		return `{"status": "ok"}`
	case 201:
		return `{"id": 1, "created": true}`
	case 204:
		return ""
	case 400:
		return `{"error": "Bad Request"}`
	case 401:
		return `{"error": "Unauthorized"}`
	case 403:
		return `{"error": "Forbidden"}`
	case 404:
		return `{"error": "Not Found"}`
	case 500:
		return `{"error": "Internal Server Error"}`
	default:
		return fmt.Sprintf(`{"status": %d}`, statusCode)
	}
}

// init registers the OpenAPI importer and exporter.
func init() {
	RegisterImporter(&OpenAPIImporter{})
	RegisterExporter(&OpenAPIExporter{AsYAML: true})
}
