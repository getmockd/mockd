package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// RunDown stops services started by 'mockd up'.
func RunDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)

	pidFile := fs.String("pid-file", defaultUpPIDPath(), "Path to PID file")
	timeout := fs.Duration("timeout", 30*time.Second, "Shutdown timeout")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd down [flags]

Stop services started by 'mockd up'.

Flags:
      --pid-file <path>  Path to PID file (default: ~/.mockd/mockd.pid)
      --timeout          Shutdown timeout (default: 30s)

Examples:
  # Stop services
  mockd down

  # Stop with custom timeout
  mockd down --timeout 60s
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Read PID file
	pidInfo, err := readUpPIDFile(*pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No running mockd services found.")
			return nil
		}
		return fmt.Errorf("reading PID file: %w", err)
	}

	// Check if main process is running
	if !processExists(pidInfo.PID) {
		fmt.Println("mockd is not running (stale PID file)")
		_ = os.Remove(*pidFile)
		return nil
	}

	fmt.Printf("Stopping mockd (PID %d)...\n", pidInfo.PID)

	// Send SIGTERM to main process
	proc, err := os.FindProcess(pidInfo.PID)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending signal: %w", err)
	}

	// Wait for process to exit
	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		if !processExists(pidInfo.PID) {
			fmt.Println("mockd stopped")
			_ = os.Remove(*pidFile)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running
	fmt.Println("Timeout reached, force killing...")
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("force kill: %w", err)
	}

	_ = os.Remove(*pidFile)
	fmt.Println("mockd stopped (forced)")
	return nil
}

func defaultUpPIDPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/mockd.pid"
	}
	return home + "/.mockd/mockd.pid"
}

func readUpPIDFile(path string) (*config.PIDFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pidInfo config.PIDFile
	if err := json.Unmarshal(data, &pidInfo); err != nil {
		return nil, fmt.Errorf("parsing PID file: %w", err)
	}

	return &pidInfo, nil
}

func writeUpPIDFile(path string, pidInfo *config.PIDFile) error {
	// Ensure directory exists
	dir := path[:len(path)-len("/mockd.pid")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(pidInfo, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check if process exists.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
