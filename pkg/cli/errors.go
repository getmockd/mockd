package cli

import "errors"

// Common CLI errors
var (
	ErrProxyNotRunning  = errors.New("proxy not running - start with: mockd proxy start")
	ErrServerNotRunning = errors.New("server not running - start with: mockd start")
)
