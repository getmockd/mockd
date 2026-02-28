---
title: Chaos Engineering
description: Inject latency, errors, and failures to test your application's resilience
---

Chaos engineering lets you simulate real-world failure conditions — slow APIs, intermittent errors, service outages — so you can verify your application handles them gracefully.

## Quick Start

```bash
# Start mockd with a mock endpoint
mockd serve &
mockd http add --path /api/users --body '[{"id":1,"name":"Alice"}]'

# Enable chaos: 200ms latency + 10% error rate
mockd chaos enable --latency 200ms --error-rate 0.1 --error-code 503

# Test it — some requests will be slow, some will fail
curl http://localhost:4280/api/users
curl http://localhost:4280/api/users
curl http://localhost:4280/api/users

# Check current chaos settings
mockd chaos status

# Disable when done
mockd chaos disable
```

## CLI Commands

### Enable Chaos

```bash
mockd chaos enable [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--latency` | string | — | Random latency range (e.g., `200ms`, `100ms-500ms`) |
| `--error-rate` | float | 0 | Fraction of requests that return errors (0.0–1.0) |
| `--error-code` | int | 500 | HTTP status code for error responses |
| `--path` | string | — | Regex pattern to scope chaos to specific paths |
| `--probability` | float | 1.0 | Probability of applying chaos at all (0.0–1.0) |

### Check Status

```bash
mockd chaos status
```

Returns the current chaos configuration (latency, error rate, affected paths).

### Disable Chaos

```bash
mockd chaos disable
```

Immediately removes all chaos injection. Requests return to normal behavior.

## Chaos Profiles

Instead of manually configuring latency and error rates, use one of 10 built-in chaos profiles that simulate common failure scenarios:

```bash
# Apply a profile at startup
mockd serve --chaos-profile flaky

# Or apply at runtime
mockd chaos apply flaky

# List available profiles
mockd chaos profiles

# Disable when done
mockd chaos disable
```

### Available Profiles

| Profile | Description | Latency | Error Rate | Bandwidth |
|---------|-------------|---------|------------|-----------|
| `slow-api` | Slow upstream API | 500ms-2s | — | — |
| `degraded` | Partially degraded service | 200ms-800ms | 5% (503) | — |
| `flaky` | Unreliable with random errors | 0-100ms | 20% (500/502/503) | — |
| `offline` | Service completely down | — | 100% (503) | — |
| `timeout` | Connection timeout simulation | 30s fixed | — | — |
| `rate-limited` | Rate-limited API | 50ms-200ms | 30% (429) | — |
| `mobile-3g` | Mobile 3G network conditions | 300ms-800ms | 2% (503) | 50 KB/s |
| `satellite` | Satellite internet simulation | 600ms-2s | 5% (503) | 20 KB/s |
| `dns-flaky` | Intermittent DNS resolution failures | — | 10% (503) | — |
| `overloaded` | Overloaded server under heavy load | 1s-5s | 15% (500/502/503/504) | 100 KB/s |

### Admin API for Profiles

```bash
# List all profiles
curl http://localhost:4290/chaos/profiles

# Get a specific profile
curl http://localhost:4290/chaos/profiles/flaky

# Apply a profile
curl -X POST http://localhost:4290/chaos/profiles/flaky/apply
```

## Examples

### Fixed Latency

Add a flat 200ms delay to every response:

```bash
mockd chaos enable --latency 200ms
```

### Random Latency Range

Responses take between 100ms and 500ms (uniformly random):

```bash
mockd chaos enable --latency 100ms-500ms
```

### Error Injection

10% of requests return HTTP 503:

```bash
mockd chaos enable --error-rate 0.1 --error-code 503
```

### Combined Latency + Errors

Simulate a degraded upstream service — slow responses with occasional failures:

```bash
mockd chaos enable --latency 200ms-800ms --error-rate 0.05 --error-code 502
```

### Path-Scoped Chaos

Only affect specific endpoints:

