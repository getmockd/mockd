package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	grpcpkg "github.com/getmockd/mockd/pkg/grpc"
)

// RunGRPC handles the grpc command and its subcommands.
func RunGRPC(args []string) error {
	if len(args) == 0 {
		printGRPCUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list":
		return runGRPCList(subArgs)
	case "call":
		return runGRPCCall(subArgs)
	case "help", "--help", "-h":
		printGRPCUsage()
		return nil
	default:
		return fmt.Errorf("unknown grpc subcommand: %s\n\nRun 'mockd grpc --help' for usage", subcommand)
	}
}

func printGRPCUsage() {
	fmt.Print(`Usage: mockd grpc <subcommand> [flags]

Manage and test gRPC endpoints.

Subcommands:
  list    List services and methods from a proto file
  call    Call a gRPC method

Run 'mockd grpc <subcommand> --help' for more information.
`)
}

// runGRPCList lists services and methods from a proto file.
func runGRPCList(args []string) error {
	fs := flag.NewFlagSet("grpc list", flag.ContinueOnError)

	importPath := fs.String("import", "", "Import path for proto includes")
	fs.StringVar(importPath, "I", "", "Import path (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd grpc list <proto-file>

List services and methods defined in a proto file.

Arguments:
  proto-file    Path to the .proto file

Flags:
  -I, --import  Import path for proto includes

Examples:
  # List services from a proto file
  mockd grpc list api.proto

  # With import path
  mockd grpc list api.proto -I ./proto
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return errors.New("proto file is required")
	}

	protoFile := fs.Arg(0)

	// Parse proto file
	var importPaths []string
	if *importPath != "" {
		importPaths = strings.Split(*importPath, ",")
	}

	schema, err := grpcpkg.ParseProtoFile(protoFile, importPaths)
	if err != nil {
		return fmt.Errorf("failed to parse proto file: %w", err)
	}

	// List services
	serviceNames := schema.ListServices()
	if len(serviceNames) == 0 {
		fmt.Println("No services found in proto file")
		return nil
	}

	fmt.Printf("Proto: %s\n\n", protoFile)

	for _, svcName := range serviceNames {
		svc := schema.GetService(svcName)
		fmt.Printf("Service: %s\n", svc.Name)

		for _, methodName := range svc.ListMethods() {
			method := svc.GetMethod(methodName)

			// Determine stream type
			var streamInfo string
			switch {
			case method.IsBidirectional():
				streamInfo = " [bidirectional streaming]"
			case method.IsClientStreaming():
				streamInfo = " [client streaming]"
			case method.IsServerStreaming():
				streamInfo = " [server streaming]"
			}

			// Extract simple type names from fully qualified names
			inputType := extractTypeName(method.InputType)
			outputType := extractTypeName(method.OutputType)

			fmt.Printf("  %s(%s) → %s%s\n",
				method.Name,
				inputType,
				outputType,
				streamInfo,
			)
		}
		fmt.Println()
	}

	return nil
}

