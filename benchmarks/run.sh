#!/usr/bin/env bash
# Main benchmark runner for mockd
# Usage: ./run.sh [docker|local|all] [test_name]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

MODE="${1:-docker}"
TEST="${2:-all}"

usage() {
    echo "Usage: $0 [docker|local|all] [test_name]"
    echo ""
    echo "Modes:"
    echo "  docker  - Run benchmarks in Docker (isolated, reproducible)"
    echo "  local   - Run benchmarks on bare metal (real-world numbers)"
    echo "  all     - Run both docker and local benchmarks"
    echo ""
    echo "Tests:"
    echo "  all        - Run all benchmarks"
    echo "  throughput - Maximum requests/second test"
    echo "  latency    - Latency percentiles test"
    echo "  load       - Full load test with ramp-up"
    echo "  startup    - Startup timing test"
    echo "  memory     - Memory profiling test"
    exit 1
}

ensure_built() {
    if [[ ! -f "./mockd" ]]; then
        echo -e "${BLUE}Building mockd...${NC}"
        go build -o mockd ./cmd/mockd
    fi
}

run_docker() {
    local test_script="$1"
    echo -e "${BLUE}Running Docker benchmark: $test_script${NC}"

    mkdir -p benchmarks/results

    # Build and run
    docker compose -f benchmarks/docker-compose.yml build
    docker compose -f benchmarks/docker-compose.yml run --rm k6 run "/scripts/${test_script}"

    echo -e "${GREEN}Results saved to benchmarks/results/${NC}"
}

run_local_throughput() {
    ensure_built
    echo -e "${BLUE}Running local throughput test...${NC}"

    # Start mockd
    ./mockd start --port 4280 --admin-port 4290 &
    PID=$!
    sleep 2

    # Setup mock
    curl -s -X POST http://localhost:4290/api/mocks \
        -H "Content-Type: application/json" \
        -d '{"name":"echo","request":{"method":"GET","path":"/echo"},"response":{"status":200,"body":"{}"}}' \
        >/dev/null

    # Check for load testing tool
    if command -v k6 &>/dev/null; then
        MOCKD_URL=http://localhost:4280 k6 run benchmarks/k6/throughput_max.js
    elif command -v wrk &>/dev/null; then
        wrk -t4 -c100 -d30s http://localhost:4280/echo
    elif command -v hey &>/dev/null; then
        hey -n 100000 -c 100 http://localhost:4280/echo
    else
        echo -e "${YELLOW}No load testing tool found. Install k6, wrk, or hey.${NC}"
        echo "  brew install k6"
        echo "  brew install wrk"
        echo "  go install github.com/rakyll/hey@latest"
    fi

    kill $PID 2>/dev/null || true
}

run_local_latency() {
    ensure_built
    echo -e "${BLUE}Running local latency test...${NC}"

    ./mockd start --port 4280 --admin-port 4290 &
    PID=$!
    sleep 2

    curl -s -X POST http://localhost:4290/api/mocks \
        -H "Content-Type: application/json" \
        -d '{"name":"echo","request":{"method":"GET","path":"/echo"},"response":{"status":200,"body":"{}"}}' \
        >/dev/null

    if command -v k6 &>/dev/null; then
        MOCKD_URL=http://localhost:4280 k6 run benchmarks/k6/latency.js
    else
        echo -e "${YELLOW}k6 required for latency test. Install with: brew install k6${NC}"
    fi

    kill $PID 2>/dev/null || true
}

run_local_startup() {
    ensure_built
    echo -e "${BLUE}Running startup timing test...${NC}"
    chmod +x benchmarks/startup_timing.sh
    MOCKD_BIN=./mockd benchmarks/startup_timing.sh
}

run_local_memory() {
    ensure_built
    echo -e "${BLUE}Running memory profiling test...${NC}"
    chmod +x benchmarks/memory_profile.sh
    MOCKD_BIN=./mockd benchmarks/memory_profile.sh
}

echo ""
echo "========================================"
echo "       MOCKD BENCHMARK SUITE"
echo "========================================"
echo "Mode: $MODE"
echo "Test: $TEST"
echo ""

case "$MODE" in
    docker)
        case "$TEST" in
            all)
                run_docker "load_test.js"
                run_docker "throughput_max.js"
                run_docker "latency.js"
                ;;
            throughput) run_docker "throughput_max.js" ;;
            latency) run_docker "latency.js" ;;
            load) run_docker "load_test.js" ;;
            *) usage ;;
        esac
        ;;
    local)
        case "$TEST" in
            all)
                run_local_startup
                run_local_memory
                run_local_throughput
                run_local_latency
                ;;
            throughput) run_local_throughput ;;
            latency) run_local_latency ;;
            startup) run_local_startup ;;
            memory) run_local_memory ;;
            *) usage ;;
        esac
        ;;
    all)
        echo -e "${BLUE}Running local benchmarks first...${NC}"
        $0 local all
        echo ""
        echo -e "${BLUE}Running Docker benchmarks...${NC}"
        $0 docker all
        ;;
    *)
        usage
        ;;
esac

echo ""
echo -e "${GREEN}Benchmarks complete!${NC}"
echo ""
echo "Summary files:"
ls -la benchmarks/results/*.json 2>/dev/null || ls -la *.json 2>/dev/null || echo "  (check benchmark output above)"
