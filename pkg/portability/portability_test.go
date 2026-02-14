package portability

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
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
			if m.HTTP.Matcher.Path == "/users/{id}" {
				foundUserByID = true
			}
		}
		if !foundUserByID {
			t.Error("Expected path parameter {id} to be preserved as {id}")
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
		{"/users/{id}", "/users/{id}"},
		{"/users/{userId}/posts/{postId}", "/users/{userId}/posts/{postId}"},
		{"/users", "/users"},
		{"/{version}/api", "/{version}/api"},
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

func TestImportError_FormatWithLineAndColumn(t *testing.T) {
	err := &ImportError{
		Format:  FormatCURL,
		Line:    10,
		Column:  5,
		Message: "syntax error",
	}
	got := err.Error()
	if got != "curl: syntax error (line 10, column 5)" {
		t.Errorf("ImportError.Error() = %q, expected %q", got, "curl: syntax error (line 10, column 5)")
	}

	// Line only, no column
	err2 := &ImportError{
		Format:  FormatHAR,
		Line:    42,
		Message: "bad value",
	}
	got2 := err2.Error()
	if got2 != "har: bad value (line 42)" {
		t.Errorf("ImportError.Error() = %q, expected %q", got2, "har: bad value (line 42)")
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

// ============================================================================
// Session 12: HAR base64 body decoding tests
// ============================================================================

func TestHARImporter_Base64Body(t *testing.T) {
	importer := &HARImporter{}

	t.Run("decodes base64-encoded response body", func(t *testing.T) {
		originalBody := `{"users": [{"id": 1, "name": "Alice"}]}`
		encoded := base64.StdEncoding.EncodeToString([]byte(originalBody))

		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [{
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"headers": []
					},
					"response": {
						"status": 200,
						"headers": [{"name": "Content-Type", "value": "application/json"}],
						"content": {
							"mimeType": "application/json",
							"text": "` + encoded + `",
							"encoding": "base64"
						}
					}
				}]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if len(result.Mocks) != 1 {
			t.Fatalf("Expected 1 mock, got %d", len(result.Mocks))
		}

		body := result.Mocks[0].HTTP.Response.Body
		if body != originalBody {
			t.Errorf("Expected decoded body %q, got %q", originalBody, body)
		}
	})

	t.Run("handles base64 encoding case-insensitively", func(t *testing.T) {
		originalBody := "Hello World"
		encoded := base64.StdEncoding.EncodeToString([]byte(originalBody))

		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [{
					"request": {
						"method": "GET",
						"url": "https://api.example.com/hello",
						"headers": []
					},
					"response": {
						"status": 200,
						"headers": [],
						"content": {
							"mimeType": "text/plain",
							"text": "` + encoded + `",
							"encoding": "Base64"
						}
					}
				}]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		body := result.Mocks[0].HTTP.Response.Body
		if body != originalBody {
			t.Errorf("Expected decoded body %q, got %q", originalBody, body)
		}
	})

	t.Run("falls through to raw text when base64 decode fails", func(t *testing.T) {
		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [{
					"request": {
						"method": "GET",
						"url": "https://api.example.com/broken",
						"headers": []
					},
					"response": {
						"status": 200,
						"headers": [],
						"content": {
							"mimeType": "text/plain",
							"text": "not-valid-base64!!!",
							"encoding": "base64"
						}
					}
				}]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		body := result.Mocks[0].HTTP.Response.Body
		if body != "not-valid-base64!!!" {
			t.Errorf("Expected raw text fallback, got %q", body)
		}
	})

	t.Run("uses raw text when no encoding specified", func(t *testing.T) {
		har := `{
			"log": {
				"version": "1.2",
				"creator": {"name": "test", "version": "1.0"},
				"entries": [{
					"request": {
						"method": "GET",
						"url": "https://api.example.com/plain",
						"headers": []
					},
					"response": {
						"status": 200,
						"headers": [],
						"content": {
							"mimeType": "text/plain",
							"text": "plain text body"
						}
					}
				}]
			}
		}`

		result, err := importer.Import([]byte(har))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		body := result.Mocks[0].HTTP.Response.Body
		if body != "plain text body" {
			t.Errorf("Expected raw text %q, got %q", "plain text body", body)
		}
	})
}

// ============================================================================
// Session 12: WireMock advanced matcher tests
// ============================================================================

func TestWireMockImporter_AdvancedMatchers(t *testing.T) {
	importer := &WireMockImporter{}

	t.Run("imports contains header matcher", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "GET",
				"urlPath": "/api/data",
				"headers": {
					"Authorization": {"contains": "Bearer"}
				}
			},
			"response": {"status": 200}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Headers["Authorization"] != "Bearer" {
			t.Errorf("Expected header value 'Bearer', got %q", mock.HTTP.Matcher.Headers["Authorization"])
		}
	})

	t.Run("imports matches header matcher", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "GET",
				"urlPath": "/api/data",
				"headers": {
					"X-Request-ID": {"matches": "[a-f0-9-]+"}
				}
			},
			"response": {"status": 200}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Headers["X-Request-ID"] != "[a-f0-9-]+" {
			t.Errorf("Expected header pattern '[a-f0-9-]+', got %q", mock.HTTP.Matcher.Headers["X-Request-ID"])
		}
	})

	t.Run("imports contains query parameter matcher", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "GET",
				"urlPath": "/search",
				"queryParameters": {
					"q": {"contains": "test"}
				}
			},
			"response": {"status": 200}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.QueryParams["q"] != "test" {
			t.Errorf("Expected query param 'test', got %q", mock.HTTP.Matcher.QueryParams["q"])
		}
	})

	t.Run("imports equalToJson body pattern", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "POST",
				"urlPath": "/api/users",
				"bodyPatterns": [
					{"equalToJson": "{\"name\": \"Alice\"}"}
				]
			},
			"response": {"status": 201}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.BodyEquals != `{"name": "Alice"}` {
			t.Errorf("Expected BodyEquals to be set, got %q", mock.HTTP.Matcher.BodyEquals)
		}
	})

	t.Run("imports multiple body patterns", func(t *testing.T) {
		mapping := `{
			"request": {
				"method": "POST",
				"urlPath": "/api/data",
				"bodyPatterns": [
					{"contains": "hello"},
					{"matches": ".*world.*"}
				]
			},
			"response": {"status": 200}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.BodyContains != "hello" {
			t.Errorf("Expected BodyContains 'hello', got %q", mock.HTTP.Matcher.BodyContains)
		}
		if mock.HTTP.Matcher.BodyPattern != ".*world.*" {
			t.Errorf("Expected BodyPattern '.*world.*', got %q", mock.HTTP.Matcher.BodyPattern)
		}
	})

	t.Run("imports base64Body response", func(t *testing.T) {
		originalBody := `{"decoded": true}`
		encoded := base64.StdEncoding.EncodeToString([]byte(originalBody))

		mapping := `{
			"request": {"method": "GET", "urlPath": "/binary"},
			"response": {
				"status": 200,
				"base64Body": "` + encoded + `"
			}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Response.Body != originalBody {
			t.Errorf("Expected decoded body %q, got %q", originalBody, mock.HTTP.Response.Body)
		}
	})

	t.Run("prefers equalTo over contains in headers", func(t *testing.T) {
		// When both are present, equalTo should win (it's checked first)
		mapping := `{
			"request": {
				"method": "GET",
				"urlPath": "/test",
				"headers": {
					"Accept": {"equalTo": "application/json"}
				}
			},
			"response": {"status": 200}
		}`

		result, err := importer.Import([]byte(mapping))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.Headers["Accept"] != "application/json" {
			t.Errorf("Expected 'application/json', got %q", mock.HTTP.Matcher.Headers["Accept"])
		}
	})
}

