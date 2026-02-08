#!/usr/bin/env bats
# ============================================================================
# HTTP Edge Cases — body matching, priority, unicode, headers, delays, paths
# ============================================================================

setup_file() {
  load '../lib/helpers'

  # Create mocks needed by this suite
  api POST /mocks -d '{
    "type": "http",
    "name": "Body Matcher",
    "http": {
      "matcher": {"method": "POST", "path": "/api/echo", "bodyContains": "hello"},
      "response": {"statusCode": 200, "body": "{\"echoed\": true}"}
    }
  }'

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

  api POST /mocks -d '{
    "type": "http",
    "name": "Unicode",
    "http": {
      "matcher": {"method": "GET", "path": "/api/emoji"},
      "response": {"statusCode": 200, "body": "{\"emoji\": \"\xf0\x9f\x8e\x89\"}"}
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

# ─── S9: Core HTTP Tests ────────────────────────────────────────────────────

@test "S9-HTTP-001: Body matching works" {
  engine POST /api/echo -d 'hello world'
  [[ "$STATUS" == "200" ]]
}

@test "S9-HTTP-009: Unicode mock responds with emoji" {
  engine GET /api/emoji
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"🎉"* ]]
}

@test "S9-HTTP-014: Higher priority wins" {
  engine GET /api/priority
  [[ "$(json_field '.priority')" == "high" ]]
}

@test "S9-HTTP-020: Unmatched path → 404" {
  engine GET /api/no-such-endpoint
  [[ "$STATUS" == "404" ]]
}

# ─── S9 Extended: Header Matching ────────────────────────────────────────────

@test "S9X-001: Create header matcher" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Header Matcher",
    "http": {
      "matcher": {"method": "GET", "path": "/api/header-test", "headers": {"X-Custom": "magic"}},
      "response": {"statusCode": 200, "body": "{\"matched\": \"header\"}"}
    }
  }'
  [[ "$STATUS" == "201" ]]
}

@test "S9X-002: Header match succeeds" {
  engine GET /api/header-test -H 'X-Custom: magic'
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.matched')" == "header" ]]
}

@test "S9X-003: Wrong header → 404" {
  engine GET /api/header-test -H 'X-Custom: wrong'
  [[ "$STATUS" == "404" ]]
}

# ─── S9 Extended: Query Parameter Matching ───────────────────────────────────

@test "S9X-004: Create query param matcher" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Query Matcher",
    "http": {
      "matcher": {"method": "GET", "path": "/api/query-test", "queryParams": {"key": "value"}},
      "response": {"statusCode": 200, "body": "{\"matched\": \"query\"}"}
    }
  }'
  [[ "$STATUS" == "201" ]]
}

@test "S9X-005: Query param match succeeds" {
  engine GET '/api/query-test?key=value'
  [[ "$STATUS" == "200" ]]
}

@test "S9X-006: Wrong query param → 404" {
  engine GET '/api/query-test?key=wrong'
  [[ "$STATUS" == "404" ]]
}

# ─── S9 Extended: Custom Response Headers ────────────────────────────────────

@test "S9X-007: Create mock with custom response headers" {
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
  [[ "$STATUS" == "201" ]]
}

@test "S9X-008: Custom response header present" {
  local resp_headers
  resp_headers=$(curl -s -D - -o /dev/null "${ENGINE}/api/custom-headers" 2>&1) || resp_headers=""
  echo "$resp_headers" | grep -qi "X-Custom-Response"
}

# ─── S9 Extended: Delayed Response ───────────────────────────────────────────

@test "S9X-009: Create delayed mock" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Delayed Response",
    "http": {
      "matcher": {"method": "GET", "path": "/api/delayed"},
      "response": {"statusCode": 200, "body": "{\"delayed\": true}", "delayMs": 100}
    }
  }'
  [[ "$STATUS" == "201" ]]
}

@test "S9X-010: Delayed mock applies delay" {
  local start_time end_time elapsed
  start_time=$(date +%s%N)
  engine GET /api/delayed
  end_time=$(date +%s%N)
  elapsed=$(( (end_time - start_time) / 1000000 ))

  [[ "$STATUS" == "200" ]]
  [[ "$elapsed" -ge 80 ]]
}

# ─── S9 Extended: Path Pattern Matching ──────────────────────────────────────

@test "S9X-011: Create wildcard path mock" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Wildcard Path",
    "http": {
      "matcher": {"method": "GET", "path": "/api/users/{id}/profile"},
      "response": {"statusCode": 200, "body": "{\"profile\": true}"}
    }
  }'
  [[ "$STATUS" == "201" ]]
}

@test "S9X-012: Wildcard path matches /users/123/profile" {
  engine GET /api/users/123/profile
  [[ "$STATUS" == "200" ]]
}

@test "S9X-013: Wildcard path matches /users/abc/profile" {
  engine GET /api/users/abc/profile
  [[ "$STATUS" == "200" ]]
}
