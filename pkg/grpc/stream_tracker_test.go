package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamTracker_RegisterUnregister(t *testing.T) {
	tracker := NewStreamTracker()

	id, ctx, cancel := tracker.Register(context.Background(), "/pkg.Svc/Method", streamServerStream, "127.0.0.1:1234")
	defer cancel()

	assert.NotEmpty(t, id)
	assert.Equal(t, 1, tracker.Count())

	info := tracker.Get(id)
	require.NotNil(t, info)
	assert.Equal(t, "/pkg.Svc/Method", info.Method)
	assert.Equal(t, streamServerStream, info.StreamType)
	assert.Equal(t, "127.0.0.1:1234", info.ClientAddr)
	assert.False(t, info.ConnectedAt.IsZero())

	// Context should not be cancelled yet
	assert.NoError(t, ctx.Err())

	tracker.Unregister(id)
	assert.Equal(t, 0, tracker.Count())
	assert.Nil(t, tracker.Get(id))
}

func TestStreamTracker_MessageCounting(t *testing.T) {
	tracker := NewStreamTracker()

	id, _, cancel := tracker.Register(context.Background(), "/pkg.Svc/Stream", streamBidi, "")
	defer cancel()

	tracker.RecordSent(id)
	tracker.RecordSent(id)
	tracker.RecordRecv(id)

	info := tracker.Get(id)
	require.NotNil(t, info)
	assert.Equal(t, int64(2), info.MessagesSent)
	assert.Equal(t, int64(1), info.MessagesRecv)

	// Verify stats include live counters
	stats := tracker.Stats()
	assert.Equal(t, int64(2), stats.TotalMsgSent)
	assert.Equal(t, int64(1), stats.TotalMsgRecv)
	assert.Equal(t, 1, stats.ActiveStreams)

	// After unregister, counters roll into lifetime totals
	tracker.Unregister(id)
	stats = tracker.Stats()
	assert.Equal(t, 0, stats.ActiveStreams)
	assert.Equal(t, int64(2), stats.TotalMsgSent)
	assert.Equal(t, int64(1), stats.TotalMsgRecv)
}

func TestStreamTracker_Cancel(t *testing.T) {
	tracker := NewStreamTracker()

	id, ctx, cancel := tracker.Register(context.Background(), "/pkg.Svc/Method", streamServerStream, "")
	defer cancel()

	// Cancel the stream
	err := tracker.Cancel(id)
	assert.NoError(t, err)

	// Context should be cancelled
	assert.Error(t, ctx.Err())

	// Cancel non-existent stream
	err = tracker.Cancel("non-existent")
	assert.Error(t, err)
}

func TestStreamTracker_CancelAll(t *testing.T) {
	tracker := NewStreamTracker()

	_, ctx1, cancel1 := tracker.Register(context.Background(), "/pkg.Svc/A", streamServerStream, "")
	defer cancel1()
	_, ctx2, cancel2 := tracker.Register(context.Background(), "/pkg.Svc/B", streamBidi, "")
	defer cancel2()

	assert.Equal(t, 2, tracker.Count())

	n := tracker.CancelAll()
	assert.Equal(t, 2, n)

	assert.Error(t, ctx1.Err())
	assert.Error(t, ctx2.Err())
}

func TestStreamTracker_List(t *testing.T) {
	tracker := NewStreamTracker()

	id1, _, cancel1 := tracker.Register(context.Background(), "/pkg.Svc/A", streamServerStream, "1.2.3.4:100")
	defer cancel1()
	id2, _, cancel2 := tracker.Register(context.Background(), "/pkg.Svc/B", streamClientStream, "5.6.7.8:200")
	defer cancel2()

	list := tracker.List()
	assert.Len(t, list, 2)

	ids := map[string]bool{}
	for _, info := range list {
		ids[info.ID] = true
	}
	assert.True(t, ids[id1])
	assert.True(t, ids[id2])
}

func TestStreamTracker_Stats(t *testing.T) {
	tracker := NewStreamTracker()

	// Record some unary RPCs
	tracker.RecordUnaryRPC()
	tracker.RecordUnaryRPC()

	// Register a stream
	id, _, cancel := tracker.Register(context.Background(), "/pkg.Svc/Stream", streamServerStream, "")
	defer cancel()
	tracker.RecordSent(id)

	stats := tracker.Stats()
	assert.Equal(t, 1, stats.ActiveStreams)
	assert.Equal(t, int64(1), stats.TotalStreams)
	// TotalRPCs = unary (2) + streams (1) = 3
	assert.Equal(t, int64(3), stats.TotalRPCs)
	assert.Equal(t, int64(1), stats.TotalMsgSent)
	assert.Equal(t, map[string]int{"/pkg.Svc/Stream": 1}, stats.StreamsByMethod)
}

func TestStreamTracker_RecordOnNonExistent(t *testing.T) {
	tracker := NewStreamTracker()

	// Should not panic
	tracker.RecordSent("non-existent")
	tracker.RecordRecv("non-existent")
}

func TestStreamTracker_UnregisterIdempotent(t *testing.T) {
	tracker := NewStreamTracker()

	id, _, cancel := tracker.Register(context.Background(), "/pkg.Svc/Method", streamBidi, "")
	defer cancel()

	tracker.Unregister(id)
	tracker.Unregister(id) // Should not panic
	assert.Equal(t, 0, tracker.Count())
}

func TestStreamTracker_Close(t *testing.T) {
	tracker := NewStreamTracker()

	_, ctx, cancel := tracker.Register(context.Background(), "/pkg.Svc/Method", streamBidi, "")
	defer cancel()

	tracker.Close()
	assert.Error(t, ctx.Err())
}

func TestStreamTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewStreamTracker()
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			id, _, cancel := tracker.Register(ctx, "/pkg.Svc/Method", streamBidi, "")
			tracker.RecordSent(id)
			tracker.RecordRecv(id)
			time.Sleep(time.Microsecond)
			tracker.Unregister(id)
			cancel()
		}
	}()

	for i := 0; i < 100; i++ {
		_ = tracker.List()
		_ = tracker.Stats()
		_ = tracker.Count()
	}

	<-done
}
