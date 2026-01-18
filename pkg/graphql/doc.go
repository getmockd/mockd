// Package graphql provides GraphQL schema parsing and mocking support for the mockd engine.
//
// This package enables mockd to handle GraphQL requests by parsing GraphQL SDL schemas
// and providing mock responses for queries, mutations, and subscriptions.
//
// Key features:
//   - Parse GraphQL SDL schemas from strings or files
//   - Access type definitions, queries, mutations, and subscriptions
//   - Configure resolvers with custom responses, delays, and error handling
//   - Support for introspection queries
//   - WebSocket subscriptions with graphql-ws and subscriptions-transport-ws protocols
//
// Basic usage:
//
//	// Parse a schema from SDL string
//	schema, err := graphql.ParseSchema(`
//	    type Query {
//	        user(id: ID!): User
//	    }
//	    type User {
//	        id: ID!
//	        name: String!
//	    }
//	`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access schema information
//	queries := schema.ListQueries()
//	userType := schema.GetType("User")
//
// Configuration example:
//
//	config := &graphql.GraphQLConfig{
//	    ID:            "my-graphql-api",
//	    Path:          "/graphql",
//	    SchemaFile:    "schema.graphql",
//	    Introspection: true,
//	    Resolvers: map[string]graphql.ResolverConfig{
//	        "Query.user": {
//	            Response: map[string]interface{}{
//	                "id":   "1",
//	                "name": "John Doe",
//	            },
//	        },
//	    },
//	    Enabled: true,
//	}
//
// Subscription configuration example:
//
//	config := &graphql.GraphQLConfig{
//	    Schema: `
//	        type Query { _: String }
//	        type Subscription {
//	            messageAdded(channel: String!): Message
//	        }
//	        type Message {
//	            id: ID!
//	            text: String!
//	        }
//	    `,
//	    Subscriptions: map[string]graphql.SubscriptionConfig{
//	        "messageAdded": {
//	            Events: []graphql.EventConfig{
//	                {Data: map[string]interface{}{"id": "1", "text": "Hello"}},
//	                {Data: map[string]interface{}{"id": "2", "text": "World"}, Delay: "1s"},
//	            },
//	            Timing: &graphql.TimingConfig{
//	                FixedDelay:  "500ms",
//	                RandomDelay: "100ms-1s",
//	                Repeat:      true,
//	            },
//	        },
//	    },
//	}
//
// The subscription handler supports both the modern graphql-transport-ws protocol
// and the legacy subscriptions-transport-ws (graphql-ws) protocol. Variable substitution
// is supported using {{args.variableName}} or {{vars.variableName}} templates.
package graphql
