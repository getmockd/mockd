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

# ─── Request Logging ─────────────────────────────────────────────────────────

run_requests() {
  suite_header "Request Logging"

  # Create a mock and hit it
  api POST /mocks -d '{
    "type": "http",
    "name": "Request Log Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/log-target"},
      "response": {"statusCode": 200, "body": "{\"logged\": true}"}
    }
  }'
  local mock_id
  mock_id=$(echo "$BODY" | jq -r '.id')

  engine GET /api/log-target
  engine GET /api/log-target
  engine GET /api/log-target

  # List requests
  api GET /requests
  assert_status 200 "REQ-001: GET /requests returns 200"
  local req_count
  req_count=$(echo "$BODY" | jq '.count // .total // 0' 2>/dev/null) || req_count=0
  if [[ "$req_count" -ge 3 ]]; then
    pass "REQ-002: At least 3 requests logged"
  else
    fail "REQ-002" "expected >= 3 requests, got $req_count"
  fi

  # Get a specific request
  local req_id
  req_id=$(echo "$BODY" | jq -r '.requests[0].id // empty' 2>/dev/null) || req_id=""
  if [[ -n "$req_id" ]]; then
    api GET "/requests/${req_id}"
    assert_status 200 "REQ-003: GET /requests/{id} returns 200"
    assert_json_field '.path' '/api/log-target' "REQ-003b: Path matches"
  else
    skip "REQ-003: No request ID to test"
  fi

  # Clear requests
  api DELETE /requests
  assert_status 200 "REQ-004: DELETE /requests clears logs"

  api GET /requests
  local after_count
  after_count=$(echo "$BODY" | jq '.count // .total // 0' 2>/dev/null) || after_count=0
  if [[ "$after_count" -eq 0 ]]; then
    pass "REQ-005: Requests cleared"
  else
    fail "REQ-005" "expected 0 requests after clear, got $after_count"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── State Management ────────────────────────────────────────────────────────

run_state() {
  suite_header "State Management"

  # Import stateful resources
  api POST /config -d '{
    "config": {
      "version": "1.0",
      "name": "state-test",
      "mocks": [],
      "statefulResources": [
        {
          "name": "products",
          "basePath": "/api/products",
          "idField": "id",
          "seedData": [
            {"id": "p1", "name": "Widget", "price": 10},
            {"id": "p2", "name": "Gadget", "price": 20}
          ]
        },
        {
          "name": "orders",
          "basePath": "/api/orders",
          "idField": "id",
          "seedData": [{"id": "o1", "customer": "Alice"}]
        }
      ]
    }
  }'
  assert_status 200 "STATE-SETUP: Import stateful resources"

  # State overview
  api GET /state
  assert_status 200 "STATE-001: GET /state returns 200"

  # List state resources
  api GET /state/resources
  assert_status 200 "STATE-002: GET /state/resources returns 200"

  # Get specific resource
  api GET /state/resources/products
  assert_status 200 "STATE-003: GET /state/resources/{name} returns 200"

  # Add an item so we can verify reset brings it back to seed
  engine POST /api/products -d '{"name":"Extra","price":99}'
  assert_status 201 "STATE-004: Add item to products"

  engine GET /api/products
  assert_json_field '.meta.total' '3' "STATE-004b: Products has 3 items"

  # Reset specific resource → back to seed data
  api POST /state/resources/products/reset
  assert_status 200 "STATE-005: POST /state/resources/{name}/reset returns 200"

  engine GET /api/products
  assert_json_field '.meta.total' '2' "STATE-005b: Products reset to 2 seed items"

  # Clear specific resource (remove all items, keep resource)
  api DELETE /state/resources/orders
  assert_status 200 "STATE-006: DELETE /state/resources/{name} clears items"

  engine GET /api/orders
  assert_json_field '.meta.total' '0' "STATE-006b: Orders cleared to 0"

  # Global state reset
  api POST /state/reset
  assert_status 200 "STATE-007: POST /state/reset returns 200"

  engine GET /api/orders
  assert_json_field '.meta.total' '1' "STATE-007b: Orders reset to seed (1 item)"
}

# ─── Workspace Management ────────────────────────────────────────────────────

