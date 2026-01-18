// Package help provides embedded documentation for mockd CLI help topics.
package help

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed topics/*.txt
var Topics embed.FS

// AvailableTopics lists all available help topics.
var AvailableTopics = []string{"templating", "matching", "config", "formats", "websocket", "graphql", "grpc", "mqtt", "soap", "sse"}

// TopicDescriptions provides short descriptions for each topic.
var TopicDescriptions = map[string]string{
	"templating": "Template variable reference",
	"matching":   "Request matching patterns",
	"config":     "Configuration file format",
	"formats":    "Import/export formats",
	"websocket":  "WebSocket mock configuration",
	"graphql":    "GraphQL mock configuration",
	"grpc":       "gRPC mock configuration",
	"mqtt":       "MQTT broker configuration",
	"soap":       "SOAP mock configuration",
	"sse":        "Server-Sent Events configuration",
}

// GetTopic retrieves the content of a help topic by name.
func GetTopic(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))

	// Check if topic exists
	found := false
	for _, t := range AvailableTopics {
		if t == name {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("unknown help topic: %s\n\nAvailable topics:\n%s", name, ListTopics())
	}

	// Read from embedded FS
	content, err := Topics.ReadFile("topics/" + name + ".txt")
	if err != nil {
		return "", fmt.Errorf("failed to read topic %s: %w", name, err)
	}

	return string(content), nil
}

// ListTopics returns a formatted list of available topics.
func ListTopics() string {
	var sb strings.Builder
	for _, topic := range AvailableTopics {
		desc := TopicDescriptions[topic]
		sb.WriteString(fmt.Sprintf("  %-15s %s\n", topic, desc))
	}
	return sb.String()
}
