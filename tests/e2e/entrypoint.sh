#!/bin/bash
set -euo pipefail

# ============================================================================
# mockd E2E Test Runner
# ============================================================================
#
# Runs test suites against a live mockd instance.
# Set TEST_SUITES env var to control which suites run:
#   TEST_SUITES=all          (default) run everything
#   TEST_SUITES=smoke        just the smoke test
#   TEST_SUITES=s7,s9,s10   specific supplement sections
#
# Environment:
#   MOCKD_ADMIN_URL   - Admin API base URL (e.g., http://mockd:4290)
#   MOCKD_ENGINE_URL  - Mock engine base URL (e.g., http://mockd:4280)

ADMIN="${MOCKD_ADMIN_URL:?MOCKD_ADMIN_URL is required}"
ENGINE="${MOCKD_ENGINE_URL:?MOCKD_ENGINE_URL is required}"
SUITES="${TEST_SUITES:-all}"

PASS=0
FAIL=0
SKIP=0
ERRORS=()
RESULTS_FILE="/results/e2e-results-$(date +%Y%m%d-%H%M%S).txt"

# ─── Helpers ─────────────────────────────────────────────────────────────────

log()  { echo "$(date +%H:%M:%S) $*"; }
pass() { ((PASS++)); log "  ✓ $1"; }
fail() { ((FAIL++)); ERRORS+=("$1: $2"); log "  ✗ $1 — $2"; }
skip() { ((SKIP++)); log "  ⊘ $1 (skipped)"; }
suite_header() { echo ""; log "━━━ $1 ━━━"; }

# Make an API call and capture status + body
# Usage: api GET /health => sets $STATUS and $BODY
api() {
  local method="$1" path="$2"
  shift 2
  local url="${ADMIN}${path}"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>&1) || true
  BODY=$(echo "$resp" | head -n -1)
  STATUS=$(echo "$resp" | tail -n 1)
}

# Hit the mock engine (not admin)
engine() {
  local method="$1" path="$2"
  shift 2
  local url="${ENGINE}${path}"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>&1) || true
  BODY=$(echo "$resp" | head -n -1)
  STATUS=$(echo "$resp" | tail -n 1)
}

