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

# ─── Helpers ─────────────────────────────────────────────────────────────────

log()  { echo "$(date +%H:%M:%S) $*"; }
pass() { PASS=$((PASS + 1)); log "  ✓ $1"; }
fail() { FAIL=$((FAIL + 1)); ERRORS+=("$1: $2"); log "  ✗ $1 — $2"; }
skip() { SKIP=$((SKIP + 1)); log "  ⊘ $1 (skipped)"; }
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
  BODY=$(echo "$resp" | sed '$d')
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
  BODY=$(echo "$resp" | sed '$d')
  STATUS=$(echo "$resp" | tail -n 1)
}

assert_status() {
  local expected="$1" test_id="$2"
  if [[ "$STATUS" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected HTTP $expected, got $STATUS (body: $(echo "$BODY" | head -c 200))"
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

assert_json_field_gt() {
  local field="$1" threshold="$2" test_id="$3"
  local actual
  actual=$(echo "$BODY" | jq -r "$field" 2>/dev/null) || actual="0"
  if [[ "$actual" -gt "$threshold" ]] 2>/dev/null; then
    pass "$test_id"
  else
    fail "$test_id" "expected $field > $threshold, got $actual"
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
  assert_json_field '.status' 'ok' "SMOKE-002: Health status is ok"

  api GET /status
  assert_status 200 "SMOKE-003: Admin status"

  api GET /mocks
  assert_status 200 "SMOKE-004: List mocks (empty)"
  assert_json_field '.total' '0' "SMOKE-005: Zero mocks initially"
}

# ─── S7: Admin API Negative Tests (P0) ──────────────────────────────────────

run_s7() {
  suite_header "S7: Admin API Negative Tests"

  # POST /mocks with invalid JSON
  api POST /mocks -d 'not json'
  assert_status 400 "S7-NEG-001: POST /mocks invalid JSON → 400"

  # GET /mocks/{id} nonexistent
  api GET /mocks/nonexistent-id-12345
  assert_status 404 "S7-NEG-005: GET /mocks/{id} nonexistent → 404"

  # PUT /mocks/{id} nonexistent
  api PUT /mocks/nonexistent-id-12345 -d '{"name":"test","type":"http"}'
  assert_status 404 "S7-NEG-006: PUT /mocks/{id} nonexistent → 404"

  # DELETE /mocks/{id} nonexistent
  api DELETE /mocks/nonexistent-id-12345
  assert_status 404 "S7-NEG-007: DELETE /mocks/{id} nonexistent → 404"

  # POST /mocks with empty body
  api POST /mocks -d ''
  assert_status 400 "S7-NEG-010: POST /mocks empty body → 400"

  # POST /config with invalid JSON
  api POST /config -d 'not json'
  assert_status 400 "S7-NEG-011: POST /config invalid JSON → 400"

  # POST /config with null config
  api POST /config -d '{"config": null}'
  assert_status 400 "S7-NEG-012: POST /config null config → 400"

  # GET /requests/{id} nonexistent
  api GET /requests/nonexistent-id-12345
  assert_status 404 "S7-NEG-020: GET /requests/{id} nonexistent → 404"

  # POST /mocks/{id}/toggle nonexistent
  api POST /mocks/nonexistent-id-12345/toggle -d '{"enabled": true}'
  assert_status 404 "S7-NEG-030: POST /mocks/{id}/toggle nonexistent → 404"

  # DELETE /folders/{id} nonexistent
  api DELETE /folders/nonexistent-folder
  assert_status 404 "S7-NEG-040: DELETE /folders/{id} nonexistent → 404"

  # GET /state/resources/{name} nonexistent
  api GET /state/resources/nonexistent-resource
  assert_status 404 "S7-NEG-050: GET /state/resources/{name} nonexistent → 404"
}

# ─── S9: HTTP Protocol Edge Cases ────────────────────────────────────────────

run_s9_http() {
  suite_header "S9: HTTP Edge Cases"

  # Create a body-matching mock
  api POST /mocks -d '{
    "type": "http",
    "name": "Body Matcher",
    "http": {
      "matcher": {"method": "POST", "path": "/api/echo", "bodyContains": "hello"},
      "response": {"statusCode": 200, "body": "{\"echoed\": true}"}
    }
  }'
  assert_status 201 "S9-HTTP-SETUP-1: Create body matcher mock"

  # Create priority mocks
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

  # Unicode in response
  api POST /mocks -d '{
    "type": "http",
    "name": "Unicode",
    "http": {
      "matcher": {"method": "GET", "path": "/api/emoji"},
      "response": {"statusCode": 200, "body": "{\"emoji\": \"🎉\"}"}
    }
  }'

  # Test: body matching
  engine POST /api/echo -d 'hello world'
  assert_status 200 "S9-HTTP-001: Body matching works"

  # Test: Unicode response
  engine GET /api/emoji
  assert_status 200 "S9-HTTP-009a: Unicode mock responds"
  assert_body_contains "🎉" "S9-HTTP-009b: Unicode in response body"

  # Test: higher priority wins
  engine GET /api/priority
  assert_json_field '.priority' 'high' "S9-HTTP-014: Higher priority wins"

  # Test: Unmatched request returns 404 (no mock)
  engine GET /api/no-such-endpoint
  assert_status 404 "S9-HTTP-020: Unmatched path → 404"

  # Cleanup
  api DELETE /mocks
  assert_status 204 "S9-HTTP-CLEANUP: Delete all mocks"
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

  # CRUD operations
  engine GET /api/items
  assert_status 200 "S9-CRUD-001: List returns 200"
  assert_json_field '.meta.total' '2' "S9-CRUD-001b: Total is 2"

  engine GET /api/items/1
  assert_status 200 "S9-CRUD-002: Get by ID returns 200"
  assert_json_field '.name' 'Alpha' "S9-CRUD-002b: Name is Alpha"

  # POST duplicate ID → 409
  engine POST /api/items -d '{"id":"1","name":"Duplicate"}'
  assert_status 409 "S9-CRUD-003: POST duplicate ID → 409"

  # DELETE nonexistent → 404
  engine DELETE /api/items/nonexistent
  assert_status 404 "S9-CRUD-004: DELETE nonexistent → 404"

  # PATCH is not supported by stateful resources → 405
  engine PATCH /api/items/nonexistent -d '{"name":"updated"}'
  assert_status 405 "S9-CRUD-005: PATCH → 405 (not supported)"

  # Pagination beyond total → empty
  engine GET '/api/items?offset=100'
  assert_json_field '.meta.count' '0' "S9-CRUD-006: Offset beyond total → empty"

  # Create new item
  engine POST /api/items -d '{"name":"Gamma","score":30}'
  assert_status 201 "S9-CRUD-007: POST new item → 201"

  engine GET /api/items
  assert_json_field '.meta.total' '3' "S9-CRUD-007b: Total is now 3"

  # Update item
  engine PUT /api/items/1 -d '{"name":"Alpha Updated","score":100}'
  assert_status 200 "S9-CRUD-008: PUT update → 200"

  engine GET /api/items/1
  assert_json_field '.name' 'Alpha Updated' "S9-CRUD-008b: Name updated"

  # Delete item
  engine DELETE /api/items/1
  assert_status 204 "S9-CRUD-009: DELETE item → 204"

  engine GET /api/items/1
  assert_status 404 "S9-CRUD-009b: Deleted item → 404"
}

# ─── S10: Startup & Shutdown ─────────────────────────────────────────────────

run_s10() {
  suite_header "S10: Startup & Shutdown"

  api GET /health
  assert_status 200 "S10-001: Health endpoint responsive"
  assert_json_field '.status' 'ok' "S10-001b: Status ok"

  api GET /status
  assert_status 200 "S10-002: Status endpoint works"

  # Engine management port may not be exposed, test via admin
  api GET /ports
  assert_status 200 "S10-003: Ports endpoint works"
}

# ─── S11: Persistent Store ───────────────────────────────────────────────────

run_s11() {
  suite_header "S11: Persistent Store"

  # Create a mock
  api POST /mocks -d '{
    "type": "http",
    "name": "Persist Test",
    "http": {
      "matcher": {"method": "GET", "path": "/api/persist-test"},
      "response": {"statusCode": 200, "body": "ok"}
    }
  }'
  assert_status 201 "S11-001: Create mock"

  # Verify it's listed
  api GET /mocks
  local count
  count=$(echo "$BODY" | jq '.total' 2>/dev/null) || count=0
  if [[ "$count" -ge 1 ]]; then
    pass "S11-001b: Mock visible after create"
  else
    fail "S11-001b" "Mock not found after create (total=$count)"
  fi

  # Cleanup
  api DELETE /mocks
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

  # 10 concurrent requests
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
    pass "S14-001: 10 concurrent requests all returned 200"
  else
    fail "S14-001" "Some concurrent requests failed"
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

  # No chaos by default
  engine GET /api/chaos-target
  assert_status 200 "S15-001: No chaos by default"

  # Enable chaos
  api PUT /chaos -d '{
    "enabled": true,
    "latency": {
      "min": "1ms",
      "max": "5ms",
      "probability": 1.0
    }
  }'
  assert_status 200 "S15-002: Enable chaos"

  # Verify chaos is active
  api GET /chaos
  assert_status 200 "S15-003: GET /chaos returns config"
  assert_json_field '.enabled' 'true' "S15-003b: Chaos is enabled"

  # Disable chaos
  api PUT /chaos -d '{"enabled": false}'
  assert_status 200 "S15-004: Disable chaos"

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

  # Check invocation count
  api GET "/mocks/${mock_id}/verify"
  assert_status 200 "S16-001: GET /mocks/{id}/verify returns 200"

  # Invocation history
  api GET "/mocks/${mock_id}/invocations"
  assert_status 200 "S16-002: GET /mocks/{id}/invocations returns 200"

  # Reset invocations
  api DELETE "/mocks/${mock_id}/invocations"
  assert_status 200 "S16-003: DELETE /mocks/{id}/invocations returns 200"

  # Cleanup
  api DELETE /mocks
}

