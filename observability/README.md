# mockd Observability Stack

This directory contains configuration for running mockd with Prometheus, Jaeger, Loki, and Grafana for full observability (metrics, tracing, and logging).

## Quick Start

```bash
# Start the full stack
docker-compose -f docker-compose.observability-local.yml up -d

# Start mockd with tracing and log aggregation enabled
./mockd serve --no-auth \
  --otlp-endpoint http://localhost:4318/v1/traces \
  --loki-endpoint http://localhost:3100/loki/api/v1/push

# View logs
docker-compose -f docker-compose.observability-local.yml logs -f

# Stop everything
docker-compose -f docker-compose.observability-local.yml down
```

## Services

| Service | URL | Description |
|---------|-----|-------------|
| mockd Mock Server | http://localhost:4280 | Mock endpoint server |
| mockd Admin API | http://localhost:4290 | Admin API & metrics |
| Prometheus | http://localhost:9090 | Metrics storage & queries |
| Jaeger UI | http://localhost:16686 | Distributed tracing |
| Loki | http://localhost:3100 | Log aggregation |
| Grafana | http://localhost:3000 | Dashboards (admin/admin) |

## Metrics

mockd exposes Prometheus metrics at `/metrics` on the Admin API port.

### Available Metrics

#### Request Metrics
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mockd_requests_total` | Counter | method, path, status | Total mock requests |
| `mockd_request_duration_seconds` | Histogram | method, path | Request latency distribution |
| `mockd_match_hits_total` | Counter | mock_id | Mock match hits |
| `mockd_match_misses_total` | Counter | - | Requests that didn't match any mock |

#### Mock Configuration Metrics
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mockd_mocks_total` | Gauge | type | Total configured mocks |
| `mockd_mocks_enabled` | Gauge | type | Enabled mocks |
| `mockd_active_connections` | Gauge | protocol | Active WebSocket/SSE connections |

#### Admin API Metrics
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mockd_admin_requests_total` | Counter | method, path, status | Admin API requests |
| `mockd_admin_request_duration_seconds` | Histogram | method, path | Admin API latency |

#### Go Runtime Metrics
| Metric | Type | Description |
|--------|------|-------------|
| `go_goroutines` | Gauge | Number of goroutines |
| `go_memstats_heap_alloc_bytes` | Gauge | Heap bytes allocated and in use |
| `go_memstats_heap_sys_bytes` | Gauge | Heap bytes obtained from system |
| `go_gc_duration_seconds` | Gauge | Total GC pause duration |
| `go_gc_cycles_total` | Gauge | Total number of completed GC cycles |
| `go_info` | Gauge | Go version information |

### Example Queries

```promql
# Request rate over 5 minutes
sum(rate(mockd_requests_total[5m]))

# p95 latency
histogram_quantile(0.95, sum(rate(mockd_request_duration_seconds_bucket[5m])) by (le))

# Error rate (4xx and 5xx responses)
sum(rate(mockd_requests_total{status=~"[45].."}[5m])) / sum(rate(mockd_requests_total[5m])) * 100

# Top 10 most-hit mocks
topk(10, sum(mockd_match_hits_total) by (mock_id))

# Mock miss ratio
sum(rate(mockd_match_misses_total[5m])) / (sum(rate(mockd_match_hits_total[5m])) + sum(rate(mockd_match_misses_total[5m])))

