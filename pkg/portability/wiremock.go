package portability

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// WireMock mapping types

// WireMockMapping represents a WireMock stub mapping.
type WireMockMapping struct {
	ID       string                 `json:"id,omitempty"`
	UUID     string                 `json:"uuid,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Priority int                    `json:"priority,omitempty"`
	Request  WireMockRequest        `json:"request"`
	Response WireMockResponse       `json:"response"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// WireMockRequest represents a WireMock request pattern.
type WireMockRequest struct {
	Method          string                     `json:"method,omitempty"`
	URL             string                     `json:"url,omitempty"`
	URLPath         string                     `json:"urlPath,omitempty"`
	URLPattern      string                     `json:"urlPattern,omitempty"`
	URLPathPattern  string                     `json:"urlPathPattern,omitempty"`
	Headers         map[string]WireMockMatcher `json:"headers,omitempty"`
	QueryParameters map[string]WireMockMatcher `json:"queryParameters,omitempty"`
	BodyPatterns    []WireMockBodyPattern      `json:"bodyPatterns,omitempty"`
}

// WireMockMatcher represents a WireMock matcher.
type WireMockMatcher struct {
	EqualTo         string `json:"equalTo,omitempty"`
	Contains        string `json:"contains,omitempty"`
	Matches         string `json:"matches,omitempty"`
	DoesNotMatch    string `json:"doesNotMatch,omitempty"`
	EqualToJSON     string `json:"equalToJson,omitempty"`
	MatchesJSONPath string `json:"matchesJsonPath,omitempty"`
	CaseInsensitive bool   `json:"caseInsensitive,omitempty"`
}

// WireMockBodyPattern represents a body pattern matcher.
type WireMockBodyPattern struct {
	EqualTo         string `json:"equalTo,omitempty"`
	Contains        string `json:"contains,omitempty"`
	Matches         string `json:"matches,omitempty"`
	EqualToJSON     string `json:"equalToJson,omitempty"`
	MatchesJSONPath string `json:"matchesJsonPath,omitempty"`
}

// WireMockResponse represents a WireMock response definition.
type WireMockResponse struct {
	Status           int               `json:"status"`
	StatusMessage    string            `json:"statusMessage,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             string            `json:"body,omitempty"`
	Base64Body       string            `json:"base64Body,omitempty"`
	JSONBody         interface{}       `json:"jsonBody,omitempty"`
	BodyFileName     string            `json:"bodyFileName,omitempty"`
	FixedDelayMillis int               `json:"fixedDelayMilliseconds,omitempty"`
	Transformers     []string          `json:"transformers,omitempty"`
}

// WireMockImporter imports WireMock JSON mappings.
type WireMockImporter struct{}

// Import parses WireMock mappings and returns a MockCollection.
func (i *WireMockImporter) Import(data []byte) (*config.MockCollection, error) {
	// Try to parse as a single mapping first
	var singleMapping WireMockMapping
	if err := json.Unmarshal(data, &singleMapping); err == nil {
		if singleMapping.Request.Method != "" || singleMapping.Request.URL != "" ||
			singleMapping.Request.URLPath != "" || singleMapping.Request.URLPattern != "" {
			return i.convertMappings([]WireMockMapping{singleMapping})
		}
	}

	// Try to parse as an array of mappings
	var mappings []WireMockMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		// Try to parse as WireMock's mappings wrapper
		var wrapper struct {
			Mappings []WireMockMapping `json:"mappings"`
		}
		if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
			return nil, &ImportError{
				Format:  FormatWireMock,
				Message: "failed to parse WireMock mappings",
				Cause:   err,
			}
		}
		mappings = wrapper.Mappings
	}

	if len(mappings) == 0 {
		return nil, &ImportError{
			Format:  FormatWireMock,
			Message: "no WireMock mappings found",
		}
	}

	return i.convertMappings(mappings)
}

// convertMappings converts WireMock mappings to a MockCollection.
func (i *WireMockImporter) convertMappings(mappings []WireMockMapping) (*config.MockCollection, error) {
	result := &config.MockCollection{
		Version: "1.0",
		Name:    "Imported from WireMock",
		Mocks:   make([]*config.MockConfiguration, 0, len(mappings)),
	}

	now := time.Now()
	for idx, mapping := range mappings {
		mock := i.mappingToMock(mapping, idx+1, now)
		result.Mocks = append(result.Mocks, mock)
	}

	return result, nil
}

// mappingToMock converts a WireMock mapping to a MockConfiguration.
func (i *WireMockImporter) mappingToMock(mapping WireMockMapping, id int, now time.Time) *config.MockConfiguration {
	enabled := true
	m := &config.MockConfiguration{
		ID:        fmt.Sprintf("imported-%d", id),
		Name:      mapping.Name,
		Type:      mock.MockTypeHTTP,
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: mapping.Priority,
			Matcher:  &mock.HTTPMatcher{},
		},
	}

	if m.Name == "" {
		m.Name = fmt.Sprintf("WireMock Import %d", id)
	}

	// Convert request matching
	req := mapping.Request

	// Method
	m.HTTP.Matcher.Method = req.Method
	if m.HTTP.Matcher.Method == "" {
		m.HTTP.Matcher.Method = "GET" // Default to GET
	}

	// URL path - prefer urlPath over url over urlPattern
	if req.URLPath != "" {
		m.HTTP.Matcher.Path = req.URLPath
	} else if req.URL != "" {
		m.HTTP.Matcher.Path = req.URL
	} else if req.URLPattern != "" {
		m.HTTP.Matcher.PathPattern = req.URLPattern
	} else if req.URLPathPattern != "" {
		m.HTTP.Matcher.PathPattern = req.URLPathPattern
	}

	// Headers
	if len(req.Headers) > 0 {
		headers := make(map[string]string)
		for name, matcher := range req.Headers {
			if matcher.EqualTo != "" {
				headers[name] = matcher.EqualTo
			}
		}
		if len(headers) > 0 {
			m.HTTP.Matcher.Headers = headers
		}
	}

	// Query parameters
	if len(req.QueryParameters) > 0 {
		queryParams := make(map[string]string)
		for name, matcher := range req.QueryParameters {
			if matcher.EqualTo != "" {
				queryParams[name] = matcher.EqualTo
			}
		}
		if len(queryParams) > 0 {
			m.HTTP.Matcher.QueryParams = queryParams
		}
	}

	// Body patterns
	if len(req.BodyPatterns) > 0 {
		bp := req.BodyPatterns[0]
		if bp.EqualTo != "" {
			m.HTTP.Matcher.BodyEquals = bp.EqualTo
		} else if bp.Contains != "" {
			m.HTTP.Matcher.BodyContains = bp.Contains
		} else if bp.Matches != "" {
			m.HTTP.Matcher.BodyPattern = bp.Matches
		}
	}

	// Convert response
	resp := mapping.Response
	m.HTTP.Response = &mock.HTTPResponse{
		StatusCode: resp.Status,
		Headers:    make(map[string]string),
		DelayMs:    resp.FixedDelayMillis,
	}

	if m.HTTP.Response.StatusCode == 0 {
		m.HTTP.Response.StatusCode = 200
	}

	// Response headers
	for name, value := range resp.Headers {
		m.HTTP.Response.Headers[name] = value
	}

	// Response body
	if resp.Body != "" {
		m.HTTP.Response.Body = resp.Body
	} else if resp.JSONBody != nil {
		bodyBytes, _ := json.MarshalIndent(resp.JSONBody, "", "  ")
		m.HTTP.Response.Body = string(bodyBytes)
		if _, ok := m.HTTP.Response.Headers["Content-Type"]; !ok {
			m.HTTP.Response.Headers["Content-Type"] = "application/json"
		}
	}

	// Note: base64Body and bodyFileName not supported - would need file system access

	return m
}

// Format returns FormatWireMock.
func (i *WireMockImporter) Format() Format {
	return FormatWireMock
}

// WireMockTemplatingNote documents WireMock-to-mockd templating limitations.
const WireMockTemplatingNote = `
WireMock Templating Migration Notes:

WireMock response templating features that are NOT automatically converted:
- {{request.path}} - Use mockd path parameters instead
- {{request.query.*}} - Use mockd query parameter matching
- {{request.body}} - Use mockd body matching
- {{jsonPath ...}} - Not supported
- {{randomValue ...}} - Not supported
- Handlebars helpers - Not supported

Recommended approach:
1. Import WireMock mappings
2. Manually add mockd templating for dynamic values
3. Use mockd stateful resources for complex scenarios
`

// ExtractWireMockVariables attempts to find WireMock template variables in a string.
func ExtractWireMockVariables(s string) []string {
	var vars []string
	for i := 0; i < len(s)-3; i++ {
		if s[i:i+2] == "{{" {
			end := strings.Index(s[i:], "}}")
			if end > 0 {
				vars = append(vars, s[i+2:i+end])
				i += end
			}
		}
	}
	return vars
}

// init registers the WireMock importer.
func init() {
	RegisterImporter(&WireMockImporter{})
}
