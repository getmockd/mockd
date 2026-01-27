package portability

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// CURLImporter imports cURL commands.
type CURLImporter struct{}

// Import parses a cURL command and returns a MockCollection.
func (i *CURLImporter) Import(data []byte) (*config.MockCollection, error) {
	cmd := strings.TrimSpace(string(data))

	// Verify it's a curl command
	if !strings.HasPrefix(cmd, "curl ") && !strings.HasPrefix(cmd, "curl\t") {
		return nil, &ImportError{
			Format:  FormatCURL,
			Message: "not a valid cURL command",
		}
	}

	parsed, err := i.parseCURL(cmd)
	if err != nil {
		return nil, &ImportError{
			Format:  FormatCURL,
			Message: "failed to parse cURL command",
			Cause:   err,
		}
	}

	now := time.Now()
	mock := i.parsedToMock(parsed, 1, now)

	return &config.MockCollection{
		Version: "1.0",
		Name:    "Imported from cURL",
		Mocks:   []*config.MockConfiguration{mock},
	}, nil
}

// curlParsed represents parsed cURL command data.
type curlParsed struct {
	method      string
	url         string
	headers     map[string]string
	body        string
	user        string // For basic auth
	contentType string
}

// parseCURL parses a cURL command string.
func (i *CURLImporter) parseCURL(cmd string) (*curlParsed, error) {
	result := &curlParsed{
		method:  "GET",
		headers: make(map[string]string),
	}

	// Tokenize the command, handling quotes
	tokens := tokenizeCURL(cmd)

	// Skip "curl" token
	if len(tokens) > 0 && tokens[0] == "curl" {
		tokens = tokens[1:]
	}

	for idx := 0; idx < len(tokens); idx++ {
		token := tokens[idx]

		switch {
		case token == "-X" || token == "--request":
			if idx+1 < len(tokens) {
				idx++
				result.method = strings.ToUpper(tokens[idx])
			}

		case token == "-H" || token == "--header":
			if idx+1 < len(tokens) {
				idx++
				header := tokens[idx]
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					result.headers[name] = value
					if strings.EqualFold(name, "Content-Type") {
						result.contentType = value
					}
				}
			}

		case token == "-d" || token == "--data" || token == "--data-raw":
			if idx+1 < len(tokens) {
				idx++
				result.body = tokens[idx]
				// If no explicit method and we have data, default to POST
				if result.method == "GET" {
					result.method = "POST"
				}
			}

		case token == "--data-binary":
			if idx+1 < len(tokens) {
				idx++
				result.body = tokens[idx]
				if result.method == "GET" {
					result.method = "POST"
				}
			}

		case token == "-u" || token == "--user":
			if idx+1 < len(tokens) {
				idx++
				result.user = tokens[idx]
			}

		case token == "--json":
			if idx+1 < len(tokens) {
				idx++
				result.body = tokens[idx]
				result.contentType = "application/json"
				if result.method == "GET" {
					result.method = "POST"
				}
			}

		case token == "-G" || token == "--get":
			result.method = "GET"

		case token == "-I" || token == "--head":
			result.method = "HEAD"

		case strings.HasPrefix(token, "-"):
			// Skip other flags and their arguments if any
			// Common flags that take an argument
			flagsWithArgs := map[string]bool{
				"-o": true, "--output": true,
				"-O": true, "--remote-name": true,
				"-A": true, "--user-agent": true,
				"-e": true, "--referer": true,
				"-b": true, "--cookie": true,
				"-c": true, "--cookie-jar": true,
				"-T": true, "--upload-file": true,
				"--connect-timeout": true,
				"-m":                true, "--max-time": true,
			}
			if flagsWithArgs[token] && idx+1 < len(tokens) {
				idx++
			}

		default:
			// This is likely the URL
			if !strings.HasPrefix(token, "-") && result.url == "" {
				result.url = token
			}
		}
	}

	if result.url == "" {
		return nil, fmt.Errorf("no URL found in cURL command")
	}

	// Handle basic auth
	if result.user != "" {
		parts := strings.SplitN(result.user, ":", 2)
		user := parts[0]
		pass := ""
		if len(parts) > 1 {
			pass = parts[1]
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		result.headers["Authorization"] = "Basic " + encoded
	}

	return result, nil
}

// tokenizeCURL tokenizes a cURL command respecting quotes.
func tokenizeCURL(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := rune(0)
	escaped := false

	for _, r := range cmd {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}

		switch r {
		case '"', '\'':
			inQuote = r
		case ' ', '\t', '\n', '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parsedToMock converts parsed cURL data to a MockConfiguration.
func (i *CURLImporter) parsedToMock(parsed *curlParsed, id int, now time.Time) *config.MockConfiguration {
	// Parse URL
	parsedURL, err := url.Parse(parsed.url)
	if err != nil {
		parsedURL = &url.URL{Path: "/"}
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	matcher := &mock.HTTPMatcher{
		Method: parsed.method,
		Path:   path,
	}

	// Add query parameters
	if parsedURL.RawQuery != "" {
		queryParams := make(map[string]string)
		for key, values := range parsedURL.Query() {
			if len(values) > 0 {
				queryParams[key] = values[0]
			}
		}
		if len(queryParams) > 0 {
			matcher.QueryParams = queryParams
		}
	}

	// Add headers for matching (skip common ones)
	skipHeaders := map[string]bool{
		"Content-Type":   true,
		"Content-Length": true,
		"Accept":         true,
		"User-Agent":     true,
	}
	if len(parsed.headers) > 0 {
		matchHeaders := make(map[string]string)
		for name, value := range parsed.headers {
			if !skipHeaders[name] {
				matchHeaders[name] = value
			}
		}
		if len(matchHeaders) > 0 {
			matcher.Headers = matchHeaders
		}
	}

	enabled := true
	m := &config.MockConfiguration{
		ID:        fmt.Sprintf("imported-%d", id),
		Type:      mock.MockTypeHTTP,
		Name:      fmt.Sprintf("%s %s", parsed.method, path),
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher:  matcher,
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"status": "ok"}`,
			},
		},
	}

	return m
}

// Format returns FormatCURL.
func (i *CURLImporter) Format() Format {
	return FormatCURL
}

// init registers the cURL importer.
func init() {
	RegisterImporter(&CURLImporter{})
}
