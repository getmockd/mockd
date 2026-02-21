// Package integration provides integration tests for the mockd server.
package integration

import (
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// Global port counter for all integration tests to avoid port collisions
// when tests run in parallel. Starting at 30000 to avoid common ports
// and give a wide range for all tests.
var globalPortCounter uint32 = 30000

// GetFreePortSafe returns a unique port for testing that won't collide
// with other tests running in parallel. This is safer than the standard
// getFreePort() which can return the same port to concurrent callers.
func GetFreePortSafe() int {
	// Try to find an actually free port in our range
	for attempts := 0; attempts < 100; attempts++ {
		port := int(atomic.AddUint32(&globalPortCounter, 1))
		if isPortFree(port) {
			return port
		}
	}
	// Fallback to the atomic counter value even if not verified free
	return int(atomic.AddUint32(&globalPortCounter, 1))
}

// isPortFree checks if a port is available for binding
func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// waitForReady polls a server's /health endpoint until it responds with 200,
// replacing fixed time.Sleep waits after srv.Start(). Fails the test if the
// server doesn't become ready within the timeout.
func waitForReady(t *testing.T, port int) {
	t.Helper()
	url := fmt.Sprintf("http://localhost:%d/health", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // test helper
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond) // Polling interval
	}
	t.Fatalf("server on port %d never became ready (waited 5s)", port)
}

// waitForTCPReady polls a TCP port until it accepts connections, for use with
// non-HTTP servers like gRPC. Fails the test if the port doesn't accept
// connections within the timeout.
func waitForTCPReady(t *testing.T, port int) {
	t.Helper()
	addr := fmt.Sprintf("localhost:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond) // Polling interval
	}
	t.Fatalf("TCP port %d never became ready (waited 5s)", port)
}
