package template

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Engine processes templates with variable substitution.
// It is stateless and thread-safe.
type Engine struct{}

// New creates a new template engine.
func New() *Engine {
	return &Engine{}
}

var templateRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Process evaluates a template string with the given context.
// It finds all {{expression}} patterns and replaces them with evaluated results.
// Returns the processed string with all substitutions made.
func (e *Engine) Process(template string, ctx *Context) (string, error) {
	result := templateRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract the expression between {{ and }}
		expr := strings.TrimSpace(match[2 : len(match)-2])
		return e.evaluate(expr, ctx)
	})

	return result, nil
}

// evaluate processes a single template expression and returns its value.
// Returns empty string on any errors to allow graceful degradation.
func (e *Engine) evaluate(expr string, ctx *Context) string {
	expr = strings.TrimSpace(expr)

	// Handle special built-in functions
	switch expr {
	case "now":
		return time.Now().Format(time.RFC3339)
	case "uuid":
		return uuid.New().String()
	case "timestamp":
		return fmt.Sprintf("%d", time.Now().Unix())
	case "random":
		// Generate a random 8-character hex string
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return ""
		}
		return fmt.Sprintf("%x", b)
	}

	// Handle request context fields
	if strings.HasPrefix(expr, "request.") {
		return e.evaluateRequest(expr[8:], ctx)
	}

	// Unknown expression - return empty string
	return ""
}

// evaluateRequest evaluates request.* expressions.
func (e *Engine) evaluateRequest(expr string, ctx *Context) string {
	if ctx == nil {
		return ""
	}

	parts := strings.SplitN(expr, ".", 2)
	field := parts[0]

	switch field {
	case "method":
		return ctx.Request.Method
	case "path":
		return ctx.Request.Path
	case "url":
		return ctx.Request.URL
	case "rawBody":
		return ctx.Request.RawBody
	case "body":
		// If there's a nested field like request.body.name
		if len(parts) == 2 && ctx.Request.Body != nil {
			return e.evaluateBodyField(parts[1], ctx.Request.Body)
		}
		// Return empty string if no nested field specified
		return ""
	case "query":
		// request.query.paramName returns first value
		if len(parts) == 2 {
			if values, ok := ctx.Request.Query[parts[1]]; ok && len(values) > 0 {
				return values[0]
			}
		}
		return ""
	case "header":
		// request.header.HeaderName returns first value
		if len(parts) == 2 {
			if values, ok := ctx.Request.Headers[parts[1]]; ok && len(values) > 0 {
				return values[0]
			}
		}
		return ""
	case "pathParam":
		// request.pathParam.paramName
		if len(parts) == 2 {
			if value, ok := ctx.Request.PathParams[parts[1]]; ok {
				return value
			}
		}
		return ""
	}

	return ""
}

// evaluateBodyField extracts a nested field from the parsed JSON body.
// Supports dot notation like "user.name" or "items.0.id"
func (e *Engine) evaluateBodyField(path string, body interface{}) string {
	parts := strings.Split(path, ".")
	current := body

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return ""
			}
		case []interface{}:
			// Array access not implemented in this basic version
			return ""
		default:
			return ""
		}
	}

	// Convert final value to string
	return fmt.Sprintf("%v", current)
}
