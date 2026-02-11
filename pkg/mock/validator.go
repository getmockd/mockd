package mock

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/getmockd/mockd/pkg/util"
	"github.com/ohler55/ojg/jp"
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

// Validate checks if the Mock is valid.
func (m *Mock) Validate() error {
	if m.ID == "" {
		return &ValidationError{Field: "id", Message: "id is required"}
	}

	// Validate based on type
	switch m.Type {
	case TypeHTTP:
		return m.validateHTTP()
	case TypeWebSocket:
		return m.validateWebSocket()
	case TypeGraphQL:
		return m.validateGraphQL()
	case TypeGRPC:
		return m.validateGRPC()
	case TypeSOAP:
		return m.validateSOAP()
	case TypeMQTT:
		return m.validateMQTT()
	case TypeOAuth:
		return m.validateOAuth()
	case "":
		// Legacy format - check if HTTP spec is present
		if m.HTTP != nil {
			return m.validateHTTP()
		}
		return &ValidationError{Field: "type", Message: "type is required"}
	default:
		return &ValidationError{Field: "type", Message: fmt.Sprintf("unknown mock type: %s", m.Type)}
	}
}

// validateHTTP validates HTTP mock specifics.
func (m *Mock) validateHTTP() error {
	if m.HTTP == nil {
		return &ValidationError{Field: "http", Message: "http spec is required for HTTP mocks"}
	}

	if m.HTTP.Matcher == nil {
		return &ValidationError{Field: "http.matcher", Message: "matcher is required"}
	}

	if err := m.HTTP.Matcher.Validate(); err != nil {
		return err
	}

	// Count how many response types are configured
	responseTypeCount := 0
	if m.HTTP.Response != nil {
		responseTypeCount++
	}
	if m.HTTP.SSE != nil {
		responseTypeCount++
	}
	if m.HTTP.Chunked != nil {
		responseTypeCount++
	}

	// Exactly one response type must be specified
	if responseTypeCount == 0 {
		return &ValidationError{Field: "http.response", Message: "one of response, sse, or chunked is required"}
	}
	if responseTypeCount > 1 {
		return &ValidationError{Field: "http.response", Message: "only one of response, sse, or chunked may be specified"}
	}

	// Validate the response type that is present
	if m.HTTP.Response != nil {
		if err := m.HTTP.Response.Validate(); err != nil {
			return err
		}
	}

	if m.HTTP.SSE != nil {
		if err := m.HTTP.SSE.Validate(); err != nil {
			return err
		}
	}

	if m.HTTP.Chunked != nil {
		if err := m.HTTP.Chunked.Validate(); err != nil {
			return err
		}
	}

	if m.HTTP.Priority < 0 {
		return &ValidationError{Field: "http.priority", Message: "priority must be >= 0"}
	}

	return nil
}

// validateWebSocket validates WebSocket mock specifics.
func (m *Mock) validateWebSocket() error {
	if m.WebSocket == nil {
		return &ValidationError{Field: "websocket", Message: "websocket spec is required for WebSocket mocks"}
	}

	if m.WebSocket.Path == "" {
		return &ValidationError{Field: "websocket.path", Message: "path is required"}
	}

	if !strings.HasPrefix(m.WebSocket.Path, "/") {
		return &ValidationError{Field: "websocket.path", Message: "path must start with /"}
	}

	return nil
}

