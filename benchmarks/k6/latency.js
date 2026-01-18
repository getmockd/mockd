import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

// Latency-focused test - lower concurrency, accurate timing
// Tests the <2ms claim

const latencyHistogram = new Trend('latency_histogram', true);

export const options = {
  // Lower concurrency for accurate latency measurement
  scenarios: {
    latency_test: {
      executor: 'constant-arrival-rate',
      rate: 1000,           // 1000 requests per second
      timeUnit: '1s',
      duration: '60s',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },

  thresholds: {
    // The <2ms claim
    'http_req_duration': [
      'p(50)<2',    // p50 < 2ms
      'p(95)<5',    // p95 < 5ms
      'p(99)<10',   // p99 < 10ms
    ],
  },
};

const BASE_URL = __ENV.MOCKD_URL || 'http://mockd:4280';

export default function () {
  const start = Date.now();
  const res = http.get(`${BASE_URL}/echo`);
  const duration = Date.now() - start;

  latencyHistogram.add(res.timings.duration);

  check(res, {
    'status 200': (r) => r.status === 200,
    'latency < 2ms': (r) => r.timings.duration < 2,
    'latency < 5ms': (r) => r.timings.duration < 5,
    'latency < 10ms': (r) => r.timings.duration < 10,
  });
}

export function handleSummary(data) {
  const p50 = data.metrics.http_req_duration?.values?.['p(50)'] || 0;
  const p95 = data.metrics.http_req_duration?.values?.['p(95)'] || 0;
  const p99 = data.metrics.http_req_duration?.values?.['p(99)'] || 0;
  const avg = data.metrics.http_req_duration?.values?.avg || 0;
  const min = data.metrics.http_req_duration?.values?.min || 0;
  const max = data.metrics.http_req_duration?.values?.max || 0;

  // Check how many requests were under 2ms
  const under2ms = data.root_group?.checks?.['latency < 2ms']?.passes || 0;
  const under5ms = data.root_group?.checks?.['latency < 5ms']?.passes || 0;
  const total = (data.root_group?.checks?.['status 200']?.passes || 0) +
                (data.root_group?.checks?.['status 200']?.fails || 0);

  console.log('\n========================================');
  console.log('       LATENCY BENCHMARK RESULTS');
  console.log('========================================');
  console.log(`  Min:     ${min.toFixed(2)}ms`);
  console.log(`  Avg:     ${avg.toFixed(2)}ms`);
  console.log(`  p50:     ${p50.toFixed(2)}ms`);
  console.log(`  p95:     ${p95.toFixed(2)}ms`);
  console.log(`  p99:     ${p99.toFixed(2)}ms`);
  console.log(`  Max:     ${max.toFixed(2)}ms`);
  console.log('----------------------------------------');
  console.log(`  < 2ms:   ${((under2ms/total)*100).toFixed(1)}%`);
  console.log(`  < 5ms:   ${((under5ms/total)*100).toFixed(1)}%`);
  console.log('========================================\n');

  const claim_valid = p99 < 2;
  console.log(claim_valid
    ? '✅ CLAIM VALID: p99 < 2ms'
    : '❌ CLAIM INVALID: p99 >= 2ms - consider adjusting marketing');

  return {
    '/results/latency.json': JSON.stringify({
      test: 'latency',
      latency_ms: { min, avg, p50, p95, p99, max },
      percent_under_2ms: (under2ms/total)*100,
      claim_valid,
      timestamp: new Date().toISOString(),
    }, null, 2),
  };
}
