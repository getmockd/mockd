package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// SpanStatus represents the status of a span.
type SpanStatus int

const (
	// StatusUnset is the default status.
	StatusUnset SpanStatus = iota
	// StatusOK indicates the operation completed successfully.
	StatusOK
	// StatusError indicates the operation failed.
	StatusError
)

// String returns the string representation of the status.
func (s SpanStatus) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusError:
		return "ERROR"
	default:
		return "UNSET"
	}
}

// SpanKind describes the relationship between the Span, its parents and children.
type SpanKind int

const (
	// SpanKindUnspecified is the default, unspecified span kind.
	SpanKindUnspecified SpanKind = 0
	// SpanKindInternal indicates an internal operation.
	SpanKindInternal SpanKind = 1
	// SpanKindServer indicates a server-side handling of an RPC or HTTP request.
	SpanKindServer SpanKind = 2
	// SpanKindClient indicates a client-side RPC or HTTP request.
	SpanKindClient SpanKind = 3
	// SpanKindProducer indicates a message producer.
	SpanKindProducer SpanKind = 4
	// SpanKindConsumer indicates a message consumer.
	SpanKindConsumer SpanKind = 5
)

// SpanEvent represents an event that occurred during a span.
type SpanEvent struct {
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Attrs     map[string]string `json:"attributes,omitempty"`
}

