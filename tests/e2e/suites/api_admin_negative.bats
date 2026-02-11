#!/usr/bin/env bats
# ============================================================================
# Admin API Negative Tests — invalid inputs, nonexistent resources, edge cases
# ============================================================================

setup() {
  load '../lib/helpers'
}

# ─── S7: Core Negative Tests ────────────────────────────────────────────────

@test "S7-NEG-001: POST /mocks invalid JSON → 400" {
  api POST /mocks -d 'not json'
  [[ "$STATUS" == "400" ]]
}

@test "S7-NEG-005: GET /mocks/{id} nonexistent → 404" {
  api GET /mocks/nonexistent-id-12345
  [[ "$STATUS" == "404" ]]
}

@test "S7-NEG-006: PUT /mocks/{id} nonexistent → 404" {
  api PUT /mocks/nonexistent-id-12345 -d '{"name":"test","type":"http"}'
  [[ "$STATUS" == "404" ]]
}

@test "S7-NEG-007: DELETE /mocks/{id} nonexistent → 404" {
  api DELETE /mocks/nonexistent-id-12345
  [[ "$STATUS" == "404" ]]
}

@test "S7-NEG-010: POST /mocks empty body → 400" {
  api POST /mocks -d ''
  [[ "$STATUS" == "400" ]]
}

@test "S7-NEG-011: POST /config invalid JSON → 400" {
  api POST /config -d 'not json'
  [[ "$STATUS" == "400" ]]
}

@test "S7-NEG-012: POST /config null config → 400" {
  api POST /config -d '{"config": null}'
  [[ "$STATUS" == "400" ]]
}

@test "S7-NEG-020: GET /requests/{id} nonexistent → 404" {
  api GET /requests/nonexistent-id-12345
  [[ "$STATUS" == "404" ]]
}

@test "S7-NEG-030: POST /mocks/{id}/toggle nonexistent → 404" {
  api POST /mocks/nonexistent-id-12345/toggle -d '{"enabled": true}'
  [[ "$STATUS" == "404" ]]
}

@test "S7-NEG-040: DELETE /folders/{id} nonexistent → 404" {
  api DELETE /folders/nonexistent-folder
  [[ "$STATUS" == "404" ]]
}

@test "S7-NEG-050: GET /state/resources/{name} nonexistent → 404" {
  api GET /state/resources/nonexistent-resource
  [[ "$STATUS" == "404" ]]
}

# ─── S7 Extended ─────────────────────────────────────────────────────────────

@test "S7-EXT-001: POST /mocks invalid type → 4xx" {
  api POST /mocks -d '{"name":"test","type":"invalid_protocol"}'
  [[ "$STATUS" == "400" || "$STATUS" == "422" ]]
}

@test "S7-EXT-002: PATCH /mocks/{id} nonexistent → 404" {
  api PATCH /mocks/nonexistent-id-12345 -d '{"name":"patched"}'
  [[ "$STATUS" == "404" ]]
}

@test "S7-EXT-003: PUT /chaos invalid JSON → 400" {
  api PUT /chaos -d 'not json'
  [[ "$STATUS" == "400" ]]
}

@test "S7-EXT-004: POST /state/reset with empty body handled" {
  api POST /state/reset -d ''
  [[ "$STATUS" == "200" || "$STATUS" == "400" ]]
}

@test "S7-EXT-005: GET /workspaces/{id} nonexistent → 404" {
  api GET /workspaces/nonexistent-ws
  [[ "$STATUS" == "404" ]]
}

@test "S7-EXT-006: PUT /workspaces/{id} nonexistent → 404" {
  api PUT /workspaces/nonexistent-ws -d '{"name":"test"}'
  [[ "$STATUS" == "404" ]]
}

@test "S7-EXT-007: POST /config with empty mocks → 200" {
  api POST /config -d '{"config":{"version":"1.0","name":"empty","mocks":[]}}'
  [[ "$STATUS" == "200" ]]
}

@test "S7-EXT-008: Large request body handled without 5xx" {
  local big_body
  big_body=$(head -c 100000 /dev/urandom | base64 | tr -d '\n' | head -c 100000)
  big_body="{\"name\":\"${big_body}\",\"type\":\"http\"}"
  api POST /mocks -d "$big_body"
  [[ "$STATUS" -lt 500 ]]
}
