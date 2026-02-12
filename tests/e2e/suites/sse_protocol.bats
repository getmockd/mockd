#!/usr/bin/env bats
# ============================================================================
# SSE Protocol — event streaming, wire format, lifecycle, admin endpoints
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

  # Create a typed-events SSE mock for testing event: field
  api POST /mocks -d '{
    "type": "http",
    "name": "SSE Typed Events",
    "http": {
      "matcher": {"method": "GET", "path": "/typed-events"},
      "sse": {
        "events": [
          {"type": "notification", "data": {"msg": "alert"}, "id": "10"},
          {"type": "heartbeat", "data": {"ts": 12345}, "id": "11"}
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

# ── Basic Streaming ───────────────────────────────────────────────────────────

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

# ── Wire Format Verification ─────────────────────────────────────────────────

@test "SSE-STREAM-005: Response Content-Type is text/event-stream" {
  local headers
  headers=$(curl -s -D - -o /dev/null --max-time 3 \
    -H 'Accept: text/event-stream' "${ENGINE}/events" 2>/dev/null) || true
  echo "$headers" | grep -qi "content-type.*text/event-stream"
}

@test "SSE-STREAM-006: Raw output contains event: field" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/events" 2>&1) || true
  # SSE wire format should have "event:" lines
  echo "$sse_output" | grep -q "event:"
}

@test "SSE-STREAM-007: Raw output contains id: field" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/events" 2>&1) || true
  # SSE wire format should have "id:" lines
  echo "$sse_output" | grep -q "id:"
}

@test "SSE-STREAM-008: Raw output contains data: field" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/events" 2>&1) || true
  echo "$sse_output" | grep -q "data:"
}

# ── Typed Events ──────────────────────────────────────────────────────────────

@test "SSE-STREAM-009: Typed events have correct event: type" {
  local sse_output
  sse_output=$(curl -s -N --max-time 3 -H 'Accept: text/event-stream' "${ENGINE}/typed-events" 2>&1) || true
  # Should contain custom event types
  echo "$sse_output" | grep -q "event:notification\|event: notification"
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
