// Package ports provides port availability checking.
package ports

import (
	"fmt"
	"net"
)

// Consolidates:
// - start.go:checkPort (505-513)
// - serve.go:checkServePort (652-659)
// - doctor.go:checkPortAvailable (138-146)

// IsAvailable checks if a port is available for binding.
// Returns true if the port is available, false otherwise.
func IsAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// Check checks if a port is available and returns an error if not.
func Check(port int) error {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}
