// Package templates provides embedded starter templates for mockd init.
package templates

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed *.yaml
var templateFS embed.FS

// Template represents a starter template.
type Template struct {
	ID          string
	Name        string
	Description string
	Filename    string
}

// AvailableTemplates returns all available starter templates.
var AvailableTemplates = []Template{
	{
		ID:          "default",
		Name:        "Default",
		Description: "Basic HTTP mocks (hello, echo, health)",
		Filename:    "default.yaml",
	},
	{
		ID:          "crud",
		Name:        "CRUD API",
		Description: "Full REST CRUD API for resources",
		Filename:    "crud.yaml",
	},
	{
		ID:          "websocket-chat",
		Name:        "WebSocket Chat",
		Description: "Chat room WebSocket endpoint with echo",
		Filename:    "websocket-chat.yaml",
	},
	{
		ID:          "graphql-api",
		Name:        "GraphQL API",
		Description: "GraphQL API with User CRUD resolvers",
		Filename:    "graphql-api.yaml",
	},
	{
		ID:          "grpc-service",
		Name:        "gRPC Service",
		Description: "gRPC Greeter service with reflection",
		Filename:    "grpc-service.yaml",
	},
	{
		ID:          "mqtt-iot",
		Name:        "MQTT IoT",
		Description: "MQTT broker with IoT sensor topics",
		Filename:    "mqtt-iot.yaml",
	},
}

// Get returns the template content by ID.
func Get(id string) ([]byte, error) {
	for _, t := range AvailableTemplates {
		if strings.EqualFold(t.ID, id) {
			return templateFS.ReadFile(t.Filename)
		}
	}
	return nil, fmt.Errorf("unknown template: %s", id)
}

// GetTemplate returns the Template metadata by ID.
func GetTemplate(id string) (*Template, error) {
	for i := range AvailableTemplates {
		if strings.EqualFold(AvailableTemplates[i].ID, id) {
			return &AvailableTemplates[i], nil
		}
	}
	return nil, fmt.Errorf("unknown template: %s", id)
}

// List returns all template IDs sorted alphabetically.
func List() []string {
	ids := make([]string, len(AvailableTemplates))
	for i, t := range AvailableTemplates {
		ids[i] = t.ID
	}
	sort.Strings(ids)
	return ids
}

// FormatList returns a formatted string listing all available templates.
func FormatList() string {
	var sb strings.Builder
	sb.WriteString("Available templates:\n\n")

	// Find max ID length for alignment
	maxLen := 0
	for _, t := range AvailableTemplates {
		if len(t.ID) > maxLen {
			maxLen = len(t.ID)
		}
	}

	for _, t := range AvailableTemplates {
		sb.WriteString(fmt.Sprintf("  %-*s  %s\n", maxLen, t.ID, t.Description))
	}

	sb.WriteString("\nUsage:\n")
	sb.WriteString("  mockd init --template <name>\n")
	sb.WriteString("  mockd init -t crud -o api.yaml\n")

	return sb.String()
}

// Exists checks if a template ID exists.
func Exists(id string) bool {
	for _, t := range AvailableTemplates {
		if strings.EqualFold(t.ID, id) {
			return true
		}
	}
	return false
}
