#!/usr/bin/env bats
# ============================================================================
# Chaos Injection — latency, error rate, bandwidth, path-specific rules
# ============================================================================

setup_file() {
  load '../lib/helpers'

  api POST /mocks -d '{
    "type": "http",
    "name": "Chaos Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/chaos-target"},
      "response": {"statusCode": 200, "body": "{\"ok\": true}"}
    }
  }'

  api POST /mocks -d '{
    "type": "http",
    "name": "Chaos Deep Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/chaos-deep"},
      "response": {"statusCode": 200, "body": "{\"ok\": true}"}
    }
  }'
}

teardown_file() {
  load '../lib/helpers'
  api PUT /chaos -d '{"enabled": false}'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

# ─── S15: Core Chaos ────────────────────────────────────────────────────────

@test "S15-001: No chaos by default" {
  engine GET /api/chaos-target
  [[ "$STATUS" == "200" ]]
}

@test "S15-002: Enable chaos with latency" {
  api PUT /chaos -d '{
    "enabled": true,
    "latency": {
      "min": "1ms",
      "max": "5ms",
      "probability": 1.0
    }
  }'
  [[ "$STATUS" == "200" ]]
}

@test "S15-003: GET /chaos returns config with enabled=true" {
  api PUT /chaos -d '{"enabled": true, "latency": {"min": "1ms", "max": "5ms", "probability": 1.0}}'
  api GET /chaos
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.enabled')" == "true" ]]
}

@test "S15-004: Disable chaos" {
  api PUT /chaos -d '{"enabled": false}'
  [[ "$STATUS" == "200" ]]
}

# ─── S15 Extended: Error Rate ────────────────────────────────────────────────

@test "S15X-001: Enable error rate chaos" {
  api PUT /chaos -d '{
    "enabled": true,
    "errorRate": {
      "probability": 1.0,
      "statusCodes": [500, 502, 503],
      "defaultCode": 500
    }
  }'
  [[ "$STATUS" == "200" ]]
}

@test "S15X-002: Error rate produces 5xx" {
  api PUT /chaos -d '{"enabled": true, "errorRate": {"probability": 1.0, "statusCodes": [500], "defaultCode": 500}}'
  engine GET /api/chaos-deep
  [[ "$STATUS" -ge 500 ]]
  api PUT /chaos -d '{"enabled": false}'
}

# ─── S15 Extended: Bandwidth ────────────────────────────────────────────────

@test "S15X-003: Enable bandwidth chaos" {
  api PUT /chaos -d '{
    "enabled": true,
    "bandwidth": {
      "bytesPerSecond": 1024,
      "probability": 1.0
    }
  }'
  [[ "$STATUS" == "200" ]]
}

@test "S15X-004: Bandwidth-throttled response received" {
  api PUT /chaos -d '{"enabled": true, "bandwidth": {"bytesPerSecond": 1024, "probability": 1.0}}'
  engine GET /api/chaos-deep
  [[ "$STATUS" -ge 200 && "$STATUS" -lt 600 ]]
  api PUT /chaos -d '{"enabled": false}'
}

# ─── S15 Extended: Path Rules ────────────────────────────────────────────────

@test "S15X-005: Enable chaos with path-specific rules" {
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
  [[ "$STATUS" == "200" ]]
  api PUT /chaos -d '{"enabled": false}'
}

@test "S15X-006: Disable chaos after rules" {
  api PUT /chaos -d '{"enabled": false}'
  [[ "$STATUS" == "200" ]]
}
