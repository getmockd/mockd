package cli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/config"
)

// RunDoctor handles the doctor command for diagnosing setup issues.
func RunDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)

	configFile := fs.String("config", "", "Path to config file to validate")
	fs.StringVar(configFile, "c", "", "Path to config file (shorthand)")
	port := fs.Int("port", 4280, "Mock server port to check")
	fs.IntVar(port, "p", 4280, "Mock server port (shorthand)")
	adminPort := fs.Int("admin-port", 4290, "Admin API port to check")
	fs.IntVar(adminPort, "a", 4290, "Admin API port (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd doctor [flags]

Diagnose common setup issues and validate configuration.

Flags:
  -c, --config        Path to config file to validate
  -p, --port          Mock server port to check (default: 4280)
  -a, --admin-port    Admin API port to check (default: 4290)

Examples:
  # Run all checks with defaults
  mockd doctor

  # Validate a specific config file
  mockd doctor --config mocks.yaml

  # Check custom ports
  mockd doctor -p 3000 -a 3001
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	fmt.Println("mockd doctor")
	fmt.Println("============")
	fmt.Println()

	allPassed := true

	// Check 1: Port availability
	fmt.Printf("Checking port %d (mock server)... ", *port)
	if ports.IsAvailable(*port) {
		fmt.Println("available")
	} else {
		fmt.Println("IN USE")
		allPassed = false
	}

	fmt.Printf("Checking port %d (admin API)... ", *adminPort)
	if ports.IsAvailable(*adminPort) {
		fmt.Println("available")
	} else {
		fmt.Println("IN USE")
		allPassed = false
	}

	// Check 2: Config file validation
	if *configFile != "" {
		fmt.Printf("Validating config file %s... ", *configFile)
		if err := validateConfigFile(*configFile); err != nil {
			fmt.Printf("FAILED\n  %s\n", err)
			allPassed = false
		} else {
			fmt.Println("valid")
		}
	}

	// Check 3: Check if mockd is already running
	fmt.Printf("Checking for running mockd on :%d... ", *adminPort)
	if checkMockdRunning(*adminPort) {
		fmt.Println("running")
	} else {
		fmt.Println("not running")
	}

	// Check 4: Check default config locations
	fmt.Print("Checking for default config files... ")
	foundConfigs := findDefaultConfigs()
	if len(foundConfigs) > 0 {
		fmt.Printf("found %d\n", len(foundConfigs))
		for _, f := range foundConfigs {
			fmt.Printf("  - %s\n", f)
		}
	} else {
		fmt.Println("none found")
	}

	// Check 5: Check PID file
	fmt.Print("Checking PID file... ")
	pidPath := DefaultPIDPath()
	if info, err := ReadPIDFile(pidPath); err == nil {
		if info.IsRunning() {
			fmt.Printf("found (PID %d, running)\n", info.PID)
		} else {
			fmt.Printf("found (PID %d, stale)\n", info.PID)
		}
	} else {
		fmt.Println("not found")
	}

	// Check 6: Check data directory
	fmt.Print("Checking data directory... ")
	dataDir := getDataDir()
	if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
		fmt.Printf("exists (%s)\n", dataDir)
	} else {
		// Output as info rather than a failure, since it will be created on demand
		fmt.Printf("not found (will be created automatically at %s)\n", dataDir)
	}

	fmt.Println()
	if allPassed {
		fmt.Println("All checks passed!")
	} else {
		fmt.Println("Some checks failed. See above for details.")
	}

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
