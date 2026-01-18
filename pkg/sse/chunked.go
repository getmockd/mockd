package sse

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// ChunkedHandler handles HTTP chunked transfer encoding responses.
type ChunkedHandler struct{}

// NewChunkedHandler creates a new chunked transfer handler.
func NewChunkedHandler() *ChunkedHandler {
	return &ChunkedHandler{}
}

// ServeHTTP handles a chunked transfer request.
func (h *ChunkedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, m *config.MockConfiguration) {
	if m.HTTP == nil || m.HTTP.Chunked == nil {
		http.Error(w, "Chunked configuration missing", http.StatusInternalServerError)
		return
	}

	// Check if Flusher is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	cfg := m.HTTP.Chunked

	// Set headers for chunked transfer
	h.setChunkedHeaders(w, cfg)

	// Get data to send
	data, err := h.getData(cfg)
	if err != nil {
		http.Error(w, "Failed to get data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle NDJSON format
	if cfg.Format == "ndjson" && len(cfg.NDJSONItems) > 0 {
		h.sendNDJSON(w, flusher, r.Context(), cfg)
		return
	}

	// Send data in chunks
	h.sendChunked(w, flusher, r.Context(), data, cfg)
}

// setChunkedHeaders sets HTTP headers for chunked transfer.
func (h *ChunkedHandler) setChunkedHeaders(w http.ResponseWriter, cfg *mock.ChunkedConfig) {
	// Transfer-Encoding is handled automatically by Go's HTTP server
	// when we don't set Content-Length

	switch cfg.Format {
	case "ndjson":
		w.Header().Set("Content-Type", ContentTypeNDJSON)
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
}

// getData retrieves the data to send.
func (h *ChunkedHandler) getData(cfg *mock.ChunkedConfig) ([]byte, error) {
	// Check for inline data
	if cfg.Data != "" {
		return []byte(cfg.Data), nil
	}

	// Check for file data
	if cfg.DataFile != "" {
		return os.ReadFile(cfg.DataFile)
	}

	return nil, nil
}

// sendChunked sends data in chunks.
func (h *ChunkedHandler) sendChunked(w http.ResponseWriter, flusher http.Flusher, ctx interface{ Done() <-chan struct{} }, data []byte, cfg *mock.ChunkedConfig) {
	if len(data) == 0 {
		return
	}

	chunkSize := cfg.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1024 // Default 1KB chunks
	}

	delay := time.Duration(cfg.ChunkDelay) * time.Millisecond

	offset := 0
	for offset < len(data) {
		// Check context
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Calculate chunk end
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}

		// Write chunk
		chunk := data[offset:end]
		_, err := w.Write(chunk)
		if err != nil {
			return
		}
		flusher.Flush()

		offset = end

		// Apply delay between chunks (not after last chunk)
		if offset < len(data) && delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

// sendNDJSON sends newline-delimited JSON items.
func (h *ChunkedHandler) sendNDJSON(w http.ResponseWriter, flusher http.Flusher, ctx interface{ Done() <-chan struct{} }, cfg *mock.ChunkedConfig) {
	delay := time.Duration(cfg.ChunkDelay) * time.Millisecond

	for i, item := range cfg.NDJSONItems {
		// Check context
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Marshal item to JSON
		jsonData, err := json.Marshal(item)
		if err != nil {
			continue
		}

		// Write with newline
		_, err = w.Write(jsonData)
		if err != nil {
			return
		}
		_, _ = w.Write([]byte{'\n'})
		flusher.Flush()

		// Apply delay between items (not after last item)
		if i < len(cfg.NDJSONItems)-1 && delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

// ChunkedResponse represents a chunked response builder.
type ChunkedResponse struct {
	chunks     [][]byte
	chunkSize  int
	chunkDelay int
}

// NewChunkedResponse creates a chunked response builder.
func NewChunkedResponse(chunkSize, chunkDelay int) *ChunkedResponse {
	if chunkSize <= 0 {
		chunkSize = 1024
	}
	return &ChunkedResponse{
		chunks:     make([][]byte, 0),
		chunkSize:  chunkSize,
		chunkDelay: chunkDelay,
	}
}

// AddChunk adds a chunk to the response.
func (r *ChunkedResponse) AddChunk(data []byte) {
	r.chunks = append(r.chunks, data)
}

// AddData splits data into chunks and adds them.
func (r *ChunkedResponse) AddData(data []byte) {
	for offset := 0; offset < len(data); offset += r.chunkSize {
		end := offset + r.chunkSize
		if end > len(data) {
			end = len(data)
		}
		r.AddChunk(data[offset:end])
	}
}

// Chunks returns all chunks.
func (r *ChunkedResponse) Chunks() [][]byte {
	return r.chunks
}

// TotalSize returns the total size of all chunks.
func (r *ChunkedResponse) TotalSize() int {
	total := 0
	for _, chunk := range r.chunks {
		total += len(chunk)
	}
	return total
}

// NDJSONBuilder builds NDJSON responses.
type NDJSONBuilder struct {
	items      []interface{}
	chunkDelay int
}

// NewNDJSONBuilder creates a new NDJSON builder.
func NewNDJSONBuilder(chunkDelay int) *NDJSONBuilder {
	return &NDJSONBuilder{
		items:      make([]interface{}, 0),
		chunkDelay: chunkDelay,
	}
}

// Add adds an item to the NDJSON response.
func (b *NDJSONBuilder) Add(item interface{}) {
	b.items = append(b.items, item)
}

// Items returns all items.
func (b *NDJSONBuilder) Items() []interface{} {
	return b.items
}

// Build returns the NDJSON as bytes.
func (b *NDJSONBuilder) Build() ([]byte, error) {
	var result []byte
	for _, item := range b.items {
		jsonData, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		result = append(result, jsonData...)
		result = append(result, '\n')
	}
	return result, nil
}
