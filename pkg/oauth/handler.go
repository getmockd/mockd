package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// hasScope checks if a specific scope is present in a space-separated scope string.
// This avoids false positives from substring matching (e.g., "openid" in "custom_openid_scope").
func hasScope(scopeString, targetScope string) bool {
	scopes := strings.Fields(scopeString)
	for _, s := range scopes {
		if s == targetScope {
			return true
		}
	}
	return false
}

// Handler provides OAuth endpoint handlers
type Handler struct {
	provider *Provider
}

// NewHandler creates OAuth HTTP handlers
func NewHandler(provider *Provider) *Handler {
	return &Handler{provider: provider}
}

// getDefaultUserID returns the user ID from the first configured user, or a default mock user ID.
func (h *Handler) getDefaultUserID() string {
	if len(h.provider.config.Users) > 0 {
		if sub, ok := h.provider.config.Users[0].Claims["sub"].(string); ok {
			return sub
		}
		return h.provider.config.Users[0].Username
	}
	return "mock-user"
}

// generateIDTokenForUser generates an ID token for the given user with standard claims.
// If user is nil, it will look up the user by userID.
func (h *Handler) generateIDTokenForUser(userID, clientID, nonce string, user *UserConfig) string {
	if user == nil {
		user = h.provider.GetUserByID(userID)
	}

	claims := map[string]interface{}{
		"sub": userID,
		"aud": clientID,
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if user != nil {
		for k, v := range user.Claims {
			if k != "sub" { // sub already set
				claims[k] = v
			}
		}
	}

	idToken, err := h.provider.GenerateIDToken(claims)
	if err != nil {
		return ""
	}
	return idToken
}

// validateScopes checks that all requested scopes are in the allowed scopes list.
// Returns an error message if any scope is invalid, or empty string if all valid.
func (h *Handler) validateScopes(requestedScope string) string {
	if requestedScope == "" {
		return ""
	}
	allowed := make(map[string]bool)
	for _, s := range h.provider.config.DefaultScopes {
		allowed[s] = true
	}
	for _, s := range strings.Fields(requestedScope) {
		if !allowed[s] {
			return fmt.Sprintf("scope %q is not supported", s)
		}
	}
	return ""
}

// verifyCodeChallenge verifies the PKCE code_verifier against the stored code_challenge.
// Supports "plain" (direct comparison) and "S256" (SHA-256 hash comparison) methods.
func (h *Handler) verifyCodeChallenge(challenge, method, verifier string) bool {
	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
	case "plain", "":
		return subtle.ConstantTimeCompare([]byte(verifier), []byte(challenge)) == 1
	default:
		return false
	}
}

