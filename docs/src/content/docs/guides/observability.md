---
title: Observability
description: Monitor mockd with Prometheus metrics, Loki log aggregation, and OpenTelemetry distributed tracing.
---

mockd provides comprehensive observability features for monitoring, debugging, and integrating with your existing observability stack.

## Prometheus Metrics

The admin API exposes Prometheus-compatible metrics at `/metrics`.

### Enabling Metrics

Metrics are available by default on the admin port:

```bash
mockd serve --admin-port 4290
# Metrics at: http://localhost:4290/metrics
```

### Available Metrics

```
# Server uptime
mockd_uptime_seconds 3600

# Request counters (all protocols: HTTP, gRPC, WebSocket, MQTT)
mockd_requests_total{method="GET",path="/api/users",status="200"} 42
mockd_requests_total{method="grpc",path="/helloworld.Greeter/SayHello",status="ok"} 10
mockd_requests_total{method="mqtt",path="sensors/temperature",status="ok"} 5

# Request latency histogram
mockd_request_duration_seconds_bucket{le="0.001",method="GET",path="/api/users"} 100
mockd_request_duration_seconds_bucket{le="0.01",method="GET",path="/api/users"} 150
mockd_request_duration_seconds_bucket{le="+Inf",method="GET",path="/api/users"} 155

# Active connections (WebSocket, etc.)
mockd_active_connections{protocol="websocket"} 3

# Mock matching counters
mockd_match_hits_total{mock_id="http_abc123"} 42
mockd_match_misses_total 5

# Go runtime metrics
go_goroutines 12
go_memstats_heap_alloc_bytes 4194304
```

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mockd'
    static_configs:
      - targets: ['localhost:4290']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Grafana Dashboard

Example Grafana queries:

```promql
# Request rate
rate(mockd_requests_total[5m])

# Error rate
sum(rate(mockd_requests_total{status=~"5.."}[5m])) 
  / sum(rate(mockd_requests_total[5m]))

# P95 latency
histogram_quantile(0.95, rate(mockd_request_duration_seconds_bucket[5m]))

# Mock match rate
rate(mockd_match_hits_total[5m])
```

---

## Loki Log Aggregation

Send mockd logs to Grafana Loki for centralized log aggregation.

### Enabling Loki

```bash
mockd serve --loki-endpoint http://localhost:3100/loki/api/v1/push
```

### Log Format

Logs are sent with the following labels:

| Label | Description |
|-------|-------------|
| `job` | Always `mockd` |
| `level` | Log level (debug, info, warn, error) |
| `component` | Component name (server, admin, engine) |

### Loki Configuration

Ensure Loki is running and accessible:

```yaml
# docker-compose.yml
services:
  loki:
    image: grafana/loki:2.9.0
    ports:
      - "3100:3100"
    command: -config.file=/etc/loki/local-config.yaml
```

### Querying Logs in Grafana

```logql
# All mockd logs
{job="mockd"}

# Errors only
{job="mockd", level="error"}

# Request logs
{job="mockd"} |= "request"

# Filter by path
{job="mockd"} | json | path="/api/users"
```

### Log Batching

Logs are batched for efficiency:
- Batch size: 100 entries or 5 seconds (whichever comes first)
- Graceful shutdown flushes pending logs

---

## OpenTelemetry Tracing

Send distributed traces to any OpenTelemetry-compatible backend (Jaeger, Zipkin, Tempo, etc.).

### Enabling Tracing

```bash
mockd serve --otlp-endpoint http://localhost:4318/v1/traces
```

### Trace Sampling

Control the sampling rate (default: 100%):

```bash
# Sample 10% of traces
mockd serve --otlp-endpoint http://localhost:4318/v1/traces --trace-sampler 0.1
```

### Trace Attributes

Each span includes:

| Attribute | Description |
|-----------|-------------|
| `http.method` | HTTP method |
| `http.url` | Request URL |
| `http.status_code` | Response status code |
| `mockd.mock_id` | Matched mock ID |
| `mockd.matched` | Whether a mock matched |

### Jaeger Setup

```yaml
# docker-compose.yml
services:
  jaeger:
    image: jaegertracing/all-in-one:1.50
    ports:
      - "16686:16686"  # UI
      - "4318:4318"    # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

```bash
mockd serve --otlp-endpoint http://localhost:4318/v1/traces
# View traces at: http://localhost:16686
```

### Grafana Tempo Setup

```yaml
# docker-compose.yml
services:
  tempo:
    image: grafana/tempo:latest
    command: ["-config.file=/etc/tempo.yaml"]
    ports:
      - "4318:4318"
```

---

## Combined Setup

Run mockd with full observability:

```bash
mockd serve \
  --log-level debug \
  --log-format json \
  --loki-endpoint http://localhost:3100/loki/api/v1/push \
  --otlp-endpoint http://localhost:4318/v1/traces \
  --trace-sampler 1.0
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  mockd:
    image: ghcr.io/getmockd/mockd:latest
    ports:
      - "4280:4280"
      - "4290:4290"
    command: >
      serve
      --loki-endpoint http://loki:3100/loki/api/v1/push
      --otlp-endpoint http://tempo:4318/v1/traces
    depends_on:
      - loki
      - tempo

  loki:
    image: grafana/loki:2.9.0
    ports:
      - "3100:3100"

  tempo:
    image: grafana/tempo:latest
    ports:
      - "4318:4318"

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=true
      - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
```

---

## Request Streaming

For real-time request monitoring, use the SSE endpoint:

```bash
curl -N http://localhost:4290/requests/stream
```

See [Admin API Reference](/reference/admin-api#get-requestsstream) for details.

---

## See Also

- [Admin API Reference](/reference/admin-api) - Metrics and streaming endpoints
- [CLI Reference](/reference/cli) - Logging and tracing flags
- [Troubleshooting](/guides/troubleshooting) - Debugging issues
