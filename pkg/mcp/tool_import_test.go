package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// validMockdYAML is a minimal mockd-format YAML config that the real
// portability package can parse into a MockCollection with one mock.
const validMockdYAML = `version: "1.0"
mocks:
  - id: test-mock
    type: http
    http:
      matcher:
        method: GET
        path: /test
      response:
        statusCode: 200
`

// validMockdJSON is the JSON equivalent for format-detection tests.
const validMockdJSON = `{"version":"1.0","mocks":[{"id":"test-mock","type":"http","http":{"matcher":{"method":"GET","path":"/test"},"response":{"statusCode":200}}}]}`

// =============================================================================
// handleImportMocks Tests
// =============================================================================

func TestHandleImportMocks_DryRun(t *testing.T) {
	t.Parallel()

	importCalled := false
	client := &mockAdminClient{
		importConfigFn: func(collection *config.MockCollection, replace bool) (*cli.ImportResult, error) {
			importCalled = true
			return nil, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
		"dryRun":  true,
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if importCalled {
		t.Error("ImportConfig should NOT be called during dry run")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["dryRun"] != true {
		t.Errorf("dryRun = %v, want true", parsed["dryRun"])
	}
	if parsed["mockCount"] != float64(1) {
		t.Errorf("mockCount = %v, want 1", parsed["mockCount"])
	}
	if parsed["wouldReplace"] != false {
		t.Errorf("wouldReplace = %v, want false", parsed["wouldReplace"])
	}
	// Format should be detected
	format, _ := parsed["format"].(string)
	if format == "" {
		t.Error("expected non-empty format field")
	}
}

func TestHandleImportMocks_AutoDetectYAML(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		importConfigFn: func(collection *config.MockCollection, replace bool) (*cli.ImportResult, error) {
			return &cli.ImportResult{Imported: len(collection.Mocks)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["imported"] != float64(1) {
		t.Errorf("imported = %v, want 1", parsed["imported"])
	}
	format, _ := parsed["format"].(string)
	if format == "" {
		t.Error("expected non-empty format field")
	}
}

func TestHandleImportMocks_SpecificFormat(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		importConfigFn: func(collection *config.MockCollection, replace bool) (*cli.ImportResult, error) {
			return &cli.ImportResult{Imported: len(collection.Mocks)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
		"format":  "mockd",
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["format"] != "mockd" {
		t.Errorf("format = %v, want mockd", parsed["format"])
	}
	if parsed["imported"] != float64(1) {
		t.Errorf("imported = %v, want 1", parsed["imported"])
	}
}

func TestHandleImportMocks_ImportSuccess(t *testing.T) {
	t.Parallel()

	var capturedReplace bool
	var capturedMockCount int
	client := &mockAdminClient{
		importConfigFn: func(collection *config.MockCollection, replace bool) (*cli.ImportResult, error) {
			capturedReplace = replace
			capturedMockCount = len(collection.Mocks)
			return &cli.ImportResult{
				Imported: capturedMockCount,
				Message:  "imported successfully",
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if capturedReplace {
		t.Error("replace should be false by default")
	}
	if capturedMockCount != 1 {
		t.Errorf("capturedMockCount = %d, want 1", capturedMockCount)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["imported"] != float64(1) {
		t.Errorf("imported = %v, want 1", parsed["imported"])
	}
	if parsed["replaced"] != false {
		t.Errorf("replaced = %v, want false", parsed["replaced"])
	}
	if parsed["message"] != "imported successfully" {
		t.Errorf("message = %v, want 'imported successfully'", parsed["message"])
	}
}

func TestHandleImportMocks_ImportReplace(t *testing.T) {
	t.Parallel()

	var capturedReplace bool
	client := &mockAdminClient{
		importConfigFn: func(collection *config.MockCollection, replace bool) (*cli.ImportResult, error) {
			capturedReplace = replace
			return &cli.ImportResult{Imported: len(collection.Mocks)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
		"replace": true,
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if !capturedReplace {
		t.Error("replace should be true when explicitly set")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["replaced"] != true {
		t.Errorf("replaced = %v, want true", parsed["replaced"])
	}
}

func TestHandleImportMocks_EmptyContent(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": "",
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for empty content")
	}

	text := resultText(t, result)
	if text != "content is required" {
		t.Errorf("error text = %q, want %q", text, "content is required")
	}
}

func TestHandleImportMocks_InvalidFormat(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
		"format":  "bogus",
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid format")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "unsupported format") {
		t.Errorf("error text = %q, want to contain 'unsupported format'", text)
	}
}

func TestHandleImportMocks_ParseError(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	// Valid format, but content that cannot be parsed as mockd YAML
	args := map[string]interface{}{
		"content": "this: is not a valid mockd config\n  broken yaml: [[[",
		"format":  "mockd",
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for parse error")
	}

	text := resultText(t, result)
	// Should either be a parse error or "no mocks found" since content has no mocks array
	if !strings.Contains(text, "import parse error") && !strings.Contains(text, "no mocks found") {
		t.Errorf("error text = %q, want to contain 'import parse error' or 'no mocks found'", text)
	}
}

func TestHandleImportMocks_ImportFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		importConfigFn: func(collection *config.MockCollection, replace bool) (*cli.ImportResult, error) {
			return nil, &cli.APIError{StatusCode: 500, Message: "import rejected"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"content": validMockdYAML,
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for import failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to apply import") {
		t.Errorf("error text = %q, want to contain 'failed to apply import'", text)
	}
}

func TestHandleImportMocks_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{
		"content": validMockdYAML,
	}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleImportMocks_MissingContentArg(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	// No "content" key at all
	args := map[string]interface{}{}
	result, err := handleImportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleImportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing content")
	}

	text := resultText(t, result)
	if text != "content is required" {
		t.Errorf("error text = %q, want %q", text, "content is required")
	}
}

// =============================================================================
// handleExportMocks Tests
// =============================================================================

func TestHandleExportMocks_YAML(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		exportConfigFn: func(name string) (*config.MockCollection, error) {
			return &config.MockCollection{
				Mocks: []*mock.Mock{
					{
						ID:      "http_abc",
						Type:    mock.TypeHTTP,
						Enabled: &enabled,
						HTTP: &mock.HTTPSpec{
							Matcher: &mock.HTTPMatcher{
								Method: "GET",
								Path:   "/api/test",
							},
							Response: &mock.HTTPResponse{
								StatusCode: 200,
							},
						},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"format": "yaml",
	}
	result, err := handleExportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleExportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if result.Content[0].MimeType != "application/yaml" {
		t.Errorf("mimeType = %s, want application/yaml", result.Content[0].MimeType)
	}
	text := result.Content[0].Text
	if text == "" {
		t.Error("expected non-empty export content")
	}
	// YAML should contain typical markers
	if !strings.Contains(text, "/api/test") {
		t.Errorf("exported YAML does not contain expected path /api/test")
	}
}

func TestHandleExportMocks_JSON(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		exportConfigFn: func(name string) (*config.MockCollection, error) {
			return &config.MockCollection{
				Mocks: []*mock.Mock{
					{
						ID:      "http_abc",
						Type:    mock.TypeHTTP,
						Enabled: &enabled,
						HTTP: &mock.HTTPSpec{
							Matcher: &mock.HTTPMatcher{
								Method: "GET",
								Path:   "/api/test",
							},
							Response: &mock.HTTPResponse{
								StatusCode: 200,
							},
						},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"format": "json",
	}
	result, err := handleExportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleExportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if result.Content[0].MimeType != "application/json" {
		t.Errorf("mimeType = %s, want application/json", result.Content[0].MimeType)
	}
	text := result.Content[0].Text
	if text == "" {
		t.Error("expected non-empty export content")
	}
	// Should be valid JSON
	if !json.Valid([]byte(text)) {
		t.Errorf("exported content is not valid JSON: %s", text[:min(100, len(text))])
	}
}

func TestHandleExportMocks_NilCollection(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		exportConfigFn: func(name string) (*config.MockCollection, error) {
			return nil, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleExportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleExportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for nil collection")
	}

	text := resultText(t, result)
	if text != "no configuration to export" {
		t.Errorf("error text = %q, want %q", text, "no configuration to export")
	}
}

func TestHandleExportMocks_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		exportConfigFn: func(name string) (*config.MockCollection, error) {
			return nil, &cli.APIError{StatusCode: 500, Message: "export failed"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleExportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleExportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for export failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to export mocks") {
		t.Errorf("error text = %q, want to contain 'failed to export mocks'", text)
	}
}

func TestHandleExportMocks_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{}
	result, err := handleExportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleExportMocks() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleExportMocks_DefaultYAML(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		exportConfigFn: func(name string) (*config.MockCollection, error) {
			return &config.MockCollection{
				Mocks: []*mock.Mock{
					{
						ID:      "http_abc",
						Type:    mock.TypeHTTP,
						Enabled: &enabled,
						HTTP: &mock.HTTPSpec{
							Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/default"},
							Response: &mock.HTTPResponse{StatusCode: 200},
						},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	// No format specified — should default to YAML
	args := map[string]interface{}{}
	result, err := handleExportMocks(args, session, server)
	if err != nil {
		t.Fatalf("handleExportMocks() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if result.Content[0].MimeType != "application/yaml" {
		t.Errorf("default mimeType = %s, want application/yaml", result.Content[0].MimeType)
	}
}
