package graphql

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/template"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/validator"
)

// Executor executes GraphQL operations against configured resolvers.
type Executor struct {
	schema         *Schema
	config         *GraphQLConfig
	resolvers      map[string][]ResolverConfig // "Query.user" -> resolvers (multiple for conditional matching)
	templateEngine *template.Engine
}

// NewExecutor creates a new GraphQL executor with the given schema and configuration.
func NewExecutor(schema *Schema, config *GraphQLConfig) *Executor {
	e := &Executor{
		schema:         schema,
		config:         config,
		resolvers:      make(map[string][]ResolverConfig),
		templateEngine: template.New(),
	}

	// Index resolvers by path for efficient lookup
	if config != nil && config.Resolvers != nil {
		for path, resolver := range config.Resolvers {
			e.resolvers[path] = append(e.resolvers[path], resolver)
		}
	}

	return e
}

// Execute executes a GraphQL request and returns a response.
func (e *Executor) Execute(ctx context.Context, req *GraphQLRequest) *GraphQLResponse {
	if req == nil || req.Query == "" {
		return &GraphQLResponse{
			Errors: []GraphQLError{{Message: "query is required"}},
		}
	}

	// Parse the query
	doc, err := e.parseQuery(req.Query)
	if err != nil {
		return &GraphQLResponse{
			Errors: []GraphQLError{{Message: err.Error()}},
		}
	}

	// Find the operation to execute
	var op *ast.OperationDefinition
	for _, opDef := range doc.Operations {
		if req.OperationName == "" || opDef.Name == req.OperationName {
			op = opDef
			break
		}
	}

	if op == nil {
		if req.OperationName != "" {
			return &GraphQLResponse{
				Errors: []GraphQLError{{Message: fmt.Sprintf("operation %q not found", req.OperationName)}},
			}
		}
		return &GraphQLResponse{
			Errors: []GraphQLError{{Message: "no operation found in query"}},
		}
	}

	// Execute the operation
	data, errors := e.executeOperation(ctx, doc, op, req.Variables)

	resp := &GraphQLResponse{
		Data: data,
	}
	if len(errors) > 0 {
		resp.Errors = make([]GraphQLError, len(errors))
		for i, err := range errors {
			resp.Errors[i] = *err
		}
	}

	return resp
}

// parseQuery parses and validates a GraphQL query against the schema.
func (e *Executor) parseQuery(query string) (*ast.QueryDocument, error) {
	doc, parseErr := gqlparser.LoadQuery(e.schema.AST(), query)
	if parseErr != nil {
		// Return first error for simplicity
		if len(parseErr) > 0 {
			return nil, fmt.Errorf("parse error: %s", parseErr[0].Message)
		}
		return nil, fmt.Errorf("parse error")
	}

	// Validate the query
	validationErrs := validator.Validate(e.schema.AST(), doc)
	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("validation error: %s", validationErrs[0].Message)
	}

	return doc, nil
}

// executeOperation executes a single GraphQL operation.
func (e *Executor) executeOperation(ctx context.Context, doc *ast.QueryDocument, op *ast.OperationDefinition, variables map[string]interface{}) (interface{}, []*GraphQLError) {
	// Determine operation type
	var opType string
	switch op.Operation {
	case ast.Query:
		opType = "Query"
	case ast.Mutation:
		opType = "Mutation"
	case ast.Subscription:
		opType = "Subscription"
	default:
		return nil, []*GraphQLError{{Message: "unsupported operation type"}}
	}

	// Check for introspection queries
	if e.isIntrospectionQuery(op.SelectionSet) {
		if e.config != nil && !e.config.Introspection {
			return nil, []*GraphQLError{{Message: "introspection is disabled"}}
		}
		// Return introspection data from schema
		return e.executeIntrospection(ctx, doc, op.SelectionSet, variables)
	}

	// Execute the selection set
	return e.executeSelectionSet(ctx, opType, op.SelectionSet, variables)
}

// isIntrospectionQuery checks if the selection set contains only introspection fields.
func (e *Executor) isIntrospectionQuery(selections ast.SelectionSet) bool {
	for _, sel := range selections {
		if field, ok := sel.(*ast.Field); ok {
			if !strings.HasPrefix(field.Name, "__") {
				return false
			}
		}
	}
	return len(selections) > 0
}

