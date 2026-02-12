package sse

import (
	"errors"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// mockSSEHook is a test double that records all method calls.
type mockSSEHook struct {
	id            string
	streamStarted bool
	streamEnded   bool
	completed     bool
	onErrorCalled bool
	lastError     error
	frames        []any
	onFrameErr    error // error to return from OnFrame
	onCompleteErr error // error to return from OnComplete
	callOrder     []string
}

func newMockSSEHook(id string) *mockSSEHook {
	return &mockSSEHook{id: id, callOrder: make([]string, 0)}
}

func (h *mockSSEHook) ID() string { return h.id }
func (h *mockSSEHook) OnStreamStart() {
	h.streamStarted = true
	h.callOrder = append(h.callOrder, "OnStreamStart")
}
func (h *mockSSEHook) OnStreamEnd() {
	h.streamEnded = true
	h.callOrder = append(h.callOrder, "OnStreamEnd")
}
func (h *mockSSEHook) OnFrame(frame any) error {
	h.frames = append(h.frames, frame)
	h.callOrder = append(h.callOrder, "OnFrame")
	return h.onFrameErr
}
func (h *mockSSEHook) OnComplete() error {
	h.completed = true
	h.callOrder = append(h.callOrder, "OnComplete")
	return h.onCompleteErr
}
func (h *mockSSEHook) OnError(err error) {
	h.onErrorCalled = true
	h.lastError = err
	h.callOrder = append(h.callOrder, "OnError")
}

var _ recording.SSERecordingHook = (*mockSSEHook)(nil)

// ---------- NewStreamRecorder tests ----------

func TestNewStreamRecorder(t *testing.T) {
	t.Run("calls_OnStreamStart_on_hook", func(t *testing.T) {
		hook := newMockSSEHook("test-1")
		stream := &SSEStream{ID: "stream-1"}

		rec := NewStreamRecorder(stream, hook)

		if !hook.streamStarted {
			t.Fatal("expected OnStreamStart to be called")
		}
		if rec.stream != stream {
			t.Fatal("expected recorder to reference the stream")
		}
		if rec.hook != hook {
			t.Fatal("expected recorder to reference the hook")
		}
		if rec.startTime.IsZero() {
			t.Fatal("expected startTime to be set")
		}
		if rec.eventSequence != 0 {
			t.Error("expected initial eventSequence to be 0")
		}
	})

	t.Run("sets_start_time_close_to_now", func(t *testing.T) {
		hook := newMockSSEHook("test-2")
		before := time.Now()
		rec := NewStreamRecorder(&SSEStream{}, hook)
		after := time.Now()

		if rec.startTime.Before(before) || rec.startTime.After(after) {
			t.Errorf("startTime %v not between %v and %v", rec.startTime, before, after)
		}
	})
}

// ---------- RecordEvent tests ----------

func TestRecordEvent(t *testing.T) {
	t.Run("increments_sequence_and_passes_to_hook", func(t *testing.T) {
		hook := newMockSSEHook("test-3")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		retry := 3000
		err := rec.RecordEvent("message", `{"hello":"world"}`, "evt-1", &retry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if rec.eventSequence != 1 {
			t.Errorf("expected eventSequence=1, got %d", rec.eventSequence)
		}
		if len(hook.frames) != 1 {
			t.Fatalf("expected 1 frame, got %d", len(hook.frames))
		}

		event, ok := hook.frames[0].(recording.SSEEvent)
		if !ok {
			t.Fatalf("expected SSEEvent, got %T", hook.frames[0])
		}
		if event.Sequence != 1 {
			t.Errorf("expected Sequence=1, got %d", event.Sequence)
		}
		if event.EventType != "message" {
			t.Errorf("expected EventType=message, got %q", event.EventType)
		}
		if event.Data != `{"hello":"world"}` {
			t.Errorf("expected Data=%q, got %q", `{"hello":"world"}`, event.Data)
		}
		if event.ID != "evt-1" {
			t.Errorf("expected ID=evt-1, got %q", event.ID)
		}
		if event.Retry == nil || *event.Retry != 3000 {
			t.Errorf("expected Retry=3000, got %v", event.Retry)
		}
		if event.DataSize != len(`{"hello":"world"}`) {
			t.Errorf("expected DataSize=%d, got %d", len(`{"hello":"world"}`), event.DataSize)
		}
	})

	t.Run("multiple_events_increment_sequence", func(t *testing.T) {
		hook := newMockSSEHook("test-4")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		for i := 0; i < 3; i++ {
			if err := rec.RecordEvent("", "data", "", nil); err != nil {
				t.Fatalf("event %d error: %v", i, err)
			}
		}

		if rec.eventSequence != 3 {
			t.Errorf("expected eventSequence=3, got %d", rec.eventSequence)
		}
		if len(hook.frames) != 3 {
			t.Fatalf("expected 3 frames, got %d", len(hook.frames))
		}

		for i, frame := range hook.frames {
			event := frame.(recording.SSEEvent)
			expectedSeq := int64(i + 1)
			if event.Sequence != expectedSeq {
				t.Errorf("frame %d: expected Sequence=%d, got %d", i, expectedSeq, event.Sequence)
			}
		}
	})

	t.Run("nil_hook_returns_nil", func(t *testing.T) {
		// Create a recorder with a hook, then nil it out to simulate nil safety path
		hook := newMockSSEHook("test-5")
		rec := NewStreamRecorder(&SSEStream{}, hook)
		rec.hook = nil // simulate nil hook

		err := rec.RecordEvent("test", "data", "id1", nil)
		if err != nil {
			t.Fatalf("expected nil error with nil hook, got %v", err)
		}
	})

	t.Run("propagates_hook_error", func(t *testing.T) {
		hook := newMockSSEHook("test-6")
		hook.onFrameErr = errors.New("frame storage full")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		err := rec.RecordEvent("msg", "data", "", nil)
		if err == nil {
			t.Fatal("expected error from hook")
		}
		if err.Error() != "frame storage full" {
			t.Errorf("expected 'frame storage full', got %q", err.Error())
		}
	})

	t.Run("nil_retry_passes_nil", func(t *testing.T) {
		hook := newMockSSEHook("test-7")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		_ = rec.RecordEvent("msg", "data", "", nil)

		event := hook.frames[0].(recording.SSEEvent)
		if event.Retry != nil {
			t.Errorf("expected nil Retry, got %v", event.Retry)
		}
	})
}

// ---------- RecordEventDef tests ----------

func TestRecordEventDef(t *testing.T) {
	t.Run("converts_string_data", func(t *testing.T) {
		hook := newMockSSEHook("def-1")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		err := rec.RecordEventDef(&SSEEventDef{
			Type: "chat",
			Data: "hello world",
			ID:   "e1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		event := hook.frames[0].(recording.SSEEvent)
		if event.Data != "hello world" {
			t.Errorf("expected Data='hello world', got %q", event.Data)
		}
		if event.EventType != "chat" {
			t.Errorf("expected EventType='chat', got %q", event.EventType)
		}
	})

	t.Run("converts_byte_slice_data", func(t *testing.T) {
		hook := newMockSSEHook("def-2")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		_ = rec.RecordEventDef(&SSEEventDef{
			Data: []byte("binary-ish data"),
		})

		event := hook.frames[0].(recording.SSEEvent)
		if event.Data != "binary-ish data" {
			t.Errorf("expected 'binary-ish data', got %q", event.Data)
		}
	})

	t.Run("marshals_struct_data_to_json", func(t *testing.T) {
		hook := newMockSSEHook("def-3")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		_ = rec.RecordEventDef(&SSEEventDef{
			Data: map[string]int{"count": 42},
		})

		event := hook.frames[0].(recording.SSEEvent)
		if event.Data != `{"count":42}` {
			t.Errorf("expected JSON, got %q", event.Data)
		}
	})

	t.Run("retry_gt_zero_becomes_pointer", func(t *testing.T) {
		hook := newMockSSEHook("def-4")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		_ = rec.RecordEventDef(&SSEEventDef{
			Data:  "x",
			Retry: 5000,
		})

		event := hook.frames[0].(recording.SSEEvent)
		if event.Retry == nil || *event.Retry != 5000 {
			t.Errorf("expected Retry=5000, got %v", event.Retry)
		}
	})

	t.Run("retry_zero_becomes_nil", func(t *testing.T) {
		hook := newMockSSEHook("def-5")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		_ = rec.RecordEventDef(&SSEEventDef{
			Data:  "x",
			Retry: 0,
		})

		event := hook.frames[0].(recording.SSEEvent)
		if event.Retry != nil {
			t.Errorf("expected nil Retry for zero value, got %v", event.Retry)
		}
	})

	t.Run("nil_hook_returns_nil", func(t *testing.T) {
		hook := newMockSSEHook("def-6")
		rec := NewStreamRecorder(&SSEStream{}, hook)
		rec.hook = nil

		err := rec.RecordEventDef(&SSEEventDef{Data: "test"})
		if err != nil {
			t.Fatalf("expected nil with nil hook, got %v", err)
		}
	})
}

// ---------- Complete tests ----------

func TestStreamRecorderComplete(t *testing.T) {
	t.Run("calls_OnStreamEnd_then_OnComplete", func(t *testing.T) {
		hook := newMockSSEHook("comp-1")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		err := rec.Complete()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !hook.streamEnded {
			t.Error("expected OnStreamEnd to be called")
		}
		if !hook.completed {
			t.Error("expected OnComplete to be called")
		}

		// Verify call order: OnStreamStart (from constructor), OnStreamEnd, OnComplete
		if len(hook.callOrder) < 3 {
			t.Fatalf("expected at least 3 calls, got %d: %v", len(hook.callOrder), hook.callOrder)
		}
		// Last two should be OnStreamEnd then OnComplete
		lastTwo := hook.callOrder[len(hook.callOrder)-2:]
		if lastTwo[0] != "OnStreamEnd" || lastTwo[1] != "OnComplete" {
			t.Errorf("expected [OnStreamEnd, OnComplete], got %v", lastTwo)
		}
	})

	t.Run("propagates_OnComplete_error", func(t *testing.T) {
		hook := newMockSSEHook("comp-2")
		hook.onCompleteErr = errors.New("complete failed")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		err := rec.Complete()
		if err == nil {
			t.Fatal("expected error from OnComplete")
		}
		if err.Error() != "complete failed" {
			t.Errorf("expected 'complete failed', got %q", err.Error())
		}
	})

	t.Run("nil_hook_returns_nil", func(t *testing.T) {
		hook := newMockSSEHook("comp-3")
		rec := NewStreamRecorder(&SSEStream{}, hook)
		rec.hook = nil

		err := rec.Complete()
		if err != nil {
			t.Fatalf("expected nil with nil hook, got %v", err)
		}
	})
}

// ---------- Error tests ----------

func TestStreamRecorderError(t *testing.T) {
	t.Run("calls_OnError_with_error", func(t *testing.T) {
		hook := newMockSSEHook("err-1")
		rec := NewStreamRecorder(&SSEStream{}, hook)

		testErr := errors.New("stream interrupted")
		rec.Error(testErr)

		if !hook.onErrorCalled {
			t.Fatal("expected OnError to be called")
		}
		if hook.lastError != testErr {
			t.Errorf("expected error %v, got %v", testErr, hook.lastError)
		}
	})

	t.Run("nil_hook_does_not_panic", func(t *testing.T) {
		hook := newMockSSEHook("err-2")
		rec := NewStreamRecorder(&SSEStream{}, hook)
		rec.hook = nil

		// Should not panic
		rec.Error(errors.New("test"))
	})
}

// ---------- Hook accessor test ----------

func TestStreamRecorderHook(t *testing.T) {
	hook := newMockSSEHook("hook-1")
	rec := NewStreamRecorder(&SSEStream{}, hook)

	got := rec.Hook()
	if got != hook {
		t.Error("Hook() did not return the expected hook")
	}
}

// ---------- Integration: full lifecycle ----------

func TestStreamRecorderFullLifecycle(t *testing.T) {
	hook := newMockSSEHook("lifecycle-1")
	rec := NewStreamRecorder(&SSEStream{ID: "stream-1"}, hook)

	// Record several events
	retry := 5000
	_ = rec.RecordEvent("update", `{"v":1}`, "1", nil)
	_ = rec.RecordEvent("update", `{"v":2}`, "2", &retry)
	_ = rec.RecordEventDef(&SSEEventDef{Type: "done", Data: "bye", ID: "3"})

	// Complete
	err := rec.Complete()
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify full call order
	expected := []string{
		"OnStreamStart", // from constructor
		"OnFrame",       // event 1
		"OnFrame",       // event 2
		"OnFrame",       // event 3 (from RecordEventDef)
		"OnStreamEnd",   // from Complete
		"OnComplete",    // from Complete
	}

	if len(hook.callOrder) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(hook.callOrder), hook.callOrder)
	}
	for i, exp := range expected {
		if hook.callOrder[i] != exp {
			t.Errorf("call %d: expected %q, got %q", i, exp, hook.callOrder[i])
		}
	}

	// Verify event sequences
	if len(hook.frames) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(hook.frames))
	}
	for i, frame := range hook.frames {
		event := frame.(recording.SSEEvent)
		if event.Sequence != int64(i+1) {
			t.Errorf("frame %d: expected seq=%d, got %d", i, i+1, event.Sequence)
		}
	}

	// Verify final sequence counter
	if rec.eventSequence != 3 {
		t.Errorf("expected final eventSequence=3, got %d", rec.eventSequence)
	}
}
