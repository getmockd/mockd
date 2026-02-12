// Package chaos provides fault injection capabilities for chaos engineering testing.
//
// Chaos engineering is the discipline of experimenting on a system to build confidence
// in the system's capability to withstand turbulent conditions in production. This
// package enables controlled fault injection to test system resilience.
//
// # Supported Fault Types
//
// The package supports various fault types:
//
//   - Latency: Adds random delay to responses
//   - Error: Returns HTTP error status codes
//   - Timeout: Simulates request timeouts
//   - Corrupt Body: Corrupts response body data
//   - Empty Response: Returns empty response bodies
//   - Slow Body: Delivers response data slowly (bandwidth limiting)
//   - Connection Reset: Simulates connection reset
//   - Partial Response: Truncates responses
//
// # Configuration
//
// Chaos injection is configured via ChaosConfig:
//
//	config := &chaos.ChaosConfig{
//	    Enabled: true,
//	    GlobalRules: &chaos.GlobalChaosRules{
//	        Latency: &chaos.LatencyFault{
//	            Min:         "10ms",
//	            Max:         "100ms",
//	            Probability: 0.1, // 10% of requests
//	        },
//	        ErrorRate: &chaos.ErrorRateFault{
//	            Probability: 0.05, // 5% of requests
//	            StatusCodes: []int{500, 502, 503},
//	        },
//	    },
//	    Rules: []chaos.ChaosRule{
//	        {
//	            PathPattern: "/api/payments.*",
//	            Faults: []chaos.FaultConfig{
//	                {
//	                    Type:        chaos.FaultTimeout,
//	                    Probability: 0.02, // 2% of requests
//	                },
//	            },
//	        },
//	    },
//	}
//
// # Usage with HTTP Handlers
//
// Use the Middleware to wrap existing handlers:
//
//	injector, err := chaos.NewInjector(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	handler := http.HandlerFunc(myHandler)
//	chaosHandler := chaos.NewMiddleware(handler, injector)
//	http.Handle("/", chaosHandler)
//
// # Statistics
//
// Track chaos injection statistics:
//
//	stats := injector.GetStats()
//	fmt.Printf("Total requests: %d\n", stats.TotalRequests)
//	fmt.Printf("Faults injected: %d\n", stats.InjectedFaults)
//
// # YAML Configuration Example
//
// The package supports YAML configuration for easy integration:
//
//	chaos:
//	  enabled: true
//	  global:
//	    latency:
//	      min: 10ms
//	      max: 100ms
//	      probability: 0.1
//	    errorRate:
//	      probability: 0.05
//	      statusCodes: [500, 502, 503]
//	  rules:
//	    - pathPattern: "/api/payments.*"
//	      faults:
//	        - type: timeout
//	          probability: 0.02
//	        - type: error
//	          probability: 0.01
//	          config:
//	            statusCodes: [500, 503]
//
// # Best Practices
//
//   - Start with low probability values (1-5%) and increase gradually
//   - Use path-specific rules for critical endpoints
//   - Monitor system behavior during chaos testing
//   - Always have a way to disable chaos injection quickly
//   - Use conditional chaos in production (header-based activation)
//
// # Safety Considerations
//
// Chaos testing should be performed carefully:
//
//   - Never enable chaos in production without proper safeguards
//   - Use feature flags or header-based activation for production
//   - Monitor error rates and system health during testing
//   - Have rollback procedures ready
package chaos
