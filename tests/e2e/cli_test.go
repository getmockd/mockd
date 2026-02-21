package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/rogpeppe/go-internal/testscript"
)

var (
	binaryPath string
	buildOnce  sync.Once
	buildErr   error
)

// buildBinary builds the mockd binary once for all testscript tests.
func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		binaryPath = filepath.Join(os.TempDir(), "mockd_testscript_bin")
		// Build the binary
		buildCmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/mockd")
		if out, err := buildCmd.CombinedOutput(); err != nil {
			buildErr = err
			t.Logf("Failed to build CLI: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return binaryPath
}

func TestCLIIntegration(t *testing.T) {
	// Build the mockd binary we will be invoking.
	bin := buildBinary(t)

	// Start a background server directly in Go to avoid process group hanging
	port := getFreePort(t)
	adminPort := getFreePort(t)

	cfg := &config.ServerConfiguration{
		HTTPPort:       port,
		ManagementPort: adminPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	server := engine.NewServer(cfg)
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()
	defer server.Stop()

	adminURL := "http://localhost:" + strconv.Itoa(adminPort)
	engineURL := "http://localhost:" + strconv.Itoa(port)

	// Wait for server health
	waitForServer(t, adminURL+"/health")

	// Run testscript against all .txt files in testdata/
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			binDir := filepath.Dir(bin)
			env.Setenv("PATH", binDir+string(os.PathListSeparator)+env.Getenv("PATH"))
			env.Setenv("MOCKD_BIN", bin)
			env.Setenv("ADMIN_URL", adminURL)
			env.Setenv("ENGINE_URL", engineURL)
			return nil
		},
	})
}



// TestMain acts as the main entrypoint. Testscript requires its own Main wrapper.
func TestMain(m *testing.M) {
	// Clean up the binary after all tests finish
	defer func() {
		if binaryPath != "" {
			os.Remove(binaryPath)
		}
	}()

	os.Exit(testscript.RunMain(m, map[string]func() int{
		// We could wire standard Go commands here if we wanted,
		// but we are relying on compiling the binary and adding it to PATH.
	}))
}