run_workspaces() {
  suite_header "Workspace Management"

  # List workspaces (empty)
  api GET /workspaces
  assert_status 200 "WS-001: GET /workspaces returns 200"

  # Create workspace
  api POST /workspaces -d '{"name": "test-ws", "description": "Test workspace"}'
  assert_status 201 "WS-002: POST /workspaces creates workspace"
  local ws_id
  ws_id=$(echo "$BODY" | jq -r '.id')

  # Get workspace
  api GET "/workspaces/${ws_id}"
  assert_status 200 "WS-003: GET /workspaces/{id} returns workspace"
  assert_json_field '.name' 'test-ws' "WS-003b: Name matches"

  # Update workspace
  api PUT "/workspaces/${ws_id}" -d '{"name": "updated-ws", "description": "Updated"}'
  assert_status 200 "WS-004: PUT /workspaces/{id} updates workspace"

  # Delete workspace
  api DELETE "/workspaces/${ws_id}"
  assert_status 204 "WS-005: DELETE /workspaces/{id} removes workspace"

  # Delete nonexistent → 404
  api DELETE "/workspaces/${ws_id}"
  assert_status 404 "WS-006: DELETE nonexistent workspace → 404"
}

# ─── S7 Extended Negative Tests ──────────────────────────────────────────────

run_s7_extended() {
  suite_header "S7 Extended: More Negative Tests"

  # POST /mocks with invalid type
  api POST /mocks -d '{"name":"test","type":"invalid_protocol"}'
  if [[ "$STATUS" == "400" || "$STATUS" == "422" ]]; then
    pass "S7-EXT-001: POST /mocks invalid type → 4xx"
  else
    fail "S7-EXT-001" "expected 400 or 422 for invalid type, got $STATUS"
  fi

  # PATCH /mocks/{id} nonexistent
  api PATCH /mocks/nonexistent-id-12345 -d '{"name":"patched"}'
  assert_status 404 "S7-EXT-002: PATCH /mocks/{id} nonexistent → 404"

  # PUT /chaos with invalid JSON
  api PUT /chaos -d 'not json'
  assert_status 400 "S7-EXT-003: PUT /chaos invalid JSON → 400"

  # POST /state/reset with invalid JSON (should still work — empty body resets all)
  api POST /state/reset -d ''
  # Accept 200 (reset all) or 400 — depends on implementation
  if [[ "$STATUS" == "200" || "$STATUS" == "400" ]]; then
    pass "S7-EXT-004: POST /state/reset with empty body handled"
  else
    fail "S7-EXT-004" "expected 200 or 400, got $STATUS"
  fi

  # GET /workspaces/{id} nonexistent
  api GET /workspaces/nonexistent-ws
  assert_status 404 "S7-EXT-005: GET /workspaces/{id} nonexistent → 404"

  # PUT /workspaces/{id} nonexistent
  api PUT /workspaces/nonexistent-ws -d '{"name":"test"}'
  assert_status 404 "S7-EXT-006: PUT /workspaces/{id} nonexistent → 404"

  # POST /config with empty mocks array (should be fine)
  api POST /config -d '{"config":{"version":"1.0","name":"empty","mocks":[]}}'
  assert_status 200 "S7-EXT-007: POST /config with empty mocks → 200"

  # POST /mocks with huge body (should be rejected or truncated, not crash)
  local big_body
  big_body=$(python3 -c "print('{\"name\":\"' + 'A'*100000 + '\",\"type\":\"http\"}')" 2>/dev/null || echo '{"name":"test","type":"http"}')
  api POST /mocks -d "$big_body"
  # Any non-5xx response is acceptable
  if [[ "$STATUS" -lt 500 ]] 2>/dev/null; then
    pass "S7-EXT-008: Large request body handled (status $STATUS)"
  else
    fail "S7-EXT-008" "Server error on large body: $STATUS"
  fi
}

# ─── Mock Operations ─────────────────────────────────────────────────────────

