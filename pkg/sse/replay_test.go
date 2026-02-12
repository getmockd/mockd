package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// flushRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
	mu      sync.Mutex
	flushed int
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (f *flushRecorder) Flush() {
	f.mu.Lock()
	f.flushed++
	f.mu.Unlock()
}

func (f *flushRecorder) FlushCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.flushed
}

// Ensure flushRecorder satisfies both interfaces.
var _ http.ResponseWriter = (*flushRecorder)(nil)
var _ http.Flusher = (*flushRecorder)(nil)

// nonFlushWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type nonFlushWriter struct {
	header http.Header
}

func (w *nonFlushWriter) Header() http.Header         { return w.header }
func (w *nonFlushWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *nonFlushWriter) WriteHeader(int)             {}

// makeTestRecording creates a StreamRecording with SSE events for testing.
func makeTestRecording(eventCount int) *recording.StreamRecording {
	events := make([]recording.SSEEvent, eventCount)
	for i := 0; i < eventCount; i++ {
		events[i] = recording.SSEEvent{
			Sequence:   int64(i + 1),
			Timestamp:  time.Now(),
			RelativeMs: int64(i * 100), // 100ms apart
			EventType:  "message",
			Data:       "event-data",
			ID:         "",
			DataSize:   len("event-data"),
		}
	}

	return &recording.StreamRecording{
		ID:       "test-recording-1",
		Protocol: recording.ProtocolSSE,
		SSE: &recording.SSERecordingData{
			Events: events,
		},
	}
}

// ---------- NewSSEReplayer validation tests ----------

func TestNewSSEReplayer(t *testing.T) {
	t.Run("nil_recording_returns_error", func(t *testing.T) {
		w := newFlushRecorder()
		_, err := NewSSEReplayer(nil, w, DefaultSSEReplayConfig())
		if err != ErrSSEInvalidRecording {
			t.Errorf("expected ErrSSEInvalidRecording, got %v", err)
		}
	})

	t.Run("wrong_protocol_returns_error", func(t *testing.T) {
		w := newFlushRecorder()
		rec := &recording.StreamRecording{
			Protocol: recording.ProtocolWebSocket,
			SSE:      &recording.SSERecordingData{Events: []recording.SSEEvent{{Data: "x"}}},
		}
		_, err := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())
		if err != ErrSSEInvalidRecording {
			t.Errorf("expected ErrSSEInvalidRecording, got %v", err)
		}
	})

	t.Run("nil_SSE_data_returns_error", func(t *testing.T) {
		w := newFlushRecorder()
		rec := &recording.StreamRecording{
			Protocol: recording.ProtocolSSE,
			SSE:      nil,
		}
		_, err := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())
		if err != ErrSSENoEventsToReplay {
			t.Errorf("expected ErrSSENoEventsToReplay, got %v", err)
		}
	})

	t.Run("empty_events_returns_error", func(t *testing.T) {
		w := newFlushRecorder()
		rec := &recording.StreamRecording{
			Protocol: recording.ProtocolSSE,
			SSE:      &recording.SSERecordingData{Events: []recording.SSEEvent{}},
		}
		_, err := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())
		if err != ErrSSENoEventsToReplay {
			t.Errorf("expected ErrSSENoEventsToReplay, got %v", err)
		}
	})

	t.Run("non_flusher_returns_error", func(t *testing.T) {
		w := &nonFlushWriter{header: make(http.Header)}
		rec := makeTestRecording(1)
		_, err := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())
		if err == nil {
			t.Fatal("expected error for non-flusher writer")
		}
		if err.Error() != "streaming not supported" {
			t.Errorf("expected 'streaming not supported', got %q", err.Error())
		}
	})

	t.Run("valid_recording_creates_replayer", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(3)
		replayer, err := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if replayer == nil {
			t.Fatal("expected non-nil replayer")
		}
		if replayer.Status() != recording.ReplayStatusPending {
			t.Errorf("expected status Pending, got %v", replayer.Status())
		}
	})

	t.Run("invalid_mode_defaults_to_pure", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(1)
		config := SSEReplayConfig{Mode: "invalid-mode"}
		replayer, err := NewSSEReplayer(rec, w, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if replayer.config.Mode != recording.ReplayModePure {
			t.Errorf("expected mode to default to pure, got %q", replayer.config.Mode)
		}
	})

	t.Run("zero_timing_scale_defaults_to_1", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(1)
		config := SSEReplayConfig{Mode: recording.ReplayModePure, TimingScale: 0}
		replayer, err := NewSSEReplayer(rec, w, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if replayer.config.TimingScale != 1.0 {
			t.Errorf("expected TimingScale=1.0, got %f", replayer.config.TimingScale)
		}
	})

	t.Run("negative_timing_scale_defaults_to_1", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(1)
		config := SSEReplayConfig{Mode: recording.ReplayModePure, TimingScale: -2.0}
		replayer, err := NewSSEReplayer(rec, w, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if replayer.config.TimingScale != 1.0 {
			t.Errorf("expected TimingScale=1.0, got %f", replayer.config.TimingScale)
		}
	})

	t.Run("triggered_mode_creates_advance_channel", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(2)
		config := SSEReplayConfig{Mode: recording.ReplayModeTriggered}
		replayer, err := NewSSEReplayer(rec, w, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if replayer.advanceChan == nil {
			t.Error("expected advanceChan to be initialized for triggered mode")
		}
	})
}

