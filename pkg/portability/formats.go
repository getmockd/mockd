package portability

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
)

// Format represents a supported import/export format.
type Format string

// Supported formats for import/export.
const (
	FormatUnknown  Format = ""
	FormatMockd    Format = "mockd"    // Native Mockd format (YAML/JSON)
	FormatOpenAPI  Format = "openapi"  // OpenAPI 3.x or Swagger 2.0
	FormatPostman  Format = "postman"  // Postman Collection v2.x
	FormatHAR      Format = "har"      // HTTP Archive format
	FormatWireMock Format = "wiremock" // WireMock JSON mappings
	FormatCURL     Format = "curl"     // cURL command
)

// String returns the string representation of the format.
func (f Format) String() string {
	return string(f)
}

// IsValid returns true if the format is a known format.
func (f Format) IsValid() bool {
	switch f {
	case FormatMockd, FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL:
		return true
	default:
		return false
	}
}

// CanImport returns true if this format supports importing.
func (f Format) CanImport() bool {
	switch f {
	case FormatMockd, FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL:
		return true
	default:
		return false
	}
}

// CanExport returns true if this format supports exporting.
// Note: We only export to Mockd native and OpenAPI formats (strategic choice).
func (f Format) CanExport() bool {
	switch f {
	case FormatMockd, FormatOpenAPI:
		return true
	default:
		return false
	}
}

// DetectFormat attempts to auto-detect the format from file content and filename.
// Returns FormatUnknown if the format cannot be determined.
func DetectFormat(data []byte, filename string) Format {
	// First check if it looks like a cURL command (starts with "curl" after whitespace)
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("curl ")) || bytes.HasPrefix(trimmed, []byte("curl\t")) {
		return FormatCURL
	}

	// Check file extension for hints
	ext := strings.ToLower(filepath.Ext(filename))

	// Check for HAR extension
	if ext == ".har" {
		return FormatHAR
	}

	// For JSON/YAML files, we need to parse and inspect content
	isYAML := ext == ".yaml" || ext == ".yml"
	isJSON := ext == ".json"

	// If it's a YAML file, we'll need to detect based on content
	if isYAML {
		return detectFormatFromYAML(data)
	}

	// For JSON or unknown extensions, try to parse and detect
	if isJSON || ext == "" {
		return detectFormatFromJSON(data)
	}

	return FormatUnknown
}

// detectFormatFromJSON detects format from JSON content.
func detectFormatFromJSON(data []byte) Format {
	// Try to parse as JSON to get key indicators
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return FormatUnknown
	}

	// Check for Mockd native format indicators
	if _, hasVersion := raw["version"]; hasVersion {
		if _, hasKind := raw["kind"]; hasKind {
			return FormatMockd
		}
		// Version + mocks is also Mockd format
		if _, hasMocks := raw["mocks"]; hasMocks {
			return FormatMockd
		}
	}

	// Check for OpenAPI 3.x indicator
	if _, hasOpenAPI := raw["openapi"]; hasOpenAPI {
		return FormatOpenAPI
	}

	// Check for Swagger 2.0 indicator
	if _, hasSwagger := raw["swagger"]; hasSwagger {
		return FormatOpenAPI
	}

	// Check for HAR format indicator
	if _, hasLog := raw["log"]; hasLog {
		return FormatHAR
	}

	// Check for Postman Collection indicator
	if _, hasInfo := raw["info"]; hasInfo {
		if _, hasItem := raw["item"]; hasItem {
			return FormatPostman
		}
	}

	// Check for WireMock indicators
	if _, hasRequest := raw["request"]; hasRequest {
		if _, hasResponse := raw["response"]; hasResponse {
			return FormatWireMock
		}
	}

	// WireMock can also be an array of mappings
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		var firstMapping map[string]json.RawMessage
		if err := json.Unmarshal(arr[0], &firstMapping); err == nil {
			if _, hasRequest := firstMapping["request"]; hasRequest {
				if _, hasResponse := firstMapping["response"]; hasResponse {
					return FormatWireMock
				}
			}
		}
	}

	return FormatUnknown
}

// detectFormatFromYAML detects format from YAML content.
// Since YAML is a superset of JSON for simple cases, we can use similar detection.
func detectFormatFromYAML(data []byte) Format {
	// Check for common YAML key patterns using string matching
	content := string(data)

	// Check for OpenAPI 3.x
	if strings.Contains(content, "openapi:") {
		return FormatOpenAPI
	}

	// Check for Swagger 2.0
	if strings.Contains(content, "swagger:") {
		return FormatOpenAPI
	}

	// Check for Mockd native format - multiple indicators
	// 1. Explicit kind: MockCollection
	if strings.Contains(content, "kind:") && strings.Contains(content, "MockCollection") {
		return FormatMockd
	}
	// 2. Has version: and mocks: fields
	if strings.Contains(content, "version:") && strings.Contains(content, "mocks:") {
		return FormatMockd
	}
	// 3. Has mocks: array with http: or type: http entries (even without version header)
	if strings.Contains(content, "mocks:") {
		if strings.Contains(content, "http:") || strings.Contains(content, "type: http") ||
			strings.Contains(content, "websocket:") || strings.Contains(content, "type: websocket") ||
			strings.Contains(content, "graphql:") || strings.Contains(content, "type: graphql") ||
			strings.Contains(content, "grpc:") || strings.Contains(content, "type: grpc") {
			return FormatMockd
		}
	}
	// 4. Single mock definition with http.matcher
	if strings.Contains(content, "matcher:") && strings.Contains(content, "path:") {
		return FormatMockd
	}

	return FormatUnknown
}

// ParseFormat parses a format string into a Format type.
// Returns FormatUnknown for unrecognized format strings.
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mockd", "native":
		return FormatMockd
	case "openapi", "swagger", "oas":
		return FormatOpenAPI
	case "postman":
		return FormatPostman
	case "har":
		return FormatHAR
	case "wiremock":
		return FormatWireMock
	case "curl":
		return FormatCURL
	default:
		return FormatUnknown
	}
}

// AllFormats returns a list of all supported formats.
func AllFormats() []Format {
	return []Format{
		FormatMockd,
		FormatOpenAPI,
		FormatPostman,
		FormatHAR,
		FormatWireMock,
		FormatCURL,
	}
}

// ImportFormats returns a list of formats that support importing.
func ImportFormats() []Format {
	return []Format{
		FormatMockd,
		FormatOpenAPI,
		FormatPostman,
		FormatHAR,
		FormatWireMock,
		FormatCURL,
	}
}

// ExportFormats returns a list of formats that support exporting.
func ExportFormats() []Format {
	return []Format{
		FormatMockd,
		FormatOpenAPI,
	}
}
