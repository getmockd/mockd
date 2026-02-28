package template

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Engine processes templates with variable substitution.
// An Engine with no SequenceStore is stateless and fully thread-safe.
// When a SequenceStore is attached (for MQTT sequence support), the store
// provides its own synchronization.
type Engine struct {
	sequences *SequenceStore
}

// New creates a new template engine with a default sequence store.
// Sequences like {{sequence("counter")}} work in all contexts (HTTP,
// GraphQL, SSE, SOAP, WebSocket, MQTT).
func New() *Engine {
	return &Engine{sequences: NewSequenceStore()}
}

// NewWithSequences creates a template engine with sequence support.
// The SequenceStore is used for {{sequence("name")}} expressions.
func NewWithSequences(store *SequenceStore) *Engine {
	return &Engine{sequences: store}
}

// templateRegex matches {{expression}} patterns with optional whitespace.
var templateRegex = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Compiled patterns for function-call syntax (parenthesized arguments).
var (
	// random.int / randomInt or random.int(min, max) / randomInt(min, max)
	randomIntPattern = regexp.MustCompile(`^(?:random\.int|randomInt)(?:\((\d+),\s*(\d+)\))?$`)
	// random.float / randomFloat or random.float(min, max) / randomFloat(min, max[, precision])
	randomFloatPattern = regexp.MustCompile(`^(?:random\.float|randomFloat)(?:\(([0-9.]+),\s*([0-9.]+)(?:,\s*(\d+))?\))?$`)
	// random.string / randomString or random.string(length) / randomString(length)
	randomStringPattern = regexp.MustCompile(`^(?:random\.string|randomString)(?:\((\d+)\))?$`)
	// sequence("name") or sequence("name", start)
	sequencePattern = regexp.MustCompile(`^sequence\("([^"]+)"(?:,\s*(\d+))?\)$`)
	// {1}, {2} for wildcard substitution
	wildcardPattern = regexp.MustCompile(`^\{(\d+)\}$`)
	// payload.field.nested
	payloadPattern = regexp.MustCompile(`^payload\.(.+)$`)
	// faker.type
	fakerPattern = regexp.MustCompile(`^faker\.(\w+)$`)
	// faker.type(args) — parameterized faker functions
	fakerParamPattern = regexp.MustCompile(`^faker\.(\w+)\((.+)\)$`)
	// upper(value) or lower(value) or default(value, fallback)
	funcCallPattern = regexp.MustCompile(`^(\w+)\((.+)\)$`)
)

// Process evaluates a template string with the given context.
// It finds all {{expression}} patterns and replaces them with evaluated results.
// Supports both parenthesized syntax: {{random.int(1, 100)}} and space-separated
// syntax: {{random.int 1 100}} for backward compatibility.
func (e *Engine) Process(template string, ctx *Context) (string, error) {
	result := templateRegex.ReplaceAllStringFunc(template, func(match string) string {
		inner := templateRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		expr := strings.TrimSpace(inner[1])
		return e.evaluate(expr, ctx)
	})

	return result, nil
}

