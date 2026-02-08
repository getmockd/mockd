package portability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"gopkg.in/yaml.v3"
)

// NativeV1 represents the Mockd native format version 1.0.
// This is the canonical portable format for mock definitions.
type NativeV1 struct {
	Version  string           `json:"version" yaml:"version"`
	Kind     string           `json:"kind" yaml:"kind"`
	Metadata NativeV1Metadata `json:"metadata" yaml:"metadata"`
	Settings NativeV1Settings `json:"settings,omitempty" yaml:"settings,omitempty"`
	// Endpoints is the list of mock endpoint definitions
	Endpoints []NativeV1Endpoint `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	// Scenarios define behavior variations triggered by headers
	Scenarios []NativeV1Scenario `json:"scenarios,omitempty" yaml:"scenarios,omitempty"`
	// Stateful defines stateful CRUD resources
	Stateful []NativeV1StatefulResource `json:"stateful,omitempty" yaml:"stateful,omitempty"`
	// WebSockets defines WebSocket endpoints
	WebSockets []NativeV1WebSocket `json:"websockets,omitempty" yaml:"websockets,omitempty"`
}

// NativeV1Metadata contains collection metadata.
type NativeV1Metadata struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// NativeV1Settings contains collection-level settings.
type NativeV1Settings struct {
	BaseURL      string          `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	DefaultDelay int             `json:"defaultDelay,omitempty" yaml:"defaultDelay,omitempty"`
	CORS         *NativeV1CORS   `json:"cors,omitempty" yaml:"cors,omitempty"`
	Logging      bool            `json:"logging,omitempty" yaml:"logging,omitempty"`
	Server       *NativeV1Server `json:"server,omitempty" yaml:"server,omitempty"`
}

// NativeV1CORS defines CORS settings.
type NativeV1CORS struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Origins []string `json:"origins,omitempty" yaml:"origins,omitempty"`
}

// NativeV1Server defines server configuration.
type NativeV1Server struct {
	HTTPPort         int  `json:"httpPort,omitempty" yaml:"httpPort,omitempty"`
	HTTPSPort        int  `json:"httpsPort,omitempty" yaml:"httpsPort,omitempty"`
	AdminPort        int  `json:"adminPort,omitempty" yaml:"adminPort,omitempty"`
	ReadTimeout      int  `json:"readTimeout,omitempty" yaml:"readTimeout,omitempty"`
	WriteTimeout     int  `json:"writeTimeout,omitempty" yaml:"writeTimeout,omitempty"`
	MaxLogEntries    int  `json:"maxLogEntries,omitempty" yaml:"maxLogEntries,omitempty"`
	AutoGenerateCert bool `json:"autoGenerateCert,omitempty" yaml:"autoGenerateCert,omitempty"`
}

// NativeV1Endpoint represents a single mock endpoint.
type NativeV1Endpoint struct {
	ID       string                 `json:"id" yaml:"id"`
	Name     string                 `json:"name,omitempty" yaml:"name,omitempty"`
	Method   string                 `json:"method" yaml:"method"`
	Path     string                 `json:"path" yaml:"path"`
	Request  *NativeV1Request       `json:"request,omitempty" yaml:"request,omitempty"`
	Response *NativeV1Response      `json:"response,omitempty" yaml:"response,omitempty"`
	SSE      *NativeV1SSE           `json:"sse,omitempty" yaml:"sse,omitempty"`
	Chunked  *NativeV1Chunked       `json:"chunked,omitempty" yaml:"chunked,omitempty"`
	Match    *NativeV1MatchSettings `json:"match,omitempty" yaml:"match,omitempty"`
	Enabled  bool                   `json:"enabled" yaml:"enabled"`
}

// NativeV1Request defines request matching criteria.
type NativeV1Request struct {
	Headers      map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Query        map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	BodyContains string            `json:"bodyContains,omitempty" yaml:"bodyContains,omitempty"`
	BodyEquals   string            `json:"bodyEquals,omitempty" yaml:"bodyEquals,omitempty"`
	BodyPattern  string            `json:"bodyPattern,omitempty" yaml:"bodyPattern,omitempty"`
}

