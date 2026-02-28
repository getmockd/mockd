package portability

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMockoonImporter_BasicRoute(t *testing.T) {
	env := MockoonEnvironment{
		Name: "Test API",
		Port: 3001,
		Routes: []MockoonRoute{
			{
				UUID:     "route-1-uuid-abcdef1234567890",
				Type:     "http",
				Method:   "get",
				Endpoint: "users",
				Enabled:  true,
				Responses: []MockoonResponse{
					{
						UUID:       "resp-1",
						StatusCode: 200,
						Body:       `[{"id": 1, "name": "Alice"}]`,
						Headers: []MockoonHeader{
							{Key: "Content-Type", Value: "application/json"},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(collection.Mocks) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(collection.Mocks))
	}

	m := collection.Mocks[0]
	if m.HTTP.Matcher.Method != "GET" {
		t.Errorf("method = %q, want GET", m.HTTP.Matcher.Method)
	}
	if m.HTTP.Matcher.Path != "/users" {
		t.Errorf("path = %q, want /users", m.HTTP.Matcher.Path)
	}
	if m.HTTP.Response.StatusCode != 200 {
		t.Errorf("statusCode = %d, want 200", m.HTTP.Response.StatusCode)
	}
	if m.HTTP.Response.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header missing or wrong")
	}
	if !strings.Contains(m.HTTP.Response.Body, "Alice") {
		t.Errorf("body should contain Alice, got %q", m.HTTP.Response.Body)
	}
}

func TestMockoonImporter_EndpointPrefix(t *testing.T) {
	env := MockoonEnvironment{
		Name:           "API with prefix",
		EndpointPrefix: "api/v1",
		Routes: []MockoonRoute{
			{
				UUID:     "route-prefix-uuid-1234567890ab",
				Type:     "http",
				Method:   "get",
				Endpoint: "users",
				Enabled:  true,
				Responses: []MockoonResponse{
					{StatusCode: 200, Body: "{}"},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if collection.Mocks[0].HTTP.Matcher.Path != "/api/v1/users" {
		t.Errorf("path = %q, want /api/v1/users", collection.Mocks[0].HTTP.Matcher.Path)
	}
}

func TestMockoonImporter_PathParams(t *testing.T) {
	env := MockoonEnvironment{
		Name: "Path params",
		Routes: []MockoonRoute{
			{
				UUID:     "route-params-uuid-1234567890ab",
				Type:     "http",
				Method:   "get",
				Endpoint: "users/:id/posts/:postId",
				Enabled:  true,
				Responses: []MockoonResponse{
					{StatusCode: 200, Body: "{}"},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	expected := "/users/{id}/posts/{postId}"
	if collection.Mocks[0].HTTP.Matcher.Path != expected {
		t.Errorf("path = %q, want %q", collection.Mocks[0].HTTP.Matcher.Path, expected)
	}
}

func TestMockoonImporter_MultipleResponses(t *testing.T) {
	env := MockoonEnvironment{
		Name: "Multi-response",
		Routes: []MockoonRoute{
			{
				UUID:     "route-multi-uuid-1234567890ab",
				Type:     "http",
				Method:   "get",
				Endpoint: "status",
				Enabled:  true,
				Responses: []MockoonResponse{
					{UUID: "r1", StatusCode: 200, Body: `{"status": "ok"}`},
					{UUID: "r2", StatusCode: 500, Body: `{"status": "error"}`},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(collection.Mocks) != 2 {
		t.Fatalf("expected 2 mocks (one per response), got %d", len(collection.Mocks))
	}

	if collection.Mocks[0].HTTP.Response.StatusCode != 200 {
		t.Errorf("first mock status = %d, want 200", collection.Mocks[0].HTTP.Response.StatusCode)
	}
	if collection.Mocks[1].HTTP.Response.StatusCode != 500 {
		t.Errorf("second mock status = %d, want 500", collection.Mocks[1].HTTP.Response.StatusCode)
	}

	// First response should have higher priority
	if collection.Mocks[0].HTTP.Priority <= collection.Mocks[1].HTTP.Priority {
		t.Error("first response should have higher priority than second")
	}
}

func TestMockoonImporter_Latency(t *testing.T) {
	env := MockoonEnvironment{
		Name:    "Latency test",
		Latency: 100, // global 100ms
		Routes: []MockoonRoute{
			{
				UUID:     "route-lat-uuid-1234567890abcd",
				Type:     "http",
				Method:   "get",
				Endpoint: "slow",
				Enabled:  true,
				Responses: []MockoonResponse{
					{StatusCode: 200, Body: "{}", LatencyMs: 200}, // per-response 200ms
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Delay should be 200 (per-response) + 100 (global) = 300
	if collection.Mocks[0].HTTP.Response.DelayMs != 300 {
		t.Errorf("delay = %d, want 300", collection.Mocks[0].HTTP.Response.DelayMs)
	}
}

func TestMockoonImporter_RulesToMatchers(t *testing.T) {
	env := MockoonEnvironment{
		Name: "Rules test",
		Routes: []MockoonRoute{
			{
				UUID:     "route-rules-uuid-1234567890ab",
				Type:     "http",
				Method:   "get",
				Endpoint: "search",
				Enabled:  true,
				Responses: []MockoonResponse{
					{
						StatusCode: 200,
						Body:       `{"results": []}`,
						Rules: []MockoonRule{
							{Target: "query", Modifier: "q", Value: "test", Operator: "equals"},
							{Target: "header", Modifier: "Accept", Value: "application/json", Operator: "equals"},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	m := collection.Mocks[0]
	if m.HTTP.Matcher.QueryParams["q"] != "test" {
		t.Errorf("query param q = %q, want test", m.HTTP.Matcher.QueryParams["q"])
	}
	if m.HTTP.Matcher.Headers["Accept"] != "application/json" {
		t.Errorf("header Accept = %q, want application/json", m.HTTP.Matcher.Headers["Accept"])
	}
}

func TestMockoonImporter_CRUDRoute(t *testing.T) {
	env := MockoonEnvironment{
		Name: "CRUD test",
		Routes: []MockoonRoute{
			{
				UUID:          "route-crud-uuid-1234567890ab",
				Type:          "crud",
				Documentation: "products",
				Endpoint:      "products",
				DatabucketID:  "bucket-1",
				Enabled:       true,
				Responses:     []MockoonResponse{{StatusCode: 200}},
			},
		},
		Databuckets: []MockoonBucket{
			{
				UUID:  "bucket-1-uuid",
				ID:    "bucket-1",
				Name:  "Products",
				Value: `[{"id": "p1", "name": "Widget", "price": 9.99}]`,
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// CRUD routes become stateful resources, not mocks
	if len(collection.Mocks) != 0 {
		t.Errorf("expected 0 mocks for CRUD route, got %d", len(collection.Mocks))
	}
	if len(collection.StatefulResources) != 1 {
		t.Fatalf("expected 1 stateful resource, got %d", len(collection.StatefulResources))
	}

	res := collection.StatefulResources[0]
	if res.Name != "products" {
		t.Errorf("resource name = %q, want products", res.Name)
	}
	if res.BasePath != "/products" {
		t.Errorf("resource basePath = %q, want /products", res.BasePath)
	}
	if len(res.SeedData) != 1 {
		t.Errorf("expected 1 seed data item, got %d", len(res.SeedData))
	}
}

func TestMockoonImporter_DisabledRoutes(t *testing.T) {
	env := MockoonEnvironment{
		Name: "Disabled routes",
		Routes: []MockoonRoute{
			{
				UUID:      "route-disabled-uuid-1234567890",
				Type:      "http",
				Method:    "get",
				Endpoint:  "active",
				Enabled:   true,
				Responses: []MockoonResponse{{StatusCode: 200, Body: "{}"}},
			},
			{
				UUID:      "route-disabled-uuid-abcdef1234",
				Type:      "http",
				Method:    "get",
				Endpoint:  "disabled",
				Enabled:   false,
				Responses: []MockoonResponse{{StatusCode: 200, Body: "{}"}},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(collection.Mocks) != 1 {
		t.Fatalf("expected 1 mock (disabled route skipped), got %d", len(collection.Mocks))
	}
}

func TestConvertMockoonTemplates(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "faker firstName",
			input:    `{"name": "{{faker 'person.firstName'}}"}`,
			expected: `{"name": "{{ faker.firstName }}"}`,
		},
		{
			name:     "faker email",
			input:    `{"email": "{{faker 'internet.email'}}"}`,
			expected: `{"email": "{{ faker.email }}"}`,
		},
		{
			name:     "uuid",
			input:    `{"id": "{{uuid}}"}`,
			expected: `{"id": "{{ uuid }}"}`,
		},
		{
			name:     "urlParam",
			input:    `{{urlParam 'id'}}`,
			expected: `{{request.pathParam.id}}`,
		},
		{
			name:     "queryParam",
			input:    `{{queryParam 'page'}}`,
			expected: `{{request.query.page}}`,
		},
		{
			name:     "header",
			input:    `{{header 'Authorization'}}`,
			expected: `{{request.header.Authorization}}`,
		},
		{
			name:     "body",
			input:    `{{body 'user.name'}}`,
			expected: `{{request.body.user.name}}`,
		},
		{
			name:     "bodyRaw",
			input:    `{{bodyRaw}}`,
			expected: `{{request.rawBody}}`,
		},
		{
			name:     "faker number int",
			input:    `{{faker 'number.int'}}`,
			expected: `{{ random.int(1, 1000) }}`,
		},
		{
			name:     "faker company name",
			input:    `{{faker 'company.name'}}`,
			expected: `{{ faker.company }}`,
		},
		{
			name:     "faker ipv4",
			input:    `{{faker 'internet.ipv4'}}`,
			expected: `{{ faker.ipv4 }}`,
		},
		{
			name:     "faker string uuid",
			input:    `{{faker 'string.uuid'}}`,
			expected: `{{ uuid }}`,
		},
		{
			name:     "faker boolean",
			input:    `{{faker 'datatype.boolean'}}`,
			expected: `{{ faker.boolean }}`,
		},
		{
			name:     "unknown faker falls back to word",
			input:    `{{faker 'something.unknown'}}`,
			expected: `{{ faker.word }}`,
		},
		{
			name:     "mixed templates",
			input:    `{"id": "{{uuid}}", "name": "{{faker 'person.firstName'}} {{faker 'person.lastName'}}", "age": {{faker 'number.int'}}}`,
			expected: `{"id": "{{ uuid }}", "name": "{{ faker.firstName }} {{ faker.lastName }}", "age": {{ random.int(1, 1000) }}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMockoonTemplates(tt.input)
			if result != tt.expected {
				t.Errorf("convertMockoonTemplates(%q)\n  got:  %q\n  want: %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertPathParams(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users/:id", "/users/{id}"},
		{"/users/:userId/posts/:postId", "/users/{userId}/posts/{postId}"},
		{"/users", "/users"},
		{"/:a/:b/:c", "/{a}/{b}/{c}"},
	}

	for _, tt := range tests {
		result := convertPathParams(tt.input)
		if result != tt.expected {
			t.Errorf("convertPathParams(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMockoonFormatDetection(t *testing.T) {
	env := MockoonEnvironment{
		Name:           "Test",
		Port:           3001,
		EndpointPrefix: "api",
		Routes:         []MockoonRoute{{UUID: "r1", Type: "http", Method: "get", Endpoint: "test"}},
	}

	data, _ := json.Marshal(env)
	format := DetectFormat(data, "environment.json")

	if format != FormatMockoon {
		t.Errorf("DetectFormat = %q, want %q", format, FormatMockoon)
	}
}

func TestMockoonParseFormat(t *testing.T) {
	if ParseFormat("mockoon") != FormatMockoon {
		t.Errorf("ParseFormat('mockoon') should return FormatMockoon")
	}
}

func TestMockoonImporter_EmptyRoutes(t *testing.T) {
	env := MockoonEnvironment{
		Name:   "Empty",
		Routes: []MockoonRoute{},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	_, err := importer.Import(data)
	if err == nil {
		t.Error("expected error for empty routes")
	}
}

func TestMockoonImporter_BodyRegexRule(t *testing.T) {
	env := MockoonEnvironment{
		Name: "Body regex",
		Routes: []MockoonRoute{
			{
				UUID:     "route-regex-uuid-1234567890ab",
				Type:     "http",
				Method:   "post",
				Endpoint: "validate",
				Enabled:  true,
				Responses: []MockoonResponse{
					{
						StatusCode: 200,
						Body:       `{"valid": true}`,
						Rules: []MockoonRule{
							{Target: "body", Modifier: "", Value: `^{"email":".*@.*"}$`, Operator: "regex"},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	m := collection.Mocks[0]
	if m.HTTP.Matcher.BodyPattern != `^{"email":".*@.*"}$` {
		t.Errorf("bodyPattern = %q, want regex pattern", m.HTTP.Matcher.BodyPattern)
	}
}

func TestMockoonImporter_FileBody(t *testing.T) {
	env := MockoonEnvironment{
		Name: "File body",
		Routes: []MockoonRoute{
			{
				UUID:     "route-file-uuid-1234567890abcd",
				Type:     "http",
				Method:   "get",
				Endpoint: "download",
				Enabled:  true,
				Responses: []MockoonResponse{
					{
						StatusCode:     200,
						FilePath:       "./data/response.json",
						SendFileAsBody: true,
					},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	m := collection.Mocks[0]
	if m.HTTP.Response.BodyFile != "./data/response.json" {
		t.Errorf("bodyFile = %q, want ./data/response.json", m.HTTP.Response.BodyFile)
	}
}

func TestMockoonImporter_DisableTemplate(t *testing.T) {
	env := MockoonEnvironment{
		Name: "No template",
		Routes: []MockoonRoute{
			{
				UUID:     "route-notmpl-uuid-1234567890ab",
				Type:     "http",
				Method:   "get",
				Endpoint: "raw",
				Enabled:  true,
				Responses: []MockoonResponse{
					{
						StatusCode:      200,
						Body:            `{{faker 'person.firstName'}}`,
						DisableTemplate: true,
					},
				},
			},
		},
	}

	data, _ := json.Marshal(env)
	importer := &MockoonImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// When templating is disabled, body should be kept as-is (no conversion)
	m := collection.Mocks[0]
	if m.HTTP.Response.Body != `{{faker 'person.firstName'}}` {
		t.Errorf("body with disabled templating should be raw, got %q", m.HTTP.Response.Body)
	}
}
