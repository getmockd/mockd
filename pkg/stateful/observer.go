package stateful

import (
	"sync/atomic"
	"time"
)

// Observer defines hooks for observability and metrics collection.
// Implementations can use these hooks to collect metrics, log operations,
// or integrate with observability platforms like Prometheus, DataDog, etc.
type Observer interface {
	// OnCreate is called after a successful create operation.
	OnCreate(resource string, itemID string, duration time.Duration)

	// OnRead is called after a successful read operation.
	OnRead(resource string, itemID string, duration time.Duration)

	// OnList is called after a successful list operation.
	OnList(resource string, count int, duration time.Duration)

	// OnUpdate is called after a successful update operation.
	OnUpdate(resource string, itemID string, duration time.Duration)

	// OnDelete is called after a successful delete operation.
	OnDelete(resource string, itemID string, duration time.Duration)

	// OnError is called when an operation fails.
	OnError(resource string, operation string, err error)

	// OnReset is called after a state reset.
	OnReset(resources []string, duration time.Duration)
}

// NoopObserver is a no-op implementation of Observer for when metrics are disabled.
type NoopObserver struct{}

func (n *NoopObserver) OnCreate(resource string, itemID string, duration time.Duration) {}
func (n *NoopObserver) OnRead(resource string, itemID string, duration time.Duration)   {}
func (n *NoopObserver) OnList(resource string, count int, duration time.Duration)       {}
func (n *NoopObserver) OnUpdate(resource string, itemID string, duration time.Duration) {}
func (n *NoopObserver) OnDelete(resource string, itemID string, duration time.Duration) {}
func (n *NoopObserver) OnError(resource string, operation string, err error)            {}
func (n *NoopObserver) OnReset(resources []string, duration time.Duration)              {}

// MetricsObserver collects basic metrics about stateful operations.
// This is a thread-safe in-memory implementation for monitoring.
// All counters use atomic operations to prevent race conditions
// when called from multiple goroutines concurrently.
type MetricsObserver struct {
	createCount    atomic.Int64
	readCount      atomic.Int64
	listCount      atomic.Int64
	updateCount    atomic.Int64
	deleteCount    atomic.Int64
	errorCount     atomic.Int64
	resetCount     atomic.Int64
	totalLatencyNs atomic.Int64 // stored as nanoseconds for atomic operations
}

// NewMetricsObserver creates a new thread-safe metrics observer.
func NewMetricsObserver() *MetricsObserver {
	return &MetricsObserver{}
}

func (m *MetricsObserver) OnCreate(resource string, itemID string, duration time.Duration) {
	m.createCount.Add(1)
	m.totalLatencyNs.Add(int64(duration))
}

func (m *MetricsObserver) OnRead(resource string, itemID string, duration time.Duration) {
	m.readCount.Add(1)
	m.totalLatencyNs.Add(int64(duration))
}

func (m *MetricsObserver) OnList(resource string, count int, duration time.Duration) {
	m.listCount.Add(1)
	m.totalLatencyNs.Add(int64(duration))
}

func (m *MetricsObserver) OnUpdate(resource string, itemID string, duration time.Duration) {
	m.updateCount.Add(1)
	m.totalLatencyNs.Add(int64(duration))
}

func (m *MetricsObserver) OnDelete(resource string, itemID string, duration time.Duration) {
	m.deleteCount.Add(1)
	m.totalLatencyNs.Add(int64(duration))
}

func (m *MetricsObserver) OnError(resource string, operation string, err error) {
	m.errorCount.Add(1)
}

func (m *MetricsObserver) OnReset(resources []string, duration time.Duration) {
	m.resetCount.Add(1)
	m.totalLatencyNs.Add(int64(duration))
}

// Snapshot returns a thread-safe copy of the current metrics.
func (m *MetricsObserver) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		CreateCount:  m.createCount.Load(),
		ReadCount:    m.readCount.Load(),
		ListCount:    m.listCount.Load(),
		UpdateCount:  m.updateCount.Load(),
		DeleteCount:  m.deleteCount.Load(),
		ErrorCount:   m.errorCount.Load(),
		ResetCount:   m.resetCount.Load(),
		TotalLatency: time.Duration(m.totalLatencyNs.Load()),
	}
}

// Reset clears all metrics counters to zero.
func (m *MetricsObserver) Reset() {
	m.createCount.Store(0)
	m.readCount.Store(0)
	m.listCount.Store(0)
	m.updateCount.Store(0)
	m.deleteCount.Store(0)
	m.errorCount.Store(0)
	m.resetCount.Store(0)
	m.totalLatencyNs.Store(0)
}

// MetricsSnapshot is a point-in-time snapshot of metrics.
type MetricsSnapshot struct {
	CreateCount  int64         `json:"createCount"`
	ReadCount    int64         `json:"readCount"`
	ListCount    int64         `json:"listCount"`
	UpdateCount  int64         `json:"updateCount"`
	DeleteCount  int64         `json:"deleteCount"`
	ErrorCount   int64         `json:"errorCount"`
	ResetCount   int64         `json:"resetCount"`
	TotalLatency time.Duration `json:"totalLatencyNs"`
}

// TotalOperations returns the total number of successful operations.
func (s MetricsSnapshot) TotalOperations() int64 {
	return s.CreateCount + s.ReadCount + s.ListCount + s.UpdateCount + s.DeleteCount
}
