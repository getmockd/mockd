// Package cli provides the command-line interface for mockd.
//
// The cli package implements all CLI commands for managing the mock server:
//   - start: Launch the mock server with HTTP and admin API
//   - add: Create a new mock endpoint
//   - list: Display all configured mocks
//   - get: Show details of a specific mock
//   - delete: Remove a mock by ID
//   - import: Import mocks from various formats (OpenAPI, Postman, HAR, WireMock, cURL)
//   - export: Export mocks to native format or OpenAPI
//   - new: Create new mock collections from templates (blank, crud, auth, pagination, errors)
//   - logs: View request logs for debugging
//   - config: Display effective configuration
//   - version: Show mockd version
//   - completion: Generate shell completion scripts
//
// Commands communicate with the running mock server via the admin API.
// The start command runs the server in the foreground with graceful shutdown.
// Use --load to load mocks from a directory and --watch for hot-reloading.
//
// Import formats:
//   - mockd: Native YAML/JSON format
//   - openapi: OpenAPI 3.x or Swagger 2.0
//   - postman: Postman Collection v2.x
//   - har: HTTP Archive (browser recordings)
//   - wiremock: WireMock JSON mappings
//   - curl: cURL commands
//
// Export formats:
//   - mockd: Native YAML/JSON format
//   - openapi: OpenAPI 3.x specification
//
// Usage:
//
//	mockd start --port 8080 --config mocks.json
//	mockd start --load ./mocks/ --watch
//	mockd add --path /api/users --status 200 --body '[]'
//	mockd list
//	mockd import openapi.yaml
//	mockd export --format openapi > api.yaml
//	mockd new --template crud --resource users -o users.yaml
package cli
