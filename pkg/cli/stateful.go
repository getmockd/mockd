package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/spf13/cobra"
)

var (
	statefulAddPath    string
	statefulAddIDField string
	statefulListLimit  int
	statefulListOffset int
	statefulListSort   string
	statefulListOrder  string

	// Custom operation flags
	customAddFile       string
	customAddDefinition string
	customRunInput      string
	customRunInputFile  string
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

	customCmd.AddCommand(customRunCmd)
	customRunCmd.Flags().StringVar(&customRunInput, "input", "", "Inline JSON input for the operation")
	customRunCmd.Flags().StringVar(&customRunInputFile, "input-file", "", "Path to JSON file containing operation input")

	customCmd.AddCommand(customDeleteCmd)
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
		fmt.Fprintf(tw, "NAME\tBASE PATH\tITEMS\tSEED\tID FIELD\n")
		fmt.Fprintf(tw, "----\t---------\t-----\t----\t--------\n")
		for _, r := range overview.Resources {
			basePath := r.BasePath
			if basePath == "" {
				basePath = "(bridge-only)"
			}
			idField := r.IDField
			if idField == "" {
				idField = "id"
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n",
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
		fmt.Fprintf(tw, "NAME\tSTEPS\n")
		fmt.Fprintf(tw, "----\t-----\n")
		for _, op := range ops {
			fmt.Fprintf(tw, "%s\t%d\n", op.Name, op.StepCount)
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
	if customAddFile == "" && customAddDefinition == "" {
		return errors.New("either --file or --definition is required")
	}
	if customAddFile != "" && customAddDefinition != "" {
		return errors.New("use either --file or --definition, not both")
	}

	var definition map[string]interface{}

	if customAddFile != "" {
		data, err := os.ReadFile(customAddFile)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Try JSON first, then YAML-as-JSON (for simplicity, support JSON only for now;
		// YAML support can be added later with a yaml library)
		if err := json.Unmarshal(data, &definition); err != nil {
			return fmt.Errorf("failed to parse definition (must be valid JSON): %w", err)
		}
	} else {
		if err := json.Unmarshal([]byte(customAddDefinition), &definition); err != nil {
			return fmt.Errorf("failed to parse inline definition: %w", err)
		}
	}

	name, _ := definition["name"].(string)
	if name == "" {
		return errors.New("definition must include a 'name' field")
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

// runCustomRun executes a custom operation.
func runCustomRun(_ *cobra.Command, args []string) error {
	name := args[0]

	var input map[string]interface{}
	switch {
	case customRunInputFile != "":
		data, err := os.ReadFile(customRunInputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("failed to parse input file: %w", err)
		}
	case customRunInput != "":
		if err := json.Unmarshal([]byte(customRunInput), &input); err != nil {
			return fmt.Errorf("failed to parse inline input: %w", err)
		}
	default:
		input = make(map[string]interface{})
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
