package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/spf13/cobra"
)

var graphqlCmd = &cobra.Command{
	Use:   "graphql",
	Short: "Manage and test GraphQL endpoints",
}

var graphqlAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new GraphQL mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use huh interactive forms if attributes are missing
		if !cmd.Flags().Changed("path") {
			var formPath, formOperationType, formOperationName, formResponse string

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("What is the GraphQL endpoint path?").
						Placeholder("/graphql").
						Value(&formPath).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("path is required")
							}
							return nil
						}),
					huh.NewSelect[string]().
						Title("Operation Type").
						Options(
							huh.NewOption("Query", "query"),
							huh.NewOption("Mutation", "mutation"),
						).
						Value(&formOperationType),
					huh.NewInput().
						Title("Operation Name").
						Placeholder("GetUser").
						Value(&formOperationName).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("operation name is required")
							}
							return nil
						}),
					huh.NewText().
						Title("Response JSON").
						Placeholder(`{"data": {"user": {"name": "Test"}}}`).
						Value(&formResponse),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			addPath = formPath
			addOpType = formOperationType
			addOperation = formOperationName
			addResponse = formResponse
		}
		addMockType = "graphql"
		return runAdd(cmd, args)
	},
}

var (
	graphqlVariables     string
	graphqlOperationName string
	graphqlHeaders       string
	graphqlPretty        bool
)

func init() {
	rootCmd.AddCommand(graphqlCmd)
	graphqlCmd.AddCommand(graphqlAddCmd)

	graphqlAddCmd.Flags().StringVar(&addPath, "path", "", "URL path to match")
	graphqlAddCmd.Flags().StringVar(&addOperation, "operation", "", "Operation name")
	graphqlAddCmd.Flags().StringVar(&addOpType, "op-type", "query", "GraphQL operation type (query/mutation)")
	graphqlAddCmd.Flags().StringVar(&addResponse, "response", "", "JSON response data")

	// Add list/get/delete generic aliases
	graphqlCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List GraphQL mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "graphql"
			return runList(cmd, args)
		},
	})
	graphqlCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of a GraphQL mock",
		RunE:  runGet,
	})
	graphqlCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete a GraphQL mock",
		RunE:  runDelete,
	})

	graphqlCmd.AddCommand(graphqlValidateCmd)

	graphqlQueryCmd.Flags().StringVarP(&graphqlVariables, "variables", "v", "", "JSON string of variables")
	graphqlQueryCmd.Flags().StringVarP(&graphqlOperationName, "operation", "o", "", "Operation name")
	graphqlQueryCmd.Flags().StringVarP(&graphqlHeaders, "header", "H", "", "Additional headers (key:value,key2:value2)")
	graphqlQueryCmd.Flags().BoolVar(&graphqlPretty, "pretty", true, "Pretty print output")
	graphqlCmd.AddCommand(graphqlQueryCmd)
}

var graphqlValidateCmd = &cobra.Command{
	Use:   "validate <schema-file>",
	Short: "Validate a GraphQL schema file",
	RunE:  runGraphQLValidate,
}

var graphqlQueryCmd = &cobra.Command{
	Use:   "query <endpoint> <query>",
	Short: "Execute a query against a GraphQL endpoint",
	RunE:  runGraphQLQuery,
}

// runGraphQLValidate validates a GraphQL schema file.
func runGraphQLValidate(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("schema file is required")
	}

	schemaFile := args[0]

	// Read schema file
	schemaBytes, err := os.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Parse and validate schema
	schema, err := graphql.ParseSchema(string(schemaBytes))
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	// Additional validation
	if err := schema.Validate(); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	// Print schema info
	typeCount := len(schema.ListTypes())
	queryCount := len(schema.ListQueries())
	mutationCount := len(schema.ListMutations())
	subscriptionCount := len(schema.ListSubscriptions())

	fmt.Printf("Schema valid: %s\n", schemaFile)
	fmt.Printf("  Types: %d\n", typeCount)
	fmt.Printf("  Queries: %d\n", queryCount)
	if mutationCount > 0 {
		fmt.Printf("  Mutations: %d\n", mutationCount)
	}
	if subscriptionCount > 0 {
		fmt.Printf("  Subscriptions: %d\n", subscriptionCount)
	}

	return nil
}

// runGraphQLQuery executes a GraphQL query against an endpoint.
func runGraphQLQuery(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("endpoint and query are required")
	}

	endpoint := args[0]
	query := args[1]

	// Load query from file if prefixed with @
	if len(query) > 0 && query[0] == '@' {
		queryBytes, err := os.ReadFile(query[1:])
		if err != nil {
			return fmt.Errorf("failed to read query file: %w", err)
		}
		query = string(queryBytes)
	}

	// Parse variables
	var varsMap map[string]interface{}
	if graphqlVariables != "" {
		if err := json.Unmarshal([]byte(graphqlVariables), &varsMap); err != nil {
			return fmt.Errorf("invalid variables JSON: %w", err)
		}
	}

	// Build request body
	reqBody := graphql.GraphQLRequest{
		Query:         query,
		Variables:     varsMap,
		OperationName: graphqlOperationName,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	if graphqlHeaders != "" {
		for _, header := range parse.SplitHeaders(graphqlHeaders) {
			parts := parse.HeaderParts(header)
			if len(parts) == 2 {
				req.Header.Set(parts[0], parts[1])
			}
		}
	}

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Print response
	if graphqlPretty {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, respBody, "", "  "); err != nil {
			fmt.Println(string(respBody))
		} else {
			fmt.Println(prettyJSON.String())
		}
	} else {
		fmt.Println(string(respBody))
	}

	// Check for errors in response
	var gqlResp graphql.GraphQLResponse
	if err := json.Unmarshal(respBody, &gqlResp); err == nil {
		if len(gqlResp.Errors) > 0 {
			return fmt.Errorf("query returned %d error(s)", len(gqlResp.Errors))
		}
	}

	return nil
}
