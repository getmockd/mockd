---
title: OAuth / OIDC Mocking
description: Simulate a full OAuth 2.0 and OpenID Connect provider for testing authentication and authorization flows without external identity providers.
---

OAuth/OIDC mocking enables you to run a complete mock identity provider locally. Configure clients, users, scopes, and token lifetimes to test every authentication flow your application supports.

## Overview

mockd's OAuth/OIDC support includes:

- **Full OIDC provider** — Discovery document, JWKS, userinfo, and ID tokens
- **All major grant types** — Authorization Code (with PKCE), Client Credentials, Password, Refresh Token, Implicit
- **RS256 JWT signing** — Real RSA key pair generated per instance with JWKS endpoint
- **Token introspection** — RFC 7662 compliant introspection endpoint
- **Token revocation** — RFC 7009 compliant revocation endpoint
- **Scope validation** — Configurable allowed scopes with enforcement
- **Multiple clients and users** — Define as many as your tests require

## Quick Start

### CLI

Add an OAuth mock with a single command:

```bash
# OAuth/OIDC mock with sensible defaults
mockd oauth add

# Custom issuer, client, and user
mockd oauth add --name "Auth Server" \
  --issuer http://localhost:4280/auth \
  --client-id my-app --client-secret s3cret \
  --oauth-user admin --oauth-password admin123
```

### Configuration File

Create a minimal OAuth mock in your `mockd.yaml`:

```yaml
version: "1.0"

mocks:
  - id: my-auth-server
    name: Auth Server
    type: oauth
    enabled: true
    oauth:
      issuer: http://localhost:4280
      tokenExpiry: "1h"
      refreshExpiry: "7d"
      defaultScopes:
        - openid
        - profile
        - email
      clients:
        - clientId: my-app
          clientSecret: my-secret
          redirectUris:
            - http://localhost:3000/callback
          grantTypes:
            - authorization_code
            - client_credentials
            - refresh_token
            - password
      users:
        - username: testuser
          password: testpass
          claims:
            sub: "user-001"
            email: "testuser@example.com"
            name: "Test User"
```

Start the server and test:

```bash
# Start mockd
mockd serve --config mockd.yaml

# Get a token using client credentials
curl -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret"

# Response:
# {
#   "access_token": "eyJhbGciOiJSUzI1NiIs...",
#   "token_type": "Bearer",
#   "expires_in": 3600,
#   "scope": "openid profile email"
# }
```

## Endpoints

mockd exposes the standard OAuth 2.0 and OIDC endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/.well-known/openid-configuration` | GET | OIDC discovery document |
| `/.well-known/jwks.json` | GET | JSON Web Key Set for token verification |
| `/authorize` | GET, POST | Authorization endpoint (code + implicit flows) |
| `/token` | POST | Token endpoint (all grant types) |
| `/userinfo` | GET, POST | OIDC UserInfo — returns claims for the authenticated user |
| `/introspect` | POST | Token introspection (RFC 7662) |
| `/revoke` | POST | Token revocation (RFC 7009) |

All endpoints are mounted relative to the mock's base path. If your issuer is `http://localhost:4280`, then the token endpoint is `http://localhost:4280/token`.

## Grant Types

### Client Credentials

Machine-to-machine authentication. No user context — the client authenticates with its own credentials.

```bash
curl -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "scope=openid profile"
```

Client credentials can also be sent via HTTP Basic authentication:

```bash
curl -X POST http://localhost:4280/token \
  -u "my-app:my-secret" \
  -d "grant_type=client_credentials"
```

Response:

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "openid profile"
}
```

The `sub` claim in the JWT is set to the `client_id` for this grant type.

### Resource Owner Password

Authenticate with a username and password. Requires a configured user.

```bash
curl -X POST http://localhost:4280/token \
  -d "grant_type=password" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "username=testuser" \
  -d "password=testpass" \
  -d "scope=openid profile email"
```

Response:

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "aB3xY9...",
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "scope": "openid profile email"
}
```

An `id_token` is returned when the `openid` scope is requested. A `refresh_token` is included if the client has `refresh_token` in its `grantTypes`.

### Authorization Code