// evaluate processes a single template expression and returns its value.
// Returns empty string for unknown expressions to allow graceful degradation.
func (e *Engine) evaluate(expr string, ctx *Context) string {
	expr = strings.TrimSpace(expr)

	// Extract the seeded RNG from context (nil = use global source).
	rng := ctxRNG(ctx)

	// Handle simple built-in variables (no arguments)
	switch expr {
	case "now":
		return time.Now().Format(time.RFC3339)
	case "uuid":
		return rngUUID(rng)
	case "uuid.short":
		u := rngUUID(rng)
		if len(u) >= 8 {
			return u[:8]
		}
		return u
	case "timestamp":
		return strconv.FormatInt(time.Now().Unix(), 10)
	case "timestamp.iso":
		return time.Now().UTC().Format(time.RFC3339Nano)
	case "timestamp.unix":
		return strconv.FormatInt(time.Now().Unix(), 10)
	case "timestamp.unix_ms":
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	case "random":
		// {{random}} (bare) stays crypto/rand for security when unseeded;
		// when seeded, uses the PRNG for reproducibility.
		if rng != nil {
			b := make([]byte, 4)
			for i := range b {
				b[i] = byte(rng.IntN(256))
			}
			return hex.EncodeToString(b)
		}
		b := make([]byte, 4)
		if _, err := cryptorand.Read(b); err != nil {
			return ""
		}
		return hex.EncodeToString(b)
	case "random.float", "randomFloat":
		return funcRandomFloat(rng)
	case "random.int", "randomInt":
		// No-arg form: random int 0-100
		return funcRandomInt(rng, 0, 100)
	case "random.string", "randomString":
		// No-arg form: random alphanumeric string of length 10
		return funcRandomString(rng, 10)
	}

	// Handle MQTT context variables
	if ctx != nil {
		switch expr {
		case "topic":
			return ctx.MQTT.Topic
		case "clientId":
			return ctx.MQTT.ClientID
		case "device_id":
			return ctx.MQTT.DeviceID
		}
	}

	// Handle parenthesized function calls: random.int(1, 100), sequence("name"), etc.
	if result, handled := e.evaluateParenthesized(expr, ctx, rng); handled {
		return result
	}

	// Handle legacy space-separated function calls: random.int 1 100, upper value, etc.
	if result, handled := e.evaluateSpaceSeparated(expr, ctx, rng); handled {
		return result
	}

	// Handle wildcard substitution: {1}, {2}
	if matches := wildcardPattern.FindStringSubmatch(expr); matches != nil {
		return e.resolveWildcard(matches, ctx)
	}

	// Handle payload.field access (MQTT incoming message data)
	if matches := payloadPattern.FindStringSubmatch(expr); matches != nil {
		return e.resolvePayloadField(matches[1], ctx)
	}

	// Handle parameterized faker.type(args) patterns
	if matches := fakerParamPattern.FindStringSubmatch(expr); matches != nil {
		return resolveFakerParam(rng, matches[1], matches[2])
	}

	// Handle faker.* patterns (no args)
	if matches := fakerPattern.FindStringSubmatch(expr); matches != nil {
		return resolveFaker(rng, matches[1])
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

// evaluateParenthesized handles function-call syntax: func(arg1, arg2)
func (e *Engine) evaluateParenthesized(expr string, ctx *Context, rng *mathrand.Rand) (string, bool) {
	// random.int(min, max)
	if matches := randomIntPattern.FindStringSubmatch(expr); matches != nil {
		if matches[1] != "" && matches[2] != "" {
			min, _ := strconv.Atoi(matches[1])
			max, _ := strconv.Atoi(matches[2])
			return funcRandomInt(rng, min, max), true
		}
		return "", false // no parens — will be caught by simple switch
	}

	// random.float(min, max) or random.float(min, max, precision)
	if matches := randomFloatPattern.FindStringSubmatch(expr); matches != nil {
		if matches[1] != "" && matches[2] != "" {
			return funcRandomFloatRange(rng, matches[1], matches[2], matches[3]), true
		}
		return "", false // no parens
	}

	// random.string(length)
	if matches := randomStringPattern.FindStringSubmatch(expr); matches != nil {
		length := 10 // default
		if matches[1] != "" {
			if n, err := strconv.Atoi(matches[1]); err == nil && n > 0 {
				length = n
			}
		}
		return funcRandomString(rng, length), true
	}

	// sequence("name") or sequence("name", start)
	if matches := sequencePattern.FindStringSubmatch(expr); matches != nil {
		return e.resolveSequence(matches), true
	}

	// upper(value), lower(value), default(value, fallback)
	if matches := funcCallPattern.FindStringSubmatch(expr); matches != nil {
		funcName := matches[1]
		argsStr := matches[2]

		switch funcName {
		case "upper":
			value := e.resolveValue(strings.TrimSpace(argsStr), ctx)
			return funcUpper(value), true
		case "lower":
			value := e.resolveValue(strings.TrimSpace(argsStr), ctx)
			return funcLower(value), true
		case "default":
			args := splitFuncArgs(argsStr)
			if len(args) >= 2 {
				value := e.resolveValue(args[0], ctx)
				fallback := parseStringArg(args[1])
				return funcDefault(value, fallback), true
			}
			return "", true
		}
	}

	return "", false
}

// evaluateSpaceSeparated handles legacy space-separated syntax: func arg1 arg2
func (e *Engine) evaluateSpaceSeparated(expr string, ctx *Context, rng *mathrand.Rand) (string, bool) {
	parts := strings.Fields(expr)
	if len(parts) < 2 {
		return "", false
	}

	funcName := parts[0]
	args := parts[1:]

	switch funcName {
	case "random.int", "randomInt":
		if len(args) != 2 {
			return "", true
		}
		min, err1 := strconv.Atoi(args[0])
		max, err2 := strconv.Atoi(args[1])
		if err1 != nil || err2 != nil {
			return "", true
		}
		return funcRandomInt(rng, min, max), true

	case "random.float", "randomFloat":
		// random.float min max  OR  random.float min max precision
		if len(args) < 2 || len(args) > 3 {
			return "", true
		}
		precision := ""
		if len(args) == 3 {
			precision = args[2]
		}
		return funcRandomFloatRange(rng, args[0], args[1], precision), true

	case "random.string", "randomString":
		if len(args) != 1 {
			return "", true
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			return "", true
		}
		return funcRandomString(rng, n), true

	case "upper":
		if len(args) != 1 {
			return "", true
		}
		value := e.resolveValue(args[0], ctx)
		return funcUpper(value), true

	case "lower":
		if len(args) != 1 {
			return "", true
		}
		value := e.resolveValue(args[0], ctx)
		return funcLower(value), true

	case "default":
		if len(args) < 2 {
			return "", true
		}
		value := e.resolveValue(args[0], ctx)
		fallback := parseStringArg(strings.Join(args[1:], " "))
		return funcDefault(value, fallback), true
	}

	return "", false
}

// resolveValue resolves a value reference.
// If it looks like a context path (e.g., request.body.name, payload.field,
// topic, uuid, etc.), it evaluates it through the main evaluator.
// Quoted strings are returned as literals with quotes removed.
func (e *Engine) resolveValue(ref string, ctx *Context) string {
	ref = strings.TrimSpace(ref)

	// Quoted strings are always literals
	if len(ref) >= 2 {
		if (ref[0] == '"' && ref[len(ref)-1] == '"') || (ref[0] == '\'' && ref[len(ref)-1] == '\'') {
			return ref[1 : len(ref)-1]
		}
	}

	// Known context prefixes and built-in names are evaluated as expressions
	if strings.HasPrefix(ref, "request.") ||
		strings.HasPrefix(ref, "mtls.") ||
		strings.HasPrefix(ref, "payload.") ||
		ref == "topic" || ref == "clientId" || ref == "device_id" ||
		ref == "uuid" || ref == "uuid.short" ||
		ref == "now" || ref == "timestamp" ||
		strings.HasPrefix(ref, "timestamp.") {
		return e.evaluate(ref, ctx)
	}

	return parseStringArg(ref)
}

// parseStringArg removes surrounding quotes from a string argument if present.
func parseStringArg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// splitFuncArgs splits function arguments separated by commas,
// respecting quoted strings.
func splitFuncArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case inQuote:
			current.WriteByte(ch)
			if ch == quoteChar {
				inQuote = false
			}
		case ch == '"' || ch == '\'':
			inQuote = true
			quoteChar = ch
			current.WriteByte(ch)
		case ch == ',':
			args = append(args, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}
	return args
}

// resolveSequence resolves sequence("name") or sequence("name", start)
func (e *Engine) resolveSequence(matches []string) string {
	if e.sequences == nil {
		return ""
	}
	name := matches[1]
	start := int64(1)
	if matches[2] != "" {
		start, _ = strconv.ParseInt(matches[2], 10, 64)
	}
	val := e.sequences.Next(name, start)
	return strconv.FormatInt(val, 10)
}

// resolveWildcard resolves {1}, {2}, etc. from MQTT wildcard matches
func (e *Engine) resolveWildcard(matches []string, ctx *Context) string {
	if ctx == nil {
		return ""
	}
	idx, _ := strconv.Atoi(matches[1])
	if idx < 1 || idx > len(ctx.MQTT.WildcardVals) {
		return ""
	}
	return ctx.MQTT.WildcardVals[idx-1]
}

// resolvePayloadField resolves payload.field access from MQTT message data
func (e *Engine) resolvePayloadField(path string, ctx *Context) string {
	if ctx == nil || ctx.MQTT.Payload == nil {
		return ""
	}

	parts := strings.Split(path, ".")
	current := ctx.MQTT.Payload

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return ""
		}
		if i == len(parts)-1 {
			return formatValue(val)
		}
		if nested, ok := val.(map[string]any); ok {
			current = nested
		} else {
			return ""
		}
	}
	return ""
}

