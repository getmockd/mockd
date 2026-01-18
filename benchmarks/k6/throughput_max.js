import http from 'k6/http';
import { check } from 'k6';

// Pure throughput test - maximum requests per second
// No think time, minimal checks, just raw speed

export const options = {
  // Fixed high concurrency for max throughput
  scenarios: {
    max_throughput: {
      executor: 'constant-vus',
      vus: 500,
      duration: '60s',
    },
  },

  // Disable HTTP/2 for maximum simplicity
  insecureSkipTLSVerify: true,
  noConnectionReuse: false,  // Reuse connections for speed

  // We want to see the actual ceiling
  thresholds: {
    'http_reqs': ['rate>5000'],  // Minimum acceptable
  },
};

const BASE_URL = __ENV.MOCKD_URL || 'http://mockd:4280';

export default function () {
  // Simplest possible request - no parsing, no checks
  http.get(`${BASE_URL}/echo`);
}

export function handleSummary(data) {
  const rps = data.metrics.http_reqs?.values?.rate || 0;
  const p99 = data.metrics.http_req_duration?.values?.['p(99)'] || 0;

  console.log('\n========================================');
  console.log('    MAXIMUM THROUGHPUT BENCHMARK');
  console.log('========================================');
  console.log(`  Requests/sec:  ${rps.toFixed(0)}`);
  console.log(`  p99 Latency:   ${p99.toFixed(2)}ms`);
  console.log('========================================\n');

  return {
    '/results/throughput.json': JSON.stringify({
      test: 'max_throughput',
      requests_per_second: rps,
      p99_latency_ms: p99,
      timestamp: new Date().toISOString(),
    }, null, 2),
  };
}
