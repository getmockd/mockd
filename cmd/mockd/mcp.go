package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/mcp"
	"github.com/getmockd/mockd/pkg/stateful"
)

func init() {
	// Wire MCP factories into pkg/cli to break circular imports.
	cli.MCPRunStdioFunc = runMCPStdio
	cli.MCPStartFunc = startMCPHTTP
}

// runMCPStdio runs the MCP server in stdio mode.
//
// Connection strategy (in order):
//  1. If --admin-url is given, connect to that specific server.
//  2. Otherwise, check the PID file / default admin URL for a running server.
//  3. If nothing is running, auto-start a background daemon (mockd start -d)
//     so the MCP session works immediately with zero setup.
//
// The auto-started server is a shared daemon — it survives the MCP session
// so multiple AI assistants can connect simultaneously and mocks persist.
// Stop it with `mockd stop`.
//
// Use --data-dir for project-scoped isolation. This starts a separate daemon
// on different ports with a project-local PID file and data directory.
// Multiple sessions in the same project share the same daemon.
func runMCPStdio(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)

	// Connection flags.
	var adminURL string
	var logLevel string
	var dataDir string
	var configFile string
	var port int
	var adminPort int
	fs.StringVar(&adminURL, "admin-url", "", "Connect to a specific admin API URL (skips auto-start)")
	fs.StringVar(&logLevel, "log-level", "warn", "Log level for stderr (debug, info, warn, error)")
	fs.StringVar(&dataDir, "data-dir", "", "Project-scoped data directory (starts separate daemon)")
	fs.StringVar(&configFile, "config", "", "Config file to load on daemon startup")
	fs.IntVar(&port, "port", 0, "Mock server port for project daemon (default: 4280, or 14280 with --data-dir)")
	fs.IntVar(&adminPort, "admin-port", 0, "Admin API port for project daemon (default: 4290, or 14290 with --data-dir)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd mcp [flags]

Start the MCP (Model Context Protocol) server in stdio mode.
Reads JSON-RPC from stdin, writes responses to stdout.

Connection behavior:
  By default, connects to a running mockd server. If no server is running,
  auto-starts one in the background as a shared daemon. The daemon survives
  the MCP session so multiple AI assistants can connect simultaneously and
  mocks persist across sessions. Stop it with 'mockd stop'.

  Use --data-dir for project-scoped isolation. This starts a separate daemon
  with its own ports and data directory. Multiple sessions in the same
  project share the same project daemon.

Claude Desktop config (~/.config/claude/claude_desktop_config.json):

  {
    "mcpServers": {
      "mockd": {
        "command": "mockd",
        "args": ["mcp"]
      }
    }
  }

Project-scoped (separate daemon per project):

  {
    "mcpServers": {
      "mockd": {
        "command": "mockd",
        "args": ["mcp", "--data-dir", "./mockd-data"]
      }
    }
  }

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Set up logger (stderr only — stdout is the MCP protocol channel).
	level := parseSlogLevel(logLevel)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Resolve context for later.
	contextName := cliconfig.ResolveContext("")
	workspace := cliconfig.ResolveWorkspaceWithContext("", "")

	// --- Determine which server to connect to ---

	if adminURL != "" {
		// Explicit URL: connect directly, no auto-start.
		adminClient := cli.NewAdminClientWithAuth(adminURL)
		if err := adminClient.Health(); err != nil {
			log.Warn("mockd server unreachable at explicit URL — tools will fail",
				"adminUrl", adminURL, "error", err.Error())
		} else {
			log.Info("connected to mockd server", "adminUrl", adminURL, "context", contextName)
		}
		return runMCPSession(log, adminClient, contextName, adminURL, workspace)
	}

	// No explicit URL: either global daemon or project-scoped daemon.
	var resolvedURL string
	if dataDir != "" {
		// Project-scoped daemon.
		url, err := ensureProjectDaemon(log, dataDir, configFile, port, adminPort)
		if err != nil {
			log.Warn("failed to start project daemon — tools will fail",
				"dataDir", dataDir, "error", err.Error(),
				"hint", fmt.Sprintf("run 'mockd start --data-dir %s' manually", dataDir))
		}
		resolvedURL = url
		contextName = "project"
	} else {
		// Global daemon.
		url, err := ensureGlobalDaemon(log)
		if err != nil {
			log.Warn("failed to start mockd — tools will fail",
				"error", err.Error(),
				"hint", "run 'mockd start' to start the server")
		}
		resolvedURL = url
	}

	if resolvedURL == "" {
		// Fall back to default URL so MCP session can at least initialize
		// (tools will fail with connection errors, but the AI can see the problem).
		resolvedURL = "http://localhost:4290"
	}

	adminClient := cli.NewAdminClientWithAuth(resolvedURL)
	return runMCPSession(log, adminClient, contextName, resolvedURL, workspace)
}

// ensureGlobalDaemon ensures the global mockd daemon is running.
// Returns the admin URL.
func ensureGlobalDaemon(log *slog.Logger) (string, error) {
	adminURL := cliconfig.ResolveAdminURLWithContext("", "")

	// Try connecting to existing server.
	client := cli.NewAdminClientWithAuth(adminURL)
	if err := client.Health(); err == nil {
		log.Info("connected to mockd server", "adminUrl", adminURL)
		return adminURL, nil
	}

	// Check PID file — maybe server is starting or on a different port.
	pidPath := cli.DefaultPIDPath()
	if pidInfo, err := cli.ReadPIDFile(pidPath); err == nil && pidInfo.IsRunning() {
		url := pidInfo.AdminURL()
		if url != "" {
			log.Info("found running daemon via PID file", "pid", pidInfo.PID, "adminUrl", url)
			return url, nil
		}
	}

	// No server found — start one.
	log.Info("no mockd server detected, auto-starting background daemon")
	return startDaemon(log, pidPath, nil)
}

