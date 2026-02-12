#!/usr/bin/env bats
# ============================================================================
# Import/Export — round-trip, replace mode, merge mode
# ============================================================================

setup() {
  load '../lib/helpers'
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

# ─── Round-Trip ──────────────────────────────────────────────────────────────

@test "IMP-001: Import collection returns 200" {
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
  [[ "$STATUS" == "200" ]]
}

@test "IMP-002: 1 mock imported" {
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
      ]
    }
  }'
  [[ "$(json_field '.imported')" == "1" ]]
}

@test "IMP-004: Imported mock responds" {
  engine GET /api/roundtrip
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.imported')" == "true" ]]
}

@test "EXP-001: Export returns 200" {
  api GET /config
  [[ "$STATUS" == "200" ]]
}

# ─── Replace Mode ────────────────────────────────────────────────────────────

@test "IMP-EDGE-001: Import with replace" {
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
  [[ "$STATUS" == "200" ]]
}

@test "IMP-EDGE-002: Replace mock A works" {
  engine GET /api/replace-a
  [[ "$STATUS" == "200" ]]
}

@test "IMP-EDGE-003: Second import with replace clears previous" {
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
  [[ "$STATUS" == "200" ]]
}

@test "IMP-EDGE-004: Replace mock B works" {
  engine GET /api/replace-b
  [[ "$STATUS" == "200" ]]
}

@test "IMP-EDGE-005: Replace mock A gone after replacement" {
  engine GET /api/replace-a
  [[ "$STATUS" == "404" ]]
}

# ─── Merge Mode ──────────────────────────────────────────────────────────────

@test "IMP-EDGE-006: Import with merge" {
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
  [[ "$STATUS" == "200" ]]
}

@test "IMP-EDGE-007: Existing mock B still present after merge" {
  engine GET /api/replace-b
  [[ "$STATUS" == "200" ]]
}

@test "IMP-EDGE-008: Merged mock C present" {
  engine GET /api/merge-c
  [[ "$STATUS" == "200" ]]
}

# ─── CLI Curl Import ─────────────────────────────────────────────────────────

@test "IMP-CURL-001: Import curl command creates working mock" {
  api DELETE /mocks

  run mockd import \
    'curl -X GET https://api.example.com/users -H "Accept: application/json"' \
    --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Imported"* || "$output" == *"Parsed"* ]]

  # Verify mock was created (GET /mocks returns {mocks: [...], total, count})
  api GET /mocks
  local count
  count=$(echo "$BODY" | jq '.mocks | length')
  [[ "$count" -ge 1 ]]
}

# ─── CLI OpenAPI Import ───────────────────────────────────────────────────────

@test "IMP-OAS-001: Import OpenAPI spec creates mocks" {
  api DELETE /mocks

  cat > /tmp/e2e-openapi.yaml << 'YAML'
openapi: "3.0.0"
info:
  title: Test API
  version: "1.0.0"
paths:
  /api/pets:
    get:
      summary: List pets
      responses:
        "200":
          description: A list of pets
          content:
            application/json:
              example:
                - id: 1
                  name: Rex
YAML

  run mockd import /tmp/e2e-openapi.yaml --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Imported"* || "$output" == *"Parsed"* ]]

  # Verify at least one mock was created (GET /mocks returns {mocks: [...], total, count})
  api GET /mocks
  local count
  count=$(echo "$BODY" | jq '.mocks | length')
  [[ "$count" -ge 1 ]]

  rm -f /tmp/e2e-openapi.yaml
}

# ─── Dry-Run Import ──────────────────────────────────────────────────────────

@test "IMP-DRY-001: Import with --dry-run does not create mocks" {
  api DELETE /mocks

  cat > /tmp/e2e-dryrun.yaml << 'YAML'
version: "1.0"
name: dry-run-test
mocks:
  - id: dryrun-mock-1
    type: http
    name: Dry Run Mock
    http:
      matcher:
        method: GET
        path: /api/dryrun
      response:
        statusCode: 200
        body: '{"dry": true}'
YAML

  run mockd import /tmp/e2e-dryrun.yaml --dry-run --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Dry run"* || "$output" == *"Parsed"* ]]

  # Verify NO mock was actually created
  engine GET /api/dryrun
  [[ "$STATUS" == "404" ]]

  rm -f /tmp/e2e-dryrun.yaml
}
