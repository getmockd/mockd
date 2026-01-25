#!/bin/bash
# Load all comprehensive demo mocks into mockd

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_KEY="${MOCKD_API_KEY:-$(cat ~/.local/share/mockd/admin-api-key 2>/dev/null)}"
ADMIN_URL="${MOCKD_ADMIN_URL:-http://localhost:4290}"

if [ -z "$API_KEY" ]; then
    echo "Error: No API key found. Set MOCKD_API_KEY or ensure ~/.local/share/mockd/admin-api-key exists"
    exit 1
fi

echo "Loading comprehensive demos into mockd..."
echo "Admin URL: $ADMIN_URL"
echo ""

# Copy proto file to /tmp for gRPC mocks (they reference ./demo.proto which needs to resolve)
if [ -f "$SCRIPT_DIR/demo.proto" ]; then
    cp "$SCRIPT_DIR/demo.proto" /tmp/demo.proto
    echo "Copied demo.proto to /tmp/"
fi

load_file() {
    local file="$1"
    local name=$(basename "$file" .json)
    
    if [ ! -f "$file" ]; then
        echo "  SKIP: $file (not found)"
        return
    fi
    
    echo -n "  Loading $name... "
    
    # Import using the config endpoint (requires {"config": {"mocks": [...]}} wrapper)
    response=$(curl -s -X POST "$ADMIN_URL/config?api_key=$API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"config\": {\"version\": \"1.0\", \"mocks\": $(cat "$file")}}" 2>&1)
    
    if echo "$response" | grep -q '"imported"'; then
        imported=$(echo "$response" | grep -o '"imported":[0-9]*' | cut -d: -f2)
        echo "OK ($imported mocks)"
    else
        echo "FAILED"
        echo "    Response: $response"
    fi
}

# Load each demo file
for demo in http-features-demo.json http-sse-features-demo.json ws-features-demo.json graphql-features-demo.json soap-features-demo.json; do
    load_file "$SCRIPT_DIR/$demo"
done

# Load stateful resources (different format - direct config with statefulResources)
if [ -f "$SCRIPT_DIR/stateful-features-demo.json" ]; then
    echo -n "  Loading stateful-features-demo... "
    
    response=$(curl -s -X POST "$ADMIN_URL/config?api_key=$API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"config\": {\"version\": \"1.0\", $(cat "$SCRIPT_DIR/stateful-features-demo.json" | tail -c +2 | head -c -2)}}" 2>&1)
    
    if echo "$response" | grep -q '"statefulResources"'; then
        count=$(echo "$response" | grep -o '"statefulResources":[0-9]*' | cut -d: -f2)
        echo "OK ($count resources)"
    elif echo "$response" | grep -q '"imported"'; then
        echo "OK"
    else
        echo "FAILED"
        echo "    Response: $response"
    fi
fi

# gRPC needs special handling - update proto paths first
if [ -f "$SCRIPT_DIR/grpc-features-demo.json" ]; then
    echo -n "  Loading grpc-features-demo... "
    
    # Update proto paths to absolute /tmp path and import
    updated=$(cat "$SCRIPT_DIR/grpc-features-demo.json" | sed 's|"./demo.proto"|"/tmp/demo.proto"|g')
    
    response=$(curl -s -X POST "$ADMIN_URL/config?api_key=$API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"config\": {\"version\": \"1.0\", \"mocks\": $updated}}" 2>&1)
    
    if echo "$response" | grep -q '"imported"'; then
        imported=$(echo "$response" | grep -o '"imported":[0-9]*' | cut -d: -f2)
        echo "OK ($imported mocks)"
    else
        echo "FAILED"
        echo "    Response: $response"
    fi
fi

echo ""
echo "Done! View mocks at: $ADMIN_URL/mocks?api_key=$API_KEY"
echo ""
echo "Export to Insomnia:"
echo "  $ADMIN_URL/insomnia.yaml?api_key=$API_KEY"
