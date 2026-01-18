// Package portability provides import/export functionality for mock configurations.
//
// This package enables users to:
//   - Export mocks to portable formats (Mockd native YAML/JSON, OpenAPI 3.x)
//   - Import mocks from various sources (OpenAPI, Swagger, Postman, HAR, WireMock, cURL)
//   - Use templates to scaffold common mock patterns (CRUD, auth, pagination, errors)
//
// # Supported Formats
//
// Import formats:
//   - OpenAPI 3.x specifications
//   - Swagger 2.0 specifications
//   - Postman Collection v2.x
//   - HAR (HTTP Archive) files
//   - WireMock JSON mappings
//   - cURL commands
//   - Mockd native format (YAML/JSON)
//
// Export formats:
//   - Mockd native format (YAML/JSON) - recommended for portability
//   - OpenAPI 3.x - for documentation and API contracts
//
// # Usage
//
// Basic import example:
//
//	data, _ := os.ReadFile("openapi.yaml")
//	format := portability.DetectFormat(data, "openapi.yaml")
//	importer := portability.GetImporter(format)
//	collection, err := importer.Import(data)
//
// Basic export example:
//
//	exporter := portability.GetExporter(portability.FormatMockd)
//	data, err := exporter.Export(collection)
//	os.WriteFile("mocks.yaml", data, 0644)
//
// # Templates
//
// Built-in templates for common patterns:
//   - blank: Empty collection
//   - crud: REST CRUD endpoints (list, get, create, update, delete)
//   - auth: Authentication flow (login, logout, refresh, me)
//   - pagination: List with cursor/offset pagination
//   - errors: Common HTTP error responses (400, 401, 403, 404, 500)
//
// Template example:
//
//	template := portability.GetTemplate("crud")
//	collection, err := template.Generate(map[string]string{"resource": "users"})
package portability