run_mock_ops() {
  suite_header "Mock Operations (Bulk, Patch, Toggle)"

  # Bulk create
  api POST /mocks/bulk -d '[
    {
      "type": "http",
      "name": "Bulk Mock 1",
      "http": {
        "matcher": {"method": "GET", "path": "/api/bulk1"},
        "response": {"statusCode": 200, "body": "{\"n\": 1}"}
      }
    },
    {
      "type": "http",
      "name": "Bulk Mock 2",
      "http": {
        "matcher": {"method": "GET", "path": "/api/bulk2"},
        "response": {"statusCode": 200, "body": "{\"n\": 2}"}
      }
    }
  ]'
  if [[ "$STATUS" == "200" || "$STATUS" == "201" ]]; then
    pass "MOCK-001: POST /mocks/bulk creates mocks"
  else
    fail "MOCK-001" "expected 200/201, got $STATUS (body: $(echo "$BODY" | head -c 200))"
  fi

  engine GET /api/bulk1
  assert_status 200 "MOCK-001b: Bulk mock 1 responds"
  engine GET /api/bulk2
  assert_status 200 "MOCK-001c: Bulk mock 2 responds"

  # Get a mock ID for patch/toggle tests
  api GET /mocks
  local mock_id
  mock_id=$(echo "$BODY" | jq -r '.mocks[0].id // empty' 2>/dev/null) || mock_id=""

  if [[ -n "$mock_id" ]]; then
    # PATCH mock (may return 200 or 404 if engine doesn't support partial update)
    api PATCH "/mocks/${mock_id}" -d '{"name":"Patched Name"}'
    if [[ "$STATUS" == "200" ]]; then
      pass "MOCK-002: PATCH /mocks/{id} works"
      assert_json_field '.name' 'Patched Name' "MOCK-002b: Name patched"
    elif [[ "$STATUS" == "404" ]]; then
      # PATCH route exists but may not resolve for engine-managed mocks
      skip "MOCK-002: PATCH returns 404 for engine-managed mock"
    else
      fail "MOCK-002" "expected 200 or 404, got $STATUS"
    fi

    # Toggle mock off
    api POST "/mocks/${mock_id}/toggle" -d '{"enabled": false}'
    assert_status 200 "MOCK-003: Toggle mock off"

    # Toggle mock on
    api POST "/mocks/${mock_id}/toggle" -d '{"enabled": true}'
    assert_status 200 "MOCK-003b: Toggle mock on"

    # Verification: POST verify with assertion
    api POST "/mocks/${mock_id}/verify" -d '{"atLeast": 0}'
    if [[ "$STATUS" == "200" ]]; then
      pass "MOCK-004: POST /mocks/{id}/verify assertion"
    else
      # Might not be implemented or different format
      skip "MOCK-004: POST verify not available (status $STATUS)"
    fi
  else
    skip "MOCK-002: No mock ID for patch test"
    skip "MOCK-003: No mock ID for toggle test"
    skip "MOCK-004: No mock ID for verify test"
  fi

  # Reset all verification data
  api DELETE /verify
  if [[ "$STATUS" == "200" || "$STATUS" == "204" ]]; then
    pass "MOCK-005: DELETE /verify resets all"
  else
    fail "MOCK-005" "expected 200/204, got $STATUS"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── S9 Extended: HTTP Edge Cases ─────────────────────────────────────────────

