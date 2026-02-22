package admin

import (
	"net/http"
	"strconv"

	"github.com/getmockd/mockd/pkg/metrics"
)

// PortInfo represents information about a single port.
type PortInfo struct {
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Component string `json:"component"`
	Status    string `json:"status"`
	TLS       bool   `json:"tls,omitempty"`

	// Extended info (populated when verbose=true)
	EngineID   string `json:"engineId,omitempty"`
	EngineName string `json:"engineName,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	PID        int    `json:"pid,omitempty"`
}

// PortsResponse is the response for the /ports endpoint.
type PortsResponse struct {
	Ports []PortInfo `json:"ports"`
}

// handleListPorts handles GET /ports.
// Returns all ports in use by mockd, grouped by component.
// Query params:
//   - verbose=true: include engine ID, name, workspace, PID
func (a *API) handleListPorts(w http.ResponseWriter, r *http.Request) {
	verbose := false
	if v := parseOptionalBool(r.URL.Query().Get("verbose")); v != nil {
		verbose = *v
	}
	var ports []PortInfo

	// Admin API port (always running if we're handling this request)
	adminPort := PortInfo{
		Port:      a.port,
		Protocol:  "HTTP",
		Component: "Admin API",
		Status:    "running",
	}
	ports = append(ports, adminPort)

	// Get engine status for HTTP/HTTPS ports
	engine := a.localEngine.Load()
	if engine != nil {
		ctx := r.Context()
		status, err := engine.Status(ctx)

		// Engine metadata for verbose output
		var engineID, engineName string
		if err == nil {
			engineID = status.ID
			engineName = status.Name
		}

		if err == nil {
			// Helper to create port info with engine metadata
			makePortInfo := func(port int, protocol, component string, tls bool) PortInfo {
				p := PortInfo{
					Port:      port,
					Protocol:  protocol,
					Component: component,
					Status:    "running",
					TLS:       tls,
				}
				if verbose {
					p.EngineID = engineID
					p.EngineName = engineName
				}
				return p
			}

			// Check for HTTP protocol handler
			if httpStatus, ok := status.Protocols["http"]; ok && httpStatus.Enabled && httpStatus.Port > 0 {
				ports = append(ports, makePortInfo(httpStatus.Port, "HTTP", "Mock Engine", false))
			}

			// Check for HTTPS protocol handler
			if httpsStatus, ok := status.Protocols["https"]; ok && httpsStatus.Enabled && httpsStatus.Port > 0 {
				ports = append(ports, makePortInfo(httpsStatus.Port, "HTTPS", "Mock Engine", true))
			}

			// Check for gRPC handler
			if grpcStatus, ok := status.Protocols["grpc"]; ok && grpcStatus.Enabled && grpcStatus.Port > 0 {
				ports = append(ports, makePortInfo(grpcStatus.Port, "gRPC", "gRPC Server", false))
			}

			// Check for MQTT handler
			if mqttStatus, ok := status.Protocols["mqtt"]; ok && mqttStatus.Enabled && mqttStatus.Port > 0 {
				ports = append(ports, makePortInfo(mqttStatus.Port, "MQTT", "MQTT Broker", false))
			}
		}

		// Also check protocol handlers list for additional ports
		handlers, err := engine.ListHandlers(ctx)
		if err == nil {
			for _, h := range handlers {
				if h.Port > 0 {
					// Avoid duplicates - check if we already have this port
					found := false
					for _, p := range ports {
						if p.Port == h.Port {
							found = true
							break
						}
					}
					if !found {
						protocol := h.Type
						component := h.Type + " Server"
						switch h.Type {
						case "websocket":
							protocol = "WebSocket"
							component = "WebSocket Server"
						case "sse":
							protocol = "HTTP"
							component = "SSE Server"
						case "graphql":
							protocol = "HTTP"
							component = "GraphQL Server"
						case "soap":
							protocol = "HTTP"
							component = "SOAP Server"
						case "mqtt":
							protocol = "MQTT"
							component = "MQTT Broker"
						case "grpc":
							protocol = "gRPC"
							component = "gRPC Server"
						}
						p := PortInfo{
							Port:      h.Port,
							Protocol:  protocol,
							Component: component,
							Status:    h.Status,
						}
						if verbose {
							p.EngineID = engineID
							p.EngineName = engineName
						}
						ports = append(ports, p)
					}
				}
			}
		}
	}

	// Update Prometheus metric for port info
	updatePortMetrics(ports)

	writeJSON(w, http.StatusOK, PortsResponse{Ports: ports})
}

// updatePortMetrics updates the mockd_port_info Prometheus metric.
func updatePortMetrics(ports []PortInfo) {
	if metrics.PortInfo == nil {
		return
	}
	metrics.PortInfo.Reset()

	// Set gauge values for each port
	for _, p := range ports {
		vec, err := metrics.PortInfo.WithLabels(
			strconv.Itoa(p.Port),
			p.Protocol,
			p.Component,
		)
		if err == nil {
			if p.Status == "running" {
				vec.Set(1)
			} else {
				vec.Set(0)
			}
		}
	}
}
