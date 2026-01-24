package graphql

import (
	"fmt"
	"os"
	"sort"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Schema represents a parsed GraphQL schema with convenient accessors
// for types, queries, mutations, and subscriptions.
type Schema struct {
	ast           *ast.Schema
	source        string
	types         map[string]*ast.Definition
	queries       map[string]*ast.FieldDefinition
	mutations     map[string]*ast.FieldDefinition
	subscriptions map[string]*ast.FieldDefinition
}

// ParseSchema parses a GraphQL SDL string and returns a Schema.
func ParseSchema(sdl string) (*Schema, error) {
	source := &ast.Source{
		Name:  "schema",
		Input: sdl,
	}

	schema, err := gqlparser.LoadSchema(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL schema: %w", err)
	}

	return newSchema(schema, sdl), nil
}

// ParseSchemaFile parses a GraphQL schema from a file and returns a Schema.
func ParseSchemaFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", path, err)
	}

	source := &ast.Source{
		Name:  path,
		Input: string(data),
	}

	schema, err := gqlparser.LoadSchema(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL schema from %s: %w", path, err)
	}

	return newSchema(schema, string(data)), nil
}

// newSchema creates a new Schema from a parsed ast.Schema.
func newSchema(schema *ast.Schema, source string) *Schema {
	s := &Schema{
		ast:           schema,
		source:        source,
		types:         make(map[string]*ast.Definition),
		queries:       make(map[string]*ast.FieldDefinition),
		mutations:     make(map[string]*ast.FieldDefinition),
		subscriptions: make(map[string]*ast.FieldDefinition),
	}

	// Index all types
	for name, def := range schema.Types {
		s.types[name] = def
	}

	// Index query fields (excluding introspection fields)
	if schema.Query != nil {
		for _, field := range schema.Query.Fields {
			if !isIntrospectionField(field.Name) {
				s.queries[field.Name] = field
			}
		}
	}

	// Index mutation fields
	if schema.Mutation != nil {
		for _, field := range schema.Mutation.Fields {
			s.mutations[field.Name] = field
		}
	}

	// Index subscription fields
	if schema.Subscription != nil {
		for _, field := range schema.Subscription.Fields {
			s.subscriptions[field.Name] = field
		}
	}

	return s
}

// isIntrospectionField returns true if the field name is a built-in introspection field.
func isIntrospectionField(name string) bool {
	return len(name) >= 2 && name[0] == '_' && name[1] == '_'
}

// AST returns the underlying gqlparser AST schema.
func (s *Schema) AST() *ast.Schema {
	return s.ast
}

// Source returns the original SDL source string.
func (s *Schema) Source() string {
	return s.source
}

// GetType returns a type definition by name, or nil if not found.
func (s *Schema) GetType(name string) *ast.Definition {
	return s.types[name]
}

// GetQueryField returns a query field definition by name, or nil if not found.
func (s *Schema) GetQueryField(name string) *ast.FieldDefinition {
	return s.queries[name]
}

// GetMutationField returns a mutation field definition by name, or nil if not found.
func (s *Schema) GetMutationField(name string) *ast.FieldDefinition {
	return s.mutations[name]
}

// GetSubscriptionField returns a subscription field definition by name, or nil if not found.
func (s *Schema) GetSubscriptionField(name string) *ast.FieldDefinition {
	return s.subscriptions[name]
}