```bash
# Chaos only on /api/payments/* routes
mockd chaos enable --latency 500ms --error-rate 0.2 --error-code 500 --path "/api/payments/.*"
```

Other endpoints continue responding normally.

### Partial Application

Apply chaos to only 50% of matching requests:

```bash
mockd chaos enable --latency 1s --probability 0.5
```

## Admin API

You can also manage chaos via the Admin API (port 4290):

### Get Current Settings

```bash
curl http://localhost:4290/chaos
```

### Enable Chaos

```bash
curl -X PUT http://localhost:4290/chaos -H 'Content-Type: application/json' -d '{
  "enabled": true,
  "latency": {"min": "100ms", "max": "500ms", "probability": 1.0},
  "errorRate": {"probability": 0.1, "defaultCode": 503}
}'
```

### Disable Chaos

```bash
curl -X PUT http://localhost:4290/chaos -H 'Content-Type: application/json' -d '{
  "enabled": false
}'
```

## Use Cases

### Timeout Testing

Verify your HTTP client's timeout handling:

```bash
# Set latency higher than your client's timeout
mockd chaos enable --latency 10s

# Your app should timeout and handle it gracefully
curl --max-time 3 http://localhost:4280/api/users
# curl: (28) Operation timed out after 3000 milliseconds
```

### Circuit Breaker Testing

Verify your circuit breaker trips after enough failures:

```bash
# High error rate to trigger circuit breaker
mockd chaos enable --error-rate 0.8 --error-code 503

# Run your app and verify the circuit opens
# Then disable chaos and verify it closes
mockd chaos disable
```

### Retry Logic Testing

Verify your retry logic with intermittent failures:

```bash
# Low error rate — retries should succeed
mockd chaos enable --error-rate 0.3 --error-code 500
```

### CI/CD Resilience Tests

Run chaos in your test pipeline to catch resilience regressions:

```bash
#!/bin/bash
# Start mockd with your API mocks
mockd serve --config mocks.yaml &
sleep 2

# Run happy-path tests first
pytest tests/integration/ || exit 1

# Enable chaos and run resilience tests
mockd chaos enable --latency 500ms --error-rate 0.1 --error-code 503
pytest tests/resilience/ || exit 1

# Clean up
mockd chaos disable
mockd stop
```

### Gradual Degradation

Simulate a service getting progressively worse:

```bash
# Start mild
mockd chaos enable --latency 50ms --error-rate 0.01

# Get worse
mockd chaos enable --latency 200ms --error-rate 0.05

# Service is struggling
mockd chaos enable --latency 1s --error-rate 0.2 --error-code 503

# Full outage
mockd chaos enable --error-rate 1.0 --error-code 503

# Recovery
mockd chaos disable
```

## Using --json

All chaos commands support `--json` for scripting:

```bash
mockd chaos status --json
```

```json
{
  "enabled": true,
  "latency": "200ms",
  "errorRate": 0.1,
  "errorCode": 503
}
```

## Stateful Fault Types

In addition to the 8 basic fault types (latency, error, timeout, corrupt body, empty response, slow body, connection reset, partial response), mockd supports 4 **stateful** fault types that maintain state across requests — simulating real-world failure patterns that evolve over time.

### Circuit Breaker

Simulates a circuit breaker pattern with three states: **closed** (normal), **open** (failing), and **half-open** (testing recovery).

```bash
# Configure via Admin API with rules
curl -X PUT http://localhost:4290/chaos -H 'Content-Type: application/json' -d '{
  "enabled": true,
  "rules": [{
    "pathPattern": "/api/payments/.*",
    "faults": [{
      "type": "circuit_breaker",
      "probability": 1.0,
      "circuitBreaker": {
        "failureThreshold": 5,
        "recoveryTimeout": "30s",
        "halfOpenRequests": 2,
        "tripStatusCode": 503
      }
    }]
  }]
}'
```

