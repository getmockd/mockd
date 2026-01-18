package graphql

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

const testSchema = `
type Query {
	user(id: ID!): User
	users: [User!]!
	post(id: ID!): Post
}

type Mutation {
	createUser(input: CreateUserInput!): User
	updateUser(id: ID!, input: UpdateUserInput!): User
	deleteUser(id: ID!): Boolean!
}

type Subscription {
	userCreated: User
	postUpdated(id: ID!): Post
}

type User {
	id: ID!
	name: String!
	email: String!
	role: Role!
	posts: [Post!]!
}

type Post {
	id: ID!
	title: String!
	content: String
	author: User!
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

interface Node {
	id: ID!
}

union SearchResult = User | Post

scalar DateTime
`

func TestParseSchema(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	if schema == nil {
		t.Fatal("ParseSchema() returned nil schema")
	}

	if schema.AST() == nil {
		t.Error("Schema.AST() returned nil")
	}

	if schema.Source() != testSchema {
		t.Error("Schema.Source() doesn't match input")
	}
}

func TestParseSchemaFile(t *testing.T) {
	// Create a temporary schema file
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.graphql")

	err := os.WriteFile(schemaPath, []byte(testSchema), 0644)
	if err != nil {
		t.Fatalf("Failed to create test schema file: %v", err)
	}

	schema, err := ParseSchemaFile(schemaPath)
	if err != nil {
		t.Fatalf("ParseSchemaFile() error = %v", err)
	}

	if schema == nil {
		t.Fatal("ParseSchemaFile() returned nil schema")
	}

	// Verify we can access schema content
	queries := schema.ListQueries()
	if len(queries) == 0 {
		t.Error("Expected queries from file-parsed schema")
	}
}

func TestParseSchemaFile_NotFound(t *testing.T) {
	_, err := ParseSchemaFile("/nonexistent/path/schema.graphql")
	if err == nil {
		t.Error("ParseSchemaFile() expected error for nonexistent file")
	}
}

func TestParseSchema_Invalid(t *testing.T) {
	_, err := ParseSchema("this is not valid GraphQL")
	if err == nil {
		t.Error("ParseSchema() expected error for invalid schema")
	}
}

func TestSchema_GetType(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	tests := []struct {
		name     string
		typeName string
		wantNil  bool
		wantKind ast.DefinitionKind
	}{
		{"User type", "User", false, ast.Object},
		{"Post type", "Post", false, ast.Object},
		{"Role enum", "Role", false, ast.Enum},
		{"CreateUserInput input", "CreateUserInput", false, ast.InputObject},
		{"Node interface", "Node", false, ast.Interface},
		{"SearchResult union", "SearchResult", false, ast.Union},
		{"DateTime scalar", "DateTime", false, ast.Scalar},
		{"Nonexistent type", "Nonexistent", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := schema.GetType(tt.typeName)
			if tt.wantNil {
				if def != nil {
					t.Errorf("GetType(%q) expected nil, got %v", tt.typeName, def)
				}
				return
			}
			if def == nil {
				t.Errorf("GetType(%q) returned nil", tt.typeName)
				return
			}
			if def.Kind != tt.wantKind {
				t.Errorf("GetType(%q).Kind = %v, want %v", tt.typeName, def.Kind, tt.wantKind)
			}
		})
	}
}

func TestSchema_GetQueryField(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	tests := []struct {
		name      string
		fieldName string
		wantNil   bool
	}{
		{"user query", "user", false},
		{"users query", "users", false},
		{"post query", "post", false},
		{"nonexistent query", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := schema.GetQueryField(tt.fieldName)
			if tt.wantNil && field != nil {
				t.Errorf("GetQueryField(%q) expected nil", tt.fieldName)
			}
			if !tt.wantNil && field == nil {
				t.Errorf("GetQueryField(%q) returned nil", tt.fieldName)
			}
		})
	}
}

func TestSchema_GetMutationField(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	tests := []struct {
		name      string
		fieldName string
		wantNil   bool
	}{
		{"createUser mutation", "createUser", false},
		{"updateUser mutation", "updateUser", false},
		{"deleteUser mutation", "deleteUser", false},
		{"nonexistent mutation", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := schema.GetMutationField(tt.fieldName)
			if tt.wantNil && field != nil {
				t.Errorf("GetMutationField(%q) expected nil", tt.fieldName)
			}
			if !tt.wantNil && field == nil {
				t.Errorf("GetMutationField(%q) returned nil", tt.fieldName)
			}
		})
	}
}

func TestSchema_GetSubscriptionField(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	tests := []struct {
		name      string
		fieldName string
		wantNil   bool
	}{
		{"userCreated subscription", "userCreated", false},
		{"postUpdated subscription", "postUpdated", false},
		{"nonexistent subscription", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := schema.GetSubscriptionField(tt.fieldName)
			if tt.wantNil && field != nil {
				t.Errorf("GetSubscriptionField(%q) expected nil", tt.fieldName)
			}
			if !tt.wantNil && field == nil {
				t.Errorf("GetSubscriptionField(%q) returned nil", tt.fieldName)
			}
		})
	}
}

