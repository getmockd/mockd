# Protocol Benchmark & Smoke Test Suite

## Overview

Each protocol test serves dual purpose:
1. **Integration smoke test** - Verifies protocol features work correctly
2. **Performance benchmark** - Measures throughput/latency for claims

## Protocol Test Matrix

### HTTP
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Basic GET/POST | ✓ Response matches | req/s |
| Path parameters | ✓ Extraction works | req/s with params |
| Query parameters | ✓ Matching works | req/s |
| Header matching | ✓ Headers extracted | req/s |
| Body matching | ✓ JSON/regex works | req/s |
| Latency simulation | ✓ Delay applied | Verify delay accuracy |
| Status codes | ✓ All codes work | - |
| Templating | ✓ Variables resolve | req/s with templates |

### WebSocket
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Connect/disconnect | ✓ Handshake works | connections/s |
| Echo messages | ✓ Response matches | msg/s |
| Broadcast | ✓ All clients receive | msg/s fan-out |
| Binary messages | ✓ Data preserved | msg/s binary |
| Reconnection | ✓ State preserved | reconnect time |

### gRPC
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Unary calls | ✓ Response matches | calls/s |
| Server streaming | ✓ All messages received | msg/s |
| Client streaming | ✓ Aggregation works | msg/s |
| Bidirectional | ✓ Echo works | msg/s round-trip |
| Reflection | ✓ Service discovery | - |
| Metadata | ✓ Headers passed | - |

### GraphQL
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Queries | ✓ Fields resolved | queries/s |
| Mutations | ✓ State changes | mutations/s |
| Subscriptions | ✓ Events pushed | events/s |
| Variables | ✓ Substitution works | - |
| Fragments | ✓ Expansion works | - |
| Errors | ✓ Error format correct | - |

### MQTT
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Connect | ✓ Broker accepts | connections/s |
| Publish QoS 0 | ✓ Fire and forget | msg/s |
| Publish QoS 1 | ✓ At least once | msg/s |
| Publish QoS 2 | ✓ Exactly once | msg/s |
| Subscribe | ✓ Messages received | - |
| Wildcards | ✓ Topic matching | - |
| Retained | ✓ Message persisted | - |
| Last Will | ✓ Disconnect triggers | - |

### SSE
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Event stream | ✓ Events received | events/s |
| Event types | ✓ Type filtering | - |
| Reconnection | ✓ Last-Event-ID | reconnect time |
| Keep-alive | ✓ Comments sent | - |

### SOAP
| Feature | Smoke Test | Performance Metric |
|---------|------------|-------------------|
| Envelope parsing | ✓ Body extracted | req/s |
| WSDL generation | ✓ Valid WSDL | - |
| Fault responses | ✓ Fault format | - |
| Headers | ✓ SOAP headers work | - |

## Output Format

```json
{
  "timestamp": "2024-01-09T12:00:00Z",
  "protocols": {
    "http": {
      "smoke_tests": { "passed": 8, "failed": 0 },
      "performance": {
        "throughput_req_s": 70000,
        "latency_p50_ms": 1.2,
        "latency_p99_ms": 8.5
      }
    },
    "websocket": {
      "smoke_tests": { "passed": 5, "failed": 0 },
      "performance": {
        "throughput_msg_s": 50000,
        "connection_time_ms": 2.1
      }
    }
    // ... other protocols
  },
  "claims": {
    "http": "50K+ req/s, <2ms p50 latency",
    "websocket": "40K+ msg/s",
    "grpc": "30K+ calls/s",
    "graphql": "20K+ queries/s",
    "mqtt": "100K+ msg/s QoS 0"
  }
}
```

## Implementation Plan

1. Create test fixtures for each protocol
2. Implement smoke test assertions
3. Add performance measurement
4. Generate claims from results
5. Run in CI with thresholds
