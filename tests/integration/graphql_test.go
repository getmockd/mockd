// Package integration provides integration tests for the mockd server.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// graphqlTestBundle groups GraphQL handler and server for tests
type graphqlTestBundle struct {
	Handler *graphql.Handler
	Server  *http.Server
	Port    int
	BaseURL string
}

// getFreeGraphQLPort returns an available port for GraphQL testing
func getFreeGraphQLPort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// setupGraphQLServer creates and starts a GraphQL server for testing
func setupGraphQLServer(t *testing.T, cfg *graphql.GraphQLConfig) *graphqlTestBundle {
	port := getFreeGraphQLPort(t)

	// Create handler using Endpoint helper
	handler, err := graphql.Endpoint(cfg)
	require.NoError(t, err, "Failed to create GraphQL endpoint")

	// Create HTTP server
	mux := http.NewServeMux()
	path := cfg.Path
	if path == "" {
		path = "/graphql"
	}
	mux.Handle(path, handler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("graphql test server error: %v", err)
		}
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	return &graphqlTestBundle{
		Handler: handler,
		Server:  server,
		Port:    port,
		BaseURL: fmt.Sprintf("http://localhost:%d%s", port, path),
	}
}

// graphqlRequest makes a GraphQL request and returns the response
func graphqlRequest(t *testing.T, url string, req *graphql.GraphQLRequest) *graphql.GraphQLResponse {
	t.Helper()

	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var gqlResp graphql.GraphQLResponse
	err = json.Unmarshal(respBody, &gqlResp)
	require.NoError(t, err, "Failed to unmarshal response: %s", string(respBody))

	return &gqlResp
}

// Full GraphQL schema for comprehensive testing
const testSchema = `
type Query {
	user(id: ID!): User
	users(limit: Int, offset: Int): [User!]!
	post(id: ID!): Post
	search(query: String!): [SearchResult!]!
}

type Mutation {
	createUser(input: CreateUserInput!): User
	updateUser(id: ID!, input: UpdateUserInput!): User
	deleteUser(id: ID!): DeleteResult!
}

type User {
	id: ID!
	name: String!
	email: String!
	role: Role!
	posts: [Post!]!
	profile: Profile
}

type Post {
	id: ID!
	title: String!
	content: String
	author: User
	comments: [Comment!]!
}

type Comment {
	id: ID!
	text: String!
	author: User
}

type Profile {
	bio: String
	avatar: String
}

type DeleteResult {
	success: Boolean!
	message: String
}

input CreateUserInput {
	name: String!
	email: String!
	role: Role
}

input UpdateUserInput {
	name: String
	email: String
	role: Role
}

enum Role {
	ADMIN
	USER
	GUEST
}

union SearchResult = User | Post
`

// ============================================================================
// User Story 1: Basic Query
// ============================================================================

func TestGraphQL_US1_BasicQuery(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-basic-query",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "{{args.id}}",
					"name":  "Test User",
					"email": "test@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "1") { name email } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors, "Expected no errors")
	assert.NotNil(t, resp.Data, "Expected data in response")

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	assert.Equal(t, "Test User", user["name"])
	assert.Equal(t, "test@example.com", user["email"])
}

func TestGraphQL_US1_BasicQueryWithAllFields(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-basic-query-all-fields",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "user-123",
					"name":  "John Doe",
					"email": "john@example.com",
					"role":  "ADMIN",
					"posts": []interface{}{},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "123") { id name email role } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	assert.Equal(t, "user-123", user["id"])
	assert.Equal(t, "John Doe", user["name"])
	assert.Equal(t, "john@example.com", user["email"])
	assert.Equal(t, "ADMIN", user["role"])
}

// ============================================================================
// User Story 2: Query with Arguments
// ============================================================================

