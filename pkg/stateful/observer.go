package stateful

import "time"

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
// This is a simple in-memory implementation for monitoring.
type MetricsObserver struct {
	CreateCount  int64
	ReadCount    int64
	ListCount    int64
	UpdateCount  int64
	DeleteCount  int64
	ErrorCount   int64
	ResetCount   int64
	TotalLatency time.Duration
}

func (m *MetricsObserver) OnCreate(resource string, itemID string, duration time.Duration) {
	m.CreateCount++
	m.TotalLatency += duration
}

func (m *MetricsObserver) OnRead(resource string, itemID string, duration time.Duration) {
	m.ReadCount++
	m.TotalLatency += duration
}

func (m *MetricsObserver) OnList(resource string, count int, duration time.Duration) {
	m.ListCount++
	m.TotalLatency += duration
}

func (m *MetricsObserver) OnUpdate(resource string, itemID string, duration time.Duration) {
	m.UpdateCount++
	m.TotalLatency += duration
}

func (m *MetricsObserver) OnDelete(resource string, itemID string, duration time.Duration) {
	m.DeleteCount++
	m.TotalLatency += duration
}

func (m *MetricsObserver) OnError(resource string, operation string, err error) {
	m.ErrorCount++
}

func (m *MetricsObserver) OnReset(resources []string, duration time.Duration) {
	m.ResetCount++
	m.TotalLatency += duration
}

// Snapshot returns a copy of the current metrics.
func (m *MetricsObserver) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		CreateCount:  m.CreateCount,
		ReadCount:    m.ReadCount,
		ListCount:    m.ListCount,
		UpdateCount:  m.UpdateCount,
		DeleteCount:  m.DeleteCount,
		ErrorCount:   m.ErrorCount,
		ResetCount:   m.ResetCount,
		TotalLatency: m.TotalLatency,
	}
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
