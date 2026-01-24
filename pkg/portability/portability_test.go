package portability

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================================================
// Format Detection Tests
// ============================================================================

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		filename string
		expected Format
	}{
		// cURL detection
		{
			name:     "curl command",
			data:     `curl https://api.example.com/users`,
			filename: "",
			expected: FormatCURL,
		},
		{
			name:     "curl with tab separator",
			data:     "curl\thttps://api.example.com/users",
			filename: "",
			expected: FormatCURL,
		},
		{
			name:     "curl with leading whitespace",
			data:     "  curl https://api.example.com/users",
			filename: "",
			expected: FormatCURL,
		},

		// HAR detection by extension
		{
			name:     "HAR by extension",
			data:     `{"log": {"version": "1.2"}}`,
			filename: "traffic.har",
			expected: FormatHAR,
		},

		// OpenAPI 3.x detection
		{
			name:     "OpenAPI 3.x JSON",
			data:     `{"openapi": "3.0.3", "info": {"title": "Test"}}`,
			filename: "api.json",
			expected: FormatOpenAPI,
		},
		{
			name:     "OpenAPI 3.x YAML",
			data:     "openapi: '3.0.3'\ninfo:\n  title: Test",
			filename: "api.yaml",
			expected: FormatOpenAPI,
		},

		// Swagger 2.0 detection
		{
			name:     "Swagger 2.0 JSON",
			data:     `{"swagger": "2.0", "info": {"title": "Test"}}`,
			filename: "api.json",
			expected: FormatOpenAPI,
		},
		{
			name:     "Swagger 2.0 YAML",
			data:     "swagger: '2.0'\ninfo:\n  title: Test",
			filename: "api.yaml",
			expected: FormatOpenAPI,
		},

		// HAR detection by content
		{
			name:     "HAR by content",
			data:     `{"log": {"version": "1.2", "creator": {"name": "test"}}}`,
			filename: "file.json",
			expected: FormatHAR,
		},

		// Postman detection
		{
			name:     "Postman Collection",
			data:     `{"info": {"name": "Test", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"}, "item": []}`,
			filename: "collection.json",
			expected: FormatPostman,
		},

		// WireMock detection - single mapping
		{
			name:     "WireMock single mapping",
			data:     `{"request": {"method": "GET"}, "response": {"status": 200}}`,
			filename: "mapping.json",
			expected: FormatWireMock,
		},

		// WireMock detection - array of mappings
		{
			name:     "WireMock array",
			data:     `[{"request": {"method": "GET"}, "response": {"status": 200}}]`,
			filename: "mappings.json",
			expected: FormatWireMock,
		},

		// Mockd native format
		{
			name:     "Mockd with version and kind",
			data:     `{"version": "1.0", "kind": "MockCollection"}`,
			filename: "mocks.json",
			expected: FormatMockd,
		},
		{
			name:     "Mockd with version and mocks",
			data:     `{"version": "1.0", "mocks": []}`,
			filename: "mocks.json",
			expected: FormatMockd,
		},
		{
			name:     "Mockd YAML with mocks and http",
			data:     "mocks:\n  - http:\n      matcher:\n        path: /api",
			filename: "mocks.yaml",
			expected: FormatMockd,
		},

		// Unknown format
		{
			name:     "Unknown JSON",
			data:     `{"random": "data"}`,
			filename: "file.json",
			expected: FormatUnknown,
		},
		{
			name:     "Invalid JSON",
			data:     `not json at all`,
			filename: "file.json",
			expected: FormatUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat([]byte(tt.data), tt.filename)
			if result != tt.expected {
				t.Errorf("DetectFormat() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"mockd", FormatMockd},
		{"MOCKD", FormatMockd},
		{"native", FormatMockd},
		{"openapi", FormatOpenAPI},
		{"OpenAPI", FormatOpenAPI},
		{"swagger", FormatOpenAPI},
		{"oas", FormatOpenAPI},
		{"postman", FormatPostman},
		{"har", FormatHAR},
		{"HAR", FormatHAR},
		{"wiremock", FormatWireMock},
		{"WireMock", FormatWireMock},
		{"curl", FormatCURL},
		{"CURL", FormatCURL},
		{"unknown", FormatUnknown},
		{"", FormatUnknown},
		{"  mockd  ", FormatMockd}, // with whitespace
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormat_IsValid(t *testing.T) {
	validFormats := []Format{FormatMockd, FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL}
	invalidFormats := []Format{FormatUnknown, Format("invalid"), Format("")}

	for _, f := range validFormats {
		if !f.IsValid() {
			t.Errorf("Format %v should be valid", f)
		}
	}

	for _, f := range invalidFormats {
		if f.IsValid() {
			t.Errorf("Format %v should be invalid", f)
		}
	}
}

func TestFormat_CanImport(t *testing.T) {
	importable := []Format{FormatMockd, FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL}

	for _, f := range importable {
		if !f.CanImport() {
			t.Errorf("Format %v should be importable", f)
		}
	}

	if FormatUnknown.CanImport() {
		t.Error("FormatUnknown should not be importable")
	}
}

func TestFormat_CanExport(t *testing.T) {
	exportable := []Format{FormatMockd, FormatOpenAPI}
	notExportable := []Format{FormatPostman, FormatHAR, FormatWireMock, FormatCURL, FormatUnknown}

	for _, f := range exportable {
		if !f.CanExport() {
			t.Errorf("Format %v should be exportable", f)
		}
	}

	for _, f := range notExportable {
		if f.CanExport() {
			t.Errorf("Format %v should not be exportable", f)
		}
	}
}

// ============================================================================
// Registry Tests
// ============================================================================

func TestRegistry(t *testing.T) {
	t.Run("importers are registered on init", func(t *testing.T) {
		// These should be registered via init()
		formats := []Format{FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL}
		for _, f := range formats {
			imp := GetImporter(f)
			if imp == nil {
				t.Errorf("Expected importer for %v to be registered", f)
			}
		}
	})

	t.Run("exporters are registered on init", func(t *testing.T) {
		exp := GetExporter(FormatOpenAPI)
		if exp == nil {
			t.Error("Expected OpenAPI exporter to be registered")
		}
	})

	t.Run("ListImporters returns all importers", func(t *testing.T) {
		importers := ListImporters()
		if len(importers) < 5 {
			t.Errorf("Expected at least 5 importers, got %d", len(importers))
		}
	})

	t.Run("ListExporters returns all exporters", func(t *testing.T) {
		exporters := ListExporters()
		if len(exporters) < 1 {
			t.Errorf("Expected at least 1 exporter, got %d", len(exporters))
		}
	})

	t.Run("nil importer is not registered", func(t *testing.T) {
		r := &Registry{
			importers: make(map[Format]Importer),
			exporters: make(map[Format]Exporter),
		}
		r.RegisterImporter(nil)
		if len(r.importers) != 0 {
			t.Error("nil importer should not be registered")
		}
	})

	t.Run("nil exporter is not registered", func(t *testing.T) {
		r := &Registry{
			importers: make(map[Format]Importer),
			exporters: make(map[Format]Exporter),
		}
		r.RegisterExporter(nil)
		if len(r.exporters) != 0 {
			t.Error("nil exporter should not be registered")
		}
	})
}

// ============================================================================
// OpenAPI Importer Tests
// ============================================================================

func TestOpenAPIImporter_Import(t *testing.T) {
	importer := &OpenAPIImporter{}

	t.Run("imports OpenAPI 3.x specification", func(t *testing.T) {
		spec := `{
			"openapi": "3.0.3",
			"info": {"title": "Test API", "version": "1.0.0"},
			"paths": {
				"/users": {
					"get": {
						"summary": "List users",
						"responses": {
							"200": {
								"description": "Success",
								"content": {
									"application/json": {
										"example": [{"id": 1, "name": "John"}]
									}
								}
							}
						}
					},
					"post": {
						"summary": "Create user",
						"responses": {
							"201": {"description": "Created"}
						}
					}
				},
				"/users/{id}": {
					"get": {
						"summary": "Get user by ID",
						"responses": {
							"200": {"description": "Success"}
						}
					}
				}
			}
		}`

		collection, err := importer.Import([]byte(spec))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if collection.Name != "Test API" {
			t.Errorf("Expected collection name 'Test API', got %q", collection.Name)
		}

		if len(collection.Mocks) != 3 {
			t.Errorf("Expected 3 mocks, got %d", len(collection.Mocks))
		}

		// Check path parameter conversion
		var foundUserByID bool
		for _, m := range collection.Mocks {
			if m.HTTP.Matcher.Path == "/users/:id" {
				foundUserByID = true
			}
		}
		if !foundUserByID {
			t.Error("Expected path parameter {id} to be converted to :id")
		}
	})

	t.Run("imports Swagger 2.0 specification", func(t *testing.T) {
		spec := `{
			"swagger": "2.0",
			"info": {"title": "Legacy API", "version": "1.0.0"},
			"basePath": "/api/v1",
			"paths": {
				"/items": {
					"get": {
						"summary": "List items",
						"produces": ["application/json"],
						"responses": {
							"200": {"description": "Success"}
						}
					}
				}
			}
		}`

		collection, err := importer.Import([]byte(spec))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if collection.Name != "Legacy API" {
			t.Errorf("Expected collection name 'Legacy API', got %q", collection.Name)
		}

		if len(collection.Mocks) != 1 {
			t.Errorf("Expected 1 mock, got %d", len(collection.Mocks))
		}

		// Check basePath is prepended
		if collection.Mocks[0].HTTP.Matcher.Path != "/api/v1/items" {
			t.Errorf("Expected path '/api/v1/items', got %q", collection.Mocks[0].HTTP.Matcher.Path)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		_, err := importer.Import([]byte(`{invalid json`))
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("returns error for non-OpenAPI document", func(t *testing.T) {
		_, err := importer.Import([]byte(`{"random": "data"}`))
		if err == nil {
			t.Error("Expected error for non-OpenAPI document")
		}
	})

	t.Run("handles empty paths", func(t *testing.T) {
		spec := `{
			"openapi": "3.0.3",
			"info": {"title": "Empty API", "version": "1.0.0"},
			"paths": {}
		}`

		collection, err := importer.Import([]byte(spec))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(collection.Mocks) != 0 {
			t.Errorf("Expected 0 mocks for empty paths, got %d", len(collection.Mocks))
		}
	})

	t.Run("extracts query parameters from examples", func(t *testing.T) {
		spec := `{
			"openapi": "3.0.3",
			"info": {"title": "Test", "version": "1.0.0"},
			"paths": {
				"/search": {
					"get": {
						"parameters": [
							{"name": "q", "in": "query", "example": "test"}
						],
						"responses": {"200": {"description": "OK"}}
					}
				}
			}
		}`

		collection, err := importer.Import([]byte(spec))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(collection.Mocks) != 1 {
			t.Fatalf("Expected 1 mock, got %d", len(collection.Mocks))
		}

		qp := collection.Mocks[0].HTTP.Matcher.QueryParams
		if qp == nil || qp["q"] != "test" {
			t.Errorf("Expected query param q=test, got %v", qp)
		}
	})
}

// ============================================================================
// OpenAPI Exporter Tests
// ============================================================================

func TestOpenAPIExporter_Export(t *testing.T) {
	t.Run("exports nil collection returns error", func(t *testing.T) {
		exporter := &OpenAPIExporter{}
		_, err := exporter.Export(nil)
		if err == nil {
			t.Error("Expected error for nil collection")
		}
	})

	t.Run("round-trip import-export preserves structure", func(t *testing.T) {
		importer := &OpenAPIImporter{}
		exporter := &OpenAPIExporter{AsYAML: false}

		original := `{
			"openapi": "3.0.3",
			"info": {"title": "Test API", "version": "1.0.0"},
			"paths": {
				"/users": {
					"get": {
						"summary": "List users",
						"responses": {"200": {"description": "Success"}}
					}
				}
			}
		}`

		// Import
		collection, err := importer.Import([]byte(original))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		// Export
		exported, err := exporter.Export(collection)
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		// Parse the exported JSON
		var spec OpenAPI
		if err := json.Unmarshal(exported, &spec); err != nil {
			t.Fatalf("Failed to parse exported JSON: %v", err)
		}

		if spec.OpenAPI != "3.0.3" {
			t.Errorf("Expected openapi version 3.0.3, got %v", spec.OpenAPI)
		}

		if spec.Info.Title != "Test API" {
			t.Errorf("Expected title 'Test API', got %v", spec.Info.Title)
		}

		if _, ok := spec.Paths["/users"]; !ok {
			t.Error("Expected /users path in exported spec")
		}
	})

	t.Run("exports as YAML when AsYAML is true", func(t *testing.T) {
		importer := &OpenAPIImporter{}
		exporter := &OpenAPIExporter{AsYAML: true}

		original := `{
			"openapi": "3.0.3",
			"info": {"title": "Test", "version": "1.0.0"},
			"paths": {"/test": {"get": {"responses": {"200": {"description": "OK"}}}}}
		}`

		collection, _ := importer.Import([]byte(original))
		exported, err := exporter.Export(collection)
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		// YAML should not start with {
		if strings.HasPrefix(string(exported), "{") {
			t.Error("Expected YAML output, got JSON")
		}

		// Should contain YAML-style content
		if !strings.Contains(string(exported), "openapi:") {
			t.Error("Expected YAML-style 'openapi:' field")
		}
	})
}

// ============================================================================
// Postman Importer Tests
// ============================================================================

func TestPostmanImporter_Import(t *testing.T) {
	importer := &PostmanImporter{}

	t.Run("imports valid Postman collection", func(t *testing.T) {
		collection := `{
			"info": {
				"name": "Test Collection",
				"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
			},
			"item": [
				{
					"name": "Get Users",
					"request": {
						"method": "GET",
						"url": {
							"raw": "https://api.example.com/users",
							"path": ["users"]
						}
					},
					"response": [
						{
							"name": "Success",
							"code": 200,
							"body": "{\"users\": []}"
						}
					]
				}
			]
		}`

		result, err := importer.Import([]byte(collection))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if result.Name != "Test Collection" {
			t.Errorf("Expected name 'Test Collection', got %q", result.Name)
		}

		if len(result.Mocks) != 1 {
			t.Errorf("Expected 1 mock, got %d", len(result.Mocks))
		}
	})

	t.Run("handles nested folders", func(t *testing.T) {
		collection := `{
			"info": {
				"name": "Nested",
				"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
			},
			"item": [
				{
					"name": "Folder",
					"item": [
						{
							"name": "Request 1",
							"request": {
								"method": "GET",
								"url": {"path": ["api", "v1"]}
							}
						}
					]
				}
			]
		}`

		result, err := importer.Import([]byte(collection))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 1 {
			t.Errorf("Expected 1 mock from nested folder, got %d", len(result.Mocks))
		}
	})

	t.Run("substitutes variables", func(t *testing.T) {
		collection := `{
			"info": {
				"name": "Variables",
				"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
			},
			"variable": [
				{"key": "baseUrl", "value": "api.example.com"}
			],
			"item": [
				{
					"name": "Test",
					"request": {
						"method": "GET",
						"url": {
							"raw": "https://{{baseUrl}}/users",
							"path": ["users"]
						}
					}
				}
			]
		}`

		result, err := importer.Import([]byte(collection))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 1 {
			t.Fatalf("Expected 1 mock, got %d", len(result.Mocks))
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		_, err := importer.Import([]byte(`{invalid`))
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("returns error for non-Postman JSON", func(t *testing.T) {
		_, err := importer.Import([]byte(`{"info": {"name": "test"}}`))
		if err == nil {
			t.Error("Expected error for non-Postman JSON")
		}
	})
}

// ============================================================================
// HAR Importer Tests
// ============================================================================

func TestHARImporter_Import(t *testing.T) {
	importer := &HARImporter{}

	t.Run("imports valid HAR file", func(t *testing.T) {
		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [
					{
						"request": {
							"method": "GET",
							"url": "https://api.example.com/users?page=1",
							"headers": [],
							"queryString": [{"name": "page", "value": "1"}]
						},
						"response": {
							"status": 200,
							"statusText": "OK",
							"headers": [{"name": "Content-Type", "value": "application/json"}],
							"content": {
								"mimeType": "application/json",
								"text": "{\"users\": []}"
							}
						}
					}
				]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 1 {
			t.Errorf("Expected 1 mock, got %d", len(result.Mocks))
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Path != "/users" {
			t.Errorf("Expected path '/users', got %q", mock.HTTP.Matcher.Path)
		}

		if mock.HTTP.Response.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", mock.HTTP.Response.StatusCode)
		}
	})

	t.Run("filters static assets by default", func(t *testing.T) {
		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [
					{
						"request": {"method": "GET", "url": "https://example.com/api/data", "headers": []},
						"response": {"status": 200, "headers": [], "content": {"mimeType": "application/json"}}
					},
					{
						"request": {"method": "GET", "url": "https://example.com/script.js", "headers": []},
						"response": {"status": 200, "headers": [], "content": {"mimeType": "application/javascript"}}
					},
					{
						"request": {"method": "GET", "url": "https://example.com/style.css", "headers": []},
						"response": {"status": 200, "headers": [], "content": {"mimeType": "text/css"}}
					}
				]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		// Only the API endpoint should be imported
		if len(result.Mocks) != 1 {
			t.Errorf("Expected 1 mock (static assets filtered), got %d", len(result.Mocks))
		}
	})

	t.Run("includes static assets when configured", func(t *testing.T) {
		importer := &HARImporter{IncludeStatic: true}

		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [
					{
						"request": {"method": "GET", "url": "https://example.com/api/data", "headers": []},
						"response": {"status": 200, "headers": [], "content": {"mimeType": "application/json"}}
					},
					{
						"request": {"method": "GET", "url": "https://example.com/script.js", "headers": []},
						"response": {"status": 200, "headers": [], "content": {"mimeType": "application/javascript"}}
					}
				]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 2 {
			t.Errorf("Expected 2 mocks (including static), got %d", len(result.Mocks))
		}
	})

	t.Run("returns error for invalid HAR", func(t *testing.T) {
		_, err := importer.Import([]byte(`{"log": {}}`))
		if err == nil {
			t.Error("Expected error for invalid HAR")
		}
	})
}

// ============================================================================
// WireMock Importer Tests
// ============================================================================

func TestWireMockImporter_Import(t *testing.T) {
	importer := &WireMockImporter{}

	t.Run("imports single mapping", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "GET",
				"urlPath": "/api/users"
			},
			"response": {
				"status": 200,
				"body": "{\"users\": []}",
				"headers": {
					"Content-Type": "application/json"
				}
			}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 1 {
			t.Errorf("Expected 1 mock, got %d", len(result.Mocks))
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Path != "/api/users" {
			t.Errorf("Expected path '/api/users', got %q", mock.HTTP.Matcher.Path)
		}
	})

	t.Run("imports array of mappings", func(t *testing.T) {
		mappings := `[
			{"request": {"method": "GET", "urlPath": "/one"}, "response": {"status": 200}},
			{"request": {"method": "POST", "urlPath": "/two"}, "response": {"status": 201}}
		]`

		result, err := importer.Import([]byte(mappings))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 2 {
			t.Errorf("Expected 2 mocks, got %d", len(result.Mocks))
		}
	})

	t.Run("imports mappings wrapper format", func(t *testing.T) {
		wrapper := `{
			"mappings": [
				{"request": {"method": "GET", "url": "/test"}, "response": {"status": 200}}
			]
		}`

		result, err := importer.Import([]byte(wrapper))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 1 {
			t.Errorf("Expected 1 mock, got %d", len(result.Mocks))
		}
	})

	t.Run("handles URL patterns", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "GET",
				"urlPattern": "/api/users/[0-9]+"
			},
			"response": {"status": 200}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.PathPattern != "/api/users/[0-9]+" {
			t.Errorf("Expected path pattern, got path=%q pattern=%q",
				mock.HTTP.Matcher.Path, mock.HTTP.Matcher.PathPattern)
		}
	})

	t.Run("handles jsonBody response", func(t *testing.T) {
		mapping := `{
			"request": {"method": "GET", "urlPath": "/json"},
			"response": {
				"status": 200,
				"jsonBody": {"key": "value", "number": 42}
			}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if !strings.Contains(mock.HTTP.Response.Body, "key") {
			t.Error("Expected JSON body to be serialized")
		}

		if mock.HTTP.Response.Headers["Content-Type"] != "application/json" {
			t.Error("Expected Content-Type header for jsonBody")
		}
	})

	t.Run("handles delay", func(t *testing.T) {
		mapping := `{
			"request": {"method": "GET", "urlPath": "/slow"},
			"response": {
				"status": 200,
				"fixedDelayMilliseconds": 500
			}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Response.DelayMs != 500 {
			t.Errorf("Expected delay 500ms, got %d", mock.HTTP.Response.DelayMs)
		}
	})

	t.Run("returns error for empty mappings", func(t *testing.T) {
		_, err := importer.Import([]byte(`[]`))
		if err == nil {
			t.Error("Expected error for empty mappings")
		}
	})
}

// ============================================================================
// cURL Importer Tests
// ============================================================================

func TestCURLImporter_Import(t *testing.T) {
	importer := &CURLImporter{}

	t.Run("imports simple GET", func(t *testing.T) {
		cmd := `curl https://api.example.com/users`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Method != "GET" {
			t.Errorf("Expected GET method, got %q", mock.HTTP.Matcher.Method)
		}
		if mock.HTTP.Matcher.Path != "/users" {
			t.Errorf("Expected path '/users', got %q", mock.HTTP.Matcher.Path)
		}
	})

	t.Run("imports POST with data", func(t *testing.T) {
		cmd := `curl -X POST -d '{"name":"test"}' https://api.example.com/users`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Method != "POST" {
			t.Errorf("Expected POST method, got %q", mock.HTTP.Matcher.Method)
		}
	})

	t.Run("imports with headers", func(t *testing.T) {
		cmd := `curl -H "Authorization: Bearer token123" https://api.example.com/protected`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Headers == nil {
			t.Fatal("Expected headers to be set")
		}
		if mock.HTTP.Matcher.Headers["Authorization"] != "Bearer token123" {
			t.Errorf("Expected Authorization header, got %v", mock.HTTP.Matcher.Headers)
		}
	})

	t.Run("handles basic auth", func(t *testing.T) {
		cmd := `curl -u user:pass https://api.example.com/auth`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Headers == nil {
			t.Fatal("Expected headers for basic auth")
		}
		auth := mock.HTTP.Matcher.Headers["Authorization"]
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Expected Basic auth header, got %q", auth)
		}
	})

	t.Run("handles --json flag", func(t *testing.T) {
		cmd := `curl --json '{"key":"value"}' https://api.example.com/data`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Method != "POST" {
			t.Errorf("Expected POST for --json, got %q", mock.HTTP.Matcher.Method)
		}
	})

	t.Run("handles query parameters", func(t *testing.T) {
		cmd := `curl "https://api.example.com/search?q=test&page=1"`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.QueryParams == nil {
			t.Fatal("Expected query params")
		}
		if mock.HTTP.Matcher.QueryParams["q"] != "test" {
			t.Errorf("Expected q=test, got %v", mock.HTTP.Matcher.QueryParams)
		}
	})

	t.Run("returns error for non-curl command", func(t *testing.T) {
		_, err := importer.Import([]byte(`wget https://example.com`))
		if err == nil {
			t.Error("Expected error for non-curl command")
		}
	})

	t.Run("returns error for missing URL", func(t *testing.T) {
		_, err := importer.Import([]byte(`curl -X GET`))
		if err == nil {
			t.Error("Expected error for missing URL")
		}
	})

	t.Run("handles quoted arguments", func(t *testing.T) {
		cmd := `curl -H "Content-Type: application/json" -d '{"name": "test"}' https://api.example.com/users`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		// Data flag should switch to POST
		if mock.HTTP.Matcher.Method != "POST" {
			t.Errorf("Expected POST, got %q", mock.HTTP.Matcher.Method)
		}
	})

	t.Run("handles escaped characters", func(t *testing.T) {
		cmd := `curl https://api.example.com/path\ with\ spaces`

		// This should still work, handling escapes
		_, err := importer.Import([]byte(cmd))
		// Just checking it doesn't panic
		if err != nil {
			// Some errors are acceptable for edge cases
			t.Logf("Got error (may be acceptable): %v", err)
		}
	})
}

// ============================================================================
// Import Error Tests
// ============================================================================

func TestImportError(t *testing.T) {
	t.Run("error message includes format", func(t *testing.T) {
		err := &ImportError{
			Format:  FormatOpenAPI,
			Message: "test error",
		}
		if !strings.Contains(err.Error(), "openapi") {
			t.Errorf("Error message should include format: %v", err.Error())
		}
	})

	t.Run("error message includes line/column", func(t *testing.T) {
		err := &ImportError{
			Format:  FormatOpenAPI,
			Line:    10,
			Column:  5,
			Message: "syntax error",
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "line 10") {
			t.Errorf("Error should include line number: %v", errStr)
		}
		if !strings.Contains(errStr, "column 5") {
			t.Errorf("Error should include column number: %v", errStr)
		}
	})

	t.Run("error unwraps cause", func(t *testing.T) {
		cause := &ImportError{Message: "inner"}
		err := &ImportError{
			Message: "outer",
			Cause:   cause,
		}
		if err.Unwrap() != cause {
			t.Error("Unwrap should return cause")
		}
	})
}

// ============================================================================
// High-level Import Function Tests
// ============================================================================

func TestImport(t *testing.T) {
	t.Run("auto-detects OpenAPI format", func(t *testing.T) {
		spec := `{"openapi": "3.0.3", "info": {"title": "Test", "version": "1.0.0"}, "paths": {}}`

		result, err := Import([]byte(spec), "api.json", nil)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if result.Collection.Name != "Test" {
			t.Errorf("Expected collection name 'Test', got %q", result.Collection.Name)
		}
	})

	t.Run("applies name from options", func(t *testing.T) {
		spec := `{"openapi": "3.0.3", "info": {"title": "Original", "version": "1.0.0"}, "paths": {}}`
		opts := &ImportOptions{Name: "Overridden"}

		result, err := Import([]byte(spec), "api.json", opts)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if result.Collection.Name != "Overridden" {
			t.Errorf("Expected name 'Overridden', got %q", result.Collection.Name)
		}
	})

	t.Run("returns error for unknown format", func(t *testing.T) {
		_, err := Import([]byte(`random data`), "file.txt", nil)
		if err == nil {
			t.Error("Expected error for unknown format")
		}
	})
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestConvertOpenAPIPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users/{id}", "/users/:id"},
		{"/users/{userId}/posts/{postId}", "/users/:userId/posts/:postId"},
		{"/users", "/users"},
		{"/{version}/api", "/:version/api"},
	}

	for _, tt := range tests {
		result := convertOpenAPIPath(tt.input)
		if result != tt.expected {
			t.Errorf("convertOpenAPIPath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestConvertMockdPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users/:id", "/users/{id}"},
		{"/users/:userId/posts/:postId", "/users/{userId}/posts/{postId}"},
		{"/users", "/users"},
		{"/:version/api", "/{version}/api"},
	}

	for _, tt := range tests {
		result := convertMockdPath(tt.input)
		if result != tt.expected {
			t.Errorf("convertMockdPath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseStatusCode(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"200", 200},
		{"201", 201},
		{"404", 404},
		{"500", 500},
		{"default", 200},
		{"2XX", 200},
		{"", 200},
		{"999", 200}, // Out of valid range
		{"50", 200},  // Too low
	}

	for _, tt := range tests {
		result := parseStatusCode(tt.input)
		if result != tt.expected {
			t.Errorf("parseStatusCode(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestExtractWireMockVariables(t *testing.T) {
	tests := []struct {
		input    string
		expected int // number of variables
	}{
		{"Hello {{name}}", 1},
		{"{{first}} and {{second}}", 2},
		{"No variables", 0},
		{"{{incomplete", 0},
		{"", 0},
	}

	for _, tt := range tests {
		result := ExtractWireMockVariables(tt.input)
		if len(result) != tt.expected {
			t.Errorf("ExtractWireMockVariables(%q) found %d vars, expected %d",
				tt.input, len(result), tt.expected)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-1, "-1"},
		{-42, "-42"},
		{123456789, "123456789"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateDefaultBody(t *testing.T) {
	tests := []struct {
		status      int
		shouldMatch string
	}{
		{200, "ok"},
		{201, "created"},
		{204, ""},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{418, "418"}, // Unknown status
	}

	for _, tt := range tests {
		result := generateDefaultBody(tt.status)
		if tt.shouldMatch != "" && !strings.Contains(result, tt.shouldMatch) {
			t.Errorf("generateDefaultBody(%d) = %q, should contain %q",
				tt.status, result, tt.shouldMatch)
		}
		if tt.shouldMatch == "" && result != "" {
			t.Errorf("generateDefaultBody(%d) = %q, expected empty", tt.status, result)
		}
	}
}
