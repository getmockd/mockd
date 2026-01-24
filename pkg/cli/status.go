package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/cliconfig"
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
	adminPort := fs.Int("admin-port", cliconfig.DefaultAdminPort, "Admin API port to probe")
	enginePort := fs.Int("port", cliconfig.DefaultPort, "Mock engine port to probe")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd status [flags]

Show the status of the running mockd server.

Flags:
      --pid-file    Path to PID file (default: ~/.mockd/mockd.pid)
      --json        Output in JSON format
  -p, --port        Mock engine port to probe (default: 4280)
  -a, --admin-port  Admin API port to probe (default: 4290)

Examples:
  # Check server status
  mockd status

  # Output as JSON
  mockd status --json

  # Use custom PID file
  mockd status --pid-file /tmp/mockd.pid

  # Check status on custom ports
  mockd status --port 8080 --admin-port 8090
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

	// Try to read PID file first
	info, err := ReadPIDFile(pidPath)
	if err == nil && info.IsRunning() {
		// PID file exists and process is running - use it
		return printRunningFromPIDFile(info, *jsonOutput)
	}

	// No PID file or stale PID file - try port detection
	detected := detectRunningServer(*adminPort, *enginePort)
	if detected != nil {
		return printRunningFromDetection(detected, *jsonOutput)
	}

	// Nothing found
	return printNotRunning(*jsonOutput)
}

// printRunningFromPIDFile prints status when we have PID file info.
func printRunningFromPIDFile(info *PIDFile, jsonOutput bool) error {
	output := buildStatusOutput(info)

	// Try to fetch live stats from admin API
	if info.Components.Admin.Enabled {
		stats, healthInfo := fetchLiveStats(info.AdminURL())
		if stats != nil {
			output.Stats = stats
		}
		// Update uptime from live health check if available
		if healthInfo != nil && healthInfo.Uptime > 0 {
			output.Uptime = formatDuration(time.Duration(healthInfo.Uptime) * time.Second)
		}
	}

	if jsonOutput {
		return printJSONStatus(output)
	}

	return printHumanStatus(output, info)
}

// DetectedServer contains information about a server detected via port probing.
type DetectedServer struct {
	AdminRunning  bool
	EngineRunning bool
	AdminPort     int
	EnginePort    int
	AdminURL      string
	EngineURL     string
	Version       string
	Stats         *StatusStats
	HealthInfo    *HealthInfo
}

// detectRunningServer probes the default ports to detect a running mockd server.
func detectRunningServer(adminPort, enginePort int) *DetectedServer {
	detected := &DetectedServer{
		AdminPort:  adminPort,
		EnginePort: enginePort,
		AdminURL:   fmt.Sprintf("http://localhost:%d", adminPort),
		EngineURL:  fmt.Sprintf("http://localhost:%d", enginePort),
	}

	// Check admin API health
	client := &http.Client{Timeout: 2 * time.Second}
	adminResp, err := client.Get(detected.AdminURL + "/health")
	if err == nil {
		defer func() { _ = adminResp.Body.Close() }()
		if adminResp.StatusCode == http.StatusOK {
			detected.AdminRunning = true

			// Try to get stats and health info
			stats, healthInfo := fetchLiveStats(detected.AdminURL)
			detected.Stats = stats
			detected.HealthInfo = healthInfo
		}
	}

	// Check if engine port is responding (could be any HTTP response)
	engineResp, err := client.Get(detected.EngineURL + "/")
	if err == nil {
		defer func() { _ = engineResp.Body.Close() }()
		// Any response means the engine is running (even 404 is valid - no mock matched)
		detected.EngineRunning = true
	}

	// Return nil if neither is running
	if !detected.AdminRunning && !detected.EngineRunning {
		return nil
	}

	return detected
}

