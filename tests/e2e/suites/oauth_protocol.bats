#!/usr/bin/env bats
# ============================================================================
# OAuth Protocol — OIDC discovery, JWKS, grants, PKCE, introspection, revoke
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  api POST /mocks -d '{
    "type": "oauth",
    "name": "Test OAuth Provider",
    "oauth": {
      "issuer": "http://localhost:4280/oauth",
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
  }'
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

# ── Discovery & JWKS ─────────────────────────────────────────────────────────

@test "OAUTH-001: Create OAuth mock returns 201" {
  api POST /mocks -d '{
    "type": "oauth",
    "name": "OAuth Verify",
    "oauth": {
      "issuer": "http://localhost:4280/oauth-verify",
      "clients": [{"clientId": "verify", "clientSecret": "s"}]
    }
  }'
  [[ "$STATUS" == "201" ]]
  local id=$(json_field '.id')
  api DELETE "/mocks/${id}"
}

@test "OAUTH-002: OIDC discovery endpoint returns 200" {
  engine GET /oauth/.well-known/openid-configuration
  [[ "$STATUS" == "200" ]]
}

@test "OAUTH-003: Discovery has token_endpoint" {
  engine GET /oauth/.well-known/openid-configuration
  [[ "$BODY" == *"token_endpoint"* ]]
}

@test "OAUTH-004: JWKS endpoint returns 200" {
  engine GET /oauth/.well-known/jwks.json
  [[ "$STATUS" == "200" ]]
}

@test "OAUTH-005: JWKS has keys" {
  engine GET /oauth/.well-known/jwks.json
  [[ "$BODY" == *"keys"* ]]
}

# ── Client Credentials Grant ──────────────────────────────────────────────────

@test "OAUTH-006: Client credentials grant returns 200" {
  engine_form POST /oauth/token \
    "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid"
  [[ "$STATUS" == "200" ]]
}

@test "OAUTH-007: Client credentials response has access_token" {
  engine_form POST /oauth/token \
    "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid"
  [[ "$BODY" == *"access_token"* ]]
}

# ── Password Grant ────────────────────────────────────────────────────────────

@test "OAUTH-008: Password grant returns 200" {
  engine_form POST /oauth/token \
    "grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid+profile"
  [[ "$STATUS" == "200" ]]
}

@test "OAUTH-009: Invalid credentials rejected" {
  engine_form POST /oauth/token \
    "grant_type=client_credentials&client_id=wrong&client_secret=wrong"
  [[ "$STATUS" == "401" || "$STATUS" == "400" ]]
}

# ── Authorization Code Grant ─────────────────────────────────────────────────

@test "OAUTH-010: Authorize endpoint returns redirect with code" {
  # Follow redirect manually (curl -s without -L)
  local resp
  resp=$(curl -s -o /dev/null -w '%{http_code}\n%{redirect_url}' \
    "${ENGINE}/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&state=xyz123" \
    2>/dev/null) || true
  local status=$(echo "$resp" | head -1)
  local redirect=$(echo "$resp" | tail -1)

  [[ "$status" == "302" ]]
  [[ "$redirect" == *"code="* ]]
  [[ "$redirect" == *"state=xyz123"* ]]
}

@test "OAUTH-011: Authorization code exchange returns tokens" {
  # Step 1: Get auth code from redirect
  local redirect
  redirect=$(curl -s -o /dev/null -w '%{redirect_url}' \
    "${ENGINE}/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&state=abc" \
    2>/dev/null) || true

  local code
  code=$(echo "$redirect" | sed 's/.*code=\([^&]*\).*/\1/')
  [[ -n "$code" ]]

  # Step 2: Exchange code for tokens
  engine_form POST /oauth/token \
    "grant_type=authorization_code&code=${code}&redirect_uri=http://localhost:3000/callback&client_id=test-app&client_secret=test-secret"
  [[ "$STATUS" == "200" ]]

  # Should have access_token, refresh_token, and id_token (openid scope)
  [[ "$BODY" == *"access_token"* ]]
  [[ "$BODY" == *"refresh_token"* ]]
  [[ "$BODY" == *"id_token"* ]]
}

# ── PKCE (RFC 7636) ──────────────────────────────────────────────────────────

@test "OAUTH-012: PKCE S256 flow succeeds" {
  # code_verifier: a random string
  local code_verifier="dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
  # code_challenge: BASE64URL(SHA256(code_verifier))
  local code_challenge
  code_challenge=$(printf '%s' "$code_verifier" | openssl dgst -sha256 -binary 2>/dev/null | base64 | tr '+/' '-_' | tr -d '=') || \
  code_challenge=$(printf '%s' "$code_verifier" | sha256sum | xxd -r -p | base64 | tr '+/' '-_' | tr -d '=')

  # Step 1: Authorize with code_challenge
  local redirect
  redirect=$(curl -s -o /dev/null -w '%{redirect_url}' \
    "${ENGINE}/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&code_challenge=${code_challenge}&code_challenge_method=S256" \
    2>/dev/null) || true

  local code
  code=$(echo "$redirect" | sed 's/.*code=\([^&]*\).*/\1/')
  [[ -n "$code" ]]

  # Step 2: Exchange with code_verifier
  engine_form POST /oauth/token \
    "grant_type=authorization_code&code=${code}&redirect_uri=http://localhost:3000/callback&client_id=test-app&client_secret=test-secret&code_verifier=${code_verifier}"
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"access_token"* ]]
}

