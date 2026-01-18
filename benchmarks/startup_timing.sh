#!/usr/bin/env bash
# Startup timing benchmark for mockd
# Measures cold start time (the ~500ms claim)

set -euo pipefail

MOCKD_BIN="${MOCKD_BIN:-./mockd}"
ITERATIONS="${ITERATIONS:-10}"
PORT="${PORT:-4280}"
ADMIN_PORT="${ADMIN_PORT:-4290}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "    MOCKD STARTUP TIMING BENCHMARK"
echo "========================================"
echo "Binary: $MOCKD_BIN"
echo "Iterations: $ITERATIONS"
echo ""

# Check binary exists
if [[ ! -x "$MOCKD_BIN" ]]; then
    echo -e "${RED}Error: $MOCKD_BIN not found or not executable${NC}"
    echo "Build with: go build -o mockd ./cmd/mockd"
    exit 1
fi

# Kill any existing mockd
pkill -f "mockd.*start" 2>/dev/null || true
sleep 0.5

declare -a startup_times
declare -a ready_times

for i in $(seq 1 $ITERATIONS); do
    echo -n "Run $i/$ITERATIONS: "

    # Clear any port conflicts
    pkill -f "mockd.*start" 2>/dev/null || true
    sleep 0.2

    # Measure startup time
    start_time=$(date +%s%3N)

    # Start mockd in background
    $MOCKD_BIN start --port $PORT --admin-port $ADMIN_PORT &
    PID=$!

    # Wait for health endpoint
    ready=false
    for attempt in $(seq 1 100); do
        if curl -s -o /dev/null -w "%{http_code}" "http://localhost:$ADMIN_PORT/health" 2>/dev/null | grep -q "200"; then
            ready_time=$(date +%s%3N)
            ready=true
            break
        fi
        sleep 0.01
    done

    if $ready; then
        elapsed=$((ready_time - start_time))
        startup_times+=($elapsed)
        echo -e "${GREEN}${elapsed}ms${NC}"
    else
        echo -e "${RED}FAILED (timeout)${NC}"
    fi

    # Cleanup
    kill $PID 2>/dev/null || true
    wait $PID 2>/dev/null || true
    sleep 0.2
done

# Calculate statistics
total=0
min=${startup_times[0]}
max=${startup_times[0]}

for t in "${startup_times[@]}"; do
    total=$((total + t))
    ((t < min)) && min=$t
    ((t > max)) && max=$t
done

avg=$((total / ${#startup_times[@]}))

# Sort for percentiles
IFS=$'\n' sorted=($(sort -n <<<"${startup_times[*]}")); unset IFS

p50_idx=$(( ${#sorted[@]} * 50 / 100 ))
p95_idx=$(( ${#sorted[@]} * 95 / 100 ))
p99_idx=$(( ${#sorted[@]} * 99 / 100 ))

p50=${sorted[$p50_idx]}
p95=${sorted[$p95_idx]}
p99=${sorted[$p99_idx]:-${sorted[-1]}}

echo ""
echo "========================================"
echo "           RESULTS"
echo "========================================"
echo "  Min:     ${min}ms"
echo "  Avg:     ${avg}ms"
echo "  p50:     ${p50}ms"
echo "  p95:     ${p95}ms"
echo "  Max:     ${max}ms"
echo "========================================"

# Validate claim
if [[ $p95 -lt 500 ]]; then
    echo -e "${GREEN}✅ CLAIM VALID: p95 startup < 500ms${NC}"
else
    echo -e "${RED}❌ CLAIM INVALID: p95 startup >= 500ms${NC}"
    echo "   Consider optimizing or adjusting claim"
fi

# Output JSON
cat > startup_results.json <<EOF
{
  "test": "startup_timing",
  "iterations": $ITERATIONS,
  "startup_ms": {
    "min": $min,
    "avg": $avg,
    "p50": $p50,
    "p95": $p95,
    "max": $max
  },
  "claim_valid": $([ $p95 -lt 500 ] && echo "true" || echo "false"),
  "timestamp": "$(date -Iseconds)"
}
EOF

echo ""
echo "Results saved to startup_results.json"
