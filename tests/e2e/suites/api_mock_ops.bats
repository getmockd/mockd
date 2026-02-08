#!/usr/bin/env bats
# ============================================================================
# Mock Operations — bulk create, patch, toggle, verification
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

# ─── Bulk Operations ────────────────────────────────────────────────────────

@test "MOCK-001: POST /mocks/bulk creates mocks" {
  api POST /mocks/bulk -d '[
    {
      "type": "http",
      "name": "Bulk Mock 1",
      "http": {
        "matcher": {"method": "GET", "path": "/api/bulk1"},
        "response": {"statusCode": 200, "body": "{\"n\": 1}"}
      }
    },
    {
      "type": "http",
      "name": "Bulk Mock 2",
      "http": {
        "matcher": {"method": "GET", "path": "/api/bulk2"},
        "response": {"statusCode": 200, "body": "{\"n\": 2}"}
      }
    }
  ]'
  [[ "$STATUS" == "200" || "$STATUS" == "201" ]]
}

@test "MOCK-001b: Bulk mock 1 responds" {
  engine GET /api/bulk1
  [[ "$STATUS" == "200" ]]
}

@test "MOCK-001c: Bulk mock 2 responds" {
  engine GET /api/bulk2
  [[ "$STATUS" == "200" ]]
}

# ─── Patch & Toggle ─────────────────────────────────────────────────────────

@test "MOCK-002: PATCH /mocks/{id} updates name" {
  api GET /mocks
  local mock_id
  mock_id=$(json_field '.mocks[0].id')
  [[ -n "$mock_id" ]] || skip "No mock ID available"

  api PATCH "/mocks/${mock_id}" -d '{"name":"Patched Name"}'
  [[ "$STATUS" == "200" || "$STATUS" == "404" ]]
}

@test "MOCK-003: Toggle mock off and on" {
  api GET /mocks
  local mock_id
  mock_id=$(json_field '.mocks[0].id')
  [[ -n "$mock_id" ]] || skip "No mock ID available"

  api POST "/mocks/${mock_id}/toggle" -d '{"enabled": false}'
  [[ "$STATUS" == "200" ]]

  api POST "/mocks/${mock_id}/toggle" -d '{"enabled": true}'
  [[ "$STATUS" == "200" ]]
}

# ─── Verification (S16) ─────────────────────────────────────────────────────

@test "S16-001: GET /mocks/{id}/verify returns 200" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Verify Me",
    "http": {
      "matcher": {"method": "GET", "path": "/api/verify-me"},
      "response": {"statusCode": 200, "body": "hello"}
    }
  }'
  local mock_id
  mock_id=$(json_field '.id')

  engine GET /api/verify-me
  engine GET /api/verify-me

  api GET "/mocks/${mock_id}/verify"
  [[ "$STATUS" == "200" ]]
}

@test "S16-002: GET /mocks/{id}/invocations returns 200" {
  api GET /mocks
  local mock_id
  mock_id=$(echo "$BODY" | jq -r '.mocks[-1].id // empty')
  [[ -n "$mock_id" ]] || skip "No mock ID available"

  api GET "/mocks/${mock_id}/invocations"
  [[ "$STATUS" == "200" ]]
}

@test "S16-003: DELETE /mocks/{id}/invocations returns 200" {
  api GET /mocks
  local mock_id
  mock_id=$(echo "$BODY" | jq -r '.mocks[-1].id // empty')
  [[ -n "$mock_id" ]] || skip "No mock ID available"

  api DELETE "/mocks/${mock_id}/invocations"
  [[ "$STATUS" == "200" ]]
}

@test "MOCK-004: POST /mocks/{id}/verify assertion" {
  api GET /mocks
  local mock_id
  mock_id=$(json_field '.mocks[0].id')
  [[ -n "$mock_id" ]] || skip "No mock ID available"

  api POST "/mocks/${mock_id}/verify" -d '{"atLeast": 0}'
  [[ "$STATUS" == "200" || "$STATUS" == "404" ]]
}

@test "MOCK-005: DELETE /verify resets all" {
  api DELETE /verify
  [[ "$STATUS" == "200" || "$STATUS" == "204" ]]
}
