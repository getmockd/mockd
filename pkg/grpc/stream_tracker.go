package grpc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getmockd/mockd/pkg/metrics"
)

// StreamInfo holds metadata about an active streaming RPC.
type StreamInfo struct {
	ID           string     `json:"id"`
	Method       string     `json:"method"`
	StreamType   streamType `json:"streamType"`
	ClientAddr   string     `json:"clientAddr,omitempty"`
	ConnectedAt  time.Time  `json:"connectedAt"`
	MessagesSent int64      `json:"messagesSent"`
	MessagesRecv int64      `json:"messagesRecv"`
}

// StreamStats holds aggregate statistics for gRPC streams.
type StreamStats struct {
	ActiveStreams    int              `json:"activeStreams"`
	TotalStreams     int64            `json:"totalStreams"`
	TotalRPCs       int64            `json:"totalRPCs"`
	TotalMsgSent    int64            `json:"totalMessagesSent"`
	TotalMsgRecv    int64            `json:"totalMessagesRecv"`
	StreamsByMethod map[string]int   `json:"streamsByMethod"`
}

// StreamTracker tracks active gRPC streaming RPCs for a single Server.
type StreamTracker struct {
	streams          map[string]*trackedStream // ID -> tracked stream
	adminCancelled   map[string]bool           // IDs cancelled by admin/mock-update
	mu               sync.RWMutex

	// Lifetime counters (include completed streams).
	totalStreams  atomic.Int64
	totalRPCs     atomic.Int64
	totalMsgSent  atomic.Int64
	totalMsgRecv  atomic.Int64
}

// trackedStream extends StreamInfo with internal bookkeeping.
type trackedStream struct {
	info     StreamInfo
	msgSent  atomic.Int64
	msgRecv  atomic.Int64
	cancel   context.CancelFunc
}

// NewStreamTracker creates a new StreamTracker.
func NewStreamTracker() *StreamTracker {
	return &StreamTracker{
		streams:        make(map[string]*trackedStream),
		adminCancelled: make(map[string]bool),
	}
}

// nextStreamID generates a short unique stream ID.
var streamIDCounter atomic.Int64

func nextStreamID() string {
	return fmt.Sprintf("grpc-stream-%d", streamIDCounter.Add(1))
}

// Register adds a new streaming RPC and returns its ID and a cancel-aware
// context. The caller should defer Unregister.
func (t *StreamTracker) Register(ctx context.Context, method string, st streamType, clientAddr string) (string, context.Context, context.CancelFunc) {
	id := nextStreamID()
	ctx, cancel := context.WithCancel(ctx) //nolint:gosec // cancel is stored in trackedStream.cancel

	ts := &trackedStream{
		info: StreamInfo{
			ID:          id,
			Method:      method,
			StreamType:  st,
			ClientAddr:  clientAddr,
			ConnectedAt: time.Now(),
		},
		cancel: cancel,
	}

	t.mu.Lock()
	t.streams[id] = ts
	t.mu.Unlock()

	t.totalStreams.Add(1)

	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("grpc"); err == nil {
			vec.Inc()
		}
	}

	return id, ctx, cancel
}

// Unregister removes a stream and accumulates its message counters.
func (t *StreamTracker) Unregister(id string) {
	t.mu.Lock()
	ts, ok := t.streams[id]
	if ok {
		delete(t.streams, id)
	}
	delete(t.adminCancelled, id)
	t.mu.Unlock()

	if !ok {
		return
	}

	t.totalMsgSent.Add(ts.msgSent.Load())
	t.totalMsgRecv.Add(ts.msgRecv.Load())

	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("grpc"); err == nil {
			vec.Dec()
		}
	}
}

// RecordSent increments the sent counter for a stream.
func (t *StreamTracker) RecordSent(id string) {
	t.mu.RLock()
	ts := t.streams[id]
	t.mu.RUnlock()
	if ts != nil {
		ts.msgSent.Add(1)
	}
}

// RecordRecv increments the received counter for a stream.
func (t *StreamTracker) RecordRecv(id string) {
	t.mu.RLock()
	ts := t.streams[id]
	t.mu.RUnlock()
	if ts != nil {
		ts.msgRecv.Add(1)
	}
}

// RecordUnaryRPC increments the total RPC counter for unary calls
// (which are not tracked as active streams).
func (t *StreamTracker) RecordUnaryRPC() {
	t.totalRPCs.Add(1)
}

// Get returns info about a specific stream, or nil.
func (t *StreamTracker) Get(id string) *StreamInfo {
	t.mu.RLock()
	ts := t.streams[id]
	t.mu.RUnlock()
	if ts == nil {
		return nil
	}
	return t.toInfo(ts)
}

// List returns info about all active streams.
func (t *StreamTracker) List() []*StreamInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*StreamInfo, 0, len(t.streams))
	for _, ts := range t.streams {
		result = append(result, t.toInfo(ts))
	}
	return result
}

// Count returns the number of active streams.
func (t *StreamTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.streams)
}

// Cancel cancels a specific stream's context, causing the RPC handler to
// return codes.Unavailable to the client.
func (t *StreamTracker) Cancel(id string) error {
	t.mu.Lock()
	ts := t.streams[id]
	if ts != nil {
		t.adminCancelled[id] = true
	}
	t.mu.Unlock()
	if ts == nil {
		return fmt.Errorf("stream %s not found", id)
	}
	ts.cancel()
	return nil
}

// WasAdminCancelled returns true if the stream was cancelled by an admin
// action (CancelAll for mock update, or explicit Cancel via admin API)
// rather than by the client.
func (t *StreamTracker) WasAdminCancelled(id string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.adminCancelled[id]
}

// CancelAll cancels all active streams with the given gRPC status.
// Returns the number of streams cancelled.
func (t *StreamTracker) CancelAll() int {
	t.mu.RLock()
	ids := make([]string, 0, len(t.streams))
	for id := range t.streams {
		ids = append(ids, id)
	}
	t.mu.RUnlock()

	count := 0
	for _, id := range ids {
		if t.Cancel(id) == nil {
			count++
		}
	}
	return count
}

// Stats returns aggregate statistics.
func (t *StreamTracker) Stats() *StreamStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var liveSent, liveRecv int64
	byMethod := make(map[string]int)
	for _, ts := range t.streams {
		liveSent += ts.msgSent.Load()
		liveRecv += ts.msgRecv.Load()
		byMethod[ts.info.Method]++
	}

	return &StreamStats{
		ActiveStreams:    len(t.streams),
		TotalStreams:     t.totalStreams.Load(),
		TotalRPCs:        t.totalRPCs.Load() + t.totalStreams.Load(),
		TotalMsgSent:    t.totalMsgSent.Load() + liveSent,
		TotalMsgRecv:    t.totalMsgRecv.Load() + liveRecv,
		StreamsByMethod: byMethod,
	}
}

// Close cancels all streams (used during server shutdown).
func (t *StreamTracker) Close() {
	t.CancelAll()
}

func (t *StreamTracker) toInfo(ts *trackedStream) *StreamInfo {
	return &StreamInfo{
		ID:           ts.info.ID,
		Method:       ts.info.Method,
		StreamType:   ts.info.StreamType,
		ClientAddr:   ts.info.ClientAddr,
		ConnectedAt:  ts.info.ConnectedAt,
		MessagesSent: ts.msgSent.Load(),
		MessagesRecv: ts.msgRecv.Load(),
	}
}

