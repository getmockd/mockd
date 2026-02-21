package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
)

var (
	stopPidFile string
	stopForce   bool
	stopTimeout int
)

var stopCmd = &cobra.Command{
	Use:   "stop [component]",
	Short: "Stop the running mockd server",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for component argument (not yet supported but reserved)
		component := ""
		if len(args) > 0 {
			component = args[0]
			if component != "admin" && component != "engine" && component != "" {
				return fmt.Errorf("unknown component: %s (valid: admin, engine)", component)
			}
			if component != "" {
				fmt.Fprintf(os.Stderr, "Note: Stopping individual components is not yet supported.\n")
				fmt.Fprintf(os.Stderr, "      Stopping entire mockd process instead.\n\n")
			}
		}

		// If admin-url was explicitly provided, verify the server is reachable before trying to stop
		if cmd.Flags().Changed("admin-url") {
			resolvedAdminURL := cliconfig.ResolveAdminURL(adminURL)
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(resolvedAdminURL + "/health")
			if err != nil {
				return fmt.Errorf("cannot reach admin API at %s: %w", resolvedAdminURL, err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("admin API at %s returned status %d", resolvedAdminURL, resp.StatusCode)
			}
		}

		// Determine PID file path
		pidPath := stopPidFile
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
			_ = RemovePIDFile(pidPath)
			return errors.New("mockd is not running (stale PID file removed)")
		}

		// Find the process
		process, err := os.FindProcess(info.PID)
		if err != nil {
			return fmt.Errorf("failed to find process %d: %w", info.PID, err)
		}

		// Determine which signal to send
		sig := signalTerm
		sigName := signalTermName()
		if stopForce {
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
		if stopForce {
			fmt.Println("done")
			// Wait a moment then clean up PID file
			time.Sleep(100 * time.Millisecond)
			_ = RemovePIDFile(pidPath)
			return nil
		}

		// Wait for process to exit with timeout
		timeoutDuration := time.Duration(stopTimeout) * time.Second
		deadline := time.Now().Add(timeoutDuration)

		for time.Now().Before(deadline) {
			if !checkProcessRunning(info.PID) {
				fmt.Println("done")
				_ = RemovePIDFile(pidPath)
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Timeout reached - process didn't stop
		fmt.Println("timeout")
		fmt.Printf("\nProcess did not stop within %d seconds.\n", stopTimeout)
		fmt.Println("Try: mockd stop --force")
		return errors.New("timeout waiting for process to stop")
	},
}

// processIsRunning is defined in stop_unix.go and stop_windows.go
// as checkProcessRunning for platform-specific implementations.

func init() {
	stopCmd.Flags().StringVar(&stopPidFile, "pid-file", "", "Path to PID file (default: ~/.mockd/mockd.pid)")
	stopCmd.Flags().BoolVarP(&stopForce, "force", "f", false, "Send SIGKILL instead of SIGTERM")
	stopCmd.Flags().IntVar(&stopTimeout, "timeout", 10, "Timeout in seconds to wait for graceful shutdown")
	rootCmd.AddCommand(stopCmd)
}
