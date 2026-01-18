package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSpanCreation(t *testing.T) {
	t.Run("creates span with trace and span IDs", func(t *testing.T) {
		tracer := NewTracer("test-service")
		ctx, span := tracer.Start(context.Background(), "test-operation")
		defer span.End()

		if span.TraceID == "" {
			t.Error("TraceID should not be empty")
		}
		if len(span.TraceID) != 32 {
			t.Errorf("TraceID should be 32 chars, got %d", len(span.TraceID))
		}
		if span.SpanID == "" {
			t.Error("SpanID should not be empty")
		}
		if len(span.SpanID) != 16 {
			t.Errorf("SpanID should be 16 chars, got %d", len(span.SpanID))
		}
		if span.Name != "test-operation" {
			t.Errorf("expected name 'test-operation', got '%s'", span.Name)
		}
		if span.StartTime.IsZero() {
			t.Error("StartTime should not be zero")
		}
		if span.Attributes["service.name"] != "test-service" {
			t.Errorf("expected service.name 'test-service', got '%s'", span.Attributes["service.name"])
		}

		// Verify span is in context
		ctxSpan := SpanFromContext(ctx)
		if ctxSpan != span {
			t.Error("span should be stored in context")
		}
	})

	t.Run("child span inherits trace ID", func(t *testing.T) {
		tracer := NewTracer("test-service")
		ctx, parent := tracer.Start(context.Background(), "parent")
		defer parent.End()

		_, child := tracer.Start(ctx, "child")
		defer child.End()

		if child.TraceID != parent.TraceID {
			t.Error("child should have same trace ID as parent")
		}
		if child.ParentID != parent.SpanID {
			t.Error("child's parent ID should be parent's span ID")
		}
		if child.SpanID == parent.SpanID {
			t.Error("child should have different span ID than parent")
		}
	})
}

func TestSpanEnd(t *testing.T) {
	t.Run("sets end time", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")

		if !span.EndTime.IsZero() {
			t.Error("EndTime should be zero before End()")
		}

		span.End()

		if span.EndTime.IsZero() {
			t.Error("EndTime should be set after End()")
		}
		if span.EndTime.Before(span.StartTime) {
			t.Error("EndTime should be after StartTime")
		}
	})

	t.Run("end is idempotent", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")

		span.End()
		firstEndTime := span.EndTime

		time.Sleep(10 * time.Millisecond)
		span.End() // Second call should be no-op

		if span.EndTime != firstEndTime {
			t.Error("second End() should not change EndTime")
		}
	})
}

func TestSpanAttributes(t *testing.T) {
	t.Run("set attribute", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")
		defer span.End()

		span.SetAttribute("http.method", "GET")
		span.SetAttribute("http.url", "/api/users")

		if span.Attributes["http.method"] != "GET" {
			t.Errorf("expected http.method 'GET', got '%s'", span.Attributes["http.method"])
		}
		if span.Attributes["http.url"] != "/api/users" {
			t.Errorf("expected http.url '/api/users', got '%s'", span.Attributes["http.url"])
		}
	})

	t.Run("attributes after end are ignored", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")
		span.End()

		span.SetAttribute("ignored", "value")
		if _, ok := span.Attributes["ignored"]; ok {
			t.Error("attribute set after End() should be ignored")
		}
	})
}

func TestSpanEvents(t *testing.T) {
	t.Run("add event", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")
		defer span.End()

		span.AddEvent("processing started")
		span.AddEvent("item processed", "item_id", "123", "status", "success")

		if len(span.Events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(span.Events))
		}

		if span.Events[0].Name != "processing started" {
			t.Errorf("expected event name 'processing started', got '%s'", span.Events[0].Name)
		}
		if span.Events[0].Timestamp.IsZero() {
			t.Error("event timestamp should not be zero")
		}

		if span.Events[1].Name != "item processed" {
			t.Errorf("expected event name 'item processed', got '%s'", span.Events[1].Name)
		}
		if span.Events[1].Attrs["item_id"] != "123" {
			t.Errorf("expected item_id '123', got '%s'", span.Events[1].Attrs["item_id"])
		}
		if span.Events[1].Attrs["status"] != "success" {
			t.Errorf("expected status 'success', got '%s'", span.Events[1].Attrs["status"])
		}
	})

	t.Run("events after end are ignored", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")
		span.End()

		span.AddEvent("ignored")
		if len(span.Events) != 0 {
			t.Error("events after End() should be ignored")
		}
	})
}

