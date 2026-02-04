package generator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getmockd/mockd/pkg/ai"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// mockProvider implements ai.Provider for testing
type mockProvider struct {
	generateFunc      func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error)
	generateBatchFunc func(ctx context.Context, reqs []*ai.GenerateRequest) ([]*ai.GenerateResponse, error)
}

func (m *mockProvider) Generate(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}
	return &ai.GenerateResponse{Value: "mock-value"}, nil
}

func (m *mockProvider) GenerateBatch(ctx context.Context, reqs []*ai.GenerateRequest) ([]*ai.GenerateResponse, error) {
	if m.generateBatchFunc != nil {
		return m.generateBatchFunc(ctx, reqs)
	}
	responses := make([]*ai.GenerateResponse, len(reqs))
	for i := range reqs {
		responses[i] = &ai.GenerateResponse{Value: "mock-value"}
	}
	return responses, nil
}

func (m *mockProvider) Name() string {
	return "mock"
}

func TestNew(t *testing.T) {
	provider := &mockProvider{}
	gen := New(provider)
	if gen == nil {
		t.Fatal("expected generator to be created")
		return
	}
	if gen.provider != provider {
		t.Error("expected provider to be set")
	}
}

func TestEnhanceOpenAPISchema(t *testing.T) {
	t.Run("uses existing example", func(t *testing.T) {
		provider := &mockProvider{}
		gen := New(provider)

		types := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type:    &types,
			Example: "existing@example.com",
		}

		result, err := gen.EnhanceOpenAPISchema(context.Background(), schema, "email")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "existing@example.com" {
			t.Errorf("expected existing example, got %v", result)
		}
	})

	t.Run("uses default value", func(t *testing.T) {
		provider := &mockProvider{}
		gen := New(provider)

		types := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type:    &types,
			Default: "default@example.com",
		}

		result, err := gen.EnhanceOpenAPISchema(context.Background(), schema, "email")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "default@example.com" {
			t.Errorf("expected default value, got %v", result)
		}
	})

	t.Run("uses first enum value", func(t *testing.T) {
		provider := &mockProvider{}
		gen := New(provider)

		types := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type: &types,
			Enum: []interface{}{"active", "inactive", "pending"},
		}

		result, err := gen.EnhanceOpenAPISchema(context.Background(), schema, "status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "active" {
			t.Errorf("expected first enum value, got %v", result)
		}
	})

	t.Run("calls AI provider when no example/default/enum", func(t *testing.T) {
		called := false
		provider := &mockProvider{
			generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
				called = true
				if req.FieldName != "email" {
					t.Errorf("expected fieldName=email, got %s", req.FieldName)
				}
				if req.FieldType != "string" {
					t.Errorf("expected fieldType=string, got %s", req.FieldType)
				}
				if req.Format != "email" {
					t.Errorf("expected format=email, got %s", req.Format)
				}
				return &ai.GenerateResponse{Value: "ai@example.com"}, nil
			},
		}
		gen := New(provider)

		types := openapi3.Types{"string"}
		schema := &openapi3.Schema{
			Type:   &types,
			Format: "email",
		}

		result, err := gen.EnhanceOpenAPISchema(context.Background(), schema, "email")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("expected AI provider to be called")
		}
		if result != "ai@example.com" {
			t.Errorf("expected AI result, got %v", result)
		}
	})

	t.Run("returns nil for nil schema", func(t *testing.T) {
		provider := &mockProvider{}
		gen := New(provider)

		_, err := gen.EnhanceOpenAPISchema(context.Background(), nil, "test")
		if err == nil {
			t.Error("expected error for nil schema")
		}
	})
}

