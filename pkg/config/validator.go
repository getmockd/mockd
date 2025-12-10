package config

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a validation failure with context.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// validHTTPMethods are the allowed HTTP methods.
var validHTTPMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"PATCH":   true,
	"HEAD":    true,
	"OPTIONS": true,
}

// headerNameRegex validates HTTP header names (RFC 7230).
var headerNameRegex = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+\-.^_\x60|~]+$`)

// Validate checks if the MockConfiguration is valid.
func (m *MockConfiguration) Validate() error {
	if m.ID == "" {
		return &ValidationError{Field: "id", Message: "id is required"}
	}

	if m.Matcher == nil {
		return &ValidationError{Field: "matcher", Message: "matcher is required"}
	}

	if err := m.Matcher.Validate(); err != nil {
		return err
	}

	// Count how many response types are configured
	responseTypeCount := 0
	if m.Response != nil {
		responseTypeCount++
	}
	if m.SSE != nil {
		responseTypeCount++
	}
	if m.Chunked != nil {
		responseTypeCount++
	}

	// Exactly one response type must be specified
	if responseTypeCount == 0 {
		return &ValidationError{Field: "response", Message: "one of response, sse, or chunked is required"}
	}
	if responseTypeCount > 1 {
		return &ValidationError{Field: "response", Message: "only one of response, sse, or chunked may be specified"}
	}

	// Validate the response type that is present
	if m.Response != nil {
		if err := m.Response.Validate(); err != nil {
			return err
		}
	}

	if m.SSE != nil {
		if err := m.SSE.Validate(); err != nil {
			return err
		}
	}

	if m.Chunked != nil {
		if err := m.Chunked.Validate(); err != nil {
			return err
		}
	}

	if m.Priority < 0 {
		return &ValidationError{Field: "priority", Message: "priority must be >= 0"}
	}

	return nil
}

// Validate checks if the SSEConfig is valid.
func (s *SSEConfig) Validate() error {
	// Either events, generator, or template must be specified
	hasEvents := len(s.Events) > 0
	hasGenerator := s.Generator != nil
	hasTemplate := s.Template != ""

	if !hasEvents && !hasGenerator && !hasTemplate {
		return &ValidationError{Field: "sse", Message: "one of events, generator, or template is required"}
	}

	// Events and generator are mutually exclusive
	if hasEvents && hasGenerator {
		return &ValidationError{Field: "sse", Message: "events and generator are mutually exclusive"}
	}

	// Validate each event if present
	for i, event := range s.Events {
		if event.Data == nil {
			return &ValidationError{Field: fmt.Sprintf("sse.events[%d].data", i), Message: "data is required"}
		}
	}

	// Validate timing config
	if s.Timing.RandomDelay != nil {
		if s.Timing.RandomDelay.Min < 0 {
			return &ValidationError{Field: "sse.timing.randomDelay.min", Message: "min must be >= 0"}
		}
		if s.Timing.RandomDelay.Max < s.Timing.RandomDelay.Min {
			return &ValidationError{Field: "sse.timing.randomDelay.max", Message: "max must be >= min"}
		}
	}

	// Validate lifecycle config
	if s.Lifecycle.KeepaliveInterval != 0 && s.Lifecycle.KeepaliveInterval < 5 {
		return &ValidationError{Field: "sse.lifecycle.keepaliveInterval", Message: "must be 0 (disabled) or >= 5 seconds"}
	}

	// Validate rate limit config
	if s.RateLimit != nil {
		if s.RateLimit.EventsPerSecond <= 0 {
			return &ValidationError{Field: "sse.rateLimit.eventsPerSecond", Message: "must be > 0"}
		}
	}

	// Validate resume config
	if s.Resume.Enabled && s.Resume.BufferSize <= 0 {
		return &ValidationError{Field: "sse.resume.bufferSize", Message: "must be > 0 when resume is enabled"}
	}

	return nil
}

// Validate checks if the ChunkedConfig is valid.
func (c *ChunkedConfig) Validate() error {
	// Either data, dataFile, or ndjsonItems must be specified
	hasData := c.Data != ""
	hasDataFile := c.DataFile != ""
	hasNDJSON := len(c.NDJSONItems) > 0

	if !hasData && !hasDataFile && !hasNDJSON {
		return &ValidationError{Field: "chunked", Message: "one of data, dataFile, or ndjsonItems is required"}
	}

	// Data and dataFile are mutually exclusive
	if hasData && hasDataFile {
		return &ValidationError{Field: "chunked", Message: "data and dataFile are mutually exclusive"}
	}

	// Validate chunk size
	if c.ChunkSize < 0 {
		return &ValidationError{Field: "chunked.chunkSize", Message: "must be >= 0"}
	}

	// Validate chunk delay
	if c.ChunkDelay < 0 {
		return &ValidationError{Field: "chunked.chunkDelay", Message: "must be >= 0"}
	}

	return nil
}

// Validate checks if the RequestMatcher is valid.
func (m *RequestMatcher) Validate() error {
	// At least one matching criterion must be specified
	hasAnyCriteria := m.Method != "" ||
		m.Path != "" ||
		m.PathPattern != "" ||
		len(m.Headers) > 0 ||
		len(m.QueryParams) > 0 ||
		m.BodyContains != "" ||
		m.BodyEquals != ""

	if !hasAnyCriteria {
		return &ValidationError{Field: "matcher", Message: "at least one matching criterion must be specified"}
	}

	// Validate method if specified
	if m.Method != "" {
		method := strings.ToUpper(m.Method)
		if !validHTTPMethods[method] {
			return &ValidationError{
				Field:   "matcher.method",
				Message: fmt.Sprintf("invalid HTTP method: %s", m.Method),
			}
		}
	}

	// Validate path if specified
	if m.Path != "" && !strings.HasPrefix(m.Path, "/") {
		return &ValidationError{Field: "matcher.path", Message: "path must start with /"}
	}

	// Validate header names
	for name := range m.Headers {
		if !headerNameRegex.MatchString(name) {
			return &ValidationError{
				Field:   "matcher.headers",
				Message: fmt.Sprintf("invalid header name: %s", name),
			}
		}
	}

	// Cannot specify both BodyEquals and BodyContains
	if m.BodyEquals != "" && m.BodyContains != "" {
		return &ValidationError{
			Field:   "matcher",
			Message: "cannot specify both bodyEquals and bodyContains",
		}
	}

	return nil
}

// Validate checks if the ResponseDefinition is valid.
func (r *ResponseDefinition) Validate() error {
	// StatusCode must be valid HTTP status code (100-599)
	if r.StatusCode < 100 || r.StatusCode > 599 {
		return &ValidationError{
			Field:   "response.statusCode",
			Message: fmt.Sprintf("statusCode must be between 100-599, got %d", r.StatusCode),
		}
	}

	// Cannot specify both Body and BodyFile
	if r.Body != "" && r.BodyFile != "" {
		return &ValidationError{
			Field:   "response",
			Message: "cannot specify both body and bodyFile",
		}
	}

	// DelayMs must be >= 0 and <= 30000
	if r.DelayMs < 0 {
		return &ValidationError{Field: "response.delayMs", Message: "delayMs must be >= 0"}
	}
	if r.DelayMs > 30000 {
		return &ValidationError{Field: "response.delayMs", Message: "delayMs must be <= 30000 (30 seconds)"}
	}

	// Validate header names
	for name := range r.Headers {
		if !headerNameRegex.MatchString(name) {
			return &ValidationError{
				Field:   "response.headers",
				Message: fmt.Sprintf("invalid header name: %s", name),
			}
		}
	}

	return nil
}

// Validate checks if the ServerConfiguration is valid.
func (s *ServerConfiguration) Validate() error {
	// At least one of HTTPPort or HTTPSPort must be > 0
	if s.HTTPPort <= 0 && s.HTTPSPort <= 0 {
		return &ValidationError{
			Field:   "serverConfig",
			Message: "at least one of httpPort or httpsPort must be > 0",
		}
	}

	// AdminPort must be > 0 and < 65536
	if s.AdminPort <= 0 || s.AdminPort >= 65536 {
		return &ValidationError{
			Field:   "serverConfig.adminPort",
			Message: "adminPort must be between 1 and 65535",
		}
	}

	// Validate port values
	if s.HTTPPort < 0 || s.HTTPPort >= 65536 {
		return &ValidationError{
			Field:   "serverConfig.httpPort",
			Message: "httpPort must be between 0 and 65535",
		}
	}

	if s.HTTPSPort < 0 || s.HTTPSPort >= 65536 {
		return &ValidationError{
			Field:   "serverConfig.httpsPort",
			Message: "httpsPort must be between 0 and 65535",
		}
	}

	// Ports must not conflict (all different if > 0)
	ports := make(map[int]string)
	if s.HTTPPort > 0 {
		ports[s.HTTPPort] = "httpPort"
	}
	if s.HTTPSPort > 0 {
		if name, exists := ports[s.HTTPSPort]; exists {
			return &ValidationError{
				Field:   "serverConfig",
				Message: fmt.Sprintf("httpsPort conflicts with %s (both are %d)", name, s.HTTPSPort),
			}
		}
		ports[s.HTTPSPort] = "httpsPort"
	}
	if s.AdminPort > 0 {
		if name, exists := ports[s.AdminPort]; exists {
			return &ValidationError{
				Field:   "serverConfig",
				Message: fmt.Sprintf("adminPort conflicts with %s (both are %d)", name, s.AdminPort),
			}
		}
	}

	// If HTTPSPort > 0 and AutoGenerateCert = false, CertFile and KeyFile must be provided
	if s.HTTPSPort > 0 && !s.AutoGenerateCert {
		if s.CertFile == "" {
			return &ValidationError{
				Field:   "serverConfig.certFile",
				Message: "certFile is required when httpsPort is set and autoGenerateCert is false",
			}
		}
		if s.KeyFile == "" {
			return &ValidationError{
				Field:   "serverConfig.keyFile",
				Message: "keyFile is required when httpsPort is set and autoGenerateCert is false",
			}
		}
	}

	// MaxBodySize must be > 0 and <= 100MB
	if s.MaxBodySize < 0 {
		return &ValidationError{Field: "serverConfig.maxBodySize", Message: "maxBodySize must be >= 0"}
	}
	if s.MaxBodySize > 100*1024*1024 {
		return &ValidationError{
			Field:   "serverConfig.maxBodySize",
			Message: "maxBodySize must be <= 104857600 (100MB)",
		}
	}

	// MaxLogEntries must be >= 0
	if s.MaxLogEntries < 0 {
		return &ValidationError{Field: "serverConfig.maxLogEntries", Message: "maxLogEntries must be >= 0"}
	}

	// Timeouts must be >= 0
	if s.ReadTimeout < 0 {
		return &ValidationError{Field: "serverConfig.readTimeout", Message: "readTimeout must be >= 0"}
	}
	if s.WriteTimeout < 0 {
		return &ValidationError{Field: "serverConfig.writeTimeout", Message: "writeTimeout must be >= 0"}
	}

	return nil
}

// Validate checks if the MockCollection is valid.
func (c *MockCollection) Validate() error {
	// Version must be "1.0" (only supported version initially)
	if c.Version != "1.0" {
		return &ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version: %s (expected 1.0)", c.Version),
		}
	}

	// Check for duplicate IDs
	ids := make(map[string]bool)
	for i, mock := range c.Mocks {
		if mock == nil {
			return &ValidationError{
				Field:   fmt.Sprintf("mocks[%d]", i),
				Message: "mock cannot be null",
			}
		}
		if err := mock.Validate(); err != nil {
			return &ValidationError{
				Field:   fmt.Sprintf("mocks[%d]", i),
				Message: err.Error(),
			}
		}
		if ids[mock.ID] {
			return &ValidationError{
				Field:   fmt.Sprintf("mocks[%d].id", i),
				Message: fmt.Sprintf("duplicate mock ID: %s", mock.ID),
			}
		}
		ids[mock.ID] = true
	}

	// Validate ServerConfig if present
	if c.ServerConfig != nil {
		if err := c.ServerConfig.Validate(); err != nil {
			return err
		}
	}

	return nil
}
