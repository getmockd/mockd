#!/usr/bin/env bats
# ============================================================================
# OAuth Protocol — OIDC discovery, JWKS, grants, token validation
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
          "grantTypes": ["client_credentials", "password"]
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

@test "OAUTH-006: Client credentials grant returns 200" {
  local token_resp
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid' 2>&1) || true
  BODY=$(echo "$token_resp" | sed '$d')
  STATUS=$(echo "$token_resp" | tail -n 1)
  [[ "$STATUS" == "200" ]]
}

@test "OAUTH-007: Client credentials response has access_token" {
  local token_resp
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid' 2>&1) || true
  BODY=$(echo "$token_resp" | sed '$d')
  [[ "$BODY" == *"access_token"* ]]
}

@test "OAUTH-008: Password grant returns 200" {
  local token_resp
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid+profile' 2>&1) || true
  BODY=$(echo "$token_resp" | sed '$d')
  STATUS=$(echo "$token_resp" | tail -n 1)
  [[ "$STATUS" == "200" ]]
}

@test "OAUTH-009: Invalid credentials rejected" {
  local token_resp
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=client_credentials&client_id=wrong&client_secret=wrong' 2>&1) || true
  STATUS=$(echo "$token_resp" | tail -n 1)
  [[ "$STATUS" == "401" || "$STATUS" == "400" ]]
}
