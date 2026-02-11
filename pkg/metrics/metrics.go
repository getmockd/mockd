package metrics

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// ErrLabelCountMismatch is returned when the number of label values doesn't match the defined labels.
var ErrLabelCountMismatch = errors.New("label count mismatch")

// ErrNegativeCounterValue is returned when attempting to add a negative value to a counter.
var ErrNegativeCounterValue = errors.New("counter cannot be decreased")

// ErrDuplicateMetric is returned when registering a metric with a name that is already registered.
var ErrDuplicateMetric = errors.New("duplicate metric name")

// atomicFloat64 provides atomic operations for float64 values.
// It stores the bits of the float64 as a uint64 for atomic access.
type atomicFloat64 struct {
	bits uint64
}

// Load atomically loads and returns the float64 value.
func (a *atomicFloat64) Load() float64 {
	return math.Float64frombits(atomic.LoadUint64(&a.bits))
}

// Store atomically stores the float64 value.
func (a *atomicFloat64) Store(val float64) {
	atomic.StoreUint64(&a.bits, math.Float64bits(val))
}

// Add atomically adds delta to the float64 value using CAS loop.
func (a *atomicFloat64) Add(delta float64) {
	for {
		old := atomic.LoadUint64(&a.bits)
		newVal := math.Float64frombits(old) + delta
		if atomic.CompareAndSwapUint64(&a.bits, old, math.Float64bits(newVal)) {
			return
		}
	}
}

// MetricType represents the type of a metric.
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

// Metric is the interface implemented by all metric types.
type Metric interface {
	// Name returns the metric name.
	Name() string
	// Help returns the help text.
	Help() string
	// Type returns the metric type.
	Type() MetricType
	// Collect returns all metric samples for exposition.
	Collect() []Sample
}

// Sample represents a single metric sample with labels.
type Sample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// ============================================================================
// Counter
// ============================================================================

// Counter is a monotonically increasing metric.
// It can only increase or be reset to zero.
type Counter struct {
	name       string
	help       string
	labelNames []string
	mu         sync.RWMutex
	values     map[string]*counterValue
}

type counterValue struct {
	labels map[string]string
	value  atomicFloat64
}

// NewCounter creates a new counter. Should be called via Registry.NewCounter.
func newCounter(name, help string, labelNames []string) *Counter {
	return &Counter{
		name:       name,
		help:       help,
		labelNames: labelNames,
		values:     make(map[string]*counterValue),
	}
}

// Name returns the metric name.
func (c *Counter) Name() string { return c.name }

// Help returns the help text.
func (c *Counter) Help() string { return c.help }

// Type returns the metric type.
func (c *Counter) Type() MetricType { return MetricTypeCounter }

// WithLabels returns a CounterVec for the given label values.
// The number of values must match the number of label names.
// Returns an error if the label count doesn't match.
//
//nolint:dupl // structural similarity with Gauge.WithLabels is intentional
func (c *Counter) WithLabels(values ...string) (*CounterVec, error) {
	if len(values) != len(c.labelNames) {
		return nil, fmt.Errorf("%w: counter %s expected %d labels, got %d", ErrLabelCountMismatch, c.name, len(c.labelNames), len(values))
	}

	key := labelsKey(values)
	c.mu.RLock()
	cv, ok := c.values[key]
	c.mu.RUnlock()

	if !ok {
		labels := make(map[string]string, len(c.labelNames))
		for i, name := range c.labelNames {
			labels[name] = values[i]
		}

		c.mu.Lock()
		// Double-check after acquiring write lock
		cv, ok = c.values[key]
		if !ok {
			cv = &counterValue{labels: labels}
			c.values[key] = cv
		}
		c.mu.Unlock()
	}

	return &CounterVec{cv: cv}, nil
}

// Inc increments the counter by 1 (for counters without labels).
func (c *Counter) Inc() error {
	return c.Add(1)
}

// Add adds the given value to the counter (for counters without labels).
// Returns an error if delta is negative.
func (c *Counter) Add(delta float64) error {
	if delta < 0 {
		return fmt.Errorf("%w: counter %s", ErrNegativeCounterValue, c.name)
	}
	vec, err := c.WithLabels()
	if err != nil {
		return err
	}
	return vec.Add(delta)
}

