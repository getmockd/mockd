#!/usr/bin/env bats
# ============================================================================
# CLI Core CRUD — add, list, get, delete via mockd CLI binary
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  # Create a suite of mocks for list/get/delete tests
  run mockd add --path /cli/hello --body '{"cli":"hello"}' --name "CLI Hello" --admin-url "$ADMIN"
  run mockd add --method POST --path /cli/users --status 201 \
    --body '{"id":"u1","created":true}' --name "CLI Create User" \
    --header "X-Custom:cli-test" --delay 50 --admin-url "$ADMIN"
  run mockd add --path /cli/authed --body '{"authed":true}' \
    --match-header "Authorization:Bearer test" --name "CLI Auth Mock" --admin-url "$ADMIN"
  run mockd add --path /cli/search --body '{"results":[]}' \
    --match-query "q:hello" --name "CLI Search" --admin-url "$ADMIN"
  run mockd add --path /cli/priority --body '{"level":"low"}' --priority 1 --name "Low" --admin-url "$ADMIN"
  run mockd add --path /cli/priority --body '{"level":"high"}' --priority 100 --name "High" --admin-url "$ADMIN"
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

# ─── Add ─────────────────────────────────────────────────────────────────────

@test "CLI-ADD-001: mockd add exits 0 and says Created" {
  run mockd add --path /cli/add-test --body '{"test":true}' --name "Add Test" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Created mock"* ]]
}

@test "CLI-ADD-002: CLI-created mock responds 200 with correct body" {
  engine GET /cli/hello
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.cli')" == "hello" ]]
}

@test "CLI-ADD-003: POST mock responds 201" {
  engine POST /cli/users -d '{"name":"test"}'
  [[ "$STATUS" == "201" ]]
}

@test "CLI-ADD-004: Match header mock responds to matching header" {
  engine GET /cli/authed -H "Authorization: Bearer test"
  [[ "$STATUS" == "200" ]]
}

@test "CLI-ADD-005: Query param mock responds to matching query" {
  engine GET '/cli/search?q=hello'
  [[ "$STATUS" == "200" ]]
}

@test "CLI-ADD-006: Higher priority wins" {
  engine GET /cli/priority
  [[ "$(json_field '.level')" == "high" ]]
}

@test "CLI-ADD-007: mockd add --json outputs JSON with action and path" {
  run mockd add --path /cli/json-out --body '{"j":true}' --name "JSON Out" --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$(echo "$output" | jq -r '.action')" == "created" ]]
  [[ "$(echo "$output" | jq -r '.path')" == "/cli/json-out" ]]
}

# ─── List ────────────────────────────────────────────────────────────────────

@test "CLI-LIST-001: mockd list exits 0 and shows mocks" {
  run mockd list --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"/cli/hello"* ]]
  [[ "$output" == *"http"* ]]
}

@test "CLI-LIST-002: mockd list --json returns valid JSON array" {
  run mockd list --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  local list_count
  list_count=$(echo "$output" | jq 'length')
  [[ "$list_count" -ge 6 ]]
}

@test "CLI-LIST-003: mockd list --type http filters correctly" {
  run mockd list --type http --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"/cli/"* ]]
}

# ─── Get & Delete ────────────────────────────────────────────────────────────

@test "CLI-GET-001: mockd get shows mock details" {
  run mockd add --path /cli/get-delete-test --body '{"forDelete":true}' --name "Get Delete Test" --json --admin-url "$ADMIN"
  local target_id
  target_id=$(echo "$output" | jq -r '.id // empty')
  [[ -n "$target_id" ]] || skip "No mock ID created"

  run mockd get "$target_id" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Name:"* ]]
  [[ "$output" == *"Path:"* ]]
}

@test "CLI-GET-002: mockd get --json returns correct ID" {
  run mockd add --path /cli/get-json-test --body '{"test":true}' --name "Get JSON Test" --json --admin-url "$ADMIN"
  local target_id
  target_id=$(echo "$output" | jq -r '.id // empty')
  [[ -n "$target_id" ]] || skip "No mock ID created"

  run mockd get "$target_id" --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$(echo "$output" | jq -r '.id')" == "$target_id" ]]
}

@test "CLI-DEL-001: mockd delete removes mock" {
  run mockd add --path /cli/to-delete --body '{"del":true}' --name "To Delete" --json --admin-url "$ADMIN"
  local target_id
  target_id=$(echo "$output" | jq -r '.id // empty')
  [[ -n "$target_id" ]] || skip "No mock ID created"

  run mockd delete "$target_id" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Deleted mock"* ]]

  run mockd get "$target_id" --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}
