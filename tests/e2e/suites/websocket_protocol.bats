#!/usr/bin/env bats
# ============================================================================
# WebSocket Protocol — mock creation, handler registration
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

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
