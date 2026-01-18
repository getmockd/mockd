// Package ai provides AI-powered mock data generation for mockd.
//
// This package enables users to:
//   - Generate realistic mock data using LLM providers
//   - Enhance OpenAPI schema imports with AI-generated values
//   - Create mock configurations from natural language descriptions
//
// # Supported Providers
//
// The following AI providers are supported:
//   - OpenAI (GPT-3.5, GPT-4, etc.)
//   - Anthropic (Claude models)
//   - Ollama (local models)
//
// # Configuration
//
// Configuration is read from environment variables:
//   - MOCKD_AI_PROVIDER: Provider name ("openai", "anthropic", "ollama")
//   - MOCKD_AI_API_KEY: API key for the provider
//   - MOCKD_AI_MODEL: Model name (e.g., "gpt-4", "claude-3-sonnet")
//   - MOCKD_AI_ENDPOINT: Custom endpoint URL (required for Ollama)
//
// # Usage
//
// Basic usage with configuration:
//
//	cfg := ai.ConfigFromEnv()
//	provider, err := ai.NewProvider(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	resp, err := provider.Generate(ctx, &ai.GenerateRequest{
//	    FieldName: "email",
//	    FieldType: "string",
//	    Context:   "User profile data",
//	})
//
// # Generator
//
// The generator subpackage provides higher-level utilities:
//
//	gen := generator.New(provider)
//	mocks, err := gen.GenerateFromDescription("user management API with CRUD operations")
package ai
