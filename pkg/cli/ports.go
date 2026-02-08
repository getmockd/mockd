package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
)

// PortInfo represents information about a single port.
type PortInfo struct {
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Component string `json:"component"`
	Status    string `json:"status"`
	TLS       bool   `json:"tls,omitempty"`

	// Extended info (populated with --verbose)
	EngineID   string `json:"engineId,omitempty"`
	EngineName string `json:"engineName,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	PID        int    `json:"pid,omitempty"`
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
	verbose := fs.Bool("verbose", false, "Show extended info (engine ID, name, workspace)")
	fs.BoolVar(verbose, "v", false, "Show extended info (shorthand)")
	adminURL := fs.String("admin-url", "", "Admin API URL (default: from context)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd ports [flags]

Show all ports in use by the running mockd server.

Flags:
      --pid-file    Path to PID file (default: ~/.mockd/mockd.pid)
      --json        Output in JSON format
  -v, --verbose     Show extended info (engine ID, name, workspace)
      --admin-url   Admin API URL (default: from context/config)

Examples:
  # Show all ports
  mockd ports

  # Show with engine details
  mockd ports --verbose

  # Output as JSON
  mockd ports --json

  # Query a specific admin server
  mockd ports --admin-url http://staging:4290
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve config from context/env/flags
	cfg := cliconfig.ResolveClientConfigSimple(*adminURL)

	// Determine PID file path
	pidPath := *pidFile
	if pidPath == "" {
		pidPath = DefaultPIDPath()
	}

	// Try to read PID file first for port information (fallback only)
	pidInfo, _ := ReadPIDFile(pidPath)

	// Try to get live port info from admin API using authenticated client
	client := NewAdminClientWithAuth(cfg.AdminURL, WithAPIKey(cfg.APIKey))
	ports, err := client.GetPortsVerbose(*verbose)
	if err != nil {
		// Fall back to PID file information
		if pidInfo != nil && pidInfo.IsRunning() {
			return printPortsFromPIDFile(pidInfo, *jsonOutput, *verbose)
		}
		return printNotRunningPorts(*jsonOutput)
	}

	return printPorts(ports, *jsonOutput, *verbose)
}

// printPortsFromPIDFile prints port information from the PID file.
func printPortsFromPIDFile(info *PIDFile, jsonOutput, verbose bool) error {
	var ports []PortInfo

	// Add engine HTTP port
	if info.Components.Engine.Enabled && info.Components.Engine.Port > 0 {
		p := PortInfo{
			Port:      info.Components.Engine.Port,
			Protocol:  "HTTP",
			Component: "Mock Engine",
			Status:    "running",
		}
		if verbose {
			p.PID = info.PID
		}
		ports = append(ports, p)
	}

	// Add engine HTTPS port
	if info.Components.Engine.Enabled && info.Components.Engine.HTTPSPort > 0 {
		p := PortInfo{
			Port:      info.Components.Engine.HTTPSPort,
			Protocol:  "HTTPS",
			Component: "Mock Engine",
			Status:    "running",
			TLS:       true,
		}
		if verbose {
			p.PID = info.PID
		}
		ports = append(ports, p)
	}

	// Add admin API port
	if info.Components.Admin.Enabled && info.Components.Admin.Port > 0 {
		p := PortInfo{
			Port:      info.Components.Admin.Port,
			Protocol:  "HTTP",
			Component: "Admin API",
			Status:    "running",
		}
		if verbose {
			p.PID = info.PID
		}
		ports = append(ports, p)
	}

	return printPorts(ports, jsonOutput, verbose)
}

// printPorts prints the port information in the requested format.
func printPorts(ports []PortInfo, jsonOutput, verbose bool) error {
	if jsonOutput {
		result := PortsOutput{
			Ports:   ports,
			Running: len(ports) > 0,
		}
		return output.JSON(result)
	}

	if len(ports) == 0 {
		fmt.Println("No ports in use by mockd")
		return nil
	}

	// Sort ports by port number
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	fmt.Println()

	if verbose {
		// Verbose output with engine info
		fmt.Printf("%-7s %-10s %-15s %-10s %-20s %s\n", "PORT", "PROTOCOL", "COMPONENT", "STATUS", "ENGINE", "ID")
		fmt.Println("------- ---------- --------------- ---------- -------------------- --------")

		for _, p := range ports {
			status := p.Status
			if p.TLS {
				status += " (TLS)"
			}
			engineName := p.EngineName
			if engineName == "" {
				engineName = "-"
			}
			engineID := p.EngineID
			if engineID == "" {
				engineID = "-"
			}
			// Truncate long IDs
			if len(engineID) > 8 {
				engineID = engineID[:8]
			}
			fmt.Printf("%-7d %-10s %-15s %-10s %-20s %s\n", p.Port, p.Protocol, p.Component, status, engineName, engineID)
		}
	} else {
		// Standard output
		fmt.Printf("%-7s %-10s %-15s %s\n", "PORT", "PROTOCOL", "COMPONENT", "STATUS")
		fmt.Println("------- ---------- --------------- --------")

		for _, p := range ports {
			status := p.Status
			if p.TLS {
				status += " (TLS)"
			}
			fmt.Printf("%-7d %-10s %-15s %s\n", p.Port, p.Protocol, p.Component, status)
		}
	}
	fmt.Println()

	return nil
}

// printNotRunningPorts prints a message when mockd is not running.
func printNotRunningPorts(jsonOutput bool) error {
	if jsonOutput {
		result := PortsOutput{
			Ports:   []PortInfo{},
			Running: false,
		}
		return output.JSON(result)
	}

	fmt.Println("mockd is not running")
	fmt.Println()
	fmt.Println("To start: mockd serve")
	return nil
}
