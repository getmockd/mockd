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
  "latency": {"min": "100ms", "max": "500ms"},
  "errorRate": 0.1,
  "errorCode": 503
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

## Notes

- Chaos applies to **all protocols** that run over HTTP (HTTP mocks, GraphQL, SOAP, SSE). gRPC and MQTT have their own transports and are not affected by HTTP chaos.
- Latency is added **on top of** any `delayMs` configured on individual mocks.
- When both latency and error rate are enabled, the error check happens first — if a request is selected for an error, it returns immediately with the error code (no latency added).
- Chaos settings are runtime-only — they reset when mockd restarts. They are not persisted in config files.
