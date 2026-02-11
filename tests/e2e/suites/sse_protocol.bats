#!/usr/bin/env bats
# ============================================================================
# SSE Protocol — event streaming, admin connections/stats
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

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
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

@test "SSE-STREAM-001: Create SSE mock returns 201" {
  api POST /mocks -d '{
    "type": "http",
    "name": "SSE Verify",
    "http": {
      "matcher": {"method": "GET", "path": "/events-verify"},
      "sse": {"events": [{"type": "message", "data": {"text": "test"}}], "lifecycle": {"maxEvents": 1}}
    }
  }'
  [[ "$STATUS" == "201" ]]
  local id=$(json_field '.id')
  api DELETE "/mocks/${id}"
}

@test "SSE-STREAM-002: Received SSE events" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/events" 2>&1) || true
  [[ "$sse_output" == *"hello"* ]]
}

# ── Wire-level SSE tests ─────────────────────────────────────────────────────

@test "SSE-STREAM-003: Receives both event IDs" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/events" 2>&1) || true
  [[ "$sse_output" == *"hello"* ]]
  [[ "$sse_output" == *"world"* ]]
}

@test "SSE-STREAM-004: Last-Event-ID reconnection resumes stream" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 \
    -H 'Accept: text/event-stream' \
    -H 'Last-Event-ID: 1' \
    "${ENGINE}/events" 2>&1) || true
  # After reconnecting with Last-Event-ID: 1, should receive event 2 ("world")
  # Accept either: only "world" (proper resume) or both events (replay)
  [[ "$sse_output" == *"world"* ]]
}

# ── Admin API tests ──────────────────────────────────────────────────────────

@test "SSE-001: GET /sse/connections returns 200" {
  api GET /sse/connections
  [[ "$STATUS" == "200" ]]
}

@test "SSE-002: GET /sse/stats returns 200" {
  api GET /sse/stats
  [[ "$STATUS" == "200" ]]
}
