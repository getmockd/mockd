#!/usr/bin/env bats
# ============================================================================
# Proxy Admin Endpoints — status, filters, CA, start/stop, mode
# ============================================================================

teardown_file() {
  load '../lib/helpers'
  # Ensure proxy is stopped after tests
  api POST /proxy/stop
}

setup() {
  load '../lib/helpers'
}

@test "PROXY-001: GET /proxy/status returns 200" {
  api GET /proxy/status
  [[ "$STATUS" == "200" ]]
}

@test "PROXY-002: GET /proxy/filters returns 200" {
  api GET /proxy/filters
  [[ "$STATUS" == "200" ]]
}

@test "PROXY-003: GET /proxy/ca responds" {
  api GET /proxy/ca
  [[ "$STATUS" == "200" || "$STATUS" == "404" ]]
}

@test "PROXY-004: POST /proxy/ca handled without 5xx" {
  api POST /proxy/ca -d '{}'
  [[ "$STATUS" -lt 500 ]]
}

@test "PROXY-005: POST /proxy/start handled without 5xx" {
  api POST /proxy/start -d '{"target": "http://httpbin.org"}'
  [[ "$STATUS" -lt 500 ]]
}

@test "PROXY-006: POST /proxy/stop handled" {
  api POST /proxy/stop
  [[ "$STATUS" == "200" || "$STATUS" == "400" ]]
}

@test "PROXY-007: PUT /proxy/mode handled without 5xx" {
  api PUT /proxy/mode -d '{"mode": "record"}'
  [[ "$STATUS" -lt 500 ]]
}

@test "PROXY-008: PUT /proxy/filters handled without 5xx" {
  api PUT /proxy/filters -d '{"include": ["*"], "exclude": []}'
  [[ "$STATUS" -lt 500 ]]
}
