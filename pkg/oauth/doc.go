// Package oauth provides a mock OAuth 2.0 and OpenID Connect provider
// for testing authentication and authorization flows.
//
// The package implements the following OAuth 2.0 grant types:
//   - Authorization Code
//   - Client Credentials
//   - Refresh Token
//   - Resource Owner Password Credentials
//
// For OpenID Connect, it provides:
//   - ID Token generation with standard claims
//   - UserInfo endpoint
//   - Discovery document (/.well-known/openid-configuration)
//   - JWKS endpoint (/.well-known/jwks.json)
//
// # Basic Usage
//
// Create a provider with configuration:
//
//	config := &oauth.OAuthConfig{
//	    ID:          "test-provider",
//	    Issuer:      "https://mock.example.com",
//	    TokenExpiry: "1h",
//	    Clients: []oauth.ClientConfig{
//	        {
//	            ClientID:     "my-client",
//	            ClientSecret: "my-secret",
//	            RedirectURIs: []string{"https://app.example.com/callback"},
//	            GrantTypes:   []string{"authorization_code", "refresh_token"},
//	        },
//	    },
//	    Users: []oauth.UserConfig{
//	        {
//	            Username: "testuser",
//	            Password: "testpass",
//	            Claims: map[string]interface{}{
//	                "sub":   "user-123",
//	                "email": "test@example.com",
//	                "name":  "Test User",
//	            },
//	        },
//	    },
//	}
//
//	provider, err := oauth.NewProvider(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # HTTP Handlers
//
// Register the OAuth endpoints with your router:
//
//	handler := oauth.NewHandler(provider)
//
//	mux.HandleFunc("GET /authorize", handler.HandleAuthorize)
//	mux.HandleFunc("POST /token", handler.HandleToken)
//	mux.HandleFunc("GET /userinfo", handler.HandleUserInfo)
//	mux.HandleFunc("GET /.well-known/jwks.json", handler.HandleJWKS)
//	mux.HandleFunc("GET /.well-known/openid-configuration", handler.HandleOpenIDConfig)
//	mux.HandleFunc("POST /introspect", handler.HandleIntrospect)
//	mux.HandleFunc("POST /revoke", handler.HandleRevoke)
//
// # Token Generation
//
// Generate tokens programmatically:
//
//	// Access token with custom claims
//	token, err := provider.GenerateToken(map[string]interface{}{
//	    "sub":   "user-123",
//	    "scope": "openid profile email",
//	})
//
//	// ID token for OIDC
//	idToken, err := provider.GenerateIDToken(map[string]interface{}{
//	    "sub":   "user-123",
//	    "email": "test@example.com",
//	    "nonce": "abc123",
//	})
//
// # Token Validation
//
// Validate incoming tokens:
//
//	claims, err := provider.ValidateToken(tokenString)
//	if err != nil {
//	    // Token is invalid
//	}
//	// Use claims for authorization decisions
//
// # JWKS
//
// Get the JSON Web Key Set for token verification:
//
//	jwks := provider.GetJWKS()
//	// Returns the public key in JWK format
package oauth
