package portability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// Mockoon environment types — mirrors the Mockoon export JSON structure.

// MockoonEnvironment represents a Mockoon environment (the top-level export format).
type MockoonEnvironment struct {
	UUID           string          `json:"uuid"`
	Name           string          `json:"name"`
	Port           int             `json:"port"`
	EndpointPrefix string          `json:"endpointPrefix"`
	Routes         []MockoonRoute  `json:"routes"`
	Databuckets    []MockoonBucket `json:"databuckets,omitempty"`
	TLSOptions     *MockoonTLS     `json:"tlsOptions,omitempty"`
	Latency        int             `json:"latency,omitempty"` // global latency in ms
}

// MockoonRoute represents a single route in a Mockoon environment.
type MockoonRoute struct {
	UUID            string            `json:"uuid"`
	Type            string            `json:"type"` // "http" or "crud"
	Documentation   string            `json:"documentation"`
	Method          string            `json:"method"`
	Endpoint        string            `json:"endpoint"`
	Responses       []MockoonResponse `json:"responses"`
	ResponseMode    *string           `json:"responseMode"` // null, "SEQUENTIAL", "RANDOM"
	Enabled         bool              `json:"enabled"`
	DatabucketID    string            `json:"databucketID,omitempty"` // for CRUD routes
	RandomResponse  bool              `json:"randomResponse"`
	SequentialIndex int               `json:"sequentialReponseNumber"` // sic: Mockoon's actual field name
}

// MockoonResponse represents a single response definition.
type MockoonResponse struct {
	UUID            string          `json:"uuid"`
	StatusCode      int             `json:"statusCode"`
	Label           string          `json:"label"`
	Body            string          `json:"body"`
	Headers         []MockoonHeader `json:"headers"`
	FilePath        string          `json:"filePath,omitempty"`
	SendFileAsBody  bool            `json:"sendFileAsBody"`
	LatencyMs       int             `json:"latency"` // per-response latency
	Rules           []MockoonRule   `json:"rules"`
	RulesOperator   string          `json:"rulesOperator"` // "OR" or "AND"
	DisableTemplate bool            `json:"disableTemplating"`
}

// MockoonHeader represents a Mockoon response header.
type MockoonHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// MockoonRule represents a response selection rule.
type MockoonRule struct {
	Target   string `json:"target"`   // "query", "header", "body", "cookie", "route_param", "request_number"
	Modifier string `json:"modifier"` // param name, header name, or JSONPath
	Value    string `json:"value"`    // expected value
	Operator string `json:"operator"` // "equals", "regex", "null"
	Invert   bool   `json:"invert"`
}

// MockoonBucket represents a data bucket (stateful data store).
type MockoonBucket struct {
	UUID  string `json:"uuid"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"` // JSON string containing seed data
}

// MockoonTLS represents Mockoon's TLS configuration.
type MockoonTLS struct {
	Enabled    bool   `json:"enabled"`
	Type       string `json:"type"` // "CERT", "PFX"
	PFXPath    string `json:"pfxPath"`
	CertPath   string `json:"certPath"`
	KeyPath    string `json:"keyPath"`
	CAPath     string `json:"caPath"`
	Passphrase string `json:"passphrase"`
}

// MockoonImporter imports Mockoon environment JSON files.
type MockoonImporter struct{}

func init() {
	RegisterImporter(&MockoonImporter{})
}

// Format returns the format this importer handles.
func (i *MockoonImporter) Format() Format {
	return FormatMockoon
}

// Import parses a Mockoon environment JSON and returns a MockCollection.
func (i *MockoonImporter) Import(data []byte) (*config.MockCollection, error) {
	var env MockoonEnvironment
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, &ImportError{
			Format:  FormatMockoon,
			Message: "failed to parse Mockoon environment JSON",
			Cause:   err,
		}
	}

	if len(env.Routes) == 0 {
		return nil, &ImportError{
			Format:  FormatMockoon,
			Message: "Mockoon environment contains no routes",
		}
	}

	return i.convertEnvironment(&env)
}

