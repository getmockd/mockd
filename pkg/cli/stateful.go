package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	statefulAddPath    string
	statefulAddIDField string
	statefulListLimit  int
	statefulListOffset int
	statefulListSort   string
	statefulListOrder  string

	// Custom operation flags
	customAddFile                string
	customAddDefinition          string
	customValidateFile           string
	customValidateDefinition     string
	customValidateInput          string
	customValidateInputFile      string
	customValidateFixturesFile   string
	customValidateCheckResources bool
	customValidateRuntimeCheck   bool
	customValidateStrict         bool
	customRunInput               string
	customRunInputFile           string
)

var statefulCmd = &cobra.Command{
	Use:   "stateful",
	Short: "Manage stateful CRUD resources",
	Long: `Manage stateful CRUD resources.

Stateful resources provide in-memory CRUD data stores that can be shared
across protocols. A resource created here is accessible via HTTP REST
endpoints, SOAP operations, GraphQL resolvers, and any protocol that
supports the stateful bridge.

Examples:
  mockd stateful list                          # List all stateful resources
  mockd stateful add users --path /api/users   # Create with HTTP REST endpoints
  mockd stateful add products                  # Bridge-only (no HTTP endpoints)
  mockd stateful reset users                   # Reset resource to seed data`,
}

var statefulAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a stateful CRUD resource",
	Long: `Create a new stateful CRUD resource.

By default, a resource is created as "bridge-only" — accessible only via
protocol integrations (SOAP, GraphQL, gRPC, etc.) but without HTTP REST
endpoints.

Use --path to also expose HTTP REST endpoints:
  GET    <path>        — List all items
  POST   <path>        — Create an item
  GET    <path>/{id}   — Get an item by ID
  PUT    <path>/{id}   — Update an item
  DELETE <path>/{id}   — Delete an item

Examples:
  mockd stateful add users --path /api/users
  mockd stateful add products
  mockd stateful add orders --path /api/orders --id-field orderId`,
	Args: cobra.ExactArgs(1),
	RunE: runStatefulAdd,
}

var statefulListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stateful resources",
	Long: `List all stateful resources and their item counts.

Examples:
  mockd stateful list
  mockd stateful list --json`,
	RunE: runStatefulList,
}

var statefulResetCmd = &cobra.Command{
	Use:   "reset <name>",
	Short: "Reset a stateful resource to seed data",
	Long: `Reset a stateful resource to its initial seed data state.

All current items are removed and replaced with the original seed data
(if any was configured). This is useful for test cleanup between runs.

Examples:
  mockd stateful reset users
  mockd stateful reset products --json`,
	Args: cobra.ExactArgs(1),
	RunE: runStatefulReset,
}

// --- Custom operation commands ---

var customCmd = &cobra.Command{
	Use:   "custom",
	Short: "Manage custom operations on stateful resources",
	Long: `Manage custom multi-step operations that run against stateful resources.

Custom operations define a pipeline of steps (read, create, update, delete, set)
with expression-based logic. They can be invoked via CLI, REST API, SOAP, GraphQL,
or any protocol that supports the stateful bridge.

Examples:
  mockd stateful custom list
  mockd stateful custom get TransferFunds
  mockd stateful custom add --file transfer.yaml
  mockd stateful custom add --definition '{"name":"TransferFunds","steps":[...]}'
  mockd stateful custom run TransferFunds --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'
  mockd stateful custom delete TransferFunds`,
}

var customListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered custom operations",
	RunE:  runCustomList,
}

var customGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show details of a custom operation",
	Args:  cobra.ExactArgs(1),
	RunE:  runCustomGet,
}

var customAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Register a new custom operation",
	Long: `Register a new custom operation from a file or inline definition.

The definition must include a "name" field and at least one step.

From a file:
  mockd stateful custom add --file transfer.yaml

Inline JSON:
  mockd stateful custom add --definition '{"name":"TransferFunds","steps":[{"type":"read","resource":"accounts","id":"input.sourceId","as":"source"}],"response":{"status":"\"done\""}}'

The definition format:
  name: TransferFunds
  steps:
    - type: read
      resource: accounts
      id: "input.sourceId"
      as: source
    - type: update
      resource: accounts
      id: "input.sourceId"
      set:
        balance: "source.balance - input.amount"
  response:
    status: '"completed"'`,
	RunE: runCustomAdd,
}

var customValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a custom operation definition (no writes)",
	Long: `Validate a custom operation definition locally before registering it.

Checks include:
  - Required fields and step shape
  - Supported consistency mode (best_effort, atomic)
  - Expression compile checks for step IDs, set/value fields, and response expressions
  - Optional live resource existence checks against the running admin API
  - Optional strict mode (fails on warnings)

Examples:
  mockd stateful custom validate --file transfer.yaml
  mockd stateful custom validate --file transfer.yaml --input '{"sourceId":"acc-1","destId":"acc-2","amount":100}'
  mockd stateful custom validate --file transfer.yaml --input '{"sourceId":"acc-1"}' --check-expressions-runtime --fixtures-file fixtures.json
  mockd stateful custom validate --file transfer.yaml --check-resources`,
	RunE: runCustomValidate,
}

var customRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Execute a custom operation",
	Long: `Execute a registered custom operation with the given input.

Examples:
  mockd stateful custom run TransferFunds --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'
  mockd stateful custom run TransferFunds --input-file transfer-input.json
  mockd stateful custom run TransferFunds   # no input (empty map)`,
	Args: cobra.ExactArgs(1),
	RunE: runCustomRun,
}

var customDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a custom operation",
	Args:  cobra.ExactArgs(1),
	RunE:  runCustomDelete,
}

func init() {
	rootCmd.AddCommand(statefulCmd)

	statefulCmd.AddCommand(statefulAddCmd)
	statefulAddCmd.Flags().StringVar(&statefulAddPath, "path", "", "URL base path for HTTP REST endpoints (omit for bridge-only)")
	statefulAddCmd.Flags().StringVar(&statefulAddIDField, "id-field", "", "Custom ID field name (default: id)")

	statefulCmd.AddCommand(statefulListCmd)
	statefulListCmd.Flags().IntVar(&statefulListLimit, "limit", 100, "Maximum items to show per resource")
	statefulListCmd.Flags().IntVar(&statefulListOffset, "offset", 0, "Skip this many items")
	statefulListCmd.Flags().StringVar(&statefulListSort, "sort", "", "Sort field")
	statefulListCmd.Flags().StringVar(&statefulListOrder, "order", "", "Sort order (asc or desc)")

	statefulCmd.AddCommand(statefulResetCmd)

	// Custom operation subcommands
	statefulCmd.AddCommand(customCmd)
	customCmd.AddCommand(customListCmd)
	customCmd.AddCommand(customGetCmd)

	customCmd.AddCommand(customAddCmd)
	customAddCmd.Flags().StringVar(&customAddFile, "file", "", "Path to YAML/JSON file containing the operation definition")
	customAddCmd.Flags().StringVar(&customAddDefinition, "definition", "", "Inline JSON operation definition")

	customCmd.AddCommand(customValidateCmd)
	customValidateCmd.Flags().StringVar(&customValidateFile, "file", "", "Path to YAML/JSON file containing the operation definition")
	customValidateCmd.Flags().StringVar(&customValidateDefinition, "definition", "", "Inline JSON operation definition")
	customValidateCmd.Flags().StringVar(&customValidateInput, "input", "", "Inline JSON input example for expression compile checks")
	customValidateCmd.Flags().StringVar(&customValidateInputFile, "input-file", "", "Path to JSON file containing input example")
	customValidateCmd.Flags().StringVar(&customValidateFixturesFile, "fixtures-file", "", "Path to JSON/YAML fixtures file used by runtime expression checks")
	customValidateCmd.Flags().BoolVar(&customValidateCheckResources, "check-resources", false, "Verify referenced stateful resources exist on the running admin/engine")
	customValidateCmd.Flags().BoolVar(&customValidateRuntimeCheck, "check-expressions-runtime", false, "Evaluate expressions with sample input/fixtures (no writes)")
	customValidateCmd.Flags().BoolVar(&customValidateStrict, "strict", false, "Treat validation warnings as errors")

	customCmd.AddCommand(customRunCmd)
	customRunCmd.Flags().StringVar(&customRunInput, "input", "", "Inline JSON input for the operation")
	customRunCmd.Flags().StringVar(&customRunInputFile, "input-file", "", "Path to JSON file containing operation input")

	customCmd.AddCommand(customDeleteCmd)
}

