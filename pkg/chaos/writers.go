package chaos

import (
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// SlowWriter writes bytes slowly to simulate bandwidth limiting
type SlowWriter struct {
	w              http.ResponseWriter
	bytesPerSecond int
	mu             sync.Mutex
}

// Header returns the header map
func (sw *SlowWriter) Header() http.Header {
	return sw.w.Header()
}

// WriteHeader writes the status code
func (sw *SlowWriter) WriteHeader(statusCode int) {
	sw.w.WriteHeader(statusCode)
}

// Write writes bytes slowly based on the configured bandwidth limit
func (sw *SlowWriter) Write(p []byte) (int, error) {
	if sw.bytesPerSecond <= 0 {
		return sw.w.Write(p)
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	totalWritten := 0

	for len(p) > 0 {
		// Calculate how many bytes we can write
		chunkSize := sw.bytesPerSecond
		if chunkSize > len(p) {
			chunkSize = len(p)
		}

		// Write chunk
		n, err := sw.w.Write(p[:chunkSize])
		totalWritten += n
		if err != nil {
			return totalWritten, err
		}
		p = p[n:]

		// Flush if possible
		if f, ok := sw.w.(http.Flusher); ok {
			f.Flush()
		}

		// Sleep to maintain bandwidth limit based on actual bytes written
		if len(p) > 0 {
			sleepDuration := time.Second * time.Duration(n) / time.Duration(sw.bytesPerSecond)
			time.Sleep(sleepDuration)
		}
	}

	return totalWritten, nil
}

// Flush flushes the underlying writer if it supports flushing
func (sw *SlowWriter) Flush() {
	if f, ok := sw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter
func (sw *SlowWriter) Unwrap() http.ResponseWriter {
	return sw.w
}

// CorruptingWriter corrupts random bytes in the response
type CorruptingWriter struct {
	w           http.ResponseWriter
	corruptRate float64
	rng         *rand.Rand // Each CorruptingWriter has its own rng instance, no mutex needed
}

// Header returns the header map
func (cw *CorruptingWriter) Header() http.Header {
	return cw.w.Header()
}

// WriteHeader writes the status code
func (cw *CorruptingWriter) WriteHeader(statusCode int) {
	cw.w.WriteHeader(statusCode)
}

// Write corrupts random bytes before writing
func (cw *CorruptingWriter) Write(p []byte) (int, error) {
	if cw.corruptRate <= 0 {
		return cw.w.Write(p)
	}

	// Make a copy to avoid modifying the original slice
	corrupted := make([]byte, len(p))
	copy(corrupted, p)

	// Corrupt random bytes (each CorruptingWriter has its own rng, so no mutex needed)
	for i := range corrupted {
		if cw.rng.Float64() < cw.corruptRate {
			corrupted[i] = byte(cw.rng.Intn(256))
		}
	}

	return cw.w.Write(corrupted)
}

// Flush flushes the underlying writer if it supports flushing
func (cw *CorruptingWriter) Flush() {
	if f, ok := cw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter
func (cw *CorruptingWriter) Unwrap() http.ResponseWriter {
	return cw.w
}

// TruncatingWriter truncates response at a specified point
type TruncatingWriter struct {
	w        http.ResponseWriter
	maxBytes int
	written  int
	mu       sync.Mutex
}

// Header returns the header map
func (tw *TruncatingWriter) Header() http.Header {
	return tw.w.Header()
}

// WriteHeader writes the status code
func (tw *TruncatingWriter) WriteHeader(statusCode int) {
	tw.w.WriteHeader(statusCode)
}

// Write writes bytes up to the maximum allowed
func (tw *TruncatingWriter) Write(p []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.written >= tw.maxBytes {
		// Pretend we wrote everything but don't actually write
		return len(p), nil
	}

	remaining := tw.maxBytes - tw.written
	if len(p) > remaining {
		p = p[:remaining]
	}

	n, err := tw.w.Write(p)
	tw.written += n
	return n, err
}

// Flush flushes the underlying writer if it supports flushing
func (tw *TruncatingWriter) Flush() {
	if f, ok := tw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter
func (tw *TruncatingWriter) Unwrap() http.ResponseWriter {
	return tw.w
}

// BytesWritten returns the number of bytes written
func (tw *TruncatingWriter) BytesWritten() int {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.written
}

// DelayedWriter delays the first write by a specified duration
type DelayedWriter struct {
	w          http.ResponseWriter
	delay      time.Duration
	firstWrite bool
	mu         sync.Mutex
}

// NewDelayedWriter creates a writer that delays the first write
func NewDelayedWriter(w http.ResponseWriter, delay time.Duration) *DelayedWriter {
	return &DelayedWriter{
		w:          w,
		delay:      delay,
		firstWrite: true,
	}
}

// Header returns the header map
func (dw *DelayedWriter) Header() http.Header {
	return dw.w.Header()
}

// WriteHeader writes the status code after delay on first call
func (dw *DelayedWriter) WriteHeader(statusCode int) {
	dw.maybeDelay()
	dw.w.WriteHeader(statusCode)
}

// Write delays on first write then writes normally
func (dw *DelayedWriter) Write(p []byte) (int, error) {
	dw.maybeDelay()
	return dw.w.Write(p)
}

func (dw *DelayedWriter) maybeDelay() {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if dw.firstWrite {
		dw.firstWrite = false
		time.Sleep(dw.delay)
	}
}

// Flush flushes the underlying writer if it supports flushing
func (dw *DelayedWriter) Flush() {
	if f, ok := dw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter
func (dw *DelayedWriter) Unwrap() http.ResponseWriter {
	return dw.w
}
