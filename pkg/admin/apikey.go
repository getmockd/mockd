// API key authentication for the Admin API.

package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// APIKeyLength is the length of generated API keys in bytes (32 bytes = 64 hex chars).
	APIKeyLength = 32

	// APIKeyPrefix is the prefix for API keys to make them identifiable.
	APIKeyPrefix = "mk_"

	// APIKeyHeader is the HTTP header for API key authentication.
	APIKeyHeader = "X-API-Key"

	// DefaultKeyFileName is the default file name for storing the API key.
	DefaultKeyFileName = "admin-api-key"
)

// APIKeyConfig holds configuration for API key authentication.
type APIKeyConfig struct {
	// Enabled controls whether API key authentication is required.
	// If false, all requests are allowed without authentication.
	Enabled bool

	// Key is the API key. If empty and Enabled is true, one will be generated.
	Key string

	// KeyFilePath is the path to store/load the API key.
	// If empty, uses default XDG data directory.
	KeyFilePath string

	// AllowLocalhost allows requests from localhost without API key.
	// Useful for development but should be disabled in production.
	AllowLocalhost bool

	// ExemptPaths are URL paths that don't require authentication.
	// Health check is always exempt.
	ExemptPaths []string
}

// DefaultAPIKeyConfig returns the default API key configuration.
func DefaultAPIKeyConfig() APIKeyConfig {
	return APIKeyConfig{
		Enabled:        true,
		AllowLocalhost: false, // Secure by default
		ExemptPaths:    []string{"/health"},
	}
}

// apiKeyAuth handles API key authentication state and validation.
type apiKeyAuth struct {
	config  APIKeyConfig
	key     string
	keyHash []byte // For constant-time comparison
	mu      sync.RWMutex
	log     func(msg string, args ...any)
}

// newAPIKeyAuth creates a new API key authenticator.
func newAPIKeyAuth(config APIKeyConfig, logFn func(msg string, args ...any)) (*apiKeyAuth, error) {
	auth := &apiKeyAuth{
		config: config,
		log:    logFn,
	}

	if !config.Enabled {
		return auth, nil
	}

	// If key is provided via config, use it
	if config.Key != "" {
		auth.setKey(config.Key)
		return auth, nil
	}

	// Check MOCKD_API_KEY environment variable (idiomatic for Docker / CI)
	if envKey := os.Getenv("MOCKD_API_KEY"); envKey != "" {
		auth.setKey(envKey)
		auth.log("Using API key from MOCKD_API_KEY environment variable")
		return auth, nil
	}

	// Try to load from file
	keyPath := auth.getKeyFilePath()
	key, err := auth.loadKeyFromFile(keyPath)
	if err == nil && key != "" {
		auth.setKey(key)
		auth.log("Loaded API key from file", "path", keyPath)
		return auth, nil
	}

	// Generate new key
	key, err = generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}
	auth.setKey(key)

	// Save to file
	if err := auth.saveKeyToFile(keyPath, key); err != nil {
		auth.log("Warning: failed to save API key to file", "path", keyPath, "error", err)
		// Don't fail - key is still usable in memory
	} else {
		auth.log("Generated and saved new API key", "path", keyPath)
	}

	// Print the key to stdout so users can discover it (especially in Docker)
	fmt.Fprintf(os.Stderr, "Admin API key: %s\n", key)
	fmt.Fprintf(os.Stderr, "  Set MOCKD_API_KEY env var or use --no-auth to skip authentication.\n")
	fmt.Fprintf(os.Stderr, "  Key saved to: %s\n", keyPath)

	return auth, nil
}

// setKey sets the API key and precomputes its hash for constant-time comparison.
func (a *apiKeyAuth) setKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.key = key
	a.keyHash = []byte(key)
}

// getKey returns the current API key.
func (a *apiKeyAuth) getKey() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.key
}

// getKeyFilePath returns the path for the API key file.
func (a *apiKeyAuth) getKeyFilePath() string {
	if a.config.KeyFilePath != "" {
		return a.config.KeyFilePath
	}

	// Use XDG data directory
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "mockd", DefaultKeyFileName)
}

// loadKeyFromFile loads the API key from a file.
func (a *apiKeyAuth) loadKeyFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("empty key file")
	}
	return key, nil
}

