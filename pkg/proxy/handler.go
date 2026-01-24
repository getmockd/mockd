// Package proxy provides HTTP/HTTPS request handling for the MITM proxy.
package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

const (
	// DefaultMaxBodySize is the default maximum body size to capture (10MB).
	DefaultMaxBodySize = 10 * 1024 * 1024
)

// ProxiedRequest represents a captured request with timing information.
type ProxiedRequest struct {
	Method    string
	URL       string
	Host      string
	Headers   http.Header
	Body      []byte
	StartTime time.Time
}

// ProxiedResponse represents a captured response with timing information.
type ProxiedResponse struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
	Duration   time.Duration
}

// handleHTTP handles regular HTTP proxy requests with optional recording.
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Read and buffer the request body
	var reqBody []byte
	if r.Body != nil {
		var err error
		reqBody, err = io.ReadAll(io.LimitReader(r.Body, DefaultMaxBodySize))
		if err != nil {
			p.log("Error reading request body: %v", err)
			http.Error(w, "Error reading request", http.StatusBadGateway)
			return
		}
		_ = r.Body.Close()
		// Replace body for forwarding
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	// Log the request
	p.log("[%s] %s %s", r.Method, r.Host, r.URL.Path)

	// Forward the request
	resp, err := p.forwardRequest(r)
	if err != nil {
		p.log("Error forwarding request: %v", err)
		http.Error(w, "Error forwarding request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read and buffer the response body
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, DefaultMaxBodySize))
	if err != nil {
		p.log("Error reading response body: %v", err)
		http.Error(w, "Error reading response", http.StatusBadGateway)
		return
	}

	duration := time.Since(startTime)

	// Check if we should record this request
	p.mu.RLock()
	mode := p.mode
	filter := p.filter
	p.mu.RUnlock()

	if mode == ModeRecord {
		// Check filter
		shouldRecord := true
		if filter != nil {
			shouldRecord = filter.ShouldRecord(r.Host, r.URL.Path)
		}

		if shouldRecord {
			// Create and store recording
			rec := recording.NewRecording("")
			rec.CaptureRequest(r, reqBody)
			rec.CaptureResponse(resp, respBody, duration)

			if err := p.store.AddRecording(rec); err != nil {
				p.log("Error storing recording: %v", err)
			} else {
				p.log("Recorded: %s %s (%d) [%v]", r.Method, r.URL.Path, resp.StatusCode, duration)
			}
		}
	}

	// Copy response to client
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	p.log("Response: %d %s [%v]", resp.StatusCode, resp.Status, duration)
}

// forwardRequest forwards an HTTP request to the target server and returns the response.
func (p *Proxy) forwardRequest(r *http.Request) (*http.Response, error) {
	// Construct the target URL
	targetURL := r.URL.String()
	if r.URL.Host == "" {
		targetURL = "http://" + r.Host + r.URL.RequestURI()
	}

	// Create outgoing request
	outReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	copyHeaders(outReq.Header, r.Header)

	// Remove hop-by-hop headers
	removeHopByHopHeaders(outReq.Header)

	// Set X-Forwarded headers
	outReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	outReq.Header.Set("X-Forwarded-Host", r.Host)

	return p.client.Do(outReq)
}

// copyHeaders copies headers from src to dst.
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// removeHopByHopHeaders removes headers that should not be forwarded.
func removeHopByHopHeaders(h http.Header) {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Proxy-Connection",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, header := range hopByHopHeaders {
		h.Del(header)
	}
}
