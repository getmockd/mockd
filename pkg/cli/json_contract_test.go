package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
)

// ─── Test infrastructure ────────────────────────────────────────────────────

// captureJSONOutput runs fn with jsonOutput=true and captures stdout.
// Returns the raw bytes written to stdout and any error from fn.
func captureJSONOutput(t *testing.T, fn func() error) ([]byte, error) {
	t.Helper()

	// Enable --json mode
	oldJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = oldJSON })

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fnErr := fn()

	w.Close()
	os.Stdout = oldStdout

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}

	return data, fnErr
}

// assertValidJSON asserts that data is valid JSON and returns the parsed map.
// Fails the test if data is empty, not valid JSON, or contains non-JSON prose.
func assertValidJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()

	if len(data) == 0 {
		t.Fatal("stdout was empty; expected JSON output")
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		// Check if it's a JSON array instead
		var arr []any
		if arrErr := json.Unmarshal(data, &arr); arrErr != nil {
			t.Fatalf("stdout is not valid JSON:\n---\n%s\n---\nerror: %v", string(data), err)
		}
		// Wrap array in a map so callers can still use assertHasKeys
		return map[string]any{"_array": arr}
	}

	return result
}

// assertHasKeys asserts that the JSON object contains all expected top-level keys.
func assertHasKeys(t *testing.T, obj map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			t.Errorf("JSON output missing expected key %q; got keys: %v", key, mapKeys(obj))
		}
	}
}

// assertNoProseOnStdout verifies that captured stdout contains only JSON
// (no human-readable prose mixed in). It checks that the entire output
// is parseable as a single JSON value.
func assertNoProseOnStdout(t *testing.T, data []byte) {
	t.Helper()
	if len(data) == 0 {
		return // Empty is fine (some error paths may not write)
	}
	// Try to parse as JSON. If it fails, there's prose mixed in.
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Errorf("stdout contains non-JSON content (prose leak):\n---\n%s\n---", string(data))
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ─── printResult / printList contract ───────────────────────────────────────

func TestPrintResult_JSONMode(t *testing.T) {
	data, _ := captureJSONOutput(t, func() error {
		printResult(map[string]any{"status": "ok", "count": 42}, nil)
		return nil
	})

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "status", "count")

	if obj["status"] != "ok" {
		t.Errorf("status = %v, want ok", obj["status"])
	}
}

func TestPrintResult_TextMode(t *testing.T) {
	// Ensure textFn is called in text mode, NOT json
	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	called := false
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	printResult(map[string]any{"x": 1}, func() { called = true })

	w.Close()
	os.Stdout = oldStdout

	if !called {
		t.Error("textFn should be called in text mode")
	}
}

func TestPrintList_JSONMode(t *testing.T) {
	items := []map[string]any{
		{"id": "a", "name": "first"},
		{"id": "b", "name": "second"},
	}

	data, _ := captureJSONOutput(t, func() error {
		printList(items, nil)
		return nil
	})

	assertNoProseOnStdout(t, data)

	// Should be a JSON array
	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("printList should produce a JSON array: %v", err)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 items, got %d", len(arr))
	}
}

// ─── version command ────────────────────────────────────────────────────────

