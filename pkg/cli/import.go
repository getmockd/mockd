package cli

import (
	"fmt"
	"strings"

	"github.com/getmockd/mockd/pkg/portability"
)

// formatImportError formats import errors with helpful suggestions.
func formatImportError(err error, format portability.Format, _ string) error {
	errStr := err.Error()

	// Check for common validation errors
	if strings.Contains(errStr, "unsupported version") {
		return fmt.Errorf(`import failed: %w

The file has an invalid or missing 'version' field.

Expected format:
  version: "1.0"
  mocks:
    - id: my-mock
      type: http
      ...

Tip: Make sure version is quoted: version: "1.0"`, err)
	}

	if strings.Contains(errStr, "one of response, sse, or chunked is required") {
		return fmt.Errorf(`import failed: %w

HTTP mocks require a response configuration.

Example:
  mocks:
    - id: my-mock
      type: http
      http:
        matcher:
          method: GET
          path: /api/example
        response:
          statusCode: 200
          body: '{"ok": true}'

Or for SSE streaming:
        sse:
          events:
            - type: message
              data: "hello"`, err)
	}

	if strings.Contains(errStr, "duplicate mock ID") {
		return fmt.Errorf(`import failed: %w - each mock must have a unique 'id', check for duplicates`, err)
	}

	if strings.Contains(errStr, "failed to parse") {
		return fmt.Errorf(`import failed: %w

The file could not be parsed. Common issues:
  • Invalid YAML/JSON syntax
  • Incorrect indentation
  • Missing required fields

Validate YAML at: https://www.yamllint.com/`, err)
	}

	// Generic error with format-specific help
	switch format {
	case portability.FormatMockd:
		return fmt.Errorf(`import failed: %w

For mockd format, ensure your file has:
  version: "1.0"
  mocks:
    - id: unique-id
      type: http
      http:
        matcher:
          method: GET
          path: /example
        response:
          statusCode: 200
          body: '{"ok": true}'`, err)

	case portability.FormatOpenAPI:
		return fmt.Errorf(`import failed: %w

For OpenAPI format, ensure your file has:
  openapi: "3.0.0"  (or swagger: "2.0")
  info:
    title: API Name
    version: "1.0.0"
  paths:
    /example:
      get:
        responses:
          200:
            description: OK`, err)

	default:
		return fmt.Errorf("import failed: %w", err)
	}
}
