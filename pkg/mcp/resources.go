package mcp

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/mock"
)

// ResourceProvider provides MCP resources from the mock engine.
type ResourceProvider struct {
	adminClient cli.AdminClient
}

// NewResourceProvider creates a new resource provider.
func NewResourceProvider(client cli.AdminClient) *ResourceProvider {
	return &ResourceProvider{
		adminClient: client,
	}
}

// WithClient returns a shallow copy of the ResourceProvider using the given
// admin client. This is used so that session-scoped context switches are
// reflected in resource listings (the session may point at a different
// admin server after switch_context).
func (p *ResourceProvider) WithClient(client cli.AdminClient) *ResourceProvider {
	if client == nil {
		return p
	}
	return &ResourceProvider{
		adminClient: client,
	}
}

// List returns all available resources.
func (p *ResourceProvider) List() []ResourceDefinition {
	resources := make([]ResourceDefinition, 0)

	// Add mock endpoint resources (all protocol types)
	if p.adminClient != nil {
		mocks, err := p.adminClient.ListMocks()
		if err == nil {
			for _, m := range mocks {
				uri, name, desc := mockResourceInfo(m)
				if uri == "" {
					continue
				}
				resources = append(resources, ResourceDefinition{
					URI:         uri,
					Name:        name,
					Description: desc,
					MimeType:    "application/json",
				})
			}
		}
	}

	// Add stateful resources (via admin API so it works in all modes)
	if p.adminClient != nil {
		overview, err := p.adminClient.GetStateOverview()
		if err == nil && overview != nil {
			for _, res := range overview.Resources {
				uri := "mock://stateful/" + res.Name
				description := "CRUD operations on " + res.Name + " (" + strconv.Itoa(res.ItemCount) + " items)"

				resources = append(resources, ResourceDefinition{
					URI:         uri,
					Name:        "Stateful: " + res.Name,
					Description: description,
					MimeType:    "application/json",
				})
			}
		}
	}

	// Add chaos resource
	resources = append(resources, ResourceDefinition{
		URI:         "mock://chaos",
		Name:        "Chaos Configuration",
		Description: "Active chaos injection rules and statistics",
		MimeType:    "application/json",
	})

	// Add verification resource template
	// Note: This is a resource template with {mockId} parameter.
	// Clients can read mock://verification/<mockId> for any mock.
	resources = append(resources, ResourceDefinition{
		URI:         "mock://verification",
		Name:        "Mock Verification",
		Description: "Verification status and invocations for a mock. Read mock://verification/{mockId} for a specific mock.",
		MimeType:    "application/json",
	})

	// Add system resources
	resources = append(resources, ResourceDefinition{
		URI:         "mock://logs",
		Name:        "Request Logs",
		Description: "Captured HTTP request/response logs",
		MimeType:    "application/json",
	})

	resources = append(resources, ResourceDefinition{
		URI:         "mock://config",
		Name:        "Server Configuration",
		Description: "Current mockd server configuration",
		MimeType:    "application/json",
	})

	resources = append(resources, ResourceDefinition{
		URI:         "mock://context",
		Name:        "Current Context",
		Description: "Active context (admin server connection) and available contexts",
		MimeType:    "application/json",
	})

	return resources
}

// Read reads the contents of a resource.
func (p *ResourceProvider) Read(uri string) ([]ResourceContent, *JSONRPCError) {
	// Parse the URI
	resourceType, path, _ := parseResourceURI(uri)

	switch resourceType {
	case "mock":
		return p.readMockResource(uri)
	case "stateful":
		return p.readStatefulResource(path)
	case "chaos":
		return p.readChaosResource()
	case "verification":
		return p.readVerificationResource(path)
	case "logs":
		return p.readLogsResource()
	case "config":
		return p.readConfigResource()
	case "context":
		return p.readContextResource()
	default:
		return nil, ResourceNotFoundError(uri)
	}
}