func TestGraphQL_US2_QueryWithArguments(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-query-with-args",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "User 1", "email": "user1@example.com", "role": "USER"},
					map[string]interface{}{"id": "2", "name": "User 2", "email": "user2@example.com", "role": "ADMIN"},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { users(limit: 10, offset: 0) { id name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	users := data["users"].([]interface{})

	assert.Len(t, users, 2)

	user1 := users[0].(map[string]interface{})
	assert.Equal(t, "1", user1["id"])
	assert.Equal(t, "User 1", user1["name"])
}

func TestGraphQL_US2_QueryArgumentSubstitution(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-arg-substitution",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "{{args.id}}",
					"name":  "User {{args.id}}",
					"email": "user{{args.id}}@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "42") { id name email } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	assert.Equal(t, "42", user["id"])
	assert.Equal(t, "User 42", user["name"])
	assert.Equal(t, "user42@example.com", user["email"])
}

// ============================================================================
// User Story 3: Mutation
// ============================================================================

func TestGraphQL_US3_Mutation(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-mutation",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Mutation.createUser": {
				Response: map[string]interface{}{
					"id":    "new-user-456",
					"name":  "John",
					"email": "john@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `mutation { createUser(input: {name: "John", email: "john@example.com"}) { id name email } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["createUser"].(map[string]interface{})

	assert.Equal(t, "new-user-456", user["id"])
	assert.Equal(t, "John", user["name"])
	assert.Equal(t, "john@example.com", user["email"])
}

func TestGraphQL_US3_MutationWithGeneratedID(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-mutation-generated-id",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Mutation.createUser": {
				Response: map[string]interface{}{
					"id":    "{{uuid}}",
					"name":  "Created User",
					"email": "created@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `mutation { createUser(input: {name: "Test", email: "test@test.com"}) { id name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["createUser"].(map[string]interface{})

	// UUID should be a valid string, not the template
	id := user["id"].(string)
	assert.NotEqual(t, "{{uuid}}", id)
	assert.NotEmpty(t, id)
	// UUID format check: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	assert.Len(t, id, 36)
	assert.Contains(t, id, "-")
}

// ============================================================================
// User Story 4: Nested Queries
// ============================================================================

func TestGraphQL_US4_NestedQuery(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-nested-query",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "Test User",
					"email": "test@example.com",
					"role":  "USER",
					"posts": []interface{}{
						map[string]interface{}{
							"id":      "post-1",
							"title":   "First Post",
							"content": "Hello World",
							"comments": []interface{}{
								map[string]interface{}{
									"id":   "comment-1",
									"text": "Great post!",
								},
								map[string]interface{}{
									"id":   "comment-2",
									"text": "Thanks for sharing",
								},
							},
						},
						map[string]interface{}{
							"id":       "post-2",
							"title":    "Second Post",
							"content":  "More content",
							"comments": []interface{}{},
						},
					},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "1") { name posts { title comments { text } } } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	assert.Equal(t, "Test User", user["name"])

	posts := user["posts"].([]interface{})
	assert.Len(t, posts, 2)

	post1 := posts[0].(map[string]interface{})
	assert.Equal(t, "First Post", post1["title"])

	comments := post1["comments"].([]interface{})
	assert.Len(t, comments, 2)

	comment1 := comments[0].(map[string]interface{})
	assert.Equal(t, "Great post!", comment1["text"])
}

func TestGraphQL_US4_DeeplyNestedQuery(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-deeply-nested",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "Test User",
					"email": "test@example.com",
					"role":  "USER",
					"posts": []interface{}{
						map[string]interface{}{
							"id":    "post-1",
							"title": "First Post",
							"comments": []interface{}{
								map[string]interface{}{
									"id":   "comment-1",
									"text": "Nice!",
									"author": map[string]interface{}{
										"id":   "user-2",
										"name": "Commenter",
									},
								},
							},
						},
					},
					"profile": map[string]interface{}{
						"bio":    "A test user",
						"avatar": "https://example.com/avatar.png",
					},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { 
			user(id: "1") { 
				name 
				profile { bio avatar }
				posts { 
					title 
					comments { 
						text 
						author { name } 
					} 
				} 
			} 
		}`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	// Check profile
	profile := user["profile"].(map[string]interface{})
	assert.Equal(t, "A test user", profile["bio"])
	assert.Equal(t, "https://example.com/avatar.png", profile["avatar"])

	// Check nested comments with author
	posts := user["posts"].([]interface{})
	post := posts[0].(map[string]interface{})
	comments := post["comments"].([]interface{})
	comment := comments[0].(map[string]interface{})
	author := comment["author"].(map[string]interface{})
	assert.Equal(t, "Commenter", author["name"])
}

// ============================================================================
// User Story 5: Schema Introspection
// ============================================================================

func TestGraphQL_US5_SchemaIntrospection(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-introspection",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { __schema { types { name } } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	schema := data["__schema"].(map[string]interface{})
	types := schema["types"].([]interface{})

	// Collect type names
	typeNames := make([]string, 0)
	for _, t := range types {
		typeDef := t.(map[string]interface{})
		typeNames = append(typeNames, typeDef["name"].(string))
	}

	// Verify our types are present
	assert.Contains(t, typeNames, "Query")
	assert.Contains(t, typeNames, "Mutation")
	assert.Contains(t, typeNames, "User")
	assert.Contains(t, typeNames, "Post")
	assert.Contains(t, typeNames, "Comment")
	assert.Contains(t, typeNames, "Role")
}

func TestGraphQL_US5_TypeIntrospection(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-type-introspection",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { 
			__type(name: "User") { 
				name 
				kind 
				fields { 
					name 
					type { name kind } 
				} 
			} 
		}`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	typeInfo := data["__type"].(map[string]interface{})

	assert.Equal(t, "User", typeInfo["name"])
	assert.Equal(t, "OBJECT", typeInfo["kind"])

	fields := typeInfo["fields"].([]interface{})
	assert.NotEmpty(t, fields)

	// Find the 'name' field
	var nameField map[string]interface{}
	for _, f := range fields {
		field := f.(map[string]interface{})
		if field["name"] == "name" {
			nameField = field
			break
		}
	}
	assert.NotNil(t, nameField, "Expected 'name' field in User type")
}

func TestGraphQL_US5_IntrospectionDisabled(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-introspection-disabled",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: false,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { __schema { queryType { name } } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected error when introspection is disabled")
	assert.Contains(t, resp.Errors[0].Message, "introspection is disabled")
}

// ============================================================================
// User Story 6: Variables
// ============================================================================

func TestGraphQL_US6_Variables(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-variables",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "{{args.id}}",
					"name":  "Variable User",
					"email": "var@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query GetUser($id: ID!) { user(id: $id) { id name } }`,
		Variables: map[string]interface{}{
			"id": "123",
		},
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	assert.Equal(t, "123", user["id"])
	assert.Equal(t, "Variable User", user["name"])
}

func TestGraphQL_US6_MultipleVariables(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-multiple-variables",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "User 1", "email": "u1@example.com", "role": "USER"},
					map[string]interface{}{"id": "2", "name": "User 2", "email": "u2@example.com", "role": "ADMIN"},
					map[string]interface{}{"id": "3", "name": "User 3", "email": "u3@example.com", "role": "GUEST"},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query GetUsers($limit: Int, $offset: Int) { users(limit: $limit, offset: $offset) { id name } }`,
		Variables: map[string]interface{}{
			"limit":  10,
			"offset": 0,
		},
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	users := data["users"].([]interface{})
	assert.Len(t, users, 3)
}

func TestGraphQL_US6_InputObjectVariable(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-input-variable",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Mutation.createUser": {
				Response: map[string]interface{}{
					"id":    "new-123",
					"name":  "Variable Name",
					"email": "var@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `mutation CreateUser($input: CreateUserInput!) { createUser(input: $input) { id name email } }`,
		Variables: map[string]interface{}{
			"input": map[string]interface{}{
				"name":  "Variable Name",
				"email": "var@example.com",
			},
		},
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["createUser"].(map[string]interface{})

	assert.Equal(t, "new-123", user["id"])
	assert.Equal(t, "Variable Name", user["name"])
}

// ============================================================================
// User Story 7: Error Responses
// ============================================================================

func TestGraphQL_US7_FieldNotFound(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-field-not-found",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { nonExistentField }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected error for non-existent field")
	assert.Contains(t, resp.Errors[0].Message, "Cannot query field")
}

func TestGraphQL_US7_InvalidSyntax(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-invalid-syntax",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: `,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected parse error")
	assert.Contains(t, resp.Errors[0].Message, "parse error")
}

func TestGraphQL_US7_ConfiguredError(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-configured-error",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Error: &graphql.GraphQLErrorConfig{
					Message: "User not found",
					Extensions: map[string]interface{}{
						"code": "NOT_FOUND",
					},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "999") { id name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected configured error")
	assert.Equal(t, "User not found", resp.Errors[0].Message)
	assert.Equal(t, "NOT_FOUND", resp.Errors[0].Extensions["code"])

	// Data should have the field with null value
	data := resp.Data.(map[string]interface{})
	assert.Nil(t, data["user"])
}

func TestGraphQL_US7_MissingRequiredArgument(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-missing-arg",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user { id name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected error for missing required argument")
}

// ============================================================================
// User Story 8: Templating in Resolvers
// ============================================================================

func TestGraphQL_US8_UUIDTemplate(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-uuid-template",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Mutation.createUser": {
				Response: map[string]interface{}{
					"id":    "{{uuid}}",
					"name":  "Template User",
					"email": "template@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `mutation { createUser(input: {name: "Test", email: "test@test.com"}) { id name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["createUser"].(map[string]interface{})

	id := user["id"].(string)
	assert.NotEqual(t, "{{uuid}}", id, "UUID template should be processed")
	assert.Len(t, id, 36, "UUID should be 36 characters")

	// Make second request to verify unique UUIDs
	resp2 := graphqlRequest(t, bundle.BaseURL, req)
	data2 := resp2.Data.(map[string]interface{})
	user2 := data2["createUser"].(map[string]interface{})
	id2 := user2["id"].(string)

	assert.NotEqual(t, id, id2, "Each request should generate a unique UUID")
}

func TestGraphQL_US8_TimestampTemplate(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:      "test-timestamp-template",
		Path:    "/graphql",
		Schema:  testSchema,
		Enabled: true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "{{now}}",
					"email": "time@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "1") { name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	name := user["name"].(string)
	assert.NotEqual(t, "{{now}}", name, "Timestamp template should be processed")
	// The timestamp should look like a date/time
	assert.NotEmpty(t, name)
}

func TestGraphQL_US8_ArgTemplateSubstitution(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-arg-template",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "{{args.id}}",
					"name":  "User-{{args.id}}",
					"email": "user{{args.id}}@test.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "ABC123") { id name email } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	assert.Equal(t, "ABC123", user["id"])
	assert.Equal(t, "User-ABC123", user["name"])
	assert.Equal(t, "userABC123@test.com", user["email"])
}

// ============================================================================
// User Story 9: Delays
// ============================================================================

func TestGraphQL_US9_ResolverDelay(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-delay",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "Delayed User",
					"email": "delay@example.com",
					"role":  "USER",
				},
				Delay: "100ms",
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "1") { id name } }`,
	}

	start := time.Now()
	resp := graphqlRequest(t, bundle.BaseURL, req)
	elapsed := time.Since(start)

	assert.Empty(t, resp.Errors)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(100), "Request should take at least 100ms")
}

func TestGraphQL_US9_NoDelayFast(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-no-delay",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "Fast User",
					"email": "fast@example.com",
					"role":  "USER",
				},
				// No delay configured
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "1") { id name } }`,
	}

	start := time.Now()
	resp := graphqlRequest(t, bundle.BaseURL, req)
	elapsed := time.Since(start)

	assert.Empty(t, resp.Errors)
	assert.Less(t, elapsed.Milliseconds(), int64(100), "Request without delay should be fast")
}

// ============================================================================
// User Story 10: Multiple Operations
// ============================================================================

func TestGraphQL_US10_MultipleOperations(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-multiple-ops",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "user-1",
					"name":  "Single User",
					"email": "single@example.com",
					"role":  "USER",
				},
			},
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "User 1", "email": "u1@example.com", "role": "USER"},
					map[string]interface{}{"id": "2", "name": "User 2", "email": "u2@example.com", "role": "ADMIN"},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	// Test selecting the first operation
	req := &graphql.GraphQLRequest{
		Query: `
			query GetSingleUser { user(id: "1") { id name } }
			query GetAllUsers { users { id name } }
		`,
		OperationName: "GetSingleUser",
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	_, hasUser := data["user"]
	_, hasUsers := data["users"]

	assert.True(t, hasUser, "Expected 'user' field when GetSingleUser is selected")
	assert.False(t, hasUsers, "Should not have 'users' field when GetSingleUser is selected")
}