// NativeV1Response defines the response to return.
type NativeV1Response struct {
	Status  int               `json:"status" yaml:"status"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty" yaml:"body,omitempty"`
	Delay   int               `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// NativeV1SSE defines SSE streaming response configuration.
type NativeV1SSE struct {
	Events    []NativeV1SSEEvent `json:"events,omitempty" yaml:"events,omitempty"`
	Template  string             `json:"template,omitempty" yaml:"template,omitempty"`
	Timing    *NativeV1SSETiming `json:"timing,omitempty" yaml:"timing,omitempty"`
	Lifecycle *NativeV1Lifecycle `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
}

// NativeV1SSEEvent defines a single SSE event.
type NativeV1SSEEvent struct {
	Type    string      `json:"type,omitempty" yaml:"type,omitempty"`
	Data    interface{} `json:"data" yaml:"data"`
	ID      string      `json:"id,omitempty" yaml:"id,omitempty"`
	Retry   int         `json:"retry,omitempty" yaml:"retry,omitempty"`
	Comment string      `json:"comment,omitempty" yaml:"comment,omitempty"`
	Delay   *int        `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// NativeV1SSETiming defines SSE timing configuration.
type NativeV1SSETiming struct {
	FixedDelay   *int `json:"fixedDelay,omitempty" yaml:"fixedDelay,omitempty"`
	InitialDelay int  `json:"initialDelay,omitempty" yaml:"initialDelay,omitempty"`
}

// NativeV1Lifecycle defines connection lifecycle settings.
type NativeV1Lifecycle struct {
	KeepaliveInterval int `json:"keepaliveInterval,omitempty" yaml:"keepaliveInterval,omitempty"`
	MaxEvents         int `json:"maxEvents,omitempty" yaml:"maxEvents,omitempty"`
	Timeout           int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// NativeV1Chunked defines chunked transfer encoding configuration.
type NativeV1Chunked struct {
	ChunkSize  int    `json:"chunkSize,omitempty" yaml:"chunkSize,omitempty"`
	ChunkDelay int    `json:"chunkDelay,omitempty" yaml:"chunkDelay,omitempty"`
	Data       string `json:"data,omitempty" yaml:"data,omitempty"`
	Format     string `json:"format,omitempty" yaml:"format,omitempty"`
}

// NativeV1MatchSettings defines matching priority and conditions.
type NativeV1MatchSettings struct {
	Priority  int    `json:"priority,omitempty" yaml:"priority,omitempty"`
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// NativeV1Scenario defines a behavior variation.
type NativeV1Scenario struct {
	ID          string                     `json:"id" yaml:"id"`
	Name        string                     `json:"name" yaml:"name"`
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Trigger     NativeV1Trigger            `json:"trigger" yaml:"trigger"`
	Endpoints   []NativeV1ScenarioEndpoint `json:"endpoints" yaml:"endpoints"`
}

// NativeV1Trigger defines how a scenario is activated.
type NativeV1Trigger struct {
	Header string `json:"header" yaml:"header"`
	Value  string `json:"value" yaml:"value"`
}

// NativeV1ScenarioEndpoint defines a scenario override for an endpoint.
type NativeV1ScenarioEndpoint struct {
	ID       string            `json:"id" yaml:"id"`
	Response *NativeV1Response `json:"response,omitempty" yaml:"response,omitempty"`
}

// NativeV1StatefulResource defines a stateful CRUD resource.
type NativeV1StatefulResource struct {
	Name        string                   `json:"name" yaml:"name"`
	BasePath    string                   `json:"basePath" yaml:"basePath"`
	IDField     string                   `json:"idField,omitempty" yaml:"idField,omitempty"`
	ParentField string                   `json:"parentField,omitempty" yaml:"parentField,omitempty"`
	SeedData    []map[string]interface{} `json:"seedData,omitempty" yaml:"seedData,omitempty"`
}

// NativeV1WebSocket defines a WebSocket endpoint configuration.
type NativeV1WebSocket struct {
	Path         string   `json:"path" yaml:"path"`
	Subprotocols []string `json:"subprotocols,omitempty" yaml:"subprotocols,omitempty"`
	EchoMode     *bool    `json:"echoMode,omitempty" yaml:"echoMode,omitempty"`
}

// NativeV1ToMockCollection converts NativeV1 to a MockCollection.
func NativeV1ToMockCollection(native *NativeV1) (*config.MockCollection, error) {
	if native == nil {
		return nil, fmt.Errorf("native config cannot be nil")
	}

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    native.Metadata.Name,
		Mocks:   make([]*config.MockConfiguration, 0, len(native.Endpoints)),
	}

	// Convert server settings
	if native.Settings.Server != nil {
		collection.ServerConfig = &config.ServerConfiguration{
			HTTPPort:      native.Settings.Server.HTTPPort,
			HTTPSPort:     native.Settings.Server.HTTPSPort,
			AdminPort:     native.Settings.Server.AdminPort,
			ReadTimeout:   native.Settings.Server.ReadTimeout,
			WriteTimeout:  native.Settings.Server.WriteTimeout,
			MaxLogEntries: native.Settings.Server.MaxLogEntries,
			LogRequests:   native.Settings.Logging,
		}
		// Convert legacy AutoGenerateCert to TLS config
		if native.Settings.Server.AutoGenerateCert || native.Settings.Server.HTTPSPort > 0 {
			collection.ServerConfig.TLS = &config.TLSConfig{
				Enabled:          true,
				AutoGenerateCert: native.Settings.Server.AutoGenerateCert,
			}
		}
	}

	// Convert endpoints to mocks
	now := time.Now()
	for _, ep := range native.Endpoints {
		mock, err := convertEndpointToMock(&ep, native.Settings.DefaultDelay, now)
		if err != nil {
			return nil, fmt.Errorf("failed to convert endpoint %s: %w", ep.ID, err)
		}
		collection.Mocks = append(collection.Mocks, mock)
	}

	// Convert stateful resources
	for _, sr := range native.Stateful {
		resource := &config.StatefulResourceConfig{
			Name:        sr.Name,
			BasePath:    sr.BasePath,
			IDField:     sr.IDField,
			ParentField: sr.ParentField,
			SeedData:    sr.SeedData,
		}
		collection.StatefulResources = append(collection.StatefulResources, resource)
	}

	// Convert WebSocket endpoints
	for _, ws := range native.WebSockets {
		wsConfig := &config.WebSocketEndpointConfig{
			Path:         ws.Path,
			Subprotocols: ws.Subprotocols,
			EchoMode:     ws.EchoMode,
		}
		collection.WebSocketEndpoints = append(collection.WebSocketEndpoints, wsConfig)
	}

	return collection, nil
}

// convertEndpointToMock converts a NativeV1Endpoint to MockConfiguration.
func convertEndpointToMock(ep *NativeV1Endpoint, defaultDelay int, now time.Time) (*config.MockConfiguration, error) {
	matcher := &mock.HTTPMatcher{
		Method: ep.Method,
		Path:   ep.Path,
	}

	priority := 0
	if ep.Match != nil {
		priority = ep.Match.Priority
	}

	// Convert request matching criteria
	if ep.Request != nil {
		matcher.Headers = ep.Request.Headers
		matcher.QueryParams = ep.Request.Query
		matcher.BodyContains = ep.Request.BodyContains
		matcher.BodyEquals = ep.Request.BodyEquals
		matcher.BodyPattern = ep.Request.BodyPattern
	}

	httpSpec := &mock.HTTPSpec{
		Priority: priority,
		Matcher:  matcher,
	}

	// Convert response
	if ep.Response != nil {
		bodyStr := ""
		if ep.Response.Body != nil {
			switch v := ep.Response.Body.(type) {
			case string:
				bodyStr = v
			default:
				bodyBytes, err := json.Marshal(v)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal response body: %w", err)
				}
				bodyStr = string(bodyBytes)
			}
		}

		delay := ep.Response.Delay
		if delay == 0 {
			delay = defaultDelay
		}

		httpSpec.Response = &mock.HTTPResponse{
			StatusCode: ep.Response.Status,
			Headers:    ep.Response.Headers,
			Body:       bodyStr,
			DelayMs:    delay,
		}
	}

	// Convert SSE config
	if ep.SSE != nil {
		sseConfig := &mock.SSEConfig{
			Template: ep.SSE.Template,
		}
		for _, event := range ep.SSE.Events {
			sseConfig.Events = append(sseConfig.Events, mock.SSEEventDef{
				Type:    event.Type,
				Data:    event.Data,
				ID:      event.ID,
				Retry:   event.Retry,
				Comment: event.Comment,
				Delay:   event.Delay,
			})
		}
		if ep.SSE.Timing != nil {
			sseConfig.Timing = mock.SSETimingConfig{
				FixedDelay:   ep.SSE.Timing.FixedDelay,
				InitialDelay: ep.SSE.Timing.InitialDelay,
			}
		}
		if ep.SSE.Lifecycle != nil {
			sseConfig.Lifecycle = mock.SSELifecycleConfig{
				KeepaliveInterval: ep.SSE.Lifecycle.KeepaliveInterval,
				MaxEvents:         ep.SSE.Lifecycle.MaxEvents,
				Timeout:           ep.SSE.Lifecycle.Timeout,
			}
		}
		httpSpec.SSE = sseConfig
	}

	// Convert Chunked config
	if ep.Chunked != nil {
		httpSpec.Chunked = &mock.ChunkedConfig{
			ChunkSize:  ep.Chunked.ChunkSize,
			ChunkDelay: ep.Chunked.ChunkDelay,
			Data:       ep.Chunked.Data,
			Format:     ep.Chunked.Format,
		}
	}

	epEnabled := ep.Enabled
	m := &config.MockConfiguration{
		ID:        ep.ID,
		Type:      mock.TypeHTTP,
		Name:      ep.Name,
		Enabled:   &epEnabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP:      httpSpec,
	}

	return m, nil
}

// MockCollectionToNativeV1 converts a MockCollection to NativeV1 format.
func MockCollectionToNativeV1(collection *config.MockCollection) (*NativeV1, error) {
	if collection == nil {
		return nil, fmt.Errorf("collection cannot be nil")
	}

	native := &NativeV1{
		Version: "1.0",
		Kind:    "MockCollection",
		Metadata: NativeV1Metadata{
			Name: collection.Name,
		},
	}

	// Convert server settings
	if collection.ServerConfig != nil {
		native.Settings.Server = &NativeV1Server{
			HTTPPort:      collection.ServerConfig.HTTPPort,
			HTTPSPort:     collection.ServerConfig.HTTPSPort,
			AdminPort:     collection.ServerConfig.AdminPort,
			ReadTimeout:   collection.ServerConfig.ReadTimeout,
			WriteTimeout:  collection.ServerConfig.WriteTimeout,
			MaxLogEntries: collection.ServerConfig.MaxLogEntries,
		}
		// Convert TLS config to legacy AutoGenerateCert
		if collection.ServerConfig.TLS != nil {
			native.Settings.Server.AutoGenerateCert = collection.ServerConfig.TLS.AutoGenerateCert
		}
		native.Settings.Logging = collection.ServerConfig.LogRequests
	}

	// Convert mocks to endpoints
	native.Endpoints = make([]NativeV1Endpoint, 0, len(collection.Mocks))
	for _, mock := range collection.Mocks {
		ep, err := convertMockToEndpoint(mock)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mock %s: %w", mock.ID, err)
		}
		native.Endpoints = append(native.Endpoints, *ep)
	}

	// Convert stateful resources
	for _, sr := range collection.StatefulResources {
		native.Stateful = append(native.Stateful, NativeV1StatefulResource{
			Name:        sr.Name,
			BasePath:    sr.BasePath,
			IDField:     sr.IDField,
			ParentField: sr.ParentField,
			SeedData:    sr.SeedData,
		})
	}

	// Convert WebSocket endpoints
	for _, ws := range collection.WebSocketEndpoints {
		native.WebSockets = append(native.WebSockets, NativeV1WebSocket{
			Path:         ws.Path,
			Subprotocols: ws.Subprotocols,
			EchoMode:     ws.EchoMode,
		})
	}

	return native, nil
}

// convertMockToEndpoint converts a MockConfiguration to NativeV1Endpoint.
//
//nolint:unparam // error is always nil but kept for future validation
func convertMockToEndpoint(m *config.MockConfiguration) (*NativeV1Endpoint, error) {
	ep := &NativeV1Endpoint{
		ID:      m.ID,
		Name:    m.Name,
		Enabled: m.Enabled == nil || *m.Enabled,
	}

	// Convert HTTP spec
	if m.HTTP != nil {
		// Convert matcher
		if m.HTTP.Matcher != nil {
			ep.Method = m.HTTP.Matcher.Method
			ep.Path = m.HTTP.Matcher.Path

			// Only include request if there are additional matching criteria
			if len(m.HTTP.Matcher.Headers) > 0 || len(m.HTTP.Matcher.QueryParams) > 0 ||
				m.HTTP.Matcher.BodyContains != "" || m.HTTP.Matcher.BodyEquals != "" ||
				m.HTTP.Matcher.BodyPattern != "" {
				ep.Request = &NativeV1Request{
					Headers:      m.HTTP.Matcher.Headers,
					Query:        m.HTTP.Matcher.QueryParams,
					BodyContains: m.HTTP.Matcher.BodyContains,
					BodyEquals:   m.HTTP.Matcher.BodyEquals,
					BodyPattern:  m.HTTP.Matcher.BodyPattern,
				}
			}
		}

		// Convert match settings
		if m.HTTP.Priority > 0 {
			ep.Match = &NativeV1MatchSettings{
				Priority: m.HTTP.Priority,
			}
		}

		// Convert response
		if m.HTTP.Response != nil {
			ep.Response = &NativeV1Response{
				Status:  m.HTTP.Response.StatusCode,
				Headers: m.HTTP.Response.Headers,
				Delay:   m.HTTP.Response.DelayMs,
			}

			// Try to parse body as JSON for nicer output
			if m.HTTP.Response.Body != "" {
				var parsed interface{}
				if err := json.Unmarshal([]byte(m.HTTP.Response.Body), &parsed); err == nil {
					ep.Response.Body = parsed
				} else {
					ep.Response.Body = m.HTTP.Response.Body
				}
			}
		}

		// Convert SSE config
		if m.HTTP.SSE != nil {
			ep.SSE = &NativeV1SSE{
				Template: m.HTTP.SSE.Template,
			}
			for _, event := range m.HTTP.SSE.Events {
				ep.SSE.Events = append(ep.SSE.Events, NativeV1SSEEvent{
					Type:    event.Type,
					Data:    event.Data,
					ID:      event.ID,
					Retry:   event.Retry,
					Comment: event.Comment,
					Delay:   event.Delay,
				})
			}
			if m.HTTP.SSE.Timing.FixedDelay != nil || m.HTTP.SSE.Timing.InitialDelay != 0 {
				ep.SSE.Timing = &NativeV1SSETiming{
					FixedDelay:   m.HTTP.SSE.Timing.FixedDelay,
					InitialDelay: m.HTTP.SSE.Timing.InitialDelay,
				}
			}
			if m.HTTP.SSE.Lifecycle.KeepaliveInterval != 0 || m.HTTP.SSE.Lifecycle.MaxEvents != 0 || m.HTTP.SSE.Lifecycle.Timeout != 0 {
				ep.SSE.Lifecycle = &NativeV1Lifecycle{
					KeepaliveInterval: m.HTTP.SSE.Lifecycle.KeepaliveInterval,
					MaxEvents:         m.HTTP.SSE.Lifecycle.MaxEvents,
					Timeout:           m.HTTP.SSE.Lifecycle.Timeout,
				}
			}
		}

		// Convert Chunked config
		if m.HTTP.Chunked != nil {
			ep.Chunked = &NativeV1Chunked{
				ChunkSize:  m.HTTP.Chunked.ChunkSize,
				ChunkDelay: m.HTTP.Chunked.ChunkDelay,
				Data:       m.HTTP.Chunked.Data,
				Format:     m.HTTP.Chunked.Format,
			}
		}
	}

	return ep, nil
}

// NativeImporter imports Mockd native format (YAML or JSON).
type NativeImporter struct{}

// Import parses Mockd native format data and returns a MockCollection.
func (i *NativeImporter) Import(data []byte) (*config.MockCollection, error) {
	// Try to detect if it's JSON or YAML
	trimmed := bytes.TrimSpace(data)

	// Check for NativeV1 format (version + kind)
	var native NativeV1
	var parseErr error

	// Try JSON first if it starts with {
	if len(trimmed) > 0 && trimmed[0] == '{' {
		parseErr = json.Unmarshal(data, &native)
	} else {
		// Try YAML
		parseErr = yaml.Unmarshal(data, &native)
	}

	if parseErr != nil {
		return nil, &ImportError{
			Format:  FormatMockd,
			Message: "failed to parse native format",
			Cause:   parseErr,
		}
	}

	// Check if it's NativeV1 format (has kind: MockCollection)
	if native.Kind == "MockCollection" {
		return NativeV1ToMockCollection(&native)
	}

	// Fall back to direct MockCollection parsing (legacy format)
	var collection config.MockCollection

	if len(trimmed) > 0 && trimmed[0] == '{' {
		parseErr = json.Unmarshal(data, &collection)
	} else {
		parseErr = yaml.Unmarshal(data, &collection)
	}

	if parseErr != nil {
		return nil, &ImportError{
			Format:  FormatMockd,
			Message: "failed to parse mock collection",
			Cause:   parseErr,
		}
	}

	// Validate the collection
	if err := collection.Validate(); err != nil {
		return nil, &ImportError{
			Format:  FormatMockd,
			Message: "validation failed",
			Cause:   err,
		}
	}

	return &collection, nil
}

// Format returns FormatMockd.
func (i *NativeImporter) Format() Format {
	return FormatMockd
}

// NativeExporter exports to Mockd native format.
type NativeExporter struct {
	// AsYAML if true, outputs YAML instead of JSON
	AsYAML bool
}

// Export converts a MockCollection to Mockd native format bytes.
func (e *NativeExporter) Export(collection *config.MockCollection) ([]byte, error) {
	if collection == nil {
		return nil, &ExportError{
			Format:  FormatMockd,
			Message: "collection cannot be nil",
		}
	}

	native, err := MockCollectionToNativeV1(collection)
	if err != nil {
		return nil, &ExportError{
			Format:  FormatMockd,
			Message: "failed to convert to native format",
			Cause:   err,
		}
	}

	var data []byte
	if e.AsYAML {
		data, err = yaml.Marshal(native)
	} else {
		data, err = json.MarshalIndent(native, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
	}

	if err != nil {
		return nil, &ExportError{
			Format:  FormatMockd,
			Message: "failed to marshal output",
			Cause:   err,
		}
	}

	return data, nil
}

// Format returns FormatMockd.
func (e *NativeExporter) Format() Format {
	return FormatMockd
}

// init registers the native importer and exporter.
func init() {
	RegisterImporter(&NativeImporter{})
	RegisterExporter(&NativeExporter{AsYAML: true})
}
