package graphql

import (
	"context"
	"strings"
	"testing"
	"time"
)

const executorTestSchema = `
type Query {
	user(id: ID!): User
	users(role: Role): [User!]!
	post(id: ID!): Post
	search(query: String!): [SearchResult!]!
}

type Mutation {
	createUser(input: CreateUserInput!): User
	updateUser(id: ID!, input: UpdateUserInput!): User
	deleteUser(id: ID!): Boolean!
}

type User {
	id: ID!
	name: String!
	email: String!
	role: Role!
}

type Post {
	id: ID!
	title: String!
	content: String
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

func TestNewExecutor(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test User",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)
	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}

	if executor.schema != schema {
		t.Error("executor.schema not set correctly")
	}

	if len(executor.resolvers) != 1 {
		t.Errorf("executor.resolvers has %d entries, want 1", len(executor.resolvers))
	}
}

func TestExecutor_Execute_SimpleQuery(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "John Doe",
					"email": "john@example.com",
					"role":  "USER",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				user(id: "1") {
					id
					name
					email
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Execute() data is not a map, got %T", resp.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("data['user'] is not a map, got %T", data["user"])
	}

	if user["id"] != "1" {
		t.Errorf("user.id = %v, want '1'", user["id"])
	}
	if user["name"] != "John Doe" {
		t.Errorf("user.name = %v, want 'John Doe'", user["name"])
	}
}

func TestExecutor_Execute_WithVariables(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "{{args.id}}",
					"name": "User {{args.id}}",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query GetUser($userId: ID!) {
				user(id: $userId) {
					id
					name
				}
			}
		`,
		Variables: map[string]interface{}{
			"userId": "42",
		},
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	if user["id"] != "42" {
		t.Errorf("user.id = %v, want '42'", user["id"])
	}
	if user["name"] != "User 42" {
		t.Errorf("user.name = %v, want 'User 42'", user["name"])
	}
}

func TestExecutor_Execute_TemplateSubstitution(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "{{args.id}}",
					"name":  "User {{args.id}}",
					"email": "user{{args.id}}@example.com",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				user(id: "123") {
					id
					name
					email
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	if user["id"] != "123" {
		t.Errorf("user.id = %v, want '123'", user["id"])
	}
	if user["name"] != "User 123" {
		t.Errorf("user.name = %v, want 'User 123'", user["name"])
	}
	if user["email"] != "user123@example.com" {
		t.Errorf("user.email = %v, want 'user123@example.com'", user["email"])
	}
}

func TestExecutor_Execute_Mutation(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Mutation.createUser": {
				Response: map[string]interface{}{
					"id":    "new-123",
					"name":  "New User",
					"email": "new@example.com",
					"role":  "USER",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			mutation {
				createUser(input: {name: "New User", email: "new@example.com"}) {
					id
					name
					email
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	user := data["createUser"].(map[string]interface{})

	if user["id"] != "new-123" {
		t.Errorf("createUser.id = %v, want 'new-123'", user["id"])
	}
}

func TestExecutor_Execute_ErrorResponse(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Error: &GraphQLErrorConfig{
					Message: "User not found",
					Extensions: map[string]interface{}{
						"code": "NOT_FOUND",
					},
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				user(id: "999") {
					id
					name
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) == 0 {
		t.Fatal("Execute() expected errors")
	}

	if resp.Errors[0].Message != "User not found" {
		t.Errorf("error.Message = %v, want 'User not found'", resp.Errors[0].Message)
	}

	if resp.Errors[0].Extensions["code"] != "NOT_FOUND" {
		t.Errorf("error.Extensions.code = %v, want 'NOT_FOUND'", resp.Errors[0].Extensions["code"])
	}

	// Data should have null for the field
	data := resp.Data.(map[string]interface{})
	if data["user"] != nil {
		t.Errorf("data.user = %v, want nil", data["user"])
	}
}

func TestExecutor_Execute_Delay(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test",
				},
				Delay: "50ms",
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `query { user(id: "1") { id } }`,
	}

	start := time.Now()
	resp := executor.Execute(context.Background(), req)
	elapsed := time.Since(start)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	if elapsed < 50*time.Millisecond {
		t.Errorf("Execute() took %v, expected at least 50ms", elapsed)
	}
}

func TestExecutor_Execute_OperationName(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query GetUser {
				user(id: "1") { id name }
			}
			query GetPost {
				post(id: "1") { id title }
			}
		`,
		OperationName: "GetUser",
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	if _, ok := data["user"]; !ok {
		t.Error("Expected 'user' field in response")
	}
	if _, ok := data["post"]; ok {
		t.Error("Did not expect 'post' field in response")
	}
}

func TestExecutor_Execute_InvalidQuery(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	executor := NewExecutor(schema, nil)

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "syntax error",
			query: "query { user(",
			want:  "parse error",
		},
		{
			name:  "unknown field",
			query: "query { unknownField }",
			want:  "Cannot query field",
		},
		{
			name:  "empty query",
			query: "",
			want:  "query is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &GraphQLRequest{Query: tt.query}
			resp := executor.Execute(context.Background(), req)

			if len(resp.Errors) == 0 {
				t.Fatal("Execute() expected error")
			}

			if !strings.Contains(resp.Errors[0].Message, tt.want) {
				t.Errorf("error.Message = %q, want to contain %q", resp.Errors[0].Message, tt.want)
			}
		})
	}
}

func TestExecutor_Execute_NilRequest(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	executor := NewExecutor(schema, nil)
	resp := executor.Execute(context.Background(), nil)

	if len(resp.Errors) == 0 {
		t.Fatal("Execute() expected error for nil request")
	}
}

func TestExecutor_Execute_Introspection(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Introspection: true,
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				__schema {
					queryType { name }
					mutationType { name }
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	schemaData := data["__schema"].(map[string]interface{})

	queryType := schemaData["queryType"].(map[string]interface{})
	if queryType["name"] != "Query" {
		t.Errorf("queryType.name = %v, want 'Query'", queryType["name"])
	}

	mutationType := schemaData["mutationType"].(map[string]interface{})
	if mutationType["name"] != "Mutation" {
		t.Errorf("mutationType.name = %v, want 'Mutation'", mutationType["name"])
	}
}

func TestExecutor_Execute_IntrospectionDisabled(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Introspection: false,
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				__schema {
					queryType { name }
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) == 0 {
		t.Fatal("Execute() expected error when introspection is disabled")
	}

	if !strings.Contains(resp.Errors[0].Message, "introspection is disabled") {
		t.Errorf("error.Message = %q, want to contain 'introspection is disabled'", resp.Errors[0].Message)
	}
}

func TestExecutor_Execute_TypenameField(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	executor := NewExecutor(schema, nil)

	req := &GraphQLRequest{
		Query: `
			query {
				__typename
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	if data["__typename"] != "Query" {
		t.Errorf("__typename = %v, want 'Query'", data["__typename"])
	}
}

func TestExecutor_Execute_FieldAlias(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test User",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				myUser: user(id: "1") {
					id
					name
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})

	// Check that the top-level aliased field name is used
	if _, ok := data["user"]; ok {
		t.Error("data should not have 'user' key, expected 'myUser'")
	}

	myUser, ok := data["myUser"].(map[string]interface{})
	if !ok {
		t.Fatalf("data['myUser'] is not a map, got %T", data["myUser"])
	}

	// The resolver returns the full response; we check the original field names
	if myUser["id"] != "1" {
		t.Errorf("myUser.id = %v, want '1'", myUser["id"])
	}
	if myUser["name"] != "Test User" {
		t.Errorf("myUser.name = %v, want 'Test User'", myUser["name"])
	}
}

func TestExecutor_Execute_ConditionalResolver(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	// Note: The current implementation uses a single resolver per path from config map
	// This test verifies basic matching behavior
	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Default User",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				user(id: "1") {
					id
					name
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})

	if user["name"] != "Default User" {
		t.Errorf("user.name = %v, want 'Default User'", user["name"])
	}
}

