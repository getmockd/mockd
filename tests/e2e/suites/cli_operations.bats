#!/usr/bin/env bats
# ============================================================================
# CLI Operations — request logs, chaos injection, ports
# ============================================================================

setup() {
  load '../lib/helpers'
}

# ─── Request Logs ────────────────────────────────────────────────────────────

@test "CLI-LOG-001: mockd logs --requests shows request paths" {
  api DELETE /mocks
  run mockd add --path /cli/log-target --body '{"logged":true}' --name "Log Target" --admin-url "$ADMIN"
  engine GET /cli/log-target
  engine GET /cli/log-target
  engine GET /cli/log-target

  run mockd logs --requests --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"/cli/log-target"* ]]
}

@test "CLI-LOG-002: mockd logs --requests --json returns valid JSON" {
  run mockd logs --requests --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  local log_len
  log_len=$(echo "$output" | jq 'length')
  [[ "$log_len" -ge 1 ]]
}

@test "CLI-LOG-003: mockd logs --requests --method GET filters by method" {
  run mockd logs --requests --method GET --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"GET"* ]]
}

@test "CLI-LOG-004: mockd logs --requests --path filters by path" {
  run mockd logs --requests --path /cli/log-target --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"log-target"* ]]
}

@test "CLI-LOG-005: mockd logs --requests --clear clears logs" {
  run mockd logs --requests --clear --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Cleared"* ]]
}

@test "CLI-LOG-006: Logs cleared successfully" {
  run mockd logs --requests --clear --admin-url "$ADMIN"
  run mockd logs --requests --json --admin-url "$ADMIN"
  local after_len
  after_len=$(echo "$output" | jq 'length')
  # May have new requests from clear call — accept small counts
  [[ "$after_len" -le 2 ]]
}

# ─── Chaos Injection ────────────────────────────────────────────────────────

@test "CLI-CHAOS-001: mockd chaos status shows disabled" {
  run mockd chaos status --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"disabled"* ]]
}

@test "CLI-CHAOS-002: mockd chaos status --json returns valid JSON" {
  run mockd chaos status --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  echo "$output" | jq . >/dev/null 2>&1
}

@test "CLI-CHAOS-003: mockd chaos enable with latency" {
  run mockd chaos enable --latency "10ms-50ms" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"enabled"* ]]
}

@test "CLI-CHAOS-004: Chaos now enabled after enable" {
  run mockd chaos enable --latency "10ms-50ms" --admin-url "$ADMIN"
  run mockd chaos status --admin-url "$ADMIN"
  [[ "$output" == *"enabled"* ]]
}

@test "CLI-CHAOS-005: mockd chaos enable with error rate" {
  run mockd chaos enable --error-rate 0.5 --error-code 503 --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-CHAOS-006: mockd chaos enable with path pattern" {
  run mockd chaos enable --latency "1ms-5ms" --path "/api/.*" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-CHAOS-007: mockd chaos disable" {
  run mockd chaos disable --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"disabled"* ]]
}

@test "CLI-CHAOS-008: Chaos confirmed disabled" {
  run mockd chaos disable --admin-url "$ADMIN"
  run mockd chaos status --admin-url "$ADMIN"
  [[ "$output" == *"disabled"* ]]
}

# ─── Ports ───────────────────────────────────────────────────────────────────

@test "CLI-PORTS-001: mockd ports shows engine and admin ports" {
  run mockd ports --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"4280"* ]]
  [[ "$output" == *"4290"* ]]
}

@test "CLI-PORTS-002: mockd ports --json returns valid JSON" {
  run mockd ports --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  echo "$output" | jq . >/dev/null 2>&1
}
