package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/ai"
	"github.com/getmockd/mockd/pkg/ai/generator"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/portability"
)

// RunGenerate handles the generate command for AI-powered mock generation.
func RunGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	// Input flags
	input := fs.String("input", "", "Input OpenAPI spec file")
	fs.StringVar(input, "i", "", "Input OpenAPI spec file (shorthand)")
	prompt := fs.String("prompt", "", "Natural language description for generation")
	fs.StringVar(prompt, "p", "", "Natural language description (shorthand)")

	// Output flags
	output := fs.String("output", "", "Output file (default: stdout)")
	fs.StringVar(output, "o", "", "Output file (shorthand)")

	// AI flags
	aiFlag := fs.Bool("ai", false, "Enable AI-powered data generation")
	provider := fs.String("provider", "", "AI provider (openai, anthropic, ollama, openrouter)")
	model := fs.String("model", "", "AI model to use")

	// Other flags
	dryRun := fs.Bool("dry-run", false, "Preview generation without saving")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd generate [flags]

Generate mock configurations using AI.

Flags:
  -i, --input        Input OpenAPI spec file
  -p, --prompt       Natural language description for generation
  -o, --output       Output file (default: stdout)
      --ai           Enable AI-powered data generation
      --provider     AI provider (openai, anthropic, ollama, openrouter)
      --model        AI model to use
      --dry-run      Preview generation without saving
      --admin-url    Admin API base URL (default: http://localhost:4290)

Environment Variables:
  MOCKD_AI_PROVIDER  Default AI provider
  MOCKD_AI_API_KEY   API key for the provider
  MOCKD_AI_MODEL     Default model
  MOCKD_AI_ENDPOINT  Custom endpoint (for Ollama, or override for any provider)

Examples:
  # Generate mocks from OpenAPI spec with AI enhancement
  mockd generate --ai --input openapi.yaml -o mocks.yaml

  # Generate mocks from natural language description
  mockd generate --ai --prompt "user management API with CRUD operations"

  # Generate mocks using specific provider
  mockd generate --ai --provider openai --prompt "payment processing API"

  # Preview what would be generated
  mockd generate --ai --prompt "blog API" --dry-run
`)
	}

	// Reorder args so flags come before positional arguments
	reorderedArgs := reorderArgs(args, []string{"admin-url", "input", "i", "output", "o", "prompt", "p", "provider", "model"})

	if err := fs.Parse(reorderedArgs); err != nil {
		return err
	}

	// Validate input
	if *input == "" && *prompt == "" {
		return fmt.Errorf(`either --input or --prompt is required

Usage: mockd generate --ai --input openapi.yaml
       mockd generate --ai --prompt "API description"

Run 'mockd generate --help' for more options`)
	}

	// Check AI configuration
	if *aiFlag {
		cfg := buildAIConfig(*provider, *model)
		if err := cfg.Validate(); err != nil {
			return formatAIConfigError(err)
		}
	}

	// Handle OpenAPI input
	if *input != "" {
		return generateFromOpenAPI(*input, *output, *aiFlag, *provider, *model, *dryRun, *adminURL)
	}

	// Handle prompt-based generation
	return generateFromPrompt(*prompt, *output, *provider, *model, *dryRun, *adminURL)
}

func buildAIConfig(providerName, model string) *ai.Config {
	cfg := ai.ConfigFromEnv()
	if cfg == nil {
		cfg = &ai.Config{}
	}

	if providerName != "" {
		cfg.Provider = providerName
	}
	if model != "" {
		cfg.Model = model
	}

	// Apply defaults based on provider
	applyAIConfigDefaults(cfg)
	return cfg
}

// applyAIConfigDefaults sets default values based on the provider.
func applyAIConfigDefaults(c *ai.Config) {
	switch c.Provider {
	case ai.ProviderOpenAI:
		if c.Model == "" {
			c.Model = ai.DefaultOpenAIModel
		}
	case ai.ProviderAnthropic:
		if c.Model == "" {
			c.Model = ai.DefaultAnthropicModel
		}
	case ai.ProviderOllama:
		if c.Model == "" {
			c.Model = ai.DefaultOllamaModel
		}
		if c.Endpoint == "" {
			c.Endpoint = ai.DefaultOllamaEndpoint
		}
	case ai.ProviderOpenRouter:
		if c.Model == "" {
			c.Model = ai.DefaultOpenRouterModel
		}
		if c.Endpoint == "" {
			c.Endpoint = ai.DefaultOpenRouterEndpoint
		}
	}
}

func formatAIConfigError(err error) error {
	return fmt.Errorf(`AI provider not configured: %w

To use AI-powered generation, set these environment variables:
  export MOCKD_AI_PROVIDER=openai    # or anthropic, ollama, openrouter
  export MOCKD_AI_API_KEY=your-key   # not needed for ollama

Or specify on command line:
  mockd generate --ai --provider openai --prompt "..."

Supported providers: openai, anthropic, ollama, openrouter`, err)
}

func generateFromOpenAPI(inputFile, outputFile string, useAI bool, providerName, model string, dryRun bool, adminURL string) error {
	// Read input file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Import using OpenAPI importer
	importer := &portability.OpenAPIImporter{}
	collection, err := importer.Import(data)
	if err != nil {
		return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	fmt.Printf("Parsed %d endpoints from %s\n", len(collection.Mocks), inputFile)

	// Enhance with AI if requested
	if useAI {
		cfg := buildAIConfig(providerName, model)
		provider, err := ai.NewProvider(cfg)
		if err != nil {
			return fmt.Errorf("failed to create AI provider: %w", err)
		}

		gen := generator.New(provider)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		fmt.Printf("Enhancing mocks with AI (%s/%s)...\n", cfg.Provider, cfg.Model)

		enhanced := 0
		for _, mock := range collection.Mocks {
			if err := gen.EnhanceMock(ctx, mock); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to enhance %s: %v\n", mock.Name, err)
				continue
			}
			enhanced++
		}
		fmt.Printf("Enhanced %d mocks with AI-generated data\n", enhanced)
	}

	return outputMocks(collection, outputFile, dryRun, adminURL)
}

func generateFromPrompt(prompt, outputFile, providerName, model string, dryRun bool, adminURL string) error {
	cfg := buildAIConfig(providerName, model)
	provider, err := ai.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	gen := generator.New(provider)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("Generating mocks from description using %s/%s...\n", cfg.Provider, cfg.Model)

	mocks, err := gen.GenerateFromDescription(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate mocks: %w", err)
	}

	fmt.Printf("Generated %d mock endpoints\n", len(mocks))

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "AI Generated Mocks",
		Mocks:   mocks,
	}

	return outputMocks(collection, outputFile, dryRun, adminURL)
}

func outputMocks(collection *config.MockCollection, outputFile string, dryRun bool, _ string) error {
	// Validate all generated mocks before output
	var validMocks []*config.MockConfiguration
	for _, m := range collection.Mocks {
		if err := m.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid mock %q: %v\n", m.Name, err)
			continue
		}
		validMocks = append(validMocks, m)
	}
	if len(validMocks) < len(collection.Mocks) {
		fmt.Fprintf(os.Stderr, "Validated %d/%d mocks (%d skipped)\n", len(validMocks), len(collection.Mocks), len(collection.Mocks)-len(validMocks))
	}
	collection.Mocks = validMocks

	if dryRun {
		fmt.Println("\nDry run - mocks that would be created:")
		for _, mock := range collection.Mocks {
			method := "???"
			path := "???"
			if mock.HTTP != nil && mock.HTTP.Matcher != nil {
				method = mock.HTTP.Matcher.Method
				path = mock.HTTP.Matcher.Path
			}
			name := mock.Name
			if name == "" {
				name = mock.ID
			}
			fmt.Printf("  %s %s (%s)\n", method, path, name)
		}
		return nil
	}

	// Determine output format
	asYAML := true
	if outputFile != "" {
		ext := strings.ToLower(filepath.Ext(outputFile))
		asYAML = ext == ".yaml" || ext == ".yml"
	}

	// Export
	exporter := &portability.NativeExporter{AsYAML: asYAML}
	data, err := exporter.Export(collection)
	if err != nil {
		return fmt.Errorf("failed to export mocks: %w", err)
	}

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Wrote %d mocks to %s\n", len(collection.Mocks), outputFile)
	} else {
		fmt.Print(string(data))
	}

	return nil
}

// RunEnhance handles the enhance command for improving existing mocks with AI.
func RunEnhance(args []string) error {
	fs := flag.NewFlagSet("enhance", flag.ContinueOnError)

	// Flags
	aiFlag := fs.Bool("ai", false, "Enable AI-powered enhancement")
	providerFlag := fs.String("provider", "", "AI provider (openai, anthropic, ollama, openrouter)")
	modelFlag := fs.String("model", "", "AI model to use")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd enhance [flags]

Enhance existing mocks with AI-generated response data.

Flags:
      --ai           Enable AI-powered enhancement (required)
      --provider     AI provider (openai, anthropic, ollama, openrouter)
      --model        AI model to use
      --admin-url    Admin API base URL (default: http://localhost:4290)

Environment Variables:
  MOCKD_AI_PROVIDER  Default AI provider
  MOCKD_AI_API_KEY   API key for the provider
  MOCKD_AI_MODEL     Default model
  MOCKD_AI_ENDPOINT  Custom endpoint (for Ollama, or override for any provider)

Examples:
  # Enhance all mocks with AI-generated data
  mockd enhance --ai

  # Use specific provider
  mockd enhance --ai --provider anthropic
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*aiFlag {
		return fmt.Errorf(`--ai flag is required

Usage: mockd enhance --ai

Run 'mockd enhance --help' for more options`)
	}

	// Check AI configuration
	cfg := buildAIConfig(*providerFlag, *modelFlag)
	if err := cfg.Validate(); err != nil {
		return formatAIConfigError(err)
	}

	// Create AI provider
	aiProvider, err := ai.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	gen := generator.New(aiProvider)

	// Get existing mocks from server
	client := NewAdminClientWithAuth(*adminURL)
	mocks, err := client.ListMocks()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	if len(mocks) == 0 {
		fmt.Println("No mocks found to enhance")
		return nil
	}

	fmt.Printf("Found %d mocks, enhancing with AI (%s/%s)...\n", len(mocks), cfg.Provider, cfg.Model)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	enhanced := 0
	for _, mock := range mocks {
		if err := gen.EnhanceMock(ctx, mock); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to enhance %s: %v\n", mock.Name, err)
			continue
		}

		// Update the mock on the server
		// Note: This would require an UpdateMock method on the client
		// For now, we'll delete and recreate
		if err := client.DeleteMock(mock.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update %s: %v\n", mock.Name, err)
			continue
		}
		if _, err := client.CreateMock(mock); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to recreate %s: %v\n", mock.Name, err)
			continue
		}

		enhanced++
		mockMethod := ""
		mockPath := ""
		if mock.HTTP != nil && mock.HTTP.Matcher != nil {
			mockMethod = mock.HTTP.Matcher.Method
			mockPath = mock.HTTP.Matcher.Path
		}
		fmt.Printf("  Enhanced: %s %s\n", mockMethod, mockPath)
	}

	fmt.Printf("Enhanced %d/%d mocks\n", enhanced, len(mocks))
	return nil
}
