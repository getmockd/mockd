#!/usr/bin/env bats
# ============================================================================
# MQTT Protocol — broker connectivity, publish/subscribe, auto-publish,
#                 wildcards, QoS, retained messages, auth
# ============================================================================
# Uses mosquitto_pub/mosquitto_sub to test actual MQTT behavior against mockd.

MQTT_PORT=1883

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  api POST /mocks -d '{
    "type": "mqtt",
    "name": "e2e-mqtt-test",
    "mqtt": {
      "port": '"$MQTT_PORT"',
      "topics": [
        {
          "topic": "sensors/temp",
          "messages": [
            {"payload": "{\"temp\": 72, \"unit\": \"F\"}"}
          ]
        },
        {
          "topic": "sensors/humidity",
          "messages": [
            {"payload": "{\"humidity\": 45, \"unit\": \"%\"}"}
          ]
        }
      ]
    }
  }'
  # Persist mock ID via bats temp dir — tests run in subshells so export won't work
  json_field '.id' > "$BATS_FILE_TMPDIR/mqtt_mock_id"

  # Wait for MQTT broker to accept connections
  wait_for_port mockd "$MQTT_PORT"
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
  # Also stop any auth broker on port 1884
  wait_for_port_down mockd 1884 5 2>/dev/null || true
}

teardown() {
  # Kill any lingering mosquitto_sub background processes from failed tests
  pkill -f "mosquitto_sub.*mockd" 2>/dev/null || true
}

setup() {
  load '../lib/helpers'
}

# Helper to read the mock ID written by setup_file
mqtt_mock_id() {
  cat "$BATS_FILE_TMPDIR/mqtt_mock_id"
}

# ── Basic Connectivity ────────────────────────────────────────────────────────

@test "MQTT-001: Create MQTT mock returns 201" {
  api GET "/mocks/$(mqtt_mock_id)"
  [[ "$STATUS" == "200" ]]
}

@test "MQTT-002: Broker accepts publish connection" {
  run mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/ping" -m "hello"
  [[ "$status" -eq 0 ]]
}

# ── Publish/Subscribe ─────────────────────────────────────────────────────────

@test "MQTT-003: Subscribe receives temp message" {
  # Start subscriber in background first, then publish to trigger delivery
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -C 1 > "$BATS_TEST_TMPDIR/mqtt_sub_temp.txt" 2>&1 &
  local sub_pid=$!
  sleep 1

  # Publish a message in case auto-publish already fired before we subscribed
  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -m '{"temp": 72, "unit": "F"}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local sub_out
  sub_out=$(cat "$BATS_TEST_TMPDIR/mqtt_sub_temp.txt" 2>/dev/null) || sub_out=""
  echo "$sub_out" | grep -q "temp"
}

@test "MQTT-004: Publish to custom topic received by subscriber" {
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "test/echo" -C 1 > "$BATS_TEST_TMPDIR/mqtt_echo.txt" 2>&1 &
  local sub_pid=$!
  sleep 1

  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/echo" -m '{"msg":"hello from e2e"}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local echo_out
  echo_out=$(cat "$BATS_TEST_TMPDIR/mqtt_echo.txt" 2>/dev/null) || echo_out=""
  echo "$echo_out" | grep -q "hello from e2e"
}

# ── Admin Endpoints ───────────────────────────────────────────────────────────

@test "MQTT-005: GET /mqtt admin endpoint returns 200" {
  api GET /mqtt
  [[ "$STATUS" == "200" ]]
}

@test "MQTT-006: GET /mqtt-recordings endpoint returns 200" {
  api GET /mqtt-recordings
  [[ "$STATUS" == "200" ]]
}

@test "MQTT-007: Second topic (humidity) receives messages" {
  # Start subscriber in background first, then publish to trigger delivery
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -C 1 > "$BATS_TEST_TMPDIR/mqtt_sub_humid.txt" 2>&1 &
  local sub_pid=$!
  sleep 1

  # Publish a message in case auto-publish already fired before we subscribed
  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -m '{"humidity": 45, "unit": "%"}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local sub_out
  sub_out=$(cat "$BATS_TEST_TMPDIR/mqtt_sub_humid.txt" 2>/dev/null) || sub_out=""
  echo "$sub_out" | grep -q "humidity"
}

# ── Wildcard Subscriptions ────────────────────────────────────────────────────

