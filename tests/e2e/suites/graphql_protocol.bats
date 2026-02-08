#!/usr/bin/env bats
# ============================================================================
# GraphQL Protocol — mock creation, queries, variables, introspection
# ============================================================================

setup_file() {
  load '../lib/helpers'
  api DELETE /mocks

  api POST /mocks -d '{
    "type": "graphql",
    "name": "Test GraphQL API",
    "graphql": {
      "path": "/graphql",
      "schema": "type Query { user(id: ID!): User\n  users: [User!]! }\ntype User { id: ID!\n  name: String!\n  email: String! }",
      "introspection": true,
      "resolvers": {
        "Query.user": {
          "response": {
            "id": "42",
            "name": "Test User",
            "email": "test@example.com"
          }
        },
        "Query.users": {
          "response": [
            {"id": "1", "name": "Alice", "email": "alice@example.com"},
            {"id": "2", "name": "Bob", "email": "bob@example.com"}
          ]
        }
      }
    }
  }'
}

teardown_file() {
  load '../lib/helpers'
  api DELETE /mocks
}

setup() {
  load '../lib/helpers'
}

@test "GQL-001: Create GraphQL mock returns 201" {
  api POST /mocks -d '{
    "type": "graphql",
    "name": "GQL Verify",
    "graphql": {
      "path": "/graphql-verify",
      "schema": "type Query { ping: String }",
      "resolvers": {"Query.ping": {"response": "pong"}}
    }
  }'
  [[ "$STATUS" == "201" ]]
  # Cleanup this extra mock
  local id=$(json_field '.id')
  api DELETE "/mocks/${id}"
}

@test "GQL-002: GraphQL query returns 200" {
  engine POST /graphql -d '{"query": "{ users { id name } }"}'
  [[ "$STATUS" == "200" ]]
}

@test "GQL-003: Response contains Alice" {
  engine POST /graphql -d '{"query": "{ users { id name } }"}'
  [[ "$BODY" == *"Alice"* ]]
}

@test "GQL-004: GraphQL query with variables returns 200" {
  engine POST /graphql -d '{"query": "query GetUser($id: ID!) { user(id: $id) { id name email } }", "variables": {"id": "42"}}'
  [[ "$STATUS" == "200" ]]
}

@test "GQL-005: User query returns data" {
  engine POST /graphql -d '{"query": "query GetUser($id: ID!) { user(id: $id) { id name email } }", "variables": {"id": "42"}}'
  [[ "$BODY" == *"Test User"* ]]
}

@test "GQL-006: Introspection query works" {
  engine POST /graphql -d '{"query": "{ __schema { queryType { name } } }"}'
  [[ "$STATUS" == "200" ]]
}

@test "GQL-007: Handlers list includes registered handler" {
  api GET /handlers
  [[ "$STATUS" == "200" ]]
}