// printRunningFromDetection prints status based on port detection (no PID file).
func printRunningFromDetection(detected *DetectedServer, jsonOutput bool) error {
	output := StatusOutput{
		Running: true,
		Components: StatusComponents{
			Admin: StatusComponentInfo{
				Status: "stopped",
			},
			Engine: StatusComponentInfo{
				Status: "stopped",
			},
		},
	}

	if detected.AdminRunning {
		output.Components.Admin = StatusComponentInfo{
			Status: "running",
			URL:    detected.AdminURL,
		}
	}

	if detected.EngineRunning {
		output.Components.Engine = StatusComponentInfo{
			Status: "running",
			URL:    detected.EngineURL,
		}
	}

	if detected.Stats != nil {
		output.Stats = detected.Stats
	}

	// Get uptime from health info
	if detected.HealthInfo != nil && detected.HealthInfo.Uptime > 0 {
		output.Uptime = formatDuration(time.Duration(detected.HealthInfo.Uptime) * time.Second)
	}

	if jsonOutput {
		return printJSONStatus(output)
	}

	return printHumanStatusDetected(output)
}

// printHumanStatusDetected prints human-readable status for detected server (no PID file).
func printHumanStatusDetected(output StatusOutput) error {
	fmt.Println("mockd (detected via port)")
	fmt.Println()

	// Components section
	fmt.Println("Components:")

	// Admin API
	adminStatus := output.Components.Admin
	if adminStatus.Status == "running" {
		uptimeStr := ""
		if output.Uptime != "" {
			uptimeStr = fmt.Sprintf("  (uptime: %s)", output.Uptime)
		}
		fmt.Printf("  Admin API    %s  %s%s\n",
			colorGreen("running"),
			adminStatus.URL,
			uptimeStr)
	} else {
		fmt.Printf("  Admin API    %s\n", colorRed("stopped"))
	}

	// Mock Engine
	engineStatus := output.Components.Engine
	if engineStatus.Status == "running" {
		fmt.Printf("  Mock Engine  %s  %s\n",
			colorGreen("running"),
			engineStatus.URL)
	} else {
		fmt.Printf("  Mock Engine  %s\n", colorRed("stopped"))
	}

	// Stats section (if available)
	if output.Stats != nil {
		fmt.Println()
		fmt.Println("Stats:")
		fmt.Printf("  Mocks loaded:      %s\n", formatNumber(output.Stats.MocksLoaded))
		fmt.Printf("  Requests served:   %s\n", formatNumber(output.Stats.RequestsServed))
	}

	// Note about PID file
	fmt.Println()
	fmt.Printf("%s No PID file found. Server may have been started manually.\n",
		colorYellow("Note:"))

	return nil
}

// colorYellow returns text wrapped in ANSI yellow color codes.
func colorYellow(s string) string {
	if !isTerminal() {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
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

// HealthInfo contains health check response data.
type HealthInfo struct {
	Status string `json:"status"`
	Uptime int    `json:"uptime"`
}

// fetchLiveStats fetches live statistics from the admin API.
// It uses /health for uptime and /mocks for mock count since there's no /stats endpoint.
func fetchLiveStats(adminURL string) (*StatusStats, *HealthInfo) {
	if adminURL == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	stats := &StatusStats{}
	var healthInfo *HealthInfo

	// Get health info (includes uptime)
	healthResp, err := client.Get(adminURL + "/health")
	if err == nil {
		defer func() { _ = healthResp.Body.Close() }()
		if healthResp.StatusCode == http.StatusOK {
			var health HealthInfo
			if json.NewDecoder(healthResp.Body).Decode(&health) == nil {
				healthInfo = &health
			}
		}
	}

	// Get mock count from /mocks endpoint
	adminClient := NewAdminClientWithAuth(adminURL, WithTimeout(2*time.Second))
	mocks, err := adminClient.ListMocks()
	if err == nil {
		stats.MocksLoaded = len(mocks)
	}

	return stats, healthInfo
}

// printJSONStatus prints status in JSON format.
func printJSONStatus(output StatusOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// printHumanStatus prints status in human-readable format.
func printHumanStatus(output StatusOutput, _ *PIDFile) error {
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
//
//nolint:unparam // s is always the same value but function is intentionally generic
func colorGreen(s string) string {
	if !isTerminal() {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

// colorRed returns text wrapped in ANSI red color codes.
//
//nolint:unparam // s is always the same value but function is intentionally generic
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
