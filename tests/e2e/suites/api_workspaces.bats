#!/usr/bin/env bats
# ============================================================================
# Workspace Management — CRUD operations on workspaces
# ============================================================================

setup() {
  load '../lib/helpers'
}

@test "WKSP-001: GET /workspaces returns 200" {
  api GET /workspaces
  [[ "$STATUS" == "200" ]]
}

@test "WKSP-002: POST /workspaces creates workspace" {
  api POST /workspaces -d '{"name": "test-ws", "description": "Test workspace"}'
  [[ "$STATUS" == "201" ]]
}

@test "WKSP-003: GET /workspaces/{id} returns workspace with correct name" {
  api POST /workspaces -d '{"name": "get-ws-test", "description": "For get test"}'
  local ws_id
  ws_id=$(json_field '.id')

  api GET "/workspaces/${ws_id}"
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.name')" == "get-ws-test" ]]

  api DELETE "/workspaces/${ws_id}"
}

@test "WKSP-004: PUT /workspaces/{id} updates workspace" {
  api POST /workspaces -d '{"name": "update-ws-test", "description": "For update test"}'
  local ws_id
  ws_id=$(json_field '.id')

  api PUT "/workspaces/${ws_id}" -d '{"name": "updated-ws", "description": "Updated"}'
  [[ "$STATUS" == "200" ]]

  api DELETE "/workspaces/${ws_id}"
}

@test "WKSP-005: DELETE /workspaces/{id} removes workspace" {
  api POST /workspaces -d '{"name": "delete-ws-test", "description": "For delete test"}'
  local ws_id
  ws_id=$(json_field '.id')

  api DELETE "/workspaces/${ws_id}"
  [[ "$STATUS" == "204" ]]
}

@test "WKSP-006: DELETE nonexistent workspace → 404" {
  api DELETE "/workspaces/nonexistent-ws-id"
  [[ "$STATUS" == "404" ]]
}