// HandleAuthorize handles GET /authorize
func (h *Handler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	// Parse parameters (from query string or form body)
	var params url.Values
	if r.Method == http.MethodGet {
		params = r.URL.Query()
	} else {
		if err := r.ParseForm(); err != nil {
			h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "failed to parse form")
			return
		}
		params = r.Form
	}

	clientID := params.Get("client_id")
	redirectURI := params.Get("redirect_uri")
	responseType := params.Get("response_type")
	scope := params.Get("scope")
	state := params.Get("state")
	nonce := params.Get("nonce")
	codeChallenge := params.Get("code_challenge")
	codeChallengeMethod := params.Get("code_challenge_method")

	// Validate scopes against allowed scopes
	if msg := h.validateScopes(scope); msg != "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidScope, msg)
		return
	}

	// Validate required parameters
	if clientID == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "client_id is required")
		return
	}
	if redirectURI == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "redirect_uri is required")
		return
	}
	if responseType == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "response_type is required")
		return
	}

	// Validate client
	client := h.provider.GetClient(clientID)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "unknown client")
		return
	}

	// Validate redirect URI
	if !h.provider.IsValidRedirectURI(client, redirectURI) {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "invalid redirect_uri")
		return
	}

	// Validate response type
	if responseType != ResponseTypeCode && responseType != ResponseTypeToken {
		h.redirectError(w, r, redirectURI, state, ErrUnsupportedResponseType, "unsupported response_type")
		return
	}

	// For authorization_code flow
	if responseType == ResponseTypeCode {
		if !h.provider.ClientSupportsGrantType(client, GrantTypeAuthorizationCode) {
			h.redirectError(w, r, redirectURI, state, ErrUnauthorizedClient, "client does not support authorization_code grant")
			return
		}

		// In a real scenario, you'd show a login page here
		// For mock purposes, we auto-approve with the first user
		userID := h.getDefaultUserID()

		// Generate authorization code
		code, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// Validate PKCE code_challenge_method if provided
		if codeChallenge != "" && codeChallengeMethod != "" && codeChallengeMethod != "plain" && codeChallengeMethod != "S256" {
			h.redirectError(w, r, redirectURI, state, ErrInvalidRequest, "unsupported code_challenge_method; must be plain or S256")
			return
		}
		// Default to "plain" if code_challenge is provided without method
		if codeChallenge != "" && codeChallengeMethod == "" {
			codeChallengeMethod = "plain"
		}

		authCode := &AuthorizationCode{
			Code:                code,
			ClientID:            clientID,
			RedirectURI:         redirectURI,
			Scope:               scope,
			UserID:              userID,
			Nonce:               nonce,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: codeChallengeMethod,
			ExpiresAt:           time.Now().Add(10 * time.Minute),
		}
		h.provider.StoreAuthorizationCode(authCode)

		// Redirect with code
		redirectURL, _ := url.Parse(redirectURI)
		q := redirectURL.Query()
		q.Set("code", code)
		if state != "" {
			q.Set("state", state)
		}
		redirectURL.RawQuery = q.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return
	}

	// Implicit flow (response_type=token)
	if responseType == ResponseTypeToken {
		// If no scope was requested, populate with default scopes
		if scope == "" && len(h.provider.config.DefaultScopes) > 0 {
			scope = strings.Join(h.provider.config.DefaultScopes, " ")
		}

		userID := h.getDefaultUserID()

		// Generate access token
		accessToken, err := h.provider.GenerateToken(map[string]interface{}{
			"sub":   userID,
			"scope": scope,
		})
		if err != nil {
			h.redirectError(w, r, redirectURI, state, ErrServerError, "failed to generate token")
			return
		}

		// Redirect with token in fragment
		redirectURL, _ := url.Parse(redirectURI)
		fragment := url.Values{}
		fragment.Set("access_token", accessToken)
		fragment.Set("token_type", "Bearer")
		fragment.Set("expires_in", strconv.Itoa(int(h.provider.tokenExpiry.Seconds())))
		if state != "" {
			fragment.Set("state", state)
		}
		if scope != "" {
			fragment.Set("scope", scope)
		}
		redirectURL.Fragment = fragment.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return
	}
}

// HandleToken handles POST /token
func (h *Handler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "failed to parse form")
		return
	}

	grantType := r.FormValue("grant_type")

	// Get client credentials (from Authorization header or form body)
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	switch grantType {
	case GrantTypeAuthorizationCode:
		h.handleAuthorizationCodeGrant(w, r, clientID, clientSecret)
	case GrantTypeClientCredentials:
		h.handleClientCredentialsGrant(w, r, clientID, clientSecret)
	case GrantTypeRefreshToken:
		h.handleRefreshTokenGrant(w, r, clientID, clientSecret)
	case GrantTypePassword:
		h.handlePasswordGrant(w, r, clientID, clientSecret)
	default:
		h.errorResponse(w, http.StatusBadRequest, ErrUnsupportedGrantType, "unsupported grant_type")
	}
}