// formatValue converts an arbitrary value to a string representation.
func formatValue(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// resolveFaker resolves faker.* patterns with realistic-looking sample data.
// When rng is non-nil, uses the seeded PRNG for deterministic output.
//
//nolint:gocyclo // Flat switch over faker types is intentional; splitting would add indirection without benefit.
func resolveFaker(rng *mathrand.Rand, fakerType string) string {
	fakerType = strings.ToLower(fakerType)
	switch fakerType {
	case "uuid":
		return rngUUID(rng)
	case "boolean":
		if rngIntN(rng, 2) == 0 {
			return "false"
		}
		return "true"
	case "name":
		names := []string{"John Smith", "Jane Doe", "Bob Johnson", "Alice Williams", "Charlie Brown"}
		return names[rngIntN(rng, len(names))]
	case "firstname":
		names := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Edward", "Fiona"}
		return names[rngIntN(rng, len(names))]
	case "lastname":
		names := []string{"Smith", "Doe", "Johnson", "Williams", "Brown", "Davis", "Miller", "Wilson"}
		return names[rngIntN(rng, len(names))]
	case "email":
		domains := []string{"example.com", "test.com", "mock.io", "demo.org"}
		fnames := []string{"john", "jane", "bob", "alice", "charlie"}
		return fnames[rngIntN(rng, len(fnames))] + strconv.Itoa(rngIntN(rng, 1000)) + "@" + domains[rngIntN(rng, len(domains))]
	case "address":
		streets := []string{"Main St", "Oak Ave", "Elm St", "Park Blvd", "Cedar Ln", "Maple Dr", "Pine Rd", "Lake Way"}
		cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Seattle", "Denver", "Boston"}
		states := []string{"NY", "CA", "IL", "TX", "AZ", "WA", "CO", "MA"}
		streetNum := rngIntN(rng, 9999) + 1
		idx := rngIntN(rng, len(cities))
		return fmt.Sprintf("%d %s, %s, %s %05d", streetNum, streets[rngIntN(rng, len(streets))], cities[idx], states[idx], rngIntN(rng, 99999))
	case "phone":
		return fmt.Sprintf("+1-%03d-%03d-%04d", rngIntN(rng, 900)+100, rngIntN(rng, 900)+100, rngIntN(rng, 10000))
	case "company":
		companies := []string{"Acme Corp", "Globex Inc", "Initech", "Umbrella Corp", "Stark Industries", "Wayne Enterprises", "Cyberdyne Systems", "Tyrell Corp"}
		return companies[rngIntN(rng, len(companies))]
	case "word":
		words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "theta", "lambda", "sigma", "omega"}
		return words[rngIntN(rng, len(words))]
	case "sentence":
		sentences := []string{
			"The quick brown fox jumps over the lazy dog.",
			"Lorem ipsum dolor sit amet.",
			"Hello world from the IoT device.",
			"Sensor data transmitted successfully.",
			"System status nominal.",
		}
		return sentences[rngIntN(rng, len(sentences))]

	// --- Internet ---
	case "ipv4":
		return fakerIPv4(rng)
	case "ipv6":
		return fakerIPv6(rng)
	case "macaddress":
		return fakerMACAddress(rng)
	case "useragent":
		return fakerUserAgents[rngIntN(rng, len(fakerUserAgents))]
	case "url":
		paths := []string{"api", "users", "posts", "products", "search", "dashboard", "settings"}
		domains := []string{"example.com", "api.example.com", "test.dev", "mock.local"}
		return "https://" + domains[rngIntN(rng, len(domains))] + "/" + paths[rngIntN(rng, len(paths))]

	// --- Finance ---
	case "creditcard":
		return fakerCreditCard(rng)
	case "creditcardexp":
		return fakerCreditCardExp(rng)
	case "cvv":
		return fakerCVV(rng)
	case "currencycode":
		return fakerCurrencyCodes[rngIntN(rng, len(fakerCurrencyCodes))]
	case "currency":
		return fakerCurrencyNames[rngIntN(rng, len(fakerCurrencyNames))]
	case "iban":
		return fakerIBAN(rng)

	// --- Commerce ---
	case "price":
		return fakerPrice(rng)
	case "productname":
		return fakerProductAdjectives[rngIntN(rng, len(fakerProductAdjectives))] + " " +
			fakerProductMaterials[rngIntN(rng, len(fakerProductMaterials))] + " " +
			fakerProductNouns[rngIntN(rng, len(fakerProductNouns))]
	case "color":
		return fakerColors[rngIntN(rng, len(fakerColors))]
	case "hexcolor":
		return fakerHexColor(rng)

	// --- Identity ---
	case "ssn":
		return fakerSSN(rng)
	case "passport":
		return fakerPassport(rng)
	case "jobtitle":
		return fakerJobLevels[rngIntN(rng, len(fakerJobLevels))] + " " +
			fakerJobFields[rngIntN(rng, len(fakerJobFields))] + " " +
			fakerJobRoles[rngIntN(rng, len(fakerJobRoles))]

	// --- Geo ---
	case "latitude":
		return fakerLatitude(rng)
	case "longitude":
		return fakerLongitude(rng)

	// --- Text ---
	case "words":
		return fakerWords(rng, rngIntN(rng, 3)+3) // default 3-5 words
	case "slug":
		return fakerSlug(rng)

	// --- Data ---
	case "mimetype":
		return fakerMIMETypes[rngIntN(rng, len(fakerMIMETypes))]
	case "fileextension":
		return fakerFileExtensions[rngIntN(rng, len(fakerFileExtensions))]

	default:
		return ""
	}
}

