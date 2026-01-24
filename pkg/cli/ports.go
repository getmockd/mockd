package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/getmockd/mockd/pkg/cliconfig"
)

// PortInfo represents information about a single port.
type PortInfo struct {
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Component string `json:"component"`
	Status    string `json:"status"`
	TLS       bool   `json:"tls,omitempty"`
}

// PortsOutput represents the JSON output format for ports command.
type PortsOutput struct {
	Ports   []PortInfo `json:"ports"`
	Running bool       `json:"running"`
}

// PortsResponse is the response from the admin API /ports endpoint.
type PortsResponse struct {
	Ports []PortInfo `json:"ports"`
}

// RunPorts handles the ports command.
func RunPorts(args []string) error {
	fs := flag.NewFlagSet("ports", flag.ContinueOnError)

	pidFile := fs.String("pid-file", "", "Path to PID file (default: ~/.mockd/mockd.pid)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	adminPort := fs.Int("admin-port", cliconfig.DefaultAdminPort, "Admin API port to query")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd ports [flags]

Show all ports in use by the running mockd server.

Flags:
      --pid-file    Path to PID file (default: ~/.mockd/mockd.pid)
      --json        Output in JSON format
  -a, --admin-port  Admin API port to query (default: 4290)

Examples:
  # Show all ports
  mockd ports

  # Output as JSON
  mockd ports --json

  # Query a different admin port
  mockd ports --admin-port 8090
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

	// Try to read PID file first for port information
	info, err := ReadPIDFile(pidPath)
	if err == nil && info.IsRunning() {
		// Use admin port from PID file if available
		if info.Components.Admin.Enabled && info.Components.Admin.Port > 0 {
			*adminPort = info.Components.Admin.Port
		}
	}

	// Try to get live port info from admin API
	adminURL := fmt.Sprintf("http://localhost:%d", *adminPort)
	ports, err := fetchPortsFromAPI(adminURL)
	if err != nil {
		// Fall back to PID file information
		if info != nil && info.IsRunning() {
			return printPortsFromPIDFile(info, *jsonOutput)
		}
		return printNotRunningPorts(*jsonOutput)
	}

	return printPorts(ports, *jsonOutput)
}

// fetchPortsFromAPI fetches port information from the admin API.
func fetchPortsFromAPI(adminURL string) ([]PortInfo, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(adminURL + "/ports")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("admin API returned status %d", resp.StatusCode)
	}

	var result PortsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Ports, nil
}

// printPortsFromPIDFile prints port information from the PID file.
func printPortsFromPIDFile(info *PIDFile, jsonOutput bool) error {
	var ports []PortInfo

	// Add engine HTTP port
	if info.Components.Engine.Enabled && info.Components.Engine.Port > 0 {
		ports = append(ports, PortInfo{
			Port:      info.Components.Engine.Port,
			Protocol:  "HTTP",
			Component: "Mock Engine",
			Status:    "running",
		})
	}

	// Add engine HTTPS port
	if info.Components.Engine.Enabled && info.Components.Engine.HTTPSPort > 0 {
		ports = append(ports, PortInfo{
			Port:      info.Components.Engine.HTTPSPort,
			Protocol:  "HTTPS",
			Component: "Mock Engine",
			Status:    "running",
			TLS:       true,
		})
	}

	// Add admin API port
	if info.Components.Admin.Enabled && info.Components.Admin.Port > 0 {
		ports = append(ports, PortInfo{
			Port:      info.Components.Admin.Port,
			Protocol:  "HTTP",
			Component: "Admin API",
			Status:    "running",
		})
	}

	return printPorts(ports, jsonOutput)
}

// printPorts prints the port information in the requested format.
func printPorts(ports []PortInfo, jsonOutput bool) error {
	if jsonOutput {
		output := PortsOutput{
			Ports:   ports,
			Running: len(ports) > 0,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	if len(ports) == 0 {
		fmt.Println("No ports in use by mockd")
		return nil
	}

	// Sort ports by port number
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	// Print header
	fmt.Println()
	fmt.Printf("%-7s %-10s %-15s %s\n", "PORT", "PROTOCOL", "COMPONENT", "STATUS")
	fmt.Println("------- ---------- --------------- --------")

	// Print each port
	for _, p := range ports {
		status := p.Status
		if p.TLS {
			status += " (TLS)"
		}
		fmt.Printf("%-7d %-10s %-15s %s\n", p.Port, p.Protocol, p.Component, status)
	}
	fmt.Println()

	return nil
}

// printNotRunningPorts prints a message when mockd is not running.
func printNotRunningPorts(jsonOutput bool) error {
	if jsonOutput {
		output := PortsOutput{
			Ports:   []PortInfo{},
			Running: false,
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
