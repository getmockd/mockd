package mcp

import (
	"github.com/getmockd/mockd/pkg/portability"
)

// =============================================================================
// Import / Export Handlers
// =============================================================================

// handleImportMocks imports mocks from inline content.
func handleImportMocks(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	content := getString(args, "content", "")
	if content == "" {
		return ToolResultError("content is required"), nil
	}

	formatHint := getString(args, "format", "auto")
	replace := getBool(args, "replace", false)
	dryRun := getBool(args, "dryRun", false)

	data := []byte(content)

	// Detect format
	var format portability.Format
	if formatHint == "auto" || formatHint == "" {
		format = portability.DetectFormat(data, "")
		if format == portability.FormatUnknown {
			return ToolResultError("could not auto-detect format. Try specifying the format parameter."), nil
		}
	} else {
		format = portability.ParseFormat(formatHint)
		if format == portability.FormatUnknown {
			return ToolResultError("unsupported format: " + formatHint), nil
		}
	}

	// Parse the content into a MockCollection using the registry
	importer := portability.GetImporter(format)
	if importer == nil {
		return ToolResultError("no importer available for format: " + string(format)), nil
	}

	collection, err := importer.Import(data)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("import parse error: " + err.Error()), nil
	}
	if collection == nil || len(collection.Mocks) == 0 {
		return ToolResultError("no mocks found in content"), nil
	}

	mockCount := len(collection.Mocks)

	// Dry-run: return what would be imported without applying
	if dryRun {
		result := map[string]interface{}{
			"dryRun":       true,
			"format":       string(format),
			"mockCount":    mockCount,
			"wouldReplace": replace,
		}
		return ToolResultJSON(result)
	}

	// Apply the import via admin API
	importResult, err := client.ImportConfig(collection, replace)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to apply import: " + adminError(err, session.GetAdminURL())), nil
	}

	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"imported": importResult.Imported,
		"format":   string(format),
		"replaced": replace,
	}
	if importResult.Message != "" {
		result["message"] = importResult.Message
	}

	return ToolResultJSON(result)
}

// handleExportMocks exports all current mocks as configuration.
func handleExportMocks(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	format := getString(args, "format", "yaml")

	// Get the collection from admin API
	collection, err := client.ExportConfig("")
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to export mocks: " + adminError(err, session.GetAdminURL())), nil
	}

	if collection == nil {
		return ToolResultError("no configuration to export"), nil
	}

	// Export via portability package
	asYAML := format != "json"
	exportResult, err := portability.Export(collection, &portability.ExportOptions{
		Format: portability.FormatMockd,
		AsYAML: &asYAML,
		Pretty: true,
	})
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to format export: " + err.Error()), nil
	}

	// Return as text content with appropriate mime type
	mimeType := "application/yaml"
	if format == "json" {
		mimeType = "application/json"
	}

	return &ToolResult{
		Content: []ContentBlock{
			{
				Type:     "text",
				Text:     string(exportResult.Data),
				MimeType: mimeType,
			},
		},
	}, nil
}
