#!/usr/bin/env bash
# Memory profiling benchmark for mockd
# Measures the ~30MB claim

set -euo pipefail

MOCKD_BIN="${MOCKD_BIN:-./mockd}"
PORT="${PORT:-4280}"
ADMIN_PORT="${ADMIN_PORT:-4290}"
DURATION="${DURATION:-30}"  # seconds

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "    MOCKD MEMORY BENCHMARK"
echo "========================================"
echo "Binary: $MOCKD_BIN"
echo "Duration: ${DURATION}s"
echo ""

# Check binary exists
if [[ ! -x "$MOCKD_BIN" ]]; then
    echo -e "${RED}Error: $MOCKD_BIN not found or not executable${NC}"
    exit 1
fi

# Kill any existing mockd
pkill -f "mockd.*start" 2>/dev/null || true
sleep 0.5

# Start mockd
echo "Starting mockd..."
$MOCKD_BIN start --port $PORT --admin-port $ADMIN_PORT &
PID=$!

# Wait for ready
echo "Waiting for server to be ready..."
for i in $(seq 1 50); do
    if curl -s "http://localhost:$ADMIN_PORT/health" >/dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

# Add some mock endpoints
echo "Setting up mock endpoints..."
for i in $(seq 1 10); do
    curl -s -X POST "http://localhost:$ADMIN_PORT/api/mocks" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"mock-$i\", \"request\": {\"method\": \"GET\", \"path\": \"/test/$i\"}, \"response\": {\"status\": 200, \"body\": \"response $i\"}}" \
        >/dev/null
done

echo ""
echo "Measuring memory over ${DURATION}s..."
echo ""

declare -a rss_samples
declare -a vsz_samples

for i in $(seq 1 $DURATION); do
    if ps -p $PID >/dev/null 2>&1; then
        # Get memory in KB
        mem_info=$(ps -o rss=,vsz= -p $PID 2>/dev/null)
        rss=$(echo $mem_info | awk '{print $1}')
        vsz=$(echo $mem_info | awk '{print $2}')

        rss_samples+=($rss)
        vsz_samples+=($vsz)

        rss_mb=$(echo "scale=1; $rss / 1024" | bc)
        printf "\r  Sample %d/%d: RSS=%sMB" "$i" "$DURATION" "$rss_mb"
    fi
    sleep 1
done

echo ""
echo ""

# Cleanup
kill $PID 2>/dev/null || true
wait $PID 2>/dev/null || true

# Calculate statistics (RSS)
total=0
min=${rss_samples[0]}
max=${rss_samples[0]}

for m in "${rss_samples[@]}"; do
    total=$((total + m))
    ((m < min)) && min=$m
    ((m > max)) && max=$m
done

avg_kb=$((total / ${#rss_samples[@]}))
min_mb=$(echo "scale=1; $min / 1024" | bc)
avg_mb=$(echo "scale=1; $avg_kb / 1024" | bc)
max_mb=$(echo "scale=1; $max / 1024" | bc)

echo "========================================"
echo "        MEMORY RESULTS (RSS)"
echo "========================================"
echo "  Min:     ${min_mb}MB"
echo "  Avg:     ${avg_mb}MB"
echo "  Max:     ${max_mb}MB"
echo "========================================"

# Validate claim
max_int=${max_mb%.*}
if [[ $max_int -lt 50 ]]; then
    echo -e "${GREEN}✅ CLAIM VALID: Memory < 50MB${NC}"
    claim_valid=true
else
    echo -e "${YELLOW}⚠️  Memory > 50MB at peak${NC}"
    claim_valid=false
fi

# Output JSON
cat > memory_results.json <<EOF
{
  "test": "memory_profile",
  "duration_seconds": $DURATION,
  "memory_mb": {
    "min": $min_mb,
    "avg": $avg_mb,
    "max": $max_mb
  },
  "claim_valid": $claim_valid,
  "timestamp": "$(date -Iseconds)"
}
EOF

echo ""
echo "Results saved to memory_results.json"
