#!/bin/bash
# ============================================================================
# gRPC Protocol Tests — uses grpcurl against mockd gRPC server
# ============================================================================
# Tests gRPC mock creation, reflection, unary RPC, server streaming,
# and lifecycle management (delete stops server, disable stops responding).

GRPC_PORT=50051

run_grpc_protocol() {
  suite_header "GRPC: Protocol Tests (grpcurl)"

  # Clean slate
  api DELETE /mocks

  # ── Create gRPC mock with inline proto ──
  api POST /mocks -d '{
    "type": "grpc",
    "name": "e2e-grpc-test",
    "grpc": {
      "port": '"$GRPC_PORT"',
      "proto": "syntax = \"proto3\";\npackage test;\nservice UserService {\n  rpc GetUser(GetUserRequest) returns (User);\n  rpc ListUsers(ListUsersRequest) returns (stream User);\n}\nmessage GetUserRequest { string id = 1; }\nmessage User { string id = 1; string name = 2; string email = 3; }\nmessage ListUsersRequest { int32 page_size = 1; }",
      "reflection": true,
      "services": {
        "test.UserService": {
          "methods": {
            "GetUser": {
              "response": {"id": "1", "name": "Alice", "email": "alice@test.com"}
            },
            "ListUsers": {
              "responses": [
                {"id": "1", "name": "Alice", "email": "alice@test.com"},
                {"id": "2", "name": "Bob", "email": "bob@test.com"}
              ]
            }
          }
        }
      }
    }
  }'
  assert_status 201 "GRPC-001: Create gRPC mock with inline proto"
  local grpc_id
  grpc_id=$(echo "$BODY" | jq -r '.id')

  # Wait for gRPC server to spin up
  sleep 1

  # ── Reflection: list services ──
  local refl_out
  refl_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" list 2>&1) || true
  if echo "$refl_out" | grep -q "test.UserService"; then
    pass "GRPC-002: Reflection lists UserService"
  else
    fail "GRPC-002: Reflection lists UserService" "got: $(echo "$refl_out" | head -c 200)"
  fi

  # ── Reflection: describe methods ──
  local methods_out
  methods_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" describe test.UserService 2>&1) || true
  if echo "$methods_out" | grep -q "GetUser"; then
    pass "GRPC-003: Describe lists GetUser method"
  else
    fail "GRPC-003: Describe lists GetUser method" "got: $(echo "$methods_out" | head -c 200)"
  fi

  # ── Unary RPC ──
  local unary_out
  unary_out=$(grpcurl -plaintext -d '{"id": "1"}' "mockd:${GRPC_PORT}" test.UserService/GetUser 2>&1) || true
  if echo "$unary_out" | grep -q "Alice"; then
    pass "GRPC-004: Unary GetUser returns Alice"
  else
    fail "GRPC-004: Unary GetUser returns Alice" "got: $(echo "$unary_out" | head -c 200)"
  fi

  # ── Response has all fields ──
  if echo "$unary_out" | jq -e '.email' >/dev/null 2>&1; then
    pass "GRPC-005: Response includes email field"
  else
    fail "GRPC-005: Response includes email field" "got: $(echo "$unary_out" | head -c 200)"
  fi

  # ── Server streaming ──
  local stream_out
  stream_out=$(grpcurl -plaintext -d '{"page_size": 10}' "mockd:${GRPC_PORT}" test.UserService/ListUsers 2>&1) || true
  local stream_count
  stream_count=$(echo "$stream_out" | grep -c '"name"' || true)
  if [[ "$stream_count" -ge 2 ]]; then
    pass "GRPC-006: Server streaming returns 2+ users"
  else
    fail "GRPC-006: Server streaming returns 2+ users" "got $stream_count names"
  fi

  # ── Admin /grpc endpoint ──
  api GET /grpc
  assert_status 200 "GRPC-007: GET /grpc admin endpoint returns 200"

  # ── Delete stops server ──
  api DELETE "/mocks/${grpc_id}"
  assert_status 204 "GRPC-008: Delete gRPC mock returns 204"
  sleep 1

  local post_delete_out
  post_delete_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" list 2>&1) || true
  if echo "$post_delete_out" | grep -qi "connect\|refused\|unavailable\|error"; then
    pass "GRPC-009: gRPC server stopped after mock deletion"
  else
    fail "GRPC-009: gRPC server stopped after mock deletion" "server still responding: $(echo "$post_delete_out" | head -c 200)"
  fi

  # ── Disable stops responding ──
  api POST /mocks -d '{
    "type": "grpc",
    "name": "e2e-grpc-disable-test",
    "grpc": {
      "port": '"$GRPC_PORT"',
      "proto": "syntax = \"proto3\";\npackage test;\nservice HealthService {\n  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);\n}\nmessage HealthCheckRequest { string service = 1; }\nmessage HealthCheckResponse { string status = 1; }",
      "reflection": true,
      "services": {
        "test.HealthService": {
          "methods": {
            "Check": {
              "response": {"status": "SERVING"}
            }
          }
        }
      }
    }
  }'
  local disable_id
  disable_id=$(echo "$BODY" | jq -r '.id')
  sleep 1

  api POST "/mocks/${disable_id}/toggle" -d '{"enabled": false}'
  assert_status 200 "GRPC-010: Toggle gRPC mock to disabled"
  sleep 1

  local disabled_out
  disabled_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" test.HealthService/Check 2>&1) || true
  if echo "$disabled_out" | grep -qi "connect\|refused\|unavailable\|error\|unimplemented"; then
    pass "GRPC-011: Disabled gRPC mock does not respond"
  else
    fail "GRPC-011: Disabled gRPC mock does not respond" "got: $(echo "$disabled_out" | head -c 200)"
  fi

  # Cleanup
  api DELETE /mocks
}