// ---------- DefaultSSEReplayConfig test ----------

func TestDefaultSSEReplayConfig(t *testing.T) {
	config := DefaultSSEReplayConfig()
	if config.Mode != recording.ReplayModePure {
		t.Errorf("expected Mode=pure, got %q", config.Mode)
	}
	if config.TimingScale != 1.0 {
		t.Errorf("expected TimingScale=1.0, got %f", config.TimingScale)
	}
	if config.InitialDelay != 0 {
		t.Errorf("expected InitialDelay=0, got %d", config.InitialDelay)
	}
}

// ---------- SetSSEHeaders test ----------

func TestSetSSEHeaders(t *testing.T) {
	w := newFlushRecorder()
	rec := makeTestRecording(1)
	replayer, _ := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())

	replayer.SetSSEHeaders()

	headers := w.Header()
	if ct := headers.Get("Content-Type"); ct != ContentTypeEventStream {
		t.Errorf("expected Content-Type=%q, got %q", ContentTypeEventStream, ct)
	}
	if cc := headers.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control=no-cache, got %q", cc)
	}
	if conn := headers.Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection=keep-alive, got %q", conn)
	}
	if xab := headers.Get("X-Accel-Buffering"); xab != "no" {
		t.Errorf("expected X-Accel-Buffering=no, got %q", xab)
	}
}

// ---------- Pure mode replay tests ----------

func TestReplayPureMode(t *testing.T) {
	t.Run("sends_all_events_in_order", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(3)
		// Use very high timing scale so test doesn't actually wait
		config := SSEReplayConfig{
			Mode:        recording.ReplayModePure,
			TimingScale: 1000.0, // 1000x speed — 100ms becomes 0.1ms
		}
		replayer, err := NewSSEReplayer(rec, w, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = replayer.Start()
		if err != nil {
			t.Fatalf("Start() error: %v", err)
		}

		body := w.Body.String()
		// Each event should produce "event:message\ndata:event-data\n\n"
		count := strings.Count(body, "data:event-data\n")
		if count != 3 {
			t.Errorf("expected 3 data lines, got %d in body:\n%s", count, body)
		}

		// Verify status is complete
		if replayer.Status() != recording.ReplayStatusComplete {
			t.Errorf("expected status Complete, got %v", replayer.Status())
		}
	})

	t.Run("progress_shows_all_sent", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(2)
		config := SSEReplayConfig{
			Mode:        recording.ReplayModePure,
			TimingScale: 1000.0,
		}
		replayer, _ := NewSSEReplayer(rec, w, config)
		_ = replayer.Start()

		current, total, sent := replayer.Progress()
		if current != 2 {
			t.Errorf("expected currentEvent=2, got %d", current)
		}
		if total != 2 {
			t.Errorf("expected totalEvents=2, got %d", total)
		}
		if sent != 2 {
			t.Errorf("expected eventsSent=2, got %d", sent)
		}
	})

	t.Run("flushes_after_each_event", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(3)
		config := SSEReplayConfig{
			Mode:        recording.ReplayModePure,
			TimingScale: 1000.0,
		}
		replayer, _ := NewSSEReplayer(rec, w, config)
		_ = replayer.Start()

		flushCount := w.FlushCount()
		if flushCount != 3 {
			t.Errorf("expected 3 flushes, got %d", flushCount)
		}
	})

	t.Run("sets_headers_automatically", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(1)
		config := SSEReplayConfig{
			Mode:        recording.ReplayModePure,
			TimingScale: 1000.0,
		}
		replayer, _ := NewSSEReplayer(rec, w, config)
		_ = replayer.Start()

		if ct := w.Header().Get("Content-Type"); ct != ContentTypeEventStream {
			t.Errorf("Start() should set SSE headers, got Content-Type=%q", ct)
		}
	})
}

