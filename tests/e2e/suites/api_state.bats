#!/usr/bin/env bats
# ============================================================================
# State Management — stateful resources, reset, persistence, concurrency
# ============================================================================

setup_file() {
  load '../lib/helpers'

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
}

setup() {
  load '../lib/helpers'
}

# ─── State Endpoints ────────────────────────────────────────────────────────

@test "STATE-001: GET /state returns 200" {
  api GET /state
  [[ "$STATUS" == "200" ]]
}

@test "STATE-002: GET /state/resources returns 200" {
  api GET /state/resources
  [[ "$STATUS" == "200" ]]
}

@test "STATE-003: GET /state/resources/{name} returns 200" {
  api GET /state/resources/products
  [[ "$STATUS" == "200" ]]
}

# ─── State Mutations ────────────────────────────────────────────────────────

@test "STATE-004: Add item to products" {
  engine POST /api/products -d '{"name":"Extra","price":99}'
  [[ "$STATUS" == "201" ]]

  engine GET /api/products
  [[ "$(json_field '.meta.total')" == "3" ]]
}

@test "STATE-005: Reset specific resource to seed data" {
  api POST /state/resources/products/reset
  [[ "$STATUS" == "200" ]]

  engine GET /api/products
  [[ "$(json_field '.meta.total')" == "2" ]]
}

@test "STATE-006: DELETE /state/resources/{name} clears items" {
  api DELETE /state/resources/orders
  [[ "$STATUS" == "200" ]]

  engine GET /api/orders
  [[ "$(json_field '.meta.total')" == "0" ]]
}

@test "STATE-007: POST /state/reset restores all seed data" {
  api POST /state/reset
  [[ "$STATUS" == "200" ]]

  engine GET /api/orders
  [[ "$(json_field '.meta.total')" == "1" ]]
}

# ─── S11: Persistence ───────────────────────────────────────────────────────

@test "S11-001: Created mock is visible after creation" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Persist Test",
    "http": {
      "matcher": {"method": "GET", "path": "/api/persist-test"},
      "response": {"statusCode": 200, "body": "ok"}
    }
  }'
  [[ "$STATUS" == "201" ]]

  api GET /mocks
  local count
  count=$(json_field '.total')
  [[ "$count" -ge 1 ]]

  api DELETE /mocks
}

# ─── S14: Concurrency ───────────────────────────────────────────────────────

@test "S14-001: 10 concurrent requests all return 200" {
  api POST /mocks -d '{
    "type": "http",
    "name": "Concurrent Target",
    "http": {
      "matcher": {"method": "GET", "path": "/api/concurrent"},
      "response": {"statusCode": 200, "body": "{\"ok\": true}"}
    }
  }'

  local pids=()
  local concurrent_pass=true
  for i in $(seq 1 10); do
    (
      local resp
      resp=$(curl -s -o /dev/null -w '%{http_code}' "${ENGINE}/api/concurrent")
      [[ "$resp" == "200" ]]
    ) &
    pids+=($!)
  done

  for pid in "${pids[@]}"; do
    wait "$pid" || concurrent_pass=false
  done

  [[ "$concurrent_pass" == "true" ]]

  api DELETE /mocks
}