// resolveFakerParam resolves parameterized faker.type(args) patterns.
// When rng is non-nil, uses the seeded PRNG for deterministic output.
// Currently supports:
//   - faker.words(n) — generate n random words, space-separated
func resolveFakerParam(rng *mathrand.Rand, fakerType, argsStr string) string {
	fakerType = strings.ToLower(fakerType)
	switch fakerType {
	case "words":
		n, err := strconv.Atoi(strings.TrimSpace(argsStr))
		if err != nil || n <= 0 {
			return fakerWords(rng, 3) // default fallback
		}
		return fakerWords(rng, n)
	default:
		return ""
	}
}

// evaluateMTLS evaluates mtls.* expressions.
func (e *Engine) evaluateMTLS(expr string, ctx *Context) string {
	if ctx == nil || !ctx.MTLS.Present {
		return ""
	}

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
		if len(parts) == 2 && parts[1] == "cn" {
			return ctx.MTLS.IssuerCN
		}
		return ""
	case "san":
		if len(parts) == 2 {
			switch parts[1] {
			case "dns":
				return ctx.MTLS.SANDNS
			case "email":
				return ctx.MTLS.SANEmail
			case "ip":
				return ctx.MTLS.SANIP
			case "uri":
				return ctx.MTLS.SANURI
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
		if len(parts) == 2 && ctx.Request.Body != nil {
			return e.evaluateBodyField(parts[1], ctx.Request.Body)
		}
		return ""
	case "query":
		if len(parts) == 2 {
			if values, ok := ctx.Request.Query[parts[1]]; ok && len(values) > 0 {
				return values[0]
			}
		}
		return ""
	case "header":
		if len(parts) == 2 {
			key := http.CanonicalHeaderKey(parts[1])
			if values, ok := ctx.Request.Headers[key]; ok && len(values) > 0 {
				return values[0]
			}
		}
		return ""
	case "pathParam":
		if len(parts) == 2 {
			if value, ok := ctx.Request.PathParams[parts[1]]; ok {
				return value
			}
		}
		return ""
	case "pathPattern":
		if len(parts) == 2 {
			if value, ok := ctx.Request.PathPatternCaptures[parts[1]]; ok {
				return value
			}
		}
		return ""
	case "jsonPath":
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
			return ""
		default:
			return ""
		}
	}

	return fmt.Sprintf("%v", current)
}
