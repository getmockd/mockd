#!/usr/bin/env bats
# ============================================================================
# CLI Version & Help — version output, help text
# ============================================================================

@test "CLI-VER-001: mockd version exits 0 and contains mockd" {
  run mockd version
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"mockd"* ]]
}

@test "CLI-VER-002: mockd version --json returns valid JSON" {
  run mockd version --json
  [[ "$status" -eq 0 ]]
  echo "$output" | jq . >/dev/null 2>&1
}

@test "CLI-HELP-001: mockd --help mentions core commands" {
  run mockd --help
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"add"* ]]
  [[ "$output" == *"list"* ]]
  [[ "$output" == *"import"* ]]
}

@test "CLI-HELP-002: mockd add --help mentions path and body flags" {
  run mockd add --help
  # Cobra may return exit 0 or 1 for --help
  [[ "$status" -eq 0 || "$status" -eq 1 ]]
  [[ "$output" == *"path"* ]]
  [[ "$output" == *"body"* ]]
}
