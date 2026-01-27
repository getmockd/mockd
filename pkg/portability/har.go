package portability

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// HAR (HTTP Archive) types

// HAR represents an HTTP Archive file.
type HAR struct {
	Log HARLog `json:"log"`
}

// HARLog contains the HAR log data.
type HARLog struct {
	Version string     `json:"version"`
	Creator HARCreator `json:"creator"`
	Entries []HAREntry `json:"entries"`
}

// HARCreator contains tool information.
type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// HAREntry represents a single request/response pair.
type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Timings         HARTimings  `json:"timings"`
}

// HARRequest represents an HTTP request.
type HARRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Headers     []HARHeader  `json:"headers"`
	QueryString []HARQuery   `json:"queryString"`
	PostData    *HARPostData `json:"postData,omitempty"`
	HeadersSize int          `json:"headersSize"`
	BodySize    int          `json:"bodySize"`
}

// HARResponse represents an HTTP response.
type HARResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HARHeader `json:"headers"`
	Content     HARContent  `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int         `json:"headersSize"`
	BodySize    int         `json:"bodySize"`
}

// HARHeader represents an HTTP header.
type HARHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARQuery represents a query parameter.
type HARQuery struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARPostData represents POST data.
type HARPostData struct {
	MimeType string     `json:"mimeType"`
	Text     string     `json:"text"`
	Params   []HARParam `json:"params,omitempty"`
}

// HARParam represents a POST parameter.
type HARParam struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// HARContent represents response content.
type HARContent struct {
	Size        int    `json:"size"`
	Compression int    `json:"compression,omitempty"`
	MimeType    string `json:"mimeType"`
	Text        string `json:"text,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
}

// HARTimings represents timing information.
type HARTimings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	SSL     float64 `json:"ssl"`
}

// HARImporter imports HAR (HTTP Archive) format.
type HARImporter struct {
	// IncludeStatic if true, includes static assets (js, css, images, etc.)
	IncludeStatic bool
}

// staticExtensions are file extensions for static assets to filter out by default.
var staticExtensions = map[string]bool{
	".js":    true,
	".css":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".svg":   true,
	".ico":   true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
	".map":   true,
}

// Import parses a HAR file and returns a MockCollection.
func (i *HARImporter) Import(data []byte) (*config.MockCollection, error) {
	var har HAR
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, &ImportError{
			Format:  FormatHAR,
			Message: "failed to parse HAR file",
			Cause:   err,
		}
	}

	// Validate it's a HAR file
	if har.Log.Version == "" {
		return nil, &ImportError{
			Format:  FormatHAR,
			Message: "not a valid HAR file (missing log.version)",
		}
	}

	result := &config.MockCollection{
		Version: "1.0",
		Name:    "Imported from HAR",
		Mocks:   make([]*config.MockConfiguration, 0),
	}

	now := time.Now()
	mockID := 1

	// Group entries by endpoint (method + path)
	type endpointKey struct {
		method string
		path   string
	}
	endpointEntries := make(map[endpointKey][]HAREntry)

	for _, entry := range har.Log.Entries {
		// Skip static assets unless explicitly included
		if !i.IncludeStatic && i.isStaticAsset(entry.Request.URL, entry.Response.Content.MimeType) {
			continue
		}

		// Parse URL to get path
		parsed, err := url.Parse(entry.Request.URL)
		if err != nil {
			continue
		}

		key := endpointKey{
			method: entry.Request.Method,
			path:   parsed.Path,
		}
		endpointEntries[key] = append(endpointEntries[key], entry)
	}

	// Sort keys for deterministic output
	keys := make([]endpointKey, 0, len(endpointEntries))
	for k := range endpointEntries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		return keys[i].method < keys[j].method
	})

	// Create a mock for each unique endpoint (using first entry)
	for _, key := range keys {
		entries := endpointEntries[key]
		if len(entries) == 0 {
			continue
		}

		// Use the first entry as the template
		entry := entries[0]
		mock := i.entryToMock(entry, mockID, now)
		result.Mocks = append(result.Mocks, mock)
		mockID++
	}

	return result, nil
}

// isStaticAsset checks if a request is for a static asset.
func (i *HARImporter) isStaticAsset(requestURL, mimeType string) bool {
	// Check by extension
	parsed, err := url.Parse(requestURL)
	if err == nil {
		ext := strings.ToLower(filepath.Ext(parsed.Path))
		if staticExtensions[ext] {
			return true
		}
	}

	// Check by mime type
	mimeType = strings.ToLower(mimeType)
	staticMimeTypes := []string{
		"text/javascript",
		"application/javascript",
		"text/css",
		"image/",
		"font/",
		"application/font",
	}
	for _, staticMime := range staticMimeTypes {
		if strings.HasPrefix(mimeType, staticMime) {
			return true
		}
	}

	return false
}

// entryToMock converts a HAR entry to a MockConfiguration.
func (i *HARImporter) entryToMock(entry HAREntry, id int, now time.Time) *config.MockConfiguration {
	// Parse URL
	parsed, _ := url.Parse(entry.Request.URL)
	path := parsed.Path
	if path == "" {
		path = "/"
	}

	matcher := &mock.HTTPMatcher{
		Method: entry.Request.Method,
		Path:   path,
	}

	// Add query parameters
	if len(entry.Request.QueryString) > 0 {
		queryParams := make(map[string]string)
		for _, q := range entry.Request.QueryString {
			queryParams[q.Name] = q.Value
		}
		matcher.QueryParams = queryParams
	}

	// Build response headers (excluding some that shouldn't be mocked)
	respHeaders := make(map[string]string)
	excludeHeaders := map[string]bool{
		"content-length":            true,
		"transfer-encoding":         true,
		"connection":                true,
		"date":                      true,
		"server":                    true,
		"x-powered-by":              true,
		"set-cookie":                true,
		"strict-transport-security": true,
	}
	for _, h := range entry.Response.Headers {
		headerLower := strings.ToLower(h.Name)
		if !excludeHeaders[headerLower] {
			respHeaders[h.Name] = h.Value
		}
	}

	// Set content-type if present
	if entry.Response.Content.MimeType != "" {
		respHeaders["Content-Type"] = entry.Response.Content.MimeType
	}

	enabled := true
	m := &config.MockConfiguration{
		ID:        fmt.Sprintf("imported-%d", id),
		Type:      mock.MockTypeHTTP,
		Name:      fmt.Sprintf("%s %s", entry.Request.Method, path),
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher:  matcher,
			Response: &mock.HTTPResponse{
				StatusCode: entry.Response.Status,
				Headers:    respHeaders,
				Body:       entry.Response.Content.Text,
			},
		},
	}

	return m
}

// Format returns FormatHAR.
func (i *HARImporter) Format() Format {
	return FormatHAR
}

// init registers the HAR importer.
func init() {
	RegisterImporter(&HARImporter{})
}
