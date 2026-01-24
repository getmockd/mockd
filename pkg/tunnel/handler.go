package tunnel

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// EngineHandler adapts an http.Handler to the tunnel RequestHandler interface.
// This allows forwarding tunnel requests to the local mock engine.
type EngineHandler struct {
	handler http.Handler
	auth    *AuthConfig
}

// NewEngineHandler creates a new EngineHandler wrapping an http.Handler.
func NewEngineHandler(handler http.Handler, auth *AuthConfig) *EngineHandler {
	return &EngineHandler{handler: handler, auth: auth}
}

// checkAuth validates the request against the configured authentication.
func (h *EngineHandler) checkAuth(req *TunnelMessage) error {
	if h.auth == nil || h.auth.Type == "" || h.auth.Type == "none" {
		return nil
	}

	switch h.auth.Type {
	case "token":
		token := req.Headers["X-Auth-Token"]
		if token == "" {
			token = req.Headers["x-auth-token"]
		}
		if token != h.auth.Token {
			return errors.New("invalid or missing auth token")
		}
	case "basic":
		auth := req.Headers["Authorization"]
		if auth == "" {
			auth = req.Headers["authorization"]
		}
		if !strings.HasPrefix(auth, "Basic ") {
			return errors.New("missing basic auth header")
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err != nil {
			return errors.New("invalid basic auth encoding")
		}
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 || parts[0] != h.auth.Username || parts[1] != h.auth.Password {
			return errors.New("invalid credentials")
		}
	case "ip":
		clientIP := req.Headers["X-Forwarded-For"]
		if clientIP == "" {
			clientIP = req.Headers["X-Real-IP"]
		}
		if clientIP == "" {
			clientIP = req.Headers["x-forwarded-for"]
		}
		if clientIP == "" {
			clientIP = req.Headers["x-real-ip"]
		}
		// Take first IP if comma-separated
		if idx := strings.Index(clientIP, ","); idx > 0 {
			clientIP = strings.TrimSpace(clientIP[:idx])
		}
		allowed := false
		for _, ip := range h.auth.AllowedIPs {
			if clientIP == ip {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("IP not allowed")
		}
	default:
		return errors.New("unknown auth type")
	}
	return nil
}

// HandleRequest processes an incoming tunnel request by forwarding it to the engine.
func (h *EngineHandler) HandleRequest(ctx context.Context, req *TunnelMessage) *TunnelMessage {
	// Check authentication
	if err := h.checkAuth(req); err != nil {
		return NewErrorMessage(req.ID, "unauthorized", err.Error())
	}

	// Build the HTTP request
	httpReq, err := h.buildRequest(ctx, req)
	if err != nil {
		return NewErrorMessage(req.ID, "request_error", err.Error())
	}

	// Create response recorder
	rr := httptest.NewRecorder()

	// Forward to engine
	h.handler.ServeHTTP(rr, httpReq)

	// Build response message
	return h.buildResponse(req.ID, rr)
}

// buildRequest creates an http.Request from a TunnelMessage.
func (h *EngineHandler) buildRequest(ctx context.Context, msg *TunnelMessage) (*http.Request, error) {
	// Parse path and query
	reqURL, err := url.Parse(msg.Path)
	if err != nil {
		return nil, err
	}

	// Create body reader
	var body io.Reader
	if len(msg.Body) > 0 {
		body = bytes.NewReader(msg.Body)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, msg.Method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	for name, value := range msg.Headers {
		req.Header.Set(name, value)
	}

	return req, nil
}

// buildResponse creates a TunnelMessage response from an httptest.ResponseRecorder.
func (h *EngineHandler) buildResponse(requestID string, rr *httptest.ResponseRecorder) *TunnelMessage {
	// Convert headers
	headers := make(map[string]string)
	for name, values := range rr.Header() {
		if len(values) > 0 {
			headers[name] = strings.Join(values, ", ")
		}
	}

	return NewResponseMessage(
		requestID,
		rr.Code,
		headers,
		rr.Body.Bytes(),
	)
}

// FuncHandler adapts a function to the RequestHandler interface.
type FuncHandler func(ctx context.Context, req *TunnelMessage) *TunnelMessage

// HandleRequest implements RequestHandler.
func (f FuncHandler) HandleRequest(ctx context.Context, req *TunnelMessage) *TunnelMessage {
	return f(ctx, req)
}
