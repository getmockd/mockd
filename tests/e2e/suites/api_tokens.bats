#!/usr/bin/env bats
# ============================================================================
# Token & API Key Management — registration tokens, API key rotation
# ============================================================================

setup() {
  load '../lib/helpers'
}

@test "TOKEN-001: Generate registration token" {
  api POST /admin/tokens/registration -d '{"name": "test-runtime"}'
  [[ "$STATUS" == "200" || "$STATUS" == "201" ]]
}

@test "TOKEN-002: List registration tokens" {
  api GET /admin/tokens/registration
  [[ "$STATUS" == "200" ]]
}

@test "TOKEN-003: Get API key" {
  api GET /admin/api-key
  [[ "$STATUS" == "200" ]]
}

@test "TOKEN-004: Rotate API key handled without 5xx" {
  api POST /admin/api-key/rotate
  [[ "$STATUS" -lt 500 ]]
}
