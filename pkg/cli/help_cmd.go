package cli

import (
	"fmt"

	"github.com/getmockd/mockd/pkg/cli/help"
	"github.com/spf13/cobra"
)

// helpTopicCmd handles 'mockd help [topic]' for topic-based help.
// This overrides Cobra's built-in help command to add topic support.
var helpTopicCmd = &cobra.Command{
	Use:   "help [topic]",
	Short: "Show help for topics or commands",
	Long: `Show detailed help for mockd topics like configuration, matching, templating, etc.

Available topics: config, matching, templating, formats, websocket, graphql, grpc, mqtt, soap, sse

Examples:
  mockd help templating    Show help on response templating
  mockd help matching      Show help on request matching rules
  mockd help graphql       Show help on GraphQL mocking`,
	ValidArgs: []string{"config", "matching", "templating", "formats", "websocket", "graphql", "grpc", "mqtt", "soap", "sse"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// No topic specified, show available topics
			fmt.Print("mockd - Help Topics\n\nAvailable Topics:\n")
			fmt.Print(help.ListTopics())
			fmt.Println()
			fmt.Println("Usage: mockd help <topic>")
			fmt.Println()
			fmt.Println("Example: mockd help templating")
			return nil
		}

		topic := args[0]

		// Check if the topic is a registered command first
		for _, child := range rootCmd.Commands() {
			if child.Name() == topic || child.HasAlias(topic) {
				return child.Help()
			}
		}

		// Otherwise, check for help topics
		content, err := help.GetTopic(topic)
		if err != nil {
			return err
		}

		fmt.Println(content)
		return nil
	},
}

func init() {
	rootCmd.SetHelpCommand(helpTopicCmd)
}