After `failureThreshold` failures, the circuit opens and all requests get `503`. After `recoveryTimeout`, it enters half-open state and allows `halfOpenRequests` test requests through. If those succeed, it closes; if they fail, it re-opens.

```bash
# Monitor circuit breaker state
mockd chaos faults

# Manually trip or reset
mockd chaos circuit-breaker trip 0:0
mockd chaos circuit-breaker reset 0:0
```

### Retry-After

Returns `429 Too Many Requests` or `503 Service Unavailable` with a `Retry-After` header. After the specified duration, requests pass through normally.

```bash
curl -X PUT http://localhost:4290/chaos -H 'Content-Type: application/json' -d '{
  "enabled": true,
  "rules": [{
    "pathPattern": "/api/.*",
    "faults": [{
      "type": "retry_after",
      "probability": 1.0,
      "retryAfter": {
        "statusCode": 429,
        "retryAfterSeconds": 30
      }
    }]
  }]
}'
```

### Progressive Degradation

Latency increases with each request, simulating a service that gets slower under load. Optionally starts returning errors after enough requests.

```bash
curl -X PUT http://localhost:4290/chaos -H 'Content-Type: application/json' -d '{
  "enabled": true,
  "rules": [{
    "pathPattern": "/api/.*",
    "faults": [{
      "type": "progressive_degradation",
      "probability": 1.0,
      "progressiveDegradation": {
        "initialDelay": "10ms",
        "delayIncrement": "50ms",
        "maxDelay": "5s",
        "errorAfterRequests": 100,
        "errorStatusCode": 503
      }
    }]
  }]
}'
```

### Chunked Dribble

Delivers the response body in timed chunks instead of all at once, simulating slow or unstable network transfers.

```bash
curl -X PUT http://localhost:4290/chaos -H 'Content-Type: application/json' -d '{
  "enabled": true,
  "rules": [{
    "pathPattern": "/api/.*",
    "faults": [{
      "type": "chunked_dribble",
      "probability": 1.0,
      "chunkedDribble": {
        "chunkCount": 5,
        "totalDuration": "2s"
      }
    }]
  }]
}'
```

### Monitoring Stateful Faults

Use the CLI or MCP tools to inspect stateful fault state:

```bash
# View all stateful fault instances
mockd chaos faults

# Via MCP tool
# get_stateful_faults — returns circuit breaker states, retry-after counters, degradation progress
# manage_circuit_breaker — trip or reset circuit breakers by key
```

### Fault Type Reference

| Fault Type | Category | Description |
|-----------|----------|-------------|
| `latency` | Basic | Adds random latency to responses |
| `error` | Basic | Returns error status codes |
| `timeout` | Basic | Simulates connection timeout |
| `corrupt_body` | Basic | Corrupts response body data |
| `empty_response` | Basic | Returns empty body |
| `slow_body` | Basic | Drip-feeds response data slowly |
| `connection_reset` | Basic | Simulates TCP connection reset |
| `partial_response` | Basic | Truncates response at random point |
| `circuit_breaker` | Stateful | Closed → open → half-open state machine |
| `retry_after` | Stateful | 429/503 with Retry-After header, auto-recovers |
| `progressive_degradation` | Stateful | Latency increases over time, optional errors |
| `chunked_dribble` | Stateful | Delivers body in timed chunks |

## Notes

- Chaos applies to **all protocols** that run over HTTP (HTTP mocks, GraphQL, SOAP, SSE). gRPC and MQTT have their own transports and are not affected by HTTP chaos.
- Latency is added **on top of** any `delayMs` configured on individual mocks.
- When both latency and error rate are enabled, the error check happens first — if a request is selected for an error, it returns immediately with the error code (no latency added).
- Chaos settings are runtime-only — they reset when mockd restarts. They are not persisted in config files.
- Stateful faults use a **rules-based** configuration with `pathPattern` matching, allowing different fault types on different routes.
- Use `get_stateful_faults` (MCP) or `mockd chaos faults` (CLI) to monitor stateful fault state machines.