// executeIntrospection handles introspection queries.
func (e *Executor) executeIntrospection(_ context.Context, doc *ast.QueryDocument, selections ast.SelectionSet, variables map[string]interface{}) (interface{}, []*GraphQLError) {
	result := make(map[string]interface{})

	for _, sel := range selections {
		field, ok := sel.(*ast.Field)
		if !ok {
			continue
		}

		alias := field.Alias
		if alias == "" {
			alias = field.Name
		}

		switch field.Name {
		case "__schema":
			result[alias] = e.buildSchemaIntrospection(doc, field.SelectionSet)
		case "__type":
			typeName := e.getArgumentValue(field, "name", variables)
			if name, ok := typeName.(string); ok {
				result[alias] = e.buildTypeIntrospection(doc, name, field.SelectionSet)
			} else {
				result[alias] = nil
			}
		case "__typename":
			result[alias] = "Query"
		}
	}

	return result, nil
}

// buildSchemaIntrospection builds the __schema introspection response.
func (e *Executor) buildSchemaIntrospection(doc *ast.QueryDocument, selections ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	for _, sel := range selections {
		field, ok := sel.(*ast.Field)
		if !ok {
			continue
		}

		alias := field.Alias
		if alias == "" {
			alias = field.Name
		}

		switch field.Name {
		case "queryType":
			if e.schema.HasQuery() {
				result[alias] = map[string]interface{}{"name": "Query"}
			} else {
				result[alias] = nil
			}
		case "mutationType":
			if e.schema.HasMutation() {
				result[alias] = map[string]interface{}{"name": "Mutation"}
			} else {
				result[alias] = nil
			}
		case "subscriptionType":
			if e.schema.HasSubscription() {
				result[alias] = map[string]interface{}{"name": "Subscription"}
			} else {
				result[alias] = nil
			}
		case "types":
			result[alias] = e.buildTypesIntrospection(doc, field.SelectionSet)
		case "directives":
			result[alias] = e.buildDirectivesIntrospection(doc, field.SelectionSet)
		}
	}

	return result
}

// buildTypesIntrospection builds the types array for introspection.
func (e *Executor) buildTypesIntrospection(doc *ast.QueryDocument, selections ast.SelectionSet) []interface{} {
	types := make([]interface{}, 0)
	for _, name := range e.schema.ListTypes() {
		if typeInfo := e.buildTypeIntrospection(doc, name, selections); typeInfo != nil {
			types = append(types, typeInfo)
		}
	}
	return types
}

// buildTypeIntrospection builds type introspection data.
func (e *Executor) buildTypeIntrospection(doc *ast.QueryDocument, typeName string, selections ast.SelectionSet) map[string]interface{} {
	def := e.schema.GetType(typeName)
	if def == nil {
		return nil
	}

	result := make(map[string]interface{})

	// Expand the selections by resolving fragment spreads
	expandedSelections := e.expandSelections(doc, selections)

	for _, sel := range expandedSelections {
		field, ok := sel.(*ast.Field)
		if !ok {
			continue
		}

		alias := field.Alias
		if alias == "" {
			alias = field.Name
		}

		switch field.Name {
		case "name":
			result[alias] = typeName
		case "kind":
			result[alias] = e.getTypeKind(def)
		case "description":
			result[alias] = def.Description
		case "specifiedByURL":
			result[alias] = nil
		case "fields":
			if def.Kind == ast.Object || def.Kind == ast.Interface {
				result[alias] = e.buildFieldsIntrospection(doc, def.Fields, field.SelectionSet)
			} else {
				result[alias] = nil
			}
		case "inputFields":
			if def.Kind == ast.InputObject {
				result[alias] = e.buildInputFieldsIntrospection(doc, def.Fields, field.SelectionSet)
			} else {
				result[alias] = nil
			}
		case "enumValues":
			if def.Kind == ast.Enum {
				result[alias] = e.buildEnumValuesIntrospection(doc, def.EnumValues, field.SelectionSet)
			} else {
				result[alias] = nil
			}
		case "interfaces":
			if def.Kind == ast.Object {
				result[alias] = e.buildInterfacesIntrospection(doc, def.Interfaces, field.SelectionSet)
			} else {
				result[alias] = nil
			}
		case "possibleTypes":
			if def.Kind == ast.Interface || def.Kind == ast.Union {
				result[alias] = e.buildPossibleTypesIntrospection(doc, def, field.SelectionSet)
			} else {
				result[alias] = nil
			}
		case "ofType":
			result[alias] = nil
		case "isOneOf":
			result[alias] = false
		}
	}

	return result
}

