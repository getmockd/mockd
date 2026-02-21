package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPrintPorts_JSON(t *testing.T) {
	ports := []PortInfo{
		{Port: 4280, Protocol: "HTTP", Component: "Mock Engine", Status: "running"},
		{Port: 4290, Protocol: "HTTP", Component: "Admin API", Status: "running"},
		{Port: 5280, Protocol: "HTTPS", Component: "Mock Engine", Status: "running", TLS: true},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printPorts(ports, true, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printPorts returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Parse JSON
	var result PortsOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if !result.Running {
		t.Error("expected Running to be true")
	}

	if len(result.Ports) != 3 {
		t.Errorf("expected 3 ports, got %d", len(result.Ports))
	}
}

func TestPrintPorts_Table(t *testing.T) {
	ports := []PortInfo{
		{Port: 4290, Protocol: "HTTP", Component: "Admin API", Status: "running"},
		{Port: 4280, Protocol: "HTTP", Component: "Mock Engine", Status: "running"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printPorts(ports, false, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printPorts returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that output contains expected content
	if !strings.Contains(output, "PORT") {
		t.Error("expected output to contain header 'PORT'")
	}
	if !strings.Contains(output, "4280") {
		t.Error("expected output to contain port 4280")
	}
	if !strings.Contains(output, "4290") {
		t.Error("expected output to contain port 4290")
	}
	if !strings.Contains(output, "Mock Engine") {
		t.Error("expected output to contain 'Mock Engine'")
	}
	if !strings.Contains(output, "Admin API") {
		t.Error("expected output to contain 'Admin API'")
	}

	// Check that ports are sorted (4280 should appear before 4290)
	idx4280 := strings.Index(output, "4280")
	idx4290 := strings.Index(output, "4290")
	if idx4280 > idx4290 {
		t.Error("expected ports to be sorted by port number")
	}
}

func TestPrintPorts_Empty(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printPorts([]PortInfo{}, false, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printPorts returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No ports in use") {
		t.Errorf("expected 'No ports in use', got: %s", output)
	}
}

func TestPrintNotRunningPorts_JSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printNotRunningPorts(true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printNotRunningPorts returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Parse JSON
	var result PortsOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Running {
		t.Error("expected Running to be false")
	}

	if len(result.Ports) != 0 {
		t.Errorf("expected 0 ports, got %d", len(result.Ports))
	}
}
