package portability

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
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

// applyRequestMatching converts WireMock request matching rules into the mockd
// HTTPMatcher, handling URL, headers, query parameters, and body patterns.
func applyRequestMatching(matcher *mock.HTTPMatcher, req WireMockRequest) {
	// Method
	matcher.Method = req.Method
	if matcher.Method == "" {
		matcher.Method = "GET" // Default to GET
	}

	// URL path - prefer urlPath over url over urlPattern
	switch {
	case req.URLPath != "":
		matcher.Path = req.URLPath
	case req.URL != "":
		// WireMock's url field is an exact URL match that may include query strings.
		// Split into path and query parameters for correct mockd matching.
		if qIdx := strings.IndexByte(req.URL, '?'); qIdx >= 0 {
			matcher.Path = req.URL[:qIdx]
			if parsed, err := url.ParseQuery(req.URL[qIdx+1:]); err == nil && len(parsed) > 0 {
				if matcher.QueryParams == nil {
					matcher.QueryParams = make(map[string]string)
				}
				for key, values := range parsed {
					if len(values) > 0 {
						matcher.QueryParams[key] = values[0]
					}
				}
			}
		} else {
			matcher.Path = req.URL
		}
	case req.URLPattern != "":
		matcher.PathPattern = req.URLPattern
	case req.URLPathPattern != "":
		matcher.PathPattern = req.URLPathPattern
	}

	// Headers — map equalTo to exact match; contains and matches are best-effort
	// mapped to the same header value (mockd supports exact header matching only,
	// so contains/matches are imported as the literal pattern for documentation).
	if len(req.Headers) > 0 {
		headers := make(map[string]string)
		for name, m := range req.Headers {
			switch {
			case m.EqualTo != "":
				headers[name] = m.EqualTo
			case m.Contains != "":
				headers[name] = m.Contains
			case m.Matches != "":
				headers[name] = m.Matches
			}
		}
		if len(headers) > 0 {
			matcher.Headers = headers
		}
	}

	// Query parameters — same approach as headers
	if len(req.QueryParameters) > 0 {
		queryParams := make(map[string]string)
		for name, m := range req.QueryParameters {
			switch {
			case m.EqualTo != "":
				queryParams[name] = m.EqualTo
			case m.Contains != "":
				queryParams[name] = m.Contains
			case m.Matches != "":
				queryParams[name] = m.Matches
			}
		}
		if len(queryParams) > 0 {
			matcher.QueryParams = queryParams
		}
	}

	// Body patterns
	applyBodyPatterns(matcher, req.BodyPatterns)
}

// mappingToMock converts a WireMock mapping to a MockConfiguration.
func (i *WireMockImporter) mappingToMock(mapping WireMockMapping, id int, now time.Time) *config.MockConfiguration {
	enabled := true

	// Generate a unique mock ID. Prefer WireMock's own UUID/ID if present,
	// otherwise generate a content-based hash to avoid ID collisions on repeated imports.
	mockID := mapping.UUID
	if mockID == "" {
		mockID = mapping.ID
	}
	if mockID == "" {
		path := mapping.Request.URLPath + mapping.Request.URL + mapping.Request.URLPattern + mapping.Request.URLPathPattern
		h := sha256.Sum256([]byte(fmt.Sprintf("wm-%s-%s-%d-%d", mapping.Request.Method, path, id, now.UnixNano())))
		mockID = "wm_" + hex.EncodeToString(h[:8])
	}

	m := &config.MockConfiguration{
		ID:        mockID,
		Name:      mapping.Name,
		Type:      mock.TypeHTTP,
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

	// Convert request matching into the mock's HTTP matcher.
	applyRequestMatching(m.HTTP.Matcher, mapping.Request)

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

	// Response body — prefer explicit body, then jsonBody, then base64Body
	switch {
	case resp.Body != "":
		m.HTTP.Response.Body = resp.Body
	case resp.JSONBody != nil:
		bodyBytes, _ := json.MarshalIndent(resp.JSONBody, "", "  ")
		m.HTTP.Response.Body = string(bodyBytes)
		if _, ok := m.HTTP.Response.Headers["Content-Type"]; !ok {
			m.HTTP.Response.Headers["Content-Type"] = "application/json"
		}
	case resp.Base64Body != "":
		decoded, err := base64.StdEncoding.DecodeString(resp.Base64Body)
		if err == nil {
			m.HTTP.Response.Body = string(decoded)
		}
	}

	// Note: bodyFileName not supported — would need file system access

	return m
}

// applyBodyPatterns maps WireMock body pattern matchers to mockd matcher fields.
// Later patterns of the same type win; different types can coexist.
func applyBodyPatterns(matcher *mock.HTTPMatcher, patterns []WireMockBodyPattern) {
	for _, bp := range patterns {
		switch {
		case bp.EqualTo != "":
			matcher.BodyEquals = bp.EqualTo
		case bp.EqualToJSON != "":
			matcher.BodyEquals = bp.EqualToJSON
		case bp.Contains != "":
			matcher.BodyContains = bp.Contains
		case bp.Matches != "":
			matcher.BodyPattern = bp.Matches
		}
	}
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