# Memory usage
go_memstats_heap_alloc_bytes / 1024 / 1024  # MB
```

## Alerting Rules

Pre-configured alerting rules are included in `prometheus/rules/mockd-alerts.yml`:

| Alert | Severity | Description |
|-------|----------|-------------|
| `MockdDown` | critical | mockd instance is down |
| `MockdNoTraffic` | warning | No requests received recently |
| `MockdHighServerErrorRate` | warning | >5% 5xx error rate |
| `MockdCriticalErrorRate` | critical | >20% 5xx error rate |
| `MockdHighLatencyP95` | warning | p95 latency >500ms |
| `MockdHighLatencyP99` | critical | p99 latency >2s |
| `MockdHighMissRate` | info | Many requests hitting unconfigured endpoints |
| `MockdHighMissRatio` | warning | >20% of requests not matching mocks |

To view active alerts:
1. Open Prometheus at http://localhost:9090/alerts
2. Or check in Grafana under Alerting

## Tracing

mockd supports OpenTelemetry tracing via OTLP HTTP. Configure with CLI flags:

```bash
# Send traces to Jaeger via OTLP
mockd serve --otlp-endpoint http://localhost:4318/v1/traces

# With sampling (e.g., sample 10% of traces)
mockd serve --otlp-endpoint http://localhost:4318/v1/traces --trace-sampler 0.1

# Combined with structured logging
mockd serve --otlp-endpoint http://localhost:4318/v1/traces --log-level debug --log-format json
```

### Trace Attributes

Each request span includes:
- `http.method` - HTTP method (GET, POST, etc.)
- `http.url` - Request URL
- `http.target` - Request path
- `http.host` - Host header
- `http.scheme` - http or https
- `http.status_code` - Response status code
- `http.user_agent` - Client user agent
- `otel.status_code` - OK or ERROR based on status code

### Excluded Paths

The following paths are excluded from tracing to reduce noise:
- `/metrics` - Prometheus scrape endpoint
- `/health`, `/healthz`, `/ready`, `/readyz`, `/livez` - Health checks
- `/_/health`, `/__health` - Alternative health checks

### Viewing Traces

1. Open Jaeger UI at http://localhost:16686
2. Select "mockd" from the Service dropdown
3. Click "Find Traces" to see recent requests
4. Click on a trace to see the span details and timing

## Logging

mockd supports structured logging with trace ID injection for log-to-trace correlation.

```bash
# JSON structured logging
mockd serve --log-format json --log-level debug

# With tracing enabled (trace_id will be injected into logs)
mockd serve --log-format json --otlp-endpoint http://localhost:4318/v1/traces
```

### Log Levels

| Level | Description |
|-------|-------------|
| `error` | Errors only |
| `warn` | Warnings and errors |
| `info` | Normal operational messages (default) |
| `debug` | Detailed debugging information |

### Loki Integration

mockd can send logs directly to Loki for centralized log aggregation:

```bash
# Send logs to Loki
./mockd serve --loki-endpoint http://localhost:3100/loki/api/v1/push

# Combined with tracing and JSON format
./mockd serve \
  --loki-endpoint http://localhost:3100/loki/api/v1/push \
  --otlp-endpoint http://localhost:4318/v1/traces \
  --log-format json \
  --log-level debug