// ---------- Start twice test ----------

func TestStartTwiceReturnsError(t *testing.T) {
	w := newFlushRecorder()
	rec := makeTestRecording(1)
	config := SSEReplayConfig{
		Mode:        recording.ReplayModePure,
		TimingScale: 1000.0,
	}
	replayer, _ := NewSSEReplayer(rec, w, config)

	// First start completes
	err := replayer.Start()
	if err != nil {
		t.Fatalf("first Start() error: %v", err)
	}

	// Second start should fail
	err = replayer.Start()
	if err != ErrSSEReplayAlreadyStarted {
		t.Errorf("expected ErrSSEReplayAlreadyStarted, got %v", err)
	}
}

// ---------- Advance in pure mode test ----------

func TestAdvanceInPureModeReturnsError(t *testing.T) {
	w := newFlushRecorder()
	rec := makeTestRecording(2)
	config := SSEReplayConfig{
		Mode:        recording.ReplayModePure,
		TimingScale: 1000.0,
	}
	replayer, _ := NewSSEReplayer(rec, w, config)

	// Start and finish playback first
	_ = replayer.Start()

	_, err := replayer.Advance(1)
	if err != ErrSSETriggeredModeOnly {
		t.Errorf("expected ErrSSETriggeredModeOnly, got %v", err)
	}
}

// ---------- Advance before start test ----------

func TestAdvanceBeforeStartReturnsError(t *testing.T) {
	w := newFlushRecorder()
	rec := makeTestRecording(2)
	config := SSEReplayConfig{Mode: recording.ReplayModeTriggered}
	replayer, _ := NewSSEReplayer(rec, w, config)

	_, err := replayer.Advance(1)
	if err != ErrSSEReplayNotStarted {
		t.Errorf("expected ErrSSEReplayNotStarted, got %v", err)
	}
}

// ---------- Stop test ----------

func TestStopCancelsPlayback(t *testing.T) {
	w := newFlushRecorder()
	// Make many events with real timing to give us time to stop
	events := make([]recording.SSEEvent, 100)
	for i := range events {
		events[i] = recording.SSEEvent{
			Sequence:   int64(i + 1),
			RelativeMs: int64(i * 50), // 50ms apart
			EventType:  "tick",
			Data:       "ping",
			DataSize:   4,
		}
	}
	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolSSE,
		SSE:      &recording.SSERecordingData{Events: events},
	}

	config := SSEReplayConfig{
		Mode:        recording.ReplayModePure,
		TimingScale: 1.0, // real-time — will be slow enough to interrupt
	}
	replayer, err := NewSSEReplayer(rec, w, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- replayer.Start()
	}()

	// Give it a moment to start sending, then stop
	time.Sleep(20 * time.Millisecond)
	replayer.Stop()

	err = <-done
	if err != ErrSSEReplayStopped {
		t.Errorf("expected ErrSSEReplayStopped, got %v", err)
	}

	status := replayer.Status()
	if status != recording.ReplayStatusAborted {
		t.Errorf("expected status Aborted, got %v", status)
	}

	// Should not have sent all 100 events
	_, _, sent := replayer.Progress()
	if sent >= 100 {
		t.Errorf("expected fewer than 100 events sent after stop, got %d", sent)
	}
}

// ---------- GetProgress test ----------