assert_status() {
  local expected="$1" test_id="$2"
  if [[ "$STATUS" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected HTTP $expected, got $STATUS"
  fi
}

assert_json_field() {
  local field="$1" expected="$2" test_id="$3"
  local actual
  actual=$(echo "$BODY" | jq -r "$field" 2>/dev/null) || actual="(jq parse error)"
  if [[ "$actual" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected $field=$expected, got $actual"
  fi
}

assert_body_contains() {
  local needle="$1" test_id="$2"
  if echo "$BODY" | grep -q "$needle"; then
    pass "$test_id"
  else
    fail "$test_id" "body does not contain '$needle'"
  fi
}

should_run() {
  [[ "$SUITES" == "all" ]] && return 0
  local suite="$1"
  echo "$SUITES" | tr ',' '\n' | grep -qw "$suite"
}

# ─── Smoke Test ──────────────────────────────────────────────────────────────

run_smoke() {
  suite_header "SMOKE: Connectivity"

  api GET /health
  assert_status 200 "SMOKE-001: Admin health"

  api GET /status
  assert_status 200 "SMOKE-002: Admin status"
  assert_json_field '.status' 'ok' "SMOKE-003: Status is ok"

  api GET /mocks
  assert_status 200 "SMOKE-004: List mocks (empty)"
  assert_json_field '.total' '0' "SMOKE-005: Zero mocks initially"
}

# ─── S7: Admin API Negative Tests (P0) ──────────────────────────────────────

run_s7() {
  suite_header "S7: Admin API Negative Tests"

  # S7-NEG-001: POST /mocks with invalid JSON
  api POST /mocks -d 'not json'
  assert_status 400 "S7-NEG-001: POST /mocks invalid JSON → 400"

  # S7-NEG-002: POST /mocks missing type
  api POST /mocks -d '{"name":"test"}'
  assert_status 400 "S7-NEG-002: POST /mocks missing type → 400"

  # S7-NEG-005: GET /mocks/{id} nonexistent
  api GET /mocks/nonexistent-id-12345
  assert_status 404 "S7-NEG-005: GET /mocks/{id} nonexistent → 404"

  # S7-NEG-006: PUT /mocks/{id} nonexistent
  api PUT /mocks/nonexistent-id-12345 -d '{"name":"test","type":"http"}'
  assert_status 404 "S7-NEG-006: PUT /mocks/{id} nonexistent → 404"

  # S7-NEG-007: DELETE /mocks/{id} nonexistent
  api DELETE /mocks/nonexistent-id-12345
  assert_status 404 "S7-NEG-007: DELETE /mocks/{id} nonexistent → 404"

  # S7-NEG-010: POST /mocks with empty body
  api POST /mocks -d ''
  assert_status 400 "S7-NEG-010: POST /mocks empty body → 400"

  # S7-NEG-011: POST /config with invalid JSON
  api POST /config -d 'not json'
  assert_status 400 "S7-NEG-011: POST /config invalid JSON → 400"

  # S7-NEG-012: POST /config with null config
  api POST /config -d '{"config": null}'
  assert_status 400 "S7-NEG-012: POST /config null config → 400"

  # S7-NEG-020: GET /requests/{id} nonexistent
  api GET /requests/nonexistent-id-12345
  assert_status 404 "S7-NEG-020: GET /requests/{id} nonexistent → 404"
}

# ─── S9: HTTP Protocol Edge Cases ────────────────────────────────────────────

run_s9_http() {
  suite_header "S9: HTTP Edge Cases"

  # Create a test mock first
  api POST /mocks -d '{
    "type": "http",
    "name": "Edge Case Test",
    "http": {
      "matcher": {"method": "POST", "path": "/api/echo", "bodyContains": "hello"},
      "response": {"statusCode": 200, "body": "{\"echoed\": true}"}
    }
  }'
  local mock_id
  mock_id=$(echo "$BODY" | jq -r '.id')

  # Create a GET mock with priority
  api POST /mocks -d '{
    "type": "http",
    "name": "Low Priority",
    "http": {
      "priority": 1,
      "matcher": {"method": "GET", "path": "/api/priority"},
      "response": {"statusCode": 200, "body": "{\"priority\": \"low\"}"}
    }
  }'

  api POST /mocks -d '{
    "type": "http",
    "name": "High Priority",
    "http": {
      "priority": 100,
      "matcher": {"method": "GET", "path": "/api/priority"},
      "response": {"statusCode": 200, "body": "{\"priority\": \"high\"}"}
    }
  }'

  # S9-HTTP-001: No Content-Type still matches body
  BODY=""; STATUS=""
  engine POST /api/echo -H 'Content-Type: ' -d 'hello world'
  assert_status 200 "S9-HTTP-001: No Content-Type matches body"

  # S9-HTTP-009: Unicode in path and body
  api POST /mocks -d '{
    "type": "http",
    "name": "Unicode",
    "http": {
      "matcher": {"method": "GET", "path": "/api/emoji"},
      "response": {"statusCode": 200, "body": "{\"emoji\": \"🎉\"}"}
    }
  }'
  engine GET /api/emoji
  assert_status 200 "S9-HTTP-009a: Unicode mock created"
  assert_body_contains "🎉" "S9-HTTP-009b: Unicode in response body"

  # S9-HTTP-014: Higher priority mock wins
  engine GET /api/priority
  assert_json_field '.priority' 'high' "S9-HTTP-014: Higher priority wins"

  # Cleanup
  api DELETE /mocks
}

# ─── S9: Stateful CRUD Edge Cases ────────────────────────────────────────────

