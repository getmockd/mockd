package template

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
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
	case "uuid.short":
		return funcUUIDShort()
	case "timestamp":
		return strconv.FormatInt(time.Now().Unix(), 10)
	case "random":
		// Generate a random 8-character hex string
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return ""
		}
		return hex.EncodeToString(b)
	case "random.float":
		return funcRandomFloat()
	}

	// Handle functions with arguments
	if result, handled := e.evaluateFunctionWithArgs(expr, ctx); handled {
		return result
	}

	// Handle request context fields
	if strings.HasPrefix(expr, "request.") {
		return e.evaluateRequest(expr[8:], ctx)
	}

	// Handle mTLS context fields
	if strings.HasPrefix(expr, "mtls.") {
		return e.evaluateMTLS(expr[5:], ctx)
	}

	// Unknown expression - return empty string
	return ""
}

// evaluateFunctionWithArgs handles functions that take arguments.
// Returns the result and true if the expression was handled, empty string and false otherwise.
func (e *Engine) evaluateFunctionWithArgs(expr string, ctx *Context) (string, bool) {
	parts := strings.Fields(expr)
	if len(parts) == 0 {
		return "", false
	}

	funcName := parts[0]
	args := parts[1:]

	switch funcName {
	case "random.int":
		// {{random.int min max}}
		if len(args) != 2 {
			return "", true // handled but invalid args
		}
		min, err1 := strconv.Atoi(args[0])
		max, err2 := strconv.Atoi(args[1])
		if err1 != nil || err2 != nil {
			return "", true
		}
		return funcRandomInt(min, max), true

	case "upper":
		// {{upper value}} - value is looked up from context or used as literal
		if len(args) != 1 {
			return "", true
		}
		value := e.resolveValue(args[0], ctx)
		return funcUpper(value), true

	case "lower":
		// {{lower value}} - value is looked up from context or used as literal
		if len(args) != 1 {
			return "", true
		}
		value := e.resolveValue(args[0], ctx)
		return funcLower(value), true

	case "default":
		// {{default value "fallback"}} or {{default value fallback}}
		if len(args) < 2 {
			return "", true
		}
		value := e.resolveValue(args[0], ctx)
		// Join remaining args and handle quoted strings
		fallback := e.parseStringArg(strings.Join(args[1:], " "))
		return funcDefault(value, fallback), true
	}

	return "", false
}

// resolveValue resolves a value reference.
// If it looks like a context path (e.g., request.body.name), it evaluates it.
// Otherwise, it returns the literal value.
func (e *Engine) resolveValue(ref string, ctx *Context) string {
	// Check if it's a context reference
	if strings.HasPrefix(ref, "request.") {
		return e.evaluateRequest(ref[8:], ctx)
	}
	if strings.HasPrefix(ref, "mtls.") {
		return e.evaluateMTLS(ref[5:], ctx)
	}
	// Return as literal (after stripping quotes if present)
	return e.parseStringArg(ref)
}

// parseStringArg removes surrounding quotes from a string argument if present.
func (e *Engine) parseStringArg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// evaluateMTLS evaluates mtls.* expressions.
func (e *Engine) evaluateMTLS(expr string, ctx *Context) string {
	if ctx == nil || !ctx.MTLS.Present {
		return ""
	}

	// Handle nested expressions like mtls.issuer.cn or mtls.san.dns
	parts := strings.SplitN(expr, ".", 2)
	field := parts[0]

	switch field {
	case "cn":
		return ctx.MTLS.CN
	case "o":
		return ctx.MTLS.O
	case "ou":
		return ctx.MTLS.OU
	case "serial":
		return ctx.MTLS.Serial
	case "fingerprint":
		return ctx.MTLS.Fingerprint
	case "notBefore":
		return ctx.MTLS.NotBefore
	case "notAfter":
		return ctx.MTLS.NotAfter
	case "verified":
		if ctx.MTLS.Verified {
			return "true"
		}
		return "false"
	case "issuer":
		// Handle mtls.issuer.cn
		if len(parts) == 2 && parts[1] == "cn" {
			return ctx.MTLS.IssuerCN
		}
		return ""
	case "san":
		// Handle mtls.san.dns and mtls.san.email
		if len(parts) == 2 {
			switch parts[1] {
			case "dns":
				return ctx.MTLS.SANDNS
			case "email":
				return ctx.MTLS.SANEmail
			}
		}
		return ""
	}

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
		// Uses canonical header key for case-insensitive lookup
		if len(parts) == 2 {
			key := http.CanonicalHeaderKey(parts[1])
			if values, ok := ctx.Request.Headers[key]; ok && len(values) > 0 {
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
	case "pathPattern":
		// request.pathPattern.captureName - named capture groups from PathPattern regex
		if len(parts) == 2 {
			if value, ok := ctx.Request.PathPatternCaptures[parts[1]]; ok {
				return value
			}
		}
		return ""
	case "jsonPath":
		// request.jsonPath.keyName returns matched JSONPath value
		if len(parts) == 2 {
			if value, ok := ctx.Request.JSONPath[parts[1]]; ok {
				return fmt.Sprintf("%v", value)
			}
		}
		return ""
	}

	return ""
}

// ProcessInterface recursively processes all string values in an interface{}
// with template variables. This is useful for processing nested data structures
// like GraphQL responses or gRPC response configs.
// Non-string values are returned unchanged.
func (e *Engine) ProcessInterface(data interface{}, ctx *Context) interface{} {
	if data == nil {
		return nil
	}

	switch v := data.(type) {
	case string:
		result, _ := e.Process(v, ctx)
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = e.ProcessInterface(val, ctx)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = e.ProcessInterface(val, ctx)
		}
		return result
	default:
		return data
	}
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
