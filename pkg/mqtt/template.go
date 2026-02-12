package mqtt

import (
	"fmt"
	"regexp"
	"strings"

	templatepkg "github.com/getmockd/mockd/pkg/template"
)

// SequenceStore is an alias for the consolidated template.SequenceStore.
// This type alias maintains backward compatibility for MQTT code that
// references mqtt.SequenceStore and mqtt.NewSequenceStore.
type SequenceStore = templatepkg.SequenceStore

// NewSequenceStore creates a new sequence store.
// This is a convenience wrapper for template.NewSequenceStore.
var NewSequenceStore = templatepkg.NewSequenceStore

// TemplateContext provides values available during MQTT template evaluation.
// These values are mapped into template.Context.MQTT before processing.
type TemplateContext struct {
	Topic        string         // Full topic name
	WildcardVals []string       // Values matched by wildcards in order
	ClientID     string         // Publishing client's ID
	Payload      map[string]any // Parsed JSON payload (if valid JSON)
	DeviceID     string         // For multi-device simulation
	MessageNum   int64          // Sequence number for this simulation
}

// Regular expressions for MQTT topic utilities.
var (
	// Matches {{ variable }} patterns — used by ValidateTemplate.
	varPattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)
	// Pattern-based validators for ValidateTemplate.
	randomIntPattern   = regexp.MustCompile(`^random\.int(?:\((\d+),\s*(\d+)\))?$`)
	randomFloatPattern = regexp.MustCompile(`^random\.float(?:\(([0-9.]+),\s*([0-9.]+)(?:,\s*(\d+))?\))?$`)
	sequencePattern    = regexp.MustCompile(`^sequence\("([^"]+)"(?:,\s*(\d+))?\)$`)
	wildcardPattern    = regexp.MustCompile(`^\{(\d+)\}$`)
	payloadPattern     = regexp.MustCompile(`^payload\.(.+)$`)
	fakerPattern       = regexp.MustCompile(`^faker\.(\w+)$`)
)

// ProcessMQTTTemplate renders an MQTT template using the consolidated template engine.
// It converts TemplateContext into a template.Context and processes in a single pass —
// all shared variables (uuid, now, timestamp, random, etc.) and MQTT-specific variables
// (topic, clientId, device_id, payload.*, sequence(), faker.*, wildcards) are resolved together.
func ProcessMQTTTemplate(tmpl string, ctx *TemplateContext, sequences *SequenceStore) string {
	if ctx == nil {
		ctx = &TemplateContext{}
	}

	tctx := &templatepkg.Context{
		MQTT: templatepkg.MQTTContext{
			Topic:        ctx.Topic,
			ClientID:     ctx.ClientID,
			Payload:      ctx.Payload,
			WildcardVals: ctx.WildcardVals,
			DeviceID:     ctx.DeviceID,
			MessageNum:   ctx.MessageNum,
		},
	}

	engine := templatepkg.NewWithSequences(sequences)
	result, _ := engine.Process(tmpl, tctx)
	return result
}

// ExtractWildcardValues extracts wildcard values from a topic based on a pattern.
// pattern: "sensors/+/temperature" topic: "sensors/device1/temperature" -> ["device1"]
// pattern: "devices/#" topic: "devices/home/living/light" -> ["home/living/light"]
func ExtractWildcardValues(pattern, topic string) []string {
	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")

	var wildcards []string
	for i, part := range patternParts {
		if part == "#" {
			if i < len(topicParts) {
				wildcards = append(wildcards, strings.Join(topicParts[i:], "/"))
			}
			break
		}
		if part == "+" {
			if i < len(topicParts) {
				wildcards = append(wildcards, topicParts[i])
			}
		}
	}
	return wildcards
}

// RenderTopicTemplate renders a topic template with wildcard substitution.
// Template can use {1}, {2}, etc. for wildcard values.
// Example: "commands/{1}/response" with wildcards ["device1"] -> "commands/device1/response"
func RenderTopicTemplate(template string, wildcards []string) string {
	result := template
	for i, val := range wildcards {
		placeholder := fmt.Sprintf("{%d}", i+1)
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result
}

// ValidateTemplate checks if a template is valid by verifying all variable references
// are recognized MQTT or shared template expressions.
func ValidateTemplate(template string) (bool, []string) {
	var errors []string

	matches := varPattern.FindAllStringSubmatch(template, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			varName := strings.TrimSpace(match[1])
			if !isValidVariable(varName) {
				errors = append(errors, "unknown variable: "+varName)
			}
		}
	}

	return len(errors) == 0, errors
}

// isValidVariable checks if a variable name is valid for MQTT templates.
func isValidVariable(varName string) bool {
	switch varName {
	case "timestamp", "timestamp.iso", "timestamp.unix", "timestamp.unix_ms",
		"uuid", "topic", "clientId", "device_id",
		"now", "uuid.short", "random", "random.float", "random.int":
		return true
	}

	if randomIntPattern.MatchString(varName) ||
		randomFloatPattern.MatchString(varName) ||
		sequencePattern.MatchString(varName) ||
		wildcardPattern.MatchString(varName) ||
		payloadPattern.MatchString(varName) ||
		fakerPattern.MatchString(varName) {
		return true
	}

	// Allow space-separated function calls
	parts := strings.Fields(varName)
	if len(parts) >= 2 {
		switch parts[0] {
		case "random.int", "upper", "lower", "default":
			return true
		}
	}

	return false
}