func TestSchema_ListQueries(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	queries := schema.ListQueries()
	expected := []string{"post", "user", "users"}

	if len(queries) != len(expected) {
		t.Errorf("ListQueries() returned %d items, want %d", len(queries), len(expected))
	}

	for i, name := range expected {
		if queries[i] != name {
			t.Errorf("ListQueries()[%d] = %q, want %q", i, queries[i], name)
		}
	}
}

func TestSchema_ListMutations(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	mutations := schema.ListMutations()
	expected := []string{"createUser", "deleteUser", "updateUser"}

	if len(mutations) != len(expected) {
		t.Errorf("ListMutations() returned %d items, want %d", len(mutations), len(expected))
	}

	for i, name := range expected {
		if mutations[i] != name {
			t.Errorf("ListMutations()[%d] = %q, want %q", i, mutations[i], name)
		}
	}
}

func TestSchema_ListSubscriptions(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	subscriptions := schema.ListSubscriptions()
	expected := []string{"postUpdated", "userCreated"}

	if len(subscriptions) != len(expected) {
		t.Errorf("ListSubscriptions() returned %d items, want %d", len(subscriptions), len(expected))
	}

	for i, name := range expected {
		if subscriptions[i] != name {
			t.Errorf("ListSubscriptions()[%d] = %q, want %q", i, subscriptions[i], name)
		}
	}
}

func TestSchema_ListTypes(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	// List all types
	allTypes := schema.ListTypes()
	if len(allTypes) == 0 {
		t.Error("ListTypes() returned empty list")
	}

	// List only object types
	objectTypes := schema.ListTypes(ast.Object)
	expectedObjects := []string{"Mutation", "Post", "Query", "Subscription", "User"}

	for _, expected := range expectedObjects {
		found := false
		for _, name := range objectTypes {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListTypes(Object) missing %q", expected)
		}
	}

	// List only enums
	enumTypes := schema.ListTypes(ast.Enum)
	foundRole := false
	for _, name := range enumTypes {
		if name == "Role" {
			foundRole = true
			break
		}
	}
	if !foundRole {
		t.Error("ListTypes(Enum) missing 'Role'")
	}
}

func TestSchema_HasMethods(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	if !schema.HasQuery() {
		t.Error("HasQuery() = false, want true")
	}

	if !schema.HasMutation() {
		t.Error("HasMutation() = false, want true")
	}

	if !schema.HasSubscription() {
		t.Error("HasSubscription() = false, want true")
	}
}

func TestSchema_HasMethods_Minimal(t *testing.T) {
	minimalSchema := `
type Query {
	hello: String
}
`
	schema, err := ParseSchema(minimalSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	if !schema.HasQuery() {
		t.Error("HasQuery() = false, want true")
	}

	if schema.HasMutation() {
		t.Error("HasMutation() = true, want false")
	}

	if schema.HasSubscription() {
		t.Error("HasSubscription() = true, want false")
	}
}

func TestSchema_Validate(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	if err := schema.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestSchema_GetField(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	tests := []struct {
		name      string
		typeName  string
		fieldName string
		wantNil   bool
	}{
		{"User.id", "User", "id", false},
		{"User.name", "User", "name", false},
		{"User.posts", "User", "posts", false},
		{"Post.author", "Post", "author", false},
		{"User.nonexistent", "User", "nonexistent", true},
		{"Nonexistent.id", "Nonexistent", "id", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := schema.GetField(tt.typeName, tt.fieldName)
			if tt.wantNil && field != nil {
				t.Errorf("GetField(%q, %q) expected nil", tt.typeName, tt.fieldName)
			}
			if !tt.wantNil && field == nil {
				t.Errorf("GetField(%q, %q) returned nil", tt.typeName, tt.fieldName)
			}
		})
	}
}

func TestSchema_TypeChecks(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	tests := []struct {
		name        string
		typeName    string
		isScalar    bool
		isEnum      bool
		isInput     bool
		isObject    bool
		isInterface bool
		isUnion     bool
	}{
		{"String scalar", "String", true, false, false, false, false, false},
		{"Role enum", "Role", false, true, false, false, false, false},
		{"CreateUserInput input", "CreateUserInput", false, false, true, false, false, false},
		{"User object", "User", false, false, false, true, false, false},
		{"Node interface", "Node", false, false, false, false, true, false},
		{"SearchResult union", "SearchResult", false, false, false, false, false, true},
		{"DateTime custom scalar", "DateTime", true, false, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := schema.IsScalarType(tt.typeName); got != tt.isScalar {
				t.Errorf("IsScalarType(%q) = %v, want %v", tt.typeName, got, tt.isScalar)
			}
			if got := schema.IsEnumType(tt.typeName); got != tt.isEnum {
				t.Errorf("IsEnumType(%q) = %v, want %v", tt.typeName, got, tt.isEnum)
			}
			if got := schema.IsInputType(tt.typeName); got != tt.isInput {
				t.Errorf("IsInputType(%q) = %v, want %v", tt.typeName, got, tt.isInput)
			}
			if got := schema.IsObjectType(tt.typeName); got != tt.isObject {
				t.Errorf("IsObjectType(%q) = %v, want %v", tt.typeName, got, tt.isObject)
			}
			if got := schema.IsInterfaceType(tt.typeName); got != tt.isInterface {
				t.Errorf("IsInterfaceType(%q) = %v, want %v", tt.typeName, got, tt.isInterface)
			}
			if got := schema.IsUnionType(tt.typeName); got != tt.isUnion {
				t.Errorf("IsUnionType(%q) = %v, want %v", tt.typeName, got, tt.isUnion)
			}
		})
	}
}