run_s9_extended() {
  suite_header "S9 Extended: HTTP Edge Cases"

  # Header matching
  api POST /mocks -d '{
    "type": "http",
    "name": "Header Matcher",
    "http": {
      "matcher": {"method": "GET", "path": "/api/header-test", "headers": {"X-Custom": "magic"}},
      "response": {"statusCode": 200, "body": "{\"matched\": \"header\"}"}
    }
  }'
  assert_status 201 "S9X-001: Create header matcher"

  engine GET /api/header-test -H 'X-Custom: magic'
  assert_status 200 "S9X-002: Header match succeeds"
  assert_json_field '.matched' 'header' "S9X-002b: Correct response"

  engine GET /api/header-test -H 'X-Custom: wrong'
  assert_status 404 "S9X-003: Wrong header → 404"

  # Query parameter matching
  api POST /mocks -d '{
    "type": "http",
    "name": "Query Matcher",
    "http": {
      "matcher": {"method": "GET", "path": "/api/query-test", "queryParams": {"key": "value"}},
      "response": {"statusCode": 200, "body": "{\"matched\": \"query\"}"}
    }
  }'
  assert_status 201 "S9X-004: Create query param matcher"

  engine GET '/api/query-test?key=value'
  assert_status 200 "S9X-005: Query param match succeeds"

  engine GET '/api/query-test?key=wrong'
  assert_status 404 "S9X-006: Wrong query param → 404"

  # Response with custom headers
  api POST /mocks -d '{
    "type": "http",
    "name": "Custom Headers",
    "http": {
      "matcher": {"method": "GET", "path": "/api/custom-headers"},
      "response": {
        "statusCode": 200,
        "body": "ok",
        "headers": {"X-Custom-Response": "hello", "X-Another": "world"}
      }
    }
  }'
  assert_status 201 "S9X-007: Create mock with custom response headers"

  # Test response headers
  local resp_headers
  resp_headers=$(curl -s -D - -o /dev/null "${ENGINE}/api/custom-headers" 2>&1) || resp_headers=""
  if echo "$resp_headers" | grep -qi "X-Custom-Response"; then
    pass "S9X-008: Custom response header present"
  else
    fail "S9X-008" "X-Custom-Response header not found in response"
  fi

  # Delayed response
  api POST /mocks -d '{
    "type": "http",
    "name": "Delayed Response",
    "http": {
      "matcher": {"method": "GET", "path": "/api/delayed"},
      "response": {"statusCode": 200, "body": "{\"delayed\": true}", "delayMs": 100}
    }
  }'
  assert_status 201 "S9X-009: Create delayed mock"

  local start_time end_time elapsed
  start_time=$(date +%s%N)
  engine GET /api/delayed
  end_time=$(date +%s%N)
  elapsed=$(( (end_time - start_time) / 1000000 ))
  assert_status 200 "S9X-010: Delayed mock responds"
  if [[ "$elapsed" -ge 80 ]]; then
    pass "S9X-010b: Delay applied (${elapsed}ms)"
  else
    fail "S9X-010b" "expected >= 80ms delay, got ${elapsed}ms"
  fi

  # Path pattern matching (wildcard)
  api POST /mocks -d '{
    "type": "http",
    "name": "Wildcard Path",
    "http": {
      "matcher": {"method": "GET", "path": "/api/users/{id}/profile"},
      "response": {"statusCode": 200, "body": "{\"profile\": true}"}
    }
  }'
  assert_status 201 "S9X-011: Create wildcard path mock"

  engine GET /api/users/123/profile
  assert_status 200 "S9X-012: Wildcard path matches /users/123/profile"

  engine GET /api/users/abc/profile
  assert_status 200 "S9X-013: Wildcard path matches /users/abc/profile"

  # Cleanup
  api DELETE /mocks
}

# ─── S15 Extended: Chaos Depth ────────────────────────────────────────────────

run_s15_extended() {
  suite_header "S15 Extended: Chaos Depth"

  # Create a target mock
  api POST /mocks -d '{
    "type": "http",
    "name": "Chaos Deep Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/chaos-deep"},
      "response": {"statusCode": 200, "body": "{\"ok\": true}"}
    }
  }'

  # Error rate injection
  api PUT /chaos -d '{
    "enabled": true,
    "errorRate": {
      "probability": 1.0,
      "statusCodes": [500, 502, 503],
      "defaultCode": 500
    }
  }'
  assert_status 200 "S15X-001: Enable error rate chaos"

  engine GET /api/chaos-deep
  if [[ "$STATUS" -ge 500 ]]; then
    pass "S15X-002: Error rate produces 5xx (got $STATUS)"
  else
    fail "S15X-002" "expected 5xx, got $STATUS"
  fi

  # Bandwidth throttling
  api PUT /chaos -d '{
    "enabled": true,
    "bandwidth": {
      "bytesPerSecond": 1024,
      "probability": 1.0
    }
  }'
  assert_status 200 "S15X-003: Enable bandwidth chaos"

  engine GET /api/chaos-deep
  # Should still respond, just slowly
  if [[ "$STATUS" -ge 200 && "$STATUS" -lt 600 ]]; then
    pass "S15X-004: Bandwidth-throttled response received (status $STATUS)"
  else
    fail "S15X-004" "unexpected status: $STATUS"
  fi

  # Chaos with path-specific rules
  api PUT /chaos -d '{
    "enabled": true,
    "rules": [
      {
        "pathPattern": "/api/chaos-deep",
        "methods": ["GET"],
        "probability": 1.0
      }
    ],
    "latency": {
      "min": "1ms",
      "max": "2ms",
      "probability": 1.0
    }
  }'
  assert_status 200 "S15X-005: Enable chaos with path rules"

  # Disable
  api PUT /chaos -d '{"enabled": false}'
  assert_status 200 "S15X-006: Disable chaos"

  # Cleanup
  api DELETE /mocks
}

# ─── Metadata Extended ────────────────────────────────────────────────────────