// Validate checks if the HTTPMatcher is valid.
func (m *HTTPMatcher) Validate() error {
	// At least one matching criterion must be specified
	hasAnyCriteria := m.Method != "" ||
		m.Path != "" ||
		m.PathPattern != "" ||
		len(m.Headers) > 0 ||
		len(m.QueryParams) > 0 ||
		m.BodyContains != "" ||
		m.BodyEquals != "" ||
		m.BodyPattern != "" ||
		len(m.BodyJSONPath) > 0

	if !hasAnyCriteria {
		return &ValidationError{Field: "matcher", Message: "at least one matching criterion must be specified"}
	}

	// Validate method if specified
	if m.Method != "" {
		method := strings.ToUpper(m.Method)
		if !validHTTPMethods[method] {
			return &ValidationError{
				Field:   "matcher.method",
				Message: "invalid HTTP method: " + m.Method,
			}
		}
	}

	// Validate path if specified
	if m.Path != "" && !strings.HasPrefix(m.Path, "/") {
		return &ValidationError{Field: "matcher.path", Message: "path must start with /"}
	}

	// Path and PathPattern are mutually exclusive
	if m.Path != "" && m.PathPattern != "" {
		return &ValidationError{
			Field:   "matcher",
			Message: "cannot specify both path and pathPattern",
		}
	}

	// Validate PathPattern regex syntax if specified
	if m.PathPattern != "" {
		if _, err := regexp.Compile(m.PathPattern); err != nil {
			return &ValidationError{
				Field:   "matcher.pathPattern",
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	// Validate BodyPattern regex syntax if specified
	if m.BodyPattern != "" {
		if _, err := regexp.Compile(m.BodyPattern); err != nil {
			return &ValidationError{
				Field:   "matcher.bodyPattern",
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	// Validate header names
	for name := range m.Headers {
		if !headerNameRegex.MatchString(name) {
			return &ValidationError{
				Field:   "matcher.headers",
				Message: "invalid header name: " + name,
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

	// Validate JSONPath expressions
	for path := range m.BodyJSONPath {
		if _, err := jp.ParseString(path); err != nil {
			return &ValidationError{
				Field:   "matcher.bodyJsonPath",
				Message: fmt.Sprintf("invalid JSONPath expression %q: %s", path, err.Error()),
			}
		}
	}

	// Validate mTLS matching criteria
	if m.MTLS != nil {
		if err := m.MTLS.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the MTLSMatch is valid.
func (m *MTLSMatch) Validate() error {
	// CN and CNPattern are mutually exclusive
	if m.CN != "" && m.CNPattern != "" {
		return &ValidationError{
			Field:   "matcher.mtls",
			Message: "cannot specify both cn and cnPattern",
		}
	}

	// Validate CNPattern regex syntax if specified
	if m.CNPattern != "" {
		if _, err := regexp.Compile(m.CNPattern); err != nil {
			return &ValidationError{
				Field:   "matcher.mtls.cnPattern",
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	// Validate Fingerprint format if specified (should be 64 hex characters)
	if m.Fingerprint != "" {
		normalized := normalizeFingerprint(m.Fingerprint)
		if len(normalized) != 64 {
			return &ValidationError{
				Field:   "matcher.mtls.fingerprint",
				Message: "fingerprint must be 64 hex characters (SHA256)",
			}
		}
		// Check that it's valid hex
		for _, c := range normalized {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return &ValidationError{
					Field:   "matcher.mtls.fingerprint",
					Message: "fingerprint must contain only hex characters",
				}
			}
		}
	}

	return nil
}

// normalizeFingerprint normalizes a certificate fingerprint for validation.
// Handles various formats: raw hex, sha256: prefix, colons, and case differences.
func normalizeFingerprint(fp string) string {
	// Remove "sha256:" prefix if present
	fp = strings.TrimPrefix(fp, "sha256:")
	fp = strings.TrimPrefix(fp, "SHA256:")

	// Remove colons
	fp = strings.ReplaceAll(fp, ":", "")

	// Convert to lowercase
	return strings.ToLower(fp)
}

// Validate checks if the HTTPResponse is valid.
func (r *HTTPResponse) Validate() error {
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

	// Validate bodyFile path safety (reject traversal and absolute paths)
	if r.BodyFile != "" {
		if _, safe := util.SafeFilePath(r.BodyFile); !safe {
			return &ValidationError{
				Field:   "response.bodyFile",
				Message: "path must be relative and cannot contain '..'",
			}
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
				Message: "invalid header name: " + name,
			}
		}
	}

	return nil
}

// Validate checks if the SSEConfig is valid.
func (s *SSEConfig) Validate() error {
	// Either events, generator, or template must be specified
	hasEvents := len(s.Events) > 0
	hasGenerator := s.Generator != nil
	hasTemplate := s.Template != ""

	// Count how many data sources are specified
	count := 0
	if hasEvents {
		count++
	}
	if hasGenerator {
		count++
	}
	if hasTemplate {
		count++
	}

	if count == 0 {
		return &ValidationError{Field: "sse", Message: "one of events, generator, or template is required"}
	}
	if count > 1 {
		return &ValidationError{Field: "sse", Message: "events, generator, and template are mutually exclusive"}
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

	// Count how many data sources are specified
	count := 0
	if hasData {
		count++
	}
	if hasDataFile {
		count++
	}
	if hasNDJSON {
		count++
	}

	if count == 0 {
		return &ValidationError{Field: "chunked", Message: "one of data, dataFile, or ndjsonItems is required"}
	}
	if count > 1 {
		return &ValidationError{Field: "chunked", Message: "data, dataFile, and ndjsonItems are mutually exclusive"}
	}

	// Validate dataFile path safety (reject traversal and absolute paths)
	if c.DataFile != "" {
		if _, safe := util.SafeFilePath(c.DataFile); !safe {
			return &ValidationError{
				Field:   "chunked.dataFile",
				Message: "path must be relative and cannot contain '..'",
			}
		}
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

// validateGraphQL validates GraphQL mock specifics.
func (m *Mock) validateGraphQL() error {
	if m.GraphQL == nil {
		return &ValidationError{Field: "graphql", Message: "graphql spec is required for GraphQL mocks"}
	}

	if m.GraphQL.Path == "" {
		return &ValidationError{Field: "graphql.path", Message: "path is required"}
	}

	if !strings.HasPrefix(m.GraphQL.Path, "/") {
		return &ValidationError{Field: "graphql.path", Message: "path must start with /"}
	}

	// Either schema or schemaFile must be specified (but not both)
	hasSchema := m.GraphQL.Schema != ""
	hasSchemaFile := m.GraphQL.SchemaFile != ""

	if !hasSchema && !hasSchemaFile {
		return &ValidationError{Field: "graphql", Message: "one of schema or schemaFile is required"}
	}

	if hasSchema && hasSchemaFile {
		return &ValidationError{Field: "graphql", Message: "cannot specify both schema and schemaFile"}
	}

	// Validate schemaFile path against traversal
	if hasSchemaFile {
		if _, safe := util.SafeFilePathAllowAbsolute(m.GraphQL.SchemaFile); !safe {
			return &ValidationError{Field: "graphql.schemaFile", Message: "path cannot contain '..'"}
		}
	}

	return nil
}

// validateGRPC validates gRPC mock specifics.
func (m *Mock) validateGRPC() error {
	if m.GRPC == nil {
		return &ValidationError{Field: "grpc", Message: "grpc spec is required for gRPC mocks"}
	}

	if m.GRPC.Port <= 0 || m.GRPC.Port > 65535 {
		return &ValidationError{Field: "grpc.port", Message: "port must be between 1 and 65535"}
	}

	// At least one proto file must be specified
	hasProtoFile := m.GRPC.ProtoFile != ""
	hasProtoFiles := len(m.GRPC.ProtoFiles) > 0

	if !hasProtoFile && !hasProtoFiles {
		return &ValidationError{Field: "grpc", Message: "one of protoFile or protoFiles is required"}
	}

	if hasProtoFile && hasProtoFiles {
		return &ValidationError{Field: "grpc", Message: "cannot specify both protoFile and protoFiles"}
	}

	// Validate proto file paths against traversal
	if m.GRPC.ProtoFile != "" {
		if _, safe := util.SafeFilePathAllowAbsolute(m.GRPC.ProtoFile); !safe {
			return &ValidationError{Field: "grpc.protoFile", Message: "path cannot contain '..'"}
		}
	}
	for i, pf := range m.GRPC.ProtoFiles {
		if _, safe := util.SafeFilePathAllowAbsolute(pf); !safe {
			return &ValidationError{
				Field:   fmt.Sprintf("grpc.protoFiles[%d]", i),
				Message: "path cannot contain '..'",
			}
		}
	}
	for i, ip := range m.GRPC.ImportPaths {
		if _, safe := util.SafeFilePathAllowAbsolute(ip); !safe {
			return &ValidationError{
				Field:   fmt.Sprintf("grpc.importPaths[%d]", i),
				Message: "path cannot contain '..'",
			}
		}
	}

	return nil
}

// validateSOAP validates SOAP mock specifics.
func (m *Mock) validateSOAP() error {
	if m.SOAP == nil {
		return &ValidationError{Field: "soap", Message: "soap spec is required for SOAP mocks"}
	}

	if m.SOAP.Path == "" {
		return &ValidationError{Field: "soap.path", Message: "path is required"}
	}

	if !strings.HasPrefix(m.SOAP.Path, "/") {
		return &ValidationError{Field: "soap.path", Message: "path must start with /"}
	}

	// WSDL and WSDLFile are mutually exclusive
	hasWSDL := m.SOAP.WSDL != ""
	hasWSDLFile := m.SOAP.WSDLFile != ""

	if hasWSDL && hasWSDLFile {
		return &ValidationError{Field: "soap", Message: "cannot specify both wsdl and wsdlFile"}
	}

	// Validate wsdlFile path against traversal
	if hasWSDLFile {
		if _, safe := util.SafeFilePathAllowAbsolute(m.SOAP.WSDLFile); !safe {
			return &ValidationError{Field: "soap.wsdlFile", Message: "path cannot contain '..'"}
		}
	}

	// Validate operations if present
	for name, op := range m.SOAP.Operations {
		if op.Response == "" && op.Fault == nil {
			return &ValidationError{
				Field:   "soap.operations." + name,
				Message: "operation must have either response or fault",
			}
		}
	}

	return nil
}

// validateMQTT validates MQTT mock specifics.
func (m *Mock) validateMQTT() error {
	if m.MQTT == nil {
		return &ValidationError{Field: "mqtt", Message: "mqtt spec is required for MQTT mocks"}
	}

	if m.MQTT.Port <= 0 || m.MQTT.Port > 65535 {
		return &ValidationError{Field: "mqtt.port", Message: "port must be between 1 and 65535"}
	}

	// Validate TLS config if present
	if m.MQTT.TLS != nil && m.MQTT.TLS.Enabled {
		if m.MQTT.TLS.CertFile == "" {
			return &ValidationError{Field: "mqtt.tls.certFile", Message: "certFile is required when TLS is enabled"}
		}
		if _, ok := util.SafeFilePathAllowAbsolute(m.MQTT.TLS.CertFile); !ok {
			return &ValidationError{Field: "mqtt.tls.certFile", Message: "path cannot contain '..'"}
		}
		if m.MQTT.TLS.KeyFile == "" {
			return &ValidationError{Field: "mqtt.tls.keyFile", Message: "keyFile is required when TLS is enabled"}
		}
		if _, ok := util.SafeFilePathAllowAbsolute(m.MQTT.TLS.KeyFile); !ok {
			return &ValidationError{Field: "mqtt.tls.keyFile", Message: "path cannot contain '..'"}
		}
	}

	// Validate auth config if present
	if m.MQTT.Auth != nil && m.MQTT.Auth.Enabled {
		if len(m.MQTT.Auth.Users) == 0 {
			return &ValidationError{Field: "mqtt.auth.users", Message: "at least one user is required when auth is enabled"}
		}
		for i, user := range m.MQTT.Auth.Users {
			if user.Username == "" {
				return &ValidationError{
					Field:   fmt.Sprintf("mqtt.auth.users[%d].username", i),
					Message: "username is required",
				}
			}
		}
	}

	// Validate topics if present
	for i, topic := range m.MQTT.Topics {
		if topic.Topic == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("mqtt.topics[%d].topic", i),
				Message: "topic is required",
			}
		}
		if topic.QoS < 0 || topic.QoS > 2 {
			return &ValidationError{
				Field:   fmt.Sprintf("mqtt.topics[%d].qos", i),
				Message: "qos must be 0, 1, or 2",
			}
		}
	}

	return nil
}

// validateOAuth validates OAuth mock specifics.
func (m *Mock) validateOAuth() error {
	if m.OAuth == nil {
		return &ValidationError{Field: "oauth", Message: "oauth spec is required for OAuth mocks"}
	}

	if m.OAuth.Issuer == "" {
		return &ValidationError{Field: "oauth.issuer", Message: "issuer is required"}
	}

	// Validate at least one client is configured
	if len(m.OAuth.Clients) == 0 {
		return &ValidationError{Field: "oauth.clients", Message: "at least one client must be configured"}
	}

	// Validate each client has clientId
	for i, client := range m.OAuth.Clients {
		if client.ClientID == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("oauth.clients[%d].clientId", i),
				Message: "clientId is required",
			}
		}
	}

	return nil
}