func TestEnhanceOpenAPISchemaRecursive(t *testing.T) {
	t.Run("handles object schema", func(t *testing.T) {
		provider := &mockProvider{
			generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
				switch req.FieldName {
				case "name":
					return &ai.GenerateResponse{Value: "John Doe"}, nil
				case "age":
					return &ai.GenerateResponse{Value: int64(30)}, nil
				default:
					return &ai.GenerateResponse{Value: "unknown"}, nil
				}
			},
		}
		gen := New(provider)

		stringType := openapi3.Types{"string"}
		intType := openapi3.Types{"integer"}
		objectType := openapi3.Types{"object"}

		schema := &openapi3.Schema{
			Type: &objectType,
			Properties: openapi3.Schemas{
				"name": &openapi3.SchemaRef{
					Value: &openapi3.Schema{Type: &stringType},
				},
				"age": &openapi3.SchemaRef{
					Value: &openapi3.Schema{Type: &intType},
				},
			},
		}

		result, err := gen.EnhanceOpenAPISchemaRecursive(context.Background(), schema, "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		obj, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", result)
		}

		if obj["name"] != "John Doe" {
			t.Errorf("expected name=John Doe, got %v", obj["name"])
		}
		if obj["age"] != int64(30) {
			t.Errorf("expected age=30, got %v", obj["age"])
		}
	})

	t.Run("handles array schema", func(t *testing.T) {
		provider := &mockProvider{
			generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
				return &ai.GenerateResponse{Value: "item"}, nil
			},
		}
		gen := New(provider)

		stringType := openapi3.Types{"string"}
		arrayType := openapi3.Types{"array"}

		schema := &openapi3.Schema{
			Type: &arrayType,
			Items: &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &stringType},
			},
		}

		result, err := gen.EnhanceOpenAPISchemaRecursive(context.Background(), schema, "items")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array result, got %T", result)
		}

		if len(arr) < 1 {
			t.Error("expected at least one item in array")
		}
	})
}

func TestGenerateFromDescription(t *testing.T) {
	t.Run("generates mocks from description", func(t *testing.T) {
		provider := &mockProvider{
			generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
				// Return a valid JSON array of mock endpoints
				mocks := []map[string]interface{}{
					{
						"name":       "List Users",
						"method":     "GET",
						"path":       "/api/users",
						"statusCode": 200,
						"responseBody": map[string]interface{}{
							"users": []map[string]interface{}{
								{"id": 1, "name": "John"},
							},
						},
					},
					{
						"name":       "Create User",
						"method":     "POST",
						"path":       "/api/users",
						"statusCode": 201,
						"responseBody": map[string]interface{}{
							"id":   1,
							"name": "New User",
						},
					},
				}
				mocksJSON, _ := json.Marshal(mocks)
				return &ai.GenerateResponse{
					Value:       mocks,
					RawResponse: string(mocksJSON),
				}, nil
			},
		}
		gen := New(provider)

		mocks, err := gen.GenerateFromDescription(context.Background(), "user management API")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mocks) != 2 {
			t.Fatalf("expected 2 mocks, got %d", len(mocks))
		}

		// Check first mock
		if mocks[0].Name != "List Users" {
			t.Errorf("expected name=List Users, got %s", mocks[0].Name)
		}
		if mocks[0].HTTP == nil || mocks[0].HTTP.Matcher == nil {
			t.Fatal("expected HTTP.Matcher to be set")
		}
		if mocks[0].HTTP.Matcher.Method != "GET" {
			t.Errorf("expected method=GET, got %s", mocks[0].HTTP.Matcher.Method)
		}
		if mocks[0].HTTP.Matcher.Path != "/api/users" {
			t.Errorf("expected path=/api/users, got %s", mocks[0].HTTP.Matcher.Path)
		}
		if mocks[0].HTTP.Response == nil {
			t.Fatal("expected HTTP.Response to be set")
		}
		if mocks[0].HTTP.Response.StatusCode != 200 {
			t.Errorf("expected statusCode=200, got %d", mocks[0].HTTP.Response.StatusCode)
		}

		// Check second mock
		if mocks[1].HTTP == nil || mocks[1].HTTP.Matcher == nil {
			t.Fatal("expected HTTP.Matcher to be set on second mock")
		}
		if mocks[1].HTTP.Matcher.Method != "POST" {
			t.Errorf("expected method=POST, got %s", mocks[1].HTTP.Matcher.Method)
		}
		if mocks[1].HTTP.Response == nil {
			t.Fatal("expected HTTP.Response to be set on second mock")
		}
		if mocks[1].HTTP.Response.StatusCode != 201 {
			t.Errorf("expected statusCode=201, got %d", mocks[1].HTTP.Response.StatusCode)
		}
	})
}

