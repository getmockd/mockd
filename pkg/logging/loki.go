package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// LokiHandler is a slog.Handler that sends logs to Loki.
type LokiHandler struct {
	url        string
	labels     map[string]string
	client     *http.Client
	level      slog.Level
	attrs      []slog.Attr
	groups     []string
	mu         sync.Mutex
	batch      []lokiEntry
	batchSize  int
	flushTimer *time.Timer
}

type lokiEntry struct {
	timestamp time.Time
	line      string
}

// LokiStream represents a Loki log stream.
type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// LokiPush represents a Loki push request.
type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

// LokiOption configures a LokiHandler.
type LokiOption func(*LokiHandler)

// WithLokiLabels sets additional labels for logs.
func WithLokiLabels(labels map[string]string) LokiOption {
	return func(h *LokiHandler) {
		for k, v := range labels {
			h.labels[k] = v
		}
	}
}

// WithLokiLevel sets the minimum log level.
func WithLokiLevel(level slog.Level) LokiOption {
	return func(h *LokiHandler) {
		h.level = level
	}
}

// WithLokiBatchSize sets the batch size before flushing.
func WithLokiBatchSize(size int) LokiOption {
	return func(h *LokiHandler) {
		h.batchSize = size
	}
}

// NewLokiHandler creates a new Loki log handler.
// The url should be the Loki push endpoint (e.g., "http://localhost:3100/loki/api/v1/push").
func NewLokiHandler(url string, opts ...LokiOption) *LokiHandler {
	h := &LokiHandler{
		url:       url,
		labels:    map[string]string{"job": "mockd"},
		client:    &http.Client{Timeout: 5 * time.Second},
		level:     slog.LevelInfo,
		batchSize: 100,
	}

	for _, opt := range opts {
		opt(h)
	}

	// Start a background flush timer
	h.flushTimer = time.AfterFunc(5*time.Second, func() {
		_ = h.Flush()
		h.resetTimer()
	})

	return h
}

func (h *LokiHandler) resetTimer() {
	h.flushTimer.Reset(5 * time.Second)
}

// Enabled implements slog.Handler.
func (h *LokiHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle implements slog.Handler.
func (h *LokiHandler) Handle(_ context.Context, r slog.Record) error {
	// Build the log line as JSON
	line := h.formatRecord(r)

	h.mu.Lock()
	h.batch = append(h.batch, lokiEntry{
		timestamp: r.Time,
		line:      line,
	})

	shouldFlush := len(h.batch) >= h.batchSize
	h.mu.Unlock()

	if shouldFlush {
		go func() { _ = h.Flush() }()
	}

	return nil
}

// formatRecord formats a log record as a JSON string.
func (h *LokiHandler) formatRecord(r slog.Record) string {
	data := map[string]interface{}{
		"level": r.Level.String(),
		"msg":   r.Message,
		"time":  r.Time.Format(time.RFC3339Nano),
	}

	// Add handler-level attrs
	for _, attr := range h.attrs {
		data[attr.Key] = attr.Value.Any()
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		data[a.Key] = a.Value.Any()
		return true
	})

	b, _ := json.Marshal(data)
	return string(b)
}

// WithAttrs implements slog.Handler.
func (h *LokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := &LokiHandler{
		url:       h.url,
		labels:    h.labels,
		client:    h.client,
		level:     h.level,
		attrs:     append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...),
		groups:    h.groups,
		batchSize: h.batchSize,
	}
	return newHandler
}

// WithGroup implements slog.Handler.
func (h *LokiHandler) WithGroup(name string) slog.Handler {
	newHandler := &LokiHandler{
		url:       h.url,
		labels:    h.labels,
		client:    h.client,
		level:     h.level,
		attrs:     h.attrs,
		groups:    append(h.groups[:len(h.groups):len(h.groups)], name),
		batchSize: h.batchSize,
	}
	return newHandler
}

// Flush sends all buffered logs to Loki.
func (h *LokiHandler) Flush() error {
	h.mu.Lock()
	if len(h.batch) == 0 {
		h.mu.Unlock()
		return nil
	}

	// Take the batch
	batch := h.batch
	h.batch = nil
	h.mu.Unlock()

	// Build Loki push request
	values := make([][]string, len(batch))
	for i, entry := range batch {
		values[i] = []string{
			strconv.FormatInt(entry.timestamp.UnixNano(), 10),
			entry.line,
		}
	}

	push := lokiPush{
		Streams: []lokiStream{
			{
				Stream: h.labels,
				Values: values,
			},
		},
	}

	body, err := json.Marshal(push)
	if err != nil {
		return fmt.Errorf("failed to marshal loki push: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create loki request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs to loki: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki returned status %d", resp.StatusCode)
	}

	return nil
}

// Close flushes remaining logs and stops the handler.
func (h *LokiHandler) Close() error {
	if h.flushTimer != nil {
		h.flushTimer.Stop()
	}
	return h.Flush()
}