// parseResourceURI parses a mock:// URI into its components.
func parseResourceURI(uri string) (resourceType, path, method string) {
	// Remove mock:// prefix
	if !strings.HasPrefix(uri, "mock://") {
		return "", "", ""
	}

	rest := strings.TrimPrefix(uri, "mock://")

	// Check for special resource types
	if strings.HasPrefix(rest, "stateful/") {
		return "stateful", strings.TrimPrefix(rest, "stateful/"), ""
	}
	if rest == "chaos" {
		return "chaos", "", ""
	}
	if strings.HasPrefix(rest, "verification/") {
		return "verification", strings.TrimPrefix(rest, "verification/"), ""
	}
	if rest == "verification" {
		return "verification", "", ""
	}
	if rest == "logs" {
		return "logs", "", ""
	}
	if rest == "config" {
		return "config", "", ""
	}
	if rest == "context" {
		return "context", "", ""
	}

	// Regular mock endpoint - path starts with /
	// May have #METHOD fragment
	method = "GET"
	if idx := strings.Index(rest, "#"); idx != -1 {
		method = rest[idx+1:]
		rest = rest[:idx]
	}

	// Path should start with /
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}

	return "mock", rest, method
}

// readMockResource reads a mock endpoint resource by matching the full URI
// against all known mocks (any protocol type).
func (p *ResourceProvider) readMockResource(requestedURI string) ([]ResourceContent, *JSONRPCError) {
	if p.adminClient == nil {
		return nil, InternalError(nil)
	}

	mocks, err := p.adminClient.ListMocks()
	if err != nil {
		return nil, InternalError(err)
	}

	for _, m := range mocks {
		uri, _, _ := mockResourceInfo(m)
		if uri == "" || uri != requestedURI {
			continue
		}

		// Return the full mock as JSON
		data, _ := json.Marshal(m)
		return []ResourceContent{
			{
				URI:      uri,
				MimeType: "application/json",
				Text:     string(data),
			},
		}, nil
	}

	return nil, ResourceNotFoundError(requestedURI)
}

// readStatefulResource reads a stateful resource via admin API.
func (p *ResourceProvider) readStatefulResource(name string) ([]ResourceContent, *JSONRPCError) {
	if p.adminClient == nil {
		return nil, InternalError(nil)
	}

	overview, err := p.adminClient.GetStateOverview()
	if err != nil {
		return nil, InternalError(err)
	}

	// Find the specific resource in the overview
	var found *cli.StatefulResourceInfo
	if overview != nil {
		for i := range overview.Resources {
			if overview.Resources[i].Name == name {
				found = &overview.Resources[i]
				break
			}
		}
	}

	if found == nil {
		return nil, ResourceNotFoundError("mock://stateful/" + name)
	}

	content := map[string]interface{}{
		"name":          found.Name,
		"basePath":      found.BasePath,
		"idField":       found.IDField,
		"itemCount":     found.ItemCount,
		"hasSeedData":   found.SeedCount > 0,
		"seedDataCount": found.SeedCount,
	}

	if found.ParentField != "" {
		content["parentField"] = found.ParentField
	}

	text, _ := json.Marshal(content)
	return []ResourceContent{
		{
			URI:      "mock://stateful/" + name,
			MimeType: "application/json",
			Text:     string(text),
		},
	}, nil
}

// readChaosResource reads the chaos configuration and statistics resource.
func (p *ResourceProvider) readChaosResource() ([]ResourceContent, *JSONRPCError) {
	if p.adminClient == nil {
		return nil, InternalError(nil)
	}

	content := make(map[string]interface{})

	chaosConfig, err := p.adminClient.GetChaosConfig()
	if err != nil {
		return nil, InternalError(err)
	}
	content["config"] = chaosConfig

	chaosStats, err := p.adminClient.GetChaosStats()
	if err == nil && chaosStats != nil {
		content["stats"] = chaosStats
	}

	// Include stateful fault status if available.
	statefulFaults, err := p.adminClient.GetStatefulFaultStats()
	if err == nil && statefulFaults != nil {
		content["statefulFaults"] = statefulFaults
	}

	text, _ := json.Marshal(content)
	return []ResourceContent{
		{
			URI:      "mock://chaos",
			MimeType: "application/json",
			Text:     string(text),
		},
	}, nil
}