// expandSelections expands fragment spreads in a selection set to their field definitions.
func (e *Executor) expandSelections(doc *ast.QueryDocument, selections ast.SelectionSet) ast.SelectionSet {
	if doc == nil {
		return selections
	}

	var expanded ast.SelectionSet
	for _, sel := range selections {
		switch s := sel.(type) {
		case *ast.Field:
			expanded = append(expanded, s)
		case *ast.FragmentSpread:
			// Find the fragment definition and expand it
			for _, frag := range doc.Fragments {
				if frag.Name == s.Name {
					// Recursively expand the fragment's selections
					expanded = append(expanded, e.expandSelections(doc, frag.SelectionSet)...)
					break
				}
			}
		case *ast.InlineFragment:
			// Expand inline fragments
			expanded = append(expanded, e.expandSelections(doc, s.SelectionSet)...)
		}
	}
	return expanded
}

// buildDirectivesIntrospection builds directives introspection data.
func (e *Executor) buildDirectivesIntrospection(doc *ast.QueryDocument, selections ast.SelectionSet) []interface{} {
	result := make([]interface{}, 0, len(e.schema.AST().Directives))

	expandedSelections := e.expandSelections(doc, selections)

	// Get directives from the schema's AST
	for _, dir := range e.schema.AST().Directives {
		dirInfo := make(map[string]interface{})
		for _, sel := range expandedSelections {
			f, ok := sel.(*ast.Field)
			if !ok {
				continue
			}
			alias := f.Alias
			if alias == "" {
				alias = f.Name
			}
			switch f.Name {
			case "name":
				dirInfo[alias] = dir.Name
			case "description":
				dirInfo[alias] = dir.Description
			case "locations":
				locations := make([]interface{}, len(dir.Locations))
				for i, loc := range dir.Locations {
					locations[i] = string(loc)
				}
				dirInfo[alias] = locations
			case "args":
				dirInfo[alias] = e.buildArgsIntrospection(doc, dir.Arguments, f.SelectionSet)
			case "isRepeatable":
				dirInfo[alias] = dir.IsRepeatable
			}
		}
		result = append(result, dirInfo)
	}

	return result
}

// getTypeKind returns the GraphQL type kind string.
func (e *Executor) getTypeKind(def *ast.Definition) string {
	switch def.Kind {
	case ast.Scalar:
		return "SCALAR"
	case ast.Object:
		return "OBJECT"
	case ast.Interface:
		return "INTERFACE"
	case ast.Union:
		return "UNION"
	case ast.Enum:
		return "ENUM"
	case ast.InputObject:
		return "INPUT_OBJECT"
	default:
		return "OBJECT"
	}
}

// buildFieldsIntrospection builds field introspection data.
func (e *Executor) buildFieldsIntrospection(doc *ast.QueryDocument, fields ast.FieldList, selections ast.SelectionSet) []interface{} {
	result := make([]interface{}, 0)

	expandedSelections := e.expandSelections(doc, selections)

	for _, field := range fields {
		if strings.HasPrefix(field.Name, "__") {
			continue // Skip introspection fields
		}
		fieldInfo := make(map[string]interface{})
		for _, sel := range expandedSelections {
			f, ok := sel.(*ast.Field)
			if !ok {
				continue
			}
			alias := f.Alias
			if alias == "" {
				alias = f.Name
			}
			switch f.Name {
			case "name":
				fieldInfo[alias] = field.Name
			case "description":
				fieldInfo[alias] = field.Description
			case "args":
				fieldInfo[alias] = e.buildArgsIntrospection(doc, field.Arguments, f.SelectionSet)
			case "type":
				fieldInfo[alias] = e.buildTypeRefIntrospection(doc, field.Type, f.SelectionSet)
			case "isDeprecated":
				fieldInfo[alias] = false
			case "deprecationReason":
				fieldInfo[alias] = nil
			}
		}
		result = append(result, fieldInfo)
	}
	return result
}

