package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/getmockd/mockd/pkg/cliconfig"
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

// RunContext handles the context command and its subcommands.
func RunContext(args []string) error {
	if len(args) == 0 {
		// No subcommand: show current context
		return runContextShow()
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "use":
		return runContextUse(subArgs)
	case "add":
		return runContextAdd(subArgs)
	case "list", "ls":
		return runContextList(subArgs)
	case "remove", "rm", "delete":
		return runContextRemove(subArgs)
	case "show":
		return runContextShow()
	case "--help", "-h", "help":
		printContextUsage()
		return nil
	default:
		return fmt.Errorf("unknown context subcommand: %s\n\nRun 'mockd context --help' for usage", subcommand)
	}
}

func printContextUsage() {
	fmt.Print(`Usage: mockd context [command]

Manage contexts (admin server + workspace pairs).

Commands:
  (no command)  Show current context
  show          Show current context (same as no command)
  use <name>    Switch to a different context
  add <name>    Add a new context
  list          List all contexts
  remove <name> Remove a context

Examples:
  # Show current context
  mockd context

  # Switch to a different context
  mockd context use staging

  # Add a new context (flags before name)
  mockd context add -u https://staging.example.com:4290 staging

  # Add context interactively
  mockd context add production

  # List all contexts
  mockd context list

  # Remove a context
  mockd context remove old-server

Configuration:
  Contexts are stored in ~/.config/mockd/contexts.json
`)
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

// runContextUse switches to a different context.
func runContextUse(args []string) error {
	fs := flag.NewFlagSet("context use", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd context use <name>

Switch to a different context.

Examples:
  mockd context use staging
  mockd context use local
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("context name required")
	}

	name := fs.Arg(0)

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
}

// runContextAdd adds a new context.
func runContextAdd(args []string) error {
	fs := flag.NewFlagSet("context add", flag.ContinueOnError)

	adminURL := fs.String("admin-url", "", "Admin API URL (e.g., http://localhost:4290)")
	fs.StringVar(adminURL, "u", "", "Admin API URL (shorthand)")
	workspace := fs.String("workspace", "", "Default workspace for this context")
	fs.StringVar(workspace, "w", "", "Default workspace (shorthand)")
	description := fs.String("description", "", "Description for this context")
	fs.StringVar(description, "d", "", "Description (shorthand)")
	authToken := fs.String("token", "", "Auth token for cloud/enterprise deployments")
	fs.StringVar(authToken, "t", "", "Auth token (shorthand)")
	tlsInsecure := fs.Bool("tls-insecure", false, "Skip TLS certificate verification")
	useCurrent := fs.Bool("use", false, "Switch to this context after adding")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd context add <name> [flags]

Add a new context.

Flags:
  -u, --admin-url    Admin API URL (e.g., http://localhost:4290)
  -w, --workspace    Default workspace for this context
  -d, --description  Description for this context
  -t, --token        Auth token for cloud/enterprise deployments
      --tls-insecure Skip TLS certificate verification (for self-signed certs)
      --use          Switch to this context after adding
      --json         Output in JSON format

Examples:
  # Add with flags (flags must come before name)
  mockd context add -u https://staging.example.com:4290 staging

  # Add interactively (will prompt for URL)
  mockd context add production

  # Add and switch to it
  mockd context add -u http://dev-server:4290 --use dev

  # Add with auth token
  mockd context add -u https://api.mockd.io -t YOUR_TOKEN --use cloud
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("context name required")
	}

	name := fs.Arg(0)

	// Validate name
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("context name cannot exceed 64 characters")
	}
	if strings.ContainsAny(name, " \t\n/\\") {
		return fmt.Errorf("context name cannot contain whitespace or path separators")
	}

	// If admin URL not provided, prompt interactively
	if *adminURL == "" {
		fmt.Printf("Adding context %q\n", name)
		fmt.Print("Admin URL (e.g., http://localhost:4290): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		*adminURL = strings.TrimSpace(input)
		if *adminURL == "" {
			return fmt.Errorf("admin URL is required")
		}
	}

	// Validate URL
	parsedURL, err := url.Parse(*adminURL)
	if err != nil {
		return fmt.Errorf("invalid admin URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid admin URL: must start with http:// or https://")
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid admin URL: missing host")
	}
	// Reject URLs with embedded credentials (user:pass@host)
	if parsedURL.User != nil {
		return fmt.Errorf("invalid admin URL: embedded credentials (user:pass@host) are not allowed; use --token for authentication")
	}

	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return fmt.Errorf("failed to load context config: %w", err)
	}

	ctx := &cliconfig.Context{
		AdminURL:    *adminURL,
		Workspace:   *workspace,
		Description: *description,
		AuthToken:   *authToken,
		TLSInsecure: *tlsInsecure,
	}

	if err := cfg.AddContext(name, ctx); err != nil {
		return err
	}

	if *useCurrent {
		cfg.CurrentContext = name
	}

	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		return fmt.Errorf("failed to save context config: %w", err)
	}

	if *jsonOutput {
		output := struct {
			Name    string          `json:"name"`
			Context *contextForJSON `json:"context"`
			Current bool            `json:"current"`
		}{
			Name:    name,
			Context: sanitizeContextForJSON(ctx),
			Current: cfg.CurrentContext == name,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Printf("Added context %q\n", name)
	if *useCurrent {
		fmt.Printf("Switched to context %q\n", name)
	}

	return nil
}

// runContextList lists all contexts.
func runContextList(args []string) error {
	fs := flag.NewFlagSet("context list", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd context list [flags]

List all contexts.

Flags:
      --json  Output in JSON format

Examples:
  mockd context list
  mockd context list --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return fmt.Errorf("failed to load context config: %w", err)
	}

	if *jsonOutput {
		output := struct {
			CurrentContext string                     `json:"currentContext"`
			Contexts       map[string]*contextForJSON `json:"contexts"`
		}{
			CurrentContext: cfg.CurrentContext,
			Contexts:       sanitizeContextsForJSON(cfg.Contexts),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
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
}

// runContextRemove removes a context.
func runContextRemove(args []string) error {
	fs := flag.NewFlagSet("context remove", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force removal without confirmation")
	fs.BoolVar(force, "f", false, "Force removal (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd context remove <name> [flags]

Remove a context.

Flags:
  -f, --force  Force removal without confirmation

Examples:
  mockd context remove old-server
  mockd context remove old-server --force
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("context name required")
	}

	name := fs.Arg(0)

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
	if !*force {
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
}
