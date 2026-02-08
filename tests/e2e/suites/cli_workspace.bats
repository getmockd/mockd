#!/usr/bin/env bats
# ============================================================================
# CLI Workspace Management — list, create (via API), delete via CLI
# ============================================================================

setup() {
  load '../lib/helpers'
}

@test "CLI-WS-001: mockd workspace list exits 0" {
  run mockd workspace list -u "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-WS-002: mockd workspace list --json returns valid JSON" {
  run mockd workspace list --json -u "$ADMIN"
  [[ "$status" -eq 0 ]]
  echo "$output" | jq . >/dev/null 2>&1
}

@test "CLI-WS-003: CLI shows workspace created via API" {
  api POST /workspaces -d '{"name": "cli-delete-test", "description": "For CLI delete test"}'
  [[ "$STATUS" == "201" ]]
  local ws_id
  ws_id=$(json_field '.id')
  [[ -n "$ws_id" && "$ws_id" != "null" ]] || skip "No workspace ID returned"

  run mockd workspace list -u "$ADMIN"
  [[ "$output" == *"cli-delete-test"* ]]

  # Cleanup
  run mockd workspace delete --force -u "$ADMIN" "$ws_id"
}

@test "CLI-WS-004: mockd workspace delete removes workspace" {
  api POST /workspaces -d '{"name": "cli-ws-del-test", "description": "For delete test"}'
  local ws_id
  ws_id=$(json_field '.id')
  [[ -n "$ws_id" && "$ws_id" != "null" ]] || skip "No workspace ID returned"

  run mockd workspace delete --force -u "$ADMIN" "$ws_id"
  [[ "$status" -eq 0 ]]

  api GET "/workspaces/${ws_id}"
  [[ "$STATUS" == "404" ]]
}