```

Loki is configured as a datasource in Grafana with trace-to-log correlation. When viewing logs in Grafana, you can click on trace IDs to jump directly to the trace in Jaeger.

#### Log Labels

When using `--loki-endpoint`, logs are sent with these labels:
- `job=mockd` - Fixed job name
- `service=mockd` - Service identifier
- `port=<port>` - The mock server port

#### Querying Logs in Grafana

1. Open Grafana at http://localhost:3000
2. Go to Explore
3. Select "Loki" datasource
4. Use LogQL queries:
   ```logql
   {job="mockd"}                      # All mockd logs
   {job="mockd"} |= "error"           # Logs containing "error"
   {job="mockd"} | json | level="ERROR" # JSON logs with ERROR level
   ```

## Grafana Dashboard

The pre-configured **mockd Overview** dashboard includes:

### Overview Row
- **Requests (5m)** - Request count in the last 5 minutes
- **Mock Hits** - Total mock match hits
- **Mock Misses** - Requests that didn't match any mock
- **Error Rate** - Percentage of 4xx/5xx responses
- **p95 Latency** - 95th percentile latency
- **Uptime** - Server uptime

### Traffic Row
- **Request Rate by Method** - Requests/second by HTTP method
- **Request Rate by Status** - Requests/second by status code

### Latency Row
- **Request Latency Percentiles** - p50, p90, p95, p99 over time
- **Latency Distribution** - Histogram of request durations

### Mocks & Connections Row
- **Requests by Status Code** - Pie chart of response codes
- **Active Connections** - WebSocket/SSE connections over time
- **Top 10 Mocks by Hits** - Most frequently matched mocks

### Admin API Row (collapsed)
- **Admin API Request Rate** - Admin endpoint usage
- **Admin API Latency** - Admin endpoint performance

### Template Variables

The dashboard supports filtering by:
- **Method** - Filter by HTTP method (GET, POST, etc.)
- **Status** - Filter by status code (200, 404, 500, etc.)

## Path Normalization

To prevent metric cardinality explosion, dynamic path segments are automatically normalized:

| Pattern | Replacement | Example |
|---------|-------------|---------|
| UUID | `{uuid}` | `/items/a1b2c3d4-...` → `/items/{uuid}` |
| MongoDB ObjectID (24 hex) | `{id}` | `/docs/507f1f77bcf86cd799439011` → `/docs/{id}` |
| Numeric ID | `{id}` | `/users/123` → `/users/{id}` |

## Production Considerations

### Security
1. **API Key Authentication**: Configure Prometheus with the mockd API key:
   ```yaml
   scrape_configs:
     - job_name: 'mockd'
       bearer_token_file: /path/to/api-key
       static_configs:
         - targets: ['mockd:4290']
   ```

### Performance
2. **Trace Sampling**: In high-traffic environments, use sampling:
   ```bash
   mockd serve --trace-sampler 0.1  # Sample 10% of traces
   ```

3. **Resource Limits**: Adjust container resource limits based on your traffic

### Retention
4. **Prometheus Retention**: Configure retention for your needs:
   ```yaml
   command:
     - '--storage.tsdb.retention.time=15d'
   ```

5. **Loki Retention**: Configure log retention in `loki-config.yml`:
   ```yaml
   limits_config:
     reject_old_samples_max_age: 168h  # 7 days
   ```

### Alerting
6. **AlertManager**: For production, configure AlertManager for alert routing:
   ```yaml
   # prometheus.yml
   alerting:
     alertmanagers:
       - static_configs:
           - targets: ['alertmanager:9093']
   ```

## Directory Structure

```
observability/
├── README.md                          # This file
├── prometheus-local.yml               # Prometheus config for local dev
├── prometheus/
│   └── rules/
│       └── mockd-alerts.yml           # Alerting rules
├── loki/
│   └── loki-config.yml                # Loki configuration
├── promtail/
│   └── promtail-config.yml            # Promtail config (optional, for file-based log shipping)
└── grafana/
    └── provisioning/
        ├── datasources/
        │   └── datasources.yml        # Prometheus, Jaeger, Loki datasources
        └── dashboards/
            ├── dashboards.yml         # Dashboard provisioning config
            └── json/
                └── mockd-overview.json # Main dashboard
```

## Troubleshooting

### Metrics not appearing
1. Check mockd is running: `curl http://localhost:4290/health`
2. Check metrics endpoint: `curl http://localhost:4290/metrics`
3. Check Prometheus targets: http://localhost:9090/targets

### Traces not appearing
1. Verify OTLP endpoint is correct: `--otlp-endpoint http://localhost:4318/v1/traces`
2. Check Jaeger is running: `docker ps | grep jaeger`
3. Generate some traffic and wait a few seconds (traces are batched)

### Dashboard shows "No data"
1. Ensure Prometheus datasource is configured with UID `prometheus`
2. Check the time range in Grafana (default: Last 15 minutes)
3. Verify mockd has received requests

### High cardinality warnings
If you see cardinality warnings, you may have dynamic paths not being normalized. Check the path normalization in `pkg/engine/metrics_middleware.go` and add patterns as needed.
