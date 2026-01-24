package audit

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Middleware wraps an http.Handler and logs requests/responses
type Middleware struct {
	handler  http.Handler
	logger   AuditLogger
	config   *AuditConfig
	redactor RedactorFunc
}

// NewMiddleware creates an audit logging middleware
func NewMiddleware(handler http.Handler, logger AuditLogger, config *AuditConfig) *Middleware {
	if config == nil {
		config = DefaultAuditConfig()
	}

	// Protect against nil logger
	if logger == nil {
		logger = &NoOpLogger{}
	}

	// Use registered redactor from enterprise extensions if available
	redactor := GetRegisteredRedactor()
	return &Middleware{
		handler:  handler,
		logger:   logger,
		config:   config,
		redactor: redactor,
	}
}

// ServeHTTP implements http.Handler and logs request/response information
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Generate a unique trace ID for this request
	traceID := uuid.New().String()

	// Capture request body without consuming it
	var requestBodyPreview string
	var requestBodySize int64
	if r.Body != nil && r.ContentLength != 0 {
		maxPreview := m.config.MaxBodyPreviewSize
		if maxPreview <= 0 {
			maxPreview = 1024
		}

		// Only read preview bytes, don't buffer entire body
		previewBytes := make([]byte, maxPreview)
		n, _ := io.ReadFull(r.Body, previewBytes)

		if n > 0 {
			requestBodyPreview = string(previewBytes[:n])
			// Recombine preview with remaining body
			r.Body = io.NopCloser(io.MultiReader(
				bytes.NewReader(previewBytes[:n]),
				r.Body,
			))
		}

		requestBodySize = r.ContentLength // Use header, not actual read
	}

	// Build request info
	requestInfo := &RequestInfo{
		Method:      r.Method,
		Path:        r.URL.Path,
		Query:       r.URL.RawQuery,
		BodySize:    requestBodySize,
		BodyPreview: requestBodyPreview,
		ContentType: r.Header.Get("Content-Type"),
	}

	// Include headers if configured
	if m.config.IncludeHeaders {
		requestInfo.Headers = r.Header.Clone()
	}

	// Build client info
	clientInfo := &ClientInfo{
		RemoteAddr: r.RemoteAddr,
		UserAgent:  r.Header.Get("User-Agent"),
	}
	if r.TLS != nil {
		clientInfo.TLS = true
		clientInfo.TLSVersion = tlsVersionString(r.TLS.Version)
		if len(r.TLS.PeerCertificates) > 0 {
			clientInfo.ClientCertCN = r.TLS.PeerCertificates[0].Subject.CommonName
		}
	}

	// Log request received
	requestEntry := NewAuditEntry(EventRequestReceived, traceID).
		WithRequest(requestInfo).
		WithClient(clientInfo)

	// Apply redaction before logging (if a redactor is registered)
	if m.redactor != nil {
		requestEntry = m.redactor(requestEntry)
	}
	if err := m.logger.Log(*requestEntry); err != nil {
		// Log error but don't fail the request
		_ = err
	}

	// Create response capture wrapper
	maxPreview := m.config.MaxBodyPreviewSize
	if maxPreview <= 0 {
		maxPreview = 1024
	}
	capture := &responseCapture{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default if WriteHeader not called
		body:           &bytes.Buffer{},
		maxCaptureSize: maxPreview,
	}

	// Call the wrapped handler
	m.handler.ServeHTTP(capture, r)

	// Calculate duration
	duration := time.Since(startTime)

	// Build response info
	responseInfo := &ResponseInfo{
		StatusCode:  capture.statusCode,
		BodySize:    int64(capture.size),
		ContentType: capture.Header().Get("Content-Type"),
		DurationMs:  duration.Milliseconds(),
	}

	// Capture body preview from response (already limited by maxCaptureSize)
	if capture.body.Len() > 0 {
		responseInfo.BodyPreview = capture.body.String()
	}

	// Include response headers if configured
	if m.config.IncludeHeaders {
		responseInfo.Headers = capture.Header().Clone()
	}

	// Log response sent
	responseEntry := NewAuditEntry(EventResponseSent, traceID).
		WithRequest(requestInfo).
		WithResponse(responseInfo).
		WithClient(clientInfo).
		WithMetadata(&EntryMetadata{
			Duration: duration.Nanoseconds(),
		})

	// Apply redaction before logging (if a redactor is registered)
	if m.redactor != nil {
		responseEntry = m.redactor(responseEntry)
	}
	if err := m.logger.Log(*responseEntry); err != nil {
		// Log error but don't fail the request
		_ = err
	}
}

// responseCapture captures response data for logging
type responseCapture struct {
	http.ResponseWriter
	statusCode     int
	body           *bytes.Buffer
	size           int
	maxCaptureSize int // limit for body capture
}

// WriteHeader captures the status code and delegates to the underlying ResponseWriter
func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

// Write captures the response body and delegates to the underlying ResponseWriter
func (rc *responseCapture) Write(b []byte) (int, error) {
	// Only capture up to maxCaptureSize
	if rc.body.Len() < rc.maxCaptureSize {
		remaining := rc.maxCaptureSize - rc.body.Len()
		if len(b) <= remaining {
			rc.body.Write(b)
		} else {
			rc.body.Write(b[:remaining])
		}
	}
	rc.size += len(b)
	return rc.ResponseWriter.Write(b)
}

// tlsVersionString converts a TLS version constant to a human-readable string
func tlsVersionString(version uint16) string {
	switch version {
	case 0x0300:
		return "SSL 3.0"
	case 0x0301:
		return "TLS 1.0"
	case 0x0302:
		return "TLS 1.1"
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}
