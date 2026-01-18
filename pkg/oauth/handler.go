package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Handler provides OAuth endpoint handlers
type Handler struct {
	provider *Provider
}

// NewHandler creates OAuth HTTP handlers
func NewHandler(provider *Provider) *Handler {
	return &Handler{provider: provider}
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
		var userID string
		if len(h.provider.config.Users) > 0 {
			if sub, ok := h.provider.config.Users[0].Claims["sub"].(string); ok {
				userID = sub
			} else {
				userID = h.provider.config.Users[0].Username
			}
		} else {
			userID = "mock-user"
		}

		// Generate authorization code
		code, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		authCode := &AuthorizationCode{
			Code:        code,
			ClientID:    clientID,
			RedirectURI: redirectURI,
			Scope:       scope,
			UserID:      userID,
			Nonce:       nonce,
			ExpiresAt:   time.Now().Add(10 * time.Minute),
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
		var userID string
		if len(h.provider.config.Users) > 0 {
			if sub, ok := h.provider.config.Users[0].Claims["sub"].(string); ok {
				userID = sub
			} else {
				userID = h.provider.config.Users[0].Username
			}
		} else {
			userID = "mock-user"
		}

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
		fragment.Set("expires_in", fmt.Sprintf("%d", int(h.provider.tokenExpiry.Seconds())))
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

	if code == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "code is required")
		return
	}
	if redirectURI == "" {
		h.errorResponse(w, http.StatusBadRequest, ErrInvalidRequest, "redirect_uri is required")
		return
	}

	// Validate client
	client := h.provider.ValidateClient(clientID, clientSecret)
	if client == nil {
		h.errorResponse(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
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

	// Generate tokens
	tokenClaims := map[string]interface{}{
		"sub":       authCode.UserID,
		"client_id": clientID,
		"scope":     authCode.Scope,
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
		Scope:       authCode.Scope,
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
			Scope:     authCode.Scope,
			ExpiresAt: time.Now().Add(h.provider.refreshExpiry),
		})
		response.RefreshToken = refreshToken
	}

	// Generate ID token if openid scope is requested
	if strings.Contains(authCode.Scope, "openid") {
		user := h.provider.GetUserByID(authCode.UserID)
		idTokenClaims := map[string]interface{}{
			"sub": authCode.UserID,
			"aud": clientID,
		}
		if authCode.Nonce != "" {
			idTokenClaims["nonce"] = authCode.Nonce
		}
		if user != nil {
			for k, v := range user.Claims {
				if k != "sub" { // sub already set
					idTokenClaims[k] = v
				}
			}
		}
		idToken, err := h.provider.GenerateIDToken(idTokenClaims)
		if err == nil {
			response.IDToken = idToken
		}
	}

	h.jsonResponse(w, http.StatusOK, response)
}

func (h *Handler) handleClientCredentialsGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	scope := r.FormValue("scope")

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
	if strings.Contains(scope, "openid") {
		user := h.provider.GetUserByID(refreshData.UserID)
		idTokenClaims := map[string]interface{}{
			"sub": refreshData.UserID,
			"aud": clientID,
		}
		if user != nil {
			for k, v := range user.Claims {
				if k != "sub" {
					idTokenClaims[k] = v
				}
			}
		}
		idToken, err := h.provider.GenerateIDToken(idTokenClaims)
		if err == nil {
			response.IDToken = idToken
		}
	}

	h.jsonResponse(w, http.StatusOK, response)
}

func (h *Handler) handlePasswordGrant(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	scope := r.FormValue("scope")

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
	if strings.Contains(scope, "openid") {
		idTokenClaims := map[string]interface{}{
			"sub": userID,
			"aud": clientID,
		}
		for k, v := range user.Claims {
			if k != "sub" {
				idTokenClaims[k] = v
			}
		}
		idToken, err := h.provider.GenerateIDToken(idTokenClaims)
		if err == nil {
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
		ResponseTypesSupported:            []string{"code", "token", "id_token", "code token", "code id_token", "token id_token", "code token id_token"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   h.provider.config.DefaultScopes,
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post"},
		ClaimsSupported:                   []string{"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce", "email", "email_verified", "name", "given_name", "family_name", "picture"},
		GrantTypesSupported:               []string{"authorization_code", "client_credentials", "refresh_token", "password"},
	}

	h.jsonResponse(w, http.StatusOK, config)
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