// readVerificationResource reads verification status and invocations for a mock.
func (p *ResourceProvider) readVerificationResource(mockID string) ([]ResourceContent, *JSONRPCError) {
	if p.adminClient == nil {
		return nil, InternalError(nil)
	}

	if mockID == "" {
		return nil, ResourceNotFoundError("mock://verification — specify a mock ID: mock://verification/{mockId}")
	}

	content := map[string]interface{}{
		"mockId": mockID,
	}

	verification, err := p.adminClient.GetMockVerification(mockID)
	if err != nil {
		return nil, ResourceNotFoundError("mock://verification/" + mockID)
	}
	content["verification"] = verification

	invocations, err := p.adminClient.ListMockInvocations(mockID)
	if err == nil && invocations != nil {
		content["invocations"] = invocations
	}

	text, _ := json.Marshal(content)
	return []ResourceContent{
		{
			URI:      "mock://verification/" + mockID,
			MimeType: "application/json",
			Text:     string(text),
		},
	}, nil
}

// readLogsResource reads the logs resource.
func (p *ResourceProvider) readLogsResource() ([]ResourceContent, *JSONRPCError) {
	if p.adminClient == nil {
		return nil, InternalError(nil)
	}

	// Get logs from admin API
	logsResult, err := p.adminClient.GetLogs(nil)
	if err != nil {
		return nil, InternalError(err)
	}

	// Get method and status distribution
	methodCounts := make(map[string]int)
	statusCounts := make(map[int]int)

	for _, log := range logsResult.Requests {
		methodCounts[log.Method]++
		statusCounts[log.ResponseStatus]++
	}

	content := map[string]interface{}{
		"count":        logsResult.Count,
		"total":        logsResult.Total,
		"methodCounts": methodCounts,
		"statusCounts": statusCounts,
	}

	if len(logsResult.Requests) > 0 {
		content["oldestEntry"] = logsResult.Requests[len(logsResult.Requests)-1].Timestamp.Format("2006-01-02T15:04:05Z")
		content["newestEntry"] = logsResult.Requests[0].Timestamp.Format("2006-01-02T15:04:05Z")
	}

	text, _ := json.Marshal(content)
	return []ResourceContent{
		{
			URI:      "mock://logs",
			MimeType: "application/json",
			Text:     string(text),
		},
	}, nil
}

// readConfigResource reads the config resource.
func (p *ResourceProvider) readConfigResource() ([]ResourceContent, *JSONRPCError) {
	content := map[string]interface{}{
		"version": ServerVersion,
	}

	if p.adminClient != nil {
		// Get mock count from admin API
		mocks, err := p.adminClient.ListMocks()
		if err == nil {
			content["mockCount"] = len(mocks)
		}

		// Get stats if available
		stats, err := p.adminClient.GetStats()
		if err == nil && stats != nil {
			content["totalRequests"] = stats.RequestCount
			content["uptime"] = stats.Uptime
		}
	}

	if p.adminClient != nil {
		overview, err := p.adminClient.GetStateOverview()
		if err == nil && overview != nil {
			content["statefulResourceCount"] = overview.Total
		}
	}

	text, _ := json.Marshal(content)
	return []ResourceContent{
		{
			URI:      "mock://config",
			MimeType: "application/json",
			Text:     string(text),
		},
	}, nil
}

