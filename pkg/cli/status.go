package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

// StatusOutput represents the JSON output format for status.
type StatusOutput struct {
	Version    string           `json:"version"`
	Commit     string           `json:"commit,omitempty"`
	Uptime     string           `json:"uptime"`
	Components StatusComponents `json:"components"`
	Stats      *StatusStats     `json:"stats,omitempty"`
	Running    bool             `json:"running"`
	PID        int              `json:"pid,omitempty"`
}

// StatusComponents contains status for each component.
type StatusComponents struct {
	Admin  StatusComponentInfo `json:"admin"`
	Engine StatusComponentInfo `json:"engine"`
}

// StatusComponentInfo contains detailed status for a component.
type StatusComponentInfo struct {
	Status   string `json:"status"` // "running", "stopped", "unknown"
	URL      string `json:"url,omitempty"`
	HTTPSURL string `json:"httpsUrl,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
}

// StatusStats contains server statistics.
type StatusStats struct {
	MocksLoaded       int `json:"mocksLoaded"`
	RequestsServed    int `json:"requestsServed"`
	RequestsMatched   int `json:"requestsMatched"`
	RequestsUnmatched int `json:"requestsUnmatched"`
}

// RunStatus handles the status command.
func RunStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)

	pidFile := fs.String("pid-file", "", "Path to PID file (default: ~/.mockd/mockd.pid)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd status [flags]

Show the status of the running mockd server.

Flags:
      --pid-file    Path to PID file (default: ~/.mockd/mockd.pid)
      --json        Output in JSON format

Examples:
  # Check server status
  mockd status

  # Output as JSON
  mockd status --json

  # Use custom PID file
  mockd status --pid-file /tmp/mockd.pid
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Determine PID file path
	pidPath := *pidFile
	if pidPath == "" {
		pidPath = DefaultPIDPath()
	}

	// Try to read PID file
	info, err := ReadPIDFile(pidPath)
	if err != nil {
		// PID file doesn't exist or is invalid
		return printNotRunning(*jsonOutput)
	}

	// Check if process is actually running
	if !info.IsRunning() {
		// Stale PID file - process is not running
		return printNotRunning(*jsonOutput)
	}

	// Build status output
	output := buildStatusOutput(info)

	// Try to fetch live stats from admin API
	if info.Components.Admin.Enabled {
		stats := fetchLiveStats(info.AdminURL())
		if stats != nil {
			output.Stats = stats
		}
	}

	if *jsonOutput {
		return printJSONStatus(output)
	}

	return printHumanStatus(output, info)
}

// printNotRunning prints the "not running" status.
func printNotRunning(jsonOutput bool) error {
	if jsonOutput {
		output := StatusOutput{
			Running: false,
			Components: StatusComponents{
				Admin:  StatusComponentInfo{Status: "stopped"},
				Engine: StatusComponentInfo{Status: "stopped"},
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Println("mockd is not running")
	fmt.Println()
	fmt.Println("To start: mockd serve")
	return nil
}

// buildStatusOutput creates a StatusOutput from PID file info.
func buildStatusOutput(info *PIDFile) StatusOutput {
	output := StatusOutput{
		Version: info.Version,
		Commit:  info.Commit,
		Uptime:  info.FormatUptime(),
		Running: true,
		PID:     info.PID,
		Components: StatusComponents{
			Admin: StatusComponentInfo{
				Status: "stopped",
			},
			Engine: StatusComponentInfo{
				Status: "stopped",
			},
		},
	}

	// Admin component
	if info.Components.Admin.Enabled {
		output.Components.Admin = StatusComponentInfo{
			Status: "running",
			URL:    info.AdminURL(),
			Uptime: info.FormatUptime(),
		}
	}

	// Engine component
	if info.Components.Engine.Enabled {
		output.Components.Engine = StatusComponentInfo{
			Status:   "running",
			URL:      info.EngineURL(),
			HTTPSURL: info.EngineHTTPSURL(),
			Uptime:   info.FormatUptime(),
		}
	}

	return output
}

// fetchLiveStats fetches live statistics from the admin API.
func fetchLiveStats(adminURL string) *StatusStats {
	if adminURL == "" {
		return nil
	}

	client := NewAdminClient(adminURL, WithTimeout(2*time.Second))
	result, err := client.GetStats()
	if err != nil {
		return nil
	}

	return &StatusStats{
		MocksLoaded:    result.MockCount,
		RequestsServed: int(result.TotalRequests),
	}
}

// printJSONStatus prints status in JSON format.
func printJSONStatus(output StatusOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// printHumanStatus prints status in human-readable format.
func printHumanStatus(output StatusOutput, info *PIDFile) error {
	// Header
	if output.Commit != "" {
		fmt.Printf("mockd v%s (%s)\n", output.Version, output.Commit)
	} else {
		fmt.Printf("mockd v%s\n", output.Version)
	}
	fmt.Println()

	// Components section
	fmt.Println("Components:")

	// Admin API
	adminStatus := output.Components.Admin
	if adminStatus.Status == "running" {
		fmt.Printf("  Admin API    %s  %s  (uptime: %s)\n",
			colorGreen("running"),
			adminStatus.URL,
			output.Uptime)
	} else {
		fmt.Printf("  Admin API    %s\n", colorRed("stopped"))
	}

	// Mock Engine
	engineStatus := output.Components.Engine
	if engineStatus.Status == "running" {
		fmt.Printf("  Mock Engine  %s  %s  (uptime: %s)\n",
			colorGreen("running"),
			engineStatus.URL,
			output.Uptime)
		if engineStatus.HTTPSURL != "" {
			fmt.Printf("               HTTPS    %s\n", engineStatus.HTTPSURL)
		}
	} else {
		fmt.Printf("  Mock Engine  %s\n", colorRed("stopped"))
	}

	// Stats section (if available)
	if output.Stats != nil {
		fmt.Println()
		fmt.Println("Stats:")
		fmt.Printf("  Mocks loaded:      %s\n", formatNumber(output.Stats.MocksLoaded))
		fmt.Printf("  Requests served:   %s\n", formatNumber(output.Stats.RequestsServed))
		if output.Stats.RequestsServed > 0 {
			matchRate := float64(output.Stats.RequestsMatched) / float64(output.Stats.RequestsServed) * 100
			fmt.Printf("  Requests matched:  %s (%.1f%%)\n",
				formatNumber(output.Stats.RequestsMatched),
				matchRate)
		}
	}

	return nil
}

// colorGreen returns text wrapped in ANSI green color codes.
func colorGreen(s string) string {
	if !isTerminal() {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

// colorRed returns text wrapped in ANSI red color codes.
func colorRed(s string) string {
	if !isTerminal() {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

// isTerminal checks if stdout is a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// formatNumber formats an integer with thousands separators.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Simple implementation for common cases
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}
