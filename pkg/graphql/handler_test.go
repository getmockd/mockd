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

	// Check CORS headers
	if cors := rr.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("Access-Control-Allow-Origin = %v, want '*'", cors)
	}
	if methods := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(methods, "POST") {
		t.Errorf("Access-Control-Allow-Methods = %v, want to contain 'POST'", methods)
	}
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

func TestHandler_CORSHeaders(t *testing.T) {
	handler := newTestHandler(t)

	body := `{"query": "query { user(id: \"1\") { id } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check CORS headers are set on all responses
	if cors := rr.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("Access-Control-Allow-Origin = %v, want '*'", cors)
	}
}