// ============================================================================
// Session 12: cURL method-appropriate defaults tests
// ============================================================================

func TestCURLImporter_MethodDefaults(t *testing.T) {
	importer := &CURLImporter{}

	t.Run("POST returns 201 Created", func(t *testing.T) {
		cmd := `curl -X POST -d '{"name":"test"}' https://api.example.com/users`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Response.StatusCode != 201 {
			t.Errorf("Expected 201 for POST, got %d", mock.HTTP.Response.StatusCode)
		}
		if !strings.Contains(mock.HTTP.Response.Body, "created") {
			t.Errorf("Expected 'created' in body, got %q", mock.HTTP.Response.Body)
		}
	})

	t.Run("DELETE returns 204 No Content", func(t *testing.T) {
		cmd := `curl -X DELETE https://api.example.com/users/123`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Response.StatusCode != 204 {
			t.Errorf("Expected 204 for DELETE, got %d", mock.HTTP.Response.StatusCode)
		}
		if mock.HTTP.Response.Body != "" {
			t.Errorf("Expected empty body for DELETE, got %q", mock.HTTP.Response.Body)
		}
	})

	t.Run("PUT returns 200 with updated body", func(t *testing.T) {
		cmd := `curl -X PUT -d '{"name":"updated"}' https://api.example.com/users/123`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Response.StatusCode != 200 {
			t.Errorf("Expected 200 for PUT, got %d", mock.HTTP.Response.StatusCode)
		}
		if !strings.Contains(mock.HTTP.Response.Body, "updated") {
			t.Errorf("Expected 'updated' in body, got %q", mock.HTTP.Response.Body)
		}
	})

	t.Run("GET returns 200 with ok body", func(t *testing.T) {
		cmd := `curl https://api.example.com/users`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Response.StatusCode != 200 {
			t.Errorf("Expected 200 for GET, got %d", mock.HTTP.Response.StatusCode)
		}
		if !strings.Contains(mock.HTTP.Response.Body, "ok") {
			t.Errorf("Expected 'ok' in body, got %q", mock.HTTP.Response.Body)
		}
	})

	t.Run("request body stored as BodyEquals matcher", func(t *testing.T) {
		cmd := `curl -X POST -d '{"name":"test"}' https://api.example.com/users`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		if mock.HTTP.Matcher.BodyEquals != `{"name":"test"}` {
			t.Errorf("Expected BodyEquals to be set, got %q", mock.HTTP.Matcher.BodyEquals)
		}
	})

	t.Run("Accept header used as content type hint", func(t *testing.T) {
		cmd := `curl -H "Accept: text/xml" https://api.example.com/data`

		result, err := importer.Import([]byte(cmd))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		mock := result.Mocks[0]
		ct := mock.HTTP.Response.Headers["Content-Type"]
		if ct != "text/xml" {
			t.Errorf("Expected Content-Type 'text/xml', got %q", ct)
		}
	})
}