// Span represents a single operation within a trace.
type Span struct {
	TraceID       string            `json:"traceId"`
	SpanID        string            `json:"spanId"`
	ParentID      string            `json:"parentId,omitempty"`
	Name          string            `json:"name"`
	Kind          SpanKind          `json:"kind,omitempty"`
	StartTime     time.Time         `json:"startTime"`
	EndTime       time.Time         `json:"endTime,omitempty"`
	Status        SpanStatus        `json:"status"`
	StatusMessage string            `json:"statusMessage,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Events        []SpanEvent       `json:"events,omitempty"`

	mu     sync.Mutex
	tracer *Tracer
	ended  bool
}

// End marks the span as ended and exports it.
func (s *Span) End() {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	s.EndTime = time.Now()
	s.mu.Unlock()

	if s.tracer != nil {
		s.tracer.exportSpan(s)
	}
}

// SetAttribute sets a key-value attribute on the span.
func (s *Span) SetAttribute(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.Attributes == nil {
		s.Attributes = make(map[string]string)
	}
	s.Attributes[key] = value
}

// AddEvent adds a timestamped event to the span.
func (s *Span) AddEvent(name string, attrs ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}

	event := SpanEvent{
		Name:      name,
		Timestamp: time.Now(),
	}

	// Parse variadic attrs as key-value pairs
	if len(attrs) > 0 {
		event.Attrs = make(map[string]string)
		for i := 0; i+1 < len(attrs); i += 2 {
			event.Attrs[attrs[i]] = attrs[i+1]
		}
	}

	s.Events = append(s.Events, event)
}

// SetKind sets the kind of the span. This should be called before End().
func (s *Span) SetKind(kind SpanKind) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.Kind = kind
}

// SetStatus sets the status of the span.
func (s *Span) SetStatus(status SpanStatus, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.Status = status
	s.StatusMessage = message
}

// IsRecording returns true if the span is recording events.
func (s *Span) IsRecording() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.ended
}

// SpanContext returns the context values needed for propagation.
func (s *Span) SpanContext() SpanContext {
	return SpanContext{
		TraceID:  s.TraceID,
		SpanID:   s.SpanID,
		ParentID: s.ParentID,
		Sampled:  true,
	}
}

// SpanContext holds the trace context for propagation.
type SpanContext struct {
	TraceID  string
	SpanID   string
	ParentID string
	Sampled  bool
}

// IsValid returns true if the span context has valid trace and span IDs.
func (sc SpanContext) IsValid() bool {
	return sc.TraceID != "" && sc.SpanID != ""
}

// Tracer creates spans and manages span lifecycle.
type Tracer struct {
	serviceName string
	exporter    Exporter
	sampler     Sampler
	mu          sync.Mutex
	spans       []*Span
	batchSize   int
	wg          sync.WaitGroup // tracks in-flight exports
}

// TracerOption configures a Tracer.
type TracerOption func(*Tracer)

// WithExporter sets the exporter for the tracer.
func WithExporter(e Exporter) TracerOption {
	return func(t *Tracer) {
		t.exporter = e
	}
}

// WithSampler sets the sampler for the tracer.
func WithSampler(s Sampler) TracerOption {
	return func(t *Tracer) {
		t.sampler = s
	}
}

// WithBatchSize sets the batch size for span export.
func WithBatchSize(size int) TracerOption {
	return func(t *Tracer) {
		t.batchSize = size
	}
}

// Sampler decides whether a span should be recorded.
type Sampler interface {
	ShouldSample(traceID string) bool
}

// AlwaysSample is a sampler that always samples.
type AlwaysSample struct{}

// ShouldSample always returns true.
func (AlwaysSample) ShouldSample(string) bool { return true }

// NeverSample is a sampler that never samples.
type NeverSample struct{}

// ShouldSample always returns false.
func (NeverSample) ShouldSample(string) bool { return false }

// RatioSampler samples a percentage of traces.
type RatioSampler struct {
	ratio float64
}

// NewRatioSampler creates a sampler that samples the given ratio of traces.
func NewRatioSampler(ratio float64) *RatioSampler {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return &RatioSampler{ratio: ratio}
}

// ShouldSample returns true if the trace should be sampled based on trace ID.
func (s *RatioSampler) ShouldSample(traceID string) bool {
	if s.ratio >= 1 {
		return true
	}
	if s.ratio <= 0 {
		return false
	}
	// Use first 8 bytes of trace ID for deterministic sampling
	if len(traceID) < 16 {
		return true
	}
	b, err := hex.DecodeString(traceID[:16])
	if err != nil {
		return true
	}
	// Convert to uint64 and check against threshold
	var val uint64
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(b[i])
	}
	threshold := uint64(s.ratio * float64(^uint64(0)))
	return val < threshold
}

// NewTracer creates a new Tracer with the given service name.
func NewTracer(serviceName string, opts ...TracerOption) *Tracer {
	t := &Tracer{
		serviceName: serviceName,
		sampler:     AlwaysSample{},
		batchSize:   100,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Start creates a new span with the given name.
// If the context already contains a span, the new span will be a child of it.
func (t *Tracer) Start(ctx context.Context, name string) (context.Context, *Span) {
	var traceID, parentID string

	// Check for existing span in context
	if parent := SpanFromContext(ctx); parent != nil {
		traceID = parent.TraceID
		parentID = parent.SpanID
	} else if sc := SpanContextFromContext(ctx); sc.IsValid() {
		// Check for propagated span context (from Extract)
		traceID = sc.TraceID
		parentID = sc.SpanID
	}

	// Generate new trace ID if none exists
	if traceID == "" {
		traceID = generateTraceID()
	}

	// Check sampling decision
	if !t.sampler.ShouldSample(traceID) {
		// Return a non-recording span
		span := &Span{
			TraceID:   traceID,
			SpanID:    generateSpanID(),
			ParentID:  parentID,
			Name:      name,
			StartTime: time.Now(),
			ended:     true, // Mark as ended so no operations record
		}
		return contextWithSpan(ctx, span), span
	}

	span := &Span{
		TraceID:    traceID,
		SpanID:     generateSpanID(),
		ParentID:   parentID,
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]string),
		tracer:     t,
	}

	// Add service name attribute
	span.Attributes["service.name"] = t.serviceName

	return contextWithSpan(ctx, span), span
}

// ServiceName returns the tracer's service name.
func (t *Tracer) ServiceName() string {
	return t.serviceName
}

// Shutdown gracefully shuts down the tracer, flushing any pending spans.
func (t *Tracer) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	spans := t.spans
	t.spans = nil
	t.mu.Unlock()

	if t.exporter != nil && len(spans) > 0 {
		if err := t.exporter.Export(spans); err != nil {
			return err
		}
		return t.exporter.Shutdown(ctx)
	}

	if t.exporter != nil {
		return t.exporter.Shutdown(ctx)
	}
	return nil
}

// exportSpan adds a span to the batch and exports if batch is full.
func (t *Tracer) exportSpan(span *Span) {
	if t.exporter == nil {
		return
	}

	t.mu.Lock()
	t.spans = append(t.spans, span)
	if len(t.spans) >= t.batchSize {
		spans := t.spans
		t.spans = nil
		t.wg.Add(1)
		t.mu.Unlock()

		// Export in background to avoid blocking
		go func() {
			defer t.wg.Done()
			_ = t.exporter.Export(spans)
		}()
		return
	}
	t.mu.Unlock()
}

// Flush exports any buffered spans immediately and waits for in-flight exports.
func (t *Tracer) Flush() error {
	// Wait for any in-flight exports to complete
	t.wg.Wait()

	t.mu.Lock()
	spans := t.spans
	t.spans = nil
	t.mu.Unlock()

	if t.exporter != nil && len(spans) > 0 {
		return t.exporter.Export(spans)
	}
	return nil
}

// generateTraceID generates a random 16-byte trace ID as a hex string.
func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateSpanID generates a random 8-byte span ID as a hex string.
func generateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Context key types for storing span information.
type spanContextKey struct{}
type spanContextValueKey struct{}

// contextWithSpan returns a new context with the span stored in it.
func contextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanContextKey{}, span)
}

// SpanFromContext returns the current span from the context, or nil if none.
func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey{}).(*Span); ok {
		return span
	}
	return nil
}

// contextWithSpanContext returns a context with the SpanContext stored in it.
func contextWithSpanContext(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, spanContextValueKey{}, sc)
}

// SpanContextFromContext returns the SpanContext from the context.
func SpanContextFromContext(ctx context.Context) SpanContext {
	if sc, ok := ctx.Value(spanContextValueKey{}).(SpanContext); ok {
		return sc
	}
	return SpanContext{}
}
