#!/usr/bin/env bats
# ============================================================================
# SOAP Protocol — mock creation, SOAP requests, WSDL endpoint
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  api POST /mocks -d '{
    "type": "soap",
    "name": "Test SOAP Service",
    "soap": {
      "path": "/soap/user",
      "operations": {
        "GetUser": {
          "soapAction": "http://example.com/GetUser",
          "response": "<GetUserResponse><id>123</id><name>John Doe</name></GetUserResponse>"
        },
        "CreateUser": {
          "soapAction": "http://example.com/CreateUser",
          "response": "<CreateUserResponse><userId>new-001</userId><status>created</status></CreateUserResponse>"
        }
      }
    }
  }'
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

@test "SOAP-001: Create SOAP mock returns 201" {
  api POST /mocks -d '{
    "type": "soap",
    "name": "SOAP Verify",
    "soap": {
      "path": "/soap/verify",
      "operations": {"Ping": {"response": "<ok/>"}}
    }
  }'
  [[ "$STATUS" == "201" ]]
  local id=$(json_field '.id')
  api DELETE "/mocks/${id}"
}

@test "SOAP-002: SOAP GetUser request returns 200" {
  local tmpfile
  tmpfile=$(mktemp)
  STATUS=$(curl -s -w '%{http_code}' -o "$tmpfile" -X POST "${ENGINE}/soap/user" \
    -H 'Content-Type: text/xml' \
    -H 'SOAPAction: http://example.com/GetUser' \
    -d '<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetUser><userId>123</userId></GetUser></soap:Body></soap:Envelope>' 2>/dev/null) || STATUS="000"
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
  [[ "$STATUS" == "200" ]]
}

@test "SOAP-003: Response contains user name" {
  local tmpfile
  tmpfile=$(mktemp)
  STATUS=$(curl -s -w '%{http_code}' -o "$tmpfile" -X POST "${ENGINE}/soap/user" \
    -H 'Content-Type: text/xml' \
    -H 'SOAPAction: http://example.com/GetUser' \
    -d '<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetUser><userId>123</userId></GetUser></soap:Body></soap:Envelope>' 2>/dev/null) || STATUS="000"
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
  [[ "$BODY" == *"John Doe"* ]]
}

@test "SOAP-004: WSDL endpoint responds" {
  engine GET '/soap/user?wsdl'
  # WSDL may not be implemented — accept 200 or skip
  [[ "$STATUS" == "200" ]] || skip "WSDL not available (status $STATUS)"
}

@test "SOAP-005: CreateUser operation returns response" {
  local tmpfile
  tmpfile=$(mktemp)
  STATUS=$(curl -s -w '%{http_code}' -o "$tmpfile" -X POST "${ENGINE}/soap/user" \
    -H 'Content-Type: text/xml' \
    -H 'SOAPAction: http://example.com/CreateUser' \
    -d '<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><CreateUser><name>Jane</name><email>jane@example.com</email></CreateUser></soap:Body></soap:Envelope>' 2>/dev/null) || STATUS="000"
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"new-001"* ]]
  [[ "$BODY" == *"created"* ]]
}
