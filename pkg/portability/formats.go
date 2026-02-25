package portability

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	FormatWSDL     Format = "wsdl"     // WSDL 1.1 service definition
)

// String returns the string representation of the format.
func (f Format) String() string {
	return string(f)
}

// IsValid returns true if the format is a known format.
func (f Format) IsValid() bool {
	switch f {
	case FormatMockd, FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL, FormatWSDL:
		return true
	default:
		return false
	}
}

// CanImport returns true if this format supports importing.
func (f Format) CanImport() bool {
	switch f {
	case FormatMockd, FormatOpenAPI, FormatPostman, FormatHAR, FormatWireMock, FormatCURL, FormatWSDL:
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

	// Check for WSDL extension
	if ext == ".wsdl" {
		return FormatWSDL
	}

	// Check for WSDL content (XML with definitions root element)
	if ext == ".xml" || ext == "" {
		if isWSDLContent(trimmed) {
			return FormatWSDL
		}
	}

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
		if f := detectFormatFromJSON(data); f != FormatUnknown {
			return f
		}
		// JSON detection failed — try YAML as fallback (covers no-extension case)
		if ext == "" {
			return detectFormatFromYAML(data)
		}
	}

	return FormatUnknown
}

// detectFormatFromJSON detects format from JSON content.
func detectFormatFromJSON(data []byte) Format {
	// Try to parse as JSON object to get key indicators
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
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

		// Check for WireMock indicators (single mapping)
		if _, hasRequest := raw["request"]; hasRequest {
			if _, hasResponse := raw["response"]; hasResponse {
				return FormatWireMock
			}
		}

		// Check for WireMock mappings wrapper format: {"mappings": [...]}
		if mappingsRaw, hasMappings := raw["mappings"]; hasMappings {
			var arr []json.RawMessage
			if json.Unmarshal(mappingsRaw, &arr) == nil && len(arr) > 0 {
				var firstMapping map[string]json.RawMessage
				if json.Unmarshal(arr[0], &firstMapping) == nil {
					if _, hasReq := firstMapping["request"]; hasReq {
						return FormatWireMock
					}
				}
			}
		}

		return FormatUnknown
	}

	// If not a JSON object, try parsing as an array (e.g., WireMock array of mappings)
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

// detectFormatFromYAML detects format from YAML content by parsing into a map
// and inspecting top-level keys, rather than fragile string matching.
func detectFormatFromYAML(data []byte) Format {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		// If YAML parsing fails entirely, fall back to basic string checks
		// for the most critical indicators.
		content := string(data)
		if strings.Contains(content, "openapi:") || strings.Contains(content, "swagger:") {
			return FormatOpenAPI
		}
		return FormatUnknown
	}

	// Check for OpenAPI 3.x
	if _, ok := raw["openapi"]; ok {
		return FormatOpenAPI
	}

	// Check for Swagger 2.0
	if _, ok := raw["swagger"]; ok {
		return FormatOpenAPI
	}

	// Check for Mockd native format
	// 1. Explicit kind: MockCollection
	if kind, ok := raw["kind"]; ok {
		if kindStr, ok := kind.(string); ok && kindStr == "MockCollection" {
			return FormatMockd
		}
	}
	// 2. Has version + mocks
	_, hasVersion := raw["version"]
	_, hasMocks := raw["mocks"]
	if hasVersion && hasMocks {
		return FormatMockd
	}
	// 3. Has mocks array (even without version)
	if hasMocks {
		return FormatMockd
	}
	// 4. Single mock definition with matcher + path (inline mock)
	if _, hasMatcher := raw["matcher"]; hasMatcher {
		if _, hasPath := raw["path"]; hasPath {
			return FormatMockd
		}
	}
	// 5. Has type field with protocol spec (single mock)
	if t, ok := raw["type"]; ok {
		if ts, ok := t.(string); ok {
			switch ts {
			case "http", "websocket", "graphql", "grpc", "soap", "mqtt", "oauth":
				return FormatMockd
			}
		}
	}

	return FormatUnknown
}

// ParseFormat parses a format string into a Format type.
// Returns FormatUnknown for unrecognized format strings.
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mockd", "native", "json", "yaml", "yml":
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
	case "wsdl":
		return FormatWSDL
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
		FormatWSDL,
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
		FormatWSDL,
	}
}

// ExportFormats returns a list of formats that support exporting.
func ExportFormats() []Format {
	return []Format{
		FormatMockd,
		FormatOpenAPI,
	}
}

// isWSDLContent checks if the data looks like a WSDL document by inspecting
// the XML root element for WSDL 1.1 <definitions> or WSDL 2.0 <description>.
func isWSDLContent(data []byte) bool {
	s := string(data)
	// Quick check: must be XML
	if !strings.HasPrefix(s, "<?xml") && !strings.HasPrefix(s, "<") {
		return false
	}
	// Check for WSDL 1.1 root element (with or without namespace prefix)
	if strings.Contains(s, "<definitions") || strings.Contains(s, "<wsdl:definitions") {
		return true
	}
	// Check for WSDL 2.0 root element
	if strings.Contains(s, "<description") || strings.Contains(s, "<wsdl:description") {
		return true
	}
	return false
}
