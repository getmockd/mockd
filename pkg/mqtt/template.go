package mqtt

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TemplateContext provides values available during template evaluation
type TemplateContext struct {
	Topic        string         // Full topic name
	WildcardVals []string       // Values matched by wildcards in order
	ClientID     string         // Publishing client's ID
	Payload      map[string]any // Parsed JSON payload (if valid JSON)
	DeviceID     string         // For multi-device simulation
	MessageNum   int64          // Sequence number for this simulation
}

// SequenceStore manages sequence state for {{ sequence("name") }} variables
type SequenceStore struct {
	sequences map[string]int64
	mu        sync.RWMutex
}

// NewSequenceStore creates a new sequence store
func NewSequenceStore() *SequenceStore {
	return &SequenceStore{
		sequences: make(map[string]int64),
	}
}

// Next returns the next value in a sequence
func (s *SequenceStore) Next(name string, start int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sequences[name]; !exists {
		s.sequences[name] = start
	}
	val := s.sequences[name]
	s.sequences[name]++
	return val
}

// Reset resets a sequence to its start value
func (s *SequenceStore) Reset(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sequences, name)
}

// Current returns the current value of a sequence without incrementing
func (s *SequenceStore) Current(name string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sequences[name]
}

// Template represents a parsed template with variable placeholders
type Template struct {
	raw       string
	sequences *SequenceStore
}

// NewTemplate creates a new template from a raw string
func NewTemplate(raw string, sequences *SequenceStore) *Template {
	if sequences == nil {
		sequences = NewSequenceStore()
	}
	return &Template{
		raw:       raw,
		sequences: sequences,
	}
}

// Regular expressions for template variable matching
var (
	// Matches {{ variable }} patterns
	varPattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)
	// Matches random.int(min, max) or random.int
	randomIntPattern = regexp.MustCompile(`^random\.int(?:\((\d+),\s*(\d+)\))?$`)
	// Matches random.float(min, max) or random.float(min, max, precision) or random.float
	randomFloatPattern = regexp.MustCompile(`^random\.float(?:\(([0-9.]+),\s*([0-9.]+)(?:,\s*(\d+))?\))?$`)
	// Matches sequence("name") or sequence("name", start)
	sequencePattern = regexp.MustCompile(`^sequence\("([^"]+)"(?:,\s*(\d+))?\)$`)
	// Matches {1}, {2} for wildcard substitution
	wildcardPattern = regexp.MustCompile(`^\{(\d+)\}$`)
	// Matches payload.field for JSON payload access
	payloadPattern = regexp.MustCompile(`^payload\.(.+)$`)
	// Matches faker.* variables
	fakerPattern = regexp.MustCompile(`^faker\.(\w+)$`)
)

// Render renders the template with the given context
func (t *Template) Render(ctx *TemplateContext) string {
	if ctx == nil {
		ctx = &TemplateContext{}
	}

	result := varPattern.ReplaceAllStringFunc(t.raw, func(match string) string {
		// Extract the variable name from {{ var }}
		inner := varPattern.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		varName := strings.TrimSpace(inner[1])
		return t.resolveVariable(varName, ctx)
	})

	return result
}

// resolveVariable resolves a single variable to its value
func (t *Template) resolveVariable(varName string, ctx *TemplateContext) string {
	// Handle simple variables first
	switch varName {
	case "timestamp", "timestamp.iso":
		return time.Now().UTC().Format(time.RFC3339Nano)
	case "timestamp.unix":
		return strconv.FormatInt(time.Now().Unix(), 10)
	case "timestamp.unix_ms":
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	case "uuid":
		return uuid.New().String()
	case "topic":
		return ctx.Topic
	case "clientId":
		return ctx.ClientID
	case "device_id":
		return ctx.DeviceID
	}

	// Handle random.int patterns
	if matches := randomIntPattern.FindStringSubmatch(varName); matches != nil {
		return t.resolveRandomInt(matches)
	}

	// Handle random.float patterns
	if matches := randomFloatPattern.FindStringSubmatch(varName); matches != nil {
		return t.resolveRandomFloat(matches)
	}

	// Handle sequence patterns
	if matches := sequencePattern.FindStringSubmatch(varName); matches != nil {
		return t.resolveSequence(matches)
	}

	// Handle wildcard substitution {1}, {2}, etc.
	if matches := wildcardPattern.FindStringSubmatch(varName); matches != nil {
		return t.resolveWildcard(matches, ctx)
	}

	// Handle payload.field access
	if matches := payloadPattern.FindStringSubmatch(varName); matches != nil {
		return t.resolvePayloadField(matches[1], ctx)
	}

	// Handle faker.* patterns
	if matches := fakerPattern.FindStringSubmatch(varName); matches != nil {
		return t.resolveFaker(matches[1])
	}

	// Unknown variable, return as-is
	return "{{ " + varName + " }}"
}

