package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Exporter exports spans to a backend.
type Exporter interface {
	// Export sends spans to the backend.
	Export(spans []*Span) error
	// Shutdown gracefully shuts down the exporter.
	Shutdown(ctx context.Context) error
}

// ============================================================================
// StdoutExporter - prints spans as JSON (for dev/debug)
// ============================================================================

// StdoutExporter writes spans to stdout as JSON.
type StdoutExporter struct {
	mu     sync.Mutex
	writer io.Writer
	pretty bool
}

// StdoutOption configures a StdoutExporter.
type StdoutOption func(*StdoutExporter)

// WithWriter sets the output writer for the exporter.
func WithWriter(w io.Writer) StdoutOption {
	return func(e *StdoutExporter) {
		e.writer = w
	}
}

// WithPrettyPrint enables pretty-printed JSON output.
func WithPrettyPrint() StdoutOption {
	return func(e *StdoutExporter) {
		e.pretty = true
	}
}

// NewStdoutExporter creates a new stdout exporter.
func NewStdoutExporter(opts ...StdoutOption) *StdoutExporter {
	e := &StdoutExporter{
		writer: os.Stdout,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export writes spans to stdout as JSON.
func (e *StdoutExporter) Export(spans []*Span) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, span := range spans {
		output := spanToOutput(span)
		var data []byte
		var err error
		if e.pretty {
			data, err = json.MarshalIndent(output, "", "  ")
		} else {
			data, err = json.Marshal(output)
		}
		if err != nil {
			return fmt.Errorf("failed to marshal span: %w", err)
		}
		fmt.Fprintln(e.writer, string(data))
	}
	return nil
}

// Shutdown is a no-op for the stdout exporter.
func (e *StdoutExporter) Shutdown(context.Context) error {
	return nil
}

// spanOutput is the JSON structure for stdout output.
type spanOutput struct {
	TraceID       string            `json:"traceId"`
	SpanID        string            `json:"spanId"`
	ParentID      string            `json:"parentId,omitempty"`
	Name          string            `json:"name"`
	StartTime     string            `json:"startTime"`
	EndTime       string            `json:"endTime"`
	Duration      string            `json:"duration"`
	Status        string            `json:"status"`
	StatusMessage string            `json:"statusMessage,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Events        []eventOutput     `json:"events,omitempty"`
}

type eventOutput struct {
	Name       string            `json:"name"`
	Timestamp  string            `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func spanToOutput(span *Span) spanOutput {
	output := spanOutput{
		TraceID:       span.TraceID,
		SpanID:        span.SpanID,
		ParentID:      span.ParentID,
		Name:          span.Name,
		StartTime:     span.StartTime.Format(time.RFC3339Nano),
		EndTime:       span.EndTime.Format(time.RFC3339Nano),
		Duration:      span.EndTime.Sub(span.StartTime).String(),
		Status:        span.Status.String(),
		StatusMessage: span.StatusMessage,
		Attributes:    span.Attributes,
	}

	if len(span.Events) > 0 {
		output.Events = make([]eventOutput, len(span.Events))
		for i, e := range span.Events {
			output.Events[i] = eventOutput{
				Name:       e.Name,
				Timestamp:  e.Timestamp.Format(time.RFC3339Nano),
				Attributes: e.Attrs,
			}
		}
	}

	return output
}

// ============================================================================
// OTLPExporter - sends spans to OTLP HTTP endpoint
// ============================================================================

// OTLPExporter exports spans to an OTLP HTTP endpoint.
type OTLPExporter struct {
	endpoint   string
	client     *http.Client
	headers    map[string]string
	mu         sync.Mutex
	shutdown   bool
	retryCount int
}

// OTLPOption configures an OTLPExporter.
type OTLPOption func(*OTLPExporter)

// WithOTLPHeaders sets custom headers for OTLP requests.
func WithOTLPHeaders(headers map[string]string) OTLPOption {
	return func(e *OTLPExporter) {
		e.headers = headers
	}
}

// WithOTLPClient sets a custom HTTP client.
func WithOTLPClient(client *http.Client) OTLPOption {
	return func(e *OTLPExporter) {
		e.client = client
	}
}

// WithOTLPRetryCount sets the number of retry attempts.
func WithOTLPRetryCount(count int) OTLPOption {
	return func(e *OTLPExporter) {
		e.retryCount = count
	}
}

// NewOTLPExporter creates a new OTLP HTTP exporter.
func NewOTLPExporter(endpoint string, opts ...OTLPOption) *OTLPExporter {
	e := &OTLPExporter{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers:    make(map[string]string),
		retryCount: 3,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export sends spans to the OTLP endpoint.
func (e *OTLPExporter) Export(spans []*Span) error {
	e.mu.Lock()
	if e.shutdown {
		e.mu.Unlock()
		return fmt.Errorf("exporter is shut down")
	}
	e.mu.Unlock()

	if len(spans) == 0 {
		return nil
	}

	payload := convertToOTLP(spans)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP payload: %w", err)
	}

	var lastErr error
	for i := 0; i <= e.retryCount; i++ {
		if err := e.send(data); err != nil {
			lastErr = err
			// Exponential backoff
			if i < e.retryCount {
				time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (e *OTLPExporter) send(data []byte) error {
	req, err := http.NewRequest(http.MethodPost, e.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OTLP export failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Shutdown gracefully shuts down the exporter.
func (e *OTLPExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdown = true
	return nil
}

// ============================================================================
// OTLP JSON Protocol Structures
// ============================================================================

// OTLP trace request structure (simplified JSON format).
type otlpTraceRequest struct {
	ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource   otlpResource    `json:"resource"`
	ScopeSpans []otlpScopeSpan `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes"`
}

type otlpScopeSpan struct {
	Scope otlpScope  `json:"scope"`
	Spans []otlpSpan `json:"spans"`
}

type otlpScope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type otlpSpan struct {
	TraceID           string         `json:"traceId"`
	SpanID            string         `json:"spanId"`
	ParentSpanID      string         `json:"parentSpanId,omitempty"`
	Name              string         `json:"name"`
	Kind              int            `json:"kind"`
	StartTimeUnixNano string         `json:"startTimeUnixNano"`
	EndTimeUnixNano   string         `json:"endTimeUnixNano"`
	Attributes        []otlpKeyValue `json:"attributes,omitempty"`
	Events            []otlpEvent    `json:"events,omitempty"`
	Status            otlpStatus     `json:"status"`
}

type otlpKeyValue struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

type otlpValue struct {
	StringValue string `json:"stringValue,omitempty"`
}

type otlpEvent struct {
	TimeUnixNano string         `json:"timeUnixNano"`
	Name         string         `json:"name"`
	Attributes   []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// convertToOTLP converts spans to OTLP JSON format.
func convertToOTLP(spans []*Span) otlpTraceRequest {
	// Group spans by service name
	serviceSpans := make(map[string][]*Span)
	for _, span := range spans {
		svc := span.Attributes["service.name"]
		if svc == "" {
			svc = "unknown"
		}
		serviceSpans[svc] = append(serviceSpans[svc], span)
	}

	var resourceSpans []otlpResourceSpans
	for serviceName, spans := range serviceSpans {
		otlpSpans := make([]otlpSpan, 0, len(spans))
		for _, span := range spans {
			otlpSpans = append(otlpSpans, convertSpan(span))
		}

		resourceSpans = append(resourceSpans, otlpResourceSpans{
			Resource: otlpResource{
				Attributes: []otlpKeyValue{
					{Key: "service.name", Value: otlpValue{StringValue: serviceName}},
				},
			},
			ScopeSpans: []otlpScopeSpan{
				{
					Scope: otlpScope{Name: "mockd/tracing"},
					Spans: otlpSpans,
				},
			},
		})
	}

	return otlpTraceRequest{ResourceSpans: resourceSpans}
}

func convertSpan(span *Span) otlpSpan {
	// Convert attributes (excluding service.name which is on resource)
	var attrs []otlpKeyValue
	for k, v := range span.Attributes {
		if k != "service.name" {
			attrs = append(attrs, otlpKeyValue{Key: k, Value: otlpValue{StringValue: v}})
		}
	}

	// Convert events
	var events []otlpEvent
	for _, e := range span.Events {
		var eventAttrs []otlpKeyValue
		for k, v := range e.Attrs {
			eventAttrs = append(eventAttrs, otlpKeyValue{Key: k, Value: otlpValue{StringValue: v}})
		}
		events = append(events, otlpEvent{
			TimeUnixNano: fmt.Sprintf("%d", e.Timestamp.UnixNano()),
			Name:         e.Name,
			Attributes:   eventAttrs,
		})
	}

	// Convert status code
	statusCode := 0 // UNSET
	switch span.Status {
	case StatusOK:
		statusCode = 1
	case StatusError:
		statusCode = 2
	}

	return otlpSpan{
		TraceID:           span.TraceID,
		SpanID:            span.SpanID,
		ParentSpanID:      span.ParentID,
		Name:              span.Name,
		Kind:              0, // UNSPECIFIED
		StartTimeUnixNano: fmt.Sprintf("%d", span.StartTime.UnixNano()),
		EndTimeUnixNano:   fmt.Sprintf("%d", span.EndTime.UnixNano()),
		Attributes:        attrs,
		Events:            events,
		Status: otlpStatus{
			Code:    statusCode,
			Message: span.StatusMessage,
		},
	}
}

// ============================================================================
// NoopExporter - does nothing (for testing/disabled tracing)
// ============================================================================

// NoopExporter is an exporter that does nothing.
type NoopExporter struct{}

// NewNoopExporter creates a new noop exporter.
func NewNoopExporter() *NoopExporter {
	return &NoopExporter{}
}

// Export does nothing and returns nil.
func (e *NoopExporter) Export([]*Span) error {
	return nil
}

// Shutdown does nothing and returns nil.
func (e *NoopExporter) Shutdown(context.Context) error {
	return nil
}
