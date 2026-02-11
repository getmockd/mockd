#!/usr/bin/env bats
# ============================================================================
# CLI Import & Export — import YAML, export, replace, dry-run, round-trip
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  cat > /tmp/cli-import-test.yaml << 'YAML'
version: "1.0"
name: cli-import-test
mocks:
  - id: cli-imp-alpha
    type: http
    name: Imported Alpha
    http:
      matcher:
        method: GET
        path: /cli/imported-alpha
      response:
        statusCode: 200
        body: '{"from":"import","name":"alpha"}'
  - id: cli-imp-beta
    type: http
    name: Imported Beta
    http:
      matcher:
        method: POST
        path: /cli/imported-beta
      response:
        statusCode: 201
        body: '{"from":"import","name":"beta"}'
YAML

  cat > /tmp/cli-replace-test.yaml << 'YAML'
version: "1.0"
name: cli-replace-test
mocks:
  - id: cli-replace-mock
    type: http
    name: Replacement Mock
    http:
      matcher:
        method: GET
        path: /cli/replaced
      response:
        statusCode: 200
        body: '{"replaced":true}'
YAML
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
  rm -f /tmp/cli-import-test.yaml /tmp/cli-replace-test.yaml /tmp/cli-exported.yaml
}

setup() {
  load '../lib/helpers'
}

@test "CLI-IMP-001: mockd import exits 0 and says Imported" {
  run mockd import /tmp/cli-import-test.yaml --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Imported"* ]]
}

@test "CLI-IMP-002: Imported alpha responds with correct body" {
  engine GET /cli/imported-alpha
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.name')" == "alpha" ]]
}

@test "CLI-IMP-003: Imported beta responds 201" {
  engine POST /cli/imported-beta -d '{}'
  [[ "$STATUS" == "201" ]]
}

@test "CLI-EXP-001: mockd export contains imported paths" {
  run mockd export --name "cli-export-test" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"/cli/imported-alpha"* ]]
  [[ "$output" == *"/cli/imported-beta"* ]]
}

@test "CLI-EXP-002: mockd export --output creates file" {
  run mockd export --name "cli-export-file" --output /tmp/cli-exported.yaml --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ -f /tmp/cli-exported.yaml ]]
  grep -q "/cli/imported-alpha" /tmp/cli-exported.yaml
}

@test "CLI-IMP-004: mockd import --replace clears old mocks" {
  run mockd import /tmp/cli-replace-test.yaml --replace --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]

  engine GET /cli/replaced
  [[ "$STATUS" == "200" ]]

  engine GET /cli/imported-alpha
  [[ "$STATUS" == "404" ]]
}

@test "CLI-IMP-005: mockd import --dry-run doesn't change state" {
  run mockd import /tmp/cli-import-test.yaml --dry-run --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Dry run"* ]]

  # Replacement mock should still be there from previous test
  engine GET /cli/replaced
  [[ "$STATUS" == "200" ]]
}

@test "CLI-IMP-006: Re-import exported file works" {
  api DELETE /mocks
  [[ -f /tmp/cli-exported.yaml ]] || skip "No export file to re-import"

  run mockd import /tmp/cli-exported.yaml --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]

  run mockd list --json --admin-url "$ADMIN"
  local reimport_count
  reimport_count=$(echo "$output" | jq 'length')
  [[ "$reimport_count" -ge 1 ]]
}
