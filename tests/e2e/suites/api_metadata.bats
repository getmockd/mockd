#!/usr/bin/env bats
# ============================================================================
# Metadata & Formats — OpenAPI, Insomnia, templates, metrics
# ============================================================================

setup() {
  load '../lib/helpers'
}

# ─── S5: Core Metadata ──────────────────────────────────────────────────────

@test "S5-001: GET /formats returns 200" {
  api GET /formats
  [[ "$STATUS" == "200" ]]
}

@test "S5-002: GET /templates returns 200" {
  api GET /templates
  [[ "$STATUS" == "200" ]]
}

@test "S5-003: GET /openapi.json returns 200" {
  api GET /openapi.json
  [[ "$STATUS" == "200" ]]
}

@test "S5-004: GET /openapi.yaml returns 200" {
  api GET /openapi.yaml
  [[ "$STATUS" == "200" ]]
}

@test "S5-005: GET /preferences responds" {
  api GET /preferences
  [[ "$STATUS" == "200" || "$STATUS" == "501" ]]
}

# ─── Extended Metadata ───────────────────────────────────────────────────────

@test "META-001: GET /insomnia.json returns 200" {
  api GET /insomnia.json
  [[ "$STATUS" == "200" ]]
}

@test "META-002: GET /insomnia.yaml returns 200" {
  api GET /insomnia.yaml
  [[ "$STATUS" == "200" ]]
}

@test "META-003: GET /metrics returns 200" {
  api GET /metrics
  [[ "$STATUS" == "200" ]]
}

@test "META-004: POST /templates/{name} route active" {
  api POST /templates/basic -d '{}'
  [[ "$STATUS" -lt 500 ]]
}

@test "META-005: GET /grpc returns 200" {
  api GET /grpc
  [[ "$STATUS" == "200" ]]
}

@test "META-006: GET /mqtt returns 200" {
  api GET /mqtt
  [[ "$STATUS" == "200" ]]
}

@test "META-007: GET /mqtt/status returns 200" {
  api GET /mqtt/status
  [[ "$STATUS" == "200" ]]
}

@test "META-008: GET /soap returns 200" {
  api GET /soap
  [[ "$STATUS" == "200" ]]
}
