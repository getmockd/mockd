#!/usr/bin/env bats
# ============================================================================
# Request Logging — capture, retrieve, and clear request logs
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  api POST /mocks -d '{
    "type": "http",
    "name": "Request Log Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/log-target"},
      "response": {"statusCode": 200, "body": "{\"logged\": true}"}
    }
  }'

  engine GET /api/log-target
  engine GET /api/log-target
  engine GET /api/log-target
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

@test "REQ-001: GET /requests returns 200" {
  api GET /requests
  [[ "$STATUS" == "200" ]]
}

@test "REQ-002: At least 3 requests logged" {
  api GET /requests
  local req_count
  req_count=$(echo "$BODY" | jq '.count // .total // 0')
  [[ "$req_count" -ge 3 ]]
}

@test "REQ-003: GET /requests/{id} returns specific request" {
  api GET /requests
  local req_id
  req_id=$(echo "$BODY" | jq -r '.requests[0].id // empty')
  [[ -n "$req_id" ]] || skip "No request ID available"

  api GET "/requests/${req_id}"
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.path')" == "/api/log-target" ]]
}

@test "REQ-004: DELETE /requests clears logs" {
  api DELETE /requests
  [[ "$STATUS" == "200" ]]
}

@test "REQ-005: Requests cleared to zero" {
  api DELETE /requests
  api GET /requests
  local after_count
  after_count=$(echo "$BODY" | jq '.count // .total // 0')
  [[ "$after_count" -eq 0 ]]
}
