package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/spf13/cobra"
)

var (
	downPidFile string
	downTimeout time.Duration
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop services started by 'mockd up'",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		pidFile := &downPidFile
		timeout := &downTimeout

		// Read PID file
		pidInfo, err := readUpPIDFile(*pidFile)
		if err != nil {
			if os.IsNotExist(err) {
				printResult(map[string]any{"stopped": false, "reason": "no running services found"}, func() {
					fmt.Println("No running mockd services found.")
				})
				return nil
			}
			return fmt.Errorf("reading PID file: %w", err)
		}

		// Check if main process is running
		if !processExists(pidInfo.PID) {
			printResult(map[string]any{"stopped": false, "reason": "not running (stale PID file)"}, func() {
				fmt.Println("mockd is not running (stale PID file)")
			})
			_ = os.Remove(*pidFile)
			return nil
		}

		if !jsonOutput {
			fmt.Printf("Stopping mockd (PID %d)...\n", pidInfo.PID)
		}

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
				_ = os.Remove(*pidFile)
				printResult(map[string]any{"stopped": true, "pid": pidInfo.PID, "forced": false}, func() {
					fmt.Println("mockd stopped")
				})
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Force kill if still running
		if !jsonOutput {
			fmt.Println("Timeout reached, force killing...")
		}
		if err := proc.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("force kill: %w", err)
		}

		_ = os.Remove(*pidFile)
		printResult(map[string]any{"stopped": true, "pid": pidInfo.PID, "forced": true}, func() {
			fmt.Println("mockd stopped (forced)")
		})
		return nil
	},
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

func init() {
	downCmd.Flags().StringVar(&downPidFile, "pid-file", defaultUpPIDPath(), "Path to PID file")
	downCmd.Flags().DurationVar(&downTimeout, "timeout", 30*time.Second, "Shutdown timeout")
	rootCmd.AddCommand(downCmd)
}