func TestSchema_GetEnumValues(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	values := schema.GetEnumValues("Role")
	expected := []string{"ADMIN", "USER", "GUEST"}

	if len(values) != len(expected) {
		t.Errorf("GetEnumValues('Role') returned %d values, want %d", len(values), len(expected))
	}

	for i, v := range expected {
		if values[i] != v {
			t.Errorf("GetEnumValues('Role')[%d] = %q, want %q", i, values[i], v)
		}
	}

	// Non-enum type should return nil
	if values := schema.GetEnumValues("User"); values != nil {
		t.Errorf("GetEnumValues('User') = %v, want nil", values)
	}

	// Nonexistent type should return nil
	if values := schema.GetEnumValues("Nonexistent"); values != nil {
		t.Errorf("GetEnumValues('Nonexistent') = %v, want nil", values)
	}
}

func TestSchema_GetUnionMembers(t *testing.T) {
	schema, err := ParseSchema(testSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	members := schema.GetUnionMembers("SearchResult")
	if len(members) != 2 {
		t.Errorf("GetUnionMembers('SearchResult') returned %d members, want 2", len(members))
	}

	// Members should be sorted
	expected := []string{"Post", "User"}
	for i, m := range expected {
		if members[i] != m {
			t.Errorf("GetUnionMembers('SearchResult')[%d] = %q, want %q", i, members[i], m)
		}
	}

	// Non-union type should return nil
	if members := schema.GetUnionMembers("User"); members != nil {
		t.Errorf("GetUnionMembers('User') = %v, want nil", members)
	}
}

func TestFieldPath(t *testing.T) {
	tests := []struct {
		path      string
		typeName  string
		fieldName string
		str       string
	}{
		{"Query.user", "Query", "user", "Query.user"},
		{"Mutation.createUser", "Mutation", "createUser", "Mutation.createUser"},
		{"User.posts", "User", "posts", "User.posts"},
		{"fieldOnly", "", "fieldOnly", ".fieldOnly"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			fp := ParseFieldPath(tt.path)
			if fp.TypeName != tt.typeName {
				t.Errorf("ParseFieldPath(%q).TypeName = %q, want %q", tt.path, fp.TypeName, tt.typeName)
			}
			if fp.FieldName != tt.fieldName {
				t.Errorf("ParseFieldPath(%q).FieldName = %q, want %q", tt.path, fp.FieldName, tt.fieldName)
			}
			if fp.String() != tt.str {
				t.Errorf("ParseFieldPath(%q).String() = %q, want %q", tt.path, fp.String(), tt.str)
			}
		})
	}
}

func TestGraphQLConfig(t *testing.T) {
	config := GraphQLConfig{
		ID:            "test-api",
		Path:          "/graphql",
		Schema:        testSchema,
		Introspection: true,
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test User",
				},
				Delay: "100ms",
			},
			"Query.users": {
				Error: &GraphQLErrorConfig{
					Message: "Not authorized",
					Extensions: map[string]interface{}{
						"code": "UNAUTHORIZED",
					},
				},
			},
		},
		Enabled: true,
	}

	if config.ID != "test-api" {
		t.Errorf("GraphQLConfig.ID = %q, want %q", config.ID, "test-api")
	}

	if len(config.Resolvers) != 2 {
		t.Errorf("GraphQLConfig.Resolvers has %d entries, want 2", len(config.Resolvers))
	}

	userResolver := config.Resolvers["Query.user"]
	if userResolver.Delay != "100ms" {
		t.Errorf("Resolver delay = %q, want %q", userResolver.Delay, "100ms")
	}

	usersResolver := config.Resolvers["Query.users"]
	if usersResolver.Error == nil {
		t.Error("Query.users resolver should have error config")
	} else if usersResolver.Error.Message != "Not authorized" {
		t.Errorf("Error message = %q, want %q", usersResolver.Error.Message, "Not authorized")
	}
}

func TestResolverMatch(t *testing.T) {
	match := ResolverMatch{
		Args: map[string]interface{}{
			"id":   "123",
			"role": "ADMIN",
		},
	}

	if match.Args["id"] != "123" {
		t.Errorf("ResolverMatch.Args['id'] = %v, want '123'", match.Args["id"])
	}
}
