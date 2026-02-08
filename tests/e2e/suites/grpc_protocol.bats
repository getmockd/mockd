#!/usr/bin/env bats
# ============================================================================
# gRPC Protocol — mock creation, reflection, unary RPC, streaming, lifecycle
# ============================================================================
# Uses grpcurl to test actual gRPC protocol behavior against mockd.
# Proto file: /fixtures/test.proto (copied into both mockd and runner containers)

GRPC_PORT=50051
GRPC_STATE_FILE="/tmp/grpc_mock_id.txt"

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  api POST /mocks -d '{
    "type": "grpc",
    "name": "e2e-grpc-test",
    "grpc": {
      "port": '"$GRPC_PORT"',
      "protoFile": "/fixtures/test.proto",
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
  # Persist mock ID via temp file — bats runs tests in subshells so export won't work
  json_field '.id' > "$GRPC_STATE_FILE"

  # Wait for gRPC server to spin up
  sleep 3
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
  rm -f "$GRPC_STATE_FILE"
}

setup() {
  load '../lib/helpers'
}

# Helper to read the mock ID written by setup_file
grpc_mock_id() {
  cat "$GRPC_STATE_FILE"
}

@test "GRPC-001: Create gRPC mock returns 201" {
  local mid
  mid=$(grpc_mock_id)
  [[ -n "$mid" ]]
  api GET "/mocks/${mid}"
  [[ "$STATUS" == "200" ]]
}

@test "GRPC-002: Reflection lists UserService" {
  run grpcurl -plaintext "mockd:${GRPC_PORT}" list
  [[ "$output" == *"test.UserService"* ]]
}

@test "GRPC-003: Describe lists GetUser method" {
  run grpcurl -plaintext "mockd:${GRPC_PORT}" describe test.UserService
  [[ "$output" == *"GetUser"* ]]
}

@test "GRPC-004: Unary GetUser returns Alice" {
  run grpcurl -plaintext -d '{"id": "1"}' "mockd:${GRPC_PORT}" test.UserService/GetUser
  [[ "$output" == *"Alice"* ]]
}

@test "GRPC-005: Response includes email field" {
  run grpcurl -plaintext -d '{"id": "1"}' "mockd:${GRPC_PORT}" test.UserService/GetUser
  echo "$output" | jq -e '.email' >/dev/null 2>&1
}

@test "GRPC-006: Server streaming returns 2+ users" {
  run grpcurl -plaintext -d '{"page_size": 10}' "mockd:${GRPC_PORT}" test.UserService/ListUsers
  local stream_count
  stream_count=$(echo "$output" | grep -c '"name"' || true)
  [[ "$stream_count" -ge 2 ]]
}

@test "GRPC-007: GET /grpc admin endpoint returns 200" {
  api GET /grpc
  [[ "$STATUS" == "200" ]]
}

@test "GRPC-008: Delete gRPC mock returns 204" {
  local mid
  mid=$(grpc_mock_id)
  api DELETE "/mocks/${mid}"
  [[ "$STATUS" == "204" ]]
  sleep 2
}

@test "GRPC-009: gRPC server stopped after mock deletion" {
  run grpcurl -plaintext "mockd:${GRPC_PORT}" list
  [[ "$output" == *"connect"* || "$output" == *"refused"* || "$output" == *"unavailable"* || "$output" == *"error"* || "$output" == *"Error"* || "$status" -ne 0 ]]
}

@test "GRPC-010: Toggle gRPC mock to disabled" {
  # Create a fresh mock for disable test
  api POST /mocks -d '{
    "type": "grpc",
    "name": "e2e-grpc-disable-test",
    "grpc": {
      "port": '"$GRPC_PORT"',
      "protoFile": "/fixtures/test.proto",
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
  sleep 2

  api POST "/mocks/${disable_id}/toggle" -d '{"enabled": false}'
  [[ "$STATUS" == "200" ]]
  sleep 2
}

@test "GRPC-011: Disabled gRPC mock does not respond" {
  run grpcurl -plaintext "mockd:${GRPC_PORT}" test.HealthService/Check
  [[ "$output" == *"connect"* || "$output" == *"refused"* || "$output" == *"unavailable"* || "$output" == *"error"* || "$output" == *"Error"* || "$output" == *"unimplemented"* || "$status" -ne 0 ]]
}
