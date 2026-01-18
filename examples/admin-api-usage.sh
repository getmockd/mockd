#!/bin/bash
# Admin API Usage Examples
# This script demonstrates how to use the mockd admin API

BASE_URL="http://localhost:4290"

echo "=== mockd Admin API Examples ==="
echo ""

# Health check
echo "1. Health Check"
echo "   curl $BASE_URL/health"
curl -s "$BASE_URL/health" | jq .
echo ""

# Create a mock
echo "2. Create a Mock"
echo '   curl -X POST $BASE_URL/mocks -H "Content-Type: application/json" -d ...'
MOCK_RESPONSE=$(curl -s -X POST "$BASE_URL/mocks" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Get Users",
    "matcher": {
      "method": "GET",
      "path": "/api/users"
    },
    "response": {
      "statusCode": 200,
      "headers": {
        "Content-Type": "application/json"
      },
      "body": "{\"users\": [\"Alice\", \"Bob\", \"Charlie\"]}"
    }
  }')
echo "$MOCK_RESPONSE" | jq .
MOCK_ID=$(echo "$MOCK_RESPONSE" | jq -r '.id')
echo ""

# List all mocks
echo "3. List All Mocks"
echo "   curl $BASE_URL/mocks"
curl -s "$BASE_URL/mocks" | jq .
echo ""

# Get a specific mock
echo "4. Get Specific Mock"
echo "   curl $BASE_URL/mocks/$MOCK_ID"
curl -s "$BASE_URL/mocks/$MOCK_ID" | jq .
echo ""

# Update a mock
echo "5. Update a Mock"
echo '   curl -X PUT $BASE_URL/mocks/$MOCK_ID -H "Content-Type: application/json" -d ...'
curl -s -X PUT "$BASE_URL/mocks/$MOCK_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Get Users (Updated)",
    "matcher": {
      "method": "GET",
      "path": "/api/users"
    },
    "response": {
      "statusCode": 200,
      "headers": {
        "Content-Type": "application/json"
      },
      "body": "{\"users\": [\"Alice\", \"Bob\", \"Charlie\", \"Dave\"]}"
    },
    "enabled": true
  }' | jq .
echo ""

# Toggle mock enabled status
echo "6. Toggle Mock (Disable)"
echo '   curl -X POST $BASE_URL/mocks/$MOCK_ID/toggle -H "Content-Type: application/json" -d ...'
curl -s -X POST "$BASE_URL/mocks/$MOCK_ID/toggle" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}' | jq .
echo ""

# Filter mocks by enabled status
echo "7. List Disabled Mocks"
echo "   curl $BASE_URL/mocks?enabled=false"
curl -s "$BASE_URL/mocks?enabled=false" | jq .
echo ""

# Re-enable the mock
echo "8. Toggle Mock (Enable)"
curl -s -X POST "$BASE_URL/mocks/$MOCK_ID/toggle" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}' | jq .
echo ""

# Test the mock endpoint (on mock server port, default 4280)
echo "9. Test Mock Endpoint"
echo "   curl http://localhost:4280/api/users"
curl -s "http://localhost:4280/api/users"
echo ""
echo ""

# Delete a mock
echo "10. Delete a Mock"
echo "    curl -X DELETE $BASE_URL/mocks/$MOCK_ID"
curl -s -X DELETE "$BASE_URL/mocks/$MOCK_ID" -w "HTTP Status: %{http_code}\n"
echo ""

# Verify mock is deleted
echo "11. Verify Deletion (should return 404)"
echo "    curl $BASE_URL/mocks/$MOCK_ID"
curl -s "$BASE_URL/mocks/$MOCK_ID" | jq .
echo ""

echo "=== Examples Complete ==="
