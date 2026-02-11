package oauth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// maxAuthCodes is the maximum number of authorization codes stored before
// oldest entries are evicted. Prevents unbounded memory growth under burst load.
const maxAuthCodes = 10000

// maxRefreshTokens is the maximum number of refresh tokens stored.
const maxRefreshTokens = 10000

// maxRevokedTokens is the maximum number of revoked token entries tracked.
const maxRevokedTokens = 50000

// Provider represents a mock OAuth/OIDC provider
type Provider struct {
	config     *OAuthConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string

	// Storage for authorization codes and refresh tokens
	authCodesMu     sync.RWMutex
	authCodes       map[string]*AuthorizationCode
	refreshTokensMu sync.RWMutex
	refreshTokens   map[string]*RefreshTokenData
	revokedTokensMu sync.RWMutex
	revokedTokens   map[string]time.Time // token -> revocation time (for cleanup)

	tokenExpiry   time.Duration
	refreshExpiry time.Duration

	// Cleanup goroutine management
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// NewProvider creates a new OAuth provider
func NewProvider(config *OAuthConfig) (*Provider, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Generate RSA key pair for JWT signing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Generate a unique key ID
	keyID, err := generateRandomString(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key ID: %w", err)
	}

	// Parse token expiry durations
	tokenExpiry := time.Hour // default
	if config.TokenExpiry != "" {
		tokenExpiry, err = parseDuration(config.TokenExpiry)
		if err != nil {
			return nil, fmt.Errorf("invalid tokenExpiry: %w", err)
		}
	}

	refreshExpiry := 7 * 24 * time.Hour // default 7 days
	if config.RefreshExpiry != "" {
		refreshExpiry, err = parseDuration(config.RefreshExpiry)
		if err != nil {
			return nil, fmt.Errorf("invalid refreshExpiry: %w", err)
		}
	}

	// Set defaults
	if config.Issuer == "" {
		config.Issuer = "https://mock-oauth.local"
	}
	if config.DefaultScopes == nil {
		config.DefaultScopes = []string{"openid", "profile", "email"}
	}

	p := &Provider{
		config:        config,
		privateKey:    privateKey,
		publicKey:     &privateKey.PublicKey,
		keyID:         keyID,
		authCodes:     make(map[string]*AuthorizationCode),
		refreshTokens: make(map[string]*RefreshTokenData),
		revokedTokens: make(map[string]time.Time),
		tokenExpiry:   tokenExpiry,
		refreshExpiry: refreshExpiry,
		stopCleanup:   make(chan struct{}),
		cleanupDone:   make(chan struct{}),
	}

	// Start background cleanup goroutine
	go p.cleanupLoop()

	return p, nil
}

// GenerateToken creates a new access token
func (p *Provider) GenerateToken(claims map[string]interface{}) (string, error) {
	now := time.Now()

	jti, err := generateRandomString(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate jti: %w", err)
	}

	jwtClaims := jwt.MapClaims{
		"iss": p.config.Issuer,
		"iat": now.Unix(),
		"exp": now.Add(p.tokenExpiry).Unix(),
		"jti": jti,
	}

	// Add default claims
	for k, v := range p.config.DefaultClaims {
		jwtClaims[k] = v
	}

	// Add provided claims (override defaults)
	for k, v := range claims {
		jwtClaims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	token.Header["kid"] = p.keyID

	return token.SignedString(p.privateKey)
}

// GenerateIDToken creates a new OIDC ID token
func (p *Provider) GenerateIDToken(claims map[string]interface{}) (string, error) {
	now := time.Now()

	jwtClaims := jwt.MapClaims{
		"iss":       p.config.Issuer,
		"iat":       now.Unix(),
		"exp":       now.Add(p.tokenExpiry).Unix(),
		"auth_time": now.Unix(),
	}

	// Add provided claims
	for k, v := range claims {
		jwtClaims[k] = v
	}

	// Ensure required OIDC claims are present
	if _, ok := jwtClaims["sub"]; !ok {
		sub, err := generateRandomString(16)
		if err != nil {
			return "", fmt.Errorf("failed to generate sub claim: %w", err)
		}
		jwtClaims["sub"] = sub
	}
	if _, ok := jwtClaims["aud"]; !ok {
		jwtClaims["aud"] = "default-client"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	token.Header["kid"] = p.keyID

	return token.SignedString(p.privateKey)
}

// GenerateRefreshToken creates a refresh token
func (p *Provider) GenerateRefreshToken() (string, error) {
	return generateRandomString(64)
}

// GetJWKS returns the JSON Web Key Set
func (p *Provider) GetJWKS() map[string]interface{} {
	// Encode the public key components
	n := base64.RawURLEncoding.EncodeToString(p.publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.publicKey.E)).Bytes())

	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": p.keyID,
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}
}

