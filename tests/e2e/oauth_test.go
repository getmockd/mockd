package e2e_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthProtocolIntegration(t *testing.T) {
	port := getFreePort(t)
	controlPort := getFreePort(t)
	adminPort := getFreePort(t)

	cfg := &config.ServerConfiguration{
		HTTPPort:       port,
		ManagementPort: controlPort,
	}

	server := engine.NewServer(cfg)
	go func() {
		_ = server.Start()
	}()
	defer server.Stop()

	adminURL := "http://localhost:" + strconv.Itoa(adminPort)
	engineURL := "http://localhost:" + strconv.Itoa(controlPort)
	mockTargetURL := "http://localhost:" + strconv.Itoa(port)

	engClient := engineclient.New(engineURL)

	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(engineURL),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(t.TempDir()),
	)
	adminAPI.SetLocalEngine(engClient)

	go func() {
		_ = adminAPI.Start()
	}()
	defer adminAPI.Stop()

	waitForServer(t, adminURL+"/health")
	waitForServer(t, engineURL+"/health")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Client that doesn't follow redirects to test OAuth authorize endpoint
	noRedirectClient := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	apiReq := func(method, path string, body []byte) *http.Response {
		urlStr := adminURL + path
		req, _ := http.NewRequest(method, urlStr, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)

		if resp.StatusCode >= 400 {
			b, _ := ioutil.ReadAll(resp.Body)
			t.Logf("API Error %s %s -> %d : %s", method, urlStr, resp.StatusCode, string(b))
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}

		return resp
	}

	engineReq := func(method, path string, body []byte) (*http.Response, string) {
		req, _ := http.NewRequest(method, mockTargetURL+path, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)
		b, _ := ioutil.ReadAll(resp.Body)
		return resp, string(b)
	}

	engineForm := func(path, formBody string) (*http.Response, string) {
		req, _ := http.NewRequest("POST", mockTargetURL+path, strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, _ := client.Do(req)
		b, _ := ioutil.ReadAll(resp.Body)
		return resp, string(b)
	}

	// Setup: Create an OAuth Mock
	mockReqBody := []byte(`{
		"type": "oauth",
		"name": "Test OAuth Provider",
		"oauth": {
		  "issuer": "` + mockTargetURL + `/oauth",
		  "tokenExpiry": "1h",
		  "defaultScopes": ["openid", "profile", "email"],
		  "clients": [
			{
			  "clientId": "test-app",
			  "clientSecret": "test-secret",
			  "redirectUris": ["http://localhost:3000/callback"],
			  "grantTypes": ["authorization_code", "client_credentials", "password", "refresh_token"]
			},
			{
			  "clientId": "public-spa",
			  "clientSecret": "",
			  "redirectUris": ["http://localhost:3000/callback"],
			  "grantTypes": ["authorization_code"]
			}
		  ],
		  "users": [
			{
			  "username": "testuser",
			  "password": "testpass",
			  "claims": {
				"sub": "user-123",
				"name": "Test User",
				"email": "test@example.com"
			  }
			}
		  ]
		}
	}`)

	resp := apiReq("POST", "/mocks", mockReqBody)
	require.Equal(t, 201, resp.StatusCode, "Failed to create OAuth mock")

	t.Run("Create Extra OAuth Mock", func(t *testing.T) {
		resp := apiReq("POST", "/mocks", []byte(`{
			"type": "oauth",
			"name": "OAuth Verify",
			"oauth": {
			  "issuer": "`+mockTargetURL+`/oauth-verify",
			  "clients": [{"clientId": "verify", "clientSecret": "s"}]
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)

		var mock struct{ ID string `json:"id"` }
		json.NewDecoder(resp.Body).Decode(&mock)

		resp = apiReq("DELETE", "/mocks/"+mock.ID, nil)
		require.Equal(t, 204, resp.StatusCode)
	})

	t.Run("OIDC discovery endpoint returns 200", func(t *testing.T) {
		resp, body := engineReq("GET", "/oauth/.well-known/openid-configuration", nil)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, body, "token_endpoint")
		assert.Contains(t, body, "introspection_endpoint")
		assert.Contains(t, body, "userinfo_endpoint")
	})

	t.Run("JWKS endpoint returns 200", func(t *testing.T) {
		resp, body := engineReq("GET", "/oauth/.well-known/jwks.json", nil)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, body, "keys")
	})

	t.Run("Client credentials grant", func(t *testing.T) {
		resp, body := engineForm("/oauth/token", "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, body, "access_token")

		// Verify JWT structure
		var tokenResp struct{ AccessToken string `json:"access_token"` }
		json.Unmarshal([]byte(body), &tokenResp)

		parts := strings.Split(tokenResp.AccessToken, ".")
		assert.Len(t, parts, 3, "JWT should have 3 segments")
	})

	t.Run("Password grant", func(t *testing.T) {
		resp, body := engineForm("/oauth/token", "grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid+profile")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, body, "access_token")
	})

	t.Run("Invalid credentials rejected", func(t *testing.T) {
		resp, _ := engineForm("/oauth/token", "grant_type=client_credentials&client_id=wrong&client_secret=wrong")
		assert.True(t, resp.StatusCode == 400 || resp.StatusCode == 401)
	})

	t.Run("Authorization code exchange", func(t *testing.T) {
		// Step 1: Authorize endpoint returns redirect with code
		authURL := mockTargetURL + "/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&state=xyz123"
		req, _ := http.NewRequest("GET", authURL, nil)
		resp, err := noRedirectClient.Do(req)
		require.NoError(t, err)

		assert.Equal(t, 302, resp.StatusCode)
		location := resp.Header.Get("Location")
		assert.Contains(t, location, "code=")
		assert.Contains(t, location, "state=xyz123")

		// Extract code
		parsedURL, err := url.Parse(location)
		require.NoError(t, err)
		code := parsedURL.Query().Get("code")
		require.NotEmpty(t, code)

		// Step 2: Exchange code for tokens
		resp2, body2 := engineForm("/oauth/token", "grant_type=authorization_code&code="+code+"&redirect_uri=http://localhost:3000/callback&client_id=test-app&client_secret=test-secret")
		assert.Equal(t, 200, resp2.StatusCode)
		assert.Contains(t, body2, "access_token")
		assert.Contains(t, body2, "refresh_token")
		assert.Contains(t, body2, "id_token")
	})

	t.Run("PKCE S256 flow", func(t *testing.T) {
		codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		codeChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

		authURL := mockTargetURL + "/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&code_challenge=" + codeChallenge + "&code_challenge_method=S256"
		req, _ := http.NewRequest("GET", authURL, nil)
		resp, err := noRedirectClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, 302, resp.StatusCode)

		parsedURL, _ := url.Parse(resp.Header.Get("Location"))
		code := parsedURL.Query().Get("code")
		require.NotEmpty(t, code)

		resp2, body2 := engineForm("/oauth/token", "grant_type=authorization_code&code="+code+"&redirect_uri=http://localhost:3000/callback&client_id=test-app&client_secret=test-secret&code_verifier="+codeVerifier)
		assert.Equal(t, 200, resp2.StatusCode)
		assert.Contains(t, body2, "access_token")
	})

	t.Run("PKCE wrong verifier rejected", func(t *testing.T) {
		codeChallenge := "6JXavkWXGUoS2woo4y0DCvHIgLNXN2Nu9VYGej4qD3w"

		authURL := mockTargetURL + "/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&code_challenge=" + codeChallenge + "&code_challenge_method=S256"
		req, _ := http.NewRequest("GET", authURL, nil)
		resp, err := noRedirectClient.Do(req)
		require.NoError(t, err)

		parsedURL, _ := url.Parse(resp.Header.Get("Location"))
		code := parsedURL.Query().Get("code")

		resp2, body2 := engineForm("/oauth/token", "grant_type=authorization_code&code="+code+"&redirect_uri=http://localhost:3000/callback&client_id=test-app&client_secret=test-secret&code_verifier=wrong-verifier-completely-different")
		assert.Equal(t, 400, resp2.StatusCode)
		assert.Contains(t, body2, "invalid_grant")
	})

	t.Run("Refresh token grant", func(t *testing.T) {
		_, body := engineForm("/oauth/token", "grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid")

		var tokenResp struct{ RefreshToken string `json:"refresh_token"` }
		json.Unmarshal([]byte(body), &tokenResp)
		require.NotEmpty(t, tokenResp.RefreshToken)

		resp2, body2 := engineForm("/oauth/token", "grant_type=refresh_token&refresh_token="+tokenResp.RefreshToken+"&client_id=test-app&client_secret=test-secret")
		assert.Equal(t, 200, resp2.StatusCode)
		assert.Contains(t, body2, "access_token")
	})

	t.Run("Token Introspection returns active=true", func(t *testing.T) {
		_, body := engineForm("/oauth/token", "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid")

		var tokenResp struct{ AccessToken string `json:"access_token"` }
		json.Unmarshal([]byte(body), &tokenResp)

		resp2, body2 := engineForm("/oauth/introspect", "token="+tokenResp.AccessToken+"&client_id=test-app&client_secret=test-secret")
		assert.Equal(t, 200, resp2.StatusCode)

		var introResp struct{ Active bool `json:"active"` }
		json.Unmarshal([]byte(body2), &introResp)
		assert.True(t, introResp.Active)
	})

	t.Run("Revoke token then introspect returns active=false", func(t *testing.T) {
		_, body := engineForm("/oauth/token", "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid")

		var tokenResp struct{ AccessToken string `json:"access_token"` }
		json.Unmarshal([]byte(body), &tokenResp)

		resp2, _ := engineForm("/oauth/revoke", "token="+tokenResp.AccessToken+"&client_id=test-app&client_secret=test-secret")
		assert.Equal(t, 200, resp2.StatusCode)

		_, body3 := engineForm("/oauth/introspect", "token="+tokenResp.AccessToken+"&client_id=test-app&client_secret=test-secret")
		var introResp struct{ Active bool `json:"active"` }
		json.Unmarshal([]byte(body3), &introResp)
		assert.False(t, introResp.Active)
	})

	t.Run("Userinfo endpoint returns user claims", func(t *testing.T) {
		_, body := engineForm("/oauth/token", "grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid+profile+email")

		var tokenResp struct{ AccessToken string `json:"access_token"` }
		json.Unmarshal([]byte(body), &tokenResp)

		req, _ := http.NewRequest("GET", mockTargetURL+"/oauth/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
		resp2, _ := client.Do(req)
		
		assert.Equal(t, 200, resp2.StatusCode)
		body2, _ := ioutil.ReadAll(resp2.Body)
		assert.Contains(t, string(body2), "user-123")
		assert.Contains(t, string(body2), "test@example.com")
	})
}
