package graphql

import (
	"context"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

// makeDirective builds a single ast.Directive with a boolean "if" argument.
func makeDirective(name string, ifValue bool) *ast.Directive {
	raw := "false"
	if ifValue {
		raw = "true"
	}
	return &ast.Directive{
		Name: name,
		Arguments: ast.ArgumentList{
			{
				Name: "if",
				Value: &ast.Value{
					Kind: ast.BooleanValue,
					Raw:  raw,
				},
			},
		},
	}
}

// makeVariableDirective builds a directive whose "if" argument references a variable.
func makeVariableDirective(name, varName string) *ast.Directive {
	return &ast.Directive{
		Name: name,
		Arguments: ast.ArgumentList{
			{
				Name: "if",
				Value: &ast.Value{
					Kind: ast.Variable,
					Raw:  varName,
				},
			},
		},
	}
}

func TestShouldSkipField(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}
	executor := NewExecutor(schema, nil)

	tests := []struct {
		name       string
		directives ast.DirectiveList
		variables  map[string]interface{}
		wantSkip   bool
	}{
		{
			name:       "no directives",
			directives: nil,
			wantSkip:   false,
		},
		{
			name:       "skip(if: true)",
			directives: ast.DirectiveList{makeDirective("skip", true)},
			wantSkip:   true,
		},
		{
			name:       "skip(if: false)",
			directives: ast.DirectiveList{makeDirective("skip", false)},
			wantSkip:   false,
		},
		{
			name:       "include(if: true)",
			directives: ast.DirectiveList{makeDirective("include", true)},
			wantSkip:   false,
		},
		{
			name:       "include(if: false)",
			directives: ast.DirectiveList{makeDirective("include", false)},
			wantSkip:   true,
		},
		{
			name: "skip(if: true) + include(if: true) — skip wins",
			directives: ast.DirectiveList{
				makeDirective("skip", true),
				makeDirective("include", true),
			},
			wantSkip: true,
		},
		{
			name: "skip(if: false) + include(if: false) — include excludes",
			directives: ast.DirectiveList{
				makeDirective("skip", false),
				makeDirective("include", false),
			},
			wantSkip: true,
		},
		{
			name: "skip(if: false) + include(if: true) — neither excludes",
			directives: ast.DirectiveList{
				makeDirective("skip", false),
				makeDirective("include", true),
			},
			wantSkip: false,
		},
		{
			name:       "variable reference — skip(if: $doSkip) with doSkip=true",
			directives: ast.DirectiveList{makeVariableDirective("skip", "doSkip")},
			variables:  map[string]interface{}{"doSkip": true},
			wantSkip:   true,
		},
		{
			name:       "variable reference — skip(if: $doSkip) with doSkip=false",
			directives: ast.DirectiveList{makeVariableDirective("skip", "doSkip")},
			variables:  map[string]interface{}{"doSkip": false},
			wantSkip:   false,
		},
		{
			name:       "variable reference — include(if: $show) with show=true",
			directives: ast.DirectiveList{makeVariableDirective("include", "show")},
			variables:  map[string]interface{}{"show": true},
			wantSkip:   false,
		},
		{
			name:       "variable reference — include(if: $show) with show=false",
			directives: ast.DirectiveList{makeVariableDirective("include", "show")},
			variables:  map[string]interface{}{"show": false},
			wantSkip:   true,
		},
		{
			name:       "variable reference — undefined variable treated as null (skip)",
			directives: ast.DirectiveList{makeVariableDirective("skip", "missing")},
			variables:  map[string]interface{}{},
			wantSkip:   false, // null → false → don't skip
		},
		{
			name:       "variable reference — undefined variable treated as null (include)",
			directives: ast.DirectiveList{makeVariableDirective("include", "missing")},
			variables:  map[string]interface{}{},
			wantSkip:   true, // null → false → don't include → skip
		},
		{
			name:       "include with nil if argument value",
			directives: ast.DirectiveList{{Name: "include", Arguments: ast.ArgumentList{{Name: "if", Value: &ast.Value{Kind: ast.NullValue, Raw: "null"}}}}},
			wantSkip:   true, // null → don't include → skip
		},
		{
			name:       "empty directive list",
			directives: ast.DirectiveList{},
			wantSkip:   false,
		},
		{
			name:       "unrelated directive is ignored",
			directives: ast.DirectiveList{{Name: "deprecated", Arguments: nil}},
			wantSkip:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.shouldSkipField(tt.directives, tt.variables)
			if got != tt.wantSkip {
				t.Errorf("shouldSkipField() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}

// TestPruneResponse_ViaExecute tests pruneResponse indirectly through the full Execute path.
// This is the realistic usage: a resolver returns a full response object, and fields with
// @skip/@include directives are pruned from the result.
func TestPruneResponse_ViaExecute(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "1",
					"name":  "Alice",
					"email": "alice@example.com",
					"role":  "ADMIN",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	t.Run("skip(if: true) removes field from response", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query {
				user(id: "1") {
					id
					name
					email @skip(if: true)
				}
			}`,
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		if _, ok := user["email"]; ok {
			t.Error("email should be pruned by @skip(if: true)")
		}
		if user["id"] != "1" {
			t.Errorf("id = %v, want '1'", user["id"])
		}
		if user["name"] != "Alice" {
			t.Errorf("name = %v, want 'Alice'", user["name"])
		}
	})

	t.Run("skip(if: false) keeps field in response", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query {
				user(id: "1") {
					id
					name
					email @skip(if: false)
				}
			}`,
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		if user["email"] != "alice@example.com" {
			t.Errorf("email = %v, want 'alice@example.com'", user["email"])
		}
	})

	t.Run("include(if: false) removes field from response", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query {
				user(id: "1") {
					id
					name @include(if: false)
					email
				}
			}`,
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		if _, ok := user["name"]; ok {
			t.Error("name should be pruned by @include(if: false)")
		}
		if user["email"] != "alice@example.com" {
			t.Errorf("email = %v, want 'alice@example.com'", user["email"])
		}
	})

	t.Run("include(if: true) keeps field in response", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query {
				user(id: "1") {
					id
					name @include(if: true)
				}
			}`,
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		if user["name"] != "Alice" {
			t.Errorf("name = %v, want 'Alice'", user["name"])
		}
	})

	t.Run("skip with variable reference", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query GetUser($hideEmail: Boolean!) {
				user(id: "1") {
					id
					name
					email @skip(if: $hideEmail)
				}
			}`,
			Variables: map[string]interface{}{
				"hideEmail": true,
			},
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		if _, ok := user["email"]; ok {
			t.Error("email should be pruned when $hideEmail=true")
		}
	})

	t.Run("all fields skipped leaves empty object", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query {
				user(id: "1") {
					id @skip(if: true)
					name @skip(if: true)
					email @skip(if: true)
				}
			}`,
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		if len(user) != 0 {
			t.Errorf("expected empty user object, got %v", user)
		}
	})

	t.Run("no directives returns all selected fields", func(t *testing.T) {
		resp := executor.Execute(context.Background(), &GraphQLRequest{
			Query: `query {
				user(id: "1") {
					id
					name
					email
					role
				}
			}`,
		})
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", resp.Errors)
		}
		data := resp.Data.(map[string]interface{})
		user := data["user"].(map[string]interface{})
		for _, field := range []string{"id", "name", "email", "role"} {
			if _, ok := user[field]; !ok {
				t.Errorf("expected field %q in response", field)
			}
		}
	})
}

// TestPruneResponse_ArrayResponse tests that pruneResponse applies to array elements.
func TestPruneResponse_ArrayResponse(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "Alice", "email": "alice@example.com", "role": "ADMIN"},
					map[string]interface{}{"id": "2", "name": "Bob", "email": "bob@example.com", "role": "USER"},
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	resp := executor.Execute(context.Background(), &GraphQLRequest{
		Query: `query {
			users {
				id
				name
				email @skip(if: true)
			}
		}`,
	})
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	users, ok := data["users"].([]interface{})
	if !ok {
		t.Fatalf("users is not an array, got %T", data["users"])
	}

	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}

	for i, item := range users {
		u := item.(map[string]interface{})
		if _, ok := u["email"]; ok {
			t.Errorf("users[%d]: email should be pruned by @skip(if: true)", i)
		}
		if _, ok := u["id"]; !ok {
			t.Errorf("users[%d]: missing 'id'", i)
		}
		if _, ok := u["name"]; !ok {
			t.Errorf("users[%d]: missing 'name'", i)
		}
	}
}

// TestPruneResponse_NilAndEmpty tests edge cases: nil resolver (null response)
// and no selection set.
func TestPruneResponse_NilAndEmpty(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	// No resolver configured → response is nil, pruneResponse is a no-op
	executor := NewExecutor(schema, nil)

	resp := executor.Execute(context.Background(), &GraphQLRequest{
		Query: `query {
			user(id: "1") {
				id
			}
		}`,
	})
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	if data["user"] != nil {
		t.Errorf("user = %v, want nil (no resolver)", data["user"])
	}
}

// TestPruneResponse_TopLevelSkip tests that @skip on a top-level field
// completely excludes it from the result (handled by executeSelectionSetWithDoc,
// not pruneResponse, but validates the full directive path).
func TestPruneResponse_TopLevelSkip(t *testing.T) {
	schema, err := ParseSchema(executorTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Alice",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)

	resp := executor.Execute(context.Background(), &GraphQLRequest{
		Query: `query {
			user(id: "1") @skip(if: true) {
				id
				name
			}
		}`,
	})
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	if _, ok := data["user"]; ok {
		t.Error("user field should be skipped entirely by @skip(if: true) on the field")
	}
}