The standard browser-based redirect flow. mockd auto-approves the authorization request using the first configured user (no login page needed for testing).

**Step 1 — Redirect to authorize:**

```bash
curl -v "http://localhost:4280/authorize?\
client_id=my-app&\
redirect_uri=http://localhost:3000/callback&\
response_type=code&\
scope=openid profile email&\
state=random-state-value"
```

mockd responds with a `302` redirect to your `redirect_uri` with the authorization code:

```
Location: http://localhost:3000/callback?code=abc123...&state=random-state-value
```

**Step 2 — Exchange code for tokens:**

```bash
curl -X POST http://localhost:4280/token \
  -d "grant_type=authorization_code" \
  -d "code=abc123..." \
  -d "redirect_uri=http://localhost:3000/callback" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret"
```

Response:

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "xY9aB3...",
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "scope": "openid profile email"
}
```

Authorization codes are single-use and expire after 10 minutes.

### Authorization Code with PKCE

For public clients (SPAs, mobile apps) that cannot securely store a client secret. mockd supports both `S256` and `plain` challenge methods.

**Step 1 — Generate a code verifier and challenge:**

```bash
# Generate a random code_verifier
CODE_VERIFIER=$(openssl rand -base64 32 | tr -d '=/+' | head -c 43)

# Compute the S256 code_challenge
CODE_CHALLENGE=$(echo -n "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')
```

**Step 2 — Redirect to authorize with PKCE parameters:**

```bash
curl -v "http://localhost:4280/authorize?\
client_id=my-app&\
redirect_uri=http://localhost:3000/callback&\
response_type=code&\
scope=openid profile&\
state=random-state&\
code_challenge=$CODE_CHALLENGE&\
code_challenge_method=S256"
```

**Step 3 — Exchange code with the verifier (no client secret required for public clients):**

```bash
curl -X POST http://localhost:4280/token \
  -d "grant_type=authorization_code" \
  -d "code=abc123..." \
  -d "redirect_uri=http://localhost:3000/callback" \
  -d "client_id=my-app" \
  -d "code_verifier=$CODE_VERIFIER"
```

If the client has a `clientSecret` configured, it must still be provided. PKCE is an additional verification layer, not a replacement for confidential client authentication.

### Refresh Token

Exchange a refresh token for a new access token. The original refresh token is returned (not rotated).

```bash
curl -X POST http://localhost:4280/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=xY9aB3..." \
  -d "client_id=my-app" \
  -d "client_secret=my-secret"
```

Response:

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "xY9aB3...",
  "scope": "openid profile email"
}
```

You can optionally request a narrower scope:

```bash
curl -X POST http://localhost:4280/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=xY9aB3..." \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "scope=openid"
```

### Implicit

The implicit flow returns an access token directly in the redirect URL fragment. This flow is discouraged in modern applications but is supported for legacy client testing.

```bash
curl -v "http://localhost:4280/authorize?\
client_id=my-app&\
redirect_uri=http://localhost:3000/callback&\
response_type=token&\
scope=openid profile&\
state=random-state"
```

mockd responds with a `302` redirect:

```
Location: http://localhost:3000/callback#access_token=eyJ...&token_type=Bearer&expires_in=3600&state=random-state&scope=openid+profile
```

## Configuration

### Full Configuration Reference

```yaml
mocks:
  - id: auth-server
    name: My Auth Server
    type: oauth
    enabled: true
    oauth:
      # Issuer URL — used in JWT `iss` claim and discovery document (default: https://mock-oauth.local)
      issuer: http://localhost:4280

      # Access token lifetime (default: 1h)
      tokenExpiry: "1h"

      # Refresh token lifetime (default: 7d)
      refreshExpiry: "7d"

      # Allowed scopes — requests for unlisted scopes are rejected (default: openid, profile, email)
      defaultScopes:
        - openid
        - profile
        - email
        - api:read
        - api:write

      # Default claims added to every access token
      defaultClaims:
        aud: "https://api.example.com"

      # OAuth clients
      clients:
        # Confidential client (server-side app)
        - clientId: backend-service
          clientSecret: backend-secret
          redirectUris: []
          grantTypes:
            - client_credentials

        # Confidential client (web app with login)
        - clientId: web-app
          clientSecret: web-secret
          redirectUris:
            - http://localhost:3000/callback
            - http://localhost:3000/silent-renew
          grantTypes:
            - authorization_code
            - refresh_token
            - password

        # Public client (SPA with PKCE, no client secret)
        - clientId: spa-app
          clientSecret: ""
          redirectUris:
            - http://localhost:5173/callback
          grantTypes:
            - authorization_code
            - refresh_token

      # Users for password and authorization_code flows
      users:
        - username: alice
          password: alice123
          claims:
            sub: "user-alice"
            email: "alice@example.com"
            email_verified: true
            name: "Alice Smith"
            given_name: "Alice"
            family_name: "Smith"
            picture: "https://example.com/alice.jpg"

        - username: bob
          password: bob123
          claims:
            sub: "user-bob"
            email: "bob@example.com"
            email_verified: true
            name: "Bob Johnson"
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `issuer` | string | `https://mock-oauth.local` | Issuer URL for JWT `iss` claim and discovery endpoints |
| `tokenExpiry` | string | `"1h"` | Access token lifetime (Go duration or `Nd` for days) |
| `refreshExpiry` | string | `"7d"` | Refresh token lifetime |
| `defaultScopes` | string[] | `["openid", "profile", "email"]` | Allowed scopes — requests for unlisted scopes are rejected |
| `defaultClaims` | map | `{}` | Claims added to every access token |
| `clients` | ClientConfig[] | | OAuth client definitions |
| `users` | UserConfig[] | | User definitions for password/authorization_code flows |

### Client Configuration

| Field | Type | Description |
|-------|------|-------------|
| `clientId` | string | OAuth client identifier |
| `clientSecret` | string | Client secret (empty string for public clients) |
| `redirectUris` | string[] | Allowed redirect URIs for authorization flows |
| `grantTypes` | string[] | Allowed grant types: `authorization_code`, `client_credentials`, `refresh_token`, `password` |

### User Configuration

| Field | Type | Description |
|-------|------|-------------|
| `username` | string | Login username |
| `password` | string | Login password |
| `claims` | map | User claims (included in ID tokens and `/userinfo`). Common: `sub`, `email`, `name`, `picture` |

## Token Validation

mockd signs all tokens with RS256 using a 2048-bit RSA key pair generated at startup. The public key is available via the JWKS endpoint.

### JWKS Endpoint

```bash
curl http://localhost:4280/.well-known/jwks.json
```

Response:

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "mock-key-id",
      "alg": "RS256",
      "n": "base64url-encoded-modulus...",
      "e": "AQAB"
    }
  ]
}
```

### Validating Tokens in Your Application

Use the OIDC discovery document to configure your JWT validation library. Point it at the mock issuer and it will fetch the JWKS automatically.

**Go (using go-jose or similar):**

```go
// Configure your middleware to use the mock issuer
issuer := "http://localhost:4280"
jwksURL := issuer + "/.well-known/jwks.json"

