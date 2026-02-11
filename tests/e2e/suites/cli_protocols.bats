#!/usr/bin/env bats
# ============================================================================
# CLI Protocol-Specific Adds — SSE, GraphQL, WebSocket, SOAP, body-file
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
  rm -f /tmp/cli-body.json
}

setup() {
  load '../lib/helpers'
}

@test "CLI-SSE-001: mockd add --sse creates SSE mock" {
  run mockd add --path /cli/events --sse \
    --sse-event "message:hello from CLI" \
    --sse-event "update:world" \
    --sse-delay 50 --name "CLI SSE" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Created mock"* ]]
}

@test "CLI-SSE-002: SSE stream delivers events" {
  local sse_output
  sse_output=$(curl -s -N --max-time 2 -H 'Accept: text/event-stream' "${ENGINE}/cli/events" 2>&1) || true
  [[ "$sse_output" == *"hello from CLI"* ]]
}

@test "CLI-GQL-001: mockd add --type graphql creates GraphQL mock" {
  run mockd add --type graphql --path /cli/graphql \
    --operation getUser --op-type query \
    --response '{"id":"1","name":"CLI User"}' \
    --name "CLI GraphQL" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-GQL-002: GraphQL mock responds with data" {
  engine POST /cli/graphql -d '{"query":"{ getUser { id name } }"}'
  [[ "$STATUS" == "200" ]]
}

@test "CLI-WS-PROTO-001: mockd add --type websocket creates WebSocket mock" {
  run mockd add --type websocket --path /cli/ws --echo --name "CLI WebSocket" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Created mock"* ]]
}

@test "CLI-WS-PROTO-002: WebSocket handler registered" {
  api GET /handlers
  [[ "$STATUS" == "200" ]]
}

@test "CLI-SOAP-001: mockd add --type soap creates SOAP mock" {
  run mockd add --type soap --path /cli/soap \
    --operation GetProduct --soap-action "http://example.com/GetProduct" \
    --response '<GetProductResponse><id>42</id><name>Widget</name></GetProductResponse>' \
    --name "CLI SOAP" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]
}

@test "CLI-SOAP-002: SOAP mock responds with data" {
  local soap_resp
  soap_resp=$(curl -s -w '\n%{http_code}' -X POST "${ENGINE}/cli/soap" \
    -H 'Content-Type: text/xml' \
    -H 'SOAPAction: http://example.com/GetProduct' \
    -d '<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetProduct/></soap:Body></soap:Envelope>' 2>&1) || true
  BODY=$(echo "$soap_resp" | sed '$d')
  STATUS=$(echo "$soap_resp" | tail -n 1)
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"Widget"* ]]
}

@test "CLI-FILE-001: mockd add --body-file creates mock from file" {
  echo '{"from":"file","ok":true}' > /tmp/cli-body.json
  run mockd add --path /cli/from-file --body-file /tmp/cli-body.json --name "Body From File" --admin-url "$ADMIN"
  [[ "$status" -eq 0 ]]

  engine GET /cli/from-file
  [[ "$STATUS" == "200" ]]
  [[ "$(json_field '.from')" == "file" ]]
}