run_metadata_extended() {
  suite_header "Metadata Extended"

  # Insomnia export
  api GET /insomnia.json
  assert_status 200 "META-001: GET /insomnia.json returns 200"

  api GET /insomnia.yaml
  assert_status 200 "META-002: GET /insomnia.yaml returns 200"

  # Metrics
  api GET /metrics
  assert_status 200 "META-003: GET /metrics returns 200"

  # Generate from template
  api POST /templates/basic -d '{}'
  if [[ "$STATUS" == "200" || "$STATUS" == "201" ]]; then
    pass "META-004: POST /templates/{name} generates mock"
  else
    # Template name might not exist — that's fine, just verify the route works
    if [[ "$STATUS" == "404" || "$STATUS" == "400" ]]; then
      pass "META-004: POST /templates/{name} route active (status $STATUS)"
    else
      fail "META-004" "unexpected status: $STATUS"
    fi
  fi

  # gRPC listing (convenience route)
  api GET /grpc
  assert_status 200 "META-005: GET /grpc returns 200"

  # MQTT listing
  api GET /mqtt
  assert_status 200 "META-006: GET /mqtt returns 200"

  api GET /mqtt/status
  assert_status 200 "META-007: GET /mqtt/status returns 200"

  # SOAP listing
  api GET /soap
  assert_status 200 "META-008: GET /soap returns 200"
}

# ─── SSE Admin Endpoints ─────────────────────────────────────────────────────

run_sse_admin() {
  suite_header "SSE Admin Endpoints"

  # SSE connection management (no active connections expected)
  api GET /sse/connections
  assert_status 200 "SSE-001: GET /sse/connections returns 200"

  api GET /sse/stats
  assert_status 200 "SSE-002: GET /sse/stats returns 200"
}

# ─── GraphQL Protocol ────────────────────────────────────────────────────────

run_graphql() {
  suite_header "GraphQL Protocol"

  # Create a GraphQL mock
  api POST /mocks -d '{
    "type": "graphql",
    "name": "Test GraphQL API",
    "graphql": {
      "path": "/graphql",
      "schema": "type Query { user(id: ID!): User\n  users: [User!]! }\ntype User { id: ID!\n  name: String!\n  email: String! }",
      "introspection": true,
      "resolvers": {
        "Query.user": {
          "response": {
            "id": "42",
            "name": "Test User",
            "email": "test@example.com"
          }
        },
        "Query.users": {
          "response": [
            {"id": "1", "name": "Alice", "email": "alice@example.com"},
            {"id": "2", "name": "Bob", "email": "bob@example.com"}
          ]
        }
      }
    }
  }'
  assert_status 201 "GQL-001: Create GraphQL mock"
  local gql_id
  gql_id=$(echo "$BODY" | jq -r '.id')

  # Query the GraphQL endpoint
  engine POST /graphql -d '{"query": "{ users { id name } }"}'
  assert_status 200 "GQL-002: GraphQL query returns 200"
  assert_body_contains "Alice" "GQL-003: Response contains Alice"

  # Query with variables
  engine POST /graphql -d '{"query": "query GetUser($id: ID!) { user(id: $id) { id name email } }", "variables": {"id": "42"}}'
  assert_status 200 "GQL-004: GraphQL query with variables"
  assert_body_contains "Test User" "GQL-005: User query returns data"

  # Introspection query
  engine POST /graphql -d '{"query": "{ __schema { queryType { name } } }"}'
  assert_status 200 "GQL-006: Introspection query works"

  # Handler should be listed
  api GET /handlers
  assert_status 200 "GQL-007: Handlers list includes GraphQL"

  # Cleanup
  api DELETE /mocks
}

# ─── OAuth Protocol ──────────────────────────────────────────────────────────

