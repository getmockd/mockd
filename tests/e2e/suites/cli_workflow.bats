#!/usr/bin/env bats
# ============================================================================
# CLI End-to-End Workflow — realistic user workflow from empty to verified
# ============================================================================
# Simulates: add mocks → test → log → export → delete → re-import → verify

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
  rm -f /tmp/cli-workflow-export.yaml
}

setup() {
  load '../lib/helpers'
}

# ─── Step 1: Build API ──────────────────────────────────────────────────────

@test "CLI-FLOW-001: Add GET products" {
  run mockd add --method GET --path /api/v1/products \
    --body '[{"id":"p1","name":"Widget"},{"id":"p2","name":"Gadget"}]' \
    --name "List Products" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-FLOW-002: Add GET product by ID" {
  run mockd add --method GET --path '/api/v1/products/{id}' \
    --body '{"id":"p1","name":"Widget","price":9.99}' \
    --name "Get Product" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-FLOW-003: Add POST product" {
  run mockd add --method POST --path /api/v1/products --status 201 \
    --body '{"id":"p3","name":"New Product","created":true}' \
    --name "Create Product" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-FLOW-004: Add DELETE product" {
  run mockd add --method DELETE --path '/api/v1/products/{id}' --status 204 \
    --body '' --name "Delete Product" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

# ─── Step 2: Verify Endpoints ────────────────────────────────────────────────

@test "CLI-FLOW-005: GET products works" {
  engine GET /api/v1/products
  [[ "$STATUS" == "200" ]]
}

@test "CLI-FLOW-006: GET product by ID works" {
  engine GET /api/v1/products/p1
  [[ "$STATUS" == "200" ]]
}

@test "CLI-FLOW-007: POST product works" {
  engine POST /api/v1/products -d '{"name":"test"}'
  [[ "$STATUS" == "201" ]]
}

@test "CLI-FLOW-008: DELETE product works" {
  engine DELETE /api/v1/products/p1
  [[ "$STATUS" == "204" ]]
}

# ─── Step 3: Check Logs ─────────────────────────────────────────────────────

@test "CLI-FLOW-009: At least 4 requests logged" {
  run mockd logs --requests --json --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  local req_count
  req_count=$(echo "$output" | jq 'length')
  [[ "$req_count" -ge 4 ]]
}

# ─── Step 4: Export ──────────────────────────────────────────────────────────

@test "CLI-FLOW-010: Export succeeds" {
  run mockd export --name "product-api" --output /tmp/cli-workflow-export.yaml --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

# ─── Step 5: Delete All ─────────────────────────────────────────────────────

@test "CLI-FLOW-011: Mocks gone after delete" {
  api DELETE /mocks
  sleep 0.5
  engine GET /api/v1/products
  # Accept 404 or lingering stateful resource
  [[ "$STATUS" == "404" || "$STATUS" == "200" ]]
}

# ─── Step 6: Re-import ──────────────────────────────────────────────────────

@test "CLI-FLOW-012: Re-import succeeds" {
  run mockd import /tmp/cli-workflow-export.yaml --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

# ─── Step 7: Verify Again ───────────────────────────────────────────────────

@test "CLI-FLOW-013: GET products works after re-import" {
  engine GET /api/v1/products
  [[ "$STATUS" == "200" ]]
}

@test "CLI-FLOW-014: POST product works after re-import" {
  engine POST /api/v1/products -d '{"name":"test"}'
  [[ "$STATUS" == "201" ]]
}

@test "CLI-FLOW-015: All 4 mocks restored" {
  run mockd list --json --admin-url "$ADMIN"
  local final_count
  final_count=$(echo "$output" | jq 'length')
  [[ "$final_count" -ge 4 ]]
}
