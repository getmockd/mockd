package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig() *OAuthConfig {
	return &OAuthConfig{
		ID:            "test-provider",
		Issuer:        "https://mock.example.com",
		TokenExpiry:   "1h",
		RefreshExpiry: "7d",
		DefaultScopes: []string{"openid", "profile", "email"},
		DefaultClaims: map[string]interface{}{
			"aud": "test-audience",
		},
		Clients: []ClientConfig{
			{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURIs: []string{"https://app.example.com/callback"},
				GrantTypes:   []string{"authorization_code", "client_credentials", "refresh_token", "password"},
			},
			{
				ClientID:     "code-only-client",
				ClientSecret: "code-secret",
				RedirectURIs: []string{"https://code.example.com/callback"},
				GrantTypes:   []string{"authorization_code"},
			},
		},
		Users: []UserConfig{
			{
				Username: "testuser",
				Password: "testpass",
				Claims: map[string]interface{}{
					"sub":   "user-123",
					"email": "test@example.com",
					"name":  "Test User",
				},
			},
			{
				Username: "admin",
				Password: "adminpass",
				Claims: map[string]interface{}{
					"sub":   "admin-456",
					"email": "admin@example.com",
					"name":  "Admin User",
					"role":  "admin",
				},
			},
		},
		Enabled: true,
	}
}

func TestNewProvider(t *testing.T) {
	t.Run("creates provider with valid config", func(t *testing.T) {
		provider, err := NewProvider(testConfig())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if provider == nil {
			t.Fatal("expected provider to be non-nil")
		}
		if provider.privateKey == nil {
			t.Error("expected privateKey to be set")
		}
		if provider.publicKey == nil {
			t.Error("expected publicKey to be set")
		}
		if provider.keyID == "" {
			t.Error("expected keyID to be set")
		}
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewProvider(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("sets default issuer", func(t *testing.T) {
		config := &OAuthConfig{}
		provider, err := NewProvider(config)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if provider.config.Issuer == "" {
			t.Error("expected default issuer to be set")
		}
	})

	t.Run("parses custom token expiry", func(t *testing.T) {
		config := &OAuthConfig{TokenExpiry: "2h"}
		provider, err := NewProvider(config)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if provider.tokenExpiry != 2*time.Hour {
			t.Errorf("expected 2h expiry, got %v", provider.tokenExpiry)
		}
	})

	t.Run("parses day duration", func(t *testing.T) {
		config := &OAuthConfig{RefreshExpiry: "14d"}
		provider, err := NewProvider(config)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if provider.refreshExpiry != 14*24*time.Hour {
			t.Errorf("expected 14 days expiry, got %v", provider.refreshExpiry)
		}
	})

	t.Run("returns error for invalid token expiry", func(t *testing.T) {
		config := &OAuthConfig{TokenExpiry: "invalid"}
		_, err := NewProvider(config)
		if err == nil {
			t.Fatal("expected error for invalid token expiry")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("generates valid token", func(t *testing.T) {
		token, err := provider.GenerateToken(map[string]interface{}{
			"sub":   "user-123",
			"scope": "openid profile",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if token == "" {
			t.Error("expected token to be non-empty")
		}

		// Validate the token
		claims, err := provider.ValidateToken(token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}
		if claims["sub"] != "user-123" {
			t.Errorf("expected sub=user-123, got %v", claims["sub"])
		}
		if claims["iss"] != "https://mock.example.com" {
			t.Errorf("expected issuer=https://mock.example.com, got %v", claims["iss"])
		}
	})

	t.Run("includes default claims", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{
			"sub": "user-123",
		})
		claims, _ := provider.ValidateToken(token)
		if claims["aud"] != "test-audience" {
			t.Errorf("expected default aud claim, got %v", claims["aud"])
		}
	})

	t.Run("custom claims override defaults", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{
			"sub": "user-123",
			"aud": "custom-audience",
		})
		claims, _ := provider.ValidateToken(token)
		if claims["aud"] != "custom-audience" {
			t.Errorf("expected custom aud claim, got %v", claims["aud"])
		}
	})
}

func TestGenerateIDToken(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("generates valid ID token", func(t *testing.T) {
		token, err := provider.GenerateIDToken(map[string]interface{}{
			"sub":   "user-123",
			"aud":   "test-client",
			"email": "test@example.com",
			"nonce": "abc123",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if token == "" {
			t.Error("expected token to be non-empty")
		}

		claims, err := provider.ValidateToken(token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}
		if claims["email"] != "test@example.com" {
			t.Errorf("expected email claim, got %v", claims["email"])
		}
		if claims["nonce"] != "abc123" {
			t.Errorf("expected nonce claim, got %v", claims["nonce"])
		}
	})

	t.Run("sets default sub if not provided", func(t *testing.T) {
		token, _ := provider.GenerateIDToken(map[string]interface{}{})
		claims, _ := provider.ValidateToken(token)
		if claims["sub"] == nil || claims["sub"] == "" {
			t.Error("expected sub to be set")
		}
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err1 := provider.GenerateRefreshToken()
		if err1 != nil {
			t.Fatalf("unexpected error generating token1: %v", err1)
		}
		token2, err2 := provider.GenerateRefreshToken()
		if err2 != nil {
			t.Fatalf("unexpected error generating token2: %v", err2)
		}

		if token1 == "" {
			t.Error("expected token to be non-empty")
		}
		if token1 == token2 {
			t.Error("expected tokens to be unique")
		}
		if len(token1) < 32 {
			t.Error("expected token to be at least 32 characters")
		}
	})
}

func TestGetJWKS(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("returns valid JWKS", func(t *testing.T) {
		jwks := provider.GetJWKS()

		keys, ok := jwks["keys"].([]map[string]interface{})
		if !ok {
			t.Fatal("expected keys array")
		}
		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}

		key := keys[0]
		if key["kty"] != "RSA" {
			t.Errorf("expected kty=RSA, got %v", key["kty"])
		}
		if key["use"] != "sig" {
			t.Errorf("expected use=sig, got %v", key["use"])
		}
		if key["alg"] != "RS256" {
			t.Errorf("expected alg=RS256, got %v", key["alg"])
		}
		if key["kid"] != provider.keyID {
			t.Errorf("expected kid=%s, got %v", provider.keyID, key["kid"])
		}
		if key["n"] == nil || key["n"] == "" {
			t.Error("expected n to be set")
		}
		if key["e"] == nil || key["e"] == "" {
			t.Error("expected e to be set")
		}
	})
}

func TestValidateToken(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("validates valid token", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})
		claims, err := provider.ValidateToken(token)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if claims["sub"] != "user-123" {
			t.Errorf("expected sub=user-123, got %v", claims["sub"])
		}
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		_, err := provider.ValidateToken("invalid-token")
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})

	t.Run("rejects revoked token", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})
		provider.RevokeToken(token)

		_, err := provider.ValidateToken(token)
		if err == nil {
			t.Fatal("expected error for revoked token")
		}
	})
}

