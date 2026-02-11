// Package admin provides a REST API for managing mock configurations at runtime.
//
// The admin API allows dynamic creation, modification, and deletion of mocks
// without restarting the server. It also provides endpoints for:
//   - Health checks
//   - Configuration import/export
//   - Request log inspection
//
// Endpoints:
//
//	GET  /health           - Server health check
//	GET  /mocks            - List all mocks
//	POST /mocks            - Create a new mock
//	GET  /mocks/{id}       - Get a specific mock
//	PUT  /mocks/{id}       - Update an existing mock
//	DELETE /mocks/{id}     - Delete a mock
//	POST /mocks/{id}/toggle - Enable/disable a mock
//	GET  /config           - Export current configuration
//	POST /config           - Import configuration
//	GET  /requests         - List request logs
//	GET  /requests/{id}    - Get a specific request log
//	DELETE /requests       - Clear request logs
//
// Usage:
//
//	srv := engine.NewServer(cfg)
//	srv.Start()
//
//	engineURL := fmt.Sprintf("http://localhost:%d", srv.ManagementPort())
//	adminAPI := admin.NewAPI(4290, admin.WithLocalEngine(engineURL))
//	adminAPI.Start()
//	defer adminAPI.Stop()
//
// Example curl commands:
//
//	# Create a mock
//	curl -X POST http://localhost:4290/mocks \
//	  -H "Content-Type: application/json" \
//	  -d '{"matcher": {"path": "/test"}, "response": {"statusCode": 200}}'
//
//	# List mocks
//	curl http://localhost:4290/mocks
//
//	# Delete a mock
//	curl -X DELETE http://localhost:4290/mocks/{id}
package admin