type customValidationResult struct {
	Valid               bool     `json:"valid"`
	Name                string   `json:"name"`
	Steps               int      `json:"steps"`
	Consistency         string   `json:"consistency"`
	RuntimeChecked      bool     `json:"runtimeChecked,omitempty"`
	CheckedResources    bool     `json:"checkedResources,omitempty"`
	ReferencedResources []string `json:"referencedResources,omitempty"`
	MissingResources    []string `json:"missingResources,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	Message             string   `json:"message,omitempty"`
}

type customValidationFixtures struct {
	Vars      map[string]interface{}                       `json:"vars" yaml:"vars"`
	Resources map[string]map[string]map[string]interface{} `json:"resources" yaml:"resources"`
}

// runStatefulAdd creates a new stateful CRUD resource.
func runStatefulAdd(_ *cobra.Command, args []string) error {
	name := args[0]

	// Validate name
	if strings.TrimSpace(name) == "" {
		return errors.New("resource name cannot be empty")
	}

	// Build the stateful resource config
	resourceCfg := &config.StatefulResourceConfig{
		Name:     name,
		BasePath: statefulAddPath,
	}
	if statefulAddIDField != "" {
		resourceCfg.IDField = statefulAddIDField
	}

	// Wrap in a MockCollection and import (same pattern as runAddStateful in add.go)
	collection := &config.MockCollection{
		Version: "1.0",
		Mocks:   []*config.MockConfiguration{},
		StatefulResources: []*config.StatefulResourceConfig{
			resourceCfg,
		},
	}

	client := NewAdminClientWithAuth(adminURL)
	_, err := client.ImportConfig(collection, false)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	bridgeOnly := statefulAddPath == ""

	if jsonOutput {
		result := struct {
			Resource  string   `json:"resource"`
			BasePath  string   `json:"basePath,omitempty"`
			IDField   string   `json:"idField,omitempty"`
			Action    string   `json:"action"`
			Mode      string   `json:"mode"`
			Endpoints []string `json:"endpoints,omitempty"`
		}{
			Resource: name,
			BasePath: statefulAddPath,
			Action:   "created",
		}

		if statefulAddIDField != "" {
			result.IDField = statefulAddIDField
		}

		if bridgeOnly {
			result.Mode = "bridge-only"
		} else {
			result.Mode = "http+bridge"
			result.Endpoints = []string{
				"GET    " + statefulAddPath,
				"POST   " + statefulAddPath,
				"GET    " + statefulAddPath + "/{id}",
				"PUT    " + statefulAddPath + "/{id}",
				"DELETE " + statefulAddPath + "/{id}",
			}
		}

		return output.JSON(result)
	}

	fmt.Printf("Created stateful resource: %s\n", name)

	if bridgeOnly {
		fmt.Printf("  Mode: bridge-only (no HTTP endpoints)\n")
		fmt.Printf("  Access via: SOAP, GraphQL, gRPC, or other protocol integrations\n")
	} else {
		fmt.Printf("  Base path: %s\n", statefulAddPath)
		fmt.Printf("  Endpoints:\n")
		fmt.Printf("    GET    %s        — List all %s\n", statefulAddPath, name)
		fmt.Printf("    POST   %s        — Create a %s\n", statefulAddPath, singularize(name))
		fmt.Printf("    GET    %s/{id}   — Get a %s by ID\n", statefulAddPath, singularize(name))
		fmt.Printf("    PUT    %s/{id}   — Update a %s\n", statefulAddPath, singularize(name))
		fmt.Printf("    DELETE %s/{id}   — Delete a %s\n", statefulAddPath, singularize(name))
	}

	if statefulAddIDField != "" {
		fmt.Printf("  ID field: %s\n", statefulAddIDField)
	}

	return nil
}

// runStatefulList lists all stateful resources.
func runStatefulList(_ *cobra.Command, _ []string) error {
	client := NewAdminClientWithAuth(adminURL)
	overview, err := client.GetStateOverview()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	if overview == nil || overview.Total == 0 {
		printResult(struct {
			Resources []interface{} `json:"resources"`
			Total     int           `json:"total"`
		}{
			Resources: []interface{}{},
			Total:     0,
		}, func() {
			fmt.Println("No stateful resources configured.")
			fmt.Println("\nCreate one with: mockd stateful add <name> [--path /api/<name>]")
		})
		return nil
	}

	printList(overview, func() {
		fmt.Printf("Stateful Resources (%d):\n\n", overview.Total)
		tw := output.Table()
		_, _ = fmt.Fprintf(tw, "NAME\tBASE PATH\tITEMS\tSEED\tID FIELD\n")
		_, _ = fmt.Fprintf(tw, "----\t---------\t-----\t----\t--------\n")
		for _, r := range overview.Resources {
			basePath := r.BasePath
			if basePath == "" {
				basePath = "(bridge-only)"
			}
			idField := r.IDField
			if idField == "" {
				idField = "id"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n",
				r.Name, basePath, r.ItemCount, r.SeedCount, idField)
		}
		_ = tw.Flush()

		fmt.Printf("\nTotal items across all resources: %d\n", overview.TotalItems)
	})

	return nil
}

// runStatefulReset resets a stateful resource to its seed data.
func runStatefulReset(_ *cobra.Command, args []string) error {
	name := args[0]

	client := NewAdminClientWithAuth(adminURL)
	err := client.ResetStatefulResource(name)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	printResult(struct {
		Resource string `json:"resource"`
		Action   string `json:"action"`
	}{
		Resource: name,
		Action:   "reset",
	}, func() {
		fmt.Printf("Reset stateful resource: %s\n", name)
		fmt.Println("  All items replaced with seed data.")
	})

	return nil
}

// --- Custom operation command implementations ---

// runCustomList lists all registered custom operations.
func runCustomList(_ *cobra.Command, _ []string) error {
	client := NewAdminClientWithAuth(adminURL)
	ops, err := client.ListCustomOperations()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	if len(ops) == 0 {
		printResult(struct {
			Operations []interface{} `json:"operations"`
			Count      int           `json:"count"`
		}{
			Operations: []interface{}{},
			Count:      0,
		}, func() {
			fmt.Println("No custom operations registered.")
			fmt.Println("\nRegister one with: mockd stateful custom add --file <definition.yaml>")
		})
		return nil
	}

	printList(struct {
		Operations []CustomOperationInfo `json:"operations"`
		Count      int                   `json:"count"`
	}{
		Operations: ops,
		Count:      len(ops),
	}, func() {
		fmt.Printf("Custom Operations (%d):\n\n", len(ops))
		tw := output.Table()
		_, _ = fmt.Fprintf(tw, "NAME\tSTEPS\tCONSISTENCY\n")
		_, _ = fmt.Fprintf(tw, "----\t-----\t-----------\n")
		for _, op := range ops {
			consistency := op.Consistency
			if consistency == "" {
				consistency = "best_effort"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\n", op.Name, op.StepCount, consistency)
		}
		_ = tw.Flush()
	})

	return nil
}

// runCustomGet shows the full definition of a custom operation.
func runCustomGet(_ *cobra.Command, args []string) error {
	name := args[0]

	client := NewAdminClientWithAuth(adminURL)
	op, err := client.GetCustomOperation(name)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	printResult(op, func() {
		fmt.Printf("Custom Operation: %s\n", op.Name)
		consistency := op.Consistency
		if consistency == "" {
			consistency = "best_effort"
		}
		fmt.Printf("  Consistency: %s\n", consistency)
		fmt.Printf("  Steps: %d\n\n", len(op.Steps))

		for i, step := range op.Steps {
			fmt.Printf("  Step %d: %s\n", i+1, step.Type)
			if step.Resource != "" {
				fmt.Printf("    Resource: %s\n", step.Resource)
			}
			if step.ID != "" {
				fmt.Printf("    ID: %s\n", step.ID)
			}
			if step.As != "" {
				fmt.Printf("    As: %s\n", step.As)
			}
			if step.Var != "" {
				fmt.Printf("    Var: %s\n", step.Var)
			}
			if step.Value != "" {
				fmt.Printf("    Value: %s\n", step.Value)
			}
			if len(step.Set) > 0 {
				fmt.Printf("    Set:\n")
				for k, v := range step.Set {
					fmt.Printf("      %s: %s\n", k, v)
				}
			}
		}

		if len(op.Response) > 0 {
			fmt.Printf("\n  Response:\n")
			for k, v := range op.Response {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	})

	return nil
}

// runCustomAdd registers a new custom operation from file or inline definition.
func runCustomAdd(_ *cobra.Command, _ []string) error {
	cfg, err := readCustomOperationConfig(customAddFile, customAddDefinition)
	if err != nil {
		return err
	}
	name := cfg.Name
	if name == "" {
		return errors.New("definition must include a 'name' field")
	}
	definition, err := customOperationConfigToMap(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode definition: %w", err)
	}

	client := NewAdminClientWithAuth(adminURL)
	if err := client.RegisterCustomOperation(definition); err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	printResult(struct {
		Name    string `json:"name"`
		Action  string `json:"action"`
		Message string `json:"message"`
	}{
		Name:    name,
		Action:  "registered",
		Message: "Custom operation registered successfully",
	}, func() {
		fmt.Printf("Registered custom operation: %s\n", name)
	})

	return nil
}

func runCustomValidate(_ *cobra.Command, _ []string) error {
	cfg, err := readCustomOperationConfig(customValidateFile, customValidateDefinition)
	if err != nil {
		return err
	}
	input, err := readCustomOperationInput(customValidateInput, customValidateInputFile)
	if err != nil {
		return err
	}

	result, err := validateCustomOperationLocally(cfg, input)
	if err != nil {
		return err
	}

	if customValidateRuntimeCheck {
		if customValidateInput == "" && customValidateInputFile == "" {
			result.Warnings = append(result.Warnings, "runtime expression checks ran with empty input {}; provide --input/--input-file for stronger validation")
		}
		fixtures, err := readCustomValidationFixtures(customValidateFixturesFile)
		if err != nil {
			return err
		}
		runtimeWarnings, err := validateCustomOperationRuntimeExpressions(cfg, input, fixtures)
		if err != nil {
			return err
		}
		result.RuntimeChecked = true
		result.Warnings = append(result.Warnings, runtimeWarnings...)
	}

	if customValidateCheckResources {
		client := NewAdminClientWithAuth(adminURL)
		overview, err := client.GetStateOverview()
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
		result.CheckedResources = true
		available := make(map[string]struct{}, len(overview.ResourceList))
		for _, name := range overview.ResourceList {
			available[name] = struct{}{}
		}
		for _, name := range result.ReferencedResources {
			if _, ok := available[name]; !ok {
				result.MissingResources = append(result.MissingResources, name)
			}
		}
		if len(result.MissingResources) > 0 {
			return fmt.Errorf("referenced stateful resources not found: %s", strings.Join(result.MissingResources, ", "))
		}
	}
	if customValidateStrict && len(result.Warnings) > 0 {
		return fmt.Errorf("validation warnings (strict mode): %s", strings.Join(result.Warnings, "; "))
	}

	result.Valid = true
	result.Message = "Custom operation definition is valid"
	printResult(result, func() {
		fmt.Printf("Custom operation is valid: %s\n", result.Name)
		fmt.Printf("  Consistency: %s\n", result.Consistency)
		fmt.Printf("  Steps: %d\n", result.Steps)
		if len(result.ReferencedResources) > 0 {
			fmt.Printf("  Resources: %s\n", strings.Join(result.ReferencedResources, ", "))
		}
		if result.CheckedResources {
			fmt.Printf("  Resource checks: passed\n")
		}
		if result.RuntimeChecked {
			fmt.Printf("  Runtime expression checks: passed\n")
		}
		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, w := range result.Warnings {
				fmt.Printf("  - %s\n", w)
			}
		}
	})

	return nil
}

// runCustomRun executes a custom operation.
func runCustomRun(_ *cobra.Command, args []string) error {
	name := args[0]

	input, err := readCustomOperationInput(customRunInput, customRunInputFile)
	if err != nil {
		return err
	}

	client := NewAdminClientWithAuth(adminURL)
	result, err := client.ExecuteCustomOperation(name, input)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	printResult(result, func() {
		fmt.Printf("Operation %s executed successfully.\n\n", name)
		fmt.Println("Result:")
		// Pretty-print the result
		prettyJSON, err := json.MarshalIndent(result, "  ", "  ")
		if err != nil {
			fmt.Printf("  %v\n", result)
		} else {
			fmt.Printf("  %s\n", prettyJSON)
		}
	})

	return nil
}

// runCustomDelete deletes a custom operation.
func runCustomDelete(_ *cobra.Command, args []string) error {
	name := args[0]

	client := NewAdminClientWithAuth(adminURL)
	if err := client.DeleteCustomOperation(name); err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	printResult(struct {
		Name    string `json:"name"`
		Action  string `json:"action"`
		Message string `json:"message"`
	}{
		Name:    name,
		Action:  "deleted",
		Message: "Custom operation deleted",
	}, func() {
		fmt.Printf("Deleted custom operation: %s\n", name)
	})

	return nil
}

func readCustomOperationConfig(filePath, inlineDefinition string) (*config.CustomOperationConfig, error) {
	if filePath == "" && inlineDefinition == "" {
		return nil, errors.New("either --file or --definition is required")
	}
	if filePath != "" && inlineDefinition != "" {
		return nil, errors.New("use either --file or --definition, not both")
	}

	var (
		data []byte
		err  error
		cfg  config.CustomOperationConfig
	)

	if filePath != "" {
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".yaml", ".yml":
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("failed to parse YAML definition: %w", err)
			}
		case ".json":
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("failed to parse JSON definition: %w", err)
			}
		default:
			// Try JSON first, then YAML for extension-less files.
			if err := json.Unmarshal(data, &cfg); err != nil {
				if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr != nil {
					return nil, fmt.Errorf("failed to parse definition as JSON or YAML: %v / %v", err, yamlErr)
				}
			}
		}
	} else {
		if err := json.Unmarshal([]byte(inlineDefinition), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse inline definition: %w", err)
		}
	}

	return &cfg, nil
}

func customOperationConfigToMap(cfg *config.CustomOperationConfig) (map[string]interface{}, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func readCustomOperationInput(inputJSON, inputFile string) (map[string]interface{}, error) {
	if inputJSON != "" && inputFile != "" {
		return nil, errors.New("use either --input or --input-file, not both")
	}

	var input map[string]interface{}
	switch {
	case inputFile != "":
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read input file: %w", err)
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, fmt.Errorf("failed to parse input file: %w", err)
		}
	case inputJSON != "":
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return nil, fmt.Errorf("failed to parse inline input: %w", err)
		}
	default:
		input = make(map[string]interface{})
	}
	if input == nil {
		input = make(map[string]interface{})
	}
	return input, nil
}

func readCustomValidationFixtures(filePath string) (*customValidationFixtures, error) {
	fixtures := &customValidationFixtures{
		Vars:      make(map[string]interface{}),
		Resources: make(map[string]map[string]map[string]interface{}),
	}
	if filePath == "" {
		return fixtures, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixtures file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, fixtures); err != nil {
			return nil, fmt.Errorf("failed to parse YAML fixtures: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, fixtures); err != nil {
			return nil, fmt.Errorf("failed to parse JSON fixtures: %w", err)
		}
	default:
		if err := json.Unmarshal(data, fixtures); err != nil {
			if yamlErr := yaml.Unmarshal(data, fixtures); yamlErr != nil {
				return nil, fmt.Errorf("failed to parse fixtures as JSON or YAML: %v / %v", err, yamlErr)
			}
		}
	}

	if fixtures.Vars == nil {
		fixtures.Vars = make(map[string]interface{})
	}
	if fixtures.Resources == nil {
		fixtures.Resources = make(map[string]map[string]map[string]interface{})
	}

	return fixtures, nil
}

func validateCustomOperationRuntimeExpressions(cfg *config.CustomOperationConfig, input map[string]interface{}, fixtures *customValidationFixtures) ([]string, error) {
	if cfg == nil {
		return nil, errors.New("definition is required")
	}
	if fixtures == nil {
		fixtures = &customValidationFixtures{}
	}
	if fixtures.Vars == nil {
		fixtures.Vars = make(map[string]interface{})
	}
	if fixtures.Resources == nil {
		fixtures.Resources = make(map[string]map[string]map[string]interface{})
	}

	exprCtx := map[string]interface{}{
		"input": input,
	}
	warnings := make([]string, 0)
	seenWarnings := make(map[string]struct{})
	addWarning := func(msg string) {
		if _, ok := seenWarnings[msg]; ok {
			return
		}
		seenWarnings[msg] = struct{}{}
		warnings = append(warnings, msg)
	}

	for k, v := range fixtures.Vars {
		if k == "input" {
			addWarning(`fixtures.vars["input"] is ignored (reserved variable)`)
			continue
		}
		exprCtx[k] = cloneValidationValue(v)
	}

	for i, step := range cfg.Steps {
		stepNum := i + 1
		stepType := strings.TrimSpace(step.Type)
		switch stepType {
		case "read":
			idVal, err := evalExprRuntime(step.ID, exprCtx)
			if err != nil {
				return warnings, fmt.Errorf("step %d (read.id): %w", stepNum, err)
			}
			item, synthetic := runtimeReadFixtureValue(step, idVal, fixtures)
			if synthetic {
				addWarning(fmt.Sprintf("step %d (read) uses synthetic placeholder for %q; add --fixtures-file with %s/%v for stronger runtime checks", stepNum, step.As, step.Resource, idVal))
			}
			if step.As != "" {
				exprCtx[step.As] = item
			}
		case "update":
			idVal, err := evalExprRuntime(step.ID, exprCtx)
			if err != nil {
				return warnings, fmt.Errorf("step %d (update.id): %w", stepNum, err)
			}
			updated := runtimeBaseItem(step, idVal, fixtures)
			if step.As != "" && runtimeIsSyntheticBase(step, idVal, fixtures) {
				addWarning(fmt.Sprintf("step %d (update) uses synthetic base object for %q; add --fixtures-file with %s/%v for stronger runtime checks", stepNum, step.As, step.Resource, idVal))
			}
			for field, exprStr := range step.Set {
				val, err := evalExprRuntime(exprStr, exprCtx)
				if err != nil {
					return warnings, fmt.Errorf("step %d (update.set.%s): %w", stepNum, field, err)
				}
				updated[field] = val
			}
			if step.As != "" {
				exprCtx[step.As] = updated
			}
		case "delete":
			if _, err := evalExprRuntime(step.ID, exprCtx); err != nil {
				return warnings, fmt.Errorf("step %d (delete.id): %w", stepNum, err)
			}
		case "create":
			created := make(map[string]interface{})
			for field, exprStr := range step.Set {
				val, err := evalExprRuntime(exprStr, exprCtx)
				if err != nil {
					return warnings, fmt.Errorf("step %d (create.set.%s): %w", stepNum, field, err)
				}
				created[field] = val
			}
			if step.As != "" {
				exprCtx[step.As] = created
			}
		case "set":
			val, err := evalExprRuntime(step.Value, exprCtx)
			if err != nil {
				return warnings, fmt.Errorf("step %d (set.value): %w", stepNum, err)
			}
			exprCtx[step.Var] = val
		default:
			return warnings, fmt.Errorf("step %d: unknown step type %q", stepNum, step.Type)
		}
	}

	for field, exprStr := range cfg.Response {
		if _, err := evalExprRuntime(exprStr, exprCtx); err != nil {
			return warnings, fmt.Errorf("response.%s: %w", field, err)
		}
	}

	return warnings, nil
}

func evalExprRuntime(expression string, env map[string]interface{}) (interface{}, error) {
	if strings.TrimSpace(expression) == "" {
		return nil, errors.New("expression is empty")
	}
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("invalid expression %q: %w", expression, err)
	}
	val, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("eval %q: %w", expression, err)
	}
	return val, nil
}

func runtimeReadFixtureValue(step config.CustomStepConfig, idVal interface{}, fixtures *customValidationFixtures) (map[string]interface{}, bool) {
	if step.As != "" {
		if v, ok := fixtures.Vars[step.As]; ok {
			if item, ok := cloneValidationValue(v).(map[string]interface{}); ok {
				return item, false
			}
		}
	}
	idKey := fmt.Sprint(idVal)
	if byResource, ok := fixtures.Resources[step.Resource]; ok {
		if item, ok := byResource[idKey]; ok {
			return cloneValidationMap(item), false
		}
	}
	return map[string]interface{}{
		"id": idKey,
	}, true
}

func runtimeBaseItem(step config.CustomStepConfig, idVal interface{}, fixtures *customValidationFixtures) map[string]interface{} {
	idKey := fmt.Sprint(idVal)
	if byResource, ok := fixtures.Resources[step.Resource]; ok {
		if item, ok := byResource[idKey]; ok {
			return cloneValidationMap(item)
		}
	}
	if step.As != "" {
		if v, ok := fixtures.Vars[step.As]; ok {
			if item, ok := cloneValidationValue(v).(map[string]interface{}); ok {
				return item
			}
		}
	}
	return map[string]interface{}{
		"id": idKey,
	}
}

func runtimeIsSyntheticBase(step config.CustomStepConfig, idVal interface{}, fixtures *customValidationFixtures) bool {
	idKey := fmt.Sprint(idVal)
	if byResource, ok := fixtures.Resources[step.Resource]; ok {
		if _, ok := byResource[idKey]; ok {
			return false
		}
	}
	if step.As != "" {
		if v, ok := fixtures.Vars[step.As]; ok {
			_, ok = cloneValidationValue(v).(map[string]interface{})
			return !ok
		}
	}
	return true
}

func cloneValidationMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = cloneValidationValue(v)
	}
	return out
}

func cloneValidationValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return cloneValidationMap(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			out[i] = cloneValidationValue(t[i])
		}
		return out
	default:
		return v
	}
}

func validateCustomOperationLocally(cfg *config.CustomOperationConfig, input map[string]interface{}) (*customValidationResult, error) { //nolint:gocyclo // step-type validation logic
	if cfg == nil {
		return nil, errors.New("definition is required")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, errors.New("definition must include a 'name' field")
	}
	if len(cfg.Steps) == 0 {
		return nil, errors.New("definition must include at least one step")
	}

	op := &stateful.CustomOperation{
		Name:        cfg.Name,
		Consistency: stateful.ConsistencyMode(cfg.Consistency),
		Response:    cfg.Response,
	}
	if _, err := stateful.NormalizeCustomOperation(op); err != nil {
		return nil, err
	}

	env := map[string]interface{}{
		"input": input,
	}
	referenced := make(map[string]struct{})
	warnings := make([]string, 0)

	for i, step := range cfg.Steps {
		stepNum := i + 1
		stepType := strings.TrimSpace(step.Type)
		if stepType == "" {
			return nil, fmt.Errorf("step %d: type is required", stepNum)
		}
		switch stepType {
		case "read":
			if step.Resource == "" {
				return nil, fmt.Errorf("step %d (read): resource is required", stepNum)
			}
			if step.ID == "" {
				return nil, fmt.Errorf("step %d (read): id is required", stepNum)
			}
			if step.As == "" {
				return nil, fmt.Errorf("step %d (read): as is required", stepNum)
			}
			if err := validateExprCompile(step.ID, env); err != nil {
				return nil, fmt.Errorf("step %d (read.id): %w", stepNum, err)
			}
			referenced[step.Resource] = struct{}{}
			env[step.As] = map[string]interface{}{}
		case "update":
			if step.Resource == "" {
				return nil, fmt.Errorf("step %d (update): resource is required", stepNum)
			}
			if step.ID == "" {
				return nil, fmt.Errorf("step %d (update): id is required", stepNum)
			}
			if err := validateExprCompile(step.ID, env); err != nil {
				return nil, fmt.Errorf("step %d (update.id): %w", stepNum, err)
			}
			for field, exprStr := range step.Set {
				if err := validateExprCompile(exprStr, env); err != nil {
					return nil, fmt.Errorf("step %d (update.set.%s): %w", stepNum, field, err)
				}
			}
			if len(step.Set) == 0 {
				warnings = append(warnings, fmt.Sprintf("step %d (update) has an empty set map", stepNum))
			}
			referenced[step.Resource] = struct{}{}
			if step.As != "" {
				env[step.As] = map[string]interface{}{}
			}
		case "delete":
			if step.Resource == "" {
				return nil, fmt.Errorf("step %d (delete): resource is required", stepNum)
			}
			if step.ID == "" {
				return nil, fmt.Errorf("step %d (delete): id is required", stepNum)
			}
			if err := validateExprCompile(step.ID, env); err != nil {
				return nil, fmt.Errorf("step %d (delete.id): %w", stepNum, err)
			}
			referenced[step.Resource] = struct{}{}
		case "create":
			if step.Resource == "" {
				return nil, fmt.Errorf("step %d (create): resource is required", stepNum)
			}
			for field, exprStr := range step.Set {
				if err := validateExprCompile(exprStr, env); err != nil {
					return nil, fmt.Errorf("step %d (create.set.%s): %w", stepNum, field, err)
				}
			}
			if len(step.Set) == 0 {
				warnings = append(warnings, fmt.Sprintf("step %d (create) has an empty set map", stepNum))
			}
			referenced[step.Resource] = struct{}{}
			if step.As != "" {
				env[step.As] = map[string]interface{}{}
			}
		case "set":
			if step.Var == "" {
				return nil, fmt.Errorf("step %d (set): var is required", stepNum)
			}
			if step.Value == "" {
				return nil, fmt.Errorf("step %d (set): value is required", stepNum)
			}
			if err := validateExprCompile(step.Value, env); err != nil {
				return nil, fmt.Errorf("step %d (set.value): %w", stepNum, err)
			}
			env[step.Var] = 0
		default:
			return nil, fmt.Errorf("step %d: unknown step type %q", stepNum, step.Type)
		}
	}

	for field, exprStr := range cfg.Response {
		if err := validateExprCompile(exprStr, env); err != nil {
			return nil, fmt.Errorf("response.%s: %w", field, err)
		}
	}

	resourceList := make([]string, 0, len(referenced))
	for name := range referenced {
		resourceList = append(resourceList, name)
	}
	slices.Sort(resourceList)

	return &customValidationResult{
		Name:                cfg.Name,
		Steps:               len(cfg.Steps),
		Consistency:         string(op.Consistency),
		ReferencedResources: resourceList,
		Warnings:            warnings,
	}, nil
}

func validateExprCompile(expression string, env map[string]interface{}) error {
	if strings.TrimSpace(expression) == "" {
		return errors.New("expression is empty")
	}
	if _, err := expr.Compile(expression, expr.Env(env)); err != nil {
		return fmt.Errorf("invalid expression %q: %w", expression, err)
	}
	return nil
}
