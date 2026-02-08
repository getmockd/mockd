#!/usr/bin/env bats
# ============================================================================
# Smoke Tests — connectivity, health, status, basic admin endpoints
# ============================================================================
# Verifies the mockd server is reachable and core admin endpoints respond.
# Always runs first (00_ prefix) to catch connectivity issues early.

setup_file() {
  load '../lib/helpers'
}

setup() {
  load '../lib/helpers'
}

# ─── Health & Status ─────────────────────────────────────────────────────────

@test "SMOKE-001: Admin health endpoint returns 200" {
  api GET /health
  [[ "$STATUS" == "200" ]]
}

@test "SMOKE-002: Health status is ok" {
  api GET /health
  [[ "$(json_field '.status')" == "ok" ]]
}

@test "SMOKE-003: Admin status endpoint returns 200" {
  api GET /status
  [[ "$STATUS" == "200" ]]
}

@test "SMOKE-004: List mocks returns 200" {
  api GET /mocks
  [[ "$STATUS" == "200" ]]
}

@test "SMOKE-005: Zero mocks initially" {
  api GET /mocks
  [[ "$(json_field '.total')" == "0" ]]
}

# ─── Startup & Ports ────────────────────────────────────────────────────────

@test "S10-001: Health endpoint responsive" {
  api GET /health
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.status')" == "ok" ]]
}

@test "S10-002: Status endpoint works" {
  api GET /status
  [[ "$STATUS" == "200" ]]
}

@test "S10-003: Ports endpoint works" {
  api GET /ports
  [[ "$STATUS" == "200" ]]
}

# ─── Sessions & Handlers ────────────────────────────────────────────────────

@test "S2-001: GET /sessions returns 200" {
  api GET /sessions
  [[ "$STATUS" == "200" ]]
}

@test "S3-001: GET /handlers returns 200" {
  api GET /handlers
  [[ "$STATUS" == "200" ]]
}