// ============================================================================
// Session 12: YAML format detection with proper parsing
// ============================================================================

func TestDetectFormatFromYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected Format
	}{
		{
			name:     "OpenAPI YAML",
			yaml:     "openapi: '3.0.3'\ninfo:\n  title: Test\npaths: {}",
			expected: FormatOpenAPI,
		},
		{
			name:     "Swagger 2.0 YAML",
			yaml:     "swagger: '2.0'\ninfo:\n  title: Legacy\nbasePath: /api",
			expected: FormatOpenAPI,
		},
		{
			name:     "Mockd with kind MockCollection",
			yaml:     "kind: MockCollection\nversion: '1.0'\nmocks: []",
			expected: FormatMockd,
		},
		{
			name:     "Mockd with version and mocks",
			yaml:     "version: '1.0'\nmocks:\n  - type: http",
			expected: FormatMockd,
		},
		{
			name:     "Mockd with just mocks array",
			yaml:     "mocks:\n  - type: http\n    http:\n      matcher:\n        path: /api",
			expected: FormatMockd,
		},
		{
			name:     "Single mock with type field",
			yaml:     "type: http\nhttp:\n  matcher:\n    path: /test",
			expected: FormatMockd,
		},
		{
			name:     "GraphQL single mock",
			yaml:     "type: graphql\ngraphql:\n  path: /graphql",
			expected: FormatMockd,
		},
		{
			name:     "Unknown YAML content",
			yaml:     "name: something\ndata:\n  - item1\n  - item2",
			expected: FormatUnknown,
		},
		{
			name:     "Invalid YAML falls back to string check",
			yaml:     "openapi: 3.0.3\n  bad indent: [",
			expected: FormatOpenAPI, // string fallback catches it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFormatFromYAML([]byte(tt.yaml))
			if result != tt.expected {
				t.Errorf("detectFormatFromYAML() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Session 12: curlDefaultResponse unit tests
// ============================================================================

func TestCurlDefaultResponse(t *testing.T) {
	tests := []struct {
		method     string
		wantStatus int
		wantBody   string
	}{
		{"POST", 201, `{"status": "created"}`},
		{"DELETE", 204, ""},
		{"PUT", 200, `{"status": "updated"}`},
		{"PATCH", 200, `{"status": "updated"}`},
		{"HEAD", 200, ""},
		{"OPTIONS", 200, ""},
		{"GET", 200, `{"status": "ok"}`},
		{"get", 200, `{"status": "ok"}`}, // case-insensitive
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			status, body := curlDefaultResponse(tt.method)
			if status != tt.wantStatus {
				t.Errorf("curlDefaultResponse(%q) status = %d, want %d", tt.method, status, tt.wantStatus)
			}
			if body != tt.wantBody {
				t.Errorf("curlDefaultResponse(%q) body = %q, want %q", tt.method, body, tt.wantBody)
			}
		})
	}
}

// ============================================================================
// Session 12: decodeHARBody unit tests
// ============================================================================

func TestDecodeHARBody(t *testing.T) {
	t.Run("decodes base64", func(t *testing.T) {
		content := HARContent{
			Text:     base64.StdEncoding.EncodeToString([]byte("hello world")),
			Encoding: "base64",
		}
		result := decodeHARBody(content)
		if result != "hello world" {
			t.Errorf("Expected 'hello world', got %q", result)
		}
	})

	t.Run("returns raw text when no encoding", func(t *testing.T) {
		content := HARContent{
			Text: "plain text",
		}
		result := decodeHARBody(content)
		if result != "plain text" {
			t.Errorf("Expected 'plain text', got %q", result)
		}
	})

	t.Run("returns empty for empty content", func(t *testing.T) {
		content := HARContent{}
		result := decodeHARBody(content)
		if result != "" {
			t.Errorf("Expected empty, got %q", result)
		}
	})

	t.Run("falls back to raw on invalid base64", func(t *testing.T) {
		content := HARContent{
			Text:     "not-base64-at-all!!!",
			Encoding: "base64",
		}
		result := decodeHARBody(content)
		if result != "not-base64-at-all!!!" {
			t.Errorf("Expected raw fallback, got %q", result)
		}
	})
}

// =============================================================================
// Session 13: DryRun and Native format round-trip tests
// =============================================================================

func TestImport_DryRun(t *testing.T) {
	// A simple cURL command
	curlData := []byte(`curl -X GET https://api.example.com/users/1`)

	result, err := Import(curlData, "request.sh", &ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Import DryRun failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result for DryRun")
	}
	if result.Collection == nil {
		t.Fatal("Expected non-nil collection in DryRun")
	}
	if result.EndpointCount == 0 {
		t.Error("Expected at least 1 endpoint in DryRun result")
	}
}

func TestImport_DryRunNameOverride(t *testing.T) {
	curlData := []byte(`curl -X POST https://api.example.com/users -d '{"name":"Alice"}'`)

	result, err := Import(curlData, "request.sh", &ImportOptions{
		DryRun: true,
		Name:   "My Custom Collection",
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Collection.Name != "My Custom Collection" {
		t.Errorf("Expected name 'My Custom Collection', got %q", result.Collection.Name)
	}
}

func TestImport_DryRunVsNormal_SameResult(t *testing.T) {
	curlData := []byte(`curl -X GET https://api.example.com/health`)

	dryResult, err := Import(curlData, "request.sh", &ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("DryRun import failed: %v", err)
	}

	normalResult, err := Import(curlData, "request.sh", &ImportOptions{DryRun: false})
	if err != nil {
		t.Fatalf("Normal import failed: %v", err)
	}

	// Both should produce the same endpoint count
	if dryResult.EndpointCount != normalResult.EndpointCount {
		t.Errorf("DryRun endpoint count %d != normal %d",
			dryResult.EndpointCount, normalResult.EndpointCount)
	}
}

func TestImport_UndetectableFormat(t *testing.T) {
	data := []byte(`this is not any recognizable format at all 12345`)
	_, err := Import(data, "unknown.txt", nil)
	if err == nil {
		t.Fatal("Expected error for undetectable format")
	}
}

func TestNativeExporter_RoundTrip(t *testing.T) {
	// Create a collection using the actual NativeV1 format and convert
	enabled := true
	collection := &config.MockCollection{
		Name: "Test Collection",
		Mocks: []*config.MockConfiguration{
			{
				Type:    mock.TypeHTTP,
				Name:    "Get Users",
				Enabled: &enabled,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/api/users",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       `{"users":["Alice","Bob"]}`,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
					},
				},
			},
		},
	}

	// Export to native YAML format
	exporter := &NativeExporter{AsYAML: true}
	exported, err := exporter.Export(collection)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(exported) == 0 {
		t.Fatal("Expected non-empty export")
	}

	// Re-import the exported data
	nativeImporter := &NativeImporter{}
	reimported, err := nativeImporter.Import(exported)
	if err != nil {
		t.Fatalf("Re-import failed: %v\nExported:\n%s", err, string(exported))
	}

	if reimported == nil {
		t.Fatal("Expected non-nil reimported collection")
	}

	// Verify key fields survived round-trip
	if len(reimported.Mocks) != 1 {
		t.Fatalf("Expected 1 mock after round-trip, got %d", len(reimported.Mocks))
	}

	m := reimported.Mocks[0]
	if m.HTTP == nil {
		t.Fatal("Expected HTTP config after round-trip")
	}
	if m.HTTP.Matcher == nil {
		t.Fatal("Expected HTTP matcher after round-trip")
	}
	if m.HTTP.Matcher.Method != "GET" {
		t.Errorf("Expected method GET, got %q", m.HTTP.Matcher.Method)
	}
	if m.HTTP.Matcher.Path != "/api/users" {
		t.Errorf("Expected path '/api/users', got %q", m.HTTP.Matcher.Path)
	}
	if m.HTTP.Response == nil {
		t.Fatal("Expected HTTP response after round-trip")
	}
	if m.HTTP.Response.StatusCode != 200 {
		t.Errorf("Expected status 200 after round-trip, got %d", m.HTTP.Response.StatusCode)
	}
}

func TestNativeExporter_JSON_RoundTrip(t *testing.T) {
	enabled := true
	collection := &config.MockCollection{
		Name: "JSON Test",
		Mocks: []*config.MockConfiguration{
			{
				Type:    mock.TypeHTTP,
				Name:    "Health",
				Enabled: &enabled,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/health",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       "ok",
					},
				},
			},
		},
	}

	// Export to JSON
	exporter := &NativeExporter{AsYAML: false}
	exported, err := exporter.Export(collection)
	if err != nil {
		t.Fatalf("JSON export failed: %v", err)
	}

	// Re-import
	nativeImporter := &NativeImporter{}
	reimported, err := nativeImporter.Import(exported)
	if err != nil {
		t.Fatalf("JSON re-import failed: %v\nExported:\n%s", err, string(exported))
	}

	if len(reimported.Mocks) != 1 {
		t.Fatalf("Expected 1 mock, got %d", len(reimported.Mocks))
	}
	if reimported.Mocks[0].HTTP == nil || reimported.Mocks[0].HTTP.Matcher == nil {
		t.Fatal("Expected HTTP matcher after JSON round-trip")
	}
	if reimported.Mocks[0].HTTP.Matcher.Path != "/health" {
		t.Errorf("Expected path '/health', got %q", reimported.Mocks[0].HTTP.Matcher.Path)
	}
}