run_s9_crud() {
  suite_header "S9: Stateful CRUD Edge Cases"

  # Import a stateful resource
  api POST /config -d '{
    "config": {
      "version": "1.0",
      "name": "crud-test",
      "mocks": [],
      "statefulResources": [{
        "name": "items",
        "basePath": "/api/items",
        "idField": "id",
        "seedData": [
          {"id": "1", "name": "Alpha", "score": 10},
          {"id": "2", "name": "Beta", "score": 20}
        ]
      }]
    }
  }'
  assert_status 200 "S9-CRUD-SETUP: Import stateful resource"

  # S9-CRUD-001: PATCH nonexistent → 404
  engine PATCH /api/items/nonexistent -d '{"name":"updated"}'
  assert_status 404 "S9-CRUD-001: PATCH nonexistent → 404"

  # S9-CRUD-002: POST duplicate ID → 409
  engine POST /api/items -d '{"id":"1","name":"Duplicate"}'
  assert_status 409 "S9-CRUD-002: POST duplicate ID → 409"

  # S9-CRUD-003: DELETE nonexistent → 404
  engine DELETE /api/items/nonexistent
  assert_status 404 "S9-CRUD-003: DELETE nonexistent → 404"

  # S9-CRUD-004: List returns seed data
  engine GET /api/items
  assert_status 200 "S9-CRUD-004: List returns 200"
  assert_json_field '.meta.total' '2' "S9-CRUD-004b: Total is 2"

  # S9-CRUD-007: Pagination beyond total → empty
  engine GET '/api/items?offset=100'
  assert_json_field '.meta.count' '0' "S9-CRUD-007: Offset beyond total → empty"

  # S9-CRUD-008: Sort by custom field
  engine GET '/api/items?sort=name&order=asc'
  assert_status 200 "S9-CRUD-008: Sort by name"
}

# ─── S10: Startup & Shutdown ─────────────────────────────────────────────────

run_s10() {
  suite_header "S10: Startup & Shutdown"

  # S10-START-001: Server started with zero mocks
  api GET /mocks
  # After any earlier cleanup, should be at zero or whatever state
  assert_status 200 "S10-START-001: Server responds (healthy)"

  # S10-START-005: Engine health
  # The engine management port should be accessible from within the container
  # but typically not exposed. We test via admin status.
  api GET /status
  assert_status 200 "S10-START-005: Status endpoint works"
  assert_json_field '.status' 'ok' "S10-START-005b: Status is ok"
}

# ─── S11: Persistent Store ───────────────────────────────────────────────────

run_s11() {
  suite_header "S11: Persistent Store"

  # Note: We run with --no-persist, so store ops should still work
  # (in-memory mode). This tests the no-persist path doesn't crash.

  # Create a mock — should work even without file persistence
  api POST /mocks -d '{
    "type": "http",
    "name": "Persist Test",
    "http": {
      "matcher": {"method": "GET", "path": "/api/persist-test"},
      "response": {"statusCode": 200, "body": "ok"}
    }
  }'
  assert_status 201 "S11-STORE-001: Create mock in no-persist mode"

  # Verify it's listed
  api GET /mocks
  local count
  count=$(echo "$BODY" | jq '.total' 2>/dev/null)
  if [[ "$count" -ge 1 ]]; then
    pass "S11-STORE-001b: Mock visible after create"
  else
    fail "S11-STORE-001b" "Mock not found after create"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── S15: Chaos Injection ────────────────────────────────────────────────────

run_s15() {
  suite_header "S15: Chaos Injection"

  # Create a test mock
  api POST /mocks -d '{
    "type": "http",
    "name": "Chaos Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/chaos-target"},
      "response": {"statusCode": 200, "body": "{\"ok\": true}"}
    }
  }'

  # S15-NEG-003: Chaos disabled → no faults
  engine GET /api/chaos-target
  assert_status 200 "S15-NEG-003: No chaos by default"

  # Enable chaos with latency
  api PUT /chaos -d '{
    "enabled": true,
    "global": {
      "latency": {
        "min": "1ms",
        "max": "5ms",
        "probability": 1.0
      }
    }
  }'
  assert_status 200 "S15-SETUP: Enable chaos"

  # Verify chaos is active
  api GET /chaos
  assert_status 200 "S15-STATS-001: GET /chaos returns config"
  assert_json_field '.enabled' 'true' "S15-STATS-001b: Chaos is enabled"

  # Disable chaos
  api PUT /chaos -d '{"enabled": false}'
  assert_status 200 "S15-CLEANUP: Disable chaos"

  # Cleanup
  api DELETE /mocks
}