// ensureProjectDaemon ensures a project-scoped daemon is running.
// Uses a PID file inside the data directory and different default ports.
func ensureProjectDaemon(log *slog.Logger, dataDir, configFile string, port, adminPort int) (string, error) {
	// Resolve absolute path for the data dir.
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		absDataDir = dataDir
	}

	// Project PID file lives inside the data directory.
	pidPath := filepath.Join(absDataDir, "mockd.pid")

	// Default ports for project daemons differ from global to avoid conflicts.
	if port == 0 {
		port = 14280
	}
	if adminPort == 0 {
		adminPort = 14290
	}

	// Check if a project daemon is already running.
	if pidInfo, err := cli.ReadPIDFile(pidPath); err == nil && pidInfo.IsRunning() {
		url := pidInfo.AdminURL()
		if url != "" {
			// Verify it's actually healthy.
			healthClient := &http.Client{Timeout: 2 * time.Second}
			if resp, err := healthClient.Get(url + "/health"); err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Info("connected to project daemon", "adminUrl", url, "dataDir", absDataDir)
					return url, nil
				}
			}
		}
	}

	// Start a project-scoped daemon.
	log.Info("starting project-scoped daemon", "dataDir", absDataDir, "port", port, "adminPort", adminPort)

	extraArgs := []string{
		"--data-dir", absDataDir,
		"--port", fmt.Sprintf("%d", port),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--pid-file", pidPath,
	}
	if configFile != "" {
		absConfig, err := filepath.Abs(configFile)
		if err == nil {
			configFile = absConfig
		}
		extraArgs = append(extraArgs, "--config", configFile)
	}

	return startDaemon(log, pidPath, extraArgs)
}

// startDaemon starts `mockd start --detach --no-auth` with optional extra args.
// Returns the admin URL once the daemon is healthy.
func startDaemon(log *slog.Logger, pidPath string, extraArgs []string) (string, error) {
	binary, err := os.Executable()
	if err != nil {
		binary = "mockd"
	}

	cmdArgs := []string{"start", "--detach", "--no-auth"}
	cmdArgs = append(cmdArgs, extraArgs...)

	cmd := exec.Command(binary, cmdArgs...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stderr // Daemon output goes to MCP stderr for debugging.
	cmd.Stderr = os.Stderr

	log.Info("starting mockd daemon", "cmd", cmd.String())
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mockd start --detach failed: %w", err)
	}

	// Wait for PID file and read admin URL.
	var adminURL string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		pidInfo, err := cli.ReadPIDFile(pidPath)
		if err == nil && pidInfo.IsRunning() {
			adminURL = pidInfo.AdminURL()
			if adminURL != "" {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	if adminURL == "" {
		return "", fmt.Errorf("daemon started but admin URL not found in PID file at %s", pidPath)
	}

	// Wait for admin health.
	healthClient := &http.Client{Timeout: 2 * time.Second}
	healthDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(healthDeadline) {
		resp, err := healthClient.Get(adminURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return adminURL, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Return URL even if health didn't pass yet — it might be slow to start.
	return adminURL, nil
}

// runMCPSession creates the MCP server and runs the stdio transport.
func runMCPSession(log *slog.Logger, adminClient cli.AdminClient, contextName, adminURL, workspace string) error {
	mcpCfg := mcp.DefaultConfig()
	mcpCfg.Enabled = true
	mcpCfg.AdminURL = adminURL

	mcp.SetServerVersion(cli.Version)
	server := mcp.NewServer(mcpCfg, adminClient, nil)

	server.SetInitialContext(contextName, adminURL, workspace)
	server.SetClientFactory(func(url string) cli.AdminClient {
		return cli.NewAdminClientWithAuth(url)
	})
	server.SetLogger(log)

	stdio := mcp.NewStdioServer(server)
	stdio.SetLogger(log)
	return stdio.Run()
}

// parseSlogLevel converts a string log level to slog.Level.
func parseSlogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// startMCPHTTP creates and starts the MCP HTTP server for embedding in mockd serve.
func startMCPHTTP(adminURL string, port int, allowRemote bool, storeIface interface{}, logIface interface{}) (cli.MCPStopper, error) {
	mcpCfg := mcp.DefaultConfig()
	mcpCfg.Enabled = true
	mcpCfg.Port = port
	mcpCfg.AllowRemote = allowRemote
	mcpCfg.AdminURL = adminURL

	adminClient := cli.NewAdminClient(adminURL)

	// Type-assert the stateful store (passed as interface{} to avoid circular imports).
	var s *stateful.StateStore
	if st, ok := storeIface.(*stateful.StateStore); ok {
		s = st
	}

	mcp.SetServerVersion(cli.Version)
	server := mcp.NewServer(mcpCfg, adminClient, s)

	// Seed sessions with the resolved context.
	contextName := cliconfig.ResolveContext("")
	workspace := cliconfig.ResolveWorkspaceWithContext("", "")
	server.SetInitialContext(contextName, adminURL, workspace)

	server.SetClientFactory(func(url string) cli.AdminClient {
		return cli.NewAdminClientWithAuth(url)
	})

	if l, ok := logIface.(*slog.Logger); ok && l != nil {
		server.SetLogger(l)
	}

	if err := server.Start(); err != nil {
		return nil, err
	}
	return server, nil
}
