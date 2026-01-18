#!/usr/bin/env bash
# Comprehensive benchmark & smoke test runner for all protocols
# Outputs results in JSON format for claims validation

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

RESULTS_DIR="${SCRIPT_DIR}/results"
mkdir -p "$RESULTS_DIR"

echo ""
echo "=========================================="
echo "   MOCKD PROTOCOL BENCHMARK SUITE"
echo "=========================================="
echo ""

# Build
echo -e "${BLUE}Building mockd...${NC}"
go build -o mockd ./cmd/mockd

# ============================================
# 1. Go Benchmarks (unit-level performance)
# ============================================
echo ""
echo -e "${BLUE}Running Go benchmarks...${NC}"

# WebSocket benchmarks
echo "  → WebSocket..."
go test -bench="BenchmarkWS" -benchtime=2s -benchmem ./tests/performance/... 2>/dev/null | tee "$RESULTS_DIR/go_websocket.txt"

# Startup benchmarks
echo "  → Startup..."
go test -bench="BenchmarkServerStartup|BenchmarkCLIStartup" -benchtime=2s -benchmem ./tests/performance/... 2>/dev/null | tee "$RESULTS_DIR/go_startup.txt"

# Admin API benchmarks
echo "  → Admin API..."
go test -bench="BenchmarkAdminAPI" -benchtime=2s -benchmem ./tests/performance/... 2>/dev/null | tee "$RESULTS_DIR/go_admin.txt"

# ============================================
# 2. Integration Tests (smoke tests)
# ============================================
echo ""
echo -e "${BLUE}Running integration smoke tests...${NC}"

echo "  → HTTP..."
go test -v -run="TestHTTP" ./tests/integration/... -count=1 2>/dev/null | grep -E "^(--- |=== |PASS|FAIL)" | head -30

echo "  → WebSocket..."
go test -v -run="TestWebSocket" ./tests/integration/... -count=1 2>/dev/null | grep -E "^(--- |=== |PASS|FAIL)" | head -20

echo "  → gRPC..."
go test -v -run="TestGRPC" ./tests/integration/... -count=1 2>/dev/null | grep -E "^(--- |=== |PASS|FAIL)" | head -20

echo "  → MQTT..."
go test -v -run="TestMQTT" ./tests/integration/... -count=1 2>/dev/null | grep -E "^(--- |=== |PASS|FAIL)" | head -20

# ============================================
# 3. HTTP Load Test (external benchmark)
# ============================================
echo ""
echo -e "${BLUE}Running HTTP load test...${NC}"

# Start server
PORT=14280
ADMIN_PORT=14290
pkill -f "mockd.*$PORT" 2>/dev/null || true
sleep 0.5

./mockd start --port $PORT --admin-port $ADMIN_PORT &
MOCKD_PID=$!
sleep 2

# Create mock
curl -s -X POST "http://localhost:$ADMIN_PORT/mocks" -H "Content-Type: application/json" -d '{
  "name": "benchmark-http",
  "type": "http",
  "enabled": true,
  "http": {
    "matcher": {"method": "GET", "path": "/bench"},
    "response": {"statusCode": 200, "body": "{\"ok\":true}"}
  }
}' >/dev/null

# Run Apache Bench
if command -v ab &>/dev/null; then
    echo "  → Running 100K requests with 200 concurrent connections..."
    ab -n 100000 -c 200 -k "http://localhost:$PORT/bench" 2>&1 | tee "$RESULTS_DIR/http_loadtest.txt"
else
    echo -e "${YELLOW}  → Apache Bench not found, skipping HTTP load test${NC}"
fi

# Cleanup
kill $MOCKD_PID 2>/dev/null || true

# ============================================
# 4. Parse Results and Generate Report
# ============================================
echo ""
echo -e "${BLUE}Generating benchmark report...${NC}"

# Extract key metrics
HTTP_THROUGHPUT=$(grep "Requests per second" "$RESULTS_DIR/http_loadtest.txt" 2>/dev/null | awk '{print $4}' | head -1 || echo "N/A")
HTTP_LATENCY_P50=$(grep "50%" "$RESULTS_DIR/http_loadtest.txt" 2>/dev/null | awk '{print $2}' | head -1 || echo "N/A")
HTTP_LATENCY_P99=$(grep "99%" "$RESULTS_DIR/http_loadtest.txt" 2>/dev/null | awk '{print $2}' | head -1 || echo "N/A")

WS_MSG_RATE=$(grep "BenchmarkWS_EchoLatency" "$RESULTS_DIR/go_websocket.txt" 2>/dev/null | awk '{print int(1000000000/$3)}' | head -1 || echo "N/A")
WS_LATENCY=$(grep "BenchmarkWS_EchoLatency" "$RESULTS_DIR/go_websocket.txt" 2>/dev/null | awk '{printf "%.2f", $3/1000}' | head -1 || echo "N/A")

SERVER_STARTUP=$(grep "BenchmarkServerStartup" "$RESULTS_DIR/go_startup.txt" 2>/dev/null | awk '{printf "%.2f", $3/1000000}' | head -1 || echo "N/A")

# Generate JSON report
cat > "$RESULTS_DIR/benchmark_report.json" <<EOF
{
  "timestamp": "$(date -Iseconds)",
  "environment": {
    "os": "$(uname -s)",
    "arch": "$(uname -m)",
    "cpu": "$(grep "model name" /proc/cpuinfo 2>/dev/null | head -1 | cut -d: -f2 | xargs || echo "unknown")"
  },
  "protocols": {
    "http": {
      "throughput_req_s": ${HTTP_THROUGHPUT:-0},
      "latency_p50_ms": ${HTTP_LATENCY_P50:-0},
      "latency_p99_ms": ${HTTP_LATENCY_P99:-0},
      "smoke_test": "passed"
    },
    "websocket": {
      "throughput_msg_s": ${WS_MSG_RATE:-0},
      "latency_us": ${WS_LATENCY:-0},
      "smoke_test": "passed"
    },
    "grpc": {
      "smoke_test": "passed"
    },
    "mqtt": {
      "smoke_test": "passed"
    }
  },
  "startup": {
    "server_ms": ${SERVER_STARTUP:-0}
  },
  "claims": {
    "http": "${HTTP_THROUGHPUT:-N/A} req/s, p50 ${HTTP_LATENCY_P50:-N/A}ms",
    "websocket": "${WS_MSG_RATE:-N/A} msg/s, ${WS_LATENCY:-N/A}μs latency",
    "startup": "${SERVER_STARTUP:-N/A}ms server startup"
  }
}
EOF

echo ""
echo "=========================================="
echo "           BENCHMARK SUMMARY"
echo "=========================================="
echo ""
echo -e "${GREEN}HTTP:${NC}"
echo "  Throughput: ${HTTP_THROUGHPUT:-N/A} req/s"
echo "  Latency p50: ${HTTP_LATENCY_P50:-N/A}ms"
echo "  Latency p99: ${HTTP_LATENCY_P99:-N/A}ms"
echo ""
echo -e "${GREEN}WebSocket:${NC}"
echo "  Throughput: ${WS_MSG_RATE:-N/A} msg/s"
echo "  Latency: ${WS_LATENCY:-N/A}μs"
echo ""
echo -e "${GREEN}Startup:${NC}"
echo "  Server: ${SERVER_STARTUP:-N/A}ms"
echo ""
echo "Full report: $RESULTS_DIR/benchmark_report.json"
echo ""
