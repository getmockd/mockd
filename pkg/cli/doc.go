// Package cli provides the command-line interface for mockd.
//
// The cli package implements all CLI commands for managing the mock server:
//   - start: Launch the mock server with HTTP and admin API
//   - add: Create a new mock endpoint
//   - list: Display all configured mocks
//   - get: Show details of a specific mock
//   - delete: Remove a mock by ID
//   - import: Load mocks from a configuration file
//   - export: Save current mocks to a file
//   - logs: View request logs for debugging
//   - config: Display effective configuration
//   - version: Show mockd version
//   - completion: Generate shell completion scripts
//
// Commands communicate with the running mock server via the admin API.
// The start command runs the server in the foreground with graceful shutdown.
//
// Usage:
//
//	mockd start --port 8080 --config mocks.json
//	mockd add --path /api/users --status 200 --body '[]'
//	mockd list
//	mockd export > backup.json
package cli