func (h *Handler) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	if code == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "code is required")
		return
	}
	if redirectURI == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "redirect_uri is required")
		return
	}

	// Validate client — for PKCE flows, client_secret may be omitted (public clients)
	client := h.provider.GetClient(clientID)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "unknown client")
		return
	}
	// If client has a secret configured, require and validate it (confidential client)
	if client.ClientSecret != "" {
		if clientSecret == "" {
			h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "client_secret is required for confidential clients")
			return
		}
		client = h.provider.ValidateClient(clientID, clientSecret)
		if client == nil {
			h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
			return
		}
	}

	if !h.provider.ClientSupportsGrantType(client, GrantTypeAuthorizationCode) {
		h.errorResponse(w, http.StatusBadRequest, ErrUnauthorizedClient, "client does not support authorization_code grant")
		return
	}

	// Exchange authorization code
	authCode, err := h.provider.ExchangeAuthorizationCode(code, clientID, redirectURI)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidGrant, err.Error())
		return
	}

	// Verify PKCE code_verifier if code_challenge was provided during authorization
	if authCode.CodeChallenge != "" {
		if codeVerifier == "" {
			h.errorResponse(w, http.StatusBadRequest, ErrInvalidGrant, "code_verifier is required for PKCE")
			return
		}
		if !h.verifyCodeChallenge(authCode.CodeChallenge, authCode.CodeChallengeMethod, codeVerifier) {
			h.errorResponse(w, http.StatusBadRequest, ErrInvalidGrant, "code_verifier does not match code_challenge")
			return
		}
	}

	// If no scope was present in the authorization code, populate with default scopes
	scope := authCode.Scope
	if scope == "" && len(h.provider.config.DefaultScopes) > 0 {
		scope = strings.Join(h.provider.config.DefaultScopes, " ")
	}

	// Generate tokens
	tokenClaims := map[string]interface{}{
		"sub":       authCode.UserID,
		"client_id": clientID,
		"scope":     scope,
	}

	accessToken, err := h.provider.GenerateToken(tokenClaims)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, ErrServerError, "failed to generate access token")
		return
	}

	response := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.provider.tokenExpiry.Seconds()),
		Scope:       scope,
	}

	// Generate refresh token if client supports it
	if h.provider.ClientSupportsGrantType(client, GrantTypeRefreshToken) {
		refreshToken, err := h.provider.GenerateRefreshToken()
		if err != nil {
			h.errorResponse(w, http.StatusInternalServerError, ErrServerError, "failed to generate refresh token")
			return
		}
		h.provider.StoreRefreshToken(&RefreshTokenData{
			Token:     refreshToken,
			ClientID:  clientID,
			UserID:    authCode.UserID,
			Scope:     scope,
			ExpiresAt: time.Now().Add(h.provider.refreshExpiry),
		})
		response.RefreshToken = refreshToken
	}

	// Generate ID token if openid scope is requested
	if hasScope(scope, "openid") {
		if idToken := h.generateIDTokenForUser(authCode.UserID, clientID, authCode.Nonce, nil); idToken != "" {
			response.IDToken = idToken
		}
	}

	h.jsonResponse(w, http.StatusOK, response)
}

func (h *Handler) handleClientCredentialsGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	scope := r.FormValue("scope")

	// Validate scopes against allowed scopes
	if msg := h.validateScopes(scope); msg != "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidScope, msg)
		return
	}

	// If no scope was requested, populate with default scopes
	if scope == "" && len(h.provider.config.DefaultScopes) > 0 {
		scope = strings.Join(h.provider.config.DefaultScopes, " ")
	}

	// Validate client
	client := h.provider.ValidateClient(clientID, clientSecret)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
	}

	if !h.provider.ClientSupportsGrantType(client, GrantTypeClientCredentials) {
		h.errorResponse(w, http.StatusBadRequest, ErrUnauthorizedClient, "client does not support client_credentials grant")
		return
	}

	// Generate access token
	tokenClaims := map[string]interface{}{
		"sub":       clientID,
		"client_id": clientID,
		"scope":     scope,
	}

	accessToken, err := h.provider.GenerateToken(tokenClaims)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, ErrServerError, "failed to generate access token")
		return
	}

	response := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.provider.tokenExpiry.Seconds()),
		Scope:       scope,
	}

	h.jsonResponse(w, http.StatusOK, response)
}

