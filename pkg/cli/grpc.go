package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	grpcpkg "github.com/getmockd/mockd/pkg/grpc"
	"github.com/spf13/cobra"
)

var grpcCmd = &cobra.Command{
	Use:   "grpc",
	Short: "Manage and test gRPC endpoints",
}

var grpcAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new gRPC mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use huh interactive forms if attributes are missing
		if !cmd.Flags().Changed("service") {
			var formService, formMethod, formProto string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Path to the .proto file?").
						Placeholder("api.proto").
						Value(&formProto),
					huh.NewInput().
						Title("What is the gRPC service name to mock?").
						Placeholder("greet.Greeter").
						Value(&formService),
					huh.NewInput().
						Title("What gRPC method in that service?").
						Placeholder("SayHello").
						Value(&formMethod),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			addService = formService
			addRPCMethod = formMethod
			addProtoFiles = append(addProtoFiles, formProto)
		}
		addMockType = "grpc"
		return runAdd(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(grpcCmd)
	grpcCmd.AddCommand(grpcAddCmd)

	grpcAddCmd.Flags().StringVar(&addService, "service", "", "gRPC service name (e.g., greeter.Greeter)")
	grpcAddCmd.Flags().StringVar(&addRPCMethod, "rpc-method", "", "gRPC method name")
	grpcAddCmd.Flags().IntVar(&addGRPCPort, "grpc-port", 50051, "gRPC server port")
	grpcAddCmd.Flags().Var(&addProtoFiles, "proto", "Path to .proto file")
	grpcAddCmd.Flags().StringVar(&addResponse, "response", "", "JSON response data")

	// Add list/get/delete generic aliases
	grpcCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List gRPC mocks or definitions from a proto file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --mocks is passed, list mocks on server, otherwise do internal proto list
			listMocks, _ := cmd.Flags().GetBool("mocks")
			if listMocks {
				listMockType = "grpc"
				return runList(cmd, args)
			}
			return runGRPCList(cmd, args)
		},
	})
	// Just bind the boolean for mock listing
	grpcCmd.Commands()[1].Flags().Bool("mocks", false, "List server gRPC mocks instead of local proto definitions")
	grpcCmd.Commands()[1].Flags().StringVar(&grpcImportPath, "import", "", "Import path for proto includes")
	grpcCmd.Commands()[1].Flags().StringVar(&grpcImportPath, "I", "", "Import path (shorthand)")

	grpcCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of a gRPC mock",
		RunE:  runGet,
	})
	grpcCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete a gRPC mock",
		RunE:  runDelete,
	})

	// Bind call functionality
	grpcCallCmd.Flags().StringVarP(&grpcMetadata, "metadata", "m", "", "gRPC metadata")
	grpcCallCmd.Flags().BoolVar(&grpcPlaintext, "plaintext", true, "Use plaintext")
	grpcCallCmd.Flags().BoolVar(&grpcPretty, "pretty", true, "Pretty print")
	grpcCmd.AddCommand(grpcCallCmd)
}

var grpcImportPath string
var grpcMetadata string
var grpcPlaintext bool
var grpcPretty bool

var grpcCallCmd = &cobra.Command{
	Use:   "call <endpoint> <service/method> <json-body>",
	Short: "Call a gRPC method",
	RunE:  runGRPCCall,
}

// runGRPCList lists services and methods from a proto file.
func runGRPCList(_ *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("proto file is required")
	}

	protoFile := args[0]

	// Parse proto file
	var importPaths []string
	if grpcImportPath != "" {
		importPaths = strings.Split(grpcImportPath, ",")
	}

	schema, err := grpcpkg.ParseProtoFile(protoFile, importPaths)
	if err != nil {
		return fmt.Errorf("failed to parse proto file: %w", err)
	}

	// List services
	serviceNames := schema.ListServices()
	if len(serviceNames) == 0 {
		printResult(map[string]any{"file": protoFile, "services": []any{}}, func() {
			fmt.Println("No services found in proto file")
		})
		return nil
	}

	// Build structured result for JSON
	type methodInfo struct {
		Name       string `json:"name"`
		InputType  string `json:"inputType"`
		OutputType string `json:"outputType"`
		Streaming  string `json:"streaming,omitempty"`
	}
	type serviceInfo struct {
		Name    string       `json:"name"`
		Methods []methodInfo `json:"methods"`
	}
	var services []serviceInfo
	for _, svcName := range serviceNames {
		svc := schema.GetService(svcName)
		si := serviceInfo{Name: svc.Name}
		for _, methodName := range svc.ListMethods() {
			method := svc.GetMethod(methodName)
			mi := methodInfo{
				Name:       method.Name,
				InputType:  extractTypeName(method.InputType),
				OutputType: extractTypeName(method.OutputType),
			}
			switch {
			case method.IsBidirectional():
				mi.Streaming = "bidirectional"
			case method.IsClientStreaming():
				mi.Streaming = "client"
			case method.IsServerStreaming():
				mi.Streaming = "server"
			}
			si.Methods = append(si.Methods, mi)
		}
		services = append(services, si)
	}

	printResult(map[string]any{"file": protoFile, "services": services}, func() {
		fmt.Printf("Proto: %s\n\n", protoFile)
		for _, si := range services {
			fmt.Printf("Service: %s\n", si.Name)
			for _, mi := range si.Methods {
				streamInfo := ""
				if mi.Streaming != "" {
					streamInfo = fmt.Sprintf(" [%s streaming]", mi.Streaming)
				}
				fmt.Printf("  %s(%s) → %s%s\n", mi.Name, mi.InputType, mi.OutputType, streamInfo)
			}
			fmt.Println()
		}
	})
	return nil
}

// runGRPCCall executes a gRPC call against an endpoint.
func runGRPCCall(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		return errors.New("endpoint, service/method, and json-body are required")
	}

	endpoint := args[0]
	serviceMethod := args[1]
	jsonBody := args[2]

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
		return printGRPCCallInstructions(endpoint, serviceMethod, jsonBody, grpcMetadata, grpcPlaintext)
	}

	// Build grpcurl command
	grpcArgs := []string{}
	if grpcPlaintext {
		grpcArgs = append(grpcArgs, "-plaintext")
	}

	// Add metadata
	if grpcMetadata != "" {
		for _, m := range strings.Split(grpcMetadata, ",") {
			parts := strings.SplitN(m, ":", 2)
			if len(parts) == 2 {
				grpcArgs = append(grpcArgs, "-H", fmt.Sprintf("%s: %s", parts[0], parts[1]))
			}
		}
	}

	grpcArgs = append(grpcArgs, "-d", jsonBody)
	grpcArgs = append(grpcArgs, endpoint, serviceMethod)

	execCmd := exec.Command(grpcurlPath, grpcArgs...)
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("grpc call failed: %s", stderr.String())
		}
		return fmt.Errorf("grpc call failed: %w", err)
	}

	// Print response
	output := stdout.String()
	if grpcPretty {
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
