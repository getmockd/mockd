// Package cli provides the command-line interface for mockd.
//
// The cli package implements all CLI commands for managing the mock server:
//   - start: Launch the mock server with HTTP and admin API
//   - serve: Enhanced start with runtime mode and cloud features
//   - add: Create a new mock endpoint
//   - list: Display all configured mocks
//   - get: Show details of a specific mock
//   - delete: Remove a mock by ID
//   - import: Import mocks from various formats (OpenAPI, Postman, HAR, WireMock, cURL)
//   - export: Export mocks to native format or OpenAPI
//   - new: Create new mock collections from templates (blank, crud, auth, pagination, errors)
//   - generate: AI-powered mock generation from OpenAPI specs or descriptions
//   - enhance: Improve existing mocks with AI-generated data
//   - logs: View request logs for debugging
//   - config: Display effective configuration
//   - version: Show mockd version
//   - completion: Generate shell completion scripts
//
// AI commands (require MOCKD_AI_PROVIDER and MOCKD_AI_API_KEY environment variables):
//   - generate: Generate mocks from OpenAPI with AI enhancement, or from natural language
//   - enhance: Add AI-generated realistic data to existing mocks
//
// Protocol commands:
//   - graphql: Manage and test GraphQL endpoints (validate schemas, execute queries)
//   - grpc: Manage and test gRPC endpoints (list services, call methods)
//   - mqtt: Publish and subscribe to MQTT topics for testing
//   - soap: Validate WSDL files and call SOAP operations
//   - chaos: Manage chaos injection for fault testing (enable, disable, status)
//
// Commands communicate with the running mock server via the admin API.
// The start command runs the server in the foreground with graceful shutdown.
// Use --load to load mocks from a directory and --watch for hot-reloading.
//
// Protocol flags (available in start and serve):
//   - --graphql-schema, --graphql-path: Configure GraphQL mocking
//   - --grpc-port, --grpc-proto, --grpc-reflection: Configure gRPC mocking
//   - --oauth-enabled, --oauth-issuer, --oauth-port: Configure OAuth provider
//   - --mqtt-port, --mqtt-auth: Configure MQTT broker
//   - --chaos-enabled, --chaos-latency, --chaos-error-rate: Configure chaos injection
//   - --validate-spec, --validate-fail: Configure OpenAPI request validation
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
//	mockd start --port 4280 --config mocks.json
//	mockd start --load ./mocks/ --watch
//	mockd start --graphql-schema schema.graphql --graphql-path /graphql
//	mockd start --grpc-port 50051 --grpc-proto api.proto
//	mockd start --chaos-enabled --chaos-latency "10ms-100ms"
//	mockd add --path /api/users --status 200 --body '[]'
//	mockd list
//	mockd import openapi.yaml
//	mockd export --format openapi > api.yaml
//	mockd graphql validate schema.graphql
//	mockd grpc list api.proto
//	mockd chaos enable --latency "50ms-200ms"
//	mockd generate --ai --input openapi.yaml -o mocks.yaml
//	mockd generate --ai --prompt "user management API"
//	mockd enhance --ai
package cli
