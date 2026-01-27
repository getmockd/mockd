package graphql

// GraphQLConfig represents a GraphQL endpoint configuration.
type GraphQLConfig struct {
	// ID is the unique identifier for this GraphQL endpoint.
	ID string `json:"id" yaml:"id"`
	// Name is a human-readable name for this endpoint.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// ParentID is the folder ID this endpoint belongs to ("" = root level)
	ParentID string `json:"parentId,omitempty" yaml:"parentId,omitempty"`
	// MetaSortKey is used for manual ordering within a folder
	MetaSortKey float64 `json:"metaSortKey,omitempty" yaml:"metaSortKey,omitempty"`
	// Path is the URL path where this GraphQL endpoint is served.
	Path string `json:"path" yaml:"path"`
	// Schema is the inline GraphQL SDL schema definition.
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty"`
	// SchemaFile is the path to a file containing the GraphQL SDL schema.
	SchemaFile string `json:"schemaFile,omitempty" yaml:"schemaFile,omitempty"`
	// Introspection enables the GraphQL introspection query.
	Introspection bool `json:"introspection" yaml:"introspection"`
	// Resolvers maps field paths (e.g., "Query.user") to their resolver configurations.
	Resolvers map[string]ResolverConfig `json:"resolvers,omitempty" yaml:"resolvers,omitempty"`
	// Subscriptions maps subscription field names to their configurations.
	Subscriptions map[string]SubscriptionConfig `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	// Enabled indicates whether this GraphQL endpoint is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// SkipOriginVerify skips verification of the Origin header during WebSocket handshake
	// for GraphQL subscriptions. Default: true (allows any origin for development/testing).
	// Set to false to enforce that Origin matches the Host header.
	SkipOriginVerify *bool `json:"skipOriginVerify,omitempty" yaml:"skipOriginVerify,omitempty"`
}

// ResolverConfig configures how a GraphQL field is resolved.
type ResolverConfig struct {
	// Response is the mock response data to return for this field.
	Response interface{} `json:"response,omitempty" yaml:"response,omitempty"`
	// Delay is the simulated latency before returning the response (e.g., "100ms", "2s").
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`
	// Match specifies conditions that must be met for this resolver to be used.
	Match *ResolverMatch `json:"match,omitempty" yaml:"match,omitempty"`
	// Error configures an error response instead of data.
	Error *GraphQLErrorConfig `json:"error,omitempty" yaml:"error,omitempty"`
}

// ResolverMatch specifies matching conditions for a resolver.
type ResolverMatch struct {
	// Args specifies argument values that must match for this resolver to apply.
	Args map[string]interface{} `json:"args,omitempty" yaml:"args,omitempty"`
}

// GraphQLErrorConfig configures a GraphQL error response.
type GraphQLErrorConfig struct {
	// Message is the error message.
	Message string `json:"message"`
	// Path is the field path where the error occurred.
	Path []string `json:"path,omitempty"`
	// Extensions contains additional error metadata.
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLError represents a GraphQL error in the response format.
type GraphQLError struct {
	// Message is the error message.
	Message string `json:"message"`
	// Locations indicates where in the query the error occurred.
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	// Path is the response field path where the error occurred.
	Path []interface{} `json:"path,omitempty"`
	// Extensions contains additional error metadata.
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLErrorLocation represents a location in the GraphQL query where an error occurred.
type GraphQLErrorLocation struct {
	// Line is the line number (1-indexed).
	Line int `json:"line"`
	// Column is the column number (1-indexed).
	Column int `json:"column"`
}

// GraphQLRequest represents an incoming GraphQL request.
type GraphQLRequest struct {
	// Query is the GraphQL query string.
	Query string `json:"query"`
	// OperationName is the name of the operation to execute (for multi-operation documents).
	OperationName string `json:"operationName,omitempty"`
	// Variables are the variable values for the query.
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response.
type GraphQLResponse struct {
	// Data contains the result of the query execution.
	Data interface{} `json:"data,omitempty"`
	// Errors contains any errors that occurred during execution.
	Errors []GraphQLError `json:"errors,omitempty"`
	// Extensions contains additional response metadata.
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// FieldPath represents a path to a field in the schema (e.g., "Query.user" or "Mutation.createUser").
type FieldPath struct {
	// TypeName is the parent type name (e.g., "Query", "Mutation", "User").
	TypeName string
	// FieldName is the field name.
	FieldName string
}

// String returns the string representation of the field path.
func (fp FieldPath) String() string {
	return fp.TypeName + "." + fp.FieldName
}

// ParseFieldPath parses a field path string (e.g., "Query.user") into a FieldPath.
func ParseFieldPath(path string) FieldPath {
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			return FieldPath{
				TypeName:  path[:i],
				FieldName: path[i+1:],
			}
		}
	}
	// No dot found, treat the whole string as a field name
	return FieldPath{FieldName: path}
}