// Most JWT libraries will fetch and cache the JWKS from this URL
validator := jwt.NewValidator(
    jwt.WithIssuer(issuer),
    jwt.WithJWKSURL(jwksURL),
)
```

**Node.js (using jose):**

```javascript
import { createRemoteJWKSet, jwtVerify } from 'jose';

const JWKS = createRemoteJWKSet(
  new URL('http://localhost:4280/.well-known/jwks.json')
);

const { payload } = await jwtVerify(token, JWKS, {
  issuer: 'http://localhost:4280',
});
```

### Token Introspection

Resource servers can verify tokens via the introspection endpoint (RFC 7662). This is useful when you don't want to validate JWTs locally.

```bash
curl -X POST http://localhost:4280/introspect \
  -u "my-app:my-secret" \
  -d "token=eyJhbGciOiJSUzI1NiIs..."
```

Active token response:

```json
{
  "active": true,
  "scope": "openid profile email",
  "client_id": "my-app",
  "sub": "user-001",
  "token_type": "Bearer",
  "exp": 1700000000,
  "iat": 1699996400,
  "iss": "http://localhost:4280"
}
```

Expired or invalid token response:

```json
{
  "active": false
}
```

### Token Revocation

Revoke an access token or refresh token (RFC 7009):

```bash
curl -X POST http://localhost:4280/revoke \
  -u "my-app:my-secret" \
  -d "token=eyJhbGciOiJSUzI1NiIs..."