func TestSpanStatus(t *testing.T) {
	t.Run("set status", func(t *testing.T) {
		tracer := NewTracer("test-service")
		_, span := tracer.Start(context.Background(), "test")
		defer span.End()

		span.SetStatus(StatusError, "something went wrong")

		if span.Status != StatusError {
			t.Errorf("expected status ERROR, got %s", span.Status.String())
		}
		if span.StatusMessage != "something went wrong" {
			t.Errorf("expected message 'something went wrong', got '%s'", span.StatusMessage)
		}
	})

	t.Run("status string values", func(t *testing.T) {
		tests := []struct {
			status   SpanStatus
			expected string
		}{
			{StatusUnset, "UNSET"},
			{StatusOK, "OK"},
			{StatusError, "ERROR"},
		}
		for _, tt := range tests {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		}
	})
}

func TestContextPropagation(t *testing.T) {
	t.Run("extract valid traceparent", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

		ctx := Extract(context.Background(), headers)
		sc := SpanContextFromContext(ctx)

		if !sc.IsValid() {
			t.Error("span context should be valid")
		}
		if sc.TraceID != "0af7651916cd43dd8448eb211c80319c" {
			t.Errorf("expected trace ID '0af7651916cd43dd8448eb211c80319c', got '%s'", sc.TraceID)
		}
		if sc.SpanID != "b7ad6b7169203331" {
			t.Errorf("expected span ID 'b7ad6b7169203331', got '%s'", sc.SpanID)
		}
		if !sc.Sampled {
			t.Error("sampled should be true")
		}
	})

	t.Run("extract unsampled traceparent", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00")

		ctx := Extract(context.Background(), headers)
		sc := SpanContextFromContext(ctx)

		if sc.Sampled {
			t.Error("sampled should be false")
		}
	})

	t.Run("extract invalid traceparent returns original context", func(t *testing.T) {
		tests := []struct {
			name        string
			traceparent string
		}{
			{"empty", ""},
			{"wrong parts", "00-abc-def"},
			{"invalid version length", "0-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
			{"invalid trace ID length", "00-0af7651916cd43dd-b7ad6b7169203331-01"},
			{"invalid span ID length", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b71-01"},
			{"all zeros trace ID", "00-00000000000000000000000000000000-b7ad6b7169203331-01"},
			{"all zeros span ID", "00-0af7651916cd43dd8448eb211c80319c-0000000000000000-01"},
			{"invalid hex in trace ID", "00-0zf7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				headers := http.Header{}
				if tt.traceparent != "" {
					headers.Set("traceparent", tt.traceparent)
				}

				ctx := Extract(context.Background(), headers)
				sc := SpanContextFromContext(ctx)

				if sc.IsValid() {
					t.Error("span context should not be valid for invalid traceparent")
				}
			})
		}
	})

	t.Run("inject span context into headers", func(t *testing.T) {
		tracer := NewTracer("test-service")
		ctx, span := tracer.Start(context.Background(), "test")
		defer span.End()

		headers := http.Header{}
		Inject(ctx, headers)

		traceparent := headers.Get("traceparent")
		if traceparent == "" {
			t.Fatal("traceparent header should be set")
		}

		// Verify format
		parts := strings.Split(traceparent, "-")
		if len(parts) != 4 {
			t.Fatalf("expected 4 parts, got %d", len(parts))
		}
		if parts[0] != "00" {
			t.Errorf("expected version '00', got '%s'", parts[0])
		}
		if parts[1] != span.TraceID {
			t.Errorf("expected trace ID '%s', got '%s'", span.TraceID, parts[1])
		}
		if parts[2] != span.SpanID {
			t.Errorf("expected span ID '%s', got '%s'", span.SpanID, parts[2])
		}
		if parts[3] != "01" {
			t.Errorf("expected flags '01', got '%s'", parts[3])
		}
	})

	t.Run("inject from extracted context", func(t *testing.T) {
		incomingHeaders := http.Header{}
		incomingHeaders.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

		ctx := Extract(context.Background(), incomingHeaders)

		outgoingHeaders := http.Header{}
		Inject(ctx, outgoingHeaders)

		traceparent := outgoingHeaders.Get("traceparent")
		if traceparent != "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01" {
			t.Errorf("expected original traceparent, got '%s'", traceparent)
		}
	})

	t.Run("child span continues trace from extracted context", func(t *testing.T) {
		incomingHeaders := http.Header{}
		incomingHeaders.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

		ctx := Extract(context.Background(), incomingHeaders)

		tracer := NewTracer("test-service")
		_, span := tracer.Start(ctx, "child-operation")
		defer span.End()

		if span.TraceID != "0af7651916cd43dd8448eb211c80319c" {
			t.Errorf("child should inherit trace ID, got '%s'", span.TraceID)
		}
		if span.ParentID != "b7ad6b7169203331" {
			t.Errorf("child's parent ID should be extracted span ID, got '%s'", span.ParentID)
		}
	})
}