// resolveRandomInt resolves random.int or random.int(min, max)
func (t *Template) resolveRandomInt(matches []string) string {
	min, max := 0, 100
	if matches[1] != "" && matches[2] != "" {
		min, _ = strconv.Atoi(matches[1])
		max, _ = strconv.Atoi(matches[2])
	}
	if min >= max {
		max = min + 1
	}
	val := min + rand.Intn(max-min+1)
	return strconv.Itoa(val)
}

// resolveRandomFloat resolves random.float or random.float(min, max) or random.float(min, max, precision)
func (t *Template) resolveRandomFloat(matches []string) string {
	minVal, maxVal := 0.0, 1.0
	precision := -1 // -1 means no rounding

	if matches[1] != "" && matches[2] != "" {
		minVal, _ = strconv.ParseFloat(matches[1], 64)
		maxVal, _ = strconv.ParseFloat(matches[2], 64)
	}
	if matches[3] != "" {
		precision, _ = strconv.Atoi(matches[3])
	}

	if minVal >= maxVal {
		maxVal = minVal + 1
	}

	val := minVal + rand.Float64()*(maxVal-minVal)

	if precision >= 0 {
		format := fmt.Sprintf("%%.%df", precision)
		return fmt.Sprintf(format, val)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// resolveSequence resolves sequence("name") or sequence("name", start)
func (t *Template) resolveSequence(matches []string) string {
	name := matches[1]
	start := int64(1)
	if matches[2] != "" {
		start, _ = strconv.ParseInt(matches[2], 10, 64)
	}
	val := t.sequences.Next(name, start)
	return strconv.FormatInt(val, 10)
}

// resolveWildcard resolves {1}, {2}, etc. from wildcard matches
func (t *Template) resolveWildcard(matches []string, ctx *TemplateContext) string {
	idx, _ := strconv.Atoi(matches[1])
	if idx < 1 || idx > len(ctx.WildcardVals) {
		return ""
	}
	return ctx.WildcardVals[idx-1]
}

// resolvePayloadField resolves payload.field access
func (t *Template) resolvePayloadField(path string, ctx *TemplateContext) string {
	if ctx.Payload == nil {
		return ""
	}

	// Simple single-level access for now
	parts := strings.Split(path, ".")
	current := ctx.Payload

	for i, part := range parts {
		if val, ok := current[part]; ok {
			if i == len(parts)-1 {
				// Last part, return the value
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
			// Not last part, check if it's a map for nested access
			if nested, ok := val.(map[string]any); ok {
				current = nested
			} else {
				return ""
			}
		} else {
			return ""
		}
	}
	return ""
}

// resolveFaker resolves faker.* patterns (basic implementation)
func (t *Template) resolveFaker(fakerType string) string {
	switch fakerType {
	case "uuid":
		return uuid.New().String()
	case "boolean":
		if rand.Intn(2) == 0 {
			return "false"
		}
		return "true"
	case "name":
		names := []string{"John Smith", "Jane Doe", "Bob Johnson", "Alice Williams", "Charlie Brown"}
		return names[rand.Intn(len(names))]
	case "firstName":
		names := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Edward", "Fiona"}
		return names[rand.Intn(len(names))]
	case "lastName":
		names := []string{"Smith", "Doe", "Johnson", "Williams", "Brown", "Davis", "Miller", "Wilson"}
		return names[rand.Intn(len(names))]
	case "email":
		domains := []string{"example.com", "test.com", "mock.io", "demo.org"}
		names := []string{"john", "jane", "bob", "alice", "charlie"}
		return names[rand.Intn(len(names))] + strconv.Itoa(rand.Intn(1000)) + "@" + domains[rand.Intn(len(domains))]
	case "address":
		// FR-019: faker.address support
		streets := []string{"Main St", "Oak Ave", "Elm St", "Park Blvd", "Cedar Ln", "Maple Dr", "Pine Rd", "Lake Way"}
		cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Seattle", "Denver", "Boston"}
		states := []string{"NY", "CA", "IL", "TX", "AZ", "WA", "CO", "MA"}
		streetNum := rand.Intn(9999) + 1
		idx := rand.Intn(len(cities))
		return fmt.Sprintf("%d %s, %s, %s %05d", streetNum, streets[rand.Intn(len(streets))], cities[idx], states[idx], rand.Intn(99999))
	case "phone":
		return fmt.Sprintf("+1-%03d-%03d-%04d", rand.Intn(900)+100, rand.Intn(900)+100, rand.Intn(10000))
	case "company":
		companies := []string{"Acme Corp", "Globex Inc", "Initech", "Umbrella Corp", "Stark Industries", "Wayne Enterprises", "Cyberdyne Systems", "Tyrell Corp"}
		return companies[rand.Intn(len(companies))]
	case "word":
		words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "theta", "lambda", "sigma", "omega"}
		return words[rand.Intn(len(words))]
	case "sentence":
		sentences := []string{
			"The quick brown fox jumps over the lazy dog.",
			"Lorem ipsum dolor sit amet.",
			"Hello world from the IoT device.",
			"Sensor data transmitted successfully.",
			"System status nominal.",
		}
		return sentences[rand.Intn(len(sentences))]
	default:
		return "{{ faker." + fakerType + " }}"
	}
}

// ExtractWildcardValues extracts wildcard values from a topic based on a pattern
// pattern: "sensors/+/temperature" topic: "sensors/device1/temperature" -> ["device1"]
// pattern: "devices/#" topic: "devices/home/living/light" -> ["home/living/light"]
func ExtractWildcardValues(pattern, topic string) []string {
	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")

	var wildcards []string
	for i, part := range patternParts {
		if part == "#" {
			// # matches everything remaining
			if i < len(topicParts) {
				wildcards = append(wildcards, strings.Join(topicParts[i:], "/"))
			}
			break
		}
		if part == "+" {
			// + matches a single level
			if i < len(topicParts) {
				wildcards = append(wildcards, topicParts[i])
			}
		}
	}
	return wildcards
}

// RenderTopicTemplate renders a topic template with wildcard substitution
// Template can use {1}, {2}, etc. for wildcard values
// Example: "commands/{1}/response" with wildcards ["device1"] -> "commands/device1/response"
func RenderTopicTemplate(template string, wildcards []string) string {
	result := template
	for i, val := range wildcards {
		placeholder := fmt.Sprintf("{%d}", i+1)
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result
}

// ValidateTemplate checks if a template is valid
func ValidateTemplate(template string) (bool, []string) {
	var errors []string

	matches := varPattern.FindAllStringSubmatch(template, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			varName := strings.TrimSpace(match[1])
			if !isValidVariable(varName) {
				errors = append(errors, fmt.Sprintf("unknown variable: %s", varName))
			}
		}
	}

	return len(errors) == 0, errors
}

// isValidVariable checks if a variable name is valid
func isValidVariable(varName string) bool {
	// Simple variables
	switch varName {
	case "timestamp", "timestamp.iso", "timestamp.unix", "timestamp.unix_ms",
		"uuid", "topic", "clientId", "device_id":
		return true
	}

	// Pattern-based variables
	if randomIntPattern.MatchString(varName) ||
		randomFloatPattern.MatchString(varName) ||
		sequencePattern.MatchString(varName) ||
		wildcardPattern.MatchString(varName) ||
		payloadPattern.MatchString(varName) ||
		fakerPattern.MatchString(varName) {
		return true
	}

	return false
}