# ─── S1: Folder Management ───────────────────────────────────────────────────

run_s1() {
  suite_header "S1: Folder Management"

  # List empty
  api GET /folders
  assert_status 200 "S1-001: GET /folders returns 200"

  # Create folder
  api POST /folders -d '{"name": "Test Folder", "description": "For testing"}'
  assert_status 201 "S1-002: POST /folders creates folder"
  local folder_id
  folder_id=$(echo "$BODY" | jq -r '.id')

  # Get folder
  api GET "/folders/${folder_id}"
  assert_status 200 "S1-003: GET /folders/{id} returns folder"
  assert_json_field '.name' 'Test Folder' "S1-003b: Name matches"

  # Update folder
  api PUT "/folders/${folder_id}" -d '{"name": "Renamed Folder"}'
  assert_status 200 "S1-004: PUT /folders/{id} updates folder"

  # Delete folder
  api DELETE "/folders/${folder_id}"
  assert_status 204 "S1-005: DELETE /folders/{id} removes folder"

  # Delete nonexistent → 404
  api DELETE "/folders/${folder_id}"
  assert_status 404 "S1-006: DELETE nonexistent → 404"
}

# ─── S5: Metadata & Export ───────────────────────────────────────────────────

run_s5() {
  suite_header "S5: Metadata & Export"

  api GET /formats
  assert_status 200 "S5-001: GET /formats returns 200"

  api GET /templates
  assert_status 200 "S5-002: GET /templates returns 200"

  api GET /openapi.json
  assert_status 200 "S5-003: GET /openapi.json returns 200"

  api GET /openapi.yaml
  assert_status 200 "S5-004: GET /openapi.yaml returns 200"

  # Note: /preferences requires persistent storage, returns 501 without it
  api GET /preferences
  # Accept 200 or 501 — depends on storage config
  if [[ "$STATUS" == "200" || "$STATUS" == "501" ]]; then
    pass "S5-005: GET /preferences responds"
  else
    fail "S5-005" "expected HTTP 200 or 501, got $STATUS"
  fi
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
  assert_status 200 "IMP-001: Import collection"
  assert_json_field '.imported' '1' "IMP-002: 1 mock imported"
  assert_json_field '.statefulResources' '1' "IMP-003: 1 stateful resource imported"

  # Verify mock works
  engine GET /api/roundtrip
  assert_status 200 "IMP-004: Imported mock responds"
  assert_json_field '.imported' 'true' "IMP-005: Response body correct"

  # Verify stateful resource works
  engine GET /api/rt-users
  assert_status 200 "IMP-006: Stateful resource responds"
  assert_json_field '.meta.total' '1' "IMP-007: Seed data loaded"

  # Export
  api GET /config
  assert_status 200 "EXP-001: Export returns 200"

  # Cleanup
  api DELETE /mocks
}

