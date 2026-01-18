//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

// Signals for Windows systems
// Windows doesn't have SIGTERM, so we use os.Interrupt for graceful and os.Kill for force
var (
	signalTerm = os.Interrupt
	signalKill = os.Kill
)

// signalTermName returns the name of the graceful shutdown signal.
func signalTermName() string {
	return "interrupt"
}

// signalKillName returns the name of the force kill signal.
func signalKillName() string {
	return "kill"
}

// checkProcessRunning checks if a process is running on Windows.
func checkProcessRunning(pid int) bool {
	// On Windows, we need to open the process with SYNCHRONIZE access
	// and check if it's still running
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	// Check if process has exited (wait with zero timeout)
	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}

	// WAIT_TIMEOUT means process is still running
	return event == uint32(windows.WAIT_TIMEOUT)
}

// processIsRunning is an alias for checkProcessRunning for backwards compatibility.
var processIsRunning = checkProcessRunning
