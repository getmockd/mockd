package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/spf13/cobra"
)

// makeUpdateCmd creates a fresh cobra.Command with the same flags as updateCmd,
// bound to the package-level globals. This allows tests to call
// cmd.Flags().Set("status", "201") so that cmd.Flags().Changed("status")
// returns true — matching how runUpdate actually works.
//
// IMPORTANT: Because StringVarP writes the default value to the global at
// registration time, callers must set globals AFTER calling makeUpdateCmd().
func makeUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "update <id>",
		Args: cobra.ExactArgs(1),
		RunE: runUpdate,
	}
	cmd.Flags().StringVarP(&updateBody, "body", "b", "", "Response body")
	cmd.Flags().StringVar(&updateBodyFile, "body-file", "", "Read response body from file")
	cmd.Flags().IntVarP(&updateStatus, "status", "s", 0, "Response status code")
	cmd.Flags().VarP(&updateHeaders, "header", "H", "Response header (key:value), repeatable")
	cmd.Flags().IntVar(&updateDelay, "delay", 0, "Response delay in milliseconds")
	cmd.Flags().StringVar(&updateTable, "table", "", "Bind to a stateful resource table")
	cmd.Flags().StringVar(&updateBind, "bind", "", "Stateful action")
	cmd.Flags().StringVar(&updateOperation, "operation", "", "Custom operation name")
	cmd.Flags().StringVarP(&updateName, "name", "n", "", "Mock display name")
	cmd.Flags().StringVar(&updateEnabled, "enabled", "", "Enable or disable")
	return cmd
}

// saveUpdateGlobals saves the current state of all update-related globals and
// returns a restore function suitable for defer.
func saveUpdateGlobals() func() {
	saved := struct {
		body, bodyFile, table, bind, operation, name, enabled string
		status, delay                                         int
		headers                                               flags.StringSlice
		adminURLVal                                           string
		jsonOutputVal                                         bool
	}{
		body:          updateBody,
		bodyFile:      updateBodyFile,
		status:        updateStatus,
		delay:         updateDelay,
		headers:       append(flags.StringSlice{}, updateHeaders...),
		table:         updateTable,
		bind:          updateBind,
		operation:     updateOperation,
		name:          updateName,
		enabled:       updateEnabled,
		adminURLVal:   adminURL,
		jsonOutputVal: jsonOutput,
	}
	return func() {
		updateBody = saved.body
		updateBodyFile = saved.bodyFile
		updateStatus = saved.status
		updateDelay = saved.delay
		updateHeaders = saved.headers
		updateTable = saved.table
		updateBind = saved.bind
		updateOperation = saved.operation
		updateName = saved.name
		updateEnabled = saved.enabled
		adminURL = saved.adminURLVal
		jsonOutput = saved.jsonOutputVal
	}
}

// patchServer creates an httptest.Server that records the PATCH request body and
// responds with a valid mock envelope. The caller can inspect receivedPatch
// after runUpdate returns.
func patchServer(t *testing.T, receivedPatch *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("unexpected method %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, receivedPatch)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mock": map[string]interface{}{
				"id":   strings.TrimPrefix(r.URL.Path, "/mocks/"),
				"type": "http",
				"http": map[string]interface{}{
					"matcher":  map[string]interface{}{"method": "GET", "path": "/api/test"},
					"response": map[string]interface{}{"statusCode": 200},
				},
			},
		})
	}))
}

// captureStdout runs fn while capturing stdout, returning the output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// =============================================================================
// Validation / error tests (no server needed)
// =============================================================================

func TestRunUpdate_NoFlags(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	// All globals are now zeroed by makeUpdateCmd defaults

	err := cmd.RunE(cmd, []string{"http_abc123"})
	if err == nil {
		t.Fatal("expected error when no flags specified")
	}
	if !strings.Contains(err.Error(), "no update flags specified") {
		t.Errorf("error = %q, want it to contain 'no update flags specified'", err.Error())
	}
}

func TestRunUpdate_InvalidEnabled(t *testing.T) {
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	// Set global AFTER makeUpdateCmd (which writes defaults to globals)
	updateEnabled = "yes"

	err := cmd.RunE(cmd, []string{"http_abc123"})
	if err == nil {
		t.Fatal("expected error for invalid --enabled value")
	}
	if !strings.Contains(err.Error(), "--enabled must be 'true' or 'false'") {
		t.Errorf("error = %q, want it to mention --enabled constraint", err.Error())
	}
}

func TestRunUpdate_TableWithoutBind(t *testing.T) {
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	updateTable = "users"

	err := cmd.RunE(cmd, []string{"http_abc123"})
	if err == nil {
		t.Fatal("expected error when --table used without --bind")
	}
	if !strings.Contains(err.Error(), "--table and --bind must be used together") {
		t.Errorf("error = %q, want '--table and --bind must be used together'", err.Error())
	}
}

func TestRunUpdate_BindWithoutTable(t *testing.T) {
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	updateBind = "list"

	err := cmd.RunE(cmd, []string{"http_abc123"})
	if err == nil {
		t.Fatal("expected error when --bind used without --table")
	}
	if !strings.Contains(err.Error(), "--table and --bind must be used together") {
		t.Errorf("error = %q, want '--table and --bind must be used together'", err.Error())
	}
}

