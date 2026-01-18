package portability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getmockd/mockd/pkg/ai"
	"github.com/getmockd/mockd/pkg/ai/generator"
	"github.com/getmockd/mockd/pkg/config"
)

// OpenAPIAIImporter imports OpenAPI specifications with AI-enhanced response data.
type OpenAPIAIImporter struct {
	provider ai.Provider
}

// NewOpenAPIAIImporter creates a new OpenAPI importer with AI enhancement.
func NewOpenAPIAIImporter(provider ai.Provider) *OpenAPIAIImporter {
	return &OpenAPIAIImporter{
		provider: provider,
	}
}

// Import parses an OpenAPI specification and returns a MockCollection with AI-generated data.
func (i *OpenAPIAIImporter) Import(data []byte) (*config.MockCollection, error) {
	// First, use the standard OpenAPI import
	standardImporter := &OpenAPIImporter{}
	collection, err := standardImporter.Import(data)
	if err != nil {
		return nil, err
	}

	// Then enhance with AI-generated data
	if i.provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		gen := generator.New(i.provider)

		// Load the OpenAPI spec for schema information
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromData(data)
		if err == nil {
			// Enhance each mock with AI-generated response bodies
			for _, mock := range collection.Mocks {
				if err := i.enhanceMockFromSpec(ctx, gen, doc, mock); err != nil {
					// Log warning but continue
					continue
				}
			}
		}
	}

	return collection, nil
}

// enhanceMockFromSpec enhances a mock configuration using the OpenAPI schema.
func (i *OpenAPIAIImporter) enhanceMockFromSpec(ctx context.Context, gen *generator.Generator, doc *openapi3.T, mock *config.MockConfiguration) error {
	if mock.HTTP == nil || mock.HTTP.Matcher == nil || mock.HTTP.Response == nil {
		return nil
	}

	// Find the path item in the spec
	pathItem := doc.Paths.Find(convertMockdPath(mock.HTTP.Matcher.Path))
	if pathItem == nil {
		return nil
	}

	// Get the operation for this method
	var op *openapi3.Operation
	switch mock.HTTP.Matcher.Method {
	case "GET":
		op = pathItem.Get
	case "POST":
		op = pathItem.Post
	case "PUT":
		op = pathItem.Put
	case "DELETE":
		op = pathItem.Delete
	case "PATCH":
		op = pathItem.Patch
	case "HEAD":
		op = pathItem.Head
	case "OPTIONS":
		op = pathItem.Options
	}

	if op == nil {
		return nil
	}

	// Find the response schema for the status code
	statusStr := fmt.Sprintf("%d", mock.HTTP.Response.StatusCode)
	responseRef := op.Responses.Status(mock.HTTP.Response.StatusCode)
	if responseRef == nil {
		responseRef = op.Responses.Status(0) // Default response
	}
	if responseRef == nil || responseRef.Value == nil {
		return nil
	}

	// Find the content schema (prefer application/json)
	var schema *openapi3.Schema
	for contentType, mediaType := range responseRef.Value.Content {
		if contentType == "application/json" || contentType == "application/json; charset=utf-8" {
			if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				schema = mediaType.Schema.Value
				break
			}
		}
	}

	if schema == nil {
		return nil
	}

	// Generate enhanced response using AI
	value, err := gen.EnhanceOpenAPISchemaRecursive(ctx, schema, statusStr)
	if err != nil {
		return err
	}

	// Convert to JSON body
	bodyBytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	mock.HTTP.Response.Body = string(bodyBytes)
	return nil
}

// Format returns FormatOpenAPI.
func (i *OpenAPIAIImporter) Format() Format {
	return FormatOpenAPI
}