func TestGetProgress(t *testing.T) {
	t.Run("before_start", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(5)
		replayer, _ := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())

		p := replayer.GetProgress()
		if p.CurrentEvent != 0 {
			t.Errorf("expected CurrentEvent=0, got %d", p.CurrentEvent)
		}
		if p.TotalEvents != 5 {
			t.Errorf("expected TotalEvents=5, got %d", p.TotalEvents)
		}
		if p.EventsSent != 0 {
			t.Errorf("expected EventsSent=0, got %d", p.EventsSent)
		}
		if p.Status != recording.ReplayStatusPending {
			t.Errorf("expected status Pending, got %v", p.Status)
		}
		if p.Elapsed != 0 {
			t.Errorf("expected Elapsed=0 before start, got %v", p.Elapsed)
		}
	})

	t.Run("after_complete", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(3)
		config := SSEReplayConfig{
			Mode:        recording.ReplayModePure,
			TimingScale: 1000.0,
		}
		replayer, _ := NewSSEReplayer(rec, w, config)
		_ = replayer.Start()

		p := replayer.GetProgress()
		if p.CurrentEvent != 3 {
			t.Errorf("expected CurrentEvent=3, got %d", p.CurrentEvent)
		}
		if p.TotalEvents != 3 {
			t.Errorf("expected TotalEvents=3, got %d", p.TotalEvents)
		}
		if p.EventsSent != 3 {
			t.Errorf("expected EventsSent=3, got %d", p.EventsSent)
		}
		if p.Status != recording.ReplayStatusComplete {
			t.Errorf("expected status Complete, got %v", p.Status)
		}
		if p.Elapsed <= 0 {
			t.Errorf("expected positive Elapsed, got %v", p.Elapsed)
		}
	})
}

// ---------- Context test ----------

func TestReplayerContext(t *testing.T) {
	w := newFlushRecorder()
	rec := makeTestRecording(1)
	replayer, _ := NewSSEReplayer(rec, w, DefaultSSEReplayConfig())

	ctx := replayer.Context()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Context should not be cancelled yet
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done before Stop")
	default:
		// good
	}

	replayer.Stop()

	// Context should now be cancelled
	select {
	case <-ctx.Done():
		// good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be done after Stop")
	}
}

// ---------- Triggered mode tests ----------

func TestTriggeredMode(t *testing.T) {
	t.Run("advance_sends_requested_events", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(5)
		config := SSEReplayConfig{Mode: recording.ReplayModeTriggered}
		replayer, err := NewSSEReplayer(rec, w, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		done := make(chan error, 1)
		go func() {
			done <- replayer.Start()
		}()

		// Wait for replayer to enter waiting state
		time.Sleep(20 * time.Millisecond)

		// Advance 2 events
		sent, err := replayer.Advance(2)
		if err != nil {
			t.Fatalf("Advance error: %v", err)
		}
		if sent != 2 {
			t.Errorf("expected to send 2, got %d", sent)
		}

		// Give time for events to be processed
		time.Sleep(20 * time.Millisecond)

		_, _, eventsSent := replayer.Progress()
		if eventsSent != 2 {
			t.Errorf("expected 2 events sent, got %d", eventsSent)
		}

		// Advance remaining 3
		sent, err = replayer.Advance(3)
		if err != nil {
			t.Fatalf("Advance error: %v", err)
		}
		if sent != 3 {
			t.Errorf("expected to send 3, got %d", sent)
		}

		// Wait for completion
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Start() returned error: %v", err)
			}
		case <-time.After(2 * time.Second):
			replayer.Stop()
			t.Fatal("timeout waiting for replay to complete")
		}

		if replayer.Status() != recording.ReplayStatusComplete {
			t.Errorf("expected status Complete, got %v", replayer.Status())
		}

		body := w.Body.String()
		count := strings.Count(body, "data:event-data\n")
		if count != 5 {
			t.Errorf("expected 5 data lines, got %d", count)
		}
	})

	t.Run("advance_caps_at_remaining_events", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(2)
		config := SSEReplayConfig{Mode: recording.ReplayModeTriggered}
		replayer, _ := NewSSEReplayer(rec, w, config)

		done := make(chan error, 1)
		go func() {
			done <- replayer.Start()
		}()

		time.Sleep(20 * time.Millisecond)

		// Request more than available
		sent, err := replayer.Advance(100)
		if err != nil {
			t.Fatalf("Advance error: %v", err)
		}
		if sent != 2 {
			t.Errorf("expected capped to 2, got %d", sent)
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			replayer.Stop()
			t.Fatal("timeout")
		}
	})

	t.Run("stop_interrupts_triggered_mode", func(t *testing.T) {
		w := newFlushRecorder()
		rec := makeTestRecording(10)
		config := SSEReplayConfig{Mode: recording.ReplayModeTriggered}
		replayer, _ := NewSSEReplayer(rec, w, config)

		done := make(chan error, 1)
		go func() {
			done <- replayer.Start()
		}()

		time.Sleep(20 * time.Millisecond)

		// Send only 2, then stop
		_, _ = replayer.Advance(2)
		time.Sleep(20 * time.Millisecond)
		replayer.Stop()

		err := <-done
		if err != ErrSSEReplayStopped {
			t.Errorf("expected ErrSSEReplayStopped, got %v", err)
		}
	})
}