func TestRunUpdate_InvalidBindValue(t *testing.T) {
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	updateTable = "users"
	updateBind = "upsert"

	err := cmd.RunE(cmd, []string{"http_abc123"})
	if err == nil {
		t.Fatal("expected error for invalid --bind value")
	}
	if !strings.Contains(err.Error(), "invalid --bind value") {
		t.Errorf("error = %q, want 'invalid --bind value'", err.Error())
	}
}

func TestRunUpdate_InvalidHeaderFormat(t *testing.T) {
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	updateHeaders = flags.StringSlice{"bad-header-no-colon"}

	err := cmd.RunE(cmd, []string{"http_abc123"})
	if err == nil {
		t.Fatal("expected error for invalid header format")
	}
	if !strings.Contains(err.Error(), "invalid header format") {
		t.Errorf("error = %q, want 'invalid header format'", err.Error())
	}
}

// =============================================================================
// Server-backed tests
// =============================================================================

func TestRunUpdate_StatusCode(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	_ = cmd.Flags().Set("status", "201")

	out := captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"http_abc123"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	// Verify patch body
	httpPatch, ok := received["http"].(map[string]interface{})
	if !ok {
		t.Fatal("patch should contain 'http' key")
	}
	resp, ok := httpPatch["response"].(map[string]interface{})
	if !ok {
		t.Fatal("http patch should contain 'response' key")
	}
	if resp["statusCode"] != float64(201) {
		t.Errorf("statusCode = %v, want 201", resp["statusCode"])
	}

	// Verify output
	if !strings.Contains(out, "Updated mock: http_abc123") {
		t.Errorf("output = %q, want it to contain 'Updated mock: http_abc123'", out)
	}
}

func TestRunUpdate_Body(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateBody = `{"updated": true}`

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch := received["http"].(map[string]interface{})
	resp := httpPatch["response"].(map[string]interface{})
	if resp["body"] != `{"updated": true}` {
		t.Errorf("body = %v, want {\"updated\": true}", resp["body"])
	}
}

