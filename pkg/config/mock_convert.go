package config

import (
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
)

// ConvertMockEntry converts a config.MockEntry (YAML schema) to a mock.Mock (runtime type).
// Only inline HTTP mocks are currently supported. File references and globs should be
// resolved before calling this function.
func ConvertMockEntry(entry MockEntry) (*mock.Mock, error) {
	if !entry.IsInline() {
		return nil, fmt.Errorf("only inline mock entries can be converted; got file=%q files=%q", entry.File, entry.Files)
	}

	now := time.Now()
	enabled := true

	m := &mock.Mock{
		ID:          entry.ID,
		Type:        mock.MockType(entry.Type),
		Enabled:     &enabled,
		WorkspaceID: entry.Workspace,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Generate ID if empty using type prefix
	if m.ID == "" {
		m.ID = generateIDForType(m.Type)
	}

	// Default type to HTTP
	if m.Type == "" {
		m.Type = mock.MockTypeHTTP
	}

	// Convert HTTP-specific config
	if entry.HTTP != nil {
		spec, err := convertHTTPMockConfig(entry.HTTP)
		if err != nil {
			return nil, fmt.Errorf("converting HTTP config for mock %q: %w", m.ID, err)
		}
		m.HTTP = spec
	}

	return m, nil
}

// ConvertMockEntries converts a slice of MockEntry to a slice of mock.Mock.
// Only inline entries are converted; file/glob entries are skipped with a warning.
func ConvertMockEntries(entries []MockEntry) ([]*mock.Mock, []error) {
	var mocks []*mock.Mock
	var errs []error

	for _, entry := range entries {
		if !entry.IsInline() {
			continue
		}
		m, err := ConvertMockEntry(entry)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		mocks = append(mocks, m)
	}

	return mocks, errs
}

// convertHTTPMockConfig converts a config.HTTPMockConfig to a mock.HTTPSpec.
func convertHTTPMockConfig(cfg *HTTPMockConfig) (*mock.HTTPSpec, error) {
	spec := &mock.HTTPSpec{
		Matcher: &mock.HTTPMatcher{
			Method:      cfg.Matcher.Method,
			Path:        cfg.Matcher.Path,
			PathPattern: cfg.Matcher.PathPattern,
		},
		Response: &mock.HTTPResponse{
			StatusCode: cfg.Response.StatusCode,
			Body:       cfg.Response.Body,
			BodyFile:   cfg.Response.BodyFile,
		},
	}

	// Default status code to 200
	if spec.Response.StatusCode == 0 {
		spec.Response.StatusCode = 200
	}

	// Copy matcher headers
	if len(cfg.Matcher.Headers) > 0 {
		spec.Matcher.Headers = make(map[string]string, len(cfg.Matcher.Headers))
		for k, v := range cfg.Matcher.Headers {
			spec.Matcher.Headers[k] = v
		}
	}

	// Copy matcher query params
	if len(cfg.Matcher.QueryParams) > 0 {
		spec.Matcher.QueryParams = make(map[string]string, len(cfg.Matcher.QueryParams))
		for k, v := range cfg.Matcher.QueryParams {
			spec.Matcher.QueryParams[k] = v
		}
	}

	// Copy response headers
	if len(cfg.Response.Headers) > 0 {
		spec.Response.Headers = make(map[string]string, len(cfg.Response.Headers))
		for k, v := range cfg.Response.Headers {
			spec.Response.Headers[k] = v
		}
	}

	// Convert delay string to DelayMs int
	if cfg.Response.Delay != "" {
		d, err := time.ParseDuration(cfg.Response.Delay)
		if err != nil {
			return nil, fmt.Errorf("invalid delay %q: %w", cfg.Response.Delay, err)
		}
		spec.Response.DelayMs = int(d.Milliseconds())
	}

	return spec, nil
}

// generateIDForType generates a prefixed mock ID based on the mock type.
func generateIDForType(t mock.MockType) string {
	prefix := "mock"
	switch t {
	case mock.MockTypeHTTP:
		prefix = "http"
	case mock.MockTypeWebSocket:
		prefix = "ws"
	case mock.MockTypeGraphQL:
		prefix = "gql"
	case mock.MockTypeGRPC:
		prefix = "grpc"
	case mock.MockTypeSOAP:
		prefix = "soap"
	case mock.MockTypeMQTT:
		prefix = "mqtt"
	case mock.MockTypeOAuth:
		prefix = "oauth"
	}
	return fmt.Sprintf("%s_%x", prefix, time.Now().UnixNano())
}
