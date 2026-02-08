#!/usr/bin/env bats
# ============================================================================
# Stateful CRUD Edge Cases — in-memory resource CRUD operations
# ============================================================================

setup_file() {
  load '../lib/helpers'

  api POST /config -d '{
    "config": {
      "version": "1.0",
      "name": "crud-test",
      "mocks": [],
      "statefulResources": [{
        "name": "items",
        "basePath": "/api/items",
        "idField": "id",
        "seedData": [
          {"id": "1", "name": "Alpha", "score": 10},
          {"id": "2", "name": "Beta", "score": 20}
        ]
      }]
    }
  }'
}

setup() {
  load '../lib/helpers'
}

@test "S9-CRUD-001: List returns 200 with total 2" {
  engine GET /api/items
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.meta.total')" == "2" ]]
}

@test "S9-CRUD-002: Get by ID returns correct name" {
  engine GET /api/items/1
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.name')" == "Alpha" ]]
}

@test "S9-CRUD-003: POST duplicate ID → 409" {
  engine POST /api/items -d '{"id":"1","name":"Duplicate"}'
  [[ "$STATUS" == "409" ]]
}

@test "S9-CRUD-004: DELETE nonexistent → 404" {
  engine DELETE /api/items/nonexistent
  [[ "$STATUS" == "404" ]]
}

@test "S9-CRUD-005: PATCH → 405 (not supported)" {
  engine PATCH /api/items/nonexistent -d '{"name":"updated"}'
  [[ "$STATUS" == "405" ]]
}

@test "S9-CRUD-006: Offset beyond total → empty" {
  engine GET '/api/items?offset=100'
  [[ "$(json_field '.meta.count')" == "0" ]]
}

@test "S9-CRUD-007: POST new item → 201" {
  engine POST /api/items -d '{"name":"Gamma","score":30}'
  [[ "$STATUS" == "201" ]]

  engine GET /api/items
  [[ "$(json_field '.meta.total')" == "3" ]]
}

@test "S9-CRUD-008: PUT update → 200" {
  engine PUT /api/items/1 -d '{"name":"Alpha Updated","score":100}'
  [[ "$STATUS" == "200" ]]

  engine GET /api/items/1
  [[ "$(json_field '.name')" == "Alpha Updated" ]]
}

@test "S9-CRUD-009: DELETE item → 204, then 404" {
  # Create a disposable item for this test
  engine POST /api/items -d '{"id":"delete-me","name":"Disposable","score":0}'
  engine DELETE /api/items/delete-me
  [[ "$STATUS" == "204" ]]

  engine GET /api/items/delete-me
  [[ "$STATUS" == "404" ]]
}
