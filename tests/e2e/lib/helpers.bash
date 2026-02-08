#!/bin/bash
# ============================================================================
# mockd E2E Test Helpers — shared by all bats suites
# ============================================================================
# Provides HTTP helper functions for calling the admin API and mock engine.
# Loaded by bats suites via: load '../lib/helpers'

# ─── Environment ───────────────────────────────────────────────────────────────

ADMIN="${MOCKD_ADMIN_URL:?MOCKD_ADMIN_URL is required}"
ENGINE="${MOCKD_ENGINE_URL:?MOCKD_ENGINE_URL is required}"
MOCKD_CLI="mockd"

# ─── HTTP Helpers ──────────────────────────────────────────────────────────────

# Make an admin API call and capture status + body.
# Usage: api GET /health => sets $STATUS and $BODY
api() {
  local method="$1" path="$2"
  shift 2
  local url="${ADMIN}${path}"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>&1) || true
  BODY=$(echo "$resp" | sed '$d')
  STATUS=$(echo "$resp" | tail -n 1)
}

# Hit the mock engine (not admin).
# Usage: engine GET /api/users => sets $STATUS and $BODY
engine() {
  local method="$1" path="$2"
  shift 2
  local url="${ENGINE}${path}"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>&1) || true
  BODY=$(echo "$resp" | sed '$d')
  STATUS=$(echo "$resp" | tail -n 1)
}

# ─── JSON Helpers ──────────────────────────────────────────────────────────────

# Extract a JSON field from $BODY.
# Usage: json_field '.name' => prints the value
json_field() {
  echo "$BODY" | jq -r "$1" 2>/dev/null
}

# Extract a JSON field from arbitrary input.
# Usage: echo "$output" | json_val '.id'
json_val() {
  jq -r "$1" 2>/dev/null
}