// ListQueries returns all query field names in sorted order.
func (s *Schema) ListQueries() []string {
	names := make([]string, 0, len(s.queries))
	for name := range s.queries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListMutations returns all mutation field names in sorted order.
func (s *Schema) ListMutations() []string {
	names := make([]string, 0, len(s.mutations))
	for name := range s.mutations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListSubscriptions returns all subscription field names in sorted order.
func (s *Schema) ListSubscriptions() []string {
	names := make([]string, 0, len(s.subscriptions))
	for name := range s.subscriptions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListTypes returns all type names in sorted order, optionally filtering by kind.
// If kinds is empty, all types are returned.
func (s *Schema) ListTypes(kinds ...ast.DefinitionKind) []string {
	kindSet := make(map[ast.DefinitionKind]bool)
	for _, k := range kinds {
		kindSet[k] = true
	}

	names := make([]string, 0)
	for name, def := range s.types {
		if len(kindSet) == 0 || kindSet[def.Kind] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// HasQuery returns true if the schema has a query type with fields.
func (s *Schema) HasQuery() bool {
	return s.ast.Query != nil && len(s.ast.Query.Fields) > 0
}

// HasMutation returns true if the schema has a mutation type with fields.
func (s *Schema) HasMutation() bool {
	return s.ast.Mutation != nil && len(s.ast.Mutation.Fields) > 0
}

// HasSubscription returns true if the schema has a subscription type with fields.
func (s *Schema) HasSubscription() bool {
	return s.ast.Subscription != nil && len(s.ast.Subscription.Fields) > 0
}

// Validate validates the schema and returns any errors.
// Since gqlparser validates during parsing, this performs additional semantic checks.
func (s *Schema) Validate() error {
	// gqlparser already validates the schema during parsing
	// This method can be extended for additional custom validation rules

	// Check that we have at least a Query type (required by GraphQL spec)
	if !s.HasQuery() {
		return fmt.Errorf("schema must define a Query type with at least one field")
	}

	return nil
}

// GetField returns a field definition by type and field name.
func (s *Schema) GetField(typeName, fieldName string) *ast.FieldDefinition {
	def := s.GetType(typeName)
	if def == nil {
		return nil
	}

	for _, field := range def.Fields {
		if field.Name == fieldName {
			return field
		}
	}
	return nil
}

// IsScalarType returns true if the given type name is a scalar type.
func (s *Schema) IsScalarType(name string) bool {
	// Built-in scalar types
	switch name {
	case "Int", "Float", "String", "Boolean", "ID":
		return true
	}

	// Custom scalar types
	def := s.GetType(name)
	return def != nil && def.Kind == ast.Scalar
}

// IsEnumType returns true if the given type name is an enum type.
func (s *Schema) IsEnumType(name string) bool {
	def := s.GetType(name)
	return def != nil && def.Kind == ast.Enum
}

// IsInputType returns true if the given type name is an input type.
func (s *Schema) IsInputType(name string) bool {
	def := s.GetType(name)
	return def != nil && def.Kind == ast.InputObject
}

// IsObjectType returns true if the given type name is an object type.
func (s *Schema) IsObjectType(name string) bool {
	def := s.GetType(name)
	return def != nil && def.Kind == ast.Object
}

// IsInterfaceType returns true if the given type name is an interface type.
func (s *Schema) IsInterfaceType(name string) bool {
	def := s.GetType(name)
	return def != nil && def.Kind == ast.Interface
}

// IsUnionType returns true if the given type name is a union type.
func (s *Schema) IsUnionType(name string) bool {
	def := s.GetType(name)
	return def != nil && def.Kind == ast.Union
}

// GetEnumValues returns the enum values for an enum type, or nil if not an enum.
func (s *Schema) GetEnumValues(name string) []string {
	def := s.GetType(name)
	if def == nil || def.Kind != ast.Enum {
		return nil
	}

	values := make([]string, 0, len(def.EnumValues))
	for _, v := range def.EnumValues {
		values = append(values, v.Name)
	}
	return values
}

// GetInterfaceImplementors returns all types that implement the given interface.
func (s *Schema) GetInterfaceImplementors(interfaceName string) []string {
	var implementors []string
	for name, def := range s.types {
		if def.Kind != ast.Object {
			continue
		}
		for _, iface := range def.Interfaces {
			if iface == interfaceName {
				implementors = append(implementors, name)
				break
			}
		}
	}
	sort.Strings(implementors)
	return implementors
}

// GetUnionMembers returns the member types of a union, or nil if not a union.
func (s *Schema) GetUnionMembers(name string) []string {
	def := s.GetType(name)
	if def == nil || def.Kind != ast.Union {
		return nil
	}

	members := make([]string, 0, len(def.Types))
	members = append(members, def.Types...)
	sort.Strings(members)
	return members
}