// readContextResource reads the context resource (available contexts from config).
func (p *ResourceProvider) readContextResource() ([]ResourceContent, *JSONRPCError) {
	ctxConfig, _ := cliconfig.LoadContextConfig()

	content := map[string]interface{}{
		"currentContext": "",
	}

	if ctxConfig != nil {
		content["currentContext"] = ctxConfig.CurrentContext

		contexts := make(map[string]interface{})
		for name, ctx := range ctxConfig.Contexts {
			// AuthToken intentionally omitted for security
			info := map[string]interface{}{
				"adminUrl": ctx.AdminURL,
			}
			if ctx.Workspace != "" {
				info["workspace"] = ctx.Workspace
			}
			if ctx.Description != "" {
				info["description"] = ctx.Description
			}
			contexts[name] = info
		}
		content["contexts"] = contexts
	}

	text, _ := json.Marshal(content)
	return []ResourceContent{
		{
			URI:      "mock://context",
			MimeType: "application/json",
			Text:     string(text),
		},
	}, nil
}

// mockResourceInfo returns the URI, display name, and description for a mock resource.
// Works for all protocol types, not just HTTP.
func mockResourceInfo(m *mock.Mock) (uri, name, desc string) {
	switch m.Type {
	case mock.TypeHTTP:
		if m.HTTP == nil || m.HTTP.Matcher == nil {
			return "", "", ""
		}
		method := m.HTTP.Matcher.Method
		if method == "" {
			method = "GET"
		}
		uri = "mock://" + m.HTTP.Matcher.Path + "#" + method
		name = method + " " + m.HTTP.Matcher.Path
		desc = m.Name
		if desc == "" {
			desc = "Mock endpoint for " + m.HTTP.Matcher.Path
		}
	case mock.TypeWebSocket:
		if m.WebSocket == nil {
			return "", "", ""
		}
		uri = "mock://websocket" + m.WebSocket.Path
		name = "WS " + m.WebSocket.Path
		desc = m.Name
		if desc == "" {
			desc = "WebSocket endpoint " + m.WebSocket.Path
		}
	case mock.TypeGraphQL:
		if m.GraphQL == nil {
			return "", "", ""
		}
		uri = "mock://graphql" + m.GraphQL.Path
		name = "GraphQL " + m.GraphQL.Path
		desc = m.Name
		if desc == "" {
			desc = "GraphQL endpoint " + m.GraphQL.Path
		}
	case mock.TypeGRPC:
		if m.GRPC == nil {
			return "", "", ""
		}
		uri = "mock://grpc/" + m.ID
		name = "gRPC :" + strconv.Itoa(m.GRPC.Port)
		desc = m.Name
		if desc == "" {
			desc = "gRPC mock on port " + strconv.Itoa(m.GRPC.Port)
		}
	case mock.TypeSOAP:
		if m.SOAP == nil {
			return "", "", ""
		}
		uri = "mock://soap" + m.SOAP.Path
		name = "SOAP " + m.SOAP.Path
		desc = m.Name
		if desc == "" {
			desc = "SOAP endpoint " + m.SOAP.Path
		}
	case mock.TypeMQTT:
		if m.MQTT == nil {
			return "", "", ""
		}
		uri = "mock://mqtt/" + m.ID
		name = "MQTT :" + strconv.Itoa(m.MQTT.Port)
		desc = m.Name
		if desc == "" {
			desc = "MQTT broker on port " + strconv.Itoa(m.MQTT.Port)
		}
	case mock.TypeOAuth:
		if m.OAuth == nil {
			return "", "", ""
		}
		uri = "mock://oauth/" + m.ID
		name = "OAuth " + m.OAuth.Issuer
		desc = m.Name
		if desc == "" {
			desc = "OAuth provider " + m.OAuth.Issuer
		}
	default:
		return "", "", ""
	}
	return uri, name, desc
}

// GenerateURI generates a mock:// URI for a mock configuration.
func GenerateURI(path, method string) string {
	if method == "" {
		method = "GET"
	}
	return "mock://" + path + "#" + method
}

// GenerateStatefulURI generates a mock://stateful/ URI.
func GenerateStatefulURI(name string) string {
	return "mock://stateful/" + name
}
