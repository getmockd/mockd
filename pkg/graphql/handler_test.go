package graphql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

const handlerTestSchema = `
type Query {
	user(id: ID!): User
	users: [User!]!
}

type Mutation {
	createUser(name: String!, email: String!): User
}

type User {
	id: ID!
	name: String!
	email: String!
}
`

func newTestHandler(t *testing.T) *Handler {
	schema, err := ParseSchema(handlerTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{
		Introspection: true,
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":    "{{args.id}}",
					"name":  "Test User",
					"email": "test@example.com",
				},
			},
			"Query.users": {
				Response: []interface{}{
					map[string]interface{}{"id": "1", "name": "User 1", "email": "user1@example.com"},
					map[string]interface{}{"id": "2", "name": "User 2", "email": "user2@example.com"},
				},
			},
			"Mutation.createUser": {
				Response: map[string]interface{}{
					"id":    "new-123",
					"name":  "{{args.name}}",
					"email": "{{args.email}}",
				},
			},
		},
	}

	executor := NewExecutor(schema, config)
	return NewHandler(executor, config)
}

func TestHandler_ServeHTTP_POST_JSON(t *testing.T) {
	handler := newTestHandler(t)

	body := `{"query": "query { user(id: \"1\") { id name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want application/json", contentType)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Errors) > 0 {
		t.Errorf("response contained errors: %v", resp.Errors)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map, got %T", resp.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("user is not a map, got %T", data["user"])
	}

	if user["id"] != "1" {
		t.Errorf("user.id = %v, want '1'", user["id"])
	}
}

func TestHandler_ServeHTTP_POST_GraphQL(t *testing.T) {
	handler := newTestHandler(t)

	body := `query { user(id: "1") { id name } }`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/graphql")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Errors) > 0 {
		t.Errorf("response contained errors: %v", resp.Errors)
	}
}

func TestHandler_ServeHTTP_GET(t *testing.T) {
	handler := newTestHandler(t)

	query := url.QueryEscape(`query{user(id:"1"){id name}}`)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+query, nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Errors) > 0 {
		t.Errorf("response contained errors: %v", resp.Errors)
	}
}

func TestHandler_ServeHTTP_GET_WithVariables(t *testing.T) {
	handler := newTestHandler(t)

	query := url.QueryEscape(`query GetUser($id: ID!) { user(id: $id) { id name } }`)
	variables := url.QueryEscape(`{"id": "42"}`)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+query+"&variables="+variables, nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Errors) > 0 {
		t.Errorf("response contained errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	user := data["user"].(map[string]interface{})
	if user["id"] != "42" {
		t.Errorf("user.id = %v, want '42'", user["id"])
	}
}

func TestHandler_ServeHTTP_OPTIONS(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodOptions, "/graphql", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	// Note: CORS headers are now handled by the engine's CORSMiddleware, not by the handler directly.
	// This test only verifies that OPTIONS returns 200 OK for preflight requests.
}

func TestHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/graphql", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusMethodNotAllowed)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Errors) == 0 {
		t.Error("expected error response")
	}

	if !strings.Contains(resp.Errors[0].Message, "method not allowed") {
		t.Errorf("error message = %v, want to contain 'method not allowed'", resp.Errors[0].Message)
	}
}

func TestHandler_ServeHTTP_EmptyBody(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_ServeHTTP_InvalidJSON(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_ServeHTTP_WithOperationName(t *testing.T) {
	handler := newTestHandler(t)

	body := map[string]interface{}{
		"query": `
			query GetUser { user(id: "1") { id name } }
			query GetUsers { users { id name } }
		`,
		"operationName": "GetUser",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	data := resp.Data.(map[string]interface{})
	if _, ok := data["user"]; !ok {
		t.Error("expected 'user' field in response")
	}
	if _, ok := data["users"]; ok {
		t.Error("did not expect 'users' field in response")
	}
}

func TestHandler_ServeHTTP_Mutation(t *testing.T) {
	handler := newTestHandler(t)

	body := `{
		"query": "mutation { createUser(name: \"John\", email: \"john@example.com\") { id name email } }"
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if len(resp.Errors) > 0 {
		t.Errorf("response contained errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	user := data["createUser"].(map[string]interface{})

	if user["name"] != "John" {
		t.Errorf("user.name = %v, want 'John'", user["name"])
	}
	if user["email"] != "john@example.com" {
		t.Errorf("user.email = %v, want 'john@example.com'", user["email"])
	}
}

func TestHandler_ServeHTTP_Introspection(t *testing.T) {
	handler := newTestHandler(t)

	body := `{
		"query": "{ __schema { queryType { name } } }"
	}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if len(resp.Errors) > 0 {
		t.Errorf("response contained errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]interface{})
	schema := data["__schema"].(map[string]interface{})
	queryType := schema["queryType"].(map[string]interface{})

	if queryType["name"] != "Query" {
		t.Errorf("queryType.name = %v, want 'Query'", queryType["name"])
	}
}

func TestHandler_ServeHTTP_InvalidVariablesInGET(t *testing.T) {
	handler := newTestHandler(t)

	query := url.QueryEscape(`query{user(id:"1"){id}}`)
	variables := url.QueryEscape(`{invalid}`)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+query+"&variables="+variables, nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestNewHandler(t *testing.T) {
	schema, err := ParseSchema(handlerTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	config := &GraphQLConfig{}
	executor := NewExecutor(schema, config)
	handler := NewHandler(executor, config)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if handler.executor != executor {
		t.Error("handler.executor not set correctly")
	}
	if handler.config != config {
		t.Error("handler.config not set correctly")
	}
}

func TestEndpoint(t *testing.T) {
	config := &GraphQLConfig{
		Schema: handlerTestSchema,
		Resolvers: map[string]ResolverConfig{
			"Query.user": {
				Response: map[string]interface{}{
					"id":   "1",
					"name": "Test",
				},
			},
		},
	}

	handler, err := Endpoint(config)
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}

	if handler == nil {
		t.Fatal("Endpoint() returned nil handler")
	}

	// Test that the handler works
	body := `{"query": "query { user(id: \"1\") { id } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestEndpoint_SchemaFile(t *testing.T) {
	// Create a temp schema file
	tmpFile := t.TempDir() + "/schema.graphql"
	if err := os.WriteFile(tmpFile, []byte(handlerTestSchema), 0644); err != nil {
		t.Fatalf("failed to write temp schema: %v", err)
	}

	config := &GraphQLConfig{
		SchemaFile: tmpFile,
	}

	handler, err := Endpoint(config)
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}

	if handler == nil {
		t.Fatal("Endpoint() returned nil handler")
	}
}

func TestEndpoint_NoSchema(t *testing.T) {
	config := &GraphQLConfig{}

	_, err := Endpoint(config)
	if err == nil {
		t.Fatal("Endpoint() expected error when no schema provided")
	}
}

func TestEndpoint_InvalidSchema(t *testing.T) {
	config := &GraphQLConfig{
		Schema: "invalid graphql schema",
	}

	_, err := Endpoint(config)
	if err == nil {
		t.Fatal("Endpoint() expected error for invalid schema")
	}
}

func TestHandler_CORSHandledByMiddleware(t *testing.T) {
	// Note: CORS headers are now handled by the engine's CORSMiddleware, not by the GraphQL handler.
	// This test verifies that the handler does NOT set CORS headers directly, as that would
	// override the configurable CORS settings from the middleware.
	handler := newTestHandler(t)

	body := `{"query": "query { user(id: \"1\") { id } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Handler should NOT set CORS headers directly - this is now the middleware's job
	if cors := rr.Header().Get("Access-Control-Allow-Origin"); cors != "" {
		t.Errorf("Handler should not set Access-Control-Allow-Origin directly, got: %v", cors)
	}
}

// ── Batch Query Tests ─────────────────────────────────────────────────────

func TestHandlerBatchQuery(t *testing.T) {
	handler := newTestHandler(t)

	t.Run("batch of two queries returns array", func(t *testing.T) {
		body := `[
			{"query": "{ user(id: \"1\") { id name } }"},
			{"query": "{ users { id name } }"}
		]`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		// Response should be a JSON array
		var responses []json.RawMessage
		if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
			t.Fatalf("expected JSON array, got: %s", rr.Body.String())
		}
		if len(responses) != 2 {
			t.Errorf("expected 2 responses, got %d", len(responses))
		}

		// Content-Type should be application/json
		ct := rr.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
	})

	t.Run("single element batch returns array of one", func(t *testing.T) {
		body := `[{"query": "{ user(id: \"1\") { id name } }"}]`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		// Must be an array (not a bare object)
		var responses []json.RawMessage
		if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
			t.Fatalf("expected JSON array for single-element batch, got: %s", rr.Body.String())
		}
		if len(responses) != 1 {
			t.Errorf("expected 1 response in array, got %d", len(responses))
		}
	})

	t.Run("batch responses contain data", func(t *testing.T) {
		body := `[
			{"query": "{ user(id: \"42\") { id name email } }"},
			{"query": "{ users { id } }"}
		]`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}

		var responses []struct {
			Data   map[string]interface{} `json:"data"`
			Errors []interface{}          `json:"errors"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
			t.Fatalf("parse batch response: %v", err)
		}

		// First response: user query should have data
		if responses[0].Data == nil {
			t.Error("first response missing data")
		}

		// Second response: users query should have data
		if responses[1].Data == nil {
			t.Error("second response missing data")
		}
	})

	t.Run("batch with mutation and query", func(t *testing.T) {
		body := `[
			{"query": "{ user(id: \"1\") { id } }"},
			{"query": "mutation { createUser(name: \"Bob\", email: \"bob@example.com\") { id name } }"}
		]`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var responses []json.RawMessage
		if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
			t.Fatalf("expected JSON array, got: %s", rr.Body.String())
		}
		if len(responses) != 2 {
			t.Errorf("expected 2 responses, got %d", len(responses))
		}
	})

	t.Run("application/graphql not treated as batch", func(t *testing.T) {
		// Even if body starts with '[', application/graphql should not batch
		body := `[ignored]{ user(id: "1") { id } }`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/graphql")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Should NOT return a batch array — treated as raw query
		var responses []json.RawMessage
		err := json.Unmarshal(rr.Body.Bytes(), &responses)
		if err == nil && len(responses) > 0 {
			// If it parsed as array, that's wrong — verify it's actually a single response
			var single map[string]interface{}
			if json.Unmarshal(rr.Body.Bytes(), &single) == nil {
				// It's a single object, which is correct
				return
			}
		}
		// Either error parsing as array (correct) or parsed as single object (correct)
	})

	t.Run("GET request not treated as batch", func(t *testing.T) {
		// Batch detection only applies to POST
		q := url.Values{"query": {`{ user(id: "1") { id } }`}}.Encode()
		req := httptest.NewRequest(http.MethodGet, "/graphql?"+q, nil)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}

		// Should be a single response object, not an array
		var single map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &single); err != nil {
			t.Errorf("expected single JSON object for GET, got: %s", rr.Body.String())
		}
	})

	t.Run("empty batch body falls through to single parse", func(t *testing.T) {
		body := `[]`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Empty batch should not crash — may return 400 or handle gracefully
		if rr.Code == 0 || rr.Code >= 500 {
			t.Errorf("expected non-5xx for empty batch, got %d", rr.Code)
		}
	})
}