@test "OAUTH-013: PKCE wrong verifier rejected" {
  local code_verifier="correct-verifier-value-long-enough-for-pkce"
  local code_challenge
  code_challenge=$(printf '%s' "$code_verifier" | openssl dgst -sha256 -binary 2>/dev/null | base64 | tr '+/' '-_' | tr -d '=') || \
  code_challenge=$(printf '%s' "$code_verifier" | sha256sum | xxd -r -p | base64 | tr '+/' '-_' | tr -d '=')

  local redirect
  redirect=$(curl -s -o /dev/null -w '%{redirect_url}' \
    "${ENGINE}/oauth/authorize?client_id=test-app&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid&code_challenge=${code_challenge}&code_challenge_method=S256" \
    2>/dev/null) || true

  local code
  code=$(echo "$redirect" | sed 's/.*code=\([^&]*\).*/\1/')

  # Exchange with WRONG code_verifier
  engine_form POST /oauth/token \
    "grant_type=authorization_code&code=${code}&redirect_uri=http://localhost:3000/callback&client_id=test-app&client_secret=test-secret&code_verifier=wrong-verifier-completely-different"
  [[ "$STATUS" == "400" ]]
  [[ "$BODY" == *"invalid_grant"* ]]
}

# ── Refresh Token ─────────────────────────────────────────────────────────────

@test "OAUTH-014: Refresh token grant returns new access_token" {
  # First get a refresh token via password grant
  engine_form POST /oauth/token \
    "grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid"
  [[ "$STATUS" == "200" ]]

  local refresh_token
  refresh_token=$(echo "$BODY" | jq -r '.refresh_token')
  [[ -n "$refresh_token" && "$refresh_token" != "null" ]]

  # Exchange refresh token for new access token
  engine_form POST /oauth/token \
    "grant_type=refresh_token&refresh_token=${refresh_token}&client_id=test-app&client_secret=test-secret"
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"access_token"* ]]
}

# ── Token Introspection (RFC 7662) ───────────────────────────────────────────

@test "OAUTH-015: Token introspection returns active=true" {
  # Get a token
  engine_form POST /oauth/token \
    "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid"
  local access_token
  access_token=$(echo "$BODY" | jq -r '.access_token')

  # Introspect it
  engine_form POST /oauth/introspect \
    "token=${access_token}&client_id=test-app&client_secret=test-secret"
  [[ "$STATUS" == "200" ]]

  local active
  active=$(echo "$BODY" | jq -r '.active')
  [[ "$active" == "true" ]]
}

# ── Token Revocation (RFC 7009) ──────────────────────────────────────────────

@test "OAUTH-016: Revoke token then introspect returns active=false" {
  # Get a token
  engine_form POST /oauth/token \
    "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid"
  local access_token
  access_token=$(echo "$BODY" | jq -r '.access_token')

  # Revoke it
  engine_form POST /oauth/revoke \
    "token=${access_token}&client_id=test-app&client_secret=test-secret"
  [[ "$STATUS" == "200" ]]

  # Introspect — should be inactive
  engine_form POST /oauth/introspect \
    "token=${access_token}&client_id=test-app&client_secret=test-secret"
  [[ "$STATUS" == "200" ]]

  local active
  active=$(echo "$BODY" | jq -r '.active')
  [[ "$active" == "false" ]]
}

# ── Userinfo Endpoint ─────────────────────────────────────────────────────────

@test "OAUTH-017: Userinfo endpoint returns user claims" {
  # Get a token via password grant (ties to a user)
  engine_form POST /oauth/token \
    "grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid+profile+email"
  local access_token
  access_token=$(echo "$BODY" | jq -r '.access_token')

  # Call userinfo with Bearer token
  local resp
  resp=$(curl -s -w '\n%{http_code}' \
    -H "Authorization: Bearer ${access_token}" \
    "${ENGINE}/oauth/userinfo" 2>/dev/null) || true
  BODY=$(echo "$resp" | sed '$d')
  STATUS=$(echo "$resp" | tail -n 1)

  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"user-123"* ]]
  [[ "$BODY" == *"test@example.com"* ]]
}

# ── Discovery Extended Fields ─────────────────────────────────────────────────

@test "OAUTH-018: Discovery includes introspection_endpoint" {
  engine GET /oauth/.well-known/openid-configuration
  [[ "$BODY" == *"introspection_endpoint"* ]]
}

@test "OAUTH-019: Discovery includes userinfo_endpoint" {
  engine GET /oauth/.well-known/openid-configuration
  [[ "$BODY" == *"userinfo_endpoint"* ]]
}

# ── JWT Structure ─────────────────────────────────────────────────────────────

@test "OAUTH-020: Access token is a valid JWT with expected claims" {
  engine_form POST /oauth/token \
    "grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid"
  local access_token
  access_token=$(echo "$BODY" | jq -r '.access_token')

  # JWT is 3 dot-separated base64url segments — decode the payload (segment 2)
  local payload
  payload=$(echo "$access_token" | cut -d. -f2 | tr '_-' '/+' | base64 -d 2>/dev/null || \
            echo "$access_token" | cut -d. -f2 | tr '_-' '/+' | base64 -D 2>/dev/null) || true

  # Verify issuer claim
  local iss
  iss=$(echo "$payload" | jq -r '.iss' 2>/dev/null) || true
  [[ "$iss" == *"oauth"* ]]

  # Verify expiry exists
  local exp
  exp=$(echo "$payload" | jq -r '.exp' 2>/dev/null) || true
  [[ -n "$exp" && "$exp" != "null" ]]
}
