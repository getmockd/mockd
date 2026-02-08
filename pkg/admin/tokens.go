// Token management for engine authentication.

package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
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
func (a *API) GenerateRegistrationToken() (string, error) {
	token, err := generateRandomHex(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate registration token: %w", err)
	}
	now := time.Now()
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	a.registrationTokens[token] = storedToken{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(a.registrationTokenExpiration),
	}
	return token, nil
}

// ValidateRegistrationToken checks if a registration token is valid.
// If valid, it consumes the token (one-time use).
// Returns false if the token doesn't exist or has expired.
func (a *API) ValidateRegistrationToken(token string) bool {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	stored, exists := a.registrationTokens[token]
	if !exists {
		return false
	}
	// Check expiration
	if stored.isExpired() {
		delete(a.registrationTokens, token)
		a.log.Debug("registration token expired and removed during validation", "token_prefix", token[:8])
		return false
	}
	delete(a.registrationTokens, token) // One-time use
	return true
}

// ValidateEngineToken checks if an engine token is valid for the given engine ID.
// Returns false if the token doesn't exist, doesn't match, or has expired.
func (a *API) ValidateEngineToken(engineID, token string) bool {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	stored, exists := a.engineTokens[engineID]
	if !exists || stored.Token != token {
		return false
	}
	// Check expiration
	if stored.isExpired() {
		// Note: we don't delete here since we only have RLock
		// The cleanup goroutine will handle removal
		a.log.Debug("engine token expired during validation", "engine_id", engineID)
		return false
	}
	return true
}

// generateEngineToken creates and stores a new token for an engine.
func (a *API) generateEngineToken(engineID string) (string, error) {
	token, err := generateRandomHex(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate engine token: %w", err)
	}
	now := time.Now()
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
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
			a.log.Debug("token cleanup goroutine stopped")
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
		a.log.Debug("cleaned up expired tokens",
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