func (h *Handler) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	refreshToken := r.FormValue("refresh_token")
	scope := r.FormValue("scope")

	// Validate scopes against allowed scopes
	if msg := h.validateScopes(scope); msg != "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidScope, msg)
		return
	}

	if refreshToken == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "refresh_token is required")
		return
	}

	// Validate client
	client := h.provider.ValidateClient(clientID, clientSecret)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
	}

	if !h.provider.ClientSupportsGrantType(client, GrantTypeRefreshToken) {
		h.errorResponse(w, http.StatusBadRequest, ErrUnauthorizedClient, "client does not support refresh_token grant")
		return
	}

	// Validate refresh token
	refreshData, err := h.provider.ValidateRefreshToken(refreshToken, clientID)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidGrant, err.Error())
		return
	}

	// Use original scope if not specified
	if scope == "" {
		scope = refreshData.Scope
	}

	// If scope is still empty after refresh data fallback, use default scopes
	if scope == "" && len(h.provider.config.DefaultScopes) > 0 {
		scope = strings.Join(h.provider.config.DefaultScopes, " ")
	}

	// Generate new access token
	tokenClaims := map[string]interface{}{
		"sub":       refreshData.UserID,
		"client_id": clientID,
		"scope":     scope,
	}

	accessToken, err := h.provider.GenerateToken(tokenClaims)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, ErrServerError, "failed to generate access token")
		return
	}

	response := &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(h.provider.tokenExpiry.Seconds()),
		RefreshToken: refreshToken, // Return same refresh token
		Scope:        scope,
	}

	// Generate new ID token if openid scope is requested
	if hasScope(scope, "openid") {
		if idToken := h.generateIDTokenForUser(refreshData.UserID, clientID, "", nil); idToken != "" {
			response.IDToken = idToken
		}
	}

	h.jsonResponse(w, http.StatusOK, response)
}

func (h *Handler) handlePasswordGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	scope := r.FormValue("scope")

	// Validate scopes against allowed scopes
	if msg := h.validateScopes(scope); msg != "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidScope, msg)
		return
	}

	// If no scope was requested, populate with default scopes
	if scope == "" && len(h.provider.config.DefaultScopes) > 0 {
		scope = strings.Join(h.provider.config.DefaultScopes, " ")
	}

	if username == "" || password == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "username and password are required")
		return
	}

	// Validate client
	client := h.provider.ValidateClient(clientID, clientSecret)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
	}

	if !h.provider.ClientSupportsGrantType(client, GrantTypePassword) {
		h.errorResponse(w, http.StatusBadRequest, ErrUnauthorizedClient, "client does not support password grant")
		return
	}

	// Validate user
	user := h.provider.ValidateUser(username, password)
	if user == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidGrant, "invalid user credentials")
		return
	}

	// Get user's subject ID
	userID := username
	if sub, ok := user.Claims["sub"].(string); ok {
		userID = sub
	}

	// Generate access token
	tokenClaims := map[string]interface{}{
		"sub":       userID,
		"client_id": clientID,
		"scope":     scope,
	}

	accessToken, err := h.provider.GenerateToken(tokenClaims)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, ErrServerError, "failed to generate access token")
		return
	}

	response := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.provider.tokenExpiry.Seconds()),
		Scope:       scope,
	}

	// Generate refresh token if client supports it
	if h.provider.ClientSupportsGrantType(client, GrantTypeRefreshToken) {
		refreshToken, err := h.provider.GenerateRefreshToken()
		if err != nil {
			h.errorResponse(w, http.StatusInternalServerError, ErrServerError, "failed to generate refresh token")
			return
		}
		h.provider.StoreRefreshToken(&RefreshTokenData{
			Token:     refreshToken,
			ClientID:  clientID,
			UserID:    userID,
			Scope:     scope,
			ExpiresAt: time.Now().Add(h.provider.refreshExpiry),
		})
		response.RefreshToken = refreshToken
	}

	// Generate ID token if openid scope is requested
	if hasScope(scope, "openid") {
		if idToken := h.generateIDTokenForUser(userID, clientID, "", user); idToken != "" {
			response.IDToken = idToken
		}
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// HandleUserInfo handles GET /userinfo
func (h *Handler) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	// Extract bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidRequest, "missing authorization header")
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidRequest, "invalid authorization header")
		return
	}

	tokenString := parts[1]

	// Validate token
	claims, err := h.provider.ValidateToken(tokenString)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer error=\"invalid_token\"")
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidGrant, err.Error())
		return
	}

	// Get user info
	userInfo := map[string]interface{}{
		"sub": claims["sub"],
	}

	// Add user claims if we can find the user
	if sub, ok := claims["sub"].(string); ok {
		user := h.provider.GetUserByID(sub)
		if user != nil {
			for k, v := range user.Claims {
				userInfo[k] = v
			}
		}
	}

	h.jsonResponse(w, http.StatusOK, userInfo)
}

// HandleJWKS handles GET /.well-known/jwks.json
func (h *Handler) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	jwks := h.provider.GetJWKS()
	h.jsonResponse(w, http.StatusOK, jwks)
}

