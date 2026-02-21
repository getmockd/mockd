package integration

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// Suppress unused import warning
var _ = io.Discard

// ============================================================================
// OAuth Test Types
// ============================================================================

// oauthTestBundle groups server and client for OAuth tests.
type oauthTestBundle struct {
	Server       *engine.Server
	Client       *engineclient.Client
	HTTPPort     int
	BaseURL      string
	OAuthBaseURL string
}

// tokenResponse represents an OAuth token response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// errorResponse represents an OAuth error response.
type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// openIDConfiguration represents the OIDC discovery document.
type openIDConfiguration struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserInfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
}

// jwks represents a JSON Web Key Set.
type jwks struct {
	Keys []jwk `json:"keys"`
}

// jwk represents a JSON Web Key.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// introspectionResponse represents a token introspection response.
type introspectionResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	NotBefore int64  `json:"nbf,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Audience  string `json:"aud,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	TokenID   string `json:"jti,omitempty"`
}

// userInfoResponse represents userinfo endpoint response.
type userInfoResponse struct {
	Sub   string `json:"sub"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// ============================================================================
// Test Setup
// ============================================================================

// setupOAuthServer creates an OAuth mock server for testing.
func setupOAuthServer(t *testing.T) *oauthTestBundle {
	httpPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		srv.Stop()
	})

	waitForReady(t, srv.ManagementPort())

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))
	baseURL := fmt.Sprintf("http://localhost:%d", httpPort)
	oauthBaseURL := baseURL + "/oauth"

	// Create the OAuth mock
	oauthMock := &config.MockConfiguration{
		ID:      "test-oauth-provider",
		Name:    "Test OAuth Provider",
		Enabled: boolPtr(true),
		Type:    mock.TypeOAuth,
		OAuth: &mock.OAuthSpec{
			Issuer:        oauthBaseURL,
			TokenExpiry:   "1h",
			RefreshExpiry: "7d",
			DefaultScopes: []string{"openid", "profile", "email", "api:read", "api:write"},
			Clients: []mock.OAuthClient{
				{
					ClientID:     "test-client",
					ClientSecret: "test-secret",
					RedirectURIs: []string{"http://localhost/callback", "http://localhost:3000/callback"},
					GrantTypes:   []string{"authorization_code", "client_credentials", "password", "refresh_token"},
				},
				{
					ClientID:     "public-client",
					ClientSecret: "public-secret",
					RedirectURIs: []string{"http://localhost/callback"},
					GrantTypes:   []string{"authorization_code"},
				},
			},
			Users: []mock.OAuthUser{
				{
					Username: "testuser",
					Password: "testpass",
					Claims: map[string]string{
						"sub":   "user-123",
						"name":  "Test User",
						"email": "test@example.com",
					},
				},
				{
					Username: "admin",
					Password: "adminpass",
					Claims: map[string]string{
						"sub":   "admin-456",
						"name":  "Admin User",
						"email": "admin@example.com",
					},
				},
			},
		},
	}

	_, err = client.CreateMock(context.Background(), oauthMock)
	require.NoError(t, err)

	// Wait for OAuth provider to be registered
	time.Sleep(50 * time.Millisecond)

	return &oauthTestBundle{
		Server:       srv,
		Client:       client,
		HTTPPort:     httpPort,
		BaseURL:      baseURL,
		OAuthBaseURL: oauthBaseURL,
	}
}

// ============================================================================
// Test 1: OIDC Discovery
// ============================================================================

func TestOAuth_OIDCDiscovery(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("returns valid OIDC discovery document", func(t *testing.T) {
		resp, err := http.Get(bundle.OAuthBaseURL + "/.well-known/openid-configuration")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var config openIDConfiguration
		err = json.NewDecoder(resp.Body).Decode(&config)
		require.NoError(t, err)

		// Verify required fields
		assert.Equal(t, bundle.OAuthBaseURL, config.Issuer)
		assert.Equal(t, bundle.OAuthBaseURL+"/authorize", config.AuthorizationEndpoint)
		assert.Equal(t, bundle.OAuthBaseURL+"/token", config.TokenEndpoint)
		assert.Equal(t, bundle.OAuthBaseURL+"/userinfo", config.UserInfoEndpoint)
		assert.Equal(t, bundle.OAuthBaseURL+"/.well-known/jwks.json", config.JwksURI)
		assert.Equal(t, bundle.OAuthBaseURL+"/introspect", config.IntrospectionEndpoint)
		assert.Equal(t, bundle.OAuthBaseURL+"/revoke", config.RevocationEndpoint)

		// Verify supported features
		assert.Contains(t, config.ResponseTypesSupported, "code")
		assert.Contains(t, config.ResponseTypesSupported, "token")
		assert.Contains(t, config.GrantTypesSupported, "authorization_code")
		assert.Contains(t, config.GrantTypesSupported, "client_credentials")
		assert.Contains(t, config.GrantTypesSupported, "refresh_token")
		assert.Contains(t, config.GrantTypesSupported, "password")
		assert.Contains(t, config.IDTokenSigningAlgValuesSupported, "RS256")
		assert.Contains(t, config.TokenEndpointAuthMethodsSupported, "client_secret_basic")
		assert.Contains(t, config.TokenEndpointAuthMethodsSupported, "client_secret_post")
		assert.Contains(t, config.SubjectTypesSupported, "public")
	})

	t.Run("discovery document has no-cache headers", func(t *testing.T) {
		resp, err := http.Get(bundle.OAuthBaseURL + "/.well-known/openid-configuration")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "no-cache", resp.Header.Get("Pragma"))
	})
}

// ============================================================================
// Test 2: JWKS Endpoint
// ============================================================================

func TestOAuth_JWKS(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("returns valid JWKS", func(t *testing.T) {
		resp, err := http.Get(bundle.OAuthBaseURL + "/.well-known/jwks.json")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var keys jwks
		err = json.NewDecoder(resp.Body).Decode(&keys)
		require.NoError(t, err)

		require.Len(t, keys.Keys, 1)
		key := keys.Keys[0]

		assert.Equal(t, "RSA", key.Kty)
		assert.Equal(t, "sig", key.Use)
		assert.Equal(t, "RS256", key.Alg)
		assert.NotEmpty(t, key.Kid)
		assert.NotEmpty(t, key.N)
		assert.NotEmpty(t, key.E)
	})

	t.Run("JWKS can be used to verify tokens", func(t *testing.T) {
		// First get a token
		token := getClientCredentialsToken(t, bundle, "api:read")

		// Get JWKS
		resp, err := http.Get(bundle.OAuthBaseURL + "/.well-known/jwks.json")
		require.NoError(t, err)
		defer resp.Body.Close()

		var keys jwks
		err = json.NewDecoder(resp.Body).Decode(&keys)
		require.NoError(t, err)

		// Build RSA public key from JWKS
		key := keys.Keys[0]
		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		require.NoError(t, err)
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		require.NoError(t, err)

		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		publicKey := &rsa.PublicKey{N: n, E: e}

		// Parse and verify token
		parsedToken, err := jwt.Parse(token.AccessToken, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return publicKey, nil
		})
		require.NoError(t, err)
		assert.True(t, parsedToken.Valid)
	})
}

// ============================================================================
// Test 3: Authorization Code Flow
// ============================================================================

func TestOAuth_AuthorizationCodeFlow(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("authorize endpoint redirects with code", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		}

		params := url.Values{}
		params.Set("client_id", "test-client")
		params.Set("response_type", "code")
		params.Set("redirect_uri", "http://localhost/callback")
		params.Set("state", "xyz123")
		params.Set("scope", "openid profile email")

		authURL := bundle.OAuthBaseURL + "/authorize?" + params.Encode()

		resp, err := client.Get(authURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusFound, resp.StatusCode)

		location := resp.Header.Get("Location")
		require.NotEmpty(t, location)

		redirectURL, err := url.Parse(location)
		require.NoError(t, err)

		// Verify redirect contains code and state
		assert.NotEmpty(t, redirectURL.Query().Get("code"))
		assert.Equal(t, "xyz123", redirectURL.Query().Get("state"))
		assert.Equal(t, "localhost", redirectURL.Host)
		assert.Equal(t, "/callback", redirectURL.Path)
	})

	t.Run("full authorization code flow with token exchange", func(t *testing.T) {
		// Step 1: Get authorization code
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		params := url.Values{}
		params.Set("client_id", "test-client")
		params.Set("response_type", "code")
		params.Set("redirect_uri", "http://localhost/callback")
		params.Set("state", "test-state")
		params.Set("scope", "openid profile email")

		authURL := bundle.OAuthBaseURL + "/authorize?" + params.Encode()

		resp, err := client.Get(authURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusFound, resp.StatusCode)

		location := resp.Header.Get("Location")
		redirectURL, err := url.Parse(location)
		require.NoError(t, err)

		code := redirectURL.Query().Get("code")
		require.NotEmpty(t, code)

		// Step 2: Exchange code for token
		data := url.Values{}
		data.Set("grant_type", "authorization_code")
		data.Set("code", code)
		data.Set("redirect_uri", "http://localhost/callback")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		tokenResp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer tokenResp.Body.Close()

		require.Equal(t, http.StatusOK, tokenResp.StatusCode)

		var token tokenResponse
		err = json.NewDecoder(tokenResp.Body).Decode(&token)
		require.NoError(t, err)

		assert.NotEmpty(t, token.AccessToken)
		assert.Equal(t, "Bearer", token.TokenType)
		assert.Greater(t, token.ExpiresIn, 0)
		assert.NotEmpty(t, token.RefreshToken) // test-client supports refresh_token grant
		assert.NotEmpty(t, token.IDToken)      // openid scope was requested
	})

	t.Run("authorization code can only be used once", func(t *testing.T) {
		// Get authorization code
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		authURL := fmt.Sprintf("%s/authorize?client_id=test-client&response_type=code&redirect_uri=%s&state=once-test",
			bundle.OAuthBaseURL,
			url.QueryEscape("http://localhost/callback"))

		resp, err := client.Get(authURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		location := resp.Header.Get("Location")
		redirectURL, _ := url.Parse(location)
		code := redirectURL.Query().Get("code")

		// First exchange should succeed
		data := url.Values{}
		data.Set("grant_type", "authorization_code")
		data.Set("code", code)
		data.Set("redirect_uri", "http://localhost/callback")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		tokenResp1, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		tokenResp1.Body.Close()
		require.Equal(t, http.StatusOK, tokenResp1.StatusCode)

		// Second exchange should fail
		tokenResp2, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer tokenResp2.Body.Close()

		require.Equal(t, http.StatusBadRequest, tokenResp2.StatusCode)

		var errResp errorResponse
		json.NewDecoder(tokenResp2.Body).Decode(&errResp)
		assert.Equal(t, "invalid_grant", errResp.Error)
	})

	t.Run("invalid redirect_uri returns error", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/authorize?client_id=test-client&response_type=code&redirect_uri=%s",
			bundle.OAuthBaseURL,
			url.QueryEscape("http://evil.com/callback")))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_request", errResp.Error)
	})
}

// ============================================================================
// Test 4: Client Credentials Flow
// ============================================================================

func TestOAuth_ClientCredentialsFlow(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("successful client credentials grant", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "client_credentials")
		data.Set("scope", "api:read api:write")

		req, err := http.NewRequest("POST", bundle.OAuthBaseURL+"/token", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "test-secret")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var token tokenResponse
		err = json.NewDecoder(resp.Body).Decode(&token)
		require.NoError(t, err)

		assert.NotEmpty(t, token.AccessToken)
		assert.Equal(t, "Bearer", token.TokenType)
		assert.Greater(t, token.ExpiresIn, 0)
		assert.Equal(t, "api:read api:write", token.Scope)
		assert.Empty(t, token.RefreshToken) // No refresh token for client_credentials
		assert.Empty(t, token.IDToken)      // No ID token for client_credentials
	})

	t.Run("client credentials with form body authentication", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")
		data.Set("scope", "api:read")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var token tokenResponse
		json.NewDecoder(resp.Body).Decode(&token)
		assert.NotEmpty(t, token.AccessToken)
	})

	t.Run("invalid client credentials returns error", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "client_credentials")

		req, err := http.NewRequest("POST", bundle.OAuthBaseURL+"/token", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "wrong-secret")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_client", errResp.Error)
	})

	t.Run("client without client_credentials grant returns error", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", "public-client")
		data.Set("client_secret", "public-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "unauthorized_client", errResp.Error)
	})
}

// ============================================================================
// Test 5: Password Grant
// ============================================================================

func TestOAuth_PasswordGrant(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("successful password grant", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "password")
		data.Set("username", "testuser")
		data.Set("password", "testpass")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")
		data.Set("scope", "openid profile email")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var token tokenResponse
		err = json.NewDecoder(resp.Body).Decode(&token)
		require.NoError(t, err)

		assert.NotEmpty(t, token.AccessToken)
		assert.Equal(t, "Bearer", token.TokenType)
		assert.Greater(t, token.ExpiresIn, 0)
		assert.NotEmpty(t, token.RefreshToken)
		assert.NotEmpty(t, token.IDToken) // openid scope was requested
	})

	t.Run("invalid password returns error", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "password")
		data.Set("username", "testuser")
		data.Set("password", "wrongpassword")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_grant", errResp.Error)
	})

	t.Run("unknown user returns error", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "password")
		data.Set("username", "unknownuser")
		data.Set("password", "somepass")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_grant", errResp.Error)
	})

	t.Run("different user gets different subject", func(t *testing.T) {
		// Get token for testuser
		token1 := getPasswordToken(t, bundle, "testuser", "testpass", "openid")

		// Get token for admin
		token2 := getPasswordToken(t, bundle, "admin", "adminpass", "openid")

		// Parse tokens and verify different subjects
		claims1 := parseTokenClaims(t, token1.AccessToken)
		claims2 := parseTokenClaims(t, token2.AccessToken)

		assert.Equal(t, "user-123", claims1["sub"])
		assert.Equal(t, "admin-456", claims2["sub"])
	})
}

// ============================================================================
// Test 6: Refresh Token Flow
// ============================================================================

func TestOAuth_RefreshTokenFlow(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("successful refresh token grant", func(t *testing.T) {
		// First get a token with password grant
		initialToken := getPasswordToken(t, bundle, "testuser", "testpass", "openid profile")
		require.NotEmpty(t, initialToken.RefreshToken)

		// Now refresh the token
		data := url.Values{}
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", initialToken.RefreshToken)
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var refreshedToken tokenResponse
		err = json.NewDecoder(resp.Body).Decode(&refreshedToken)
		require.NoError(t, err)

		assert.NotEmpty(t, refreshedToken.AccessToken)
		assert.NotEqual(t, initialToken.AccessToken, refreshedToken.AccessToken) // New access token
		assert.Equal(t, "Bearer", refreshedToken.TokenType)
		assert.NotEmpty(t, refreshedToken.RefreshToken)
		assert.NotEmpty(t, refreshedToken.IDToken) // openid scope preserved
	})

	t.Run("refresh token preserves original scope", func(t *testing.T) {
		// Get token with specific scope
		initialToken := getPasswordToken(t, bundle, "testuser", "testpass", "openid api:read")
		require.NotEmpty(t, initialToken.RefreshToken)

		// Refresh without specifying scope
		data := url.Values{}
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", initialToken.RefreshToken)
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		var refreshedToken tokenResponse
		json.NewDecoder(resp.Body).Decode(&refreshedToken)

		// Verify scope is preserved
		assert.Equal(t, "openid api:read", refreshedToken.Scope)
	})

	t.Run("invalid refresh token returns error", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", "invalid-refresh-token")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_grant", errResp.Error)
	})

	t.Run("refresh token with wrong client_id returns error", func(t *testing.T) {
		// Get token for test-client
		initialToken := getPasswordToken(t, bundle, "testuser", "testpass", "openid")
		require.NotEmpty(t, initialToken.RefreshToken)

		// Try to use refresh token with different client (public-client doesn't support refresh_token)
		data := url.Values{}
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", initialToken.RefreshToken)
		data.Set("client_id", "public-client")
		data.Set("client_secret", "public-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "unauthorized_client", errResp.Error)
	})
}

// ============================================================================
// Test 7: Token Introspection (RFC 7662)
// ============================================================================

func TestOAuth_TokenIntrospection(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("active token returns full claims", func(t *testing.T) {
		// Get a token
		token := getClientCredentialsToken(t, bundle, "api:read api:write")

		// Introspect the token
		data := url.Values{}
		data.Set("token", token.AccessToken)
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/introspect", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var introspect introspectionResponse
		err = json.NewDecoder(resp.Body).Decode(&introspect)
		require.NoError(t, err)

		assert.True(t, introspect.Active)
		assert.Equal(t, "test-client", introspect.Subject) // client_credentials uses client as subject
		assert.Equal(t, bundle.OAuthBaseURL, introspect.Issuer)
		assert.Equal(t, "api:read api:write", introspect.Scope)
		assert.Equal(t, "Bearer", introspect.TokenType)
		assert.Greater(t, introspect.ExpiresAt, int64(0))
		assert.Greater(t, introspect.IssuedAt, int64(0))
		assert.NotEmpty(t, introspect.TokenID)
	})

	t.Run("introspect with Basic auth", func(t *testing.T) {
		token := getClientCredentialsToken(t, bundle, "api:read")

		data := url.Values{}
		data.Set("token", token.AccessToken)

		req, err := http.NewRequest("POST", bundle.OAuthBaseURL+"/introspect", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "test-secret")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var introspect introspectionResponse
		json.NewDecoder(resp.Body).Decode(&introspect)
		assert.True(t, introspect.Active)
	})

	t.Run("invalid token returns active false", func(t *testing.T) {
		data := url.Values{}
		data.Set("token", "invalid-token-value")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/introspect", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var introspect introspectionResponse
		json.NewDecoder(resp.Body).Decode(&introspect)
		assert.False(t, introspect.Active)
	})

	t.Run("missing client credentials returns error", func(t *testing.T) {
		data := url.Values{}
		data.Set("token", "some-token")

		resp, err := http.Post(bundle.OAuthBaseURL+"/introspect", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_client", errResp.Error)
	})

	t.Run("revoked token returns active false", func(t *testing.T) {
		// Get a token
		token := getClientCredentialsToken(t, bundle, "api:read")

		// Revoke it
		data := url.Values{}
		data.Set("token", token.AccessToken)
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		revokeResp, err := http.Post(bundle.OAuthBaseURL+"/revoke", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		revokeResp.Body.Close()
		require.Equal(t, http.StatusOK, revokeResp.StatusCode)

		// Introspect the revoked token
		introspectResp, err := http.Post(bundle.OAuthBaseURL+"/introspect", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer introspectResp.Body.Close()

		var introspect introspectionResponse
		json.NewDecoder(introspectResp.Body).Decode(&introspect)
		assert.False(t, introspect.Active)
	})
}

// ============================================================================
// Test 8: UserInfo Endpoint (OIDC)
// ============================================================================

func TestOAuth_UserInfo(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("returns user claims with valid token", func(t *testing.T) {
		// Get token for testuser
		token := getPasswordToken(t, bundle, "testuser", "testpass", "openid profile email")

		req, err := http.NewRequest("GET", bundle.OAuthBaseURL+"/userinfo", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var userInfo userInfoResponse
		err = json.NewDecoder(resp.Body).Decode(&userInfo)
		require.NoError(t, err)

		assert.Equal(t, "user-123", userInfo.Sub)
		assert.Equal(t, "Test User", userInfo.Name)
		assert.Equal(t, "test@example.com", userInfo.Email)
	})

	t.Run("missing authorization returns 401", func(t *testing.T) {
		resp, err := http.Get(bundle.OAuthBaseURL + "/userinfo")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "Bearer")
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req, err := http.NewRequest("GET", bundle.OAuthBaseURL+"/userinfo", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-token")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "invalid_token")
	})

	t.Run("returns different claims for different users", func(t *testing.T) {
		// Get token for admin
		token := getPasswordToken(t, bundle, "admin", "adminpass", "openid profile email")

		req, err := http.NewRequest("GET", bundle.OAuthBaseURL+"/userinfo", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var userInfo userInfoResponse
		json.NewDecoder(resp.Body).Decode(&userInfo)

		assert.Equal(t, "admin-456", userInfo.Sub)
		assert.Equal(t, "Admin User", userInfo.Name)
		assert.Equal(t, "admin@example.com", userInfo.Email)
	})
}

// ============================================================================
// Test 9: Error Handling
// ============================================================================

func TestOAuth_ErrorHandling(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("invalid client_id returns invalid_client", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", "unknown-client")
		data.Set("client_secret", "secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_client", errResp.Error)
	})

	t.Run("unsupported grant_type returns unsupported_grant_type", func(t *testing.T) {
		data := url.Values{}
		data.Set("grant_type", "implicit")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "unsupported_grant_type", errResp.Error)
	})

	t.Run("missing required parameters returns invalid_request", func(t *testing.T) {
		// Missing client_id in authorize
		resp, err := http.Get(bundle.OAuthBaseURL + "/authorize?response_type=code&redirect_uri=http://localhost/callback")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_request", errResp.Error)
	})

	t.Run("invalid response_type returns unsupported_response_type", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		authURL := fmt.Sprintf("%s/authorize?client_id=test-client&response_type=invalid&redirect_uri=%s",
			bundle.OAuthBaseURL,
			url.QueryEscape("http://localhost/callback"))

		resp, err := client.Get(authURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should redirect with error
		require.Equal(t, http.StatusFound, resp.StatusCode)

		location := resp.Header.Get("Location")
		redirectURL, _ := url.Parse(location)
		assert.Equal(t, "unsupported_response_type", redirectURL.Query().Get("error"))
	})

	t.Run("expired code returns invalid_grant", func(t *testing.T) {
		// Use a fake expired code
		data := url.Values{}
		data.Set("grant_type", "authorization_code")
		data.Set("code", "expired-or-nonexistent-code")
		data.Set("redirect_uri", "http://localhost/callback")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Equal(t, "invalid_grant", errResp.Error)
	})

	t.Run("method not allowed returns error", func(t *testing.T) {
		// GET on /token should fail
		resp, err := http.Get(bundle.OAuthBaseURL + "/token")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

// ============================================================================
// Test 10: JWT Token Validation
// ============================================================================

func TestOAuth_JWTTokenValidation(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("access token has valid JWT structure", func(t *testing.T) {
		token := getClientCredentialsToken(t, bundle, "api:read")

		// Parse the token (don't verify signature yet)
		parts := strings.Split(token.AccessToken, ".")
		require.Len(t, parts, 3, "JWT should have 3 parts")

		// Decode header
		headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
		require.NoError(t, err)

		var header map[string]interface{}
		err = json.Unmarshal(headerJSON, &header)
		require.NoError(t, err)

		assert.Equal(t, "RS256", header["alg"])
		assert.Equal(t, "JWT", header["typ"])
		assert.NotEmpty(t, header["kid"])
	})

	t.Run("token claims are correct", func(t *testing.T) {
		token := getClientCredentialsToken(t, bundle, "api:read api:write")

		claims := parseTokenClaims(t, token.AccessToken)

		assert.Equal(t, bundle.OAuthBaseURL, claims["iss"])
		assert.Equal(t, "test-client", claims["sub"])
		assert.Equal(t, "test-client", claims["client_id"])
		assert.Equal(t, "api:read api:write", claims["scope"])
		assert.NotNil(t, claims["iat"])
		assert.NotNil(t, claims["exp"])
		assert.NotNil(t, claims["jti"])

		// Verify exp > iat
		exp := int64(claims["exp"].(float64))
		iat := int64(claims["iat"].(float64))
		assert.Greater(t, exp, iat)
	})

	t.Run("token signature is valid against JWKS", func(t *testing.T) {
		token := getClientCredentialsToken(t, bundle, "api:read")

		// Get public key from JWKS
		resp, err := http.Get(bundle.OAuthBaseURL + "/.well-known/jwks.json")
		require.NoError(t, err)
		defer resp.Body.Close()

		var keys jwks
		json.NewDecoder(resp.Body).Decode(&keys)

		key := keys.Keys[0]
		nBytes, _ := base64.RawURLEncoding.DecodeString(key.N)
		eBytes, _ := base64.RawURLEncoding.DecodeString(key.E)

		publicKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		}

		// Verify signature
		parsedToken, err := jwt.Parse(token.AccessToken, func(t *jwt.Token) (interface{}, error) {
			// Verify kid matches
			kid, ok := t.Header["kid"].(string)
			if !ok || kid != key.Kid {
				return nil, fmt.Errorf("unexpected kid")
			}
			return publicKey, nil
		})

		require.NoError(t, err)
		assert.True(t, parsedToken.Valid)
	})

	t.Run("ID token includes user claims", func(t *testing.T) {
		token := getPasswordToken(t, bundle, "testuser", "testpass", "openid profile email")
		require.NotEmpty(t, token.IDToken)

		claims := parseTokenClaims(t, token.IDToken)

		assert.Equal(t, bundle.OAuthBaseURL, claims["iss"])
		assert.Equal(t, "user-123", claims["sub"])
		assert.Equal(t, "test-client", claims["aud"])
		assert.Equal(t, "Test User", claims["name"])
		assert.Equal(t, "test@example.com", claims["email"])
		assert.NotNil(t, claims["auth_time"])
	})
}

// ============================================================================
// Test: Token Revocation
// ============================================================================

func TestOAuth_TokenRevocation(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("revoke access token", func(t *testing.T) {
		token := getClientCredentialsToken(t, bundle, "api:read")

		// Revoke the token
		data := url.Values{}
		data.Set("token", token.AccessToken)
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/revoke", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Token should now be invalid for userinfo
		req, err := http.NewRequest("GET", bundle.OAuthBaseURL+"/userinfo", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		userInfoResp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer userInfoResp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, userInfoResp.StatusCode)
	})

	t.Run("revoke refresh token", func(t *testing.T) {
		token := getPasswordToken(t, bundle, "testuser", "testpass", "openid")
		require.NotEmpty(t, token.RefreshToken)

		// Revoke the refresh token
		data := url.Values{}
		data.Set("token", token.RefreshToken)
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/revoke", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Refresh token should now be invalid
		refreshData := url.Values{}
		refreshData.Set("grant_type", "refresh_token")
		refreshData.Set("refresh_token", token.RefreshToken)
		refreshData.Set("client_id", "test-client")
		refreshData.Set("client_secret", "test-secret")

		refreshResp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(refreshData.Encode()))
		require.NoError(t, err)
		defer refreshResp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, refreshResp.StatusCode)
	})

	t.Run("revoking unknown token returns 200", func(t *testing.T) {
		// Per RFC 7009, revoke should always return 200
		data := url.Values{}
		data.Set("token", "nonexistent-token")
		data.Set("client_id", "test-client")
		data.Set("client_secret", "test-secret")

		resp, err := http.Post(bundle.OAuthBaseURL+"/revoke", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// ============================================================================
// Test: Implicit Flow
// ============================================================================

func TestOAuth_ImplicitFlow(t *testing.T) {
	bundle := setupOAuthServer(t)

	t.Run("response_type=token returns token in fragment", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		authURL := fmt.Sprintf("%s/authorize?client_id=test-client&response_type=token&redirect_uri=%s&state=implicit-state&scope=api:read",
			bundle.OAuthBaseURL,
			url.QueryEscape("http://localhost/callback"))

		resp, err := client.Get(authURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusFound, resp.StatusCode)

		location := resp.Header.Get("Location")
		redirectURL, err := url.Parse(location)
		require.NoError(t, err)

		// Token should be in the fragment
		fragment, err := url.ParseQuery(redirectURL.Fragment)
		require.NoError(t, err)

		assert.NotEmpty(t, fragment.Get("access_token"))
		assert.Equal(t, "Bearer", fragment.Get("token_type"))
		assert.Equal(t, "implicit-state", fragment.Get("state"))
		assert.NotEmpty(t, fragment.Get("expires_in"))
	})
}

// ============================================================================
// Test: Multiple OAuth Providers
// ============================================================================

func TestOAuth_MultipleProviders(t *testing.T) {
	httpPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		srv.Stop()
	})

	waitForReady(t, srv.ManagementPort())

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))
	baseURL := fmt.Sprintf("http://localhost:%d", httpPort)

	// Create first OAuth provider at /oauth1
	_, err = client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "oauth-provider-1",
		Name:    "OAuth Provider 1",
		Enabled: boolPtr(true),
		Type:    mock.TypeOAuth,
		OAuth: &mock.OAuthSpec{
			Issuer:      baseURL + "/oauth1",
			TokenExpiry: "1h",
			Clients: []mock.OAuthClient{
				{
					ClientID:     "client1",
					ClientSecret: "secret1",
					RedirectURIs: []string{"http://localhost/callback"},
					GrantTypes:   []string{"client_credentials"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Create second OAuth provider at /oauth2
	_, err = client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "oauth-provider-2",
		Name:    "OAuth Provider 2",
		Enabled: boolPtr(true),
		Type:    mock.TypeOAuth,
		OAuth: &mock.OAuthSpec{
			Issuer:      baseURL + "/oauth2",
			TokenExpiry: "2h",
			Clients: []mock.OAuthClient{
				{
					ClientID:     "client2",
					ClientSecret: "secret2",
					RedirectURIs: []string{"http://localhost/callback"},
					GrantTypes:   []string{"client_credentials"},
				},
			},
		},
	})
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // Allow mock registration

	t.Run("both providers are accessible", func(t *testing.T) {
		// Provider 1
		resp1, err := http.Get(baseURL + "/oauth1/.well-known/openid-configuration")
		require.NoError(t, err)
		defer resp1.Body.Close()
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		var config1 openIDConfiguration
		json.NewDecoder(resp1.Body).Decode(&config1)
		assert.Equal(t, baseURL+"/oauth1", config1.Issuer)

		// Provider 2
		resp2, err := http.Get(baseURL + "/oauth2/.well-known/openid-configuration")
		require.NoError(t, err)
		defer resp2.Body.Close()
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		var config2 openIDConfiguration
		json.NewDecoder(resp2.Body).Decode(&config2)
		assert.Equal(t, baseURL+"/oauth2", config2.Issuer)
	})

	t.Run("each provider has its own clients", func(t *testing.T) {
		// Client1 should work with OAuth1
		data1 := url.Values{}
		data1.Set("grant_type", "client_credentials")
		data1.Set("client_id", "client1")
		data1.Set("client_secret", "secret1")

		resp1, err := http.Post(baseURL+"/oauth1/token", "application/x-www-form-urlencoded", strings.NewReader(data1.Encode()))
		require.NoError(t, err)
		defer resp1.Body.Close()
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		// Client1 should NOT work with OAuth2
		resp1on2, err := http.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(data1.Encode()))
		require.NoError(t, err)
		defer resp1on2.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp1on2.StatusCode)

		// Client2 should work with OAuth2
		data2 := url.Values{}
		data2.Set("grant_type", "client_credentials")
		data2.Set("client_id", "client2")
		data2.Set("client_secret", "secret2")

		resp2, err := http.Post(baseURL+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(data2.Encode()))
		require.NoError(t, err)
		defer resp2.Body.Close()
		require.Equal(t, http.StatusOK, resp2.StatusCode)
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

// getClientCredentialsToken gets an access token using client credentials grant.
func getClientCredentialsToken(t *testing.T, bundle *oauthTestBundle, scope string) tokenResponse {
	t.Helper()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", scope)

	req, err := http.NewRequest("POST", bundle.OAuthBaseURL+"/token", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var token tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	require.NoError(t, err)

	return token
}

// getPasswordToken gets an access token using password grant.
func getPasswordToken(t *testing.T, bundle *oauthTestBundle, username, password, scope string) tokenResponse {
	t.Helper()

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", username)
	data.Set("password", password)
	data.Set("scope", scope)
	data.Set("client_id", "test-client")
	data.Set("client_secret", "test-secret")

	resp, err := http.Post(bundle.OAuthBaseURL+"/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var token tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	require.NoError(t, err)

	return token
}

// parseTokenClaims parses JWT claims from a token string.
func parseTokenClaims(t *testing.T, tokenString string) map[string]interface{} {
	t.Helper()

	parts := strings.Split(tokenString, ".")
	require.Len(t, parts, 3, "JWT should have 3 parts")

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var claims map[string]interface{}
	err = json.Unmarshal(claimsJSON, &claims)
	require.NoError(t, err)

	return claims
}
