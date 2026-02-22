package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/spf13/cobra"
)

var (
	doctorConfigFile string
	doctorPort       int
	doctorAdminPort  int
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common setup issues and validate configuration",
	Long:  `Diagnose common setup issues and validate configuration.`,
	Example: `  # Run all checks with defaults
  mockd doctor

  # Validate a specific config file
  mockd doctor --config mocks.yaml

  # Check custom ports
  mockd doctor -p 3000 -a 3001`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().StringVar(&doctorConfigFile, "config", "", "Path to config file to validate")
	doctorCmd.Flags().IntVarP(&doctorPort, "port", "p", 4280, "Mock server port to check")
	doctorCmd.Flags().IntVarP(&doctorAdminPort, "admin-port", "a", 4290, "Admin API port to check")
}

// doctorCheck holds the result of a single doctor check.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok", "fail", "info"
	Detail string `json:"detail"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	configFile := doctorConfigFile
	port := doctorPort
	adminPort := doctorAdminPort

	allPassed := true
	var checks []doctorCheck

	// Check 1: Mock server port availability
	if ports.IsAvailable(port) {
		checks = append(checks, doctorCheck{Name: fmt.Sprintf("port_%d_mock_server", port), Status: "ok", Detail: "available"})
	} else {
		checks = append(checks, doctorCheck{Name: fmt.Sprintf("port_%d_mock_server", port), Status: "fail", Detail: "in use"})
		allPassed = false
	}

	// Check 2: Admin API port availability
	if ports.IsAvailable(adminPort) {
		checks = append(checks, doctorCheck{Name: fmt.Sprintf("port_%d_admin_api", adminPort), Status: "ok", Detail: "available"})
	} else {
		checks = append(checks, doctorCheck{Name: fmt.Sprintf("port_%d_admin_api", adminPort), Status: "fail", Detail: "in use"})
		allPassed = false
	}

	// Check 3: Config file validation
	if configFile != "" {
		if err := validateConfigFile(configFile); err != nil {
			checks = append(checks, doctorCheck{Name: "config_file", Status: "fail", Detail: err.Error()})
			allPassed = false
		} else {
			checks = append(checks, doctorCheck{Name: "config_file", Status: "ok", Detail: configFile})
		}
	}

	// Check 4: Check if mockd is already running
	if checkMockdRunning(adminPort) {
		checks = append(checks, doctorCheck{Name: "mockd_running", Status: "ok", Detail: fmt.Sprintf("responding on :%d", adminPort)})
	} else {
		checks = append(checks, doctorCheck{Name: "mockd_running", Status: "info", Detail: fmt.Sprintf("not running on :%d", adminPort)})
	}

	// Check 5: Check default config locations
	foundConfigs := findDefaultConfigs()
	if len(foundConfigs) > 0 {
		checks = append(checks, doctorCheck{Name: "default_configs", Status: "ok", Detail: fmt.Sprintf("found %d: %s", len(foundConfigs), strings.Join(foundConfigs, ", "))})
	} else {
		checks = append(checks, doctorCheck{Name: "default_configs", Status: "info", Detail: "none found"})
	}

	// Check 6: Check PID file
	pidPath := DefaultPIDPath()
	if info, err := ReadPIDFile(pidPath); err == nil {
		if info.IsRunning() {
			checks = append(checks, doctorCheck{Name: "pid_file", Status: "ok", Detail: fmt.Sprintf("PID %d, running", info.PID)})
		} else {
			checks = append(checks, doctorCheck{Name: "pid_file", Status: "info", Detail: fmt.Sprintf("PID %d, stale", info.PID)})
		}
	} else {
		checks = append(checks, doctorCheck{Name: "pid_file", Status: "info", Detail: "not found"})
	}

	// Check 7: Check data directory
	dataDir := getDataDir()
	if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
		checks = append(checks, doctorCheck{Name: "data_directory", Status: "ok", Detail: dataDir})
	} else {
		checks = append(checks, doctorCheck{Name: "data_directory", Status: "info", Detail: fmt.Sprintf("not found (will be created at %s)", dataDir)})
	}

	printResult(map[string]any{"checks": checks, "allPassed": allPassed}, func() {
		fmt.Println("mockd doctor")
		fmt.Println("============")
		fmt.Println()
		for _, c := range checks {
			switch c.Status {
			case "ok":
				fmt.Printf("  ✓ %s: %s\n", c.Name, c.Detail)
			case "fail":
				fmt.Printf("  ✗ %s: %s\n", c.Name, c.Detail)
			default:
				fmt.Printf("  • %s: %s\n", c.Name, c.Detail)
			}
		}
		fmt.Println()
		if allPassed {
			fmt.Println("All checks passed!")
		} else {
			fmt.Println("Some checks failed. See above for details.")
		}
	})

	return nil
}

// validateConfigFile loads and validates a config file.
func validateConfigFile(path string) error {
	_, err := config.LoadFromFile(path)
	return err
}

// checkMockdRunning checks if mockd admin API is responding.
func checkMockdRunning(adminPort int) bool {
	client := NewAdminClientWithAuth(
		fmt.Sprintf("http://localhost:%d", adminPort),
		WithTimeout(2*time.Second),
	)
	return client.Health() == nil
}

// findDefaultConfigs looks for config files in common locations.
func findDefaultConfigs() []string {
	var found []string
	candidates := []string{
		"mockd.yaml",
		"mockd.yml",
		"mockd.json",
		".mockd.yaml",
		".mockd.yml",
		".mockd.json",
	}

	for _, name := range candidates {
		if _, err := os.Stat(name); err == nil {
			found = append(found, name)
		}
	}

	return found
}

// getDataDir returns the default data directory path.
func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.mockd/data"
}
