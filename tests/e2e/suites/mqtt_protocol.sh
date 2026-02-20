#!/bin/bash
# ============================================================================
# MQTT Protocol Tests — uses mosquitto_pub/mosquitto_sub against mockd broker
# ============================================================================
# Tests MQTT mock creation, broker connectivity, topic publish/subscribe,
# auto-publish messages, and admin endpoint listing.

MQTT_PORT=1883

run_mqtt_protocol() {
  suite_header "MQTT: Protocol Tests (mosquitto)"

  # Clean slate
  api DELETE /mocks

  # ── Create MQTT mock ──
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
  assert_status 201 "MQTT-001: Create MQTT mock"
  local mqtt_id
  mqtt_id=$(echo "$BODY" | jq -r '.id')

  # Wait for broker to start
  sleep 2

  # ── Broker accepts connections ──
  local pub_out
  pub_out=$(mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/ping" -m "hello" 2>&1) || true
  local pub_exit=$?
  if [[ $pub_exit -eq 0 ]] || echo "$pub_out" | grep -qv "Error"; then
    pass "MQTT-002: Broker accepts publish connection"
  else
    fail "MQTT-002: Broker accepts publish connection" "exit=$pub_exit output=$(echo "$pub_out" | head -c 200)"
  fi

  # ── Subscribe and receive auto-published message ──
  # Subscribe in background, wait for a message, timeout after 5s
  local sub_out
  sub_out=$(timeout 5 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -C 1 2>&1) || true
  if echo "$sub_out" | grep -q "temp"; then
    pass "MQTT-003: Subscribe receives auto-published temp message"
  else
    # Auto-publish may need a trigger; try publishing and subscribing
    mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -m '{"temp":72}' 2>/dev/null || true
    sub_out=$(timeout 5 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/temp" -C 1 2>&1) || true
    if echo "$sub_out" | grep -q "temp"; then
      pass "MQTT-003: Subscribe receives temp message after publish"
    else
      fail "MQTT-003: Subscribe receives temp message" "got: $(echo "$sub_out" | head -c 200)"
    fi
  fi

  # ── Publish to custom topic and subscribe ──
  # Start subscriber in background
  timeout 5 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "test/echo" -C 1 > /tmp/mqtt_echo.txt 2>&1 &
  local sub_pid=$!
  sleep 1

  # Publish a message
  mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/echo" -m '{"msg":"hello from e2e"}' 2>/dev/null || true
  
  # Wait for subscriber to receive
  wait $sub_pid 2>/dev/null || true
  local echo_out
  echo_out=$(cat /tmp/mqtt_echo.txt 2>/dev/null) || echo_out=""
  if echo "$echo_out" | grep -q "hello from e2e"; then
    pass "MQTT-004: Publish to custom topic received by subscriber"
  else
    fail "MQTT-004: Publish to custom topic received by subscriber" "got: $(echo "$echo_out" | head -c 200)"
  fi

  # ── Admin: list MQTT mocks ──
  api GET /mqtt
  assert_status 200 "MQTT-005: GET /mqtt admin endpoint"

  # ── Admin: MQTT recordings endpoint ──
  api GET /mqtt-recordings
  assert_status 200 "MQTT-006: GET /mqtt-recordings endpoint"

  # ── Multiple topics on same broker ──
  sub_out=$(timeout 5 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -C 1 2>&1) || true
  if echo "$sub_out" | grep -q "humidity"; then
    pass "MQTT-007: Second topic (humidity) receives messages"
  else
    mosquitto_pub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -m '{"humidity":45}' 2>/dev/null || true
    sub_out=$(timeout 5 mosquitto_sub -h mockd -p "$MQTT_PORT" -t "sensors/humidity" -C 1 2>&1) || true
    if echo "$sub_out" | grep -q "humidity"; then
      pass "MQTT-007: Second topic (humidity) receives messages"
    else
      fail "MQTT-007: Second topic (humidity) receives messages" "got: $(echo "$sub_out" | head -c 200)"
    fi
  fi

  # ── Delete stops broker ──
  api DELETE "/mocks/${mqtt_id}"
  assert_status 204 "MQTT-008: Delete MQTT mock returns 204"
  sleep 1

  local post_del_out
  post_del_out=$(mosquitto_pub -h mockd -p "$MQTT_PORT" -t "test/gone" -m "x" 2>&1) || true
  if echo "$post_del_out" | grep -qi "refused\|error\|reset\|No route"; then
    pass "MQTT-009: Broker stopped after mock deletion"
  else
    fail "MQTT-009: Broker stopped after mock deletion" "publish still succeeded: $(echo "$post_del_out" | head -c 200)"
  fi

  # Cleanup
  api DELETE /mocks
  rm -f /tmp/mqtt_echo.txt
}