func TestTraceIDFromContext(t *testing.T) {
	t.Run("from span", func(t *testing.T) {
		tracer := NewTracer("test-service")
		ctx, span := tracer.Start(context.Background(), "test")
		defer span.End()

		traceID := TraceIDFromContext(ctx)
		if traceID != span.TraceID {
			t.Errorf("expected trace ID '%s', got '%s'", span.TraceID, traceID)
		}
	})

	t.Run("from span context", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
		ctx := Extract(context.Background(), headers)

		traceID := TraceIDFromContext(ctx)
		if traceID != "0af7651916cd43dd8448eb211c80319c" {
			t.Errorf("expected trace ID '0af7651916cd43dd8448eb211c80319c', got '%s'", traceID)
		}
	})

	t.Run("empty context", func(t *testing.T) {
		traceID := TraceIDFromContext(context.Background())
		if traceID != "" {
			t.Errorf("expected empty trace ID, got '%s'", traceID)
		}
	})
}

func TestStdoutExporter(t *testing.T) {
	t.Run("exports spans as JSON", func(t *testing.T) {
		var buf bytes.Buffer
		exporter := NewStdoutExporter(WithWriter(&buf))

		tracer := NewTracer("test-service", WithExporter(exporter), WithBatchSize(1))
		_, span := tracer.Start(context.Background(), "test-operation")
		span.SetAttribute("key", "value")
		span.AddEvent("test-event")
		span.End()

		// Flush to ensure export completes before reading buffer
		if err := tracer.Flush(); err != nil {
			t.Fatalf("flush failed: %v", err)
		}

		output := buf.String()
		if output == "" {
			t.Fatal("expected output, got empty string")
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["name"] != "test-operation" {
			t.Errorf("expected name 'test-operation', got '%v'", result["name"])
		}
		if result["traceId"] == "" {
			t.Error("traceId should not be empty")
		}
	})

	t.Run("pretty print", func(t *testing.T) {
		var buf bytes.Buffer
		exporter := NewStdoutExporter(WithWriter(&buf), WithPrettyPrint())

		span := &Span{
			TraceID:   "abc123",
			SpanID:    "def456",
			Name:      "test",
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
		if err := exporter.Export([]*Span{span}); err != nil {
			t.Fatalf("export failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "\n  ") {
			t.Error("expected pretty-printed output with indentation")
		}
	})
}

func TestW3CTraceContextPropagator(t *testing.T) {
	propagator := NewW3CTraceContextPropagator()

	t.Run("extract via carrier", func(t *testing.T) {
		carrier := HeaderCarrier(http.Header{})
		carrier.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

		ctx := propagator.Extract(context.Background(), carrier)
		sc := SpanContextFromContext(ctx)

		if !sc.IsValid() {
			t.Error("span context should be valid")
		}
		if sc.TraceID != "0af7651916cd43dd8448eb211c80319c" {
			t.Error("trace ID mismatch")
		}
	})

	t.Run("inject via carrier", func(t *testing.T) {
		tracer := NewTracer("test-service")
		ctx, span := tracer.Start(context.Background(), "test")
		defer span.End()

		carrier := HeaderCarrier(http.Header{})
		propagator.Inject(ctx, carrier)

		if carrier.Get("traceparent") == "" {
			t.Error("traceparent should be set")
		}
	})
}

func TestSampler(t *testing.T) {
	t.Run("always sample", func(t *testing.T) {
		sampler := AlwaysSample{}
		if !sampler.ShouldSample("any-trace-id") {
			t.Error("AlwaysSample should return true")
		}
	})

	t.Run("never sample", func(t *testing.T) {
		sampler := NeverSample{}
		if sampler.ShouldSample("any-trace-id") {
			t.Error("NeverSample should return false")
		}
	})

	t.Run("ratio sampler bounds", func(t *testing.T) {
		// Ratio 0 should never sample
		s0 := NewRatioSampler(0)
		if s0.ShouldSample("0af7651916cd43dd8448eb211c80319c") {
			t.Error("ratio 0 should never sample")
		}

		// Ratio 1 should always sample
		s1 := NewRatioSampler(1)
		if !s1.ShouldSample("0af7651916cd43dd8448eb211c80319c") {
			t.Error("ratio 1 should always sample")
		}

		// Negative ratio should be clamped to 0
		sNeg := NewRatioSampler(-0.5)
		if sNeg.ShouldSample("0af7651916cd43dd8448eb211c80319c") {
			t.Error("negative ratio should be clamped to 0")
		}

		// Ratio > 1 should be clamped to 1
		sOver := NewRatioSampler(1.5)
		if !sOver.ShouldSample("0af7651916cd43dd8448eb211c80319c") {
			t.Error("ratio > 1 should be clamped to 1")
		}
	})

	t.Run("never sampler creates non-recording span", func(t *testing.T) {
		tracer := NewTracer("test-service", WithSampler(NeverSample{}))
		_, span := tracer.Start(context.Background(), "test")

		// Span should exist but not record
		if span == nil {
			t.Fatal("span should not be nil")
		}
		if span.IsRecording() {
			t.Error("unsampled span should not be recording")
		}

		// Operations should be no-ops
		span.SetAttribute("key", "value")
		// Attributes may or may not be set depending on implementation
		span.End()
	})
}

func TestConcurrentSpanOperations(t *testing.T) {
	tracer := NewTracer("test-service")
	_, span := tracer.Start(context.Background(), "test")
	defer span.End()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			span.SetAttribute("key", "value")
			span.AddEvent("event")
			span.SetStatus(StatusOK, "ok")
		}(i)
	}
	wg.Wait()

	// Should not panic or produce corrupt data
	if len(span.Events) < 1 {
		t.Error("expected at least one event")
	}
}

func TestTracerFlush(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewStdoutExporter(WithWriter(&buf))
	tracer := NewTracer("test-service", WithExporter(exporter), WithBatchSize(100))

	// Create a few spans
	for i := 0; i < 5; i++ {
		_, span := tracer.Start(context.Background(), "test")
		span.End()
	}

	// Flush should export buffered spans
	if err := tracer.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// Should have 5 JSON lines
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}
}