// runGRPCCall executes a gRPC call against an endpoint.
func runGRPCCall(args []string) error {
	fs := flag.NewFlagSet("grpc call", flag.ContinueOnError)

	metadata := fs.String("metadata", "", "gRPC metadata as key:value,key2:value2")
	fs.StringVar(metadata, "m", "", "gRPC metadata (shorthand)")

	plaintext := fs.Bool("plaintext", true, "Use plaintext (no TLS)")
	pretty := fs.Bool("pretty", true, "Pretty print output")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd grpc call <endpoint> <service/method> <json-body>

Call a gRPC method on an endpoint.

Arguments:
  endpoint        gRPC server address (e.g., localhost:50051)
  service/method  Full service and method name (e.g., package.Service/Method)
  json-body       JSON request body or @filename

Flags:
  -m, --metadata  gRPC metadata as key:value,key2:value2
      --plaintext Use plaintext (no TLS, default: true)
      --pretty    Pretty print output (default: true)

Examples:
  # Call a method
  mockd grpc call localhost:50051 greet.Greeter/SayHello '{"name": "World"}'

  # With metadata
  mockd grpc call localhost:50051 greet.Greeter/SayHello '{"name": "World"}' \
    -m "authorization:Bearer token123"

  # Request from file
  mockd grpc call localhost:50051 greet.Greeter/SayHello @request.json

Note: This command uses grpcurl if available, otherwise provides instructions.
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 3 {
		fs.Usage()
		return errors.New("endpoint, service/method, and json-body are required")
	}

	endpoint := fs.Arg(0)
	serviceMethod := fs.Arg(1)
	jsonBody := fs.Arg(2)

	// Load body from file if prefixed with @
	if len(jsonBody) > 0 && jsonBody[0] == '@' {
		bodyBytes, err := os.ReadFile(jsonBody[1:])
		if err != nil {
			return fmt.Errorf("failed to read request file: %w", err)
		}
		jsonBody = string(bodyBytes)
	}

	// Validate JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(jsonBody), &jsonData); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}

	// Check if grpcurl is available
	grpcurlPath, err := exec.LookPath("grpcurl")
	if err != nil {
		return printGRPCCallInstructions(endpoint, serviceMethod, jsonBody, *metadata, *plaintext)
	}

	// Build grpcurl command
	grpcArgs := []string{}
	if *plaintext {
		grpcArgs = append(grpcArgs, "-plaintext")
	}

	// Add metadata
	if *metadata != "" {
		for _, m := range strings.Split(*metadata, ",") {
			parts := strings.SplitN(m, ":", 2)
			if len(parts) == 2 {
				grpcArgs = append(grpcArgs, "-H", fmt.Sprintf("%s: %s", parts[0], parts[1]))
			}
		}
	}

	grpcArgs = append(grpcArgs, "-d", jsonBody)
	grpcArgs = append(grpcArgs, endpoint, serviceMethod)

	cmd := exec.Command(grpcurlPath, grpcArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("grpc call failed: %s", stderr.String())
		}
		return fmt.Errorf("grpc call failed: %w", err)
	}

	// Print response
	output := stdout.String()
	if *pretty {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, []byte(output), "", "  "); err != nil {
			fmt.Println(output)
		} else {
			fmt.Println(prettyJSON.String())
		}
	} else {
		fmt.Println(output)
	}

	return nil
}

// extractTypeName extracts the simple type name from a fully qualified name.
func extractTypeName(fqn string) string {
	for i := len(fqn) - 1; i >= 0; i-- {
		if fqn[i] == '.' {
			return fqn[i+1:]
		}
	}
	return fqn
}

// printGRPCCallInstructions prints instructions when grpcurl is not available.
func printGRPCCallInstructions(endpoint, serviceMethod, body, metadata string, plaintext bool) error {
	fmt.Println("grpcurl is not installed. To make gRPC calls, install grpcurl:")
	fmt.Println()
	fmt.Println("  # macOS")
	fmt.Println("  brew install grpcurl")
	fmt.Println()
	fmt.Println("  # Linux")
	fmt.Println("  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest")
	fmt.Println()
	fmt.Println("Or use this command directly:")
	fmt.Println()

	var b strings.Builder
	b.WriteString("grpcurl")
	if plaintext {
		b.WriteString(" -plaintext")
	}
	if metadata != "" {
		for _, m := range strings.Split(metadata, ",") {
			parts := strings.SplitN(m, ":", 2)
			if len(parts) == 2 {
				b.WriteString(" -H '")
				b.WriteString(parts[0])
				b.WriteString(": ")
				b.WriteString(parts[1])
				b.WriteByte('\'')
			}
		}
	}
	b.WriteString(" -d '")
	b.WriteString(body)
	b.WriteByte('\'')
	b.WriteByte(' ')
	b.WriteString(endpoint)
	b.WriteByte(' ')
	b.WriteString(serviceMethod)

	fmt.Println("  " + b.String())

	return errors.New("grpcurl not found")
}