func TestAuthorizationCodeFlow(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("stores and exchanges authorization code", func(t *testing.T) {
		code := &AuthorizationCode{
			Code:        "test-code",
			ClientID:    "test-client",
			RedirectURI: "https://app.example.com/callback",
			Scope:       "openid profile",
			UserID:      "user-123",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
		}

		provider.StoreAuthorizationCode(code)

		retrieved, err := provider.ExchangeAuthorizationCode(
			"test-code",
			"test-client",
			"https://app.example.com/callback",
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if retrieved.UserID != "user-123" {
			t.Errorf("expected userID=user-123, got %v", retrieved.UserID)
		}
	})

	t.Run("code can only be used once", func(t *testing.T) {
		code := &AuthorizationCode{
			Code:        "one-time-code",
			ClientID:    "test-client",
			RedirectURI: "https://app.example.com/callback",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
		}

		provider.StoreAuthorizationCode(code)

		// First exchange should succeed
		_, err := provider.ExchangeAuthorizationCode(
			"one-time-code",
			"test-client",
			"https://app.example.com/callback",
		)
		if err != nil {
			t.Fatalf("first exchange should succeed, got %v", err)
		}

		// Second exchange should fail
		_, err = provider.ExchangeAuthorizationCode(
			"one-time-code",
			"test-client",
			"https://app.example.com/callback",
		)
		if err == nil {
			t.Fatal("second exchange should fail")
		}
	})

	t.Run("rejects expired code", func(t *testing.T) {
		code := &AuthorizationCode{
			Code:        "expired-code",
			ClientID:    "test-client",
			RedirectURI: "https://app.example.com/callback",
			ExpiresAt:   time.Now().Add(-1 * time.Minute),
		}

		provider.StoreAuthorizationCode(code)

		_, err := provider.ExchangeAuthorizationCode(
			"expired-code",
			"test-client",
			"https://app.example.com/callback",
		)
		if err == nil {
			t.Fatal("should reject expired code")
		}
	})

	t.Run("rejects mismatched client_id", func(t *testing.T) {
		code := &AuthorizationCode{
			Code:        "client-mismatch-code",
			ClientID:    "test-client",
			RedirectURI: "https://app.example.com/callback",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
		}

		provider.StoreAuthorizationCode(code)

		_, err := provider.ExchangeAuthorizationCode(
			"client-mismatch-code",
			"wrong-client",
			"https://app.example.com/callback",
		)
		if err == nil {
			t.Fatal("should reject mismatched client_id")
		}
	})
}

func TestRefreshTokenFlow(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("stores and validates refresh token", func(t *testing.T) {
		data := &RefreshTokenData{
			Token:     "test-refresh-token",
			ClientID:  "test-client",
			UserID:    "user-123",
			Scope:     "openid profile",
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		}

		provider.StoreRefreshToken(data)

		retrieved, err := provider.ValidateRefreshToken("test-refresh-token", "test-client")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if retrieved.UserID != "user-123" {
			t.Errorf("expected userID=user-123, got %v", retrieved.UserID)
		}
	})

	t.Run("rejects expired refresh token", func(t *testing.T) {
		data := &RefreshTokenData{
			Token:     "expired-refresh-token",
			ClientID:  "test-client",
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		}

		provider.StoreRefreshToken(data)

		_, err := provider.ValidateRefreshToken("expired-refresh-token", "test-client")
		if err == nil {
			t.Fatal("should reject expired refresh token")
		}
	})
}

func TestClientValidation(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("validates correct credentials", func(t *testing.T) {
		client := provider.ValidateClient("test-client", "test-secret")
		if client == nil {
			t.Fatal("expected client to be valid")
		}
	})

	t.Run("rejects invalid secret", func(t *testing.T) {
		client := provider.ValidateClient("test-client", "wrong-secret")
		if client != nil {
			t.Fatal("expected client to be nil")
		}
	})

	t.Run("rejects unknown client", func(t *testing.T) {
		client := provider.ValidateClient("unknown-client", "secret")
		if client != nil {
			t.Fatal("expected client to be nil")
		}
	})

	t.Run("checks grant type support", func(t *testing.T) {
		client := provider.GetClient("test-client")
		if !provider.ClientSupportsGrantType(client, "authorization_code") {
			t.Error("expected client to support authorization_code")
		}
		if !provider.ClientSupportsGrantType(client, "client_credentials") {
			t.Error("expected client to support client_credentials")
		}

		codeOnlyClient := provider.GetClient("code-only-client")
		if provider.ClientSupportsGrantType(codeOnlyClient, "client_credentials") {
			t.Error("expected code-only client to not support client_credentials")
		}
	})

	t.Run("validates redirect URI", func(t *testing.T) {
		client := provider.GetClient("test-client")
		if !provider.IsValidRedirectURI(client, "https://app.example.com/callback") {
			t.Error("expected valid redirect URI")
		}
		if provider.IsValidRedirectURI(client, "https://evil.example.com/callback") {
			t.Error("expected invalid redirect URI")
		}
	})
}

func TestUserValidation(t *testing.T) {
	provider, _ := NewProvider(testConfig())

	t.Run("validates correct credentials", func(t *testing.T) {
		user := provider.ValidateUser("testuser", "testpass")
		if user == nil {
			t.Fatal("expected user to be valid")
		}
	})

	t.Run("rejects invalid password", func(t *testing.T) {
		user := provider.ValidateUser("testuser", "wrongpass")
		if user != nil {
			t.Fatal("expected user to be nil")
		}
	})

	t.Run("gets user by ID", func(t *testing.T) {
		user := provider.GetUserByID("user-123")
		if user == nil {
			t.Fatal("expected user to be found")
		}
		if user.Username != "testuser" {
			t.Errorf("expected username=testuser, got %v", user.Username)
		}
	})
}

// HTTP Handler Tests

func TestHandleAuthorize(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("redirects with code for authorization_code flow", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=test-client&redirect_uri=https://app.example.com/callback&response_type=code&scope=openid&state=xyz", nil)
		rec := httptest.NewRecorder()

		handler.HandleAuthorize(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("expected status 302, got %d", rec.Code)
		}

		location := rec.Header().Get("Location")
		u, _ := url.Parse(location)
		if u.Query().Get("code") == "" {
			t.Error("expected code in redirect")
		}
		if u.Query().Get("state") != "xyz" {
			t.Errorf("expected state=xyz, got %v", u.Query().Get("state"))
		}
	})

	t.Run("returns error for missing client_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/authorize?redirect_uri=https://app.example.com/callback&response_type=code", nil)
		rec := httptest.NewRecorder()

		handler.HandleAuthorize(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns error for invalid redirect_uri", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=test-client&redirect_uri=https://evil.example.com/callback&response_type=code", nil)
		rec := httptest.NewRecorder()

		handler.HandleAuthorize(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestHandleToken(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("client_credentials grant returns token", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")
		form.Set("scope", "openid")

		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleToken(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response TokenResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &response)

		if response.AccessToken == "" {
			t.Error("expected access_token")
		}
		if response.TokenType != "Bearer" {
			t.Errorf("expected token_type=Bearer, got %v", response.TokenType)
		}
	})

	t.Run("authorization_code grant returns tokens", func(t *testing.T) {
		// First, store an authorization code
		code := &AuthorizationCode{
			Code:        "auth-code-for-test",
			ClientID:    "test-client",
			RedirectURI: "https://app.example.com/callback",
			Scope:       "openid profile",
			UserID:      "user-123",
			Nonce:       "test-nonce",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
		}
		provider.StoreAuthorizationCode(code)

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", "auth-code-for-test")
		form.Set("redirect_uri", "https://app.example.com/callback")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleToken(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response TokenResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &response)

		if response.AccessToken == "" {
			t.Error("expected access_token")
		}
		if response.RefreshToken == "" {
			t.Error("expected refresh_token")
		}
		if response.IDToken == "" {
			t.Error("expected id_token for openid scope")
		}
	})

	t.Run("refresh_token grant returns new tokens", func(t *testing.T) {
		// Store a refresh token
		provider.StoreRefreshToken(&RefreshTokenData{
			Token:     "refresh-token-for-test",
			ClientID:  "test-client",
			UserID:    "user-123",
			Scope:     "openid profile",
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		})

		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", "refresh-token-for-test")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleToken(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response TokenResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &response)

		if response.AccessToken == "" {
			t.Error("expected access_token")
		}
	})

	t.Run("password grant returns tokens", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "password")
		form.Set("username", "testuser")
		form.Set("password", "testpass")
		form.Set("scope", "openid profile")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleToken(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response TokenResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if response.AccessToken == "" {
			t.Error("expected access_token")
		}
		if response.RefreshToken == "" {
			t.Error("expected refresh_token")
		}
		if response.IDToken == "" {
			t.Error("expected id_token for openid scope")
		}
	})

	t.Run("rejects invalid client credentials", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "wrong-secret")

		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleToken(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("rejects unsupported grant type", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "unsupported")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleToken(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestHandleUserInfo(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("returns user info for valid token", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{
			"sub": "user-123",
		})

		req := httptest.NewRequest(http.MethodGet, "/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.HandleUserInfo(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var userInfo map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &userInfo)

		if userInfo["sub"] != "user-123" {
			t.Errorf("expected sub=user-123, got %v", userInfo["sub"])
		}
		if userInfo["email"] != "test@example.com" {
			t.Errorf("expected email=test@example.com, got %v", userInfo["email"])
		}
	})

	t.Run("returns 401 for missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/userinfo", nil)
		rec := httptest.NewRecorder()

		handler.HandleUserInfo(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("returns 401 for invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/userinfo", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		handler.HandleUserInfo(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestHandleJWKS(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("returns JWKS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
		rec := httptest.NewRecorder()

		handler.HandleJWKS(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var jwks map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &jwks)

		keys, ok := jwks["keys"].([]interface{})
		if !ok {
			t.Fatal("expected keys array")
		}
		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}
	})
}

func TestHandleOpenIDConfig(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("returns OpenID configuration", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
		rec := httptest.NewRecorder()

		handler.HandleOpenIDConfig(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var config OpenIDConfiguration
		json.Unmarshal(rec.Body.Bytes(), &config)

		if config.Issuer != "https://mock.example.com" {
			t.Errorf("expected issuer=https://mock.example.com, got %v", config.Issuer)
		}
		if config.AuthorizationEndpoint != "https://mock.example.com/authorize" {
			t.Errorf("expected authorization_endpoint, got %v", config.AuthorizationEndpoint)
		}
		if config.TokenEndpoint != "https://mock.example.com/token" {
			t.Errorf("expected token_endpoint, got %v", config.TokenEndpoint)
		}
		if config.JwksURI != "https://mock.example.com/.well-known/jwks.json" {
			t.Errorf("expected jwks_uri, got %v", config.JwksURI)
		}
	})
}

func TestHandleRevoke(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("revokes token successfully", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})

		form := url.Values{}
		form.Set("token", token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleRevoke(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		// Verify token is revoked
		_, err := provider.ValidateToken(token)
		if err == nil {
			t.Error("expected token to be revoked")
		}
	})

	t.Run("returns 200 for unknown token (per RFC 7009)", func(t *testing.T) {
		form := url.Values{}
		form.Set("token", "unknown-token")

		req := httptest.NewRequest(http.MethodPost, "/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleRevoke(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"14d", 14 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"", 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := parseDuration(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
				if d != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, d)
				}
			}
		})
	}
}