func TestExecutor_Execute_NoResolver(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	// No resolvers configured
	executor := NewExecutor(schema, nil)

	req := &GraphQLRequest{
		Query: `
			query {
				user(id: "1") {
					id
					name
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	// Should succeed but return null for the field
	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	if data["user"] != nil {
		t.Errorf("user = %v, want nil (no resolver configured)", data["user"])
	}
}

func TestExecutor_Execute_ArrayResponse(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "User 1", "email": "user1@example.com", "role": "USER"},
					map[string]interface{}{"id": "2", "name": "User 2", "email": "user2@example.com", "role": "ADMIN"},
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				users {
					id
					name
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	users, ok := data["users"].([]interface{})
	if !ok {
		t.Fatalf("data['users'] is not an array, got %T", data["users"])
	}

	if len(users) != 2 {
		t.Errorf("len(users) = %d, want 2", len(users))
	}
}

func TestExecutor_Execute_ContextCancellation(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test",
				},
				Delay: "1s",
			},
		},
	}

	executor := NewExecutor(schema, config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req := &GraphQLRequest{
		Query: `query { user(id: "1") { id } }`,
	}

	start := time.Now()
	resp := executor.Execute(ctx, req)
	elapsed := time.Since(start)

	// Should complete quickly due to cancellation
	if elapsed > 500*time.Millisecond {
		t.Errorf("Execute() took %v, expected quick cancellation", elapsed)
	}

	// Should have an error about cancellation
	if len(resp.Errors) == 0 {
		t.Error("Execute() expected error for cancelled context")
	}
}

func TestExecutor_Execute_TypeIntrospection(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Introspection: true,
	}

	executor := NewExecutor(schema, config)

	req := &GraphQLRequest{
		Query: `
			query {
				__type(name: "User") {
					name
					kind
					fields {
						name
						type {
							name
							kind
						}
					}
				}
			}
		`,
	}

	resp := executor.Execute(context.Background(), req)

	if len(resp.Errors) > 0 {
		t.Errorf("Execute() returned errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	typeData := data["__type"].(map[string]interface{})

	if typeData["name"] != "User" {
		t.Errorf("__type.name = %v, want 'User'", typeData["name"])
	}
	if typeData["kind"] != "OBJECT" {
		t.Errorf("__type.kind = %v, want 'OBJECT'", typeData["kind"])
	}

	fields, ok := typeData["fields"].([]interface{})
	if !ok {
		t.Fatalf("__type.fields is not an array, got %T", typeData["fields"])
	}

	if len(fields) < 1 {
		t.Error("Expected at least one field in User type")
	}
}