// convertEnvironment converts a full Mockoon environment to a MockCollection.
func (i *MockoonImporter) convertEnvironment(env *MockoonEnvironment) (*config.MockCollection, error) {
	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "Imported from Mockoon: " + env.Name,
		Mocks:   make([]*config.MockConfiguration, 0, len(env.Routes)),
	}

	now := time.Now()

	for idx, route := range env.Routes {
		if !route.Enabled {
			continue
		}

		switch route.Type {
		case "crud":
			// CRUD routes become stateful resources
			resource := i.convertCRUDRoute(&route, env, now)
			if resource != nil {
				collection.StatefulResources = append(collection.StatefulResources, resource)
			}
		default:
			// HTTP routes become mocks (one mock per response)
			mocks := i.convertHTTPRoute(&route, env, idx+1, now)
			collection.Mocks = append(collection.Mocks, mocks...)
		}
	}

	return collection, nil
}

// convertHTTPRoute converts a Mockoon HTTP route to one or more MockConfigurations.
// If the route has multiple responses with rules, each response becomes a separate mock
// with the rules converted to matchers. If no rules, the first response is used.
func (i *MockoonImporter) convertHTTPRoute(route *MockoonRoute, env *MockoonEnvironment, idx int, now time.Time) []*config.MockConfiguration {
	if len(route.Responses) == 0 {
		return nil
	}

	// Build the full path with endpoint prefix
	path := i.buildPath(route.Endpoint, env.EndpointPrefix)
	method := strings.ToUpper(route.Method)
	if method == "" {
		method = "GET"
	}

	var mocks []*config.MockConfiguration

	for respIdx, resp := range route.Responses {
		enabled := true
		mockID := i.generateMockID(route, respIdx, now)

		m := &config.MockConfiguration{
			ID:        mockID,
			Name:      i.generateMockName(route, &resp, idx, respIdx),
			Type:      mock.TypeHTTP,
			Enabled:   &enabled,
			CreatedAt: now,
			UpdatedAt: now,
			HTTP: &mock.HTTPSpec{
				Priority: len(route.Responses) - respIdx, // higher priority for earlier responses
				Matcher:  &mock.HTTPMatcher{},
				Response: &mock.HTTPResponse{
					StatusCode: resp.StatusCode,
					Headers:    make(map[string]string),
				},
			},
		}

		// Set method and path
		m.HTTP.Matcher.Method = method
		m.HTTP.Matcher.Path = path

		// Convert Mockoon :param path params to mockd {param} style
		m.HTTP.Matcher.Path = convertPathParams(m.HTTP.Matcher.Path)

		// Convert response headers
		for _, h := range resp.Headers {
			if h.Key != "" {
				m.HTTP.Response.Headers[h.Key] = h.Value
			}
		}

		// Convert response body
		if resp.FilePath != "" && resp.SendFileAsBody {
			m.HTTP.Response.BodyFile = resp.FilePath
		} else if resp.Body != "" {
			if resp.DisableTemplate {
				m.HTTP.Response.Body = resp.Body
			} else {
				m.HTTP.Response.Body = convertMockoonTemplates(resp.Body)
			}
		}

		// Calculate delay: per-response latency + global latency
		delay := resp.LatencyMs + env.Latency
		if delay > 0 {
			m.HTTP.Response.DelayMs = delay
		}

		// Default status code
		if m.HTTP.Response.StatusCode == 0 {
			m.HTTP.Response.StatusCode = 200
		}

		// Convert response rules to additional matchers
		if len(resp.Rules) > 0 {
			i.applyRulesToMatcher(m.HTTP.Matcher, resp.Rules)
		}

		mocks = append(mocks, m)
	}

	return mocks
}

// convertCRUDRoute converts a Mockoon CRUD route to a StatefulResourceConfig.
func (i *MockoonImporter) convertCRUDRoute(route *MockoonRoute, env *MockoonEnvironment, _ time.Time) *config.StatefulResourceConfig {
	path := i.buildPath(route.Endpoint, env.EndpointPrefix)
	path = convertPathParams(path)

	// Find the associated data bucket
	var seedData []map[string]interface{}
	for _, bucket := range env.Databuckets {
		if bucket.ID == route.DatabucketID || bucket.UUID == route.DatabucketID {
			_ = json.Unmarshal([]byte(bucket.Value), &seedData)
			break
		}
	}

	name := route.Documentation
	if name == "" {
		name = route.Endpoint
	}

	return &config.StatefulResourceConfig{
		Name:     name,
		BasePath: path,
		IDField:  "id",
		SeedData: seedData,
	}
}