// Collect returns all metric samples.
func (c *Counter) Collect() []Sample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	samples := make([]Sample, 0, len(c.values))
	for _, cv := range c.values {
		samples = append(samples, Sample{
			Name:   c.name,
			Labels: cv.labels,
			Value:  cv.value.Load(),
		})
	}
	return samples
}

// CounterVec provides methods for a specific label combination.
type CounterVec struct {
	cv *counterValue
}

// Inc increments the counter by 1.
func (v *CounterVec) Inc() error {
	return v.Add(1)
}

// Add adds the given value to the counter.
// Returns an error if delta is negative.
func (v *CounterVec) Add(delta float64) error {
	if delta < 0 {
		return ErrNegativeCounterValue
	}
	v.cv.value.Add(delta)
	return nil
}

// ============================================================================
// Gauge
// ============================================================================

// Gauge is a metric that can arbitrarily go up and down.
type Gauge struct {
	name       string
	help       string
	labelNames []string
	mu         sync.RWMutex
	values     map[string]*gaugeValue
}

type gaugeValue struct {
	labels map[string]string
	value  atomicFloat64
}

// newGauge creates a new gauge. Should be called via Registry.NewGauge.
func newGauge(name, help string, labelNames []string) *Gauge {
	return &Gauge{
		name:       name,
		help:       help,
		labelNames: labelNames,
		values:     make(map[string]*gaugeValue),
	}
}

// Name returns the metric name.
func (g *Gauge) Name() string { return g.name }

// Help returns the help text.
func (g *Gauge) Help() string { return g.help }

// Type returns the metric type.
func (g *Gauge) Type() MetricType { return MetricTypeGauge }

// WithLabels returns a GaugeVec for the given label values.
// Returns an error if the label count doesn't match.
//
//nolint:dupl // structural similarity with Counter.WithLabels is intentional
func (g *Gauge) WithLabels(values ...string) (*GaugeVec, error) {
	if len(values) != len(g.labelNames) {
		return nil, fmt.Errorf("%w: gauge %s expected %d labels, got %d", ErrLabelCountMismatch, g.name, len(g.labelNames), len(values))
	}

	key := labelsKey(values)
	g.mu.RLock()
	gv, ok := g.values[key]
	g.mu.RUnlock()

	if !ok {
		labels := make(map[string]string, len(g.labelNames))
		for i, name := range g.labelNames {
			labels[name] = values[i]
		}

		g.mu.Lock()
		gv, ok = g.values[key]
		if !ok {
			gv = &gaugeValue{labels: labels}
			g.values[key] = gv
		}
		g.mu.Unlock()
	}

	return &GaugeVec{gv: gv}, nil
}

// Set sets the gauge to the given value (for gauges without labels).
func (g *Gauge) Set(value float64) error {
	vec, err := g.WithLabels()
	if err != nil {
		return err
	}
	vec.Set(value)
	return nil
}

// Inc increments the gauge by 1 (for gauges without labels).
func (g *Gauge) Inc() error {
	return g.Add(1)
}

// Dec decrements the gauge by 1 (for gauges without labels).
func (g *Gauge) Dec() error {
	return g.Add(-1)
}

// Add adds the given value to the gauge (for gauges without labels).
func (g *Gauge) Add(delta float64) error {
	vec, err := g.WithLabels()
	if err != nil {
		return err
	}
	vec.Add(delta)
	return nil
}

// Collect returns all metric samples.
func (g *Gauge) Collect() []Sample {
	g.mu.RLock()
	defer g.mu.RUnlock()

	samples := make([]Sample, 0, len(g.values))
	for _, gv := range g.values {
		samples = append(samples, Sample{
			Name:   g.name,
			Labels: gv.labels,
			Value:  gv.value.Load(),
		})
	}
	return samples
}

// GaugeVec provides methods for a specific label combination.
type GaugeVec struct {
	gv *gaugeValue
}

// Set sets the gauge to the given value.
func (v *GaugeVec) Set(value float64) {
	v.gv.value.Store(value)
}

// Inc increments the gauge by 1.
func (v *GaugeVec) Inc() {
	v.Add(1)
}

// Dec decrements the gauge by 1.
func (v *GaugeVec) Dec() {
	v.Add(-1)
}

// Add adds the given value to the gauge.
func (v *GaugeVec) Add(delta float64) {
	v.gv.value.Add(delta)
}