// buildArgsIntrospection builds argument introspection data.
func (e *Executor) buildArgsIntrospection(doc *ast.QueryDocument, args ast.ArgumentDefinitionList, selections ast.SelectionSet) []interface{} {
	// Initialize as empty slice (not nil) so JSON marshals to [] instead of null
	result := make([]interface{}, 0, len(args))

	expandedSelections := e.expandSelections(doc, selections)

	for _, arg := range args {
		argInfo := make(map[string]interface{})
		for _, sel := range expandedSelections {
			f, ok := sel.(*ast.Field)
			if !ok {
				continue
			}
			alias := f.Alias
			if alias == "" {
				alias = f.Name
			}
			switch f.Name {
			case "name":
				argInfo[alias] = arg.Name
			case "description":
				argInfo[alias] = arg.Description
			case "type":
				argInfo[alias] = e.buildTypeRefIntrospection(doc, arg.Type, f.SelectionSet)
			case "defaultValue":
				if arg.DefaultValue != nil {
					argInfo[alias] = arg.DefaultValue.String()
				} else {
					argInfo[alias] = nil
				}
			case "isDeprecated":
				argInfo[alias] = false
			case "deprecationReason":
				argInfo[alias] = nil
			}
		}
		result = append(result, argInfo)
	}
	return result
}

// buildTypeRefIntrospection builds type reference introspection data.
func (e *Executor) buildTypeRefIntrospection(doc *ast.QueryDocument, t *ast.Type, selections ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	expandedSelections := e.expandSelections(doc, selections)

	for _, sel := range expandedSelections {
		f, ok := sel.(*ast.Field)
		if !ok {
			continue
		}
		alias := f.Alias
		if alias == "" {
			alias = f.Name
		}

		if t.NonNull {
			switch f.Name {
			case "kind":
				result[alias] = "NON_NULL"
			case "name":
				result[alias] = nil
			case "ofType":
				innerType := *t
				innerType.NonNull = false
				result[alias] = e.buildTypeRefIntrospection(doc, &innerType, f.SelectionSet)
			}
		} else if t.Elem != nil {
			switch f.Name {
			case "kind":
				result[alias] = "LIST"
			case "name":
				result[alias] = nil
			case "ofType":
				result[alias] = e.buildTypeRefIntrospection(doc, t.Elem, f.SelectionSet)
			}
		} else {
			switch f.Name {
			case "kind":
				def := e.schema.GetType(t.NamedType)
				if def != nil {
					result[alias] = e.getTypeKind(def)
				} else {
					result[alias] = "SCALAR"
				}
			case "name":
				result[alias] = t.NamedType
			case "ofType":
				result[alias] = nil
			}
		}
	}

	return result
}

// buildInputFieldsIntrospection builds input field introspection data.
func (e *Executor) buildInputFieldsIntrospection(doc *ast.QueryDocument, fields ast.FieldList, selections ast.SelectionSet) []interface{} {
	result := make([]interface{}, 0, len(fields))

	expandedSelections := e.expandSelections(doc, selections)

	for _, field := range fields {
		fieldInfo := make(map[string]interface{})
		for _, sel := range expandedSelections {
			f, ok := sel.(*ast.Field)
			if !ok {
				continue
			}
			alias := f.Alias
			if alias == "" {
				alias = f.Name
			}
			switch f.Name {
			case "name":
				fieldInfo[alias] = field.Name
			case "description":
				fieldInfo[alias] = field.Description
			case "type":
				fieldInfo[alias] = e.buildTypeRefIntrospection(doc, field.Type, f.SelectionSet)
			case "defaultValue":
				if field.DefaultValue != nil {
					fieldInfo[alias] = field.DefaultValue.String()
				} else {
					fieldInfo[alias] = nil
				}
			case "isDeprecated":
				fieldInfo[alias] = false
			case "deprecationReason":
				fieldInfo[alias] = nil
			}
		}
		result = append(result, fieldInfo)
	}
	return result
}

