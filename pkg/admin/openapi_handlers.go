package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/portability"
)

// yamlEscapeString escapes a string for safe inclusion in single-quoted YAML.
// Single quotes inside the string are escaped by doubling them.
func yamlEscapeString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// handleGetOpenAPISpec handles GET /openapi.json and GET /openapi.yaml
// Returns an OpenAPI 3.x specification of the currently configured HTTP mocks.
// This allows importing the mock endpoints into tools like Insomnia, Postman, or Swagger UI.
func (a *API) handleGetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Determine format from path or query param
	asYAML := r.URL.Path == "/openapi.yaml" || r.URL.Path == "/openapi.yml"
	if r.URL.Query().Get("format") == "yaml" {
		asYAML = true
	}

	// Get all mocks
	mocks, err := a.getAllMocksForExport(ctx)
	if err != nil {
		a.logger().Error("failed to list mocks for OpenAPI export", "error", err)
		writeError(w, http.StatusInternalServerError, "list_error", ErrMsgInternalError)
		return
	}

	// Filter to HTTP-only for OpenAPI (it doesn't support other protocols)
	httpMocks := make([]*config.MockConfiguration, 0)
	for _, m := range mocks {
		if m.Type == mock.TypeHTTP {
			httpMocks = append(httpMocks, m)
		}
	}

	// Build collection
	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "mockd API",
		Mocks:   httpMocks,
	}

	// Export to OpenAPI
	exporter := &portability.OpenAPIExporter{AsYAML: asYAML}
	data, err := exporter.Export(collection)
	if err != nil {
		a.logger().Error("failed to export OpenAPI spec", "error", err)
		writeError(w, http.StatusInternalServerError, "export_error", ErrMsgInternalError)
		return
	}

	// Set appropriate content type
	if asYAML {
		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleGetInsomniaExport handles GET /insomnia.json and GET /insomnia.yaml
// Returns an Insomnia collection with all mock types (HTTP, gRPC, WebSocket, GraphQL).
// - /insomnia.yaml returns Insomnia v5 format (recommended for modern Insomnia)
// - /insomnia.json returns Insomnia v4 format (legacy)
func (a *API) handleGetInsomniaExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mocks, err := a.getAllMocksForExport(ctx)
	if err != nil {
		a.logger().Error("failed to list mocks for Insomnia export", "error", err)
		writeError(w, http.StatusInternalServerError, "list_error", ErrMsgInternalError)
		return
	}

	// Check if v5 YAML format requested
	if r.URL.Path == "/insomnia.yaml" || r.URL.Query().Get("format") == "yaml" || r.URL.Query().Get("format") == "v5" {
		// Get stateful resources if available
		var statefulResources []statefulResourceInfo
		if client := a.localEngine.Load(); client != nil {
			overview, err := client.GetStateOverview(ctx)
			if err == nil && overview != nil {
				for _, res := range overview.Resources {
					// Generate a sample ID - use singular form of resource name + "-1"
					// e.g., "todos" -> "todo-1", "orders" -> "order-1"
					sampleID := strings.TrimSuffix(res.Name, "s") + "-1"

					// Extract parent path params from basePath (e.g., :postId from /api/posts/:postId/comments)
					pathParams := extractPathParams(res.BasePath)

					statefulResources = append(statefulResources, statefulResourceInfo{
						Name:       res.Name,
						BasePath:   res.BasePath,
						SampleID:   sampleID,
						PathParams: pathParams,
					})
				}
			}
		}

		export := buildInsomniaV5Export(mocks, statefulResources, a.port)
		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=mockd-insomnia.yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(export))
		return
	}

	// Build Insomnia v4 export (JSON)
	export := buildInsomniaExport(mocks, a.port)

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		a.logger().Error("failed to marshal Insomnia export", "error", err)
		writeError(w, http.StatusInternalServerError, "export_error", ErrMsgInternalError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=mockd-insomnia.json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// getAllMocksForExport gets mocks from engine or dataStore for export
func (a *API) getAllMocksForExport(ctx context.Context) ([]*config.MockConfiguration, error) {
	if engine := a.localEngine.Load(); engine != nil {
		mocks, err := engine.ListMocks(ctx)
		if err == nil {
			if mocks == nil {
				return []*config.MockConfiguration{}, nil
			}
			return mocks, nil
		}
		// Fall back to persistent data if available so export still works when the
		// local engine is temporarily unavailable.
		if a.dataStore != nil {
			a.logger().Warn("falling back to datastore for export after engine list failure", "error", err)
		} else {
			return nil, err
		}
	}
	// Fall back to dataStore
	if a.dataStore != nil {
		mocks, err := a.dataStore.Mocks().List(ctx, nil)
		if err != nil {
			return nil, err
		}
		if mocks == nil {
			return []*config.MockConfiguration{}, nil
		}
		return mocks, nil
	}
	return nil, errors.New("no mock source available")
}

// Insomnia v4 export types
type insomniaExport struct {
	Type      string             `json:"_type"`
	Format    int                `json:"__export_format"`
	Date      string             `json:"__export_date"`
	Source    string             `json:"__export_source"`
	Resources []insomniaResource `json:"resources"`
}

type insomniaResource struct {
	ID          string  `json:"_id"`
	Type        string  `json:"_type"`
	ParentID    *string `json:"parentId"` // null for workspace, string for others
	Modified    int64   `json:"modified"`
	Created     int64   `json:"created"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	IsPrivate   bool    `json:"isPrivate,omitempty"`
	MetaSortKey int64   `json:"metaSortKey,omitempty"`

	// Workspace fields
	Scope string `json:"scope,omitempty"`

	// Request fields (HTTP)
	Method     string           `json:"method,omitempty"`
	URL        string           `json:"url,omitempty"`
	Body       *insomniaBody    `json:"body,omitempty"`
	Headers    []insomniaHeader `json:"headers,omitempty"`
	Parameters []insomniaParam  `json:"parameters,omitempty"`

	// HTTP Request settings
	Authentication                  map[string]any `json:"authentication,omitempty"`
	PathParameters                  []any          `json:"pathParameters,omitempty"`
	SettingStoreCookies             *bool          `json:"settingStoreCookies,omitempty"`
	SettingSendCookies              *bool          `json:"settingSendCookies,omitempty"`
	SettingDisableRenderRequestBody *bool          `json:"settingDisableRenderRequestBody,omitempty"`
	SettingEncodeUrl                *bool          `json:"settingEncodeUrl,omitempty"` //nolint:revive,staticcheck // JSON API compatibility
	SettingRebuildPath              *bool          `json:"settingRebuildPath,omitempty"`
	SettingFollowRedirects          string         `json:"settingFollowRedirects,omitempty"`

	// gRPC fields
	ProtoFileID     string           `json:"protoFileId,omitempty"`
	ProtoMethodName string           `json:"protoMethodName,omitempty"`
	Metadata        []insomniaHeader `json:"metadata,omitempty"`

	// Request Group (folder) fields
	Environment              map[string]any `json:"environment,omitempty"`
	EnvironmentPropertyOrder *string        `json:"environmentPropertyOrder,omitempty"`

	// Environment fields
	Data              map[string]any `json:"data,omitempty"`
	DataPropertyOrder *string        `json:"dataPropertyOrder,omitempty"`
	Color             *string        `json:"color,omitempty"`
}

type insomniaBody struct {
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type insomniaHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type insomniaParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Helper to create string pointer
func strPtr(s string) *string {
	return &s
}

func buildInsomniaExport(mocks []*config.MockConfiguration, adminPort int) *insomniaExport {
	now := time.Now().UnixMilli()

	export := &insomniaExport{
		Type:      "export",
		Format:    4,
		Date:      time.Now().UTC().Format(time.RFC3339),
		Source:    "mockd",
		Resources: make([]insomniaResource, 0),
	}

	// Create workspace (parentId must be null)
	workspaceID := "wrk_mockd"
	export.Resources = append(export.Resources, insomniaResource{
		ID:          workspaceID,
		Type:        "workspace",
		ParentID:    nil, // null for workspace
		Modified:    now,
		Created:     now,
		Name:        "mockd Mocks",
		Description: "Exported from mockd",
		Scope:       "collection",
	})

	// Create base environment
	envID := "env_mockd_base"
	export.Resources = append(export.Resources, insomniaResource{
		ID:          envID,
		Type:        "environment",
		ParentID:    strPtr(workspaceID),
		Modified:    now,
		Created:     now,
		Name:        "Base Environment",
		IsPrivate:   false,
		MetaSortKey: now,
		Data: map[string]any{
			"base_url":  fmt.Sprintf("http://localhost:%d", adminPort-10), // Default engine port
			"admin_url": fmt.Sprintf("http://localhost:%d", adminPort),
		},
	})

	// Create folder groups by type
	folders := map[mock.Type]string{
		mock.TypeHTTP:      "fld_http",
		mock.TypeGraphQL:   "fld_graphql",
		mock.TypeGRPC:      "fld_grpc",
		mock.TypeWebSocket: "fld_websocket",
		mock.TypeMQTT:      "fld_mqtt",
		mock.TypeSOAP:      "fld_soap",
	}

	folderNames := map[mock.Type]string{
		mock.TypeHTTP:      "HTTP Mocks",
		mock.TypeGraphQL:   "GraphQL Mocks",
		mock.TypeGRPC:      "gRPC Mocks",
		mock.TypeWebSocket: "WebSocket Mocks",
		mock.TypeMQTT:      "MQTT Mocks",
		mock.TypeSOAP:      "SOAP Mocks",
	}

	// Track which folders we need
	usedFolders := make(map[mock.Type]bool)
	for _, m := range mocks {
		usedFolders[m.Type] = true
	}

	// Add only used folders
	sortKey := -now
	for mockType, folderID := range folders {
		if usedFolders[mockType] {
			export.Resources = append(export.Resources, insomniaResource{
				ID:          folderID,
				Type:        "request_group",
				ParentID:    strPtr(workspaceID),
				Modified:    now,
				Created:     now,
				Name:        folderNames[mockType],
				MetaSortKey: sortKey,
				Environment: map[string]any{},
			})
			sortKey--
		}
	}

	// Add mocks as requests
	for _, m := range mocks {
		resource := mockToInsomniaResource(m, folders[m.Type], now)
		if resource != nil {
			export.Resources = append(export.Resources, *resource)
		}
	}

	return export
}

func mockToInsomniaResource(m *config.MockConfiguration, parentID string, now int64) *insomniaResource {
	// Helper for boolean pointers
	boolPtr := func(b bool) *bool { return &b }

	name := m.Name
	if name == "" {
		name = m.ID
	}

	res := &insomniaResource{
		ParentID:    strPtr(parentID),
		Modified:    now,
		Created:     now,
		Name:        name,
		IsPrivate:   false,
		MetaSortKey: -now,
	}

	switch m.Type {
	case mock.TypeHTTP:
		if m.HTTP == nil || m.HTTP.Matcher == nil {
			return nil
		}
		res.ID = "req_" + m.ID
		res.Type = "request"
		res.Method = m.HTTP.Matcher.Method
		if res.Method == "" {
			res.Method = "GET"
		}
		res.URL = "{{ _.base_url }}" + m.HTTP.Matcher.Path

		// HTTP request settings (required)
		res.Body = &insomniaBody{}
		res.Headers = []insomniaHeader{}
		res.Parameters = []insomniaParam{}
		res.Authentication = map[string]any{}
		res.PathParameters = []any{}
		res.SettingStoreCookies = boolPtr(true)
		res.SettingSendCookies = boolPtr(true)
		res.SettingDisableRenderRequestBody = boolPtr(false)
		res.SettingEncodeUrl = boolPtr(true)
		res.SettingRebuildPath = boolPtr(true)
		res.SettingFollowRedirects = "global"

		// Add headers from matcher
		if m.HTTP.Matcher.Headers != nil {
			for k, v := range m.HTTP.Matcher.Headers {
				res.Headers = append(res.Headers, insomniaHeader{Name: k, Value: v})
			}
		}

		// Add query params
		if m.HTTP.Matcher.QueryParams != nil {
			for k, v := range m.HTTP.Matcher.QueryParams {
				res.Parameters = append(res.Parameters, insomniaParam{Name: k, Value: v})
			}
		}

		// Check if this is an SSE endpoint (has SSE config)
		if m.HTTP.SSE != nil {
			res.Headers = append(res.Headers, insomniaHeader{Name: "Accept", Value: "text/event-stream"})
		}

	case mock.TypeGraphQL:
		if m.GraphQL == nil {
			return nil
		}
		res.ID = "req_" + m.ID
		res.Type = "request"
		res.Method = "POST"
		res.URL = "{{ _.base_url }}" + m.GraphQL.Path

		// GraphQL body - create a sample query based on schema/resolvers
		query := "{ __typename }" // Default introspection query
		if m.GraphQL.Schema != "" {
			query = "{ __schema { types { name } } }"
		}
		body := map[string]string{"query": query}
		if bodyBytes, err := json.Marshal(body); err == nil {
			res.Body = &insomniaBody{
				MimeType: "application/json",
				Text:     string(bodyBytes),
			}
		}
		res.Headers = []insomniaHeader{{Name: "Content-Type", Value: "application/json"}}
		res.Parameters = []insomniaParam{}
		res.Authentication = map[string]any{}
		res.PathParameters = []any{}
		res.SettingStoreCookies = boolPtr(true)
		res.SettingSendCookies = boolPtr(true)
		res.SettingDisableRenderRequestBody = boolPtr(false)
		res.SettingEncodeUrl = boolPtr(true)
		res.SettingRebuildPath = boolPtr(true)
		res.SettingFollowRedirects = "global"

	case mock.TypeGRPC:
		if m.GRPC == nil {
			return nil
		}
		res.ID = "greq_" + m.ID // gRPC uses greq_ prefix
		res.Type = "grpc_request"
		res.ProtoFileID = ""
		res.Metadata = []insomniaHeader{}

		// gRPC URL is host:port
		if m.GRPC.Port > 0 {
			res.URL = fmt.Sprintf("localhost:%d", m.GRPC.Port)
		} else {
			res.URL = "localhost:50051"
		}

		// Service/method info from the map
		for svcName, svc := range m.GRPC.Services {
			for methodName, method := range svc.Methods {
				res.ProtoMethodName = fmt.Sprintf("/%s/%s", svcName, methodName)
				// Add sample request body (gRPC body is just text, no mimeType)
				if method.Response != nil {
					if bodyBytes, err := json.Marshal(method.Response); err == nil {
						res.Body = &insomniaBody{
							Text: string(bodyBytes),
						}
					}
				} else {
					res.Body = &insomniaBody{Text: "{}"}
				}
				break // Just use first method
			}
			break // Just use first service
		}

	case mock.TypeWebSocket:
		if m.WebSocket == nil {
			return nil
		}
		res.ID = "req_" + m.ID
		res.Type = "websocket_request"
		res.URL = "{{ _.base_url }}" + m.WebSocket.Path
		res.Headers = []insomniaHeader{}
		res.Parameters = []insomniaParam{}
		res.Authentication = map[string]any{}
		res.PathParameters = []any{}
		res.SettingStoreCookies = boolPtr(true)
		res.SettingSendCookies = boolPtr(true)
		res.SettingEncodeUrl = boolPtr(true)
		res.SettingFollowRedirects = "global"

	case mock.TypeSOAP:
		if m.SOAP == nil {
			return nil
		}
		res.ID = "req_" + m.ID
		res.Type = "request"
		res.Method = "POST"
		res.URL = "{{ _.base_url }}" + m.SOAP.Path
		res.Body = &insomniaBody{}
		res.Headers = []insomniaHeader{{Name: "Content-Type", Value: "text/xml; charset=utf-8"}}
		res.Parameters = []insomniaParam{}
		res.Authentication = map[string]any{}
		res.PathParameters = []any{}
		res.SettingStoreCookies = boolPtr(true)
		res.SettingSendCookies = boolPtr(true)
		res.SettingDisableRenderRequestBody = boolPtr(false)
		res.SettingEncodeUrl = boolPtr(true)
		res.SettingRebuildPath = boolPtr(true)
		res.SettingFollowRedirects = "global"

		// Add SOAPAction if we have operations
		for _, op := range m.SOAP.Operations {
			if op.SOAPAction != "" {
				res.Headers = append(res.Headers, insomniaHeader{Name: "SOAPAction", Value: op.SOAPAction})
				break
			}
		}

	case mock.TypeMQTT:
		// MQTT not directly supported by Insomnia, skip
		return nil

	default:
		return nil
	}

	return res
}

// statefulResourceInfo holds basic info about a stateful resource for export
type statefulResourceInfo struct {
	Name       string
	BasePath   string
	SampleID   string            // A sample ID to pre-fill in path parameters
	PathParams map[string]string // Parent path params (e.g., "postId" -> "post-1")
}

// extractPathParams extracts path parameter names from a URL path and generates sample values.
// e.g., "/api/posts/:postId/comments" -> {"postId": "post-1"}
func extractPathParams(path string) map[string]string {
	result := make(map[string]string)
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			// Generate sample value based on param name
			// e.g., "postId" -> "post-1", "userId" -> "user-1"
			sampleValue := generateSampleParamValue(paramName)
			result[paramName] = sampleValue
		}
	}

	return result
}

// generateSampleParamValue generates a sample value for a path parameter.
// e.g., "postId" -> "post-1", "userId" -> "user-1", "id" -> "item-1"
func generateSampleParamValue(paramName string) string {
	// Remove common suffixes to get the base name
	baseName := strings.TrimSuffix(paramName, "Id")
	baseName = strings.TrimSuffix(baseName, "ID")
	baseName = strings.TrimSuffix(baseName, "_id")

	if baseName == "" || baseName == paramName {
		// No suffix found, use the param name itself
		return paramName + "-1"
	}

	// Convert camelCase to kebab-case for the value
	return strings.ToLower(baseName) + "-1"
}

// writeInsomniaSettings writes the common Insomnia request settings block
func writeInsomniaSettings(sb *strings.Builder, indent string) {
	sb.WriteString(indent + "settings:\n")
	sb.WriteString(indent + "  renderRequestBody: true\n")
	sb.WriteString(indent + "  encodeUrl: true\n")
	sb.WriteString(indent + "  followRedirects: global\n")
	sb.WriteString(indent + "  cookies:\n")
	sb.WriteString(indent + "    send: true\n")
	sb.WriteString(indent + "    store: true\n")
	sb.WriteString(indent + "  rebuildPath: true\n")
}

// buildInsomniaV5Export creates an Insomnia v5 YAML export
func buildInsomniaV5Export(mocks []*config.MockConfiguration, statefulResources []statefulResourceInfo, adminPort int) string {
	now := time.Now().UnixMilli()
	baseURL := fmt.Sprintf("http://localhost:%d", adminPort-10)

	// Group mocks by type into folders
	httpMocks := make([]*config.MockConfiguration, 0)
	grpcMocks := make([]*config.MockConfiguration, 0)
	wsMocks := make([]*config.MockConfiguration, 0)
	graphqlMocks := make([]*config.MockConfiguration, 0)
	soapMocks := make([]*config.MockConfiguration, 0)

	for _, m := range mocks {
		switch m.Type { //nolint:exhaustive // only exportable mock types need handling
		case mock.TypeHTTP:
			httpMocks = append(httpMocks, m)
		case mock.TypeGRPC:
			grpcMocks = append(grpcMocks, m)
		case mock.TypeWebSocket:
			wsMocks = append(wsMocks, m)
		case mock.TypeGraphQL:
			graphqlMocks = append(graphqlMocks, m)
		case mock.TypeSOAP:
			soapMocks = append(soapMocks, m)
		}
	}

	// Build YAML manually for precise control
	sb := &strings.Builder{}
	sb.WriteString("type: collection.insomnia.rest/5.0\n")
	sb.WriteString("schema_version: \"5.1\"\n")
	sb.WriteString("name: mockd Collection\n")
	sb.WriteString("meta:\n")
	sb.WriteString("  id: wrk_mockd\n")
	fmt.Fprintf(sb, "  created: %d\n", now)
	fmt.Fprintf(sb, "  modified: %d\n", now)
	sb.WriteString("  description: Exported from mockd\n")

	// Collection (folders and requests)
	sb.WriteString("collection:\n")

	// HTTP folder
	if len(httpMocks) > 0 {
		sb.WriteString("  - name: HTTP Mocks\n")
		sb.WriteString("    meta:\n")
		sb.WriteString("      id: fld_http\n")
		fmt.Fprintf(sb, "      created: %d\n", now)
		fmt.Fprintf(sb, "      modified: %d\n", now)
		fmt.Fprintf(sb, "      sortKey: %d\n", -now)
		sb.WriteString("      description: \"\"\n")
		sb.WriteString("    children:\n")
		for _, m := range httpMocks {
			writeHTTPRequestV5(sb, m, now)
		}
	}

	// gRPC folder
	if len(grpcMocks) > 0 {
		sb.WriteString("  - name: gRPC Mocks\n")
		sb.WriteString("    meta:\n")
		sb.WriteString("      id: fld_grpc\n")
		fmt.Fprintf(sb, "      created: %d\n", now)
		fmt.Fprintf(sb, "      modified: %d\n", now)
		fmt.Fprintf(sb, "      sortKey: %d\n", -now-1)
		sb.WriteString("      description: \"\"\n")
		sb.WriteString("    children:\n")
		for _, m := range grpcMocks {
			writeGRPCRequestV5(sb, m, now)
		}
	}

	// WebSocket folder
	if len(wsMocks) > 0 {
		sb.WriteString("  - name: WebSocket Mocks\n")
		sb.WriteString("    meta:\n")
		sb.WriteString("      id: fld_ws\n")
		fmt.Fprintf(sb, "      created: %d\n", now)
		fmt.Fprintf(sb, "      modified: %d\n", now)
		fmt.Fprintf(sb, "      sortKey: %d\n", -now-2)
		sb.WriteString("      description: \"\"\n")
		sb.WriteString("    children:\n")
		for _, m := range wsMocks {
			writeWebSocketRequestV5(sb, m, now)
		}
	}

	// GraphQL folder
	if len(graphqlMocks) > 0 {
		sb.WriteString("  - name: GraphQL Mocks\n")
		sb.WriteString("    meta:\n")
		sb.WriteString("      id: fld_graphql\n")
		fmt.Fprintf(sb, "      created: %d\n", now)
		fmt.Fprintf(sb, "      modified: %d\n", now)
		fmt.Fprintf(sb, "      sortKey: %d\n", -now-3)
		sb.WriteString("      description: \"\"\n")
		sb.WriteString("    children:\n")
		for _, m := range graphqlMocks {
			writeGraphQLRequestV5(sb, m, now)
		}
	}

	// SOAP folder
	if len(soapMocks) > 0 {
		sb.WriteString("  - name: SOAP Mocks\n")
		sb.WriteString("    meta:\n")
		sb.WriteString("      id: fld_soap\n")
		fmt.Fprintf(sb, "      created: %d\n", now)
		fmt.Fprintf(sb, "      modified: %d\n", now)
		fmt.Fprintf(sb, "      sortKey: %d\n", -now-4)
		sb.WriteString("      description: \"\"\n")
		sb.WriteString("    children:\n")
		for _, m := range soapMocks {
			writeSOAPRequestV5(sb, m, now)
		}
	}

	// Stateful Resources folder (CRUD APIs)
	if len(statefulResources) > 0 {
		sb.WriteString("  - name: Stateful Resources (CRUD)\n")
		sb.WriteString("    meta:\n")
		sb.WriteString("      id: fld_stateful\n")
		fmt.Fprintf(sb, "      created: %d\n", now)
		fmt.Fprintf(sb, "      modified: %d\n", now)
		fmt.Fprintf(sb, "      sortKey: %d\n", -now-5)
		sb.WriteString("      description: \"In-memory CRUD resources with seed data. Use POST /state/reset to restore.\"\n")
		sb.WriteString("    children:\n")
		for _, res := range statefulResources {
			writeStatefulResourceRequestsV5(sb, res, now)
		}
	}

	// Cookie jar
	sb.WriteString("cookieJar:\n")
	sb.WriteString("  name: Default Jar\n")
	sb.WriteString("  meta:\n")
	sb.WriteString("    id: jar_mockd\n")
	fmt.Fprintf(sb, "    created: %d\n", now)
	fmt.Fprintf(sb, "    modified: %d\n", now)

	// Environments
	sb.WriteString("environments:\n")
	sb.WriteString("  name: Base Environment\n")
	sb.WriteString("  meta:\n")
	sb.WriteString("    id: env_mockd_base\n")
	fmt.Fprintf(sb, "    created: %d\n", now)
	fmt.Fprintf(sb, "    modified: %d\n", now)
	sb.WriteString("    isPrivate: false\n")
	sb.WriteString("  data:\n")
	fmt.Fprintf(sb, "    base_url: %s\n", baseURL)
	fmt.Fprintf(sb, "    admin_url: http://localhost:%d\n", adminPort)

	return sb.String()
}

func writeHTTPRequestV5(sb *strings.Builder, m *config.MockConfiguration, now int64) {
	if m.HTTP == nil || m.HTTP.Matcher == nil {
		return
	}
	name := m.Name
	if name == "" {
		name = m.ID
	}
	method := m.HTTP.Matcher.Method
	if method == "" {
		method = "GET"
	}
	path := m.HTTP.Matcher.Path
	if path == "" {
		path = "/"
	}

	fmt.Fprintf(sb, "      - url: \"{{ _.base_url }}%s\"\n", path)
	fmt.Fprintf(sb, "        name: %s\n", name)
	sb.WriteString("        meta:\n")
	fmt.Fprintf(sb, "          id: req_%s\n", m.ID)
	fmt.Fprintf(sb, "          created: %d\n", now)
	fmt.Fprintf(sb, "          modified: %d\n", now)
	sb.WriteString("          isPrivate: false\n")
	sb.WriteString("          description: \"\"\n")
	fmt.Fprintf(sb, "          sortKey: %d\n", -now)
	fmt.Fprintf(sb, "        method: %s\n", method)

	// Add sample body for methods that typically have request bodies
	if method == "POST" || method == "PUT" || method == "PATCH" {
		// Try to use the body matcher as a sample, otherwise use a generic JSON body
		sampleBody := `{"example": "data"}`
		switch {
		case m.HTTP.Matcher.BodyEquals != "":
			sampleBody = m.HTTP.Matcher.BodyEquals
		case m.HTTP.Matcher.BodyContains != "":
			sampleBody = m.HTTP.Matcher.BodyContains
		case m.HTTP.Matcher.BodyJSONPath != nil:
			if bodyBytes, err := json.Marshal(m.HTTP.Matcher.BodyJSONPath); err == nil {
				sampleBody = string(bodyBytes)
			}
		}
		sb.WriteString("        body:\n")
		sb.WriteString("          mimeType: application/json\n")
		fmt.Fprintf(sb, "          text: '%s'\n", yamlEscapeString(sampleBody))
		sb.WriteString("        headers:\n")
		sb.WriteString("          - name: Content-Type\n")
		sb.WriteString("            value: application/json\n")
	}

	writeInsomniaSettings(sb, "        ")
}

func writeGRPCRequestV5(sb *strings.Builder, m *config.MockConfiguration, now int64) {
	if m.GRPC == nil {
		return
	}
	name := m.Name
	if name == "" {
		name = m.ID
	}
	port := 50051
	if m.GRPC.Port > 0 {
		port = m.GRPC.Port
	}
	url := fmt.Sprintf("localhost:%d", port)

	var protoMethod string
	bodyText := "{}"
	for svcName, svc := range m.GRPC.Services {
		for methodName, method := range svc.Methods {
			protoMethod = fmt.Sprintf("/%s/%s", svcName, methodName)
			if method.Response != nil {
				if bodyBytes, err := json.Marshal(method.Response); err == nil {
					bodyText = string(bodyBytes)
				}
			}
			break
		}
		break
	}

	fmt.Fprintf(sb, "      - url: %s\n", url)
	fmt.Fprintf(sb, "        name: %s\n", name)
	sb.WriteString("        meta:\n")
	fmt.Fprintf(sb, "          id: greq_%s\n", m.ID)
	fmt.Fprintf(sb, "          created: %d\n", now)
	fmt.Fprintf(sb, "          modified: %d\n", now)
	sb.WriteString("          isPrivate: false\n")
	fmt.Fprintf(sb, "          description: \"gRPC service on port %d\"\n", port)
	fmt.Fprintf(sb, "          sortKey: %d\n", -now)
	sb.WriteString("        body:\n")
	fmt.Fprintf(sb, "          text: '%s'\n", yamlEscapeString(bodyText))
	sb.WriteString("        protoFileId: \"\"\n")
	fmt.Fprintf(sb, "        protoMethodName: %s\n", protoMethod)
	// reflectionApi is for Buf Schema Registry, not direct server reflection
	// For server reflection, Insomnia uses the main 'url' field directly
	// We leave reflectionApi disabled since mockd uses direct server reflection
	sb.WriteString("        reflectionApi:\n")
	sb.WriteString("          enabled: false\n")
	sb.WriteString("          url: \"\"\n")
	sb.WriteString("          apiKey: \"\"\n")
	sb.WriteString("          module: \"\"\n")
}

func writeWebSocketRequestV5(sb *strings.Builder, m *config.MockConfiguration, now int64) {
	if m.WebSocket == nil {
		return
	}
	name := m.Name
	if name == "" {
		name = m.ID
	}

	// Build description with sample payloads from matchers
	description := buildWSDescription(m)

	fmt.Fprintf(sb, "      - url: {{ _.base_url }}%s\n", m.WebSocket.Path)
	fmt.Fprintf(sb, "        name: %s\n", name)
	sb.WriteString("        meta:\n")
	fmt.Fprintf(sb, "          id: ws-req_%s\n", m.ID) // Must start with ws-req for Insomnia v5
	fmt.Fprintf(sb, "          created: %d\n", now)
	fmt.Fprintf(sb, "          modified: %d\n", now)
	sb.WriteString("          isPrivate: false\n")
	fmt.Fprintf(sb, "          description: \"%s\"\n", description)
	fmt.Fprintf(sb, "          sortKey: %d\n", -now)
	sb.WriteString("        settings:\n")
	sb.WriteString("          encodeUrl: true\n")
	sb.WriteString("          followRedirects: global\n")
	sb.WriteString("          cookies:\n")
	sb.WriteString("            send: true\n")
	sb.WriteString("            store: true\n")
}

func buildWSDescription(m *config.MockConfiguration) string {
	if m.WebSocket == nil {
		return ""
	}

	var parts []string

	// Check for echo mode
	if m.WebSocket.EchoMode != nil && *m.WebSocket.EchoMode {
		parts = append(parts, "Echo mode: send any message")
	}

	// Extract sample payloads from matchers
	if len(m.WebSocket.Matchers) > 0 {
		parts = append(parts, "Try:")
		for _, matcher := range m.WebSocket.Matchers {
			if matcher.Match == nil {
				continue
			}
			switch matcher.Match.Type {
			case "exact":
				parts = append(parts, "  "+matcher.Match.Value)
			case "json":
				// Build a sample JSON payload
				if matcher.Match.Path != "" && matcher.Match.Value != "" {
					// Clean up the path (remove $. prefix if present)
					path := strings.TrimPrefix(matcher.Match.Path, "$.")
					parts = append(parts, fmt.Sprintf("  {\\\"%s\\\": \\\"%s\\\"}", path, matcher.Match.Value))
				}
			case "regex":
				// Escape backslashes for YAML
				escaped := strings.ReplaceAll(matcher.Match.Value, "\\", "\\\\")
				parts = append(parts, fmt.Sprintf("  (regex: %s)", escaped))
			case "contains":
				parts = append(parts, fmt.Sprintf("  (contains: %s)", matcher.Match.Value))
			}
		}
	}

	// Check for scenarios
	if m.WebSocket.Scenario != nil && len(m.WebSocket.Scenario.Steps) > 0 {
		parts = append(parts, fmt.Sprintf("Scenario with %d steps", len(m.WebSocket.Scenario.Steps)))
	}

	// Check for heartbeat
	if m.WebSocket.Heartbeat != nil {
		parts = append(parts, "Has heartbeat")
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}

func writeGraphQLRequestV5(sb *strings.Builder, m *config.MockConfiguration, now int64) {
	if m.GraphQL == nil {
		return
	}
	name := m.Name
	if name == "" {
		name = m.ID
	}
	path := m.GraphQL.Path
	if path == "" {
		path = "/graphql"
	}

	// Build a useful sample query based on the schema/resolvers
	sampleQuery := "{ __typename }"
	description := "GraphQL endpoint"

	// Try to extract a meaningful query from resolvers (keys are like "Query.user", "Mutation.createUser")
	if m.GraphQL.Resolvers != nil {
		for fieldPath := range m.GraphQL.Resolvers {
			if strings.HasPrefix(fieldPath, "Query.") {
				fieldName := strings.TrimPrefix(fieldPath, "Query.")
				sampleQuery = fmt.Sprintf("{ %s }", fieldName)
				description = "Try: " + fieldName
				break
			}
		}
	}

	fmt.Fprintf(sb, "      - url: \"{{ _.base_url }}%s\"\n", path)
	fmt.Fprintf(sb, "        name: %s\n", name)
	sb.WriteString("        meta:\n")
	fmt.Fprintf(sb, "          id: req_%s\n", m.ID)
	fmt.Fprintf(sb, "          created: %d\n", now)
	fmt.Fprintf(sb, "          modified: %d\n", now)
	sb.WriteString("          isPrivate: false\n")
	fmt.Fprintf(sb, "          description: \"%s\"\n", description)
	fmt.Fprintf(sb, "          sortKey: %d\n", -now)
	sb.WriteString("        method: POST\n")
	sb.WriteString("        body:\n")
	sb.WriteString("          mimeType: application/json\n")
	fmt.Fprintf(sb, "          text: '%s'\n", yamlEscapeString(fmt.Sprintf(`{"query": "%s"}`, sampleQuery)))
	sb.WriteString("        headers:\n")
	sb.WriteString("          - name: Content-Type\n")
	sb.WriteString("            value: application/json\n")
	writeInsomniaSettings(sb, "        ")
}

func writeSOAPRequestV5(sb *strings.Builder, m *config.MockConfiguration, now int64) {
	if m.SOAP == nil {
		return
	}
	name := m.Name
	if name == "" {
		name = m.ID
	}
	path := m.SOAP.Path
	if path == "" {
		path = "/soap"
	}

	// Get first operation name for sample body
	var firstOp string
	var soapAction string
	for opName, op := range m.SOAP.Operations {
		firstOp = opName
		soapAction = op.SOAPAction
		break
	}
	if firstOp == "" {
		firstOp = "Operation"
	}

	// Build sample SOAP request body using single quotes (simpler YAML escaping)
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><%s xmlns="http://example.com/"></%s></soap:Body></soap:Envelope>`, firstOp, firstOp)

	fmt.Fprintf(sb, "      - url: \"{{ _.base_url }}%s\"\n", path)
	fmt.Fprintf(sb, "        name: %s\n", name)
	sb.WriteString("        meta:\n")
	fmt.Fprintf(sb, "          id: req_%s\n", m.ID)
	fmt.Fprintf(sb, "          created: %d\n", now)
	fmt.Fprintf(sb, "          modified: %d\n", now)
	sb.WriteString("          isPrivate: false\n")
	fmt.Fprintf(sb, "          description: \"SOAP operation: %s\"\n", firstOp)
	fmt.Fprintf(sb, "          sortKey: %d\n", -now)
	sb.WriteString("        method: POST\n")
	sb.WriteString("        body:\n")
	sb.WriteString("          mimeType: application/xml\n")
	fmt.Fprintf(sb, "          text: '%s'\n", yamlEscapeString(soapBody))
	sb.WriteString("        headers:\n")
	sb.WriteString("          - name: Content-Type\n")
	sb.WriteString("            value: text/xml; charset=utf-8\n")
	if soapAction != "" {
		sb.WriteString("          - name: SOAPAction\n")
		fmt.Fprintf(sb, "            value: \"%s\"\n", soapAction)
	}
	writeInsomniaSettings(sb, "        ")
}