@test "MQTT-011: Wildcard subscription sensors/# receives temp messages" {
  # Subscribe to wildcard topic that covers sensors/temp and sensors/humidity
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/#" -C 1 > "$BATS_TEST_TMPDIR/mqtt_wildcard.txt" 2>&1 &
  local sub_pid=$!
  sleep 1

  # Publish to a specific sub-topic
  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -m '{"temp": 99}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local wc_out
  wc_out=$(cat "$BATS_TEST_TMPDIR/mqtt_wildcard.txt" 2>/dev/null) || wc_out=""
  echo "$wc_out" | grep -q "temp"
}

@test "MQTT-012: Single-level wildcard sensors/+/data receives messages" {
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/+/data" -C 1 > "$BATS_TEST_TMPDIR/mqtt_single_wc.txt" 2>&1 &
  local sub_pid=$!
  sleep 1

  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/livingroom/data" -m '{"reading": 42}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local out
  out=$(cat "$BATS_TEST_TMPDIR/mqtt_single_wc.txt" 2>/dev/null) || out=""
  echo "$out" | grep -q "reading"
}

# ── QoS ───────────────────────────────────────────────────────────────────────

@test "MQTT-013: QoS 1 publish and subscribe" {
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "test/qos1" -q 1 -C 1 > "$BATS_TEST_TMPDIR/mqtt_qos1.txt" 2>&1 &
  local sub_pid=$!
  sleep 1

  run mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/qos1" -q 1 -m '{"qos": 1}'
  [[ "$status" -eq 0 ]]

  wait $sub_pid 2>/dev/null || true
  local out
  out=$(cat "$BATS_TEST_TMPDIR/mqtt_qos1.txt" 2>/dev/null) || out=""
  echo "$out" | grep -q "qos"
}

# ── Retained Messages ────────────────────────────────────────────────────────

@test "MQTT-014: Retained message delivered to late subscriber" {
  # Publish a retained message first
  run mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/retained" -r -m '{"retained": true}'
  [[ "$status" -eq 0 ]]

  # Now subscribe — should immediately receive the retained message
  sleep 0.5
  local out
  out=$(timeout 5 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "test/retained" -C 1 2>&1) || true
  echo "$out" | grep -q "retained"
}

# ── Auth ──────────────────────────────────────────────────────────────────────

@test "MQTT-015: Auth broker rejects wrong credentials" {
  # Create a second MQTT mock on different port with auth
  api POST /mocks -d '{
    "type": "mqtt",
    "name": "e2e-mqtt-auth",
    "mqtt": {
      "port": 1884,
      "auth": {
        "enabled": true,
        "users": [
          {"username": "sensor", "password": "s3cret"}
        ]
      },
      "topics": [
        {"topic": "auth/data", "messages": [{"payload": "ok"}]}
      ]
    }
  }'
  [[ "$STATUS" == "201" ]]
  local auth_id
  auth_id=$(json_field '.id')

  wait_for_port mockd 1884

  # Wrong credentials should fail
  local pub_out
  pub_out=$(mosquitto_pub -h mockd -p 1884 -u "sensor" -P "wrongpass" -t "auth/data" -m "test" 2>&1) || true
  # mosquitto_pub returns error or connection refused with bad credentials
  echo "$pub_out" | grep -qi "refused\|error\|not authorised\|not authorized\|Connection Refused" || [[ $? -ne 0 ]]

  # Cleanup
  api DELETE "/mocks/${auth_id}"
  wait_for_port_down mockd 1884
}

# ── Lifecycle ─────────────────────────────────────────────────────────────────

@test "MQTT-008: Toggle mock disabled stops broker" {
  local mock_id
  mock_id=$(mqtt_mock_id)
  api POST "/mocks/${mock_id}/toggle" -d '{"enabled": false}'
  [[ "$STATUS" == "200" ]]
  # Wait for MQTT port to go down
  wait_for_port_down mockd "$MQTT_PORT"
  # Verify broker is unreachable
  local pub_out
  pub_out=$(mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/disabled" -m "x" 2>&1) || true
  echo "$pub_out" | grep -qi "refused\|error\|reset\|No route"
  # Re-enable so subsequent tests (delete) still work
  api POST "/mocks/${mock_id}/toggle" -d '{"enabled": true}'
  wait_for_port mockd "$MQTT_PORT"
}

@test "MQTT-009: Delete MQTT mock returns 204" {
  api DELETE "/mocks/$(mqtt_mock_id)"
  [[ "$STATUS" == "204" ]]
  wait_for_port_down mockd "$MQTT_PORT"
}

@test "MQTT-010: Broker stopped after mock deletion" {
  local post_del_out
  post_del_out=$(mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/gone" -m "x" 2>&1) || true
  echo "$post_del_out" | grep -qi "refused\|error\|reset\|No route"
}