// buildEnumValuesIntrospection builds enum values introspection data.
func (e *Executor) buildEnumValuesIntrospection(doc *ast.QueryDocument, values ast.EnumValueList, selections ast.SelectionSet) []interface{} {
	result := make([]interface{}, 0, len(values))

	expandedSelections := e.expandSelections(doc, selections)

	for _, val := range values {
		valInfo := make(map[string]interface{})
		for _, sel := range expandedSelections {
			f, ok := sel.(*ast.Field)
			if !ok {
				continue
			}
			alias := f.Alias
			if alias == "" {
				alias = f.Name
			}
			switch f.Name {
			case "name":
				valInfo[alias] = val.Name
			case "description":
				valInfo[alias] = val.Description
			case "isDeprecated":
				valInfo[alias] = false
			case "deprecationReason":
				valInfo[alias] = nil
			}
		}
		result = append(result, valInfo)
	}
	return result
}

// buildInterfacesIntrospection builds interfaces introspection data.
func (e *Executor) buildInterfacesIntrospection(doc *ast.QueryDocument, interfaces []string, selections ast.SelectionSet) []interface{} {
	result := make([]interface{}, 0, len(interfaces))
	for _, iface := range interfaces {
		result = append(result, e.buildTypeIntrospection(doc, iface, selections))
	}
	return result
}

// buildPossibleTypesIntrospection builds possible types introspection data.
func (e *Executor) buildPossibleTypesIntrospection(doc *ast.QueryDocument, def *ast.Definition, selections ast.SelectionSet) []interface{} {
	var typeNames []string

	switch def.Kind {
	case ast.Union:
		typeNames = def.Types
	case ast.Interface:
		typeNames = e.schema.GetInterfaceImplementors(def.Name)
	}

	result := make([]interface{}, 0, len(typeNames))
	for _, name := range typeNames {
		result = append(result, e.buildTypeIntrospection(doc, name, selections))
	}
	return result
}

// executeSelectionSet executes a selection set against resolvers.
func (e *Executor) executeSelectionSet(ctx context.Context, opType string, selections ast.SelectionSet, variables map[string]interface{}) (map[string]interface{}, []*GraphQLError) {
	result := make(map[string]interface{})
	var errors []*GraphQLError

	for _, sel := range selections {
		switch s := sel.(type) {
		case *ast.Field:
			alias := s.Alias
			if alias == "" {
				alias = s.Name
			}

			// Handle __typename
			if s.Name == "__typename" {
				result[alias] = opType
				continue
			}

			// Build the resolver path
			path := opType + "." + s.Name

			// Get arguments
			args := e.extractArguments(s, variables)

			// Find matching resolver
			resolver := e.findResolver(path, args)

			// Resolve the field
			value, err := e.resolveField(ctx, s, resolver, variables)
			if err != nil {
				err.Path = []interface{}{alias}
				errors = append(errors, err)
				result[alias] = nil
			} else {
				result[alias] = value
			}

		case *ast.FragmentSpread:
			// Fragment spreads are handled during query validation
			// For mocking purposes, we don't need to handle them specially
			continue

		case *ast.InlineFragment:
			// Inline fragments are handled during query validation
			// For mocking purposes, we don't need to handle them specially
			continue
		}
	}

	return result, errors
}

// extractArguments extracts argument values from a field.
func (e *Executor) extractArguments(field *ast.Field, variables map[string]interface{}) map[string]interface{} {
	args := make(map[string]interface{})
	for _, arg := range field.Arguments {
		args[arg.Name] = e.resolveValue(arg.Value, variables)
	}
	return args
}

// resolveValue resolves an AST value to a Go value.
func (e *Executor) resolveValue(value *ast.Value, variables map[string]interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch value.Kind {
	case ast.Variable:
		if variables != nil {
			return variables[value.Raw]
		}
		return nil
	case ast.IntValue:
		// Parse as int
		var n int64
		_, _ = fmt.Sscanf(value.Raw, "%d", &n)
		return n
	case ast.FloatValue:
		var f float64
		_, _ = fmt.Sscanf(value.Raw, "%f", &f)
		return f
	case ast.StringValue, ast.BlockValue:
		return value.Raw
	case ast.BooleanValue:
		return value.Raw == "true"
	case ast.NullValue:
		return nil
	case ast.EnumValue:
		return value.Raw
	case ast.ListValue:
		var list []interface{}
		for _, child := range value.Children {
			list = append(list, e.resolveValue(child.Value, variables))
		}
		return list
	case ast.ObjectValue:
		obj := make(map[string]interface{})
		for _, child := range value.Children {
			obj[child.Name] = e.resolveValue(child.Value, variables)
		}
		return obj
	default:
		return value.Raw
	}
}