```

The endpoint always returns `200 OK` regardless of whether the token existed, per the RFC specification. Revoked tokens will return `active: false` from the introspection endpoint and will be rejected by the `/userinfo` endpoint.

## OIDC Discovery

The discovery document at `/.well-known/openid-configuration` advertises all supported endpoints and capabilities:

```bash
curl http://localhost:4280/.well-known/openid-configuration
```

```json
{
  "issuer": "http://localhost:4280",
  "authorization_endpoint": "http://localhost:4280/authorize",
  "token_endpoint": "http://localhost:4280/token",
  "userinfo_endpoint": "http://localhost:4280/userinfo",
  "jwks_uri": "http://localhost:4280/.well-known/jwks.json",
  "revocation_endpoint": "http://localhost:4280/revoke",
  "introspection_endpoint": "http://localhost:4280/introspect",
  "response_types_supported": ["code", "token"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "iss", "aud", "exp", "iat", "auth_time", "nonce", "email", "email_verified", "name", "given_name", "family_name", "picture"],
  "grant_types_supported": ["authorization_code", "client_credentials", "refresh_token", "password"],
  "code_challenge_methods_supported": ["S256", "plain"]
}
```

Most OAuth/OIDC libraries can auto-configure themselves from this document. Point your library at `http://localhost:4280/.well-known/openid-configuration` and it will discover all endpoints automatically.

## Testing Patterns

### Testing Auth Middleware

Verify your API correctly rejects unauthenticated requests and accepts valid tokens:

```bash
# Get a token
TOKEN=$(curl -s -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" | jq -r '.access_token')

# Authenticated request to your API (which validates against mockd's JWKS)
curl -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/protected

# Verify rejection without a token
curl -v http://localhost:3000/api/protected
# Should return 401
```

### Testing Token Refresh

Simulate token expiration and renewal:

```yaml
oauth:
  tokenExpiry: "5s"   # Short-lived access tokens for testing
  refreshExpiry: "1h"
```

```bash
# Get initial tokens
RESPONSE=$(curl -s -X POST http://localhost:4280/token \
  -d "grant_type=password" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "username=testuser" \
  -d "password=testpass")

REFRESH_TOKEN=$(echo "$RESPONSE" | jq -r '.refresh_token')

# Wait for access token to expire
sleep 6

# Refresh the token
curl -X POST http://localhost:4280/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=$REFRESH_TOKEN" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret"
```

### Testing Scope Enforcement

Verify your API enforces scope requirements:

```bash
# Token with limited scopes
TOKEN=$(curl -s -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "scope=api:read" | jq -r '.access_token')

# Should succeed — read-only endpoint
curl -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/items

# Should fail — write endpoint requires api:write scope
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/items \
  -d '{"name": "new item"}'
# Your API should return 403
```

### Testing Invalid Credentials

Verify your application handles authentication errors:

```bash
# Invalid client credentials
curl -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=wrong-id" \
  -d "client_secret=wrong-secret"

# Response:
# {"error": "invalid_client", "error_description": "invalid client credentials"}

# Invalid user credentials
curl -X POST http://localhost:4280/token \
  -d "grant_type=password" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "username=testuser" \
  -d "password=wrong-password"

# Response:
# {"error": "invalid_grant", "error_description": "invalid user credentials"}

# Invalid scope
curl -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" \
  -d "scope=nonexistent"

# Response:
# {"error": "invalid_scope", "error_description": "scope \"nonexistent\" is not supported"}
```

### Testing Token Revocation Flow

Verify your application handles revoked tokens:

```bash
# Get a token
TOKEN=$(curl -s -X POST http://localhost:4280/token \
  -d "grant_type=client_credentials" \
  -d "client_id=my-app" \
  -d "client_secret=my-secret" | jq -r '.access_token')

# Token works
curl -H "Authorization: Bearer $TOKEN" http://localhost:4280/userinfo
# Returns user info

# Revoke it
curl -X POST http://localhost:4280/revoke \
  -u "my-app:my-secret" \
  -d "token=$TOKEN"

# Token no longer works
curl -H "Authorization: Bearer $TOKEN" http://localhost:4280/userinfo
# Returns 401

# Introspection confirms revocation
curl -X POST http://localhost:4280/introspect \
  -u "my-app:my-secret" \
  -d "token=$TOKEN"
# Returns {"active": false}
```

