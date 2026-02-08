package portability

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// Postman Collection v2.x types

// PostmanCollection represents a Postman Collection v2.x.
type PostmanCollection struct {
	Info     PostmanInfo       `json:"info"`
	Item     []PostmanItem     `json:"item"`
	Variable []PostmanVariable `json:"variable,omitempty"`
}

// PostmanInfo contains collection metadata.
type PostmanInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schema      string `json:"schema"`
}

// PostmanItem represents an item in the collection (request or folder).
type PostmanItem struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Request     *PostmanRequest   `json:"request,omitempty"`
	Response    []PostmanResponse `json:"response,omitempty"`
	Item        []PostmanItem     `json:"item,omitempty"` // Nested items (folders)
}

// PostmanRequest represents a Postman request.
type PostmanRequest struct {
	Method string          `json:"method"`
	URL    PostmanURL      `json:"url"`
	Header []PostmanHeader `json:"header,omitempty"`
	Body   *PostmanBody    `json:"body,omitempty"`
	Auth   *PostmanAuth    `json:"auth,omitempty"`
}

// PostmanURL represents a URL in Postman format.
type PostmanURL struct {
	Raw      string         `json:"raw,omitempty"`
	Protocol string         `json:"protocol,omitempty"`
	Host     []string       `json:"host,omitempty"`
	Path     []string       `json:"path,omitempty"`
	Query    []PostmanQuery `json:"query,omitempty"`
}

// PostmanQuery represents a query parameter.
type PostmanQuery struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// PostmanHeader represents a request header.
type PostmanHeader struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// PostmanBody represents a request body.
type PostmanBody struct {
	Mode       string            `json:"mode"`
	Raw        string            `json:"raw,omitempty"`
	URLEncoded []PostmanQuery    `json:"urlencoded,omitempty"`
	FormData   []PostmanFormData `json:"formdata,omitempty"`
}

// PostmanFormData represents form data.
type PostmanFormData struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Type     string `json:"type,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// PostmanAuth represents authentication configuration.
type PostmanAuth struct {
	Type   string `json:"type"`
	Bearer []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"bearer,omitempty"`
}

// PostmanResponse represents a saved response.
type PostmanResponse struct {
	Name   string          `json:"name"`
	Status string          `json:"status,omitempty"`
	Code   int             `json:"code,omitempty"`
	Header []PostmanHeader `json:"header,omitempty"`
	Body   string          `json:"body,omitempty"`
}

// PostmanVariable represents a collection variable.
type PostmanVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

// PostmanImporter imports Postman Collection v2.x format.
type PostmanImporter struct{}

// Import parses a Postman Collection and returns a MockCollection.
func (i *PostmanImporter) Import(data []byte) (*config.MockCollection, error) {
	var collection PostmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, &ImportError{
			Format:  FormatPostman,
			Message: "failed to parse Postman Collection",
			Cause:   err,
		}
	}

	// Validate it's a Postman collection
	if collection.Info.Schema == "" || !strings.Contains(collection.Info.Schema, "postman") {
		return nil, &ImportError{
			Format:  FormatPostman,
			Message: "not a valid Postman Collection v2.x",
		}
	}

	result := &config.MockCollection{
		Version: "1.0",
		Name:    collection.Info.Name,
		Mocks:   make([]*config.MockConfiguration, 0),
	}

	// Build variable map for substitution
	variables := make(map[string]string)
	for _, v := range collection.Variable {
		variables[v.Key] = v.Value
	}

	now := time.Now()
	mockID := 1

	// Process all items recursively
	i.processItems(collection.Item, variables, result, &mockID, now)

	return result, nil
}

// processItems recursively processes Postman items (requests and folders).
func (i *PostmanImporter) processItems(items []PostmanItem, variables map[string]string, result *config.MockCollection, mockID *int, now time.Time) {
	for _, item := range items {
		// If this is a folder, process nested items
		if len(item.Item) > 0 {
			i.processItems(item.Item, variables, result, mockID, now)
			continue
		}

		// Skip if no request
		if item.Request == nil {
			continue
		}

		mock := i.requestToMock(item, variables, *mockID, now)
		result.Mocks = append(result.Mocks, mock)
		*mockID++
	}
}

// requestToMock converts a Postman request to a MockConfiguration.
func (i *PostmanImporter) requestToMock(item PostmanItem, variables map[string]string, id int, now time.Time) *config.MockConfiguration {
	req := item.Request

	// Extract path from URL
	path := i.extractPath(req.URL, variables)

	enabled := true
	m := &config.MockConfiguration{
		ID:        fmt.Sprintf("imported-%d", id),
		Name:      item.Name,
		Type:      mock.TypeHTTP,
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: strings.ToUpper(req.Method),
				Path:   path,
			},
		},
	}

	// Add query parameters
	queryParams := make(map[string]string)
	for _, q := range req.URL.Query {
		if !q.Disabled {
			queryParams[q.Key] = i.substituteVariables(q.Value, variables)
		}
	}
	if len(queryParams) > 0 {
		m.HTTP.Matcher.QueryParams = queryParams
	}

	// Add headers for matching
	headers := make(map[string]string)
	for _, h := range req.Header {
		if !h.Disabled {
			// Skip common headers that shouldn't be used for matching
			key := strings.ToLower(h.Key)
			if key != "content-type" && key != "content-length" && key != "accept" {
				headers[h.Key] = i.substituteVariables(h.Value, variables)
			}
		}
	}
	if len(headers) > 0 {
		m.HTTP.Matcher.Headers = headers
	}

	// Use saved response if available, otherwise generate default
	if len(item.Response) > 0 {
		resp := item.Response[0] // Use first saved response
		m.HTTP.Response = &mock.HTTPResponse{
			StatusCode: resp.Code,
			Headers:    make(map[string]string),
			Body:       i.substituteVariables(resp.Body, variables),
		}
		if m.HTTP.Response.StatusCode == 0 {
			m.HTTP.Response.StatusCode = 200
		}
		for _, h := range resp.Header {
			m.HTTP.Response.Headers[h.Key] = h.Value
		}
	} else {
		// Generate default response
		m.HTTP.Response = &mock.HTTPResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"status": "ok"}`,
		}
	}

	return m
}

// extractPath extracts the URL path from a Postman URL.
func (i *PostmanImporter) extractPath(postmanURL PostmanURL, variables map[string]string) string {
	// If path parts are available, use them
	if len(postmanURL.Path) > 0 {
		parts := make([]string, len(postmanURL.Path))
		for idx, part := range postmanURL.Path {
			substituted := i.substituteVariables(part, variables)
			// Convert :param to mockd format
			if strings.HasPrefix(substituted, ":") {
				parts[idx] = substituted
			} else {
				parts[idx] = substituted
			}
		}
		return "/" + strings.Join(parts, "/")
	}

	// Fall back to parsing raw URL
	if postmanURL.Raw != "" {
		raw := i.substituteVariables(postmanURL.Raw, variables)
		parsed, err := url.Parse(raw)
		if err == nil {
			return parsed.Path
		}
	}

	return "/"
}

// substituteVariables replaces Postman variables {{var}} with their values.
func (i *PostmanImporter) substituteVariables(s string, variables map[string]string) string {
	result := s
	for key, value := range variables {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

// Format returns FormatPostman.
func (i *PostmanImporter) Format() Format {
	return FormatPostman
}

// init registers the Postman importer.
func init() {
	RegisterImporter(&PostmanImporter{})
}
