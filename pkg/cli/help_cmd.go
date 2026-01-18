package cli

import (
	"fmt"

	"github.com/getmockd/mockd/pkg/cli/help"
)

// RunHelp handles the 'mockd help [topic]' command.
func RunHelp(args []string) error {
	if len(args) == 0 {
		// No topic specified, show available topics
		fmt.Print(`mockd - Help Topics

Available Topics:
`)
		fmt.Print(help.ListTopics())
		fmt.Println()
		fmt.Println("Usage: mockd help <topic>")
		fmt.Println()
		fmt.Println("Example: mockd help templating")
		return nil
	}

	// Get the specified topic
	topic := args[0]
	content, err := help.GetTopic(topic)
	if err != nil {
		return err
	}

	fmt.Println(content)
	return nil
}