# ─── S3: Protocol Handler Management ─────────────────────────────────────────

run_s3() {
  suite_header "S3: Protocol Handler Management"

  api GET /handlers
  assert_status 200 "S3-001: GET /handlers returns 200"
}

# ─── S6: Recordings ──────────────────────────────────────────────────────────

run_s6() {
  suite_header "S6: Recordings & Stream Recordings"

  # HTTP recordings (proxy recordings — may be empty)
  api GET /recordings
  assert_status 200 "S6-001: GET /recordings returns 200"

  # Stream recordings
  api GET /stream-recordings
  assert_status 200 "S6-002: GET /stream-recordings returns 200"

  api GET /stream-recordings/stats
  assert_status 200 "S6-003: GET /stream-recordings/stats returns 200"

  # Replay sessions
  api GET /replay
  assert_status 200 "S6-004: GET /replay returns 200"

  # MQTT recordings
  api GET /mqtt-recordings
  assert_status 200 "S6-005: GET /mqtt-recordings returns 200"

  # SOAP recordings
  api GET /soap-recordings
  assert_status 200 "S6-006: GET /soap-recordings returns 200"
}

# ─── S2: Session Management ──────────────────────────────────────────────────

run_s2() {
  suite_header "S2: Session Management"

  api GET /sessions
  assert_status 200 "S2-001: GET /sessions returns 200"
}

