// Token management for engine authentication.

package admin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Token expiration defaults
const (
	// RegistrationTokenExpiration is the default expiration time for registration tokens.
	RegistrationTokenExpiration = 1 * time.Hour

	// EngineTokenExpiration is the default expiration time for engine tokens.
	EngineTokenExpiration = 24 * time.Hour

	// RefreshTokenExpiration is the default expiration time for refresh tokens.
	RefreshTokenExpiration = 7 * 24 * time.Hour // 7 days

	// TokenCleanupInterval is how often the cleanup goroutine runs.
	TokenCleanupInterval = 5 * time.Minute

	// MaxRegistrationTokens is the maximum number of active registration tokens.
	// Prevents unbounded map growth from repeated token generation.
	MaxRegistrationTokens = 100

	// MaxEngineTokens is the maximum number of active engine tokens.
	// One per registered engine; cap prevents runaway registration.
	MaxEngineTokens = 1000
)

// storedToken represents a token with expiration metadata.
type storedToken struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// isExpired returns true if the token has expired.
func (t storedToken) isExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// GenerateRegistrationToken creates a new registration token.
// Returns an error if the maximum number of registration tokens has been reached.
func (a *API) GenerateRegistrationToken() (string, error) {
	token, err := generateRandomHex(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate registration token: %w", err)
	}
	now := time.Now()
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// Enforce cap to prevent unbounded map growth.
	if len(a.registrationTokens) >= MaxRegistrationTokens {
		return "", fmt.Errorf("maximum number of registration tokens (%d) reached; wait for expiry or cleanup", MaxRegistrationTokens)
	}

	a.registrationTokens[token] = storedToken{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(a.registrationTokenExpiration),
	}
	return token, nil
}

// ValidateRegistrationToken checks if a registration token is valid using
// constant-time comparison to prevent timing side-channel attacks.
// If valid, it consumes the token (one-time use).
// Returns false if the token doesn't match any stored token or has expired.
func (a *API) ValidateRegistrationToken(token string) bool {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	tokenBytes := []byte(token)
	var matchedKey string
	var matchedToken *storedToken

	// Iterate all tokens with constant-time comparison so that timing
	// does not reveal whether a token exists. The number of registration
	// tokens is always very small (single digits).
	for key, stored := range a.registrationTokens {
		if subtle.ConstantTimeCompare([]byte(stored.Token), tokenBytes) == 1 {
			matchedKey = key
			s := stored // copy
			matchedToken = &s
		}
	}

	if matchedToken == nil {
		return false
	}

	// Check expiration
	if matchedToken.isExpired() {
		delete(a.registrationTokens, matchedKey)
		if len(token) >= 8 {
			a.logger().Debug("registration token expired and removed during validation", "token_prefix", token[:8])
		}
		return false
	}
	delete(a.registrationTokens, matchedKey) // One-time use
	return true
}

// ValidateEngineToken checks if an engine token is valid for the given engine ID.
// Returns false if the token doesn't exist, doesn't match, or has expired.
func (a *API) ValidateEngineToken(engineID, token string) bool {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	stored, exists := a.engineTokens[engineID]
	if !exists || subtle.ConstantTimeCompare([]byte(stored.Token), []byte(token)) != 1 {
		return false
	}
	// Check expiration
	if stored.isExpired() {
		// Note: we don't delete here since we only have RLock
		// The cleanup goroutine will handle removal
		a.logger().Debug("engine token expired during validation", "engine_id", engineID)
		return false
	}
	return true
}

// generateEngineToken creates and stores a new token for an engine.
// Returns an error if the maximum number of engine tokens has been reached.
func (a *API) generateEngineToken(engineID string) (string, error) {
	token, err := generateRandomHex(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate engine token: %w", err)
	}
	now := time.Now()
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// Allow overwriting an existing token for this engine (re-registration).
	// Only enforce cap for genuinely new entries.
	if _, exists := a.engineTokens[engineID]; !exists && len(a.engineTokens) >= MaxEngineTokens {
		return "", fmt.Errorf("maximum number of engine tokens (%d) reached; wait for expiry or cleanup", MaxEngineTokens)
	}

	a.engineTokens[engineID] = storedToken{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(a.engineTokenExpiration),
	}
	return token, nil
}

// removeEngineToken removes the token for an engine.
func (a *API) removeEngineToken(engineID string) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	delete(a.engineTokens, engineID)
}

