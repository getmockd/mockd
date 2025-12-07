package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
)

// stringSliceFlag is a custom flag type that can be specified multiple times.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// RunAdd handles the add command.
func RunAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)

	// Method and path
	method := fs.String("method", "GET", "HTTP method to match")
	fs.StringVar(method, "m", "GET", "HTTP method to match (shorthand)")
	path := fs.String("path", "", "URL path to match (required)")

	// Response settings
	status := fs.Int("status", 200, "Response status code")
	fs.IntVar(status, "s", 200, "Response status code (shorthand)")
	body := fs.String("body", "", "Response body")
	fs.StringVar(body, "b", "", "Response body (shorthand)")
	bodyFile := fs.String("body-file", "", "Read response body from file")

	// Response headers (repeatable)
	var headers stringSliceFlag
	fs.Var(&headers, "header", "Response header (key:value), repeatable")
	fs.Var(&headers, "H", "Response header (key:value), repeatable (shorthand)")

	// Request matching (repeatable)
	var matchHeaders stringSliceFlag
	fs.Var(&matchHeaders, "match-header", "Required request header (key:value), repeatable")
	var matchQueries stringSliceFlag
	fs.Var(&matchQueries, "match-query", "Required query param (key:value), repeatable")

	// Mock metadata
	name := fs.String("name", "", "Mock display name")
	fs.StringVar(name, "n", "", "Mock display name (shorthand)")
	priority := fs.Int("priority", 0, "Mock priority (higher = matched first)")
	delay := fs.Int("delay", 0, "Response delay in milliseconds")

	// Admin URL and output format
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd add [flags]

Add a new mock endpoint.

Flags:
  -m, --method        HTTP method to match (default: GET)
      --path          URL path to match (required)
  -s, --status        Response status code (default: 200)
  -b, --body          Response body
      --body-file     Read response body from file
  -H, --header        Response header (key:value), repeatable
      --match-header  Required request header (key:value), repeatable
      --match-query   Required query param (key:value), repeatable
  -n, --name          Mock display name
      --priority      Mock priority (higher = matched first)
      --delay         Response delay in milliseconds
      --admin-url     Admin API base URL (default: http://localhost:9090)
      --json          Output in JSON format

Examples:
  # Simple GET mock
  mockd add --path /api/users --status 200 --body '[]'

  # POST with JSON response
  mockd add -m POST --path /api/users -s 201 \
    -b '{"id": "new-id", "created": true}' \
    -H "Content-Type:application/json"

  # Mock with body from file
  mockd add --path /api/products --body-file products.json

  # Mock with request matching
  mockd add --path /api/users \
    --match-header "Authorization:Bearer *" \
    --match-query "limit:10"
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required fields
	if *path == "" {
		return fmt.Errorf(`--path is required

Usage: mockd add --path /api/endpoint [flags]

Run 'mockd add --help' for more options`)
	}

	// Read body from file if specified
	responseBody := *body
	if *bodyFile != "" {
		data, err := os.ReadFile(*bodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		responseBody = string(data)
	}

	// Parse response headers
	responseHeaders := make(map[string]string)
	for _, h := range headers {
		key, value, ok := parseKeyValue(h)
		if !ok {
			return fmt.Errorf("invalid header format: %s (expected key:value)", h)
		}
		responseHeaders[key] = value
	}

	// Parse match headers
	matchHeadersMap := make(map[string]string)
	for _, h := range matchHeaders {
		key, value, ok := parseKeyValue(h)
		if !ok {
			return fmt.Errorf("invalid match-header format: %s (expected key:value)", h)
		}
		matchHeadersMap[key] = value
	}

	// Parse match query params
	matchQueryMap := make(map[string]string)
	for _, q := range matchQueries {
		key, value, ok := parseKeyValue(q)
		if !ok {
			return fmt.Errorf("invalid match-query format: %s (expected key:value)", q)
		}
		matchQueryMap[key] = value
	}

	// Build mock configuration
	mock := &config.MockConfiguration{
		Name:     *name,
		Priority: *priority,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: strings.ToUpper(*method),
			Path:   *path,
		},
		Response: &config.ResponseDefinition{
			StatusCode: *status,
			Body:       responseBody,
			DelayMs:    *delay,
		},
	}

	// Add optional matchers
	if len(matchHeadersMap) > 0 {
		mock.Matcher.Headers = matchHeadersMap
	}
	if len(matchQueryMap) > 0 {
		mock.Matcher.QueryParams = matchQueryMap
	}

	// Add response headers
	if len(responseHeaders) > 0 {
		mock.Response.Headers = responseHeaders
	}

	// Create admin client and add mock
	client := NewAdminClient(*adminURL)
	created, err := client.CreateMock(mock)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			ID         string `json:"id"`
			Method     string `json:"method"`
			Path       string `json:"path"`
			StatusCode int    `json:"statusCode"`
		}{
			ID:         created.ID,
			Method:     created.Matcher.Method,
			Path:       created.Matcher.Path,
			StatusCode: created.Response.StatusCode,
		})
	}

	fmt.Printf("Created mock: %s\n", created.ID)
	fmt.Printf("  Method: %s\n", created.Matcher.Method)
	fmt.Printf("  Path:   %s\n", created.Matcher.Path)
	fmt.Printf("  Status: %d\n", created.Response.StatusCode)

	return nil
}

// parseKeyValue parses a "key:value" string.
func parseKeyValue(s string) (key, value string, ok bool) {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
