#!/usr/bin/env bats
# ============================================================================
# Recordings — HTTP, stream, MQTT, SOAP recordings and replay
# ============================================================================

setup() {
  load '../lib/helpers'
}

@test "S6-001: GET /recordings returns 200" {
  api GET /recordings
  [[ "$STATUS" == "200" ]]
}

@test "S6-002: GET /stream-recordings returns 200" {
  api GET /stream-recordings
  [[ "$STATUS" == "200" ]]
}

@test "S6-003: GET /stream-recordings/stats returns 200" {
  api GET /stream-recordings/stats
  [[ "$STATUS" == "200" ]]
}

@test "S6-004: GET /replay returns 200" {
  api GET /replay
  [[ "$STATUS" == "200" ]]
}

@test "S6-005: GET /mqtt-recordings returns 200" {
  api GET /mqtt-recordings
  [[ "$STATUS" == "200" ]]
}

@test "S6-006: GET /soap-recordings returns 200" {
  api GET /soap-recordings
  [[ "$STATUS" == "200" ]]
}
