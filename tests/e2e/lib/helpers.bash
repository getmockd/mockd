#!/bin/bash
# ============================================================================
# mockd E2E Test Helpers — shared by all bats suites
# ============================================================================
# Provides HTTP helper functions for calling the admin API and mock engine.
# Loaded by bats suites via: load '../lib/helpers'
#
# Globals set by api()/engine():
#   STATUS  — HTTP status code (e.g., "200", "404"). Reset on every call.
#   BODY    — Response body text. Reset on every call.

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
  local tmpfile
  tmpfile=$(mktemp)
  STATUS=$(curl -s -w '%{http_code}' -o "$tmpfile" -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>/dev/null) || STATUS="000"
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
}

# Hit the mock engine (not admin).
# Usage: engine GET /api/users => sets $STATUS and $BODY
engine() {
  local method="$1" path="$2"
  shift 2
  local url="${ENGINE}${path}"
  local tmpfile
  tmpfile=$(mktemp)
  STATUS=$(curl -s -w '%{http_code}' -o "$tmpfile" -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>/dev/null) || STATUS="000"
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
}

# Hit the mock engine with form-encoded body (for OAuth token endpoints, etc.).
# Usage: engine_form POST /oauth/token "grant_type=client_credentials&client_id=app"
engine_form() {
  local method="$1" path="$2" data="$3"
  shift 3
  local url="${ENGINE}${path}"
  local tmpfile
  tmpfile=$(mktemp)
  STATUS=$(curl -s -w '%{http_code}' -o "$tmpfile" -X "$method" "$url" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d "$data" "$@" 2>/dev/null) || STATUS="000"
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
}

# ─── JSON Helpers ──────────────────────────────────────────────────────────────

# Extract a JSON field from $BODY.
# Usage: json_field '.name' => prints the value
json_field() {
  echo "$BODY" | jq -r "$1" 2>/dev/null
}

# ─── Wait Helpers ──────────────────────────────────────────────────────────────

# Wait for a TCP port to accept connections.
# Usage: wait_for_port mockd 50051       (default 30 retries × 0.5s = 15s)
#        wait_for_port mockd 1883 20     (20 retries × 0.5s = 10s)
wait_for_port() {
  local host="$1" port="$2" retries="${3:-30}"
  for ((i=0; i<retries; i++)); do
    if timeout 1 bash -c "echo >/dev/tcp/${host}/${port}" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
  done
  echo "wait_for_port: ${host}:${port} not reachable after ${retries} retries" >&2
  return 1
}

# Wait for a TCP port to stop accepting connections (after deletion/disable).
# Usage: wait_for_port_down mockd 50051
wait_for_port_down() {
  local host="$1" port="$2" retries="${3:-20}"
  for ((i=0; i<retries; i++)); do
    if ! timeout 1 bash -c "echo >/dev/tcp/${host}/${port}" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
  done
  echo "wait_for_port_down: ${host}:${port} still reachable after ${retries} retries" >&2
  return 1
}
