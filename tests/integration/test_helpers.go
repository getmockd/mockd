// Package integration provides integration tests for the mockd server.
package integration

import (
	"fmt"
	"net"
	"sync/atomic"
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
