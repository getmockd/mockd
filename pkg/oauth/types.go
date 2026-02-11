package oauth

import "time"

// OAuthConfig configures the OAuth mock provider
type OAuthConfig struct {
	ID            string                 `json:"id" yaml:"id"`
	Issuer        string                 `json:"issuer" yaml:"issuer"`
	TokenExpiry   string                 `json:"tokenExpiry" yaml:"tokenExpiry"`     // e.g., "1h"
	RefreshExpiry string                 `json:"refreshExpiry" yaml:"refreshExpiry"` // e.g., "7d"
	DefaultScopes []string               `json:"defaultScopes" yaml:"defaultScopes"`
	DefaultClaims map[string]interface{} `json:"defaultClaims" yaml:"defaultClaims"`
	Clients       []ClientConfig         `json:"clients" yaml:"clients"`
	Users         []UserConfig           `json:"users" yaml:"users"`
	Enabled       bool                   `json:"enabled" yaml:"enabled"`
}

// ClientConfig defines an OAuth client configuration
type ClientConfig struct {
	ClientID     string   `json:"clientId" yaml:"clientId"`
	ClientSecret string   `json:"clientSecret" yaml:"clientSecret"`
	RedirectURIs []string `json:"redirectUris" yaml:"redirectUris"`
	GrantTypes   []string `json:"grantTypes" yaml:"grantTypes"` // authorization_code, client_credentials, refresh_token
}

// UserConfig defines a user for the resource owner password credentials flow
type UserConfig struct {
	Username string                 `json:"username" yaml:"username"`
	Password string                 `json:"password" yaml:"password"`
	Claims   map[string]interface{} `json:"claims" yaml:"claims"` // sub, email, name, etc.
}

// TokenResponse represents an OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ErrorResponse represents an OAuth error response
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// AuthorizationCode represents a stored authorization code
type AuthorizationCode struct {
	Code                string
	ClientID            string
	RedirectURI         string
	Scope               string
	UserID              string
	Nonce               string
	CodeChallenge       string // PKCE: stored code_challenge
	CodeChallengeMethod string // PKCE: "S256" or "plain"
	ExpiresAt           time.Time
}

// RefreshTokenData represents stored refresh token data
type RefreshTokenData struct {
	Token     string
	ClientID  string
	UserID    string
	Scope     string
	ExpiresAt time.Time
}

// OpenIDConfiguration represents the OIDC discovery document
type OpenIDConfiguration struct {
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
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
}

// IntrospectionResponse represents an OAuth 2.0 Token Introspection response (RFC 7662)
type IntrospectionResponse struct {
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

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// Standard OAuth error codes
const (
	ErrInvalidRequest          = "invalid_request"
	ErrUnauthorizedClient      = "unauthorized_client"
	ErrAccessDenied            = "access_denied"
	ErrUnsupportedResponseType = "unsupported_response_type"
	ErrInvalidScope            = "invalid_scope"
	ErrServerError             = "server_error"
	ErrInvalidClient           = "invalid_client"
	ErrInvalidGrant            = "invalid_grant"
	ErrUnsupportedGrantType    = "unsupported_grant_type"
)

// Grant types
const (
	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeClientCredentials = "client_credentials"
	GrantTypeRefreshToken      = "refresh_token"
	GrantTypePassword          = "password"
)

// Response types
const (
	ResponseTypeCode  = "code"
	ResponseTypeToken = "token"
)
