package metrics

import (
	"runtime"
	"runtime/pprof"
	"time"
)

// RuntimeCollector collects Go runtime metrics.
type RuntimeCollector struct {
	// Gauges
	goroutines    *Gauge
	threads       *Gauge
	heapAlloc     *Gauge
	heapSys       *Gauge
	heapIdle      *Gauge
	heapInuse     *Gauge
	heapObjects   *Gauge
	stackInuse    *Gauge
	gcPauseNs     *Gauge
	gcLastPauseNs *Gauge
	numGC         *Gauge
	goInfo        *Gauge

	// Uptime gauge (passed in from defaults)
	uptime *Gauge

	// Start time for uptime calculation
	startTime time.Time
}

// NewRuntimeCollector creates a new runtime metrics collector and registers metrics.
// The uptimeGauge parameter should be the UptimeSeconds gauge from defaults.
func NewRuntimeCollector(r *Registry, uptimeGauge *Gauge) *RuntimeCollector {
	rc := &RuntimeCollector{
		startTime: time.Now(),
		uptime:    uptimeGauge,

		goroutines: r.NewGauge(
			"go_goroutines",
			"Number of goroutines that currently exist",
		),
		threads: r.NewGauge(
			"go_threads",
			"Number of OS threads created",
		),
		heapAlloc: r.NewGauge(
			"go_memstats_heap_alloc_bytes",
			"Number of heap bytes allocated and still in use",
		),
		heapSys: r.NewGauge(
			"go_memstats_heap_sys_bytes",
			"Number of heap bytes obtained from system",
		),
		heapIdle: r.NewGauge(
			"go_memstats_heap_idle_bytes",
			"Number of heap bytes waiting to be used",
		),
		heapInuse: r.NewGauge(
			"go_memstats_heap_inuse_bytes",
			"Number of heap bytes that are in use",
		),
		heapObjects: r.NewGauge(
			"go_memstats_heap_objects",
			"Number of allocated heap objects",
		),
		stackInuse: r.NewGauge(
			"go_memstats_stack_inuse_bytes",
			"Number of bytes in use by the stack allocator",
		),
		gcPauseNs: r.NewGauge(
			"go_gc_duration_seconds",
			"Total GC pause duration in seconds",
		),
		gcLastPauseNs: r.NewGauge(
			"go_gc_last_pause_seconds",
			"Duration of the last GC pause in seconds",
		),
		numGC: r.NewGauge(
			"go_gc_cycles_total",
			"Total number of completed GC cycles",
		),
		goInfo: r.NewGauge(
			"go_info",
			"Information about the Go environment",
			"version",
		),
	}

	// Set static info
	if vec, err := rc.goInfo.WithLabels(runtime.Version()); err == nil {
		vec.Set(1)
	}

	return rc
}

// Collect updates all runtime metrics with current values.
// Call this periodically (e.g., every few seconds) to keep metrics current.
func (rc *RuntimeCollector) Collect() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// Uptime
	_ = rc.uptime.Set(time.Since(rc.startTime).Seconds())

	// Goroutines and threads
	_ = rc.goroutines.Set(float64(runtime.NumGoroutine()))

	// OS thread count from pprof threadcreate profile
	if numThreads, ok := getNumThreads(); ok {
		_ = rc.threads.Set(float64(numThreads))
	}

	// Heap metrics
	_ = rc.heapAlloc.Set(float64(mem.HeapAlloc))
	_ = rc.heapSys.Set(float64(mem.HeapSys))
	_ = rc.heapIdle.Set(float64(mem.HeapIdle))
	_ = rc.heapInuse.Set(float64(mem.HeapInuse))
	_ = rc.heapObjects.Set(float64(mem.HeapObjects))
	_ = rc.stackInuse.Set(float64(mem.StackInuse))

	// GC metrics — use PauseTotalNs which is the authoritative cumulative total,
	// rather than summing the PauseNs circular buffer which wraps after 256 entries.
	_ = rc.gcPauseNs.Set(float64(mem.PauseTotalNs) / 1e9) // Convert to seconds

	if mem.NumGC > 0 {
		lastPause := mem.PauseNs[(mem.NumGC-1)%256]
		_ = rc.gcLastPauseNs.Set(float64(lastPause) / 1e9)
	}
	_ = rc.numGC.Set(float64(mem.NumGC))
}

// getNumThreads returns the number of OS threads via the pprof
// "threadcreate" profile, which tracks threads created by the runtime.
func getNumThreads() (int, bool) {
	p := pprof.Lookup("threadcreate")
	if p == nil {
		return 0, false
	}
	return p.Count(), true
}

// StartCollector starts a goroutine that periodically collects runtime metrics.
// Returns a stop function to cancel the collection.
func (rc *RuntimeCollector) StartCollector(interval time.Duration) func() {
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Collect immediately
		rc.Collect()

		for {
			select {
			case <-ticker.C:
				rc.Collect()
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}