// saveKeyToFile saves the API key to a file with secure permissions.
func (a *apiKeyAuth) saveKeyToFile(path string, key string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(path, []byte(key+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// validate checks if the provided key is valid.
func (a *apiKeyAuth) validate(providedKey string) bool {
	a.mu.RLock()
	keyHash := a.keyHash
	a.mu.RUnlock()

	if len(keyHash) == 0 {
		return false
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(providedKey), keyHash) == 1
}

// isExempt checks if a path is exempt from authentication.
func (a *apiKeyAuth) isExempt(path string) bool {
	// Health check is always exempt
	if path == "/health" {
		return true
	}

	for _, exempt := range a.config.ExemptPaths {
		if path == exempt || strings.HasPrefix(path, exempt+"/") {
			return true
		}
	}
	return false
}

// middleware returns an HTTP middleware that enforces API key authentication.
func (a *apiKeyAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if auth is disabled
		if !a.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check if path is exempt
		if a.isExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check localhost exemption
		if a.config.AllowLocalhost && isLocalhost(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Get API key from header or query param
		apiKey := r.Header.Get(APIKeyHeader)
		if apiKey == "" {
			// Also check Authorization header (Bearer token format)
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if apiKey == "" {
			// Check query parameter as last resort
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, "missing_api_key",
				"API key required. Provide via X-API-Key header, Authorization: Bearer <key>, or api_key query parameter.")
			return
		}

		if !a.validate(apiKey) {
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "Invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rotateKey generates a new API key, saves it, and returns the new key.
func (a *apiKeyAuth) rotateKey() (string, error) {
	newKey, err := generateAPIKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate new API key: %w", err)
	}

	a.setKey(newKey)

	// Save to file
	keyPath := a.getKeyFilePath()
	if err := a.saveKeyToFile(keyPath, newKey); err != nil {
		a.log("Warning: failed to save rotated API key to file", "path", keyPath, "error", err)
	}

	return newKey, nil
}

// generateAPIKey generates a new random API key.
func generateAPIKey() (string, error) {
	bytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return APIKeyPrefix + hex.EncodeToString(bytes), nil
}

// APIKeyInfo contains information about the current API key.
type APIKeyInfo struct {
	Key         string    `json:"key,omitempty"` // Only included when explicitly requested
	KeyPrefix   string    `json:"keyPrefix"`     // First 8 chars for identification
	Enabled     bool      `json:"enabled"`
	KeyFilePath string    `json:"keyFilePath"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

// getInfo returns information about the current API key.
func (a *apiKeyAuth) getInfo(includeFullKey bool) APIKeyInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	info := APIKeyInfo{
		Enabled:     a.config.Enabled,
		KeyFilePath: a.getKeyFilePath(),
	}

	if a.key != "" {
		if includeFullKey {
			info.Key = a.key
		}
		// Show prefix for identification (mk_ + first 8 hex chars)
		if len(a.key) > 11 {
			info.KeyPrefix = a.key[:11] + "..."
		} else {
			info.KeyPrefix = a.key
		}
	}

	return info
}

// ============================================
// HTTP Handlers for API Key Management
// ============================================

// handleGetAPIKey handles GET /admin/api-key.
// Returns information about the API key (without the full key by default).
func (a *API) handleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	if a.apiKeyAuth == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "API key authentication is not configured")
		return
	}

	// Check if full key should be included (requires explicit query param)
	includeKey := r.URL.Query().Get("show_key") == "true"

	info := a.apiKeyAuth.getInfo(includeKey)
	writeJSON(w, http.StatusOK, info)
}

// handleRotateAPIKey handles POST /admin/api-key/rotate.
// Generates a new API key and returns it.
func (a *API) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	if a.apiKeyAuth == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "API key authentication is not configured")
		return
	}

	if !a.apiKeyAuth.config.Enabled {
		writeError(w, http.StatusBadRequest, "auth_disabled", "API key authentication is disabled")
		return
	}

	newKey, err := a.apiKeyAuth.rotateKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rotation_failed", "Failed to rotate API key: "+err.Error())
		return
	}

	a.log.Info("API key rotated")

	writeJSON(w, http.StatusOK, map[string]string{
		"key":     newKey,
		"message": "API key rotated successfully. All existing sessions using the old key will be invalidated.",
	})
}

// APIKey returns the current API key.
// This is used by the desktop app to get the key for local CLI operations.
func (a *API) APIKey() string {
	if a.apiKeyAuth == nil {
		return ""
	}
	return a.apiKeyAuth.getKey()
}
