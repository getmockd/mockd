package cli

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
)

// contextForJSON is a sanitized version of Context for JSON output.
// It masks sensitive fields like AuthToken to prevent accidental exposure.
type contextForJSON struct {
	AdminURL    string `json:"adminUrl"`
	Workspace   string `json:"workspace,omitempty"`
	Description string `json:"description,omitempty"`
	HasToken    bool   `json:"hasToken,omitempty"`
	TLSInsecure bool   `json:"tlsInsecure,omitempty"`
}

// sanitizeContextForJSON converts a Context to a safe-for-output version.
func sanitizeContextForJSON(ctx *cliconfig.Context) *contextForJSON {
	return &contextForJSON{
		AdminURL:    ctx.AdminURL,
		Workspace:   ctx.Workspace,
		Description: ctx.Description,
		HasToken:    ctx.AuthToken != "",
		TLSInsecure: ctx.TLSInsecure,
	}
}

// sanitizeContextsForJSON converts a map of Contexts to safe-for-output versions.
func sanitizeContextsForJSON(contexts map[string]*cliconfig.Context) map[string]*contextForJSON {
	result := make(map[string]*contextForJSON, len(contexts))
	for name, ctx := range contexts {
		result[name] = sanitizeContextForJSON(ctx)
	}
	return result
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage contexts (admin server + workspace pairs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runContextShow()
	},
}

var contextShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current context",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runContextShow()
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.AddCommand(contextShowCmd)
}

// runContextShow displays the current context.
func runContextShow() error {
	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return fmt.Errorf("failed to load context config: %w", err)
	}

	// Check for env var override
	envContext := cliconfig.GetContextFromEnv()
	effectiveContext := cfg.CurrentContext
	envOverride := false

	if envContext != "" {
		effectiveContext = envContext
		envOverride = true
	}

	ctx := cfg.Contexts[effectiveContext]
	if ctx == nil {
		if envOverride {
			return fmt.Errorf("context %q (from MOCKD_CONTEXT) not found", envContext)
		}
		fmt.Println("No current context set")
		fmt.Println("\nRun 'mockd context add <name>' to create a context")
		return nil
	}

	fmt.Printf("Current context: %s", effectiveContext)
	if envOverride {
		fmt.Print("  (from MOCKD_CONTEXT)")
	}
	fmt.Println()

	fmt.Printf("  Admin URL:  %s\n", ctx.AdminURL)

	// Check for workspace env override
	envWorkspace := cliconfig.GetWorkspaceFromEnv()
	if envWorkspace != "" {
		fmt.Printf("  Workspace:  %s  (from MOCKD_WORKSPACE)\n", envWorkspace)
	} else if ctx.Workspace != "" {
		fmt.Printf("  Workspace:  %s\n", ctx.Workspace)
	}

	if ctx.Description != "" {
		fmt.Printf("  Description: %s\n", ctx.Description)
	}

	fmt.Println("\nRun 'mockd context list' to see all contexts")
	return nil
}

var contextUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a different context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := cliconfig.LoadContextConfig()
		if err != nil {
			return fmt.Errorf("failed to load context config: %w", err)
		}

		if err := cfg.SetCurrentContext(name); err != nil {
			// List available contexts in error message
			var available []string
			for n := range cfg.Contexts {
				available = append(available, n)
			}
			sort.Strings(available)
			return fmt.Errorf("%w\n\nAvailable contexts: %s", err, strings.Join(available, ", "))
		}

		if err := cliconfig.SaveContextConfig(cfg); err != nil {
			return fmt.Errorf("failed to save context config: %w", err)
		}

		ctx := cfg.Contexts[name]
		fmt.Printf("Switched to context %q\n", name)
		fmt.Printf("  Admin URL: %s\n", ctx.AdminURL)
		if ctx.Workspace != "" {
			fmt.Printf("  Workspace: %s\n", ctx.Workspace)
		}

		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextUseCmd)
}

var (
	contextAddAdminURL    string
	contextAddWorkspace   string
	contextAddDescription string
	contextAddToken       string
	contextAddTLSInsecure bool
	contextAddUseCurrent  bool
)

var contextAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new context",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		}

		// Validate name
		if name == "" {
			return errors.New("context name cannot be empty")
		}
		if len(name) > 64 {
			return errors.New("context name cannot exceed 64 characters")
		}
		if strings.ContainsAny(name, " \t\n/\\") {
			return errors.New("context name cannot contain whitespace or path separators")
		}

		// If admin URL not provided, prompt interactively
		if contextAddAdminURL == "" {
			fmt.Printf("Adding context %q\n", name)
			fmt.Print("Admin URL (e.g., http://localhost:4290): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			contextAddAdminURL = strings.TrimSpace(input)
			if contextAddAdminURL == "" {
				return errors.New("admin URL is required")
			}
		}

		// Validate URL
		parsedURL, err := url.Parse(contextAddAdminURL)
		if err != nil {
			return fmt.Errorf("invalid admin URL: %w", err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return errors.New("invalid admin URL: must start with http:// or https://")
		}
		if parsedURL.Host == "" {
			return errors.New("invalid admin URL: missing host")
		}
		// Reject URLs with embedded credentials (user:pass@host)
		if parsedURL.User != nil {
			return errors.New("invalid admin URL: embedded credentials (user:pass@host) are not allowed; use --token for authentication")
		}

		cfg, err := cliconfig.LoadContextConfig()
		if err != nil {
			return fmt.Errorf("failed to load context config: %w", err)
		}

		ctx := &cliconfig.Context{
			AdminURL:    contextAddAdminURL,
			Workspace:   contextAddWorkspace,
			Description: contextAddDescription,
			AuthToken:   contextAddToken,
			TLSInsecure: contextAddTLSInsecure,
		}

		if err := cfg.AddContext(name, ctx); err != nil {
			return err
		}

		if contextAddUseCurrent {
			cfg.CurrentContext = name
		}

		if err := cliconfig.SaveContextConfig(cfg); err != nil {
			return fmt.Errorf("failed to save context config: %w", err)
		}

		if jsonOutput {
			result := struct {
				Name    string          `json:"name"`
				Context *contextForJSON `json:"context"`
				Current bool            `json:"current"`
			}{
				Name:    name,
				Context: sanitizeContextForJSON(ctx),
				Current: cfg.CurrentContext == name,
			}
			return output.JSON(result)
		}

		fmt.Printf("Added context %q\n", name)
		if contextAddUseCurrent {
			fmt.Printf("Switched to context %q\n", name)
		}

		return nil
	},
}

func init() {
	contextAddCmd.Flags().StringVarP(&contextAddAdminURL, "admin-url", "u", "", "Admin API URL (e.g., http://localhost:4290)")
	contextAddCmd.Flags().StringVarP(&contextAddWorkspace, "workspace", "w", "", "Default workspace for this context")
	contextAddCmd.Flags().StringVarP(&contextAddDescription, "description", "d", "", "Description for this context")
	contextAddCmd.Flags().StringVarP(&contextAddToken, "token", "t", "", "Auth token for cloud/enterprise deployments")
	contextAddCmd.Flags().BoolVar(&contextAddTLSInsecure, "tls-insecure", false, "Skip TLS certificate verification")
	contextAddCmd.Flags().BoolVar(&contextAddUseCurrent, "use", false, "Switch to this context after adding")
	contextCmd.AddCommand(contextAddCmd)
}

var contextListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cliconfig.LoadContextConfig()
		if err != nil {
			return fmt.Errorf("failed to load context config: %w", err)
		}

		if jsonOutput {
			result := struct {
				CurrentContext string                     `json:"currentContext"`
				Contexts       map[string]*contextForJSON `json:"contexts"`
			}{
				CurrentContext: cfg.CurrentContext,
				Contexts:       sanitizeContextsForJSON(cfg.Contexts),
			}
			return output.JSON(result)
		}

		if len(cfg.Contexts) == 0 {
			fmt.Println("No contexts configured")
			fmt.Println("\nRun 'mockd context add <name>' to create a context")
			return nil
		}

		// Sort context names for consistent output
		names := make([]string, 0, len(cfg.Contexts))
		for name := range cfg.Contexts {
			names = append(names, name)
		}
		sort.Strings(names)

		w := output.Table()
		_, _ = fmt.Fprintln(w, "CURRENT\tNAME\tADMIN URL\tWORKSPACE\tDESCRIPTION")

		for _, name := range names {
			ctx := cfg.Contexts[name]
			current := ""
			if name == cfg.CurrentContext {
				current = "*"
			}

			workspace := ctx.Workspace
			if workspace == "" {
				workspace = "-"
			}

			description := ctx.Description
			if len(description) > 30 {
				description = description[:27] + "..."
			}
			if description == "" {
				description = "-"
			}

			adminURL := ctx.AdminURL
			if len(adminURL) > 35 {
				adminURL = adminURL[:32] + "..."
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", current, name, adminURL, workspace, description)
		}

		return w.Flush()
	},
}

func init() {
	contextCmd.AddCommand(contextListCmd)
}

var contextRemoveForce bool

var contextRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a context",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := cliconfig.LoadContextConfig()
		if err != nil {
			return fmt.Errorf("failed to load context config: %w", err)
		}

		// Check if context exists
		ctx, exists := cfg.Contexts[name]
		if !exists {
			return fmt.Errorf("context not found: %s", name)
		}

		// Confirm unless forced
		if !contextRemoveForce {
			fmt.Printf("Remove context %q?\n", name)
			fmt.Printf("  Admin URL: %s\n", ctx.AdminURL)
			if ctx.Workspace != "" {
				fmt.Printf("  Workspace: %s\n", ctx.Workspace)
			}
			fmt.Print("Type 'yes' to confirm: ")

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			if strings.TrimSpace(input) != "yes" {
				fmt.Println("Aborted")
				return nil
			}
		}

		if err := cfg.RemoveContext(name); err != nil {
			return err
		}

		if err := cliconfig.SaveContextConfig(cfg); err != nil {
			return fmt.Errorf("failed to save context config: %w", err)
		}

		fmt.Printf("Removed context %q\n", name)
		return nil
	},
}

func init() {
	contextRemoveCmd.Flags().BoolVarP(&contextRemoveForce, "force", "f", false, "Force removal without confirmation")
	contextCmd.AddCommand(contextRemoveCmd)
}
