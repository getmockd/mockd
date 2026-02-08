#!/usr/bin/env bats
# ============================================================================
# Template Engine — dynamic response templating (uuid, now, etc.)
# ============================================================================

setup() {
  load '../lib/helpers'
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

@test "S17-001: Create template mock" {
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
  [[ "$STATUS" == "201" ]]
}

@test "S17-002: Template mock responds" {
  engine GET /api/template
  [[ "$STATUS" == "200" ]]
}

@test "S17-003: Template uuid is expanded" {
  engine GET /api/template
  local uuid_val
  uuid_val=$(json_field '.uuid')
  [[ "$uuid_val" != '{{uuid}}' ]]
  [[ -n "$uuid_val" ]]
  [[ "$uuid_val" != "null" ]]
}
