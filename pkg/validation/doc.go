// Package validation provides OpenAPI/Swagger request and response validation
// for the mockd core engine.
//
// The package validates HTTP requests and responses against OpenAPI 3.0 and 3.1
// specifications, checking:
//   - Path parameters
//   - Query parameters
//   - Request headers
//   - Request body against JSON Schema
//   - Response body and headers (optional)
//
// # Basic Usage
//
// Create a validator from an OpenAPI spec file:
//
//	config := &ValidationConfig{
//	    Enabled:         true,
//	    SpecFile:        "openapi.yaml",
//	    ValidateRequest: true,
//	    FailOnError:     true,
//	}
//	validator, err := NewOpenAPIValidator(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Validate a request:
//
//	result := validator.ValidateRequest(req)
//	if !result.Valid {
//	    for _, e := range result.Errors {
//	        log.Printf("Validation error: %s - %s", e.Type, e.Message)
//	    }
//	}
//
// # Middleware Usage
//
// Wrap an HTTP handler with validation middleware:
//
//	handler := http.HandlerFunc(myHandler)
//	middleware := NewMiddleware(handler, validator, config)
//	http.ListenAndServe(":4280", middleware)
//
// # Configuration
//
// The ValidationConfig struct controls validation behavior:
//   - Enabled: Enable/disable validation
//   - SpecFile: Path to OpenAPI spec file
//   - SpecURL: URL to fetch OpenAPI spec from
//   - Spec: Inline OpenAPI spec as string
//   - ValidateRequest: Validate incoming requests
//   - ValidateResponse: Validate outgoing responses
//   - FailOnError: Return 400 on validation failure
//   - LogWarnings: Log warnings without failing
//
// # Spec Loading
//
// The package supports loading specs from:
//   - Local files (YAML or JSON)
//   - Remote URLs
//   - Inline strings
//
// Both OpenAPI 3.0 and 3.1 specifications are supported.
package validation