// ValidateToken validates an access token and returns its claims
func (p *Provider) ValidateToken(tokenString string) (map[string]interface{}, error) {
	// Check if token is revoked
	p.revokedTokensMu.RLock()
	_, isRevoked := p.revokedTokens[tokenString]
	p.revokedTokensMu.RUnlock()
	if isRevoked {
		return nil, errors.New("token has been revoked")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return p.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims format")
	}

	return claims, nil
}

// StoreAuthorizationCode stores an authorization code for later exchange.
// If the map exceeds maxAuthCodes, expired entries are evicted first;
// if still over capacity, the oldest entry by expiry is removed.
func (p *Provider) StoreAuthorizationCode(code *AuthorizationCode) {
	p.authCodesMu.Lock()
	defer p.authCodesMu.Unlock()

	p.authCodes[code.Code] = code

	if len(p.authCodes) > maxAuthCodes {
		now := time.Now()
		// First pass: remove expired entries
		for k, v := range p.authCodes {
			if now.After(v.ExpiresAt) {
				delete(p.authCodes, k)
			}
		}
		// If still over cap, remove oldest by expiry
		for len(p.authCodes) > maxAuthCodes {
			var oldestKey string
			var oldestTime time.Time
			for k, v := range p.authCodes {
				if oldestKey == "" || v.ExpiresAt.Before(oldestTime) {
					oldestKey = k
					oldestTime = v.ExpiresAt
				}
			}
			if oldestKey != "" {
				delete(p.authCodes, oldestKey)
			}
		}
	}
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens
func (p *Provider) ExchangeAuthorizationCode(code, clientID, redirectURI string) (*AuthorizationCode, error) {
	p.authCodesMu.Lock()
	defer p.authCodesMu.Unlock()

	authCode, ok := p.authCodes[code]
	if !ok {
		return nil, errors.New("invalid authorization code")
	}

	// Delete the code (one-time use)
	delete(p.authCodes, code)

	// Validate the code
	if time.Now().After(authCode.ExpiresAt) {
		return nil, errors.New("authorization code has expired")
	}

	if authCode.ClientID != clientID {
		return nil, errors.New("client_id mismatch")
	}

	if authCode.RedirectURI != redirectURI {
		return nil, errors.New("redirect_uri mismatch")
	}

	return authCode, nil
}

// StoreRefreshToken stores a refresh token.
// If the map exceeds maxRefreshTokens, expired entries are evicted first;
// if still over capacity, the oldest entry by expiry is removed.
func (p *Provider) StoreRefreshToken(data *RefreshTokenData) {
	p.refreshTokensMu.Lock()
	defer p.refreshTokensMu.Unlock()

	p.refreshTokens[data.Token] = data

	if len(p.refreshTokens) > maxRefreshTokens {
		now := time.Now()
		// First pass: remove expired entries
		for k, v := range p.refreshTokens {
			if now.After(v.ExpiresAt) {
				delete(p.refreshTokens, k)
			}
		}
		// If still over cap, remove oldest by expiry
		for len(p.refreshTokens) > maxRefreshTokens {
			var oldestKey string
			var oldestTime time.Time
			for k, v := range p.refreshTokens {
				if oldestKey == "" || v.ExpiresAt.Before(oldestTime) {
					oldestKey = k
					oldestTime = v.ExpiresAt
				}
			}
			if oldestKey != "" {
				delete(p.refreshTokens, oldestKey)
			}
		}
	}
}

// ValidateRefreshToken validates and returns refresh token data
func (p *Provider) ValidateRefreshToken(token, clientID string) (*RefreshTokenData, error) {
	p.refreshTokensMu.Lock()
	defer p.refreshTokensMu.Unlock()

	data, ok := p.refreshTokens[token]
	if !ok {
		return nil, errors.New("invalid refresh token")
	}

	if time.Now().After(data.ExpiresAt) {
		delete(p.refreshTokens, token)
		return nil, errors.New("refresh token has expired")
	}

	if data.ClientID != clientID {
		return nil, errors.New("client_id mismatch")
	}

	return data, nil
}

// RevokeToken marks a token as revoked.
// If the revoked tokens map exceeds maxRevokedTokens, the oldest entries are removed.
func (p *Provider) RevokeToken(token string) {
	// Remove from refresh tokens first (if it's a refresh token)
	// This prevents deadlock by always acquiring locks in the same order
	p.refreshTokensMu.Lock()
	delete(p.refreshTokens, token)
	p.refreshTokensMu.Unlock()

	// Mark as revoked with timestamp for cleanup
	p.revokedTokensMu.Lock()
	p.revokedTokens[token] = time.Now()

	if len(p.revokedTokens) > maxRevokedTokens {
		// Remove oldest revoked entries first
		cutoff := time.Now().Add(-24 * time.Hour)
		for k, v := range p.revokedTokens {
			if v.Before(cutoff) {
				delete(p.revokedTokens, k)
			}
		}
		// If still over cap, remove oldest
		for len(p.revokedTokens) > maxRevokedTokens {
			var oldestKey string
			var oldestTime time.Time
			for k, v := range p.revokedTokens {
				if oldestKey == "" || v.Before(oldestTime) {
					oldestKey = k
					oldestTime = v
				}
			}
			if oldestKey != "" {
				delete(p.revokedTokens, oldestKey)
			}
		}
	}
	p.revokedTokensMu.Unlock()
}

// GetClient returns a client configuration by ID
func (p *Provider) GetClient(clientID string) *ClientConfig {
	for i := range p.config.Clients {
		if p.config.Clients[i].ClientID == clientID {
			return &p.config.Clients[i]
		}
	}
	return nil
}

// ValidateClient validates client credentials
func (p *Provider) ValidateClient(clientID, clientSecret string) *ClientConfig {
	client := p.GetClient(clientID)
	if client == nil {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(client.ClientSecret), []byte(clientSecret)) != 1 {
		return nil
	}
	return client
}

// GetUser returns a user configuration by username
func (p *Provider) GetUser(username string) *UserConfig {
	for i := range p.config.Users {
		if p.config.Users[i].Username == username {
			return &p.config.Users[i]
		}
	}
	return nil
}

// ValidateUser validates user credentials
func (p *Provider) ValidateUser(username, password string) *UserConfig {
	user := p.GetUser(username)
	if user == nil {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(user.Password), []byte(password)) != 1 {
		return nil
	}
	return user
}

// GetUserByID returns a user by their subject ID
func (p *Provider) GetUserByID(userID string) *UserConfig {
	for i := range p.config.Users {
		if sub, ok := p.config.Users[i].Claims["sub"].(string); ok && sub == userID {
			return &p.config.Users[i]
		}
	}
	return nil
}

// ClientSupportsGrantType checks if a client supports a specific grant type
func (p *Provider) ClientSupportsGrantType(client *ClientConfig, grantType string) bool {
	for _, gt := range client.GrantTypes {
		if gt == grantType {
			return true
		}
	}
	return false
}

// IsValidRedirectURI checks if a redirect URI is valid for a client
func (p *Provider) IsValidRedirectURI(client *ClientConfig, uri string) bool {
	for _, ru := range client.RedirectURIs {
		if ru == uri {
			return true
		}
	}
	return false
}

// Config returns the provider configuration
func (p *Provider) Config() *OAuthConfig {
	return p.config
}

// TokenExpiry returns the access token expiry duration
func (p *Provider) TokenExpiry() time.Duration {
	return p.tokenExpiry
}

// RefreshExpiry returns the refresh token expiry duration
func (p *Provider) RefreshExpiry() time.Duration {
	return p.refreshExpiry
}

// Stop stops the cleanup goroutine and releases resources.
// Should be called when the provider is no longer needed.
func (p *Provider) Stop() {
	close(p.stopCleanup)
	<-p.cleanupDone
}

// cleanupLoop periodically removes expired tokens to prevent memory leaks.
func (p *Provider) cleanupLoop() {
	defer close(p.cleanupDone)

	// Run cleanup every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCleanup:
			return
		case <-ticker.C:
			p.cleanupExpiredTokens()
		}
	}
}