# ─── S16: Mock Verification ──────────────────────────────────────────────────

run_s16() {
  suite_header "S16: Mock Verification"

  # Create a mock
  api POST /mocks -d '{
    "type": "http",
    "name": "Verify Me",
    "http": {
      "matcher": {"method": "GET", "path": "/api/verify-me"},
      "response": {"statusCode": 200, "body": "hello"}
    }
  }'
  local mock_id
  mock_id=$(echo "$BODY" | jq -r '.id')

  # Hit the mock twice
  engine GET /api/verify-me
  engine GET /api/verify-me

  # S16-VER-001: Check invocation count
  api GET "/mocks/${mock_id}/verify"
  assert_status 200 "S16-VER-001: GET /mocks/{id}/verify returns 200"

  # S16-VER-003: Invocation history
  api GET "/mocks/${mock_id}/invocations"
  assert_status 200 "S16-VER-003: GET /mocks/{id}/invocations returns 200"

  # S16-VER-004: Reset invocations
  api DELETE "/mocks/${mock_id}/invocations"
  assert_status 200 "S16-VER-004: DELETE /mocks/{id}/invocations returns 200"

  # Cleanup
  api DELETE /mocks
}

# ─── S1: Folder Management ───────────────────────────────────────────────────

run_s1() {
  suite_header "S1: Folder Management"

  # S1-FOLD-001: List empty
  api GET /folders
  assert_status 200 "S1-FOLD-001: GET /folders returns 200"

  # S1-FOLD-002: Create folder
  api POST /folders -d '{"name": "Test Folder", "description": "For testing"}'
  assert_status 201 "S1-FOLD-002: POST /folders creates folder"
  local folder_id
  folder_id=$(echo "$BODY" | jq -r '.id')

  # S1-FOLD-003: Get folder
  api GET "/folders/${folder_id}"
  assert_status 200 "S1-FOLD-003: GET /folders/{id} returns folder"
  assert_json_field '.name' 'Test Folder' "S1-FOLD-003b: Name matches"

  # S1-FOLD-004: Update folder
  api PUT "/folders/${folder_id}" -d '{"name": "Renamed Folder"}'
  assert_status 200 "S1-FOLD-004: PUT /folders/{id} updates folder"

  # S1-FOLD-005: Delete folder
  api DELETE "/folders/${folder_id}"
  assert_status 204 "S1-FOLD-005: DELETE /folders/{id} removes folder"

  # S1-FOLD-006: Delete nonexistent → 404
  api DELETE "/folders/${folder_id}"
  assert_status 404 "S1-FOLD-006: DELETE nonexistent → 404"
}

# ─── S5: Metadata & Export ───────────────────────────────────────────────────

run_s5() {
  suite_header "S5: Metadata & Export"

  # S5-META-001: List formats
  api GET /formats
  assert_status 200 "S5-META-001: GET /formats returns 200"

  # S5-META-002: List templates
  api GET /templates
  assert_status 200 "S5-META-002: GET /templates returns 200"

  # S5-EXP-003: OpenAPI export
  api GET /openapi.json
  assert_status 200 "S5-EXP-003: GET /openapi.json returns 200"

  # S5-EXP-004: OpenAPI YAML export
  api GET /openapi.yaml
  assert_status 200 "S5-EXP-004: GET /openapi.yaml returns 200"

  # S5-PREF-001: Get preferences
  api GET /preferences
  assert_status 200 "S5-PREF-001: GET /preferences returns 200"
}

# ─── S14: Concurrency ────────────────────────────────────────────────────────

