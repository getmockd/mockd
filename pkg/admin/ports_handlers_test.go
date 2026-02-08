package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/metrics"
)

func TestHandleListPorts_NoEngine(t *testing.T) {
	// Initialize metrics
	metrics.Reset()
	metrics.Init()
	defer metrics.Reset()

	// Create admin API without engine
	api := NewAPI(4290)
	defer api.Stop()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/ports", nil)
	w := httptest.NewRecorder()

	// Call handler
	api.handleListPorts(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp PortsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should have at least the admin API port
	if len(resp.Ports) < 1 {
		t.Errorf("expected at least 1 port, got %d", len(resp.Ports))
	}

	// Check that admin API port is present
	found := false
	for _, p := range resp.Ports {
		if p.Component == "Admin API" && p.Port == 4290 {
			found = true
			if p.Protocol != "HTTP" {
				t.Errorf("expected admin API protocol 'HTTP', got %s", p.Protocol)
			}
			if p.Status != "running" {
				t.Errorf("expected admin API status 'running', got %s", p.Status)
			}
		}
	}
	if !found {
		t.Error("expected to find Admin API port in response")
	}
}

func TestPortInfo_JSON(t *testing.T) {
	p := PortInfo{
		Port:      4280,
		Protocol:  "HTTP",
		Component: "Mock Engine",
		Status:    "running",
		TLS:       false,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal PortInfo: %v", err)
	}

	var p2 PortInfo
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatalf("failed to unmarshal PortInfo: %v", err)
	}

	if p2.Port != p.Port {
		t.Errorf("expected Port %d, got %d", p.Port, p2.Port)
	}
	if p2.Protocol != p.Protocol {
		t.Errorf("expected Protocol %s, got %s", p.Protocol, p2.Protocol)
	}
	if p2.Component != p.Component {
		t.Errorf("expected Component %s, got %s", p.Component, p2.Component)
	}
	if p2.Status != p.Status {
		t.Errorf("expected Status %s, got %s", p.Status, p2.Status)
	}
}

func TestPortInfo_JSON_WithTLS(t *testing.T) {
	p := PortInfo{
		Port:      5280,
		Protocol:  "HTTPS",
		Component: "Mock Engine",
		Status:    "running",
		TLS:       true,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal PortInfo: %v", err)
	}

	// Check that TLS is included when true
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := m["tls"]; !ok {
		t.Error("expected 'tls' field to be present when TLS is true")
	}
}

func TestPortInfo_JSON_WithoutTLS(t *testing.T) {
	p := PortInfo{
		Port:      4280,
		Protocol:  "HTTP",
		Component: "Mock Engine",
		Status:    "running",
		TLS:       false,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal PortInfo: %v", err)
	}

	// Check that TLS is omitted when false (omitempty)
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := m["tls"]; ok {
		t.Error("expected 'tls' field to be omitted when TLS is false")
	}
}