// ListRegistrationTokens returns all active (non-expired) registration tokens.
func (a *API) ListRegistrationTokens() []string {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	tokens := make([]string, 0, len(a.registrationTokens))
	now := time.Now()
	for token, stored := range a.registrationTokens {
		if now.Before(stored.ExpiresAt) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// startTokenCleanup runs a background goroutine that periodically removes expired tokens.
func (a *API) startTokenCleanup(ctx context.Context) {
	ticker := time.NewTicker(TokenCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.logger().Debug("token cleanup goroutine stopped")
			return
		case <-ticker.C:
			a.cleanupExpiredTokens()
		}
	}
}

// cleanupExpiredTokens removes all expired tokens from both registration and engine token maps.
func (a *API) cleanupExpiredTokens() {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	now := time.Now()
	registrationCleaned := 0
	engineCleaned := 0

	// Clean up expired registration tokens
	for token, stored := range a.registrationTokens {
		if now.After(stored.ExpiresAt) {
			delete(a.registrationTokens, token)
			registrationCleaned++
		}
	}

	// Clean up expired engine tokens
	for engineID, stored := range a.engineTokens {
		if now.After(stored.ExpiresAt) {
			delete(a.engineTokens, engineID)
			engineCleaned++
		}
	}

	if registrationCleaned > 0 || engineCleaned > 0 {
		a.logger().Debug("cleaned up expired tokens",
			"registration_tokens_removed", registrationCleaned,
			"engine_tokens_removed", engineCleaned,
		)
	}
}

// TokenStats returns statistics about current tokens.
type TokenStats struct {
	ActiveRegistrationTokens  int `json:"active_registration_tokens"`
	ExpiredRegistrationTokens int `json:"expired_registration_tokens"`
	ActiveEngineTokens        int `json:"active_engine_tokens"`
	ExpiredEngineTokens       int `json:"expired_engine_tokens"`
}

// GetTokenStats returns statistics about token storage.
func (a *API) GetTokenStats() TokenStats {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()

	now := time.Now()
	stats := TokenStats{}

	for _, stored := range a.registrationTokens {
		if now.Before(stored.ExpiresAt) {
			stats.ActiveRegistrationTokens++
		} else {
			stats.ExpiredRegistrationTokens++
		}
	}

	for _, stored := range a.engineTokens {
		if now.Before(stored.ExpiresAt) {
			stats.ActiveEngineTokens++
		} else {
			stats.ExpiredEngineTokens++
		}
	}

	return stats
}

// isLocalhost checks if the request originates from localhost by examining RemoteAddr.
// This is more secure than checking r.Host which is client-controlled and can be spoofed.
func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false
	}

	return parsedIP.IsLoopback()
}

// isBrowserRequest reports whether the request looks like it was issued by a
// web page, as opposed to a non-browser client (curl, the mockd CLI, or an MCP
// client) which send none of these headers. It is used to decide whether the
// keyless localhost bypass may be honored: a fetch() from any website the user
// is browsing also arrives over the loopback socket, so a loopback RemoteAddr
// alone is NOT proof of trust.
//
// A request is treated as a browser request when it carries EITHER any
// Sec-Fetch-* header (Chrome 76+, Firefox 90+, Safari 16.4+ send these on every
// request and page JS cannot forge them — they are forbidden Sec- headers) OR a
// browser Origin header (the fallback that covers older Safari/WebViews which
// omit Sec-Fetch-*). Requests with no browser fingerprint at all return false
// so CLI/curl/MCP keep their keyless localhost convenience unchanged.
func isBrowserRequest(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Site") != "" ||
		r.Header.Get("Sec-Fetch-Mode") != "" ||
		r.Header.Get("Sec-Fetch-Dest") != "" ||
		r.Header.Get("Origin") != ""
}