func TestEnhanceMock(t *testing.T) {
	t.Run("skips mock with existing body", func(t *testing.T) {
		provider := &mockProvider{
			generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
				t.Error("should not be called for mock with existing body")
				return nil, nil
			},
		}
		gen := New(provider)

		mockCfg := &config.MockConfiguration{
			Name: "Test Mock",
			Type: mock.MockTypeHTTP,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/test",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"existing": "data"}`,
				},
			},
		}

		err := gen.EnhanceMock(context.Background(), mockCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Body should be unchanged
		if mockCfg.HTTP.Response.Body != `{"existing": "data"}` {
			t.Errorf("body should be unchanged, got %s", mockCfg.HTTP.Response.Body)
		}
	})

	t.Run("enhances mock with placeholder body", func(t *testing.T) {
		provider := &mockProvider{
			generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
				return &ai.GenerateResponse{
					Value: map[string]interface{}{
						"id":   1,
						"name": "Generated",
					},
				}, nil
			},
		}
		gen := New(provider)

		mockCfg := &config.MockConfiguration{
			Name: "Test Mock",
			Type: mock.MockTypeHTTP,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/test",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"status": "ok"}`,
				},
			},
		}

		err := gen.EnhanceMock(context.Background(), mockCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Body should be updated
		if mockCfg.HTTP.Response.Body == `{"status": "ok"}` {
			t.Error("body should be updated")
		}

		// Should contain generated data
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(mockCfg.HTTP.Response.Body), &body); err != nil {
			t.Fatalf("failed to parse body: %v", err)
		}
		if body["name"] != "Generated" {
			t.Errorf("expected name=Generated, got %v", body["name"])
		}
	})

	t.Run("handles nil mock", func(t *testing.T) {
		provider := &mockProvider{}
		gen := New(provider)

		err := gen.EnhanceMock(context.Background(), nil)
		if err != nil {
			t.Errorf("expected no error for nil mock, got %v", err)
		}
	})

	t.Run("handles mock without response", func(t *testing.T) {
		provider := &mockProvider{}
		gen := New(provider)

		mockCfg := &config.MockConfiguration{
			Name: "Test Mock",
			Type: mock.MockTypeHTTP,
			HTTP: &mock.HTTPSpec{
				Response: nil,
			},
		}

		err := gen.EnhanceMock(context.Background(), mockCfg)
		if err != nil {
			t.Errorf("expected no error for mock without response, got %v", err)
		}
	})
}

func TestFallbackValues(t *testing.T) {
	provider := &mockProvider{
		generateFunc: func(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
			return nil, ai.ErrGenerationFailed
		},
	}
	gen := New(provider)

	t.Run("string fallback", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		schema := &openapi3.Schema{Type: &stringType}
		result, _ := gen.EnhanceOpenAPISchema(context.Background(), schema, "test")
		if result != "string" {
			t.Errorf("expected fallback string, got %v", result)
		}
	})

	t.Run("email format fallback", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		schema := &openapi3.Schema{Type: &stringType, Format: "email"}
		result, _ := gen.EnhanceOpenAPISchema(context.Background(), schema, "email")
		if result != "user@example.com" {
			t.Errorf("expected email fallback, got %v", result)
		}
	})

	t.Run("uuid format fallback", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		schema := &openapi3.Schema{Type: &stringType, Format: "uuid"}
		result, _ := gen.EnhanceOpenAPISchema(context.Background(), schema, "id")
		if result == "" {
			t.Error("expected uuid fallback")
		}
	})

	t.Run("integer fallback", func(t *testing.T) {
		intType := openapi3.Types{"integer"}
		schema := &openapi3.Schema{Type: &intType}
		result, _ := gen.EnhanceOpenAPISchema(context.Background(), schema, "count")
		if result != 1 {
			t.Errorf("expected integer fallback 1, got %v", result)
		}
	})

	t.Run("boolean fallback", func(t *testing.T) {
		boolType := openapi3.Types{"boolean"}
		schema := &openapi3.Schema{Type: &boolType}
		result, _ := gen.EnhanceOpenAPISchema(context.Background(), schema, "active")
		if result != true {
			t.Errorf("expected boolean fallback true, got %v", result)
		}
	})
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no fences", `[{"name": "test"}]`, `[{"name": "test"}]`},
		{"json fence", "```json\n[{\"name\": \"test\"}]\n```", `[{"name": "test"}]`},
		{"plain fence", "```\n[{\"name\": \"test\"}]\n```", `[{"name": "test"}]`},
		{"fence with whitespace", "  ```json\n[{\"name\": \"test\"}]\n```  ", `[{"name": "test"}]`},
		{"JSON uppercase fence", "```JSON\n[{\"name\": \"test\"}]\n```", `[{"name": "test"}]`},
		{"no closing fence", "```json\n[{\"name\": \"test\"}]", `[{"name": "test"}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripCodeFences(tt.input)
			if result != tt.expected {
				t.Errorf("stripCodeFences() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizePathParams(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/users", "/api/users"},
		{"/api/users/:id", "/api/users/{id}"},
		{"/api/users/:userId/posts/:postId", "/api/users/{userId}/posts/{postId}"},
		{"/api/users/{id}", "/api/users/{id}"}, // already correct
		{"/:version/api", "/{version}/api"},
		{"/", "/"},
		{"/api/users/:id/comments", "/api/users/{id}/comments"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePathParams(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathParams(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
