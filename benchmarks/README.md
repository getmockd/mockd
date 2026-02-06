# MockD Benchmarks

Performance benchmarks for all MockD protocols. Results are automatically generated and can be compared across systems.

## Quick Start

```bash
# Run all Go benchmarks and generate results
go run benchmarks/run_benchmarks.go

# Results are saved to:
# - benchmarks/results/latest.json  (machine-readable)
# - benchmarks/results/LATEST.md    (human-readable)
```

## Protocol-Specific Targets

Performance targets measured on typical hardware (Intel Core i7, 8GB RAM).
Results may vary based on system configuration.

| Protocol | Metric | Target | Typical | Test |
|----------|--------|--------|---------|------|
| **HTTP** | Throughput | 50K+ req/s | ~70K | `ab` / `wrk` / `k6` |
| **HTTP** | Latency | ~2ms p50 | 2ms | `k6/latency.js` |
| **WebSocket** | Throughput | 20K+ msg/s | ~25K | `BenchmarkWS_EchoLatency` |
| **WebSocket** | Latency | <50μs | ~40μs | `BenchmarkWS_EchoLatency` |
| **gRPC** | Concurrent | 15K+ calls/s | ~15-35K | `BenchmarkGRPC_ConcurrentUnary` |
| **gRPC** | Latency | ~200μs | ~200μs | `BenchmarkGRPC_UnaryLatency` |
| **MQTT QoS 0** | Throughput | 40K+ msg/s | ~47K | `BenchmarkMQTT_PublishQoS0` |
| **MQTT QoS 1** | Throughput | 10K+ msg/s | ~12K | `BenchmarkMQTT_PublishQoS1` |
| **MQTT QoS 2** | Throughput | 8K+ msg/s | ~10K | `BenchmarkMQTT_PublishQoS2` |
| **Startup** | CLI | <20ms | ~15ms | `BenchmarkCLIStartup` |
| **Memory** | RSS | <30MB | ~22MB | `memory_profile.sh` |

## Results

| File | Description |
|------|-------------|
| `results/latest.json` | Latest benchmark data (JSON) |
| `results/LATEST.md` | Latest results (Markdown) |
| `results/BENCHMARK_RESULTS.md` | Reference baseline |

## Running Individual Benchmarks

### Go Benchmarks (Protocol Tests)

```bash
# WebSocket
go test -bench=BenchmarkWS -benchtime=3s -benchmem ./tests/performance/...

# gRPC
go test -bench=BenchmarkGRPC -benchtime=3s -benchmem ./tests/performance/...

# MQTT
go test -bench=BenchmarkMQTT -benchtime=3s -benchmem ./tests/performance/...

# Startup
go test -bench="BenchmarkServerStartup|BenchmarkCLIStartup" -benchtime=3s -benchmem ./tests/performance/...

# All benchmarks
go test -bench=. -benchtime=3s -benchmem ./tests/performance/...
```

### HTTP Load Testing

HTTP requires an external load testing tool:

```bash
# Start mockd
./mockd start --port 4280 --admin-port 4290 &

# Create test mock
curl -X POST http://localhost:4290/mocks -H "Content-Type: application/json" \
  -d '{"name":"bench","type":"http","enabled":true,"http":{"matcher":{"method":"GET","path":"/bench"},"response":{"statusCode":200,"body":"{}"}}}'

# Using Apache Bench
ab -n 100000 -c 200 -k http://localhost:4280/bench

# Using wrk
wrk -t4 -c100 -d30s http://localhost:4280/bench

# Using k6
k6 run benchmarks/k6/load_test.js
```

### Bash Scripts

```bash
# Startup timing (10 iterations)
MOCKD_BIN=./mockd ./benchmarks/startup_timing.sh

# Memory profiling (30 seconds)
MOCKD_BIN=./mockd ./benchmarks/memory_profile.sh
```

## Docker Mode

For isolated, reproducible benchmarks:

```bash
# Run all benchmarks in Docker
./benchmarks/run.sh docker all

# Run specific test
./benchmarks/run.sh docker throughput
./benchmarks/run.sh docker latency
```

## Interpreting Results

### Key Metrics

| Metric | Description |
|--------|-------------|
| `ops/sec` | Operations per second (higher = better) |
| `ns/op` | Nanoseconds per operation (lower = better) |
| `B/op` | Bytes allocated per operation |
| `allocs/op` | Memory allocations per operation |

### Factors Affecting Performance

1. **System load** - Run on quiet system for accurate results
2. **CPU** - Higher clock speed = better throughput
3. **Go version** - Newer versions have optimizations
4. **OS tuning** - TCP settings, file descriptors

### Comparing Your Results

Run multiple times and average:

```bash
for i in 1 2 3; do
  go run benchmarks/run_benchmarks.go
  cp benchmarks/results/latest.json "benchmarks/results/run_$i.json"
done
```

## Benchmark Files

| File | Purpose |
|------|---------|
| `run_benchmarks.go` | Aggregated benchmark runner (generates JSON/MD) |
| `tests/performance/websocket_bench_test.go` | WebSocket benchmarks |
| `tests/performance/grpc_bench_test.go` | gRPC benchmarks |
| `tests/performance/mqtt_bench_test.go` | MQTT benchmarks |
| `tests/performance/startup_test.go` | Startup time |
| `k6/*.js` | HTTP load test scripts |
| `startup_timing.sh` | Startup measurement |
| `memory_profile.sh` | Memory profiling |

## CI Integration

```yaml
benchmark:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.25'
    - run: go run benchmarks/run_benchmarks.go
    - uses: actions/upload-artifact@v4
      with:
        name: benchmark-results
        path: benchmarks/results/
```

## Requirements

### Go Benchmarks
- Go 1.25+

### HTTP Load Testing
- One of: k6, wrk, hey, or ab

### Docker Mode
- Docker & Docker Compose

### Bash Scripts
- bash, curl, bc
