package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/getmockd/mockd/pkg/graphql"
)

// RunGraphQL handles the graphql command and its subcommands.
func RunGraphQL(args []string) error {
	if len(args) == 0 {
		printGraphQLUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		return RunAdd(append([]string{"graphql"}, subArgs...))
	case "list":
		return RunList(append([]string{"--type", "graphql"}, subArgs...))
	case "get":
		return RunGet(subArgs)
	case "delete", "rm", "remove":
		return RunDelete(subArgs)
	case "validate":
		return runGraphQLValidate(subArgs)
	case "query":
		return runGraphQLQuery(subArgs)
	case "help", "--help", "-h":
		printGraphQLUsage()
		return nil
	default:
		return fmt.Errorf("unknown graphql subcommand: %s\n\nRun 'mockd graphql --help' for usage", subcommand)
	}
}

func printGraphQLUsage() {
	fmt.Print(`Usage: mockd graphql <subcommand> [flags]

Manage and test GraphQL endpoints.

Subcommands:
  add       Add a new GraphQL mock endpoint
  list      List GraphQL mocks
  get       Get details of a GraphQL mock
  delete    Delete a GraphQL mock
  validate  Validate a GraphQL schema file
  query     Execute a query against a GraphQL endpoint

Run 'mockd graphql <subcommand> --help' for more information.
`)
}

// runGraphQLValidate validates a GraphQL schema file.
func runGraphQLValidate(args []string) error {
	fs := flag.NewFlagSet("graphql validate", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd graphql validate <schema-file>

Validate a GraphQL schema file.

Arguments:
  schema-file    Path to the GraphQL schema file (.graphql or .gql)

Examples:
  # Validate a schema file
  mockd graphql validate schema.graphql

  # Validate with full path
  mockd graphql validate ./schemas/api.graphql
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return errors.New("schema file is required")
	}

	schemaFile := fs.Arg(0)

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
func runGraphQLQuery(args []string) error {
	fs := flag.NewFlagSet("graphql query", flag.ContinueOnError)

	variables := fs.String("variables", "", "JSON string of variables")
	fs.StringVar(variables, "v", "", "JSON string of variables (shorthand)")

	operationName := fs.String("operation", "", "Operation name for multi-operation documents")
	fs.StringVar(operationName, "o", "", "Operation name (shorthand)")

	headers := fs.String("header", "", "Additional headers (key:value,key2:value2)")
	fs.StringVar(headers, "H", "", "Additional headers (shorthand)")

	pretty := fs.Bool("pretty", true, "Pretty print output")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd graphql query <endpoint> <query>

Execute a GraphQL query against an endpoint.

Arguments:
  endpoint    GraphQL endpoint URL (e.g., http://localhost:4280/graphql)
  query       GraphQL query string or @filename

Flags:
  -v, --variables   JSON string of variables
  -o, --operation   Operation name for multi-operation documents
  -H, --header      Additional headers (key:value,key2:value2)
      --pretty      Pretty print output (default: true)

Examples:
  # Simple query
  mockd graphql query http://localhost:4280/graphql "{ users { id name } }"

  # Query with variables
  mockd graphql query http://localhost:4280/graphql \
    "query GetUser($id: ID!) { user(id: $id) { name } }" \
    -v '{"id": "123"}'

  # Query from file
  mockd graphql query http://localhost:4280/graphql @query.graphql

  # With custom headers
  mockd graphql query http://localhost:4280/graphql "{ me { name } }" \
    -H "Authorization:Bearer token123"
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return errors.New("endpoint and query are required")
	}

	endpoint := fs.Arg(0)
	query := fs.Arg(1)

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
	if *variables != "" {
		if err := json.Unmarshal([]byte(*variables), &varsMap); err != nil {
			return fmt.Errorf("invalid variables JSON: %w", err)
		}
	}

	// Build request body
	reqBody := graphql.GraphQLRequest{
		Query:         query,
		Variables:     varsMap,
		OperationName: *operationName,
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
	if *headers != "" {
		for _, header := range parse.SplitHeaders(*headers) {
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
	if *pretty {
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