## Examples

### Microservice-to-Microservice Auth

Service-to-service communication using client credentials:

```yaml
mocks:
  - id: service-auth
    name: Service Auth
    type: oauth
    enabled: true
    oauth:
      issuer: http://localhost:4280
      tokenExpiry: "30m"
      defaultScopes:
        - service:read
        - service:write
      clients:
        - clientId: order-service
          clientSecret: order-secret
          grantTypes:
            - client_credentials
        - clientId: inventory-service
          clientSecret: inventory-secret
          grantTypes:
            - client_credentials
```

```bash
# Order service gets a token
curl -X POST http://localhost:4280/token \
  -u "order-service:order-secret" \
  -d "grant_type=client_credentials" \
  -d "scope=service:read service:write"
```

### Single Page Application with PKCE

Public client using authorization code + PKCE:

```yaml
mocks:
  - id: spa-auth
    name: SPA Auth Provider
    type: oauth
    enabled: true
    oauth:
      issuer: http://localhost:4280
      tokenExpiry: "15m"
      refreshExpiry: "7d"
      defaultScopes:
        - openid
        - profile
        - email
        - offline_access
      clients:
        - clientId: my-spa
          clientSecret: ""
          redirectUris:
            - http://localhost:5173/callback
            - http://localhost:5173/silent-renew
          grantTypes:
            - authorization_code
            - refresh_token
      users:
        - username: demo
          password: demo
          claims:
            sub: "user-demo"
            email: "demo@example.com"
            name: "Demo User"
            picture: "https://i.pravatar.cc/150?u=demo"
```

### Multi-Tenant API

Multiple clients with different scope permissions:

```yaml
mocks:
  - id: multi-tenant-auth
    name: Multi-Tenant Auth
    type: oauth
    enabled: true
    oauth:
      issuer: http://localhost:4280
      tokenExpiry: "1h"
      refreshExpiry: "30d"
      defaultScopes:
        - openid
        - profile
        - email
        - tenant:read
        - tenant:write
        - admin
      defaultClaims:
        aud: "https://api.example.com"
      clients:
        - clientId: tenant-a
          clientSecret: secret-a
          redirectUris:
            - http://tenant-a.localhost:3000/callback
          grantTypes:
            - authorization_code
            - refresh_token
            - password

        - clientId: tenant-b
          clientSecret: secret-b
          redirectUris:
            - http://tenant-b.localhost:3000/callback
          grantTypes:
            - authorization_code
            - refresh_token
            - password

        - clientId: admin-cli
          clientSecret: admin-secret
          grantTypes:
            - client_credentials
      users:
        - username: alice
          password: alice123
          claims:
            sub: "user-alice"
            email: "alice@tenant-a.com"
            name: "Alice (Tenant A)"
            tenant_id: "tenant-a"
            roles: ["editor"]

        - username: bob
          password: bob123
          claims:
            sub: "user-bob"
            email: "bob@tenant-b.com"
            name: "Bob (Tenant B)"
            tenant_id: "tenant-b"
            roles: ["viewer"]
```

```bash
# Alice logs in through Tenant A's client
curl -X POST http://localhost:4280/token \
  -d "grant_type=password" \
  -d "client_id=tenant-a" \
  -d "client_secret=secret-a" \
  -d "username=alice" \
  -d "password=alice123" \
  -d "scope=openid profile tenant:read tenant:write"

# Admin CLI gets a service token
curl -X POST http://localhost:4280/token \
  -u "admin-cli:admin-secret" \
  -d "grant_type=client_credentials" \
  -d "scope=admin tenant:read tenant:write"
```

## Next Steps

- [Configuration Reference](/reference/configuration) — Full configuration schema
- [Response Templating](/guides/response-templating) — Dynamic response values
- [CLI Reference](/reference/cli) — All CLI commands and flags