func TestRunUpdate_BodyFile(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	tmpDir := t.TempDir()
	bodyPath := filepath.Join(tmpDir, "body.json")
	if err := os.WriteFile(bodyPath, []byte(`{"from":"file"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateBodyFile = bodyPath

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch := received["http"].(map[string]interface{})
	resp := httpPatch["response"].(map[string]interface{})
	if resp["body"] != `{"from":"file"}` {
		t.Errorf("body = %v, want {\"from\":\"file\"}", resp["body"])
	}
}

func TestRunUpdate_BodyFileNotFound(t *testing.T) {
	defer saveUpdateGlobals()()
	cmd := makeUpdateCmd()
	updateBodyFile = "/nonexistent/file.json"

	err := cmd.RunE(cmd, []string{"mock1"})
	if err == nil {
		t.Fatal("expected error for missing body file")
	}
	if !strings.Contains(err.Error(), "failed to read body file") {
		t.Errorf("error = %q, want 'failed to read body file'", err.Error())
	}
}

func TestRunUpdate_Delay(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	_ = cmd.Flags().Set("delay", "500")

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch := received["http"].(map[string]interface{})
	resp := httpPatch["response"].(map[string]interface{})
	if resp["delayMs"] != float64(500) {
		t.Errorf("delayMs = %v, want 500", resp["delayMs"])
	}
}

func TestRunUpdate_Headers(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateHeaders = flags.StringSlice{"Content-Type:application/xml", "X-Custom:myval"}

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch := received["http"].(map[string]interface{})
	resp := httpPatch["response"].(map[string]interface{})
	headers, ok := resp["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected headers in response patch")
	}
	if headers["Content-Type"] != "application/xml" {
		t.Errorf("Content-Type = %v, want application/xml", headers["Content-Type"])
	}
	if headers["X-Custom"] != "myval" {
		t.Errorf("X-Custom = %v, want myval", headers["X-Custom"])
	}
}

func TestRunUpdate_Name(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateName = "My Updated Mock"

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	if received["name"] != "My Updated Mock" {
		t.Errorf("name = %v, want 'My Updated Mock'", received["name"])
	}
	// name-only patch should NOT have 'http' key
	if _, ok := received["http"]; ok {
		t.Error("name-only update should not include http patch")
	}
}

func TestRunUpdate_EnabledTrue(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateEnabled = "true"

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	if received["enabled"] != true {
		t.Errorf("enabled = %v, want true", received["enabled"])
	}
}

func TestRunUpdate_EnabledFalse(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateEnabled = "false"

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	if received["enabled"] != false {
		t.Errorf("enabled = %v, want false", received["enabled"])
	}
}

func TestRunUpdate_TableAndBind(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateTable = "users"
	updateBind = "list"

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch, ok := received["http"].(map[string]interface{})
	if !ok {
		t.Fatal("patch should contain 'http' key")
	}
	binding, ok := httpPatch["statefulBinding"].(map[string]interface{})
	if !ok {
		t.Fatal("http patch should contain 'statefulBinding'")
	}
	if binding["table"] != "users" {
		t.Errorf("table = %v, want 'users'", binding["table"])
	}
	if binding["action"] != "list" {
		t.Errorf("action = %v, want 'list'", binding["action"])
	}
	// Conflicting types should be cleared
	if httpPatch["response"] != nil {
		t.Error("response should be nil when binding to table")
	}
	if httpPatch["sse"] != nil {
		t.Error("sse should be nil when binding to table")
	}
}

func TestRunUpdate_TableBindCustomWithOperation(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateTable = "orders"
	updateBind = "custom"
	updateOperation = "CancelOrder"

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch := received["http"].(map[string]interface{})
	binding := httpPatch["statefulBinding"].(map[string]interface{})
	if binding["action"] != "custom" {
		t.Errorf("action = %v, want 'custom'", binding["action"])
	}
	if binding["operation"] != "CancelOrder" {
		t.Errorf("operation = %v, want 'CancelOrder'", binding["operation"])
	}
}

func TestRunUpdate_AllBindActions(t *testing.T) {
	validActions := []string{"list", "get", "create", "update", "delete", "custom", "patch"}
	for _, action := range validActions {
		t.Run(action, func(t *testing.T) {
			defer saveUpdateGlobals()()

			var received map[string]interface{}
			ts := patchServer(t, &received)
			defer ts.Close()

			cmd := makeUpdateCmd()
			adminURL = ts.URL
			updateTable = "items"
			updateBind = action

			_ = captureStdout(t, func() {
				err := cmd.RunE(cmd, []string{"mock1"})
				if err != nil {
					t.Fatalf("runUpdate(%s) error: %v", action, err)
				}
			})

			httpPatch := received["http"].(map[string]interface{})
			binding := httpPatch["statefulBinding"].(map[string]interface{})
			if binding["action"] != action {
				t.Errorf("action = %v, want %q", binding["action"], action)
			}
		})
	}
}

func TestRunUpdate_MultipleResponseFields(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateBody = `{"ok":true}`
	updateHeaders = flags.StringSlice{"X-Req-ID:abc"}
	_ = cmd.Flags().Set("status", "202")
	_ = cmd.Flags().Set("delay", "100")

	_ = captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"mock1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	httpPatch := received["http"].(map[string]interface{})
	resp := httpPatch["response"].(map[string]interface{})

	if resp["body"] != `{"ok":true}` {
		t.Errorf("body = %v, want {\"ok\":true}", resp["body"])
	}
	if resp["statusCode"] != float64(202) {
		t.Errorf("statusCode = %v, want 202", resp["statusCode"])
	}
	if resp["delayMs"] != float64(100) {
		t.Errorf("delayMs = %v, want 100", resp["delayMs"])
	}
	headers := resp["headers"].(map[string]interface{})
	if headers["X-Req-ID"] != "abc" {
		t.Errorf("X-Req-ID = %v, want 'abc'", headers["X-Req-ID"])
	}
}

func TestRunUpdate_JSONOutput(t *testing.T) {
	defer saveUpdateGlobals()()

	var received map[string]interface{}
	ts := patchServer(t, &received)
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	jsonOutput = true
	updateName = "json-test"

	out := captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"http_abc123"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput: %s", err, out)
	}
	if result["id"] != "http_abc123" {
		t.Errorf("JSON id = %v, want http_abc123", result["id"])
	}
	if result["action"] != "updated" {
		t.Errorf("JSON action = %v, want 'updated'", result["action"])
	}
}

func TestRunUpdate_ServerError(t *testing.T) {
	defer saveUpdateGlobals()()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "mock not found: nonexistent",
		})
	}))
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateName = "trigger-patch"

	err := cmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestRunUpdate_StatefulBindingOutput(t *testing.T) {
	defer saveUpdateGlobals()()

	// Server returns a mock with statefulBinding in the response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mock": map[string]interface{}{
				"id":   "http_stateful1",
				"type": "http",
				"http": map[string]interface{}{
					"matcher": map[string]interface{}{"method": "GET", "path": "/api/users"},
					"statefulBinding": map[string]interface{}{
						"table":  "users",
						"action": "list",
					},
				},
			},
		})
	}))
	defer ts.Close()

	cmd := makeUpdateCmd()
	adminURL = ts.URL
	updateTable = "users"
	updateBind = "list"

	out := captureStdout(t, func() {
		err := cmd.RunE(cmd, []string{"http_stateful1"})
		if err != nil {
			t.Fatalf("runUpdate() error: %v", err)
		}
	})

	if !strings.Contains(out, "Updated mock: http_stateful1") {
		t.Errorf("output should contain 'Updated mock: http_stateful1', got: %s", out)
	}
	if !strings.Contains(out, "Table: users") {
		t.Errorf("output should contain 'Table: users', got: %s", out)
	}
	if !strings.Contains(out, "Bind:  list") {
		t.Errorf("output should contain 'Bind:  list', got: %s", out)
	}
}
