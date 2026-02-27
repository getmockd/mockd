package mqtt

import (
	"fmt"
	"strings"

	templatepkg "github.com/getmockd/mockd/pkg/template"
)

// ProcessTemplate renders an MQTT template using the consolidated template engine.
// It builds a template.Context with the MQTT field populated and delegates all
// processing to pkg/template — there is no MQTT-specific template engine.
//
// All shared variables (uuid, now, timestamp, random, faker.*, sequence(), etc.)
// and MQTT-specific variables (topic, clientId, device_id, payload.*, wildcards)
// are resolved in a single pass by the unified engine.
func ProcessTemplate(tmpl string, ctx *templatepkg.Context, sequences *templatepkg.SequenceStore) string {
	if ctx == nil {
		ctx = &templatepkg.Context{}
	}

	engine := templatepkg.NewWithSequences(sequences)
	result, _ := engine.Process(tmpl, ctx)
	return result
}

// NewTemplateContext is a convenience constructor that builds a template.Context
// populated with MQTT-specific data. Callers that only need MQTT context can use
// this instead of constructing template.Context directly.
func NewTemplateContext(topic, clientID string, payload map[string]any, wildcardVals []string) *templatepkg.Context {
	return &templatepkg.Context{
		MQTT: templatepkg.MQTTContext{
			Topic:        topic,
			ClientID:     clientID,
			Payload:      payload,
			WildcardVals: wildcardVals,
		},
	}
}

// NewDeviceTemplateContext builds a template.Context for device simulation,
// which uses DeviceID and optionally MessageNum.
func NewDeviceTemplateContext(deviceID, topic string) *templatepkg.Context {
	return &templatepkg.Context{
		MQTT: templatepkg.MQTTContext{
			DeviceID: deviceID,
			Topic:    topic,
		},
	}
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
