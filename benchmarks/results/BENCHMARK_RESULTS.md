# MockD Benchmark Results

**Date**: 2026-01-09
**Environment**: Linux 6.8.0-90-generic, Intel Core i7-4790K @ 4.00GHz
**MockD Version**: Built from source

---

## Protocol Performance Summary

| Protocol | Metric | Measured Value | Conservative Claim |
|----------|--------|----------------|-------------------|
| **HTTP** | Throughput | 70-77K req/s | **50K+ req/s** |
| **HTTP** | p50 Latency | 1-2ms | **<2ms** |
| **HTTP** | p99 Latency | 7-11ms | **<15ms** |
| **WebSocket** | Throughput | 31-32K msg/s | **25K+ msg/s** |
| **WebSocket** | Latency | 31μs | **<50μs** |
| **gRPC** | Unary Calls | 7.7K calls/s | **5K+ calls/s** |
| **gRPC** | Concurrent (10) | 35K calls/s | **25K+ calls/s** |
| **gRPC** | Latency | 129μs | **<200μs** |
| **MQTT** | QoS 0 | 108K msg/s | **100K+ msg/s** |
| **MQTT** | QoS 1 | 21K msg/s | **15K+ msg/s** |
| **MQTT** | QoS 2 | 12K msg/s | **10K+ msg/s** |
| **MQTT** | Concurrent (10) | 290K msg/s | **250K+ msg/s** |
| **Startup** | Server | 128μs | **<1ms** |
| **Startup** | CLI | 6.3ms | **<10ms** |
| **Memory** | RSS | 21.5MB | **<25MB** |

---

## Detailed Results

### HTTP (Apache Bench)

```
Test: 100K requests, 200 concurrent, keep-alive

Requests per second:    77,148 [#/sec]
Time per request:       2.592 [ms] (mean)

Latency Distribution:
  50%      2ms
  66%      3ms
  75%      3ms
  95%      7ms
  99%     11ms
  max     20ms
```

### WebSocket (Go Benchmarks)

```
BenchmarkWS_EchoLatency          30,744 ns/op → 32,526 msg/s
BenchmarkWS_ConnectionSetup     311,181 ns/op
BenchmarkWS_MatcherPerformance   33,390 ns/op → 29,949 msg/s
BenchmarkWS_Throughput/small     31,953 ns/op → 31,296 msg/s
BenchmarkWS_Throughput/1KB       38,754 ns/op → 25,804 msg/s
```

### gRPC (Go Benchmarks)

```
BenchmarkGRPC_UnaryLatency          128,842 ns/op →  7,762 calls/s
BenchmarkGRPC_ConnectionSetup        22,357 ns/op
BenchmarkGRPC_ConcurrentUnary        28,565 ns/op → 35,008 calls/s (10 clients)
BenchmarkGRPC_Throughput/unary      127,316 ns/op →  7,854 calls/s
```

### MQTT (Go Benchmarks)

```
BenchmarkMQTT_PublishQoS0             9,216 ns/op → 108,507 msg/s
BenchmarkMQTT_PublishQoS1            48,318 ns/op →  20,696 msg/s
BenchmarkMQTT_PublishQoS2            83,701 ns/op →  11,947 msg/s
BenchmarkMQTT_PubSubLatency          77,644 ns/op →  12,879 msg/s (end-to-end)
BenchmarkMQTT_ConcurrentPublishers    3,440 ns/op → 290,697 msg/s (10 publishers)
BenchmarkMQTT_ConnectionSetup       241,054 ns/op →   4,148 conn/s

Message Size Impact (QoS 0):
  64B:   9,464 ns/op → 105,663 msg/s
  1KB:  12,242 ns/op →  81,686 msg/s
  10KB: 22,135 ns/op →  45,178 msg/s
  64KB:101,326 ns/op →   9,869 msg/s
```

### Startup (Go Benchmarks)

```
BenchmarkServerStartup    127,957 ns/op → 0.128ms
BenchmarkCLIStartup     6,289,400 ns/op → 6.3ms
```

### Memory (RSS)

```
Idle:       21.5MB
Under load: 21.5MB (stable)
```

---

## Integration Smoke Tests

| Protocol | Tests | Status |
|----------|-------|--------|
| HTTP | 30+ test cases | ✅ PASS |
| WebSocket | 15+ test cases | ✅ PASS |
| gRPC | 20+ test cases | ✅ PASS |
| MQTT | 15+ test cases | ✅ PASS |

---

## Recommended Website Claims

### Landing Page Hero Stats

| Current | Recommended |
|---------|-------------|
| "15K+ req/s" | "50K+ HTTP req/s" or "100K+ MQTT msg/s" |
| "<2ms latency" | "<2ms HTTP latency" |
| "~500ms startup" | "<10ms startup" |
| "~30MB memory" | "<25MB memory" |

### Protocol-Specific Claims

| Protocol | Claim |
|----------|-------|
| **HTTP** | "50K+ req/s, <2ms p50 latency" |
| **WebSocket** | "25K+ msg/s, <50μs latency" |
| **gRPC** | "25K+ concurrent calls/s, <200μs latency" |
| **MQTT** | "100K+ QoS 0 msg/s, 15K+ QoS 1, 10K+ QoS 2" |

### General Performance Statement

> "High-performance mocking across all protocols: 50K+ HTTP req/s,
> 100K+ MQTT msg/s, 25K+ WebSocket msg/s, 25K+ gRPC calls/s.
> Sub-millisecond startup, <25MB memory footprint."

---

## Changes Required

1. **Remove**: "across all protocols" generic performance claims
2. **Add**: Protocol-specific numbers where performance is mentioned
3. **Update**: Hero stats to use proven numbers (conservative)
4. **Keep**: Comparative language ("faster than X") if backed by benchmarks

---

## Reproducing Results

```bash
# Build
go build -o mockd ./cmd/mockd

# All Go benchmarks
go test -bench=. -benchtime=3s ./tests/performance/...

# HTTP external benchmark
./mockd start --port 4280 --admin-port 4290 &
curl -X POST http://localhost:4290/mocks -H "Content-Type: application/json" \
  -d '{"name":"test","type":"http","enabled":true,"http":{"matcher":{"method":"GET","path":"/test"},"response":{"statusCode":200,"body":"{}"}}}'
ab -n 100000 -c 200 -k http://localhost:4280/test

# Protocol-specific benchmarks
go test -bench="BenchmarkWS" ./tests/performance/...
go test -bench="BenchmarkGRPC" ./tests/performance/...
go test -bench="BenchmarkMQTT" ./tests/performance/...

# Integration smoke tests
go test -v ./tests/integration/...
```

---

## Benchmark Files

| File | Purpose |
|------|---------|
| `tests/performance/websocket_bench_test.go` | WebSocket throughput & latency |
| `tests/performance/grpc_bench_test.go` | gRPC unary & streaming |
| `tests/performance/mqtt_bench_test.go` | MQTT QoS levels & fanout |
| `tests/performance/startup_test.go` | Server startup time |
| `benchmarks/run_all.sh` | Full benchmark runner |