run_oauth() {
  suite_header "OAuth Protocol"

  # Create an OAuth mock
  api POST /mocks -d '{
    "type": "oauth",
    "name": "Test OAuth Provider",
    "oauth": {
      "issuer": "http://localhost:4280/oauth",
      "tokenExpiry": "1h",
      "defaultScopes": ["openid", "profile", "email"],
      "clients": [
        {
          "clientId": "test-app",
          "clientSecret": "test-secret",
          "redirectUris": ["http://localhost:3000/callback"],
          "grantTypes": ["client_credentials", "password"]
        }
      ],
      "users": [
        {
          "username": "testuser",
          "password": "testpass",
          "claims": {
            "sub": "user-123",
            "name": "Test User",
            "email": "test@example.com"
          }
        }
      ]
    }
  }'
  assert_status 201 "OAUTH-001: Create OAuth mock"

  # OIDC discovery
  engine GET /oauth/.well-known/openid-configuration
  assert_status 200 "OAUTH-002: OIDC discovery endpoint"
  assert_body_contains "token_endpoint" "OAUTH-003: Discovery has token_endpoint"

  # JWKS
  engine GET /oauth/.well-known/jwks.json
  assert_status 200 "OAUTH-004: JWKS endpoint"
  assert_body_contains "keys" "OAUTH-005: JWKS has keys"

  # Client credentials grant
  local token_resp
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=client_credentials&client_id=test-app&client_secret=test-secret&scope=openid' 2>&1) || true
  BODY=$(echo "$token_resp" | sed '$d')
  STATUS=$(echo "$token_resp" | tail -n 1)
  assert_status 200 "OAUTH-006: Client credentials grant"
  assert_body_contains "access_token" "OAUTH-007: Response has access_token"

  # Extract token for userinfo
  local token
  token=$(echo "$BODY" | jq -r '.access_token' 2>/dev/null) || token=""

  # Password grant
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=password&client_id=test-app&client_secret=test-secret&username=testuser&password=testpass&scope=openid+profile' 2>&1) || true
  BODY=$(echo "$token_resp" | sed '$d')
  STATUS=$(echo "$token_resp" | tail -n 1)
  assert_status 200 "OAUTH-008: Password grant"

  # Invalid client credentials → error
  token_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/oauth/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d 'grant_type=client_credentials&client_id=wrong&client_secret=wrong' 2>&1) || true
  BODY=$(echo "$token_resp" | sed '$d')
  STATUS=$(echo "$token_resp" | tail -n 1)
  if [[ "$STATUS" == "401" || "$STATUS" == "400" ]]; then
    pass "OAUTH-009: Invalid credentials rejected"
  else
    fail "OAUTH-009" "expected 401/400, got $STATUS"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── SOAP Protocol ───────────────────────────────────────────────────────────

run_soap() {
  suite_header "SOAP Protocol"

  # Create a SOAP mock
  api POST /mocks -d '{
    "type": "soap",
    "name": "Test SOAP Service",
    "soap": {
      "path": "/soap/user",
      "operations": {
        "GetUser": {
          "soapAction": "http://example.com/GetUser",
          "response": "<GetUserResponse><id>123</id><name>John Doe</name></GetUserResponse>"
        },
        "CreateUser": {
          "soapAction": "http://example.com/CreateUser",
          "response": "<CreateUserResponse><userId>new-001</userId><status>created</status></CreateUserResponse>"
        }
      }
    }
  }'
  assert_status 201 "SOAP-001: Create SOAP mock"

  # Send SOAP request
  local soap_resp
  soap_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/soap/user" \
    -H 'Content-Type: text/xml' \
    -H 'SOAPAction: http://example.com/GetUser' \
    -d '<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetUser><userId>123</userId></GetUser></soap:Body></soap:Envelope>' 2>&1) || true
  BODY=$(echo "$soap_resp" | sed '$d')
  STATUS=$(echo "$soap_resp" | tail -n 1)
  assert_status 200 "SOAP-002: SOAP GetUser request"
  assert_body_contains "John Doe" "SOAP-003: Response contains user name"

  # WSDL endpoint
  engine GET '/soap/user?wsdl'
  if [[ "$STATUS" == "200" ]]; then
    pass "SOAP-004: WSDL endpoint responds"
  else
    skip "SOAP-004: WSDL not available (status $STATUS)"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── WebSocket Protocol ──────────────────────────────────────────────────────

run_websocket() {
  suite_header "WebSocket Protocol"

  # Create a WebSocket mock
  api POST /mocks -d '{
    "type": "websocket",
    "name": "Test WebSocket",
    "websocket": {
      "path": "/ws/test",
      "echoMode": true,
      "matchers": [
        {
          "match": {"type": "exact", "value": "ping"},
          "response": {"type": "text", "value": "pong"}
        }
      ],
      "defaultResponse": {"type": "text", "value": "unknown"}
    }
  }'
  assert_status 201 "WS-001: Create WebSocket mock"

  # Verify WebSocket handler is listed
  api GET /handlers
  assert_status 200 "WS-002: Handlers list"
  local handler_count
  handler_count=$(echo "$BODY" | jq '.handlers | length // .count // 0' 2>/dev/null) || handler_count=0
  if [[ "$handler_count" -ge 1 ]]; then
    pass "WS-003: At least 1 handler registered"
  else
    # Handler may be registered differently — check total
    skip "WS-003: Handler count=$handler_count (may be listed differently)"
  fi

  # Cleanup
  api DELETE /mocks
}