// buildPath constructs the full path from an endpoint and optional prefix.
func (i *MockoonImporter) buildPath(endpoint, prefix string) string {
	// Mockoon endpoints don't start with /; add it
	endpoint = strings.TrimPrefix(endpoint, "/")
	prefix = strings.TrimSuffix(strings.TrimPrefix(prefix, "/"), "/")

	if prefix != "" {
		return "/" + prefix + "/" + endpoint
	}
	return "/" + endpoint
}

// generateMockID generates a unique, deterministic ID for a mock.
func (i *MockoonImporter) generateMockID(route *MockoonRoute, respIdx int, now time.Time) string {
	if route.UUID != "" {
		if respIdx == 0 {
			return "mn_" + route.UUID[:min(16, len(route.UUID))]
		}
		return fmt.Sprintf("mn_%s_%d", route.UUID[:min(12, len(route.UUID))], respIdx)
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("mockoon-%s-%s-%d-%d", route.Method, route.Endpoint, respIdx, now.UnixNano())))
	return "mn_" + hex.EncodeToString(h[:8])
}

// generateMockName creates a readable name for the mock.
func (i *MockoonImporter) generateMockName(route *MockoonRoute, resp *MockoonResponse, _, respIdx int) string {
	if resp.Label != "" {
		return resp.Label
	}
	if route.Documentation != "" {
		if respIdx > 0 {
			return fmt.Sprintf("%s (response %d)", route.Documentation, respIdx+1)
		}
		return route.Documentation
	}
	method := strings.ToUpper(route.Method)
	if method == "" {
		method = "GET"
	}
	return fmt.Sprintf("Mockoon Import: %s /%s", method, route.Endpoint)
}

// applyRulesToMatcher converts Mockoon response rules to mockd matcher fields.
func (i *MockoonImporter) applyRulesToMatcher(matcher *mock.HTTPMatcher, rules []MockoonRule) {
	for _, rule := range rules {
		if rule.Invert {
			continue // mockd doesn't support inverted rules natively; skip
		}

		switch rule.Target {
		case "query":
			if matcher.QueryParams == nil {
				matcher.QueryParams = make(map[string]string)
			}
			matcher.QueryParams[rule.Modifier] = rule.Value

		case "header":
			if matcher.Headers == nil {
				matcher.Headers = make(map[string]string)
			}
			matcher.Headers[rule.Modifier] = rule.Value

		case "body":
			switch rule.Operator {
			case "regex":
				matcher.BodyPattern = rule.Value
			case "equals":
				if strings.HasPrefix(rule.Modifier, "$.") || strings.HasPrefix(rule.Modifier, "$[") {
					// JSONPath-style body matching
					if matcher.BodyJSONPath == nil {
						matcher.BodyJSONPath = make(map[string]interface{})
					}
					matcher.BodyJSONPath[rule.Modifier] = rule.Value
				} else {
					matcher.BodyContains = rule.Value
				}
			default:
				matcher.BodyContains = rule.Value
			}

		case "cookie":
			// mockd doesn't have a dedicated cookie matcher;
			// express as a Cookie header contains check
			if matcher.Headers == nil {
				matcher.Headers = make(map[string]string)
			}
			// Best effort: set Cookie header with key=value
			existing := matcher.Headers["Cookie"]
			if existing != "" {
				existing += "; "
			}
			matcher.Headers["Cookie"] = existing + rule.Modifier + "=" + rule.Value

		case "route_param":
			// Route params are already in the path pattern; no extra matcher needed

		case "request_number":
			// Mockoon's request_number is a counter-based rule.
			// Not directly mappable to mockd matchers — skip silently.
		}
	}
}

// convertPathParams converts Mockoon :param syntax to mockd {param} syntax.
// e.g., "/users/:id/posts/:postId" -> "/users/{id}/posts/{postId}"
var mockoonParamPattern = regexp.MustCompile(`:(\w+)`)

func convertPathParams(path string) string {
	return mockoonParamPattern.ReplaceAllString(path, "{$1}")
}

// Mockoon template conversion — Handlebars → mockd template syntax