# ─── S17: Template Engine ────────────────────────────────────────────────────

run_s17() {
  suite_header "S17: Template Engine"

  # Create a mock with template response
  api POST /mocks -d '{
    "type": "http",
    "name": "Template Test",
    "http": {
      "matcher": {"method": "GET", "path": "/api/template"},
      "response": {
        "statusCode": 200,
        "body": "{\"uuid\": \"{{uuid}}\", \"timestamp\": \"{{now}}\"}"
      }
    }
  }'
  assert_status 201 "S17-001: Create template mock"

  engine GET /api/template
  assert_status 200 "S17-002: Template mock responds"
  # UUID should not literally be "{{uuid}}"
  local uuid_val
  uuid_val=$(echo "$BODY" | jq -r '.uuid' 2>/dev/null) || uuid_val=""
  if [[ "$uuid_val" != '{{uuid}}' && -n "$uuid_val" && "$uuid_val" != "null" ]]; then
    pass "S17-003: Template uuid expanded"
  else
    fail "S17-003" "Template not expanded: uuid=$uuid_val"
  fi

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

  should_run "s1"     && run_s1
  should_run "s2"     && run_s2
  should_run "s3"     && run_s3
  should_run "s5"     && run_s5
  should_run "s6"     && run_s6
  should_run "s7"     && run_s7
  should_run "s9"     && { run_s9_http; run_s9_crud; }
  should_run "s10"    && run_s10
  should_run "s11"    && run_s11
  should_run "s14"    && run_s14
  should_run "s15"    && run_s15
  should_run "s16"    && run_s16
  should_run "s17"    && run_s17
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

  echo ""
  if [[ $FAIL -gt 0 ]]; then
    log "EXIT: FAIL"
    exit 1
  fi
  log "EXIT: PASS"
  exit 0
}

main "$@"