// ============================================================================
// Histogram
// ============================================================================

// Histogram tracks the distribution of observed values.
// It provides buckets for counting values and sum/count aggregations.
type Histogram struct {
	name       string
	help       string
	labelNames []string
	buckets    []float64
	mu         sync.RWMutex
	values     map[string]*histogramValue
}

type histogramValue struct {
	labels  map[string]string
	buckets []float64     // Upper bounds
	counts  []uint64      // Atomic counters per bucket
	sum     atomicFloat64 // Sum of all observed values
	count   uint64        // Total count (atomic)
}

// newHistogram creates a new histogram. Should be called via Registry.NewHistogram.
func newHistogram(name, help string, buckets []float64, labelNames []string) *Histogram {
	// Ensure buckets are sorted
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	// Add +Inf bucket if not present
	if len(sortedBuckets) == 0 || sortedBuckets[len(sortedBuckets)-1] != math.Inf(1) {
		sortedBuckets = append(sortedBuckets, math.Inf(1))
	}

	return &Histogram{
		name:       name,
		help:       help,
		labelNames: labelNames,
		buckets:    sortedBuckets,
		values:     make(map[string]*histogramValue),
	}
}

// Name returns the metric name.
func (h *Histogram) Name() string { return h.name }

// Help returns the help text.
func (h *Histogram) Help() string { return h.help }

// Type returns the metric type.
func (h *Histogram) Type() MetricType { return MetricTypeHistogram }

// WithLabels returns a HistogramVec for the given label values.
// Returns an error if the label count doesn't match.
func (h *Histogram) WithLabels(values ...string) (*HistogramVec, error) {
	if len(values) != len(h.labelNames) {
		return nil, fmt.Errorf("%w: histogram %s expected %d labels, got %d", ErrLabelCountMismatch, h.name, len(h.labelNames), len(values))
	}

	key := labelsKey(values)
	h.mu.RLock()
	hv, ok := h.values[key]
	h.mu.RUnlock()

	if !ok {
		labels := make(map[string]string, len(h.labelNames))
		for i, name := range h.labelNames {
			labels[name] = values[i]
		}

		h.mu.Lock()
		hv, ok = h.values[key]
		if !ok {
			hv = &histogramValue{
				labels:  labels,
				buckets: h.buckets,
				counts:  make([]uint64, len(h.buckets)),
			}
			h.values[key] = hv
		}
		h.mu.Unlock()
	}

	return &HistogramVec{hv: hv}, nil
}

// Observe records a value in the histogram (for histograms without labels).
func (h *Histogram) Observe(value float64) error {
	vec, err := h.WithLabels()
	if err != nil {
		return err
	}
	vec.Observe(value)
	return nil
}

// Collect returns all metric samples.
func (h *Histogram) Collect() []Sample {
	h.mu.RLock()
	defer h.mu.RUnlock()

	samples := make([]Sample, 0, (len(h.buckets)+2)*len(h.values))
	for _, hv := range h.values {
		// Emit bucket samples
		cumulativeCount := uint64(0)
		for i, bound := range hv.buckets {
			cumulativeCount += atomic.LoadUint64(&hv.counts[i])
			bucketLabels := make(map[string]string, len(hv.labels)+1)
			for k, v := range hv.labels {
				bucketLabels[k] = v
			}
			if math.IsInf(bound, 1) {
				bucketLabels["le"] = "+Inf"
			} else {
				bucketLabels["le"] = formatFloat(bound)
			}
			samples = append(samples, Sample{
				Name:   h.name + "_bucket",
				Labels: bucketLabels,
				Value:  float64(cumulativeCount),
			})
		}

		// Emit _sum
		samples = append(samples, Sample{
			Name:   h.name + "_sum",
			Labels: hv.labels,
			Value:  hv.sum.Load(),
		})

		// Emit _count
		samples = append(samples, Sample{
			Name:   h.name + "_count",
			Labels: hv.labels,
			Value:  float64(atomic.LoadUint64(&hv.count)),
		})
	}
	return samples
}

// HistogramVec provides methods for a specific label combination.
type HistogramVec struct {
	hv *histogramValue
}

