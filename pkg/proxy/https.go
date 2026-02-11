// Package proxy provides HTTPS MITM interception using dynamic certificates.
package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// handleConnect handles HTTPS CONNECT requests for TLS interception.
func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Extract host from request
	host := r.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	// If no CA manager, we can't do MITM - just tunnel
	if p.ca == nil {
		p.log("No CA configured, tunneling HTTPS to %s", host)
		p.tunnelConnect(w, r, host)
		return
	}

	// Get host certificate
	hostOnly := strings.Split(host, ":")[0]
	certPair, err := p.ca.GenerateHostCert(hostOnly)
	if err != nil {
		p.log("Error generating host cert for %s: %v", hostOnly, err)
		http.Error(w, "Error generating certificate", http.StatusInternalServerError)
		return
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		p.log("HTTP server does not support hijacking")
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		p.log("Error hijacking connection: %v", err)
		http.Error(w, "Error hijacking connection", http.StatusInternalServerError)
		return
	}

	// Send 200 Connection Established
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		p.log("Error sending CONNECT response: %v", err)
		_ = clientConn.Close()
		return
	}

	// Create TLS config with the host certificate
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certPair.Cert.Raw},
		PrivateKey:  certPair.Key,
	}

	//nolint:gosec // G402: TLS MinVersion not set because proxy needs to support various client TLS versions
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	// Wrap client connection with TLS
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		p.log("TLS handshake with client failed: %v", err)
		_ = clientConn.Close()
		return
	}

	p.log("[CONNECT] %s (MITM)", host)

	// Handle requests on the TLS connection
	p.handleTLSConnection(tlsClientConn, hostOnly, host)
}

// handleTLSConnection handles HTTP requests over an established TLS connection.
func (p *Proxy) handleTLSConnection(clientConn *tls.Conn, hostOnly, fullHost string) {
	defer func() { _ = clientConn.Close() }()

	reader := bufio.NewReader(clientConn)

	for {
		// Read the HTTP request from the client
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				p.log("Error reading request from TLS connection: %v", err)
			}
			return
		}

		// Set the host for the request
		req.URL.Scheme = "https"
		req.URL.Host = hostOnly
		req.Host = hostOnly

		// Handle the request
		p.handleHTTPSRequest(clientConn, req, fullHost)
	}
}

// handleHTTPSRequest handles a single HTTPS request through the proxy.
func (p *Proxy) handleHTTPSRequest(clientConn net.Conn, r *http.Request, fullHost string) {
	startTime := time.Now()

	// Read and buffer the request body
	var reqBody []byte
	if r.Body != nil {
		var err error
		reqBody, err = io.ReadAll(io.LimitReader(r.Body, DefaultMaxBodySize))
		if err != nil {
			p.log("Error reading HTTPS request body: %v", err)
			writeHTTPError(clientConn, http.StatusBadGateway, "Error reading request")
			return
		}
		_ = r.Body.Close()
	}

	// Log the request
	p.log("[HTTPS] %s %s%s", r.Method, r.Host, r.URL.Path)

	// Connect to target server
	targetHost := fullHost
	if !strings.Contains(targetHost, ":") {
		targetHost += ":443"
	}

	//nolint:gosec // G402: proxy intentionally accepts any certificate
	targetConn, err := tls.Dial("tcp", targetHost, &tls.Config{
		InsecureSkipVerify: true, // We're a proxy, we accept any cert
	})
	if err != nil {
		p.log("Error connecting to target %s: %v", targetHost, err)
		writeHTTPError(clientConn, http.StatusBadGateway, "Error connecting to target")
		return
	}
	defer func() { _ = targetConn.Close() }()

	// Send the request to the target
	if err := r.Write(targetConn); err != nil {
		p.log("Error sending request to target: %v", err)
		writeHTTPError(clientConn, http.StatusBadGateway, "Error sending request")
		return
	}

	// Read the response from the target
	resp, err := http.ReadResponse(bufio.NewReader(targetConn), r)
	if err != nil {
		p.log("Error reading response from target: %v", err)
		writeHTTPError(clientConn, http.StatusBadGateway, "Error reading response")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read and buffer the response body
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, DefaultMaxBodySize))
	if err != nil {
		p.log("Error reading HTTPS response body: %v", err)
		writeHTTPError(clientConn, http.StatusBadGateway, "Error reading response")
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
				p.log("Error storing HTTPS recording: %v", err)
			} else {
				p.log("Recorded HTTPS: %s %s (%d) [%v]", r.Method, r.URL.Path, resp.StatusCode, duration)
			}
		}
	}

	// Write response back to client with body restored
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	resp.ContentLength = int64(len(respBody))
	if err := resp.Write(clientConn); err != nil {
		p.log("Error writing response: %v", err)
		return
	}

	p.log("HTTPS Response: %d %s [%v]", resp.StatusCode, resp.Status, duration)
}

// tunnelConnect creates a direct TCP tunnel for HTTPS without MITM.
func (p *Proxy) tunnelConnect(w http.ResponseWriter, _ *http.Request, host string) {
	// Connect to target
	targetConn, err := net.DialTimeout("tcp", host, 30*time.Second)
	if err != nil {
		p.log("Error connecting to target %s: %v", host, err)
		http.Error(w, "Error connecting to target", http.StatusBadGateway)
		return
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		p.log("HTTP server does not support hijacking")
		_ = targetConn.Close()
		http.Error(w, "HTTP server does not support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		p.log("Error hijacking connection: %v", err)
		_ = targetConn.Close()
		return
	}

	// Send 200 Connection Established
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		p.log("Error sending CONNECT response: %v", err)
		_ = clientConn.Close()
		_ = targetConn.Close()
		return
	}

	p.log("[CONNECT] %s (tunnel)", host)

	// Start bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(targetConn, clientConn)
		_ = targetConn.Close()
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, targetConn)
		_ = clientConn.Close()
	}()

	wg.Wait()
}

// writeHTTPError writes an HTTP error response to a raw connection.
//
//nolint:unparam // statusCode is always the same value but function is intentionally generic
func writeHTTPError(conn net.Conn, statusCode int, message string) {
	resp := &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "text/plain")
	_ = resp.Write(conn)
	_, _ = conn.Write([]byte(message))
}
