#!/usr/bin/env bats
# ============================================================================
# Recordings — HTTP proxy recording, MITM capture, convert to mock, replay
# ============================================================================
# Tests the full recording lifecycle:
#   1. Create an HTTP mock (serves as upstream target)
#   2. Start proxy in record mode
#   3. Make proxied requests through the proxy
#   4. Verify recordings were captured
#   5. Convert recording to a new mock
#   6. Verify the new mock responds on the engine

PROXY_PORT=8888

setup_file() {
  load '../lib/helpers'

  # Ensure clean state — stop any running proxy, delete all mocks
  api POST /proxy/stop 2>/dev/null || true
  api DELETE /mocks

  # Create an HTTP mock to serve as our "upstream" target
  api POST /mocks -d '{
    "type": "http",
    "name": "Upstream Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/users"},
      "response": {
        "statusCode": 200,
        "headers": {"X-Custom": "recorded"},
        "body": "{\"users\": [{\"id\": 1, \"name\": \"Alice\"}, {\"id\": 2, \"name\": \"Bob\"}]}"
      }
    }
  }'
  json_field '.id' > "$BATS_FILE_TMPDIR/upstream_mock_id"

  # Create a second endpoint for POST recording
  api POST /mocks -d '{
    "type": "http",
    "name": "Upstream Create",
    "http": {
      "matcher": {"method": "POST", "path": "/api/users"},
      "response": {
        "statusCode": 201,
        "body": "{\"id\": 3, \"name\": \"Charlie\"}"
      }
    }
  }'
}

teardown_file() {
  load '../lib/helpers'
  api POST /proxy/stop 2>/dev/null || true
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

# ── Smoke Tests (existing) ───────────────────────────────────────────────────

@test "S6-001: GET /recordings returns 200" {
  api GET /recordings
  [[ "$STATUS" == "200" ]]
}

@test "S6-002: GET /stream-recordings returns 200" {
  api GET /stream-recordings
  [[ "$STATUS" == "200" ]]
}

@test "S6-003: GET /stream-recordings/stats returns 200" {
  api GET /stream-recordings/stats
  [[ "$STATUS" == "200" ]]
}

@test "S6-004: GET /replay returns 200" {
  api GET /replay
  [[ "$STATUS" == "200" ]]
}

@test "S6-005: GET /mqtt-recordings returns 200" {
  api GET /mqtt-recordings
  [[ "$STATUS" == "200" ]]
}

@test "S6-006: GET /soap-recordings returns 200" {
  api GET /soap-recordings
  [[ "$STATUS" == "200" ]]
}

# ── MITM Proxy Recording Lifecycle ───────────────────────────────────────────

@test "REC-001: Start proxy in record mode" {
  api POST /proxy/start -d "{\"port\": ${PROXY_PORT}, \"mode\": \"record\"}"
  [[ "$STATUS" == "200" || "$STATUS" == "201" ]]

  # Verify proxy is running
  api GET /proxy/status
  local running
  running=$(echo "$BODY" | jq -r '.running')
  [[ "$running" == "true" ]]
}

@test "REC-002: Proxied GET request succeeds" {
  # Make a request through the proxy to our upstream mock
  local resp
  resp=$(curl -s -w '\n%{http_code}' -x "http://mockd:${PROXY_PORT}" \
    "http://mockd:4280/api/users" 2>/dev/null) || true
  local body=$(echo "$resp" | sed '$d')
  local status=$(echo "$resp" | tail -n 1)

  [[ "$status" == "200" ]]
  echo "$body" | grep -q "Alice"
}

@test "REC-003: Proxied POST request succeeds" {
  local resp
  resp=$(curl -s -w '\n%{http_code}' -x "http://mockd:${PROXY_PORT}" \
    -X POST "http://mockd:4280/api/users" \
    -H 'Content-Type: application/json' \
    -d '{"name": "Charlie"}' 2>/dev/null) || true
  local status=$(echo "$resp" | tail -n 1)

  [[ "$status" == "201" ]]
}

@test "REC-004: Recordings captured after proxied requests" {
  api GET /recordings
  [[ "$STATUS" == "200" ]]

  # Should have at least 2 recordings (GET + POST from previous tests)
  local count
  count=$(echo "$BODY" | jq 'if type == "array" then length elif .recordings then (.recordings | length) elif .count then .count else 0 end')
  [[ "$count" -ge 2 ]]
}

@test "REC-005: Individual recording has correct request details" {
  api GET /recordings
  # Get the first recording ID
  local rec_id
  rec_id=$(echo "$BODY" | jq -r 'if type == "array" then .[0].id elif .recordings then .recordings[0].id else empty end')
  [[ -n "$rec_id" && "$rec_id" != "null" ]] || skip "No recording ID available"

  api GET "/recordings/${rec_id}"
  [[ "$STATUS" == "200" ]]

  # Verify it captured request details
  [[ "$BODY" == *"/api/users"* ]]
}

@test "REC-006: Convert recording to mock" {
  api GET /recordings
  local rec_id
  rec_id=$(echo "$BODY" | jq -r 'if type == "array" then .[0].id elif .recordings then .recordings[0].id else empty end')
  [[ -n "$rec_id" && "$rec_id" != "null" ]] || skip "No recording ID available"

  # Convert to mock
  api POST "/recordings/${rec_id}/to-mock" -d '{"addToServer": true}'
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"mock"* ]]
}

@test "REC-007: Stop proxy" {
  api POST /proxy/stop
  [[ "$STATUS" == "200" ]]

  # Verify stopped
  api GET /proxy/status
  local running
  running=$(echo "$BODY" | jq -r '.running')
  [[ "$running" == "false" ]]
}

@test "REC-008: Clear recordings" {
  api DELETE /recordings
  [[ "$STATUS" == "200" || "$STATUS" == "204" ]]
}
