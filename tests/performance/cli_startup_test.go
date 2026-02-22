package performance

import (
	"fmt"
	"os/exec"
	"testing"
	"time"
)

// BenchmarkCLIStartup measures CLI binary startup time.
// The success criteria is <500ms for command response (from spec SC-001).
func BenchmarkCLIStartup(b *testing.B) {
	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "mockd_bench", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		b.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_bench").Run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("./mockd_bench", "version")
		cmd.Dir = "../.."
		if err := cmd.Run(); err != nil {
			b.Fatalf("Version command failed: %v", err)
		}
	}
}

// BenchmarkCLIHelp measures CLI help command response time.
func BenchmarkCLIHelp(b *testing.B) {
	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "mockd_bench", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		b.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_bench").Run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("./mockd_bench", "--help")
		cmd.Dir = "../.."
		_ = cmd.Run() // --help exits with 0
	}
}

// TestCLIStartupTime verifies CLI startup is under 500ms (SC-001).
func TestCLIStartupTime(t *testing.T) {
	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "mockd_bench", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_bench").Run()

	// Warm up
	for i := 0; i < 3; i++ {
		cmd := exec.Command("./mockd_bench", "version")
		cmd.Dir = "../.."
		cmd.Run()
	}

	// Measure
	iterations := 10
	var totalDuration time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		cmd := exec.Command("./mockd_bench", "version")
		cmd.Dir = "../.."
		if err := cmd.Run(); err != nil {
			t.Fatalf("Version command failed: %v", err)
		}
		totalDuration += time.Since(start)
	}

	avgDuration := totalDuration / time.Duration(iterations)
	t.Logf("Average CLI startup time: %v", avgDuration)

	if avgDuration > 500*time.Millisecond {
		t.Errorf("CLI startup time %v exceeds 500ms requirement (SC-001)", avgDuration)
	}
}

// TestCLIBinarySize checks the binary is reasonably sized.
func TestCLIBinarySize(t *testing.T) {
	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_bench", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_bench").Run()

	// Check file size
	statCmd := exec.Command("stat", "-c", "%s", "mockd_bench")
	statCmd.Dir = "../.."
	out, err := statCmd.Output()
	if err != nil {
		// Try macOS syntax
		statCmd = exec.Command("stat", "-f", "%z", "mockd_bench")
		statCmd.Dir = "../.."
		out, err = statCmd.Output()
		if err != nil {
			t.Skipf("Could not get file size: %v", err)
		}
	}

	var size int64
	if _, err := fmt.Sscanf(string(out), "%d", &size); err != nil {
		t.Fatalf("Failed to parse file size: %v", err)
	}

	sizeMB := float64(size) / (1024 * 1024)
	t.Logf("Binary size: %.2f MB", sizeMB)

	// Binary size is ~41MB due to:
	// - gRPC/protobuf support (~6000 symbols)
	// - OpenAPI validator
	// - Protocol compiler for gRPC reflection
	// - MQTT broker
	// - JSONPath parser
	// - Cobra + Charmbracelet TUI (huh, bubbletea, lipgloss) for interactive CLI
	// This is expected for a feature-rich mock server.
	// A stripped binary (-ldflags="-s -w") is ~29MB.
	if sizeMB > 45 {
		t.Errorf("Binary size %.2f MB seems excessive (expected < 45MB)", sizeMB)
	}
}