// cleanupExpiredTokens removes expired authorization codes, refresh tokens,
// and old revoked token entries.
func (p *Provider) cleanupExpiredTokens() {
	now := time.Now()

	// Cleanup expired authorization codes
	p.authCodesMu.Lock()
	for code, authCode := range p.authCodes {
		if now.After(authCode.ExpiresAt) {
			delete(p.authCodes, code)
		}
	}
	p.authCodesMu.Unlock()

	// Cleanup expired refresh tokens
	p.refreshTokensMu.Lock()
	for token, data := range p.refreshTokens {
		if now.After(data.ExpiresAt) {
			delete(p.refreshTokens, token)
		}
	}
	p.refreshTokensMu.Unlock()

	// Cleanup old revoked token entries (keep for 24 hours after revocation)
	// This is enough time to reject any token that was revoked but might still
	// be presented before the access token's natural expiry
	revokedCutoff := now.Add(-24 * time.Hour)
	p.revokedTokensMu.Lock()
	for token, revokedAt := range p.revokedTokens {
		if revokedAt.Before(revokedCutoff) {
			delete(p.revokedTokens, token)
		}
	}
	p.revokedTokensMu.Unlock()
}

// generateRandomString generates a cryptographically random string
func generateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random string: %w", err)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

// parseDuration parses a duration string that supports days (e.g., "7d")
func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, errors.New("empty duration string")
	}

	// Check for day suffix
	if s[len(s)-1] == 'd' {
		var days int
		_, err := fmt.Sscanf(s, "%dd", &days)
		if err != nil {
			return 0, fmt.Errorf("invalid day format: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Use standard Go duration parsing
	return time.ParseDuration(s)
}
