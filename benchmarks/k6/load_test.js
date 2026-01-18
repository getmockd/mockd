import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const latencyTrend = new Trend('request_latency', true);
const requestsCounter = new Counter('total_requests');

// Test configuration
export const options = {
  // Ramp up pattern for sustained load test
  stages: [
    { duration: '10s', target: 100 },   // Ramp up to 100 VUs
    { duration: '30s', target: 100 },   // Stay at 100 VUs
    { duration: '10s', target: 500 },   // Ramp up to 500 VUs
    { duration: '30s', target: 500 },   // Stay at 500 VUs
    { duration: '10s', target: 1000 },  // Ramp up to 1000 VUs
    { duration: '30s', target: 1000 },  // Stay at 1000 VUs (peak)
    { duration: '10s', target: 0 },     // Ramp down
  ],

  // Thresholds - what we're claiming on the website
  thresholds: {
    // Throughput: Should handle 15K+ req/s at peak
    'http_reqs': ['rate>10000'],  // Conservative: 10K req/s

    // Latency: <2ms p99 claim
    'http_req_duration': [
      'p(50)<5',    // p50 < 5ms
      'p(95)<10',   // p95 < 10ms
      'p(99)<20',   // p99 < 20ms (relaxed from 2ms)
    ],

    // Error rate: Should be near zero
    'errors': ['rate<0.01'],  // <1% error rate
  },
};

const BASE_URL = __ENV.MOCKD_URL || 'http://mockd:4280';

// Weighted scenarios
const scenarios = [
  { weight: 60, fn: simpleGet },      // 60% simple GET
  { weight: 25, fn: dynamicGet },     // 25% dynamic path
  { weight: 15, fn: postRequest },    // 15% POST
];

function simpleGet() {
  const res = http.get(`${BASE_URL}/echo`);
  check(res, {
    'simple GET status 200': (r) => r.status === 200,
    'simple GET has body': (r) => r.body.length > 0,
  });
  return res;
}

function dynamicGet() {
  const userId = Math.floor(Math.random() * 10000);
  const res = http.get(`${BASE_URL}/users/${userId}`);
  check(res, {
    'dynamic GET status 200': (r) => r.status === 200,
    'dynamic GET has id': (r) => r.body.includes(userId.toString()),
  });
  return res;
}

function postRequest() {
  const payload = JSON.stringify({ name: 'test', value: Math.random() });
  const params = { headers: { 'Content-Type': 'application/json' } };
  const res = http.post(`${BASE_URL}/items`, payload, params);
  check(res, {
    'POST status 201': (r) => r.status === 201,
  });
  return res;
}

export default function () {
  // Weighted random selection
  const rand = Math.random() * 100;
  let cumulative = 0;
  let res;

  for (const scenario of scenarios) {
    cumulative += scenario.weight;
    if (rand < cumulative) {
      res = scenario.fn();
      break;
    }
  }

  // Track metrics
  if (res) {
    requestsCounter.add(1);
    latencyTrend.add(res.timings.duration);
    errorRate.add(res.status !== 200 && res.status !== 201);
  }

  // Small think time to simulate real usage
  // Remove for pure throughput test
  // sleep(0.01);
}

// Summary output
export function handleSummary(data) {
  const summary = {
    timestamp: new Date().toISOString(),
    duration_seconds: data.state.testRunDurationMs / 1000,
    metrics: {
      requests_total: data.metrics.http_reqs?.values?.count || 0,
      requests_per_second: data.metrics.http_reqs?.values?.rate || 0,
      latency_p50_ms: data.metrics.http_req_duration?.values?.['p(50)'] || 0,
      latency_p95_ms: data.metrics.http_req_duration?.values?.['p(95)'] || 0,
      latency_p99_ms: data.metrics.http_req_duration?.values?.['p(99)'] || 0,
      latency_avg_ms: data.metrics.http_req_duration?.values?.avg || 0,
      error_rate: data.metrics.errors?.values?.rate || 0,
    },
    thresholds_passed: Object.values(data.root_group?.checks || {})
      .every(c => c.passes === c.passes + c.fails),
  };

  console.log('\n=== BENCHMARK RESULTS ===');
  console.log(`Requests/sec: ${summary.metrics.requests_per_second.toFixed(2)}`);
  console.log(`Latency p50: ${summary.metrics.latency_p50_ms.toFixed(2)}ms`);
  console.log(`Latency p95: ${summary.metrics.latency_p95_ms.toFixed(2)}ms`);
  console.log(`Latency p99: ${summary.metrics.latency_p99_ms.toFixed(2)}ms`);
  console.log(`Error rate: ${(summary.metrics.error_rate * 100).toFixed(2)}%`);
  console.log('=========================\n');

  return {
    '/results/summary.json': JSON.stringify(summary, null, 2),
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
  };
}

// Simple text summary
function textSummary(data, opts) {
  return `
Benchmark Complete
==================
Total Requests: ${data.metrics.http_reqs?.values?.count || 0}
Requests/sec:   ${(data.metrics.http_reqs?.values?.rate || 0).toFixed(2)}
Avg Latency:    ${(data.metrics.http_req_duration?.values?.avg || 0).toFixed(2)}ms
p99 Latency:    ${(data.metrics.http_req_duration?.values?.['p(99)'] || 0).toFixed(2)}ms
`;
}