// crudOperation defines a CRUD operation for Insomnia export
type crudOperation struct {
	suffix   string // URL suffix (empty or "/:id")
	name     string // Operation name prefix (List, Get, Create, Update, Delete)
	idSuffix string // ID suffix for request ID
	method   string
	desc     string
	hasID    bool // Whether it needs path parameter
	hasBody  bool // Whether it needs JSON body
	bodyText string
}

// writeStatefulResourceRequestsV5 writes a folder with CRUD requests for a stateful resource
func writeStatefulResourceRequestsV5(sb *strings.Builder, res statefulResourceInfo, now int64) {
	safeID := strings.ReplaceAll(res.Name, "-", "_")

	// Folder header
	fmt.Fprintf(sb, "      - name: %s\n", res.Name)
	sb.WriteString("        meta:\n")
	fmt.Fprintf(sb, "          id: fld_state_%s\n", safeID)
	fmt.Fprintf(sb, "          created: %d\n", now)
	fmt.Fprintf(sb, "          modified: %d\n", now)
	fmt.Fprintf(sb, "          sortKey: %d\n", -now)
	fmt.Fprintf(sb, "          description: \"CRUD operations for %s\"\n", res.Name)
	sb.WriteString("        children:\n")

	ops := []crudOperation{
		{"", "List", "list", "GET", "GET with filters: ?limit=10&offset=0&sort=id&order=asc", false, false, ""},
		{"/:id", "Get", "get", "GET", fmt.Sprintf("Fetches single %s by ID", res.Name), true, false, ""},
		{"", "Create", "create", "POST", "Creates new item, returns 201 with generated ID", false, true, `{"name": "New Item"}`},
		{"/:id", "Update", "update", "PUT", fmt.Sprintf("Updates %s by ID", res.Name), true, true, `{"name": "Updated Item"}`},
		{"/:id", "Delete", "delete", "DELETE", "Deletes item by ID, returns 204 No Content", true, false, ""},
	}

	for i, op := range ops {
		fmt.Fprintf(sb, "          - url: \"{{ _.base_url }}%s%s\"\n", res.BasePath, op.suffix)
		fmt.Fprintf(sb, "            name: %s %s\n", op.name, res.Name)
		sb.WriteString("            meta:\n")
		fmt.Fprintf(sb, "              id: req_state_%s_%s\n", safeID, op.idSuffix)
		fmt.Fprintf(sb, "              created: %d\n", now)
		fmt.Fprintf(sb, "              modified: %d\n", now)
		sb.WriteString("              isPrivate: false\n")
		fmt.Fprintf(sb, "              description: \"%s\"\n", op.desc)
		fmt.Fprintf(sb, "              sortKey: %d\n", -now-int64(i))
		fmt.Fprintf(sb, "            method: %s\n", op.method)

		if op.hasBody {
			sb.WriteString("            body:\n")
			sb.WriteString("              mimeType: application/json\n")
			fmt.Fprintf(sb, "              text: '%s'\n", op.bodyText)
			sb.WriteString("            headers:\n")
			sb.WriteString("              - name: Content-Type\n")
			sb.WriteString("                value: application/json\n")
		}

		// Add path parameters (parent params + item ID if needed)
		hasPathParams := op.hasID || len(res.PathParams) > 0
		if hasPathParams {
			sb.WriteString("            pathParameters:\n")
			// Add parent path params first (e.g., postId)
			for paramName, paramValue := range res.PathParams {
				fmt.Fprintf(sb, "              - name: %s\n", paramName)
				fmt.Fprintf(sb, "                value: \"%s\"\n", paramValue)
			}
			// Add item ID param if this operation uses it
			if op.hasID {
				sb.WriteString("              - name: id\n")
				fmt.Fprintf(sb, "                value: \"%s\"\n", res.SampleID)
			}
		}

		writeInsomniaSettings(sb, "            ")
	}
}
