// Package generator provides high-level utilities for AI-powered mock generation.
//
// This package builds on top of the ai package to provide:
//   - OpenAPI schema enhancement with realistic data
//   - Mock generation from natural language descriptions
//   - Existing mock enhancement with AI-generated values
//
// # Usage
//
// Create a generator with an AI provider:
//
//	cfg := ai.ConfigFromEnv()
//	provider, _ := ai.NewProvider(cfg)
//	gen := generator.New(provider)
//
// Generate mocks from a description:
//
//	mocks, err := gen.GenerateFromDescription(ctx, "user management API with CRUD operations")
//
// Enhance an OpenAPI schema:
//
//	schema := &openapi3.Schema{...}
//	value, err := gen.EnhanceOpenAPISchema(ctx, schema)
//
// Enhance existing mocks:
//
//	mock := &config.MockConfiguration{...}
//	err := gen.EnhanceMock(ctx, mock)
package generator
