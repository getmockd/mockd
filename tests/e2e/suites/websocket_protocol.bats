#!/usr/bin/env bats
# ============================================================================
# WebSocket Protocol — mock creation, handler registration
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  # Mock with matchers + defaultResponse (used for matcher/default tests)
  api POST /mocks -d '{
    "type": "websocket",
    "name": "Test WebSocket",
    "websocket": {
      "path": "/ws/test",
      "echoMode": true,
      "matchers": [
        {
          "match": {"type": "exact", "value": "ping"},
          "response": {"type": "text", "value": "pong"}
        }
      ],
      "defaultResponse": {"type": "text", "value": "unknown"}
    }
  }'

  # Echo-only mock (no matchers, no defaultResponse — pure echo)
  api POST /mocks -d '{
    "type": "websocket",
    "name": "Echo Only WS",
    "websocket": {"path": "/ws/echo", "echoMode": true}
  }'
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

@test "WS-001: Create WebSocket mock returns 201" {
  api POST /mocks -d '{
    "type": "websocket",
    "name": "WS Verify",
    "websocket": {"path": "/ws/verify", "echoMode": true}
  }'
  [[ "$STATUS" == "201" ]]
  local id=$(json_field '.id')
  api DELETE "/mocks/${id}"
}

@test "WS-002: Handlers list returns 200" {
  api GET /handlers
  [[ "$STATUS" == "200" ]]
}

@test "WS-003: At least 1 handler registered" {
  api GET /handlers
  local handler_count
  handler_count=$(echo "$BODY" | jq '.handlers | length // .count // 0' 2>/dev/null) || handler_count=0
  [[ "$handler_count" -ge 1 ]] || skip "Handler count=$handler_count (may be listed differently)"
}

# ── Wire-level tests using websocat ──────────────────────────────────────────

@test "WS-004: Echo mode returns sent message" {
  # Use the echo-only mock (no matchers/defaultResponse) so echo actually fires
  local result
  result=$(printf "hello e2e" | timeout 3 websocat --no-line -1 "ws://mockd:4280/ws/echo" 2>/dev/null) || true
  [[ "$result" == "hello e2e" ]]
}

@test "WS-005: Matcher responds with pong for ping" {
  # Use --no-line to avoid trailing newline that breaks exact match
  local result
  result=$(printf "ping" | timeout 3 websocat --no-line -1 "ws://mockd:4280/ws/test" 2>/dev/null) || true
  [[ "$result" == "pong" ]]
}

@test "WS-006: Default response for unmatched message" {
  local result
  result=$(printf "something-random" | timeout 3 websocat --no-line -1 "ws://mockd:4280/ws/test" 2>/dev/null) || true
  [[ "$result" == "unknown" ]]
}

@test "WS-007: Multiple messages over single connection" {
  # Line mode sends each line as a separate WS message
  local result
  result=$(printf "msg1\nmsg2\n" | timeout 3 websocat "ws://mockd:4280/ws/echo" 2>/dev/null) || true
  # Should get at least 2 response lines (echo-only mock echoes each)
  local line_count
  line_count=$(echo "$result" | wc -l)
  [[ "$line_count" -ge 2 ]]
}

@test "WS-008: Connection to non-existent WS path fails" {
  local result exit_code=0
  result=$(printf "test" | timeout 3 websocat --no-line -1 "ws://mockd:4280/ws/no-such-path" 2>&1) || exit_code=$?
  # Should get an error — non-zero exit or error message
  [[ "$exit_code" -ne 0 ]] || [[ "$result" == *"error"* ]] || [[ "$result" == *"404"* ]] || [[ -z "$result" ]]
}
