#!/usr/bin/env bats
# ============================================================================
# Folder Management — CRUD operations on mock folders
# ============================================================================

setup() {
  load '../lib/helpers'
}

@test "S1-001: GET /folders returns 200" {
  api GET /folders
  [[ "$STATUS" == "200" ]]
}

@test "S1-002: POST /folders creates folder" {
  api POST /folders -d '{"name": "Test Folder", "description": "For testing"}'
  [[ "$STATUS" == "201" ]]
}

@test "S1-003: GET /folders/{id} returns folder with correct name" {
  api POST /folders -d '{"name": "Get Folder Test", "description": "For get test"}'
  local folder_id
  folder_id=$(json_field '.id')

  api GET "/folders/${folder_id}"
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.name')" == "Get Folder Test" ]]
}

@test "S1-004: PUT /folders/{id} updates folder" {
  api POST /folders -d '{"name": "Before Rename", "description": "test"}'
  local folder_id
  folder_id=$(json_field '.id')

  api PUT "/folders/${folder_id}" -d '{"name": "Renamed Folder"}'
  [[ "$STATUS" == "200" ]]
}

@test "S1-005: DELETE /folders/{id} removes folder" {
  api POST /folders -d '{"name": "To Delete", "description": "test"}'
  local folder_id
  folder_id=$(json_field '.id')

  api DELETE "/folders/${folder_id}"
  [[ "$STATUS" == "204" ]]
}

@test "S1-006: DELETE nonexistent folder → 404" {
  api DELETE "/folders/nonexistent-folder-id"
  [[ "$STATUS" == "404" ]]
}
