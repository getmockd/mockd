package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/ai"
	"github.com/getmockd/mockd/pkg/ai/generator"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/portability"
	"github.com/spf13/cobra"
)

var (
	generateInput    string
	generatePrompt   string
	generateOutput   string
	generateAIFlag   bool
	generateProvider string
	generateModel    string
	generateDryRun   bool
	generateAdminURL string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate mock configurations using AI",
	Long: `Generate mock configurations using AI.

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
  mockd generate --ai --prompt "blog API" --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		input := &generateInput
		prompt := &generatePrompt
		outputFile := &generateOutput
		aiFlag := &generateAIFlag
		provider := &generateProvider
		model := &generateModel
		dryRun := &generateDryRun
		adminURL := &generateAdminURL

		// Validate input
		if *input == "" && *prompt == "" {
			return errors.New(`either --input or --prompt is required

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
			return generateFromOpenAPI(*input, *outputFile, *aiFlag, *provider, *model, *dryRun, *adminURL)
		}

		// Handle prompt-based generation
		return generateFromPrompt(*prompt, *outputFile, *provider, *model, *dryRun, *adminURL)
	},
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
				output.Warn("failed to enhance %s: %v", mock.Name, err)
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
			output.Warn("skipping invalid mock %q: %v", m.Name, err)
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

	// Determine output format and marshal directly to MockCollection format
	// so the output can be loaded back by `mockd serve --config`.
	// Note: We use config.ToYAML/ToJSON instead of NativeExporter because
	// NativeExporter produces NativeV1 format (with "endpoints" key) which
	// is not directly loadable by serve --config (which expects "mocks" key).
	var data []byte
	var err error
	if outputFile != "" {
		ext := strings.ToLower(filepath.Ext(outputFile))
		if ext == ".yaml" || ext == ".yml" {
			data, err = config.ToYAML(collection)
		} else {
			data, err = config.ToJSON(collection)
		}
	} else {
		// Default to YAML for stdout
		data, err = config.ToYAML(collection)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal mocks: %w", err)
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

var (
	enhanceAIFlag   bool
	enhanceProvider string
	enhanceModel    string
	enhanceAdminURL string
)

var enhanceCmd = &cobra.Command{
	Use:   "enhance",
	Short: "Enhance existing mocks with AI-generated response data",
	Long: `Enhance existing mocks with AI-generated response data.

Environment Variables:
  MOCKD_AI_PROVIDER  Default AI provider
  MOCKD_AI_API_KEY   API key for the provider
  MOCKD_AI_MODEL     Default model
  MOCKD_AI_ENDPOINT  Custom endpoint (for Ollama, or override for any provider)

Examples:
  # Enhance all mocks with AI-generated data
  mockd enhance --ai

  # Use specific provider
  mockd enhance --ai --provider anthropic`,
	RunE: func(cmd *cobra.Command, args []string) error {
		aiFlag := &enhanceAIFlag
		providerFlag := &enhanceProvider
		modelFlag := &enhanceModel
		adminURL := &enhanceAdminURL

		if !*aiFlag {
			return errors.New(`--ai flag is required

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
				output.Warn("failed to enhance %s: %v", mock.Name, err)
				continue
			}

			// Update the mock on the server
			// Note: This would require an UpdateMock method on the client
			// For now, we'll delete and recreate
			if err := client.DeleteMock(mock.ID); err != nil {
				output.Warn("failed to update %s: %v", mock.Name, err)
				continue
			}
			if _, err := client.CreateMock(mock); err != nil {
				output.Warn("failed to recreate %s: %v", mock.Name, err)
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
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&generateInput, "input", "i", "", "Input OpenAPI spec file")
	generateCmd.Flags().StringVarP(&generatePrompt, "prompt", "p", "", "Natural language description for generation")
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Output file (default: stdout)")
	generateCmd.Flags().BoolVar(&generateAIFlag, "ai", false, "Enable AI-powered data generation")
	generateCmd.Flags().StringVar(&generateProvider, "provider", "", "AI provider (openai, anthropic, ollama, openrouter)")
	generateCmd.Flags().StringVar(&generateModel, "model", "", "AI model to use")
	generateCmd.Flags().BoolVar(&generateDryRun, "dry-run", false, "Preview generation without saving")
	generateCmd.Flags().StringVar(&generateAdminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	rootCmd.AddCommand(enhanceCmd)
	enhanceCmd.Flags().BoolVar(&enhanceAIFlag, "ai", false, "Enable AI-powered enhancement")
	enhanceCmd.Flags().StringVar(&enhanceProvider, "provider", "", "AI provider (openai, anthropic, ollama, openrouter)")
	enhanceCmd.Flags().StringVar(&enhanceModel, "model", "", "AI model to use")
	enhanceCmd.Flags().StringVar(&enhanceAdminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
}