func TestGraphQL_US10_SelectSecondOperation(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-select-second-op",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "user-1",
					"name":  "Single User",
					"email": "single@example.com",
					"role":  "USER",
				},
			},
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "User 1", "email": "u1@example.com", "role": "USER"},
					map[string]interface{}{"id": "2", "name": "User 2", "email": "u2@example.com", "role": "ADMIN"},
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	// Test selecting the second operation
	req := &graphql.GraphQLRequest{
		Query: `
			query GetSingleUser { user(id: "1") { id name } }
			query GetAllUsers { users { id name } }
		`,
		OperationName: "GetAllUsers",
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	_, hasUser := data["user"]
	_, hasUsers := data["users"]

	assert.False(t, hasUser, "Should not have 'user' field when GetAllUsers is selected")
	assert.True(t, hasUsers, "Expected 'users' field when GetAllUsers is selected")
}

func TestGraphQL_US10_OperationNotFound(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-op-not-found",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `
			query GetUser { user(id: "1") { id } }
		`,
		OperationName: "NonExistentOperation",
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected error for non-existent operation")
	assert.Contains(t, resp.Errors[0].Message, "not found")
}

// ============================================================================
// Additional Tests: HTTP Methods and Content Types
// ============================================================================

func TestGraphQL_HTTP_GETRequest(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-get-request",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "get-user",
					"name":  "GET User",
					"email": "get@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	// Make GET request with query parameter (must be URL-encoded)
	query := url.QueryEscape(`query{user(id:"1"){id name}}`)
	reqURL := fmt.Sprintf("%s?query=%s", bundle.BaseURL, query)
	resp, err := http.Get(reqURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var gqlResp graphql.GraphQLResponse
	err = json.Unmarshal(body, &gqlResp)
	require.NoError(t, err)

	assert.Empty(t, gqlResp.Errors)
	data := gqlResp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})
	assert.Equal(t, "GET User", user["name"])
}

func TestGraphQL_HTTP_ContentTypeGraphQL(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-content-type",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "content-user",
					"name":  "Content User",
					"email": "content@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	// Make POST request with application/graphql content type
	query := `query { user(id: "1") { id name } }`
	req, err := http.NewRequest(http.MethodPost, bundle.BaseURL, strings.NewReader(query))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/graphql")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var gqlResp graphql.GraphQLResponse
	err = json.Unmarshal(body, &gqlResp)
	require.NoError(t, err)

	assert.Empty(t, gqlResp.Errors)
}

func TestGraphQL_HTTP_MethodNotAllowed(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-method-not-allowed",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	// PUT should not be allowed
	req, err := http.NewRequest(http.MethodPut, bundle.BaseURL, strings.NewReader("{}"))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestGraphQL_HTTP_OptionsRequest(t *testing.T) {
	// Note: CORS headers are now handled by the engine's CORSMiddleware, not by the handler.
	// This test verifies that the GraphQL handler returns 200 OK for OPTIONS preflight requests.
	// Actual CORS header testing should be done with the full engine setup.
	cfg := &graphql.GraphQLConfig{
		ID:            "test-options",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	// OPTIONS request for CORS preflight
	req, err := http.NewRequest(http.MethodOptions, bundle.BaseURL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Handler should accept OPTIONS and return 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Handler should NOT set CORS headers - that's the middleware's job
	// Note: When running through the full engine, CORS headers would be set by CORSMiddleware
}

// ============================================================================
// Additional Tests: Field Aliases
// ============================================================================

func TestGraphQL_FieldAlias(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-alias",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		Resolvers: map[string]graphql.ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "alias-user",
					"name":  "Aliased User",
					"email": "alias@example.com",
					"role":  "USER",
				},
			},
		},
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { 
			myUser: user(id: "1") { 
				userId: id 
				userName: name 
			} 
		}`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})

	// Check aliased field at top level
	_, hasUser := data["user"]
	assert.False(t, hasUser, "Should not have 'user' key, expected 'myUser'")

	myUser, hasMyUser := data["myUser"]
	assert.True(t, hasMyUser, "Expected 'myUser' alias")

	user := myUser.(map[string]interface{})
	// Note: The resolver returns the original field names, but the query uses aliases
	// The mock just returns the configured response structure
	assert.NotNil(t, user)
}

// ============================================================================
// Additional Tests: Empty Query
// ============================================================================

func TestGraphQL_EmptyQuery(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-empty-query",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: "",
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.NotEmpty(t, resp.Errors, "Expected error for empty query")
	assert.Contains(t, resp.Errors[0].Message, "query is required")
}

// ============================================================================
// Additional Tests: __typename
// ============================================================================

func TestGraphQL_Typename(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-typename",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { __typename }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Query", data["__typename"])
}

// ============================================================================
// Additional Tests: No Resolver Configured
// ============================================================================

func TestGraphQL_NoResolverReturnsNull(t *testing.T) {
	cfg := &graphql.GraphQLConfig{
		ID:            "test-no-resolver",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Enabled:       true,
		// No resolvers configured
	}

	bundle := setupGraphQLServer(t, cfg)

	req := &graphql.GraphQLRequest{
		Query: `query { user(id: "1") { id name } }`,
	}

	resp := graphqlRequest(t, bundle.BaseURL, req)

	// Should succeed but return null for the field
	assert.Empty(t, resp.Errors)

	data := resp.Data.(map[string]interface{})
	assert.Nil(t, data["user"], "Field without resolver should return null")
}