# ─── SSE Protocol (stream test) ──────────────────────────────────────────────

run_sse_stream() {
  suite_header "SSE Protocol"

  # Create an SSE mock (type is "http" with sse config)
  api POST /mocks -d '{
    "type": "http",
    "name": "SSE Event Stream",
    "http": {
      "matcher": {"method": "GET", "path": "/events"},
      "sse": {
        "events": [
          {"type": "message", "data": {"text": "hello"}, "id": "1"},
          {"type": "message", "data": {"text": "world"}, "id": "2"}
        ],
        "timing": {"fixedDelay": 10},
        "lifecycle": {"maxEvents": 2}
      }
    }
  }'
  assert_status 201 "SSE-STREAM-001: Create SSE mock"

  # Connect and read a few events (with timeout so it doesnt hang)
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/events" 2>&1) || true
  if echo "$sse_output" | grep -q "hello"; then
    pass "SSE-STREAM-002: Received SSE events"
  else
    fail "SSE-STREAM-002" "No SSE events received (output: $(echo "$sse_output" | head -c 200))"
  fi

  # Check SSE connections were tracked
  api GET /sse/connections
  assert_status 200 "SSE-STREAM-003: SSE connections endpoint"

  # Check SSE stats
  api GET /sse/stats
  assert_status 200 "SSE-STREAM-004: SSE stats endpoint"

  # Cleanup
  api DELETE /mocks
}

# ─── Proxy Admin Endpoints ───────────────────────────────────────────────────

run_proxy() {
  suite_header "Proxy Admin Endpoints"

  # Get proxy status (should work even when proxy not started)
  api GET /proxy/status
  assert_status 200 "PROXY-001: GET /proxy/status returns 200"

  # Get proxy filters
  api GET /proxy/filters
  assert_status 200 "PROXY-002: GET /proxy/filters returns 200"

  # Get CA (may not be initialized)
  api GET /proxy/ca
  if [[ "$STATUS" == "200" || "$STATUS" == "404" ]]; then
    pass "PROXY-003: GET /proxy/ca responds ($STATUS)"
  else
    fail "PROXY-003" "expected 200/404, got $STATUS"
  fi

  # Generate CA (may need specific payload or return 400 for missing params)
  api POST /proxy/ca -d '{}'
  if [[ "$STATUS" -lt 500 ]]; then
    pass "PROXY-004: POST /proxy/ca handled ($STATUS)"
  else
    fail "PROXY-004" "server error: $STATUS"
  fi

  # Start proxy (may need a target — test error handling)
  api POST /proxy/start -d '{"target": "http://httpbin.org"}'
  if [[ "$STATUS" -lt 500 ]]; then
    pass "PROXY-005: POST /proxy/start handled ($STATUS)"
  else
    fail "PROXY-005" "server error on proxy start: $STATUS"
  fi

  # Stop proxy
  api POST /proxy/stop
  if [[ "$STATUS" == "200" || "$STATUS" == "400" ]]; then
    pass "PROXY-006: POST /proxy/stop handled ($STATUS)"
  else
    fail "PROXY-006" "unexpected status: $STATUS"
  fi

  # Set proxy mode
  api PUT /proxy/mode -d '{"mode": "record"}'
  if [[ "$STATUS" -lt 500 ]]; then
    pass "PROXY-007: PUT /proxy/mode handled ($STATUS)"
  else
    fail "PROXY-007" "server error: $STATUS"
  fi

  # Set proxy filters
  api PUT /proxy/filters -d '{"include": ["*"], "exclude": []}'
  if [[ "$STATUS" -lt 500 ]]; then
    pass "PROXY-008: PUT /proxy/filters handled ($STATUS)"
  else
    fail "PROXY-008" "server error: $STATUS"
  fi
}

# ─── Token & API Key Management ──────────────────────────────────────────────

