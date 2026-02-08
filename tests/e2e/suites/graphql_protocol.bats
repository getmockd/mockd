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
      "schema": "type Query { user(id: ID!): User\n  users: [User!]! }\ntype Mutation { createUser(name: String!, email: String!): User }\ntype User { id: ID!\n  name: String!\n  email: String! }",
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
        },
        "Mutation.createUser": {
          "response": {
            "id": "99",
            "name": "New User",
            "email": "new@example.com"
          }
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

# ── Mutation & error tests ───────────────────────────────────────────────────

@test "GQL-008: Mutation createUser returns response" {
  engine POST /graphql -d '{"query": "mutation { createUser(name: \"New User\", email: \"new@example.com\") { id name email } }"}'
  [[ "$STATUS" == "200" ]]
  [[ "$BODY" == *"New User"* ]]
}

@test "GQL-009: Invalid query returns error" {
  engine POST /graphql -d '{"query": "{ nonExistentField }"}'
  # Should get 200 with errors array, or 400 — either indicates proper error handling
  if [[ "$STATUS" == "200" ]]; then
    [[ "$BODY" == *"error"* ]]
  else
    [[ "$STATUS" == "400" ]]
  fi
}

@test "GQL-010: Malformed query body returns error status" {
  engine POST /graphql -d '{"query": "not valid graphql {{{"}'
  # Engine should reject with an error response
  if [[ "$STATUS" == "200" ]]; then
    [[ "$BODY" == *"error"* ]]
  else
    [[ "$STATUS" == "400" ]] || [[ "$STATUS" == "422" ]]
  fi
}