func TestTracerShutdown(t *testing.T) {
	var buf bytes.Buffer
	exporter := NewStdoutExporter(WithWriter(&buf))
	tracer := NewTracer("test-service", WithExporter(exporter), WithBatchSize(100))

	_, span := tracer.Start(context.Background(), "test")
	span.End()

	// Shutdown should flush and close
	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("shutdown should have flushed spans")
	}
}

func TestNoopExporter(t *testing.T) {
	exporter := NewNoopExporter()

	spans := []*Span{{TraceID: "test", SpanID: "test"}}
	if err := exporter.Export(spans); err != nil {
		t.Errorf("noop export should not error: %v", err)
	}

	if err := exporter.Shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown should not error: %v", err)
	}
}

func TestSpanContext(t *testing.T) {
	t.Run("valid span context", func(t *testing.T) {
		sc := SpanContext{
			TraceID: "0af7651916cd43dd8448eb211c80319c",
			SpanID:  "b7ad6b7169203331",
		}
		if !sc.IsValid() {
			t.Error("span context with trace and span ID should be valid")
		}
	})

	t.Run("invalid span context - missing trace ID", func(t *testing.T) {
		sc := SpanContext{SpanID: "b7ad6b7169203331"}
		if sc.IsValid() {
			t.Error("span context without trace ID should be invalid")
		}
	})

	t.Run("invalid span context - missing span ID", func(t *testing.T) {
		sc := SpanContext{TraceID: "0af7651916cd43dd8448eb211c80319c"}
		if sc.IsValid() {
			t.Error("span context without span ID should be invalid")
		}
	})
}

func TestSpanSpanContext(t *testing.T) {
	tracer := NewTracer("test-service")
	_, span := tracer.Start(context.Background(), "test")
	defer span.End()

	sc := span.SpanContext()
	if sc.TraceID != span.TraceID {
		t.Errorf("expected trace ID '%s', got '%s'", span.TraceID, sc.TraceID)
	}
	if sc.SpanID != span.SpanID {
		t.Errorf("expected span ID '%s', got '%s'", span.SpanID, sc.SpanID)
	}
	if !sc.Sampled {
		t.Error("sampled should be true for recording span")
	}
}

func BenchmarkSpanCreation(b *testing.B) {
	tracer := NewTracer("bench-service")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, span := tracer.Start(ctx, "benchmark-span")
			span.End()
		}
	})
}

func BenchmarkSpanWithAttributes(b *testing.B) {
	tracer := NewTracer("bench-service")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, span := tracer.Start(ctx, "benchmark-span")
			span.SetAttribute("http.method", "GET")
			span.SetAttribute("http.url", "/api/users")
			span.SetAttribute("http.status_code", "200")
			span.End()
		}
	})
}

func BenchmarkExtract(b *testing.B) {
	headers := http.Header{}
	headers.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Extract(ctx, headers)
	}
}

func BenchmarkInject(b *testing.B) {
	tracer := NewTracer("bench-service")
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		headers := http.Header{}
		Inject(ctx, headers)
	}
}