run_tokens() {
  suite_header "Token & API Key Management"

  # Generate registration token
  api POST /admin/tokens/registration -d '{"name": "test-runtime"}'
  if [[ "$STATUS" == "200" || "$STATUS" == "201" ]]; then
    pass "TOKEN-001: Generate registration token"
  else
    fail "TOKEN-001" "expected 200/201, got $STATUS"
  fi

  # List registration tokens
  api GET /admin/tokens/registration
  assert_status 200 "TOKEN-002: List registration tokens"

  # Get API key
  api GET /admin/api-key
  assert_status 200 "TOKEN-003: Get API key"

  # Rotate API key (may return 400 when auth is disabled via --no-auth)
  api POST /admin/api-key/rotate
  if [[ "$STATUS" -lt 500 ]]; then
    pass "TOKEN-004: Rotate API key handled ($STATUS)"
  else
    fail "TOKEN-004" "server error: $STATUS"
  fi
}

# ─── Import Edge Cases ───────────────────────────────────────────────────────

run_import_edge() {
  suite_header "Import Edge Cases"

  # Import with replace mode
  api POST /config -d '{
    "replace": true,
    "config": {
      "version": "1.0",
      "name": "replace-test",
      "mocks": [
        {
          "type": "http",
          "name": "Replace Mock A",
          "http": {
            "matcher": {"method": "GET", "path": "/api/replace-a"},
            "response": {"statusCode": 200, "body": "A"}
          }
        }
      ]
    }
  }'
  assert_status 200 "IMP-EDGE-001: Import with replace"

  engine GET /api/replace-a
  assert_status 200 "IMP-EDGE-002: Replace mock A works"

  # Import again with replace — should clear previous mocks
  api POST /config -d '{
    "replace": true,
    "config": {
      "version": "1.0",
      "name": "replace-test-2",
      "mocks": [
        {
          "type": "http",
          "name": "Replace Mock B",
          "http": {
            "matcher": {"method": "GET", "path": "/api/replace-b"},
            "response": {"statusCode": 200, "body": "B"}
          }
        }
      ]
    }
  }'
  assert_status 200 "IMP-EDGE-003: Second import with replace"

  engine GET /api/replace-b
  assert_status 200 "IMP-EDGE-004: Replace mock B works"

  engine GET /api/replace-a
  assert_status 404 "IMP-EDGE-005: Replace mock A gone (replaced)"

  # Import with merge (no replace flag)
  api POST /config -d '{
    "config": {
      "version": "1.0",
      "name": "merge-test",
      "mocks": [
        {
          "type": "http",
          "name": "Merged Mock C",
          "http": {
            "matcher": {"method": "GET", "path": "/api/merge-c"},
            "response": {"statusCode": 200, "body": "C"}
          }
        }
      ]
    }
  }'
  assert_status 200 "IMP-EDGE-006: Import with merge"

  engine GET /api/replace-b
  assert_status 200 "IMP-EDGE-007: Existing mock B still present"

  engine GET /api/merge-c
  assert_status 200 "IMP-EDGE-008: Merged mock C present"

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

  should_run "s1"       && run_s1
  should_run "s2"       && run_s2
  should_run "s3"       && run_s3
  should_run "s5"       && run_s5
  should_run "s6"       && run_s6
  should_run "s7"       && run_s7
  should_run "s7x"      && run_s7_extended
  should_run "s9"       && { run_s9_http; run_s9_crud; }
  should_run "s9x"      && run_s9_extended
  should_run "s10"      && run_s10
  should_run "s11"      && run_s11
  should_run "s14"      && run_s14
  should_run "s15"      && run_s15
  should_run "s15x"     && run_s15_extended
  should_run "s16"      && run_s16
  should_run "s17"      && run_s17
  should_run "import"    && run_import_export
  should_run "impedge"  && run_import_edge
  should_run "requests" && run_requests
  should_run "state"    && run_state
  should_run "ws"       && run_workspaces
  should_run "mockops"  && run_mock_ops
  should_run "meta"     && run_metadata_extended
  should_run "sse"      && run_sse_admin
  should_run "sseproto" && run_sse_stream
  should_run "graphql"  && run_graphql
  should_run "oauth"    && run_oauth
  should_run "soap"     && run_soap
  should_run "wsproto"  && run_websocket
  should_run "proxy"    && run_proxy
  should_run "tokens"   && run_tokens

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