// getArgumentValue gets an argument value from a field.
func (e *Executor) getArgumentValue(field *ast.Field, name string, variables map[string]interface{}) interface{} {
	for _, arg := range field.Arguments {
		if arg.Name == name {
			return e.resolveValue(arg.Value, variables)
		}
	}
	return nil
}

// findResolver finds the best matching resolver for a field path and arguments.
func (e *Executor) findResolver(path string, args map[string]interface{}) *ResolverConfig {
	resolvers, ok := e.resolvers[path]
	if !ok || len(resolvers) == 0 {
		return nil
	}

	// Find the best matching resolver
	for i := range resolvers {
		resolver := &resolvers[i]
		if resolver.Match == nil || e.matchArgs(resolver.Match.Args, args) {
			return resolver
		}
	}

	// Return the first resolver without a match condition as fallback
	for i := range resolvers {
		if resolvers[i].Match == nil {
			return &resolvers[i]
		}
	}

	return nil
}

// matchArgs checks if the resolver match conditions are satisfied by the arguments.
func (e *Executor) matchArgs(matchArgs, actualArgs map[string]interface{}) bool {
	if matchArgs == nil {
		return true
	}

	for key, expected := range matchArgs {
		actual, ok := actualArgs[key]
		if !ok {
			return false
		}
		if !e.valuesEqual(expected, actual) {
			return false
		}
	}

	return true
}

// valuesEqual compares two values for equality.
func (e *Executor) valuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to string for comparison (handles type differences like int vs int64)
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// resolveField resolves a single field using the resolver configuration.
func (e *Executor) resolveField(ctx context.Context, field *ast.Field, resolver *ResolverConfig, variables map[string]interface{}) (interface{}, *GraphQLError) {
	if resolver == nil {
		// No resolver configured - return null
		return nil, nil
	}

	// Apply delay if configured
	if resolver.Delay != "" {
		if delay, err := time.ParseDuration(resolver.Delay); err == nil {
			select {
			case <-ctx.Done():
				return nil, &GraphQLError{Message: "request cancelled"}
			case <-time.After(delay):
			}
		}
	}

	// Check for error response
	if resolver.Error != nil {
		gqlErr := &GraphQLError{
			Message:    resolver.Error.Message,
			Extensions: resolver.Error.Extensions,
		}
		if resolver.Error.Path != nil {
			gqlErr.Path = make([]interface{}, len(resolver.Error.Path))
			for i, p := range resolver.Error.Path {
				gqlErr.Path[i] = p
			}
		}
		return nil, gqlErr
	}

	// Get arguments for template substitution
	args := e.extractArguments(field, variables)

	// Apply variable substitution to the response
	response := e.applyVariables(resolver.Response, args)

	return response, nil
}

// templatePattern matches {{args.fieldName}} patterns.
var templatePattern = regexp.MustCompile(`\{\{args\.([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// applyVariables substitutes {{args.fieldName}} templates and general template
// variables (like {{uuid}}, {{now}}, etc.) in response data.
func (e *Executor) applyVariables(data interface{}, args map[string]interface{}) interface{} {
	if data == nil {
		return nil
	}

	switch v := data.(type) {
	case string:
		// First, replace {{args.fieldName}} variables
		result := templatePattern.ReplaceAllStringFunc(v, func(match string) string {
			// Extract the field name from {{args.fieldName}}
			parts := templatePattern.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			fieldName := parts[1]
			if val, ok := args[fieldName]; ok {
				return fmt.Sprintf("%v", val)
			}
			return match
		})

		// Then process general template variables ({{uuid}}, {{now}}, etc.)
		// Create a template context with args as the body for request.body.* access
		ctx := template.NewContextFromMap(args, nil)
		processed, _ := e.templateEngine.Process(result, ctx)
		return processed

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = e.applyVariables(val, args)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = e.applyVariables(val, args)
		}
		return result

	default:
		return data
	}
}
