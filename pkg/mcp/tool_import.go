package mcp

import (
	"errors"
	"io/fs"
	"os"

	"github.com/getmockd/mockd/pkg/portability"
)

// =============================================================================
// Import / Export Handlers
// =============================================================================

// handleImportMocks imports mocks from inline content or a file path.
func handleImportMocks(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	content := getString(args, "content", "")
	filePath := getString(args, "file", "")

	// Validate: exactly one of content or file must be provided.
	if content != "" && filePath != "" {
		return ToolResultError("content and file are mutually exclusive — provide one or the other, not both"), nil
	}
	if content == "" && filePath == "" {
		return ToolResultError("either content or file is required"), nil
	}

	// If file path is provided, read the file from the server's filesystem.
	if filePath != "" {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return ToolResultError("file not found: " + filePath), nil
			}
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to read file: " + err.Error()), nil
		}
		content = string(fileData)
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