// isUntrustedBrowserRequest reports whether the request is a browser request
// that must NOT be allowed to ride the keyless localhost bypass. It returns
// true for a cross-site fetch() from an unrelated website (the exact threat the
// API key exists to stop) and false for trusted browser contexts: same-origin,
// same-site, or user-initiated (Sec-Fetch-Site none), and for cross-site
// requests whose Origin is itself a loopback origin (so a cross-port localhost
// dashboard — e.g. Vite :5173 calling admin :4290, which browsers label
// cross-site because localhost is not on the Public Suffix List — keeps
// working). Non-browser clients (no fingerprint) are never untrusted here.
func isUntrustedBrowserRequest(r *http.Request) bool {
	if !isBrowserRequest(r) {
		return false
	}
	origin := r.Header.Get("Origin")
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "same-site":
		// A genuine browser only labels a request same-origin/same-site when it
		// truly targets the loopback dashboard, in which case any Origin it sends
		// is itself a loopback origin. A foreign Origin paired with same-origin is
		// a forged/contradictory signal, so trust this branch only when the Origin
		// is absent or loopback.
		return origin != "" && !isLoopbackOrigin(origin)
	case "none":
		// User-initiated (address bar, bookmark, redirect) — no Origin to check.
		return false
	}
	// Sec-Fetch-Site is "cross-site"/"cross-origin", or it is absent on a legacy
	// browser that still sent an Origin. Trust it only when the Origin is a
	// loopback origin (e.g. a cross-port localhost dashboard); otherwise it is
	// untrusted web content.
	return !isLoopbackOrigin(origin)
}

// isLoopbackOrigin reports whether an Origin header value points at a loopback
// host (http(s)://localhost[:port], //127.0.0.1[:port], //[::1][:port]). It is
// port-insensitive so the dashboard works on any local port. "null"
// (sandboxed iframe / data: / file:) and any non-loopback host return false and
// are never treated as trusted.
func isLoopbackOrigin(origin string) bool {
	if origin == "" || origin == "null" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return hostnameIsLoopback(u.Hostname())
}

// hostnameIsLoopback reports whether a bare hostname (no port, no brackets) is
// loopback: the literal "localhost", or an IP that parses and IsLoopback()
// (covers all of 127.0.0.0/8, ::1, and ::ffff:127.0.0.1). A DNS-rebinding
// hostname like "127.0.0.1.attacker.com" is NOT an IP, fails net.ParseIP, and is
// rejected.
//
// "*.localhost" is deliberately NOT accepted: although RFC 6761 says it SHOULD
// resolve to loopback, that is not universally enforced, so an attacker who can
// make "evil.localhost" resolve to a host they control would otherwise obtain a
// loopback-classified Origin/Host. Only the literal "localhost" — which no
// attacker can serve content from on the victim's machine — is trusted.
func hostnameIsLoopback(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// hostIsLoopback validates the Host header against the loopback allowlist as an
// anti-DNS-rebinding control. A rebinding attack keeps the attacker's hostname
// in Host (e.g. "evil.com" or "127.0.0.1.attacker.com"), which fails
// net.ParseIP and is rejected. It is applied ONLY to browser requests (see the
// caller in apikey.go) so remote/Docker admin deployments reached via a real
// hostname, and CLI/curl clients that send any Host over a loopback socket, are
// never affected. An empty Host is allowed (it cannot carry a rebinding
// hostname).
func hostIsLoopback(r *http.Request) bool {
	host := r.Host
	if host == "" {
		return true
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return hostnameIsLoopback(host)
}

// isSafeMethod reports whether the HTTP method is read-only (RFC 7231 safe), so
// it cannot mutate server state. Used to require a stronger loopback signal for
// state-changing requests.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// keylessLocalhostTrusted reports whether a request arriving over the loopback
// socket may skip authentication. It is the single source of truth shared by the
// API-key middleware and the engine-registration handlers so the same trust
// decision is applied uniformly.
//
// A loopback socket is NOT sufficient on its own — a browser fetch() from any
// website the user is visiting also arrives over loopback. So beyond requiring a
// loopback RemoteAddr we additionally require:
//   - the request is not an untrusted (cross-site / foreign-origin) browser
//     request; and
//   - for browser requests, OR for any state-changing (unsafe) method, the Host
//     header is a loopback host. This is the anti-DNS-rebinding control and it
//     also means a header-less write cannot mutate state without either a
//     loopback Host or the API key.
//
// Non-browser clients performing safe (read-only) requests — the mockd CLI and
// MCP client — keep keyless localhost access regardless of their Host header.
func keylessLocalhostTrusted(r *http.Request) bool {
	if !isLocalhost(r) {
		return false
	}
	if isUntrustedBrowserRequest(r) {
		return false
	}
	if isBrowserRequest(r) || !isSafeMethod(r.Method) {
		return hostIsLoopback(r)
	}
	return true
}

// getBearerToken extracts the bearer token from Authorization header.
func getBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return auth[len(prefix):]
}

// generateRandomHex generates a random hex string of the given length.
func generateRandomHex(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random hex: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
