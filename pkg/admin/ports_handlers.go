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
}

// PortsResponse is the response for the /ports endpoint.
type PortsResponse struct {
	Ports []PortInfo `json:"ports"`
}

// handleListPorts handles GET /ports.
// Returns all ports in use by mockd, grouped by component.
func (a *AdminAPI) handleListPorts(w http.ResponseWriter, r *http.Request) {
	var ports []PortInfo

	// Admin API port (always running if we're handling this request)
	ports = append(ports, PortInfo{
		Port:      a.port,
		Protocol:  "HTTP",
		Component: "Admin API",
		Status:    "running",
	})

	// Get engine status for HTTP/HTTPS ports
	if a.localEngine != nil {
		ctx := r.Context()
		status, err := a.localEngine.Status(ctx)
		if err == nil {
			// Check for HTTP protocol handler
			if httpStatus, ok := status.Protocols["http"]; ok && httpStatus.Enabled && httpStatus.Port > 0 {
				ports = append(ports, PortInfo{
					Port:      httpStatus.Port,
					Protocol:  "HTTP",
					Component: "Mock Engine",
					Status:    "running",
				})
			}

			// Check for HTTPS protocol handler
			if httpsStatus, ok := status.Protocols["https"]; ok && httpsStatus.Enabled && httpsStatus.Port > 0 {
				ports = append(ports, PortInfo{
					Port:      httpsStatus.Port,
					Protocol:  "HTTPS",
					Component: "Mock Engine",
					Status:    "running",
					TLS:       true,
				})
			}

			// Check for gRPC handler
			if grpcStatus, ok := status.Protocols["grpc"]; ok && grpcStatus.Enabled && grpcStatus.Port > 0 {
				ports = append(ports, PortInfo{
					Port:      grpcStatus.Port,
					Protocol:  "gRPC",
					Component: "gRPC Server",
					Status:    "running",
				})
			}

			// Check for MQTT handler
			if mqttStatus, ok := status.Protocols["mqtt"]; ok && mqttStatus.Enabled && mqttStatus.Port > 0 {
				ports = append(ports, PortInfo{
					Port:      mqttStatus.Port,
					Protocol:  "MQTT",
					Component: "MQTT Broker",
					Status:    "running",
				})
			}
		}

		// Also check protocol handlers list for additional ports
		handlers, err := a.localEngine.ListHandlers(ctx)
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
						ports = append(ports, PortInfo{
							Port:      h.Port,
							Protocol:  protocol,
							Component: component,
							Status:    h.Status,
						})
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