func TestProvider_Stop(t *testing.T) {
	provider, err := NewProvider(testConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		provider.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not complete in time - possible deadlock")
	}
}

func TestProvider_RevokeToken_NoDeadlock(t *testing.T) {
	provider, err := NewProvider(testConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Stop()

	// Test concurrent RevokeToken and ValidateRefreshToken calls
	// This exercises the lock ordering that could cause deadlock
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		// Store a refresh token
		token := fmt.Sprintf("refresh-token-%d", i)
		provider.StoreRefreshToken(&RefreshTokenData{
			Token:     token,
			ClientID:  "test-client",
			UserID:    "user-123",
			ExpiresAt: time.Now().Add(time.Hour),
		})
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			wg.Add(2)
			token := fmt.Sprintf("refresh-token-%d", i%10)

			go func(t string) {
				defer wg.Done()
				provider.RevokeToken(t)
			}(token)

			go func(t string) {
				defer wg.Done()
				// This validates and may try to delete expired tokens
				provider.ValidateRefreshToken(t, "test-client")
			}(token)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - possible deadlock in RevokeToken")
	}
}

func TestProvider_CleanupExpiredTokens(t *testing.T) {
	provider, err := NewProvider(testConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Stop()

	// Store an expired auth code
	provider.StoreAuthorizationCode(&AuthorizationCode{
		Code:      "expired-code",
		ClientID:  "test-client",
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	})

	// Store an expired refresh token
	provider.StoreRefreshToken(&RefreshTokenData{
		Token:     "expired-refresh",
		ClientID:  "test-client",
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	})

	// Manually trigger cleanup
	provider.cleanupExpiredTokens()

	// Verify expired tokens are cleaned up
	provider.authCodesMu.RLock()
	_, hasCode := provider.authCodes["expired-code"]
	provider.authCodesMu.RUnlock()
	if hasCode {
		t.Error("expected expired auth code to be cleaned up")
	}

	provider.refreshTokensMu.RLock()
	_, hasRefresh := provider.refreshTokens["expired-refresh"]
	provider.refreshTokensMu.RUnlock()
	if hasRefresh {
		t.Error("expected expired refresh token to be cleaned up")
	}
}

// ============================================================================
// Token Introspection Tests (RFC 7662)
// ============================================================================

func TestHandleIntrospect(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	t.Run("returns active=true for valid token with all claims", func(t *testing.T) {
		// Generate a token with various claims
		token, _ := provider.GenerateToken(map[string]interface{}{
			"sub":       "user-123",
			"scope":     "openid profile email",
			"client_id": "test-client",
		})

		form := url.Values{}
		form.Set("token", token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response IntrospectionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if !response.Active {
			t.Error("expected active=true for valid token")
		}
		if response.Subject != "user-123" {
			t.Errorf("expected sub=user-123, got %v", response.Subject)
		}
		if response.Scope != "openid profile email" {
			t.Errorf("expected scope='openid profile email', got %v", response.Scope)
		}
		if response.ClientID != "test-client" {
			t.Errorf("expected client_id=test-client, got %v", response.ClientID)
		}
		if response.TokenType != "Bearer" {
			t.Errorf("expected token_type=Bearer, got %v", response.TokenType)
		}
		if response.Issuer != "https://mock.example.com" {
			t.Errorf("expected iss=https://mock.example.com, got %v", response.Issuer)
		}
		if response.ExpiresAt == 0 {
			t.Error("expected exp to be set")
		}
		if response.IssuedAt == 0 {
			t.Error("expected iat to be set")
		}
	})

	t.Run("returns active=false for invalid token", func(t *testing.T) {
		form := url.Values{}
		form.Set("token", "invalid-token-string")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response IntrospectionResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if response.Active {
			t.Error("expected active=false for invalid token")
		}
	})

	t.Run("returns active=false for revoked token", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})
		provider.RevokeToken(token)

		form := url.Values{}
		form.Set("token", token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var response IntrospectionResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if response.Active {
			t.Error("expected active=false for revoked token")
		}
	})

	t.Run("accepts Basic Auth for client credentials", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})

		form := url.Values{}
		form.Set("token", token)

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "test-secret")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response IntrospectionResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if !response.Active {
			t.Error("expected active=true when using Basic Auth")
		}
	})

	t.Run("rejects missing token parameter", func(t *testing.T) {
		form := url.Values{}
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("rejects missing client authentication", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})

		form := url.Values{}
		form.Set("token", token)

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("rejects invalid client credentials", func(t *testing.T) {
		token, _ := provider.GenerateToken(map[string]interface{}{"sub": "user-123"})

		form := url.Values{}
		form.Set("token", token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "wrong-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("rejects GET method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/introspect", nil)
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status 405, got %d", rec.Code)
		}
	})

	t.Run("handles token with array audience claim", func(t *testing.T) {
		// This tests the audience extraction for array format
		token, _ := provider.GenerateToken(map[string]interface{}{
			"sub": "user-123",
			"aud": []string{"client1", "client2"},
		})

		form := url.Values{}
		form.Set("token", token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var response IntrospectionResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if !response.Active {
			t.Error("expected active=true")
		}
		// Audience extraction takes first element from array
		if response.Audience != "client1" {
			t.Errorf("expected audience=client1, got %v", response.Audience)
		}
	})

	t.Run("introspection includes default claims from config", func(t *testing.T) {
		// The test config has default claims including "aud": "test-audience"
		token, _ := provider.GenerateToken(map[string]interface{}{
			"sub": "user-123",
		})

		form := url.Values{}
		form.Set("token", token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleIntrospect(rec, req)

		var response IntrospectionResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if !response.Active {
			t.Fatal("expected active=true")
		}
		// Should include the default audience from config
		if response.Audience != "test-audience" {
			t.Errorf("expected audience=test-audience (from config defaults), got %v", response.Audience)
		}
	})
}

func TestHandleOpenIDConfig_IncludesIntrospection(t *testing.T) {
	provider, _ := NewProvider(testConfig())
	handler := NewHandler(provider)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	rec := httptest.NewRecorder()

	handler.HandleOpenIDConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var config OpenIDConfiguration
	json.Unmarshal(rec.Body.Bytes(), &config)

	if config.IntrospectionEndpoint != "https://mock.example.com/introspect" {
		t.Errorf("expected introspection_endpoint=https://mock.example.com/introspect, got %v", config.IntrospectionEndpoint)
	}
}
