package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for mockd.

To load completions:

Bash:
  # Add to ~/.bashrc or /etc/bash_completion.d/:
  mockd completion bash > /etc/bash_completion.d/mockd
  # Or for user install:
  mockd completion bash >> ~/.bashrc

Zsh:
  # Add to fpath:
  mockd completion zsh > "${fpath[1]}/_mockd"
  # Or for Oh My Zsh:
  mockd completion zsh > ~/.oh-my-zsh/completions/_mockd
  # You may need to run: compinit

Fish:
  mockd completion fish > ~/.config/fish/completions/mockd.fish
`,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return fmt.Errorf("unknown shell: %s\n\nSupported shells: bash, zsh, fish", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
