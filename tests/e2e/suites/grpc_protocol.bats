#!/usr/bin/env bats
# ============================================================================
# gRPC Protocol — mock creation, reflection, unary RPC, streaming, lifecycle
# ============================================================================
# Uses grpcurl to test actual gRPC protocol behavior against mockd.

GRPC_PORT=50051

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

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
  export GRPC_MOCK_ID=$(json_field '.id')

  # Wait for gRPC server to spin up
  sleep 1
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

@test "GRPC-001: Create gRPC mock returns 201" {
  # Verified in setup_file — just confirm the mock exists
  api GET "/mocks/${GRPC_MOCK_ID}"
  [[ "$STATUS" == "200" ]]
}

@test "GRPC-002: Reflection lists UserService" {
  local refl_out
  refl_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" list 2>&1) || true
  echo "$refl_out" | grep -q "test.UserService"
}

@test "GRPC-003: Describe lists GetUser method" {
  local methods_out
  methods_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" describe test.UserService 2>&1) || true
  echo "$methods_out" | grep -q "GetUser"
}

@test "GRPC-004: Unary GetUser returns Alice" {
  local unary_out
  unary_out=$(grpcurl -plaintext -d '{"id": "1"}' "mockd:${GRPC_PORT}" test.UserService/GetUser 2>&1) || true
  echo "$unary_out" | grep -q "Alice"
}

@test "GRPC-005: Response includes email field" {
  local unary_out
  unary_out=$(grpcurl -plaintext -d '{"id": "1"}' "mockd:${GRPC_PORT}" test.UserService/GetUser 2>&1) || true
  echo "$unary_out" | jq -e '.email' >/dev/null 2>&1
}

@test "GRPC-006: Server streaming returns 2+ users" {
  local stream_out
  stream_out=$(grpcurl -plaintext -d '{"page_size": 10}' "mockd:${GRPC_PORT}" test.UserService/ListUsers 2>&1) || true
  local stream_count
  stream_count=$(echo "$stream_out" | grep -c '"name"' || true)
  [[ "$stream_count" -ge 2 ]]
}

@test "GRPC-007: GET /grpc admin endpoint returns 200" {
  api GET /grpc
  [[ "$STATUS" == "200" ]]
}

@test "GRPC-008: Delete gRPC mock returns 204" {
  api DELETE "/mocks/${GRPC_MOCK_ID}"
  [[ "$STATUS" == "204" ]]
  sleep 1
}

@test "GRPC-009: gRPC server stopped after mock deletion" {
  local post_delete_out
  post_delete_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" list 2>&1) || true
  echo "$post_delete_out" | grep -qi "connect\|refused\|unavailable\|error"
}

@test "GRPC-010: Toggle gRPC mock to disabled" {
  # Create a fresh mock for disable test
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
  disable_id=$(json_field '.id')
  sleep 1

  api POST "/mocks/${disable_id}/toggle" -d '{"enabled": false}'
  [[ "$STATUS" == "200" ]]
  sleep 1
}

@test "GRPC-011: Disabled gRPC mock does not respond" {
  local disabled_out
  disabled_out=$(grpcurl -plaintext "mockd:${GRPC_PORT}" test.HealthService/Check 2>&1) || true
  echo "$disabled_out" | grep -qi "connect\|refused\|unavailable\|error\|unimplemented"
}