// mockoonHelperPatterns maps Mockoon Handlebars helpers to mockd template expressions.
var mockoonHelperPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// Request data helpers — mockd uses dot-notation: {{request.pathParam.id}}, {{request.query.page}}, etc.
	{regexp.MustCompile(`\{\{urlParam\s+'(\w+)'\}\}`), `{{request.pathParam.$1}}`},
	{regexp.MustCompile(`\{\{urlParam\s+"(\w+)"\}\}`), `{{request.pathParam.$1}}`},
	{regexp.MustCompile(`\{\{queryParam\s+'(\w+)'\}\}`), `{{request.query.$1}}`},
	{regexp.MustCompile(`\{\{queryParam\s+"(\w+)"\}\}`), `{{request.query.$1}}`},
	{regexp.MustCompile(`\{\{header\s+'([^']+)'\}\}`), `{{request.header.$1}}`},
	{regexp.MustCompile(`\{\{header\s+"([^"]+)"\}\}`), `{{request.header.$1}}`},
	{regexp.MustCompile(`\{\{body\s+'([^']+)'\}\}`), `{{request.body.$1}}`},
	{regexp.MustCompile(`\{\{body\s+"([^"]+)"\}\}`), `{{request.body.$1}}`},
	{regexp.MustCompile(`\{\{bodyRaw\}\}`), `{{request.rawBody}}`},

	// Utility helpers
	{regexp.MustCompile(`\{\{uuid\}\}`), `{{ uuid }}`},
	{regexp.MustCompile(`\{\{now\s+'[^']*'\}\}`), `{{ now }}`}, // format ignored — mockd uses Go format
	{regexp.MustCompile(`\{\{now\s+"[^"]*"\}\}`), `{{ now }}`}, // format ignored
	{regexp.MustCompile(`\{\{now\}\}`), `{{ now }}`},

	// Faker helpers — map Mockoon's {{faker 'category.method'}} to mockd's {{faker.type}}
	{regexp.MustCompile(`\{\{faker\s+'person\.firstName'\}\}`), `{{ faker.firstName }}`},
	{regexp.MustCompile(`\{\{faker\s+"person\.firstName"\}\}`), `{{ faker.firstName }}`},
	{regexp.MustCompile(`\{\{faker\s+'person\.lastName'\}\}`), `{{ faker.lastName }}`},
	{regexp.MustCompile(`\{\{faker\s+"person\.lastName"\}\}`), `{{ faker.lastName }}`},
	{regexp.MustCompile(`\{\{faker\s+'person\.fullName'\}\}`), `{{ faker.name }}`},
	{regexp.MustCompile(`\{\{faker\s+"person\.fullName"\}\}`), `{{ faker.name }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.email'\}\}`), `{{ faker.email }}`},
	{regexp.MustCompile(`\{\{faker\s+"internet\.email"\}\}`), `{{ faker.email }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.url'\}\}`), `{{faker.url}}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.ip'\}\}`), `{{ faker.ipv4 }}`},
	{regexp.MustCompile(`\{\{faker\s+"internet\.ip"\}\}`), `{{ faker.ipv4 }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.ipv4'\}\}`), `{{ faker.ipv4 }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.ipv6'\}\}`), `{{ faker.ipv6 }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.userName'\}\}`), `{{ faker.firstName }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.userAgent'\}\}`), `{{ faker.userAgent }}`},
	{regexp.MustCompile(`\{\{faker\s+'phone\.number'\}\}`), `{{ faker.phone }}`},
	{regexp.MustCompile(`\{\{faker\s+"phone\.number"\}\}`), `{{ faker.phone }}`},
	{regexp.MustCompile(`\{\{faker\s+'location\.city'\}\}`), `{{ faker.address }}`},
	{regexp.MustCompile(`\{\{faker\s+"location\.city"\}\}`), `{{ faker.address }}`},
	{regexp.MustCompile(`\{\{faker\s+'location\.country'\}\}`), `{{ faker.address }}`},
	{regexp.MustCompile(`\{\{faker\s+'location\.streetAddress'\}\}`), `{{ faker.address }}`},
	{regexp.MustCompile(`\{\{faker\s+'location\.zipCode'\}\}`), `{{ faker.address }}`},
	{regexp.MustCompile(`\{\{faker\s+'location\.latitude'\}\}`), `{{ faker.latitude }}`},
	{regexp.MustCompile(`\{\{faker\s+'location\.longitude'\}\}`), `{{ faker.longitude }}`},
	{regexp.MustCompile(`\{\{faker\s+'company\.name'\}\}`), `{{ faker.company }}`},
	{regexp.MustCompile(`\{\{faker\s+"company\.name"\}\}`), `{{ faker.company }}`},
	{regexp.MustCompile(`\{\{faker\s+'company\.catchPhrase'\}\}`), `{{ faker.sentence }}`},
	{regexp.MustCompile(`\{\{faker\s+'lorem\.sentence'\}\}`), `{{ faker.sentence }}`},
	{regexp.MustCompile(`\{\{faker\s+"lorem\.sentence"\}\}`), `{{ faker.sentence }}`},
	{regexp.MustCompile(`\{\{faker\s+'lorem\.paragraph'\}\}`), `{{ faker.sentence }}`},
	{regexp.MustCompile(`\{\{faker\s+'lorem\.word'\}\}`), `{{ faker.word }}`},
	{regexp.MustCompile(`\{\{faker\s+"lorem\.word"\}\}`), `{{ faker.word }}`},
	{regexp.MustCompile(`\{\{faker\s+'lorem\.words'\}\}`), `{{ faker.words }}`},
	{regexp.MustCompile(`\{\{faker\s+'date\.past'\}\}`), `{{ now }}`},
	{regexp.MustCompile(`\{\{faker\s+'date\.future'\}\}`), `{{ now }}`},
	{regexp.MustCompile(`\{\{faker\s+'date\.recent'\}\}`), `{{ now }}`},
	{regexp.MustCompile(`\{\{faker\s+'string\.uuid'\}\}`), `{{ uuid }}`},
	{regexp.MustCompile(`\{\{faker\s+"string\.uuid"\}\}`), `{{ uuid }}`},
	{regexp.MustCompile(`\{\{faker\s+'number\.int'\}\}`), `{{ random.int(1, 1000) }}`},
	{regexp.MustCompile(`\{\{faker\s+"number\.int"\}\}`), `{{ random.int(1, 1000) }}`},
	{regexp.MustCompile(`\{\{faker\s+'number\.float'\}\}`), `{{ random.float(0.0, 100.0, 2) }}`},
	{regexp.MustCompile(`\{\{faker\s+'datatype\.boolean'\}\}`), `{{ faker.boolean }}`},
	{regexp.MustCompile(`\{\{faker\s+'finance\.amount'\}\}`), `{{ faker.price }}`},
	{regexp.MustCompile(`\{\{faker\s+'finance\.currencyCode'\}\}`), `{{ faker.currencyCode }}`},
	{regexp.MustCompile(`\{\{faker\s+'finance\.creditCardNumber'\}\}`), `{{ faker.creditCard }}`},
	{regexp.MustCompile(`\{\{faker\s+'finance\.iban'\}\}`), `{{ faker.iban }}`},
	{regexp.MustCompile(`\{\{faker\s+'commerce\.productName'\}\}`), `{{ faker.productName }}`},
	{regexp.MustCompile(`\{\{faker\s+'color\.human'\}\}`), `{{ faker.color }}`},
	{regexp.MustCompile(`\{\{faker\s+'person\.jobTitle'\}\}`), `{{ faker.jobTitle }}`},
	{regexp.MustCompile(`\{\{faker\s+"person\.jobTitle"\}\}`), `{{ faker.jobTitle }}`},
	{regexp.MustCompile(`\{\{faker\s+'system\.mimeType'\}\}`), `{{ faker.mimeType }}`},
	{regexp.MustCompile(`\{\{faker\s+'system\.fileExt'\}\}`), `{{ faker.fileExtension }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.color'\}\}`), `{{ faker.hexColor }}`},
	{regexp.MustCompile(`\{\{faker\s+'internet\.mac'\}\}`), `{{ faker.macAddress }}`},
}

// Generic faker fallback: {{faker 'anything.whatever'}} → {{ faker.word }}
var mockoonGenericFakerPattern = regexp.MustCompile(`\{\{faker\s+['"]([^'"]+)['"]\}\}`)

// convertMockoonTemplates converts Mockoon Handlebars template syntax to mockd template syntax.
func convertMockoonTemplates(body string) string {
	result := body

	// Apply specific patterns first (most precise matches)
	for _, p := range mockoonHelperPatterns {
		result = p.pattern.ReplaceAllString(result, p.replacement)
	}

	// Catch-all: any remaining {{faker 'x.y'}} patterns → {{ faker.word }}
	result = mockoonGenericFakerPattern.ReplaceAllString(result, "{{ faker.word }}")

	return result
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
