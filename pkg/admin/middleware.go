package admin

import (
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware logs HTTP requests.
type LoggingMiddleware struct {
	handler http.Handler
}

// NewLoggingMiddleware creates a new logging middleware.
func NewLoggingMiddleware(handler http.Handler) *LoggingMiddleware {
	return &LoggingMiddleware{handler: handler}
}

// ServeHTTP implements the http.Handler interface.
func (m *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Wrap response writer to capture status code
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	m.handler.ServeHTTP(lrw, r)

	log.Printf("[ADMIN] %s %s %d %v",
		r.Method,
		r.URL.Path,
		lrw.statusCode,
		time.Since(start),
	)
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware adds CORS headers to responses.
type CORSMiddleware struct {
	handler http.Handler
}

// NewCORSMiddleware creates a new CORS middleware.
func NewCORSMiddleware(handler http.Handler) *CORSMiddleware {
	return &CORSMiddleware{handler: handler}
}

// ServeHTTP implements the http.Handler interface.
func (m *CORSMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	m.handler.ServeHTTP(w, r)
}
