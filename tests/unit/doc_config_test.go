package unit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDocConfigExamples extracts YAML and JSON config blocks from documentation
// files that look like full config files (contain `mocks:` with entries) and
// validates structural correctness of mock definitions.
//
// Note: `id` and `type` fields are OPTIONAL — the config loader auto-generates
// IDs and infers types from spec fields (DX-1). This test only checks structure.
//
// This catches bugs like:
//   - Config examples using wrong field names (e.g., `request:` instead of `http.matcher:`)
//   - Config examples using bare `matcher`/`response` at mock level instead of nested under `http:`
//   - Config examples using `response.status` instead of `response.statusCode`
//
// Only blocks containing a `mocks:` key with actual entries are tested.
// Fragment examples (matcher-only, response-only) are skipped.
func TestDocConfigExamples(t *testing.T) {
	docsRoot := filepath.Join("..", "..", "docs", "src", "content", "docs")

	if _, err := os.Stat(docsRoot); os.IsNotExist(err) {
		t.Skipf("docs directory not found at %s (run from repo root)", docsRoot)
	}

	var failures []string

	err := filepath.Walk(docsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(docsRoot, path)
		blocks := extractMockBlocks(string(data))

		for _, block := range blocks {
			if errs := validateMockStructure(block); len(errs) > 0 {
				failures = append(failures, formatBlockFailure(relPath, block, errs))
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk docs directory: %v", err)
	}

	if len(failures) > 0 {
		t.Errorf("found invalid mock config examples in docs:\n\n%s",
			strings.Join(failures, "\n"))
	}
}

// codeBlockRe matches fenced code blocks with optional language identifier.
var codeBlockRe = regexp.MustCompile("(?s)```(?:ya?ml|json)?\\s*\\n(.*?)```")

type docBlock struct {
	content string
	line    int
}

// extractMockBlocks finds code blocks containing mocks: with actual mock entries.
func extractMockBlocks(markdown string) []docBlock {
	var blocks []docBlock

	matches := codeBlockRe.FindAllStringSubmatchIndex(markdown, -1)
	for _, loc := range matches {
		content := markdown[loc[2]:loc[3]]
		trimmed := strings.TrimSpace(content)

		// Only process blocks that contain mock definitions (not just
		// statefulResources, server config, fragments, etc.)
		if !containsMockEntries(trimmed) {
			continue
		}

		lineNum := strings.Count(markdown[:loc[0]], "\n") + 1
		blocks = append(blocks, docBlock{content: content, line: lineNum})
	}

	return blocks
}

// containsMockEntries checks if a code block has a `mocks:` key with actual
// entries (not just `mocks: []` or `mocks: [...]` placeholder).
func containsMockEntries(content string) bool {
	// YAML: look for mocks: followed by entries with - id: or - type:
	if strings.Contains(content, "mocks:") {
		// Skip placeholder patterns
		if strings.Contains(content, "mocks: []") ||
			strings.Contains(content, "mocks: [...]") ||
			strings.Contains(content, `"mocks": []`) ||
			strings.Contains(content, `"mocks": [...]`) {
			return false
		}

		// Check for actual mock entries — must have at least one entry
		// with id, type, protocol spec, or legacy matcher/request field
		return strings.Contains(content, "- id:") ||
			strings.Contains(content, "- type:") ||
			strings.Contains(content, "- http:") ||
			strings.Contains(content, "- graphql:") ||
			strings.Contains(content, "- grpc:") ||
			strings.Contains(content, "- websocket:") ||
			strings.Contains(content, "- soap:") ||
			strings.Contains(content, "- mqtt:") ||
			strings.Contains(content, "- oauth:") ||
			strings.Contains(content, `"id"`) ||
			strings.Contains(content, `"type"`) ||
			strings.Contains(content, `"matcher"`) ||
			strings.Contains(content, "- request:")
	}
	return false
}

// configFile represents the top-level structure we're checking.
type configFile struct {
	Mocks []map[string]interface{} `json:"mocks" yaml:"mocks"`
}

// validateMockStructure checks that mocks in a config block have correct structure.
func validateMockStructure(block docBlock) []string {
	content := strings.TrimSpace(block.content)
	var errs []string

	var cfg configFile

	// Try JSON first
	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		if err := json.Unmarshal([]byte(content), &cfg); err != nil {
			// Can't parse, skip (might be pseudo-code or a different format)
			return nil
		}
	} else {
		if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
			return nil
		}
	}

	if len(cfg.Mocks) == 0 {
		return nil
	}

	for i, mock := range cfg.Mocks {
		mockNum := i + 1

		// Skip ellipsis/placeholder entries like { ... }
		if isPlaceholder(mock) {
			continue
		}

		// Note: id and type are OPTIONAL — auto-generated by config loader (DX-1).
		// We only check structural correctness here.

		// For HTTP mocks (explicit type), check structure
		mockType, hasType := mock["type"]
		if hasType && fmt.Sprint(mockType) == "http" {
			if _, hasHTTP := mock["http"]; !hasHTTP {
				errs = append(errs, fmt.Sprintf("mock #%d: HTTP mock missing 'http' block (matcher/response should be nested under http:)", mockNum))
			}
		}

		// Check for wrong patterns — these should NOT exist at mock level
		if _, has := mock["request"]; has {
			errs = append(errs, fmt.Sprintf("mock #%d: uses 'request' at mock level (should be http.matcher)", mockNum))
		}
		if _, has := mock["matcher"]; has {
			errs = append(errs, fmt.Sprintf("mock #%d: uses 'matcher' at mock level (should be nested under http:)", mockNum))
		}
		if _, has := mock["response"]; has {
			errs = append(errs, fmt.Sprintf("mock #%d: uses 'response' at mock level (should be nested under http:)", mockNum))
		}
	}

	return errs
}

// isPlaceholder checks if a mock entry is a placeholder like { ... } or has ellipsis values.
func isPlaceholder(m map[string]interface{}) bool {
	for _, v := range m {
		if s, ok := v.(string); ok && (s == "..." || s == "{ ... }") {
			return true
		}
	}
	return len(m) == 0
}

func formatBlockFailure(file string, block docBlock, errs []string) string {
	preview := block.content
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	return strings.Join([]string{
		"  ─────────────────────────────────────",
		fmt.Sprintf("  File:  %s (line ~%d)", file, block.line),
		"  Errors:",
		"    - " + strings.Join(errs, "\n    - "),
		"  Preview:",
		"    " + strings.ReplaceAll(strings.TrimSpace(preview), "\n", "\n    "),
		"",
	}, "\n")
}