// Observe records a value in the histogram.
func (v *HistogramVec) Observe(value float64) {
	// Find the bucket and increment
	for i, bound := range v.hv.buckets {
		if value <= bound {
			atomic.AddUint64(&v.hv.counts[i], 1)
			break
		}
	}
	v.hv.sum.Add(value)
	atomic.AddUint64(&v.hv.count, 1)
}

// ============================================================================
// Registry
// ============================================================================

// Registry holds all registered metrics.
type Registry struct {
	mu      sync.RWMutex
	metrics []Metric
	names   map[string]struct{} // guards against duplicate registrations
}

// NewRegistry creates a new metric registry.
func NewRegistry() *Registry {
	return &Registry{
		metrics: make([]Metric, 0),
		names:   make(map[string]struct{}),
	}
}

// NewCounter creates and registers a new counter.
func (r *Registry) NewCounter(name, help string, labels ...string) *Counter {
	c := newCounter(name, help, labels)
	r.register(c)
	return c
}

// NewGauge creates and registers a new gauge.
func (r *Registry) NewGauge(name, help string, labels ...string) *Gauge {
	g := newGauge(name, help, labels)
	r.register(g)
	return g
}

// NewHistogram creates and registers a new histogram with the given buckets.
func (r *Registry) NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	h := newHistogram(name, help, buckets, labels)
	r.register(h)
	return h
}

// register adds a metric to the registry.
// It panics if a metric with the same name is already registered,
// since duplicate metric names produce invalid Prometheus output.
func (r *Registry) register(m Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.names[m.Name()]; exists {
		panic(fmt.Sprintf("%s: %s", ErrDuplicateMetric, m.Name()))
	}
	r.names[m.Name()] = struct{}{}
	r.metrics = append(r.metrics, m)
}

// Handler returns an http.Handler that serves the /metrics endpoint.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.RLock()
		metrics := make([]Metric, len(r.metrics))
		copy(metrics, r.metrics)
		r.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		for _, m := range metrics {
			writeMetric(w, m)
		}
	})
}

// ============================================================================
// Prometheus Text Format Writer
// ============================================================================

// writeMetric writes a single metric in Prometheus text format.
func writeMetric(w http.ResponseWriter, m Metric) {
	samples := m.Collect()
	if len(samples) == 0 {
		return
	}

	// Write HELP line
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n", m.Name(), escapeHelp(m.Help()))

	// Write TYPE line
	_, _ = fmt.Fprintf(w, "# TYPE %s %s\n", m.Name(), m.Type())

	// Write samples
	for _, s := range samples {
		writeSample(w, s)
	}
}

// writeSample writes a single sample line.
func writeSample(w http.ResponseWriter, s Sample) {
	if len(s.Labels) == 0 {
		_, _ = fmt.Fprintf(w, "%s %s\n", s.Name, formatFloat(s.Value))
	} else {
		_, _ = fmt.Fprintf(w, "%s{%s} %s\n", s.Name, formatLabels(s.Labels), formatFloat(s.Value))
	}
}

// formatLabels formats labels as key="value",key="value"
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%q", k, escapeLabelValue(labels[k]))
	}
	return strings.Join(parts, ",")
}

// formatFloat formats a float64 for Prometheus output.
func formatFloat(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	if math.IsInf(v, -1) {
		return "-Inf"
	}
	// Use %g for compact representation, but ensure integer values have decimal
	s := fmt.Sprintf("%g", v)
	// Prometheus prefers explicit format for whole numbers
	if v == float64(int64(v)) && !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		return fmt.Sprintf("%.0f", v)
	}
	return s
}

// escapeHelp escapes help text for Prometheus format.
func escapeHelp(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// escapeLabelValue escapes label values for Prometheus format.
func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// labelsKey generates a unique key for a set of label values.
func labelsKey(values []string) string {
	return strings.Join(values, "\x00")
}

// ============================================================================
// Default Buckets
// ============================================================================

// DefaultBuckets are the default histogram buckets for request durations (in seconds).
var DefaultBuckets = []float64{
	0.001, // 1ms
	0.005, // 5ms
	0.01,  // 10ms
	0.025, // 25ms
	0.05,  // 50ms
	0.1,   // 100ms
	0.25,  // 250ms
	0.5,   // 500ms
	1,     // 1s
	2.5,   // 2.5s
	5,     // 5s
	10,    // 10s
}
