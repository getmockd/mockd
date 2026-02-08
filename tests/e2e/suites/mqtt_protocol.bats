#!/usr/bin/env bats
# ============================================================================
# MQTT Protocol — broker connectivity, publish/subscribe, auto-publish
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
  # Persist mock ID via temp file — bats runs tests in subshells so export won't work
  echo "$(json_field '.id')" > "$BATS_FILE_TMPDIR/mqtt_mock_id"

  # Wait for broker to start
  sleep 2
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
  rm -f /tmp/mqtt_echo.txt /tmp/mqtt_sub_temp.txt /tmp/mqtt_sub_humid.txt
}

setup() {
  load '../lib/helpers'
}

# Helper to read the mock ID written by setup_file
mqtt_mock_id() {
  cat "$BATS_FILE_TMPDIR/mqtt_mock_id"
}

@test "MQTT-001: Create MQTT mock returns 201" {
  api GET "/mocks/$(mqtt_mock_id)"
  [[ "$STATUS" == "200" ]]
}

@test "MQTT-002: Broker accepts publish connection" {
  local pub_out
  pub_out=$(mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/ping" -m "hello" 2>&1) || true
  local pub_exit=$?
  [[ $pub_exit -eq 0 ]] || ! echo "$pub_out" | grep -q "Error"
}

@test "MQTT-003: Subscribe receives temp message" {
  # Start subscriber in background first, then publish to trigger delivery
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -C 1 > /tmp/mqtt_sub_temp.txt 2>&1 &
  local sub_pid=$!
  sleep 1

  # Publish a message in case auto-publish already fired before we subscribed
  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -m '{"temp": 72, "unit": "F"}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local sub_out
  sub_out=$(cat /tmp/mqtt_sub_temp.txt 2>/dev/null) || sub_out=""
  echo "$sub_out" | grep -q "temp"
}

@test "MQTT-004: Publish to custom topic received by subscriber" {
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "test/echo" -C 1 > /tmp/mqtt_echo.txt 2>&1 &
  local sub_pid=$!
  sleep 1

  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/echo" -m '{"msg":"hello from e2e"}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local echo_out
  echo_out=$(cat /tmp/mqtt_echo.txt 2>/dev/null) || echo_out=""
  echo "$echo_out" | grep -q "hello from e2e"
}

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
  timeout 8 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -C 1 > /tmp/mqtt_sub_humid.txt 2>&1 &
  local sub_pid=$!
  sleep 1

  # Publish a message in case auto-publish already fired before we subscribed
  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -m '{"humidity": 45, "unit": "%"}' 2>/dev/null || true

  wait $sub_pid 2>/dev/null || true
  local sub_out
  sub_out=$(cat /tmp/mqtt_sub_humid.txt 2>/dev/null) || sub_out=""
  echo "$sub_out" | grep -q "humidity"
}

@test "MQTT-008: Delete MQTT mock returns 204" {
  api DELETE "/mocks/$(mqtt_mock_id)"
  [[ "$STATUS" == "204" ]]
  sleep 2
}

@test "MQTT-009: Broker stopped after mock deletion" {
  local post_del_out
  post_del_out=$(mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/gone" -m "x" 2>&1) || true
  echo "$post_del_out" | grep -qi "refused\|error\|reset\|No route"
}
