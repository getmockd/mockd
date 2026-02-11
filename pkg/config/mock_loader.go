// Package config provides configuration types for mockd.
// This file contains utilities for loading mocks from file references and globs.

package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// MockFileContent represents the possible contents of a mock file.
// A mock file can contain either a single mock or an array of mocks.
type MockFileContent struct {
	// Single mock fields (when file contains a single mock definition)
	ID        string          `yaml:"id,omitempty"`
	Workspace string          `yaml:"workspace,omitempty"`
	Type      string          `yaml:"type,omitempty"`
	HTTP      *HTTPMockConfig `yaml:"http,omitempty"`

	// For detecting array format - populated by custom unmarshaling
	Mocks []MockFileContent `yaml:"-"`
}

// UnmarshalYAML implements custom YAML unmarshaling to handle both single mock
// and array of mocks formats.
func (m *MockFileContent) UnmarshalYAML(node *yaml.Node) error {
	// Check if it's an array
	if node.Kind == yaml.SequenceNode {
		var mocks []MockFileContent
		if err := node.Decode(&mocks); err != nil {
			return err
		}
		m.Mocks = mocks
		return nil
	}

	// Otherwise, unmarshal as a single mock using an alias to avoid recursion
	type MockFileContentAlias MockFileContent
	alias := (*MockFileContentAlias)(m)
	return node.Decode(alias)
}

// ToMockEntry converts a MockFileContent to a MockEntry (inline mock format).
func (m *MockFileContent) ToMockEntry() MockEntry {
	return MockEntry{
		ID:        m.ID,
		Workspace: m.Workspace,
		Type:      m.Type,
		HTTP:      m.HTTP,
	}
}

// LoadMocksFromEntry loads mock configurations from a MockEntry.
// For inline mocks, it returns the entry as-is.
// For file references, it loads and parses the YAML file.
// For globs, it expands the pattern and loads all matching files.
// The baseDir is used to resolve relative paths.
func LoadMocksFromEntry(entry MockEntry, baseDir string) ([]MockEntry, error) {
	switch {
	case entry.IsInline():
		return []MockEntry{entry}, nil
	case entry.IsFileRef():
		return loadMocksFromFile(entry.File, baseDir)
	case entry.IsGlob():
		return loadMocksFromGlob(entry.Files, baseDir)
	default:
		return nil, errors.New("invalid mock entry: no id, file, or files specified")
	}
}

// LoadAllMocks loads all mock configurations from a slice of MockEntry.
// It expands file references and globs, returning a flat slice of inline mocks.
func LoadAllMocks(entries []MockEntry, baseDir string) ([]MockEntry, error) {
	var result []MockEntry

	for i, entry := range entries {
		mocks, err := LoadMocksFromEntry(entry, baseDir)
		if err != nil {
			// Provide context about which entry failed
			if entry.IsFileRef() {
				return nil, fmt.Errorf("mocks[%d] (file: %s): %w", i, entry.File, err)
			}
			if entry.IsGlob() {
				return nil, fmt.Errorf("mocks[%d] (files: %s): %w", i, entry.Files, err)
			}
			return nil, fmt.Errorf("mocks[%d]: %w", i, err)
		}
		result = append(result, mocks...)
	}

	return result, nil
}

// loadMocksFromFile loads mocks from a single YAML file.
func loadMocksFromFile(filePath, baseDir string) ([]MockEntry, error) {
	// Resolve the path relative to baseDir
	resolvedPath := ResolvePath(baseDir, filePath)

	// Read file
	file, err := os.Open(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", resolvedPath)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied: %s", resolvedPath)
		}
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", resolvedPath)
	}

	// Apply environment variable expansion
	expanded := ExpandEnvVars(string(data))

	// Parse YAML - handle both single mock and array of mocks
	var content MockFileContent
	if err := yaml.Unmarshal([]byte(expanded), &content); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	// Convert to MockEntry slice
	if len(content.Mocks) > 0 {
		// Array format
		entries := make([]MockEntry, len(content.Mocks))
		for i, m := range content.Mocks {
			entries[i] = m.ToMockEntry()
		}
		return entries, nil
	}

	// Single mock format - check if it has valid content
	if content.ID == "" && content.Type == "" {
		return nil, fmt.Errorf("invalid mock file: missing 'id' or 'type' field: %s", resolvedPath)
	}

	return []MockEntry{content.ToMockEntry()}, nil
}

// loadMocksFromGlob loads mocks from files matching a glob pattern.
// Supports ** for recursive directory matching via doublestar library.
func loadMocksFromGlob(pattern, baseDir string) ([]MockEntry, error) {
	// Resolve the pattern relative to baseDir
	resolvedPattern := ResolvePath(baseDir, pattern)

	// Use doublestar for ** support
	matches, err := expandGlob(resolvedPattern)
	if err != nil {
		return nil, fmt.Errorf("expanding glob pattern: %w", err)
	}

	if len(matches) == 0 {
		// Not an error, just no matches
		return []MockEntry{}, nil
	}

	// Sort matches for deterministic ordering
	sort.Strings(matches)

	var result []MockEntry
	for _, match := range matches {
		// Calculate relative path from baseDir for error messages
		relPath, _ := filepath.Rel(baseDir, match)
		if relPath == "" {
			relPath = match
		}

		mocks, err := loadMocksFromFile(match, "")
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", relPath, err)
		}
		result = append(result, mocks...)
	}

	return result, nil
}

// expandGlob expands a glob pattern to a list of matching file paths.
// Uses doublestar for ** support, falls back to filepath.Glob for simple patterns.
func expandGlob(pattern string) ([]string, error) {
	// Check if pattern contains ** for recursive matching
	if strings.Contains(pattern, "**") {
		// Use doublestar for ** support
		// FilepathGlob returns matches using the OS path separator
		return doublestar.FilepathGlob(pattern)
	}

	// Use standard filepath.Glob for simple patterns
	return filepath.Glob(pattern)
}

// GetMockFileBaseDir returns the directory to use as baseDir for resolving
// mock file references. This is typically the directory containing the
// mockd.yaml config file.
func GetMockFileBaseDir(configPath string) string {
	if configPath == "" {
		// Fall back to current working directory
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
		return "."
	}
	return filepath.Dir(configPath)
}
