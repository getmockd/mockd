package cli

import (
	"fmt"
	"sort"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
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

var (
	portsPIDFile string
	portsVerbose bool
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show all ports in use by the running mockd server",
	Long: `Show all ports in use by the running mockd server.

Lists all listening ports for mock engines, admin APIs, and protocol-specific
listeners (gRPC, MQTT, etc.).`,
	Example: `  # Show all ports
  mockd ports

  # Show with engine details
  mockd ports --verbose

  # Output as JSON
  mockd ports --json

  # Query a specific admin server
  mockd ports --admin-url http://staging:4290`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve config from context/env/flags — uses root persistent adminURL
		cfg := cliconfig.ResolveClientConfigSimple(adminURL)

		// Determine PID file path
		pidPath := portsPIDFile
		if pidPath == "" {
			pidPath = DefaultPIDPath()
		}

		// Try to read PID file first for port information (fallback only)
		pidInfo, _ := ReadPIDFile(pidPath)

		// Try to get live port info from admin API using authenticated client
		client := NewAdminClientWithAuth(cfg.AdminURL, WithAPIKey(cfg.APIKey))
		ports, err := client.GetPortsVerbose(portsVerbose)
		if err != nil {
			// Fall back to PID file information
			if pidInfo != nil && pidInfo.IsRunning() {
				return printPortsFromPIDFile(pidInfo, jsonOutput, portsVerbose)
			}
			return printNotRunningPorts(jsonOutput)
		}

		return printPorts(ports, jsonOutput, portsVerbose)
	},
}

func init() {
	portsCmd.Flags().StringVar(&portsPIDFile, "pid-file", "", "Path to PID file (default: ~/.mockd/mockd.pid)")
	portsCmd.Flags().BoolVarP(&portsVerbose, "verbose", "v", false, "Show extended info (engine ID, name, workspace)")
	rootCmd.AddCommand(portsCmd)
}

// printPortsFromPIDFile prints port information from the PID file.
func printPortsFromPIDFile(info *PIDFile, jsonOut, verbose bool) error {
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

	return printPorts(ports, jsonOut, verbose)
}

// printPorts prints the port information in the requested format.
func printPorts(ports []PortInfo, jsonOut, verbose bool) error {
	if jsonOut {
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
func printNotRunningPorts(jsonOut bool) error {
	if jsonOut {
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