// HandleOpenIDConfig handles GET /.well-known/openid-configuration
func (h *Handler) HandleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	issuer := h.provider.config.Issuer

	config := &OpenIDConfiguration{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + "/authorize",
		TokenEndpoint:                     issuer + "/token",
		UserInfoEndpoint:                  issuer + "/userinfo",
		JwksURI:                           issuer + "/.well-known/jwks.json",
		RevocationEndpoint:                issuer + "/revoke",
		IntrospectionEndpoint:             issuer + "/introspect",
		ResponseTypesSupported:            []string{"code", "token"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   h.provider.config.DefaultScopes,
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post"},
		ClaimsSupported:                   []string{"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce", "email", "email_verified", "name", "given_name", "family_name", "picture"},
		GrantTypesSupported:               []string{"authorization_code", "client_credentials", "refresh_token", "password"},
		CodeChallengeMethodsSupported:     []string{"S256", "plain"},
	}

	h.jsonResponse(w, http.StatusOK, config)
}

// HandleIntrospect handles POST /introspect (RFC 7662)
// Token introspection allows resource servers to query the authorization server
// about the current state of an access token.
func (h *Handler) HandleIntrospect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "failed to parse form")
		return
	}

	token := r.FormValue("token")
	if token == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "token is required")
		return
	}

	// Get client credentials (required for introspection per RFC 7662)
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	// Validate client credentials
	if clientID == "" {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "client authentication required")
		return
	}

	client := h.provider.ValidateClient(clientID, clientSecret)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
	}

	// Try to validate the token
	claims, err := h.provider.ValidateToken(token)
	if err != nil {
		// Token is invalid or expired - return active: false per RFC 7662
		h.jsonResponse(w, http.StatusOK, &IntrospectionResponse{
			Active: false,
		})
		return
	}

	// Build introspection response from claims
	response := &IntrospectionResponse{
		Active: true,
	}

	// Extract standard claims
	if iss, ok := claims["iss"].(string); ok {
		response.Issuer = iss
	}
	if sub, ok := claims["sub"].(string); ok {
		response.Subject = sub
	}
	if aud, ok := claims["aud"]; ok {
		switch v := aud.(type) {
		case string:
			response.Audience = v
		case []interface{}:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					response.Audience = s
				}
			}
		}
	}
	if exp, ok := claims["exp"].(float64); ok {
		response.ExpiresAt = int64(exp)
	}
	if iat, ok := claims["iat"].(float64); ok {
		response.IssuedAt = int64(iat)
	}
	if nbf, ok := claims["nbf"].(float64); ok {
		response.NotBefore = int64(nbf)
	}
	if jti, ok := claims["jti"].(string); ok {
		response.TokenID = jti
	}
	if scope, ok := claims["scope"].(string); ok {
		response.Scope = scope
	}
	if clientID, ok := claims["client_id"].(string); ok {
		response.ClientID = clientID
	}
	if username, ok := claims["username"].(string); ok {
		response.Username = username
	}
	// Token type is always Bearer for access tokens
	response.TokenType = "Bearer"

	h.jsonResponse(w, http.StatusOK, response)
}

// HandleRevoke handles POST /revoke
func (h *Handler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "failed to parse form")
		return
	}

	token := r.FormValue("token")
	if token == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "token is required")
		return
	}

	// Get client credentials
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	// Validate client (optional for revocation per RFC 7009)
	if clientID != "" {
		client := h.provider.ValidateClient(clientID, clientSecret)
		if client == nil {
			h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
			return
		}
	}

	// Revoke the token
	h.provider.RevokeToken(token)

	// RFC 7009: respond with 200 OK regardless of whether token existed
	w.WriteHeader(http.StatusOK)
}

// Helper methods

func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) errorResponse(w http.ResponseWriter, status int, errorCode, description string) {
	h.jsonResponse(w, status, &ErrorResponse{
		Error:            errorCode,
		ErrorDescription: description,
	})
}

func (h *Handler) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state, errorCode, description string) {
	redirectURL, _ := url.Parse(redirectURI)
	q := redirectURL.Query()
	q.Set("error", errorCode)
	q.Set("error_description", description)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}
