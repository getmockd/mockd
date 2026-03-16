package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/portability"
)

// processImports resolves and loads all import entries in a MockCollection.
// For each import, it reads the file or fetches the URL, auto-detects the format,
// runs the importer, applies the namespace to operationIds, and merges the resulting
// mocks into the collection.
//
// configDir is the directory of the config file, used to resolve relative paths.
func processImports(collection *config.MockCollection, configDir string) error {
	if len(collection.Imports) == 0 {
		return nil
	}

	for _, imp := range collection.Imports {
		data, filename, err := readImportSource(imp, configDir)
		if err != nil {
			return fmt.Errorf("import %s: %w", importRef(imp), err)
		}

		// Determine format
		var format portability.Format
		if imp.Format != "" {
			format = portability.ParseFormat(imp.Format)
			if format == portability.FormatUnknown {
				return fmt.Errorf("import %s: unknown format %q", importRef(imp), imp.Format)
			}
		} else {
			format = portability.DetectFormat(data, filename)
			if format == portability.FormatUnknown {
				return fmt.Errorf("import %s: could not detect format", importRef(imp))
			}
		}

		// Run importer
		importer := portability.GetImporter(format)
		if importer == nil {
			return fmt.Errorf("import %s: no importer for format %s", importRef(imp), format)
		}

		imported, err := importer.Import(data)
		if err != nil {
			return fmt.Errorf("import %s: %w", importRef(imp), err)
		}

		// Apply namespace to operationIds
		if imp.As != "" {
			applyNamespace(imported, imp.As)
		}

		// Merge imported mocks into the collection
		collection.Mocks = append(collection.Mocks, imported.Mocks...)
	}

	return nil
}

// readImportSource reads import data from a local file or remote URL.
func readImportSource(imp *config.ImportEntry, configDir string) ([]byte, string, error) {
	if imp.Path != "" {
		path := imp.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read file %s: %w", path, err)
		}
		return data, filepath.Base(path), nil
	}

	if imp.URL != "" {
		data, err := fetchURL(imp.URL)
		if err != nil {
			return nil, "", err
		}
		// Extract filename from URL for format detection
		parts := strings.Split(imp.URL, "/")
		filename := parts[len(parts)-1]
		return data, filename, nil
	}

	return nil, "", errors.New("import entry must have 'path' or 'url'")
}

// applyNamespace prefixes all operationIds in imported mocks with the namespace.
// "PostCustomers" becomes "stripe.PostCustomers" when namespace is "stripe".
func applyNamespace(collection *config.MockCollection, namespace string) {
	for _, m := range collection.Mocks {
		if m.OperationID != "" {
			m.OperationID = namespace + "." + m.OperationID
		}
	}
}

// processTablesAndExtend converts tables to stateful resources and resolves
// extend bindings onto matched mocks. Must be called after processImports
// (so imported mocks are available for binding resolution).
//
// Steps:
//  1. Convert each TableConfig → StatefulResourceConfig and append to
//     collection.StatefulResources. Tables have no basePath because routing
//     is done via StatefulBinding on the mock, not via MatchPath.
//  2. For each ExtendBinding, find the target mock by OperationID (exact match)
//     or by "METHOD /path" fallback. Set mock.HTTP.StatefulBinding.
//  3. Resolve response transforms: binding.Response > table.Response > nil.
func processTablesAndExtend(collection *config.MockCollection) error {
	// Step 1: Convert tables to stateful resources
	if len(collection.Tables) > 0 {
		tableMap := make(map[string]*config.TableConfig, len(collection.Tables))
		for _, table := range collection.Tables {
			if table.Name == "" {
				return errors.New("table: name is required")
			}
			if _, exists := tableMap[table.Name]; exists {
				return fmt.Errorf("table %q: duplicate name", table.Name)
			}
			tableMap[table.Name] = table

			// Convert TableConfig → StatefulResourceConfig (no basePath — bridge-only)
			res := &config.StatefulResourceConfig{
				Name:          table.Name,
				IDField:       table.IDField,
				IDStrategy:    table.IDStrategy,
				IDPrefix:      table.IDPrefix,
				ParentField:   table.ParentField,
				MaxItems:      table.MaxItems,
				SeedData:      table.SeedData,
				Response:      table.Response,
				Relationships: table.Relationships,
			}
			collection.StatefulResources = append(collection.StatefulResources, res)
		}

		// Step 2: Resolve extend bindings
		if len(collection.Extend) > 0 {
			// Build indexes for mock lookup
			opIDIndex := make(map[string]*mock.Mock, len(collection.Mocks))
			methodPathIndex := make(map[string]*mock.Mock, len(collection.Mocks))
			for _, m := range collection.Mocks {
				if m.OperationID != "" {
					opIDIndex[m.OperationID] = m
				}
				if m.HTTP != nil && m.HTTP.Matcher != nil {
					method := strings.ToUpper(m.HTTP.Matcher.Method)
					path := m.HTTP.Matcher.Path
					if method != "" && path != "" {
						methodPathIndex[method+" "+path] = m
					}
				}
			}

			for _, binding := range collection.Extend {
				if binding.Mock == "" {
					return errors.New("extend: mock reference is required")
				}
				if binding.Table == "" {
					return fmt.Errorf("extend %q: table is required", binding.Mock)
				}
				if binding.Action == "" {
					return fmt.Errorf("extend %q: action is required", binding.Mock)
				}

				// Validate table reference
				table, ok := tableMap[binding.Table]
				if !ok {
					return fmt.Errorf("extend %q: table %q not found", binding.Mock, binding.Table)
				}

				// Find the target mock: operationId first, then "METHOD /path"
				target := opIDIndex[binding.Mock]
				if target == nil {
					target = methodPathIndex[binding.Mock]
				}
				if target == nil {
					return fmt.Errorf("extend %q: mock not found (checked operationId and METHOD /path)", binding.Mock)
				}

				// Ensure mock has HTTP spec
				if target.HTTP == nil {
					return fmt.Errorf("extend %q: mock is not an HTTP mock", binding.Mock)
				}

				// Validate custom action has operation name
				if binding.Action == "custom" && binding.Operation == "" {
					return fmt.Errorf("extend %q: action 'custom' requires an operation name", binding.Mock)
				}

				// Clear conflicting response types — the mock may have a static
				// Response from an OpenAPI import, but extend replaces it with
				// a stateful binding.
				target.HTTP.ClearConflictingResponseTypes()

				// Set StatefulBinding on the mock
				target.HTTP.StatefulBinding = &mock.StatefulBinding{
					Table:     binding.Table,
					Action:    binding.Action,
					Operation: binding.Operation,
				}

				// Resolve response transform: binding override > table default > nil
				var responseTransform *config.ResponseTransform
				if binding.Response != nil {
					responseTransform = binding.Response
				} else if table.Response != nil {
					responseTransform = table.Response
				}
				if responseTransform != nil {
					target.HTTP.StatefulBinding.Response = &mock.StatefulBindingResponse{
						Transform: responseTransform,
					}
				}
			}
		}
	}

	return nil
}

// importRef returns a human-readable reference for error messages.
func importRef(imp *config.ImportEntry) string {
	if imp.Path != "" {
		return imp.Path
	}
	return imp.URL
}
