package cli

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// RunStop handles the stop command.
func RunStop(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)

	pidFile := fs.String("pid-file", "", "Path to PID file (default: ~/.mockd/mockd.pid)")
	force := fs.Bool("force", false, "Send SIGKILL instead of SIGTERM")
	fs.BoolVar(force, "f", false, "Send SIGKILL instead of SIGTERM (shorthand)")
	timeout := fs.Int("timeout", 10, "Timeout in seconds to wait for graceful shutdown")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stop [component] [flags]

Stop the running mockd server.

Arguments:
  component    Optional component to stop: "admin" or "engine"
               If not specified, stops all components

Flags:
      --pid-file    Path to PID file (default: ~/.mockd/mockd.pid)
  -f, --force       Send SIGKILL instead of SIGTERM
      --timeout     Timeout in seconds to wait for shutdown (default: 10)

Examples:
  # Stop all components
  mockd stop

  # Force stop
  mockd stop --force

  # Stop with custom PID file
  mockd stop --pid-file /tmp/mockd.pid

  # Stop with longer timeout
  mockd stop --timeout 30

Note: Stopping individual components (admin/engine) is not yet supported.
      This will stop the entire mockd process.
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check for component argument (not yet supported but reserved)
	component := ""
	if fs.NArg() > 0 {
		component = fs.Arg(0)
		if component != "admin" && component != "engine" && component != "" {
			return fmt.Errorf("unknown component: %s (valid: admin, engine)", component)
		}
		if component != "" {
			fmt.Fprintf(os.Stderr, "Note: Stopping individual components is not yet supported.\n")
			fmt.Fprintf(os.Stderr, "      Stopping entire mockd process instead.\n\n")
		}
	}

	// Determine PID file path
	pidPath := *pidFile
	if pidPath == "" {
		pidPath = DefaultPIDPath()
	}

	// Read PID file
	info, err := ReadPIDFile(pidPath)
	if err != nil {
		return fmt.Errorf("mockd is not running (no PID file found at %s)", pidPath)
	}

	// Check if process is actually running
	if !info.IsRunning() {
		// Stale PID file - clean it up
		RemovePIDFile(pidPath)
		return fmt.Errorf("mockd is not running (stale PID file removed)")
	}

	// Find the process
	process, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", info.PID, err)
	}

	// Determine which signal to send
	sig := signalTerm
	sigName := signalTermName()
	if *force {
		sig = signalKill
		sigName = signalKillName()
	}

	fmt.Printf("Stopping mockd (PID %d) with %s... ", info.PID, sigName)

	// Send signal
	if err := process.Signal(sig); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("failed to send signal: %w", err)
	}

	// For SIGKILL, we don't wait gracefully
	if *force {
		fmt.Println("done")
		// Wait a moment then clean up PID file
		time.Sleep(100 * time.Millisecond)
		RemovePIDFile(pidPath)
		return nil
	}

	// Wait for process to exit with timeout
	timeoutDuration := time.Duration(*timeout) * time.Second
	deadline := time.Now().Add(timeoutDuration)

	for time.Now().Before(deadline) {
		if !checkProcessRunning(info.PID) {
			fmt.Println("done")
			RemovePIDFile(pidPath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Timeout reached - process didn't stop
	fmt.Println("timeout")
	fmt.Printf("\nProcess did not stop within %d seconds.\n", *timeout)
	fmt.Println("Try: mockd stop --force")
	return fmt.Errorf("timeout waiting for process to stop")
}

// processIsRunning is defined in stop_unix.go and stop_windows.go
// as checkProcessRunning for platform-specific implementations.
