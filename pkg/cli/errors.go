package cli

import "errors"

// Common CLI errors
var (
	ErrNoRecordings     = errors.New("no recordings found - run 'mockd proxy start' to capture traffic")
	ErrServerNotRunning = errors.New("server not running - start with: mockd start")
)
