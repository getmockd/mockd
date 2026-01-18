//go:build !windows

package cli

import (
	"os"
	"syscall"
)

// Signals for Unix systems
var (
	signalTerm = syscall.SIGTERM
	signalKill = syscall.SIGKILL
)

// signalTermName returns the name of the graceful shutdown signal.
func signalTermName() string {
	return "SIGTERM"
}

// signalKillName returns the name of the force kill signal.
func signalKillName() string {
	return "SIGKILL"
}

// checkProcessRunning checks if a process is running using signal 0.
func checkProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// processIsRunning is an alias for checkProcessRunning for backwards compatibility.
var processIsRunning = checkProcessRunning