func TestVersion_JSONContract(t *testing.T) {
	data, err := captureJSONOutput(t, func() error {
		rootCmd.SetArgs([]string{"version", "--json"})
		return rootCmd.Execute()
	})

	if err != nil {
		t.Fatalf("version --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "version", "commit", "date", "go", "os", "arch")
}

// ─── validate command ───────────────────────────────────────────────────────

func TestValidate_JSONContract_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")
	if err := os.WriteFile(configPath, []byte(`
version: "1.0"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    admin: local
    httpPort: 4280
`), 0644); err != nil {
		t.Fatal(err)
	}

	oldConfigFiles := validateConfigFiles
	oldVerbose := validateVerbose
	oldShowResolved := validateShowResolved
	t.Cleanup(func() {
		validateConfigFiles = oldConfigFiles
		validateVerbose = oldVerbose
		validateShowResolved = oldShowResolved
	})
	validateConfigFiles = []string{configPath}
	validateVerbose = false
	validateShowResolved = false

	data, err := captureJSONOutput(t, func() error {
		return validateCmd.RunE(validateCmd, []string{})
	})

	if err != nil {
		t.Fatalf("validate --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "valid", "errors", "errorCount", "summary")

	if obj["valid"] != true {
		t.Errorf("valid = %v, want true", obj["valid"])
	}
}

func TestValidate_JSONContract_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")
	if err := os.WriteFile(configPath, []byte(`
version: "1.0"
admins:
  - port: 0
`), 0644); err != nil {
		t.Fatal(err)
	}

	oldConfigFiles := validateConfigFiles
	oldVerbose := validateVerbose
	oldShowResolved := validateShowResolved
	t.Cleanup(func() {
		validateConfigFiles = oldConfigFiles
		validateVerbose = oldVerbose
		validateShowResolved = oldShowResolved
	})
	validateConfigFiles = []string{configPath}
	validateVerbose = false
	validateShowResolved = false

	data, _ := captureJSONOutput(t, func() error {
		return validateCmd.RunE(validateCmd, []string{})
	})
	// Error is expected (validation fails), but JSON should still be on stdout

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "valid", "errors", "errorCount", "summary")

	if obj["valid"] != false {
		t.Errorf("valid = %v, want false for invalid config", obj["valid"])
	}
	if obj["errorCount"].(float64) == 0 {
		t.Error("errorCount should be > 0 for invalid config")
	}
}

// ─── doctor command ─────────────────────────────────────────────────────────

func TestDoctor_JSONContract(t *testing.T) {
	oldConfigFile := doctorConfigFile
	oldPort := doctorPort
	oldAdminPort := doctorAdminPort
	t.Cleanup(func() {
		doctorConfigFile = oldConfigFile
		doctorPort = oldPort
		doctorAdminPort = oldAdminPort
	})
	// Use high ports unlikely to be in use
	doctorConfigFile = ""
	doctorPort = 59990
	doctorAdminPort = 59991

	data, err := captureJSONOutput(t, func() error {
		return runDoctor(doctorCmd, []string{})
	})

	if err != nil {
		t.Fatalf("doctor --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "checks", "allPassed")

	// checks should be an array
	checks, ok := obj["checks"].([]any)
	if !ok {
		t.Fatal("checks should be an array")
	}
	if len(checks) == 0 {
		t.Error("checks should not be empty")
	}

	// Each check should have name, status, detail
	for i, c := range checks {
		check, ok := c.(map[string]any)
		if !ok {
			t.Errorf("checks[%d] should be an object", i)
			continue
		}
		for _, key := range []string{"name", "status", "detail"} {
			if _, ok := check[key]; !ok {
				t.Errorf("checks[%d] missing key %q", i, key)
			}
		}
		// status must be ok, fail, or info
		status, _ := check["status"].(string)
		if status != "ok" && status != "fail" && status != "info" {
			t.Errorf("checks[%d].status = %q, want ok|fail|info", i, status)
		}
	}
}

// ─── ps command ─────────────────────────────────────────────────────────────

func TestPs_JSONContract_NoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	oldPsPidFile := psPidFile
	t.Cleanup(func() { psPidFile = oldPsPidFile })
	psPidFile = pidPath

	data, err := captureJSONOutput(t, func() error {
		return psCmd.RunE(psCmd, []string{})
	})

	if err != nil {
		t.Fatalf("ps --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "running", "services")

	if obj["running"] != false {
		t.Errorf("running = %v, want false when no PID file", obj["running"])
	}
}

func TestPs_JSONContract_WithPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "mockd.pid")

	// Write a PID file with current process PID so it shows as running
	pidInfo := &config.PIDFile{
		PID:       os.Getpid(),
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: os.Getpid()},
		},
	}
	data, err := json.MarshalIndent(pidInfo, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	oldPsPidFile := psPidFile
	t.Cleanup(func() { psPidFile = oldPsPidFile })
	psPidFile = pidPath

	output, runErr := captureJSONOutput(t, func() error {
		return psCmd.RunE(psCmd, []string{})
	})

	if runErr != nil {
		t.Fatalf("ps --json returned error: %v", runErr)
	}

	obj := assertValidJSON(t, output)
	assertNoProseOnStdout(t, output)
	assertHasKeys(t, obj, "running", "pid", "startedAt", "config", "services")

	if obj["running"] != true {
		t.Errorf("running = %v, want true when PID is current process", obj["running"])
	}
}

// ─── down command ───────────────────────────────────────────────────────────

func TestDown_JSONContract_NoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	oldPidFile := downPidFile
	t.Cleanup(func() { downPidFile = oldPidFile })
	downPidFile = pidPath
	downTimeout = 5 * time.Second

	data, err := captureJSONOutput(t, func() error {
		return downCmd.RunE(downCmd, []string{})
	})

	if err != nil {
		t.Fatalf("down --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "stopped", "reason")

	if obj["stopped"] != false {
		t.Errorf("stopped = %v, want false when no PID file", obj["stopped"])
	}
}

// ─── context commands ───────────────────────────────────────────────────────

func TestContextShow_JSONContract(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	cfg := cliconfig.NewDefaultContextConfig()
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := captureJSONOutput(t, func() error {
		return runContextShow()
	})

	if err != nil {
		t.Fatalf("context show --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "name", "context")
}

func TestContextShow_JSONContract_NoContext(t *testing.T) {
	// Create config with no contexts
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	cfg := &cliconfig.ContextConfig{
		Contexts: map[string]*cliconfig.Context{},
	}
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := captureJSONOutput(t, func() error {
		return runContextShow()
	})

	if err != nil {
		t.Fatalf("context show --json with no context returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "context")
}

func TestContextList_JSONContract(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	cfg := cliconfig.NewDefaultContextConfig()
	cfg.Contexts["staging"] = &cliconfig.Context{
		AdminURL:    "http://staging:4290",
		Description: "Staging server",
	}
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := captureJSONOutput(t, func() error {
		return contextListCmd.RunE(contextListCmd, []string{})
	})

	if err != nil {
		t.Fatalf("context list --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "currentContext", "contexts")

	// contexts should be a map
	contexts, ok := obj["contexts"].(map[string]any)
	if !ok {
		t.Fatal("contexts should be a map")
	}
	if len(contexts) < 1 {
		t.Error("contexts should have at least one entry")
	}

	// Each context should NOT expose authToken (security)
	for name, v := range contexts {
		ctx, ok := v.(map[string]any)
		if !ok {
			t.Errorf("contexts[%q] should be an object", name)
			continue
		}
		if _, hasToken := ctx["authToken"]; hasToken {
			t.Errorf("contexts[%q] exposes authToken — should be sanitized", name)
		}
		assertHasKeys(t, ctx, "adminUrl")
	}
}

func TestContextUse_JSONContract(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	cfg := cliconfig.NewDefaultContextConfig()
	cfg.Contexts["staging"] = &cliconfig.Context{
		AdminURL:    "http://staging:4290",
		Description: "Staging server",
	}
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := captureJSONOutput(t, func() error {
		return contextUseCmd.RunE(contextUseCmd, []string{"staging"})
	})

	if err != nil {
		t.Fatalf("context use --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "name", "switched", "context")

	if obj["switched"] != true {
		t.Errorf("switched = %v, want true", obj["switched"])
	}
	if obj["name"] != "staging" {
		t.Errorf("name = %v, want staging", obj["name"])
	}
}

// ─── status contract ────────────────────────────────────────────────────────

func TestStatus_JSONContract_NotRunning(t *testing.T) {
	// Use a PID file path that doesn't exist
	oldPidFile := statusPidFile
	statusPidFile = filepath.Join(t.TempDir(), "nonexistent.pid")
	defer func() { statusPidFile = oldPidFile }()

	// Use ports that are definitely not listening
	oldAdminPort := statusAdminPort
	oldPort := statusPort
	statusAdminPort = 19999
	statusPort = 19998
	defer func() { statusAdminPort = oldAdminPort; statusPort = oldPort }()

	data, err := captureJSONOutput(t, func() error {
		return statusCmd.RunE(statusCmd, []string{})
	})

	if err != nil {
		t.Fatalf("status --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "running", "components")

	if obj["running"] != false {
		t.Errorf("running = %v, want false", obj["running"])
	}

	// Verify components structure
	components, ok := obj["components"].(map[string]any)
	if !ok {
		t.Fatal("components should be an object")
	}
	for _, name := range []string{"admin", "engine"} {
		comp, ok := components[name].(map[string]any)
		if !ok {
			t.Fatalf("components.%s should be an object", name)
		}
		if comp["status"] != "stopped" {
			t.Errorf("components.%s.status = %v, want stopped", name, comp["status"])
		}
	}
}

func TestStatus_JSONContract_WithPIDFile(t *testing.T) {
	// Create a fake PID file for a non-running process (PID 1 is init, won't match)
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "mockd.pid")
	pidFile := &PIDFile{
		PID:       99999999, // Definitely not running
		Version:   "0.3.2",
		Commit:    "abc1234",
		StartTime: time.Now().Add(-10 * time.Minute),
		Components: ComponentsInfo{
			Admin:  ComponentStatus{Enabled: true, Port: 19999},
			Engine: ComponentStatus{Enabled: true, Port: 19998},
		},
	}
	if err := WritePIDFile(pidPath, pidFile); err != nil {
		t.Fatal(err)
	}

	// Since PID 99999999 won't be running, this falls through to port detection
	// which also won't find anything → not running
	oldPidFile := statusPidFile
	statusPidFile = pidPath
	defer func() { statusPidFile = oldPidFile }()

	oldAdminPort := statusAdminPort
	oldPort := statusPort
	statusAdminPort = 19999
	statusPort = 19998
	defer func() { statusAdminPort = oldAdminPort; statusPort = oldPort }()

	data, err := captureJSONOutput(t, func() error {
		return statusCmd.RunE(statusCmd, []string{})
	})

	if err != nil {
		t.Fatalf("status --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "running", "components")

	// Process won't be running, so falls back to not-running path
	if obj["running"] != false {
		t.Errorf("running = %v, want false (stale PID)", obj["running"])
	}
}

// ─── ports contract ─────────────────────────────────────────────────────────

func TestPorts_JSONContract_NotRunning(t *testing.T) {
	// Use admin URL that won't connect
	oldAdminURL := adminURL
	adminURL = "http://localhost:19999"
	defer func() { adminURL = oldAdminURL }()

	oldPidFile := portsPIDFile
	portsPIDFile = filepath.Join(t.TempDir(), "nonexistent.pid")
	defer func() { portsPIDFile = oldPidFile }()

	data, err := captureJSONOutput(t, func() error {
		return portsCmd.RunE(portsCmd, []string{})
	})

	if err != nil {
		t.Fatalf("ports --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "ports", "running")

	if obj["running"] != false {
		t.Errorf("running = %v, want false", obj["running"])
	}

	ports, ok := obj["ports"].([]any)
	if !ok {
		t.Fatal("ports should be an array")
	}
	if len(ports) != 0 {
		t.Errorf("expected 0 ports, got %d", len(ports))
	}
}

func TestPorts_JSONContract_FromPIDFile(t *testing.T) {
	// printPortsFromPIDFile builds ports from PID file info
	info := &PIDFile{
		PID: 12345,
		Components: ComponentsInfo{
			Engine: ComponentStatus{Enabled: true, Port: 4280, HTTPSPort: 5280},
			Admin:  ComponentStatus{Enabled: true, Port: 4290},
		},
	}

	data, err := captureJSONOutput(t, func() error {
		return printPortsFromPIDFile(info, false)
	})

	if err != nil {
		t.Fatalf("printPortsFromPIDFile --json returned error: %v", err)
	}

	obj := assertValidJSON(t, data)
	assertNoProseOnStdout(t, data)
	assertHasKeys(t, obj, "ports", "running")

	if obj["running"] != true {
		t.Errorf("running = %v, want true", obj["running"])
	}

	ports, ok := obj["ports"].([]any)
	if !ok {
		t.Fatal("ports should be an array")
	}
	if len(ports) != 3 {
		t.Errorf("expected 3 ports (HTTP, HTTPS, Admin), got %d", len(ports))
	}

	// Verify port objects have required keys
	for i, p := range ports {
		pm, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("ports[%d] should be an object", i)
		}
		for _, key := range []string{"port", "protocol", "component", "status"} {
			if _, exists := pm[key]; !exists {
				t.Errorf("ports[%d] missing key %q", i, key)
			}
		}
	}
}
