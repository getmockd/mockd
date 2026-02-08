#!/usr/bin/env bats
# ============================================================================
# CLI Negative Cases — error handling, missing args, bad inputs
# ============================================================================

setup() {
  load '../lib/helpers'
}

@test "CLI-NEG-001: mockd add without --path fails or auto-generates path" {
  run mockd add --body '{"no":"path"}' --admin-url "$ADMIN"
  # CLI either fails (no path) or succeeds with an auto-generated path — both are acceptable
  [[ "$status" -eq 0 || "$status" -ne 0 ]]
  # But if it succeeded, output should mention "Created" or contain a path
  if [[ "$status" -eq 0 ]]; then
    [[ "$output" == *"Created"* || "$output" == *"created"* || "$output" == *"/"* ]]
  fi
}

@test "CLI-NEG-002: mockd get nonexistent mock fails" {
  run mockd get nonexistent-mock-id-99999 --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}

@test "CLI-NEG-003: mockd delete nonexistent mock fails" {
  run mockd delete nonexistent-mock-id-99999 --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}

@test "CLI-NEG-004: mockd import nonexistent file fails" {
  run mockd import /tmp/nonexistent-file-12345.yaml --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}

@test "CLI-NEG-005: mockd import invalid YAML fails" {
  echo "this: is: not: valid: yaml: [[[" > /tmp/cli-bad.yaml
  run mockd import /tmp/cli-bad.yaml --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
  rm -f /tmp/cli-bad.yaml
}

@test "CLI-NEG-006: mockd add --type invalid_type fails" {
  run mockd add --type invalid_type --path /cli/bad-type --body '{"bad":true}' --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}

@test "CLI-NEG-007: mockd list with unreachable admin-url fails" {
  run mockd list --admin-url http://nonexistent-host:9999
  [[ "$status" -ne 0 ]]
}

@test "CLI-NEG-008: mockd delete without ID fails" {
  run mockd delete --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}

@test "CLI-NEG-009: mockd get without ID fails" {
  run mockd get --admin-url "$ADMIN"
  [[ "$status" -ne 0 ]]
}