run_s14() {
  suite_header "S14: Concurrency"

  # Create a target mock
  api POST /mocks -d '{
    "type": "http",
    "name": "Concurrent Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/concurrent"},
      "response": {"statusCode": 200, "body": "{\"ok\": true}"}
    }
  }'

  # S14-CONC-003: Concurrent requests to same mock
  local pids=()
  local concurrent_pass=true
  for i in $(seq 1 10); do
    (
      local resp
      resp=$(curl -s -o /dev/null -w '%{http_code}' "${ENGINE}/api/concurrent")
      if [[ "$resp" != "200" ]]; then
        exit 1
      fi
    ) &
    pids+=($!)
  done

  for pid in "${pids[@]}"; do
    wait "$pid" || concurrent_pass=false
  done

  if $concurrent_pass; then
    pass "S14-CONC-003: 10 concurrent requests all returned 200"
  else
    fail "S14-CONC-003" "Some concurrent requests failed"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── Import/Export Round-Trip ────────────────────────────────────────────────

run_import_export() {
  suite_header "Import/Export Round-Trip"

  # Import a collection
  api POST /config -d '{
    "config": {
      "version": "1.0",
      "name": "roundtrip-test",
      "mocks": [
        {
          "id": "rt-mock-1",
          "type": "http",
          "name": "Roundtrip Mock",
          "http": {
            "matcher": {"method": "GET", "path": "/api/roundtrip"},
            "response": {"statusCode": 200, "body": "{\"imported\": true}"}
          }
        }
      ],
      "statefulResources": [
        {
          "name": "rt-users",
          "basePath": "/api/rt-users",
          "idField": "id",
          "seedData": [{"id": "1", "name": "Test User"}]
        }
      ]
    }
  }'
  assert_status 200 "IMPORT-001: Import collection"
  assert_json_field '.imported' '1' "IMPORT-002: 1 mock imported"
  assert_json_field '.statefulResources' '1' "IMPORT-003: 1 stateful resource imported"

  # Verify mock works
  engine GET /api/roundtrip
  assert_status 200 "IMPORT-004: Imported mock responds"
  assert_json_field '.imported' 'true' "IMPORT-005: Response body correct"

  # Verify stateful resource works
  engine GET /api/rt-users
  assert_status 200 "IMPORT-006: Stateful resource responds"
  assert_json_field '.meta.total' '1' "IMPORT-007: Seed data loaded"

  # Export
  api GET /config
  assert_status 200 "EXPORT-001: Export returns 200"
  assert_body_contains "roundtrip" "EXPORT-002: Export contains imported mock"

  # Cleanup
  api DELETE /mocks
}

# ─── Main ────────────────────────────────────────────────────────────────────

main() {
  log "mockd E2E Test Runner"
  log "Admin:  $ADMIN"
  log "Engine: $ENGINE"
  log "Suites: $SUITES"
  echo ""

  # Always run smoke first
  run_smoke

  should_run "s1"    && run_s1
  should_run "s5"    && run_s5
  should_run "s7"    && run_s7
  should_run "s9"    && { run_s9_http; run_s9_crud; }
  should_run "s10"   && run_s10
  should_run "s11"   && run_s11
  should_run "s14"   && run_s14
  should_run "s15"   && run_s15
  should_run "s16"   && run_s16
  should_run "import" && run_import_export

  # ─── Summary ─────────────────────────────────────────────────────────────
  echo ""
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  log "Results: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo ""
    log "FAILURES:"
    for err in "${ERRORS[@]}"; do
      log "  • $err"
    done
  fi

  # Write results file
  {
    echo "mockd E2E Test Results — $(date)"
    echo "Passed: $PASS"
    echo "Failed: $FAIL"
    echo "Skipped: $SKIP"
    if [[ ${#ERRORS[@]} -gt 0 ]]; then
      echo ""
      echo "Failures:"
      for err in "${ERRORS[@]}"; do
        echo "  - $err"
      done
    fi
  } > "$RESULTS_FILE"

  if [[ $FAIL -gt 0 ]]; then
    exit 1
  fi
  exit 0
}

main "$@"