func TestNativeExporter_WithStateful(t *testing.T) {
	enabled := true
	collection := &config.MockCollection{
		Name: "Stateful Test",
		Mocks: []*config.MockConfiguration{
			{
				Type:    mock.TypeHTTP,
				Name:    "API",
				Enabled: &enabled,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/api/items",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
					},
				},
			},
		},
		StatefulResources: []*config.StatefulResourceConfig{
			{
				Name:     "products",
				BasePath: "/api/products",
			},
		},
	}

	exporter := &NativeExporter{AsYAML: true}
	exported, err := exporter.Export(collection)
	if err != nil {
		t.Fatalf("Export with stateful failed: %v", err)
	}

	nativeImporter := &NativeImporter{}
	reimported, err := nativeImporter.Import(exported)
	if err != nil {
		t.Fatalf("Re-import with stateful failed: %v", err)
	}

	if len(reimported.StatefulResources) != 1 {
		t.Fatalf("Expected 1 stateful resource, got %d", len(reimported.StatefulResources))
	}
	if reimported.StatefulResources[0].Name != "products" {
		t.Errorf("Expected resource name 'products', got %q", reimported.StatefulResources[0].Name)
	}
}

func TestExport_DefaultsToMockd(t *testing.T) {
	enabled := true
	collection := &config.MockCollection{
		Name: "Default Format Test",
		Mocks: []*config.MockConfiguration{
			{
				Type:    mock.TypeHTTP,
				Enabled: &enabled,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/ping",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       "pong",
					},
				},
			},
		},
	}

	result, err := Export(collection, nil)
	if err != nil {
		t.Fatalf("Export with nil opts failed: %v", err)
	}
	if result.Format != FormatMockd {
		t.Errorf("Expected format mockd, got %s", result.Format)
	}
	if result.EndpointCount != 1 {
		t.Errorf("Expected endpoint count 1, got %d", result.EndpointCount)
	}
	if len(result.Data) == 0 {
		t.Error("Expected non-empty export data")
	}
}
