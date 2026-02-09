package cli

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/proxy"
	"github.com/getmockd/mockd/pkg/recording"
)

// proxyServer holds the global proxy state for CLI commands
var proxyServer struct {
	proxy    *proxy.Proxy
	store    *recording.Store
	ca       *proxy.CAManager
	server   *http.Server
	listener net.Listener
	running  bool
}

// RunProxy handles the proxy command and its subcommands.
func RunProxy(args []string) error {
	if len(args) == 0 {
		printProxyUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "start":
		return runProxyStart(subArgs)
	case "stop":
		return runProxyStop(subArgs)
	case "status":
		return runProxyStatus(subArgs)
	case "mode":
		return runProxyMode(subArgs)
	case "ca":
		return runProxyCA(subArgs)
	case "help", "--help", "-h":
		printProxyUsage()
		return nil
	default:
		return fmt.Errorf("unknown proxy subcommand: %s\n\nRun 'mockd proxy --help' for usage", subcommand)
	}
}

func printProxyUsage() {
	fmt.Print(`Usage: mockd proxy <subcommand> [flags]

Manage the MITM proxy for recording API traffic.

Subcommands:
  start     Start the proxy server
  stop      Stop the proxy server
  status    Show proxy server status
  mode      Get or set proxy mode (record/passthrough)
  ca        Manage CA certificate

Run 'mockd proxy <subcommand> --help' for more information.
`)
}

// runProxyStart starts the proxy server.
func runProxyStart(args []string) error {
	fs := flag.NewFlagSet("proxy start", flag.ContinueOnError)

	port := fs.Int("port", 8888, "Proxy server port")
	fs.IntVar(port, "p", 8888, "Proxy server port (shorthand)")

	mode := fs.String("mode", "record", "Proxy mode: record or passthrough")
	fs.StringVar(mode, "m", "record", "Proxy mode (shorthand)")

	session := fs.String("session", "", "Recording session name")
	fs.StringVar(session, "s", "", "Recording session name (shorthand)")

	caPath := fs.String("ca-path", "", "Path to CA certificate directory")

	includePaths := fs.String("include", "", "Comma-separated path patterns to include")
	excludePaths := fs.String("exclude", "", "Comma-separated path patterns to exclude")
	includeHosts := fs.String("include-hosts", "", "Comma-separated host patterns to include")
	excludeHosts := fs.String("exclude-hosts", "", "Comma-separated host patterns to exclude")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy start [flags]

Start the MITM proxy server for recording API traffic.

Flags:
  -p, --port          Proxy server port (default: 8888)
  -m, --mode          Proxy mode: record or passthrough (default: record)
  -s, --session       Recording session name
      --ca-path       Path to CA certificate directory
      --include       Comma-separated path patterns to include
      --exclude       Comma-separated path patterns to exclude
      --include-hosts Comma-separated host patterns to include
      --exclude-hosts Comma-separated host patterns to exclude

Examples:
  # Start proxy in record mode
  mockd proxy start

  # Start with custom port and session
  mockd proxy start --port 9000 --session my-session

  # Start with filters
  mockd proxy start --include "/api/*" --exclude "/api/health"
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check port availability
	if err := ports.Check(*port); err != nil {
		return formatPortError(*port, err)
	}

	// Parse mode
	var proxyMode proxy.Mode
	switch *mode {
	case "record":
		proxyMode = proxy.ModeRecord
	case "passthrough":
		proxyMode = proxy.ModePassthrough
	default:
		return fmt.Errorf("invalid mode: %s (must be 'record' or 'passthrough')", *mode)
	}

	// Create store and session
	store := recording.NewStore()
	sessionName := *session
	if sessionName == "" {
		sessionName = "default"
	}
	store.CreateSession(sessionName, nil)

	// Create CA manager
	var ca *proxy.CAManager
	if *caPath != "" {
		ca = proxy.NewCAManager(*caPath+"/ca.crt", *caPath+"/ca.key")
		if err := ca.EnsureCA(); err != nil {
			return fmt.Errorf("failed to initialize CA: %w", err)
		}
	}

	// Create filter config
	filter := proxy.NewFilterConfig()
	if *includePaths != "" {
		filter.IncludePaths = splitPatterns(*includePaths)
	}
	if *excludePaths != "" {
		filter.ExcludePaths = splitPatterns(*excludePaths)
	}
	if *includeHosts != "" {
		filter.IncludeHosts = splitPatterns(*includeHosts)
	}
	if *excludeHosts != "" {
		filter.ExcludeHosts = splitPatterns(*excludeHosts)
	}

	// Create proxy
	logger := log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	p := proxy.New(proxy.Options{
		Mode:      proxyMode,
		Store:     store,
		Filter:    filter,
		CAManager: ca,
		Logger:    logger,
	})

	// Start HTTP server
	addr := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Store state
	proxyServer.proxy = p
	proxyServer.store = store
	proxyServer.ca = ca
	proxyServer.server = server
	proxyServer.listener = listener
	proxyServer.running = true

	fmt.Printf("Proxy server running on http://localhost:%d\n", *port)
	fmt.Printf("Mode: %s\n", proxyMode)
	fmt.Printf("Session: %s\n", sessionName)
	if ca != nil {
		fmt.Printf("CA certificate: %s\n", ca.CertPath())
	}
	fmt.Println("Press Ctrl+C to stop")

	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down proxy...")
	if err := server.Close(); err != nil {
		output.Warn("server shutdown error: %v", err)
	}

	proxyServer.running = false
	fmt.Println("Proxy stopped")

	// Print recording summary
	recordings, total := store.ListRecordings(recording.RecordingFilter{})
	if total > 0 {
		fmt.Printf("\nCaptured %d recordings\n", total)
		for _, r := range recordings {
			fmt.Printf("  %s %s (%d)\n", r.Request.Method, r.Request.Path, r.Response.StatusCode)
		}
	}

	return nil
}

// runProxyStop stops the proxy server.
func runProxyStop(args []string) error {
	fs := flag.NewFlagSet("proxy stop", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy stop

Stop the running proxy server.
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !proxyServer.running {
		return fmt.Errorf("proxy is not running")
	}

	if proxyServer.server != nil {
		if err := proxyServer.server.Close(); err != nil {
			return fmt.Errorf("failed to stop proxy: %w", err)
		}
	}

	proxyServer.running = false
	fmt.Println("Proxy stopped")
	return nil
}

// runProxyStatus shows proxy status.
func runProxyStatus(args []string) error {
	fs := flag.NewFlagSet("proxy status", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy status

Show the current proxy server status.
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !proxyServer.running {
		fmt.Println("Proxy is not running")
		return nil
	}

	fmt.Println("Proxy is running")
	if proxyServer.proxy != nil {
		fmt.Printf("Mode: %s\n", proxyServer.proxy.Mode())
	}
	if proxyServer.store != nil {
		_, total := proxyServer.store.ListRecordings(recording.RecordingFilter{})
		fmt.Printf("Recordings: %d\n", total)
	}

	return nil
}

// runProxyMode gets or sets the proxy mode.
func runProxyMode(args []string) error {
	fs := flag.NewFlagSet("proxy mode", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy mode [mode]

Get or set the proxy operating mode.

Arguments:
  mode    New mode: record or passthrough (optional)

Examples:
  # Get current mode
  mockd proxy mode

  # Set mode to passthrough
  mockd proxy mode passthrough
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !proxyServer.running {
		return fmt.Errorf("proxy is not running")
	}

	// Get mode
	if fs.NArg() == 0 {
		fmt.Printf("Current mode: %s\n", proxyServer.proxy.Mode())
		return nil
	}

	// Set mode
	newMode := fs.Arg(0)
	switch newMode {
	case "record":
		proxyServer.proxy.SetMode(proxy.ModeRecord)
		fmt.Println("Mode set to: record")
	case "passthrough":
		proxyServer.proxy.SetMode(proxy.ModePassthrough)
		fmt.Println("Mode set to: passthrough")
	default:
		return fmt.Errorf("invalid mode: %s (must be 'record' or 'passthrough')", newMode)
	}

	return nil
}

// runProxyCA handles CA certificate commands.
func runProxyCA(args []string) error {
	if len(args) == 0 {
		printProxyCAUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "export":
		return runProxyCAExport(subArgs)
	case "generate":
		return runProxyCAGenerate(subArgs)
	case "help", "--help", "-h":
		printProxyCAUsage()
		return nil
	default:
		return fmt.Errorf("unknown ca subcommand: %s", subcommand)
	}
}

func printProxyCAUsage() {
	fmt.Print(`Usage: mockd proxy ca <subcommand> [flags]

Manage CA certificate for HTTPS interception.

Subcommands:
  export    Export CA certificate for trust installation
  generate  Generate a new CA certificate

Run 'mockd proxy ca <subcommand> --help' for more information.
`)
}

// runProxyCAExport exports the CA certificate.
func runProxyCAExport(args []string) error {
	fs := flag.NewFlagSet("proxy ca export", flag.ContinueOnError)

	output := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(output, "o", "", "Output file path (shorthand)")

	caPath := fs.String("ca-path", "", "Path to CA certificate directory")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy ca export [flags]

Export the CA certificate for trust installation.

Flags:
  -o, --output   Output file path (default: stdout)
      --ca-path  Path to CA certificate directory

Examples:
  # Export to stdout
  mockd proxy ca export

  # Export to file
  mockd proxy ca export -o ca.crt
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Use running proxy's CA if available
	var ca *proxy.CAManager
	if proxyServer.ca != nil {
		ca = proxyServer.ca
	} else if *caPath != "" {
		ca = proxy.NewCAManager(*caPath+"/ca.crt", *caPath+"/ca.key")
		if err := ca.Load(); err != nil {
			return fmt.Errorf("failed to load CA: %w", err)
		}
	} else {
		return fmt.Errorf("no CA available (start proxy with --ca-path or specify --ca-path)")
	}

	certPEM, err := ca.CACertPEM()
	if err != nil {
		return fmt.Errorf("failed to export CA certificate: %w", err)
	}

	if *output == "" {
		fmt.Print(string(certPEM))
	} else {
		if err := os.WriteFile(*output, certPEM, 0644); err != nil {
			return fmt.Errorf("failed to write certificate: %w", err)
		}
		fmt.Printf("CA certificate exported to: %s\n", *output)
	}

	return nil
}

// runProxyCAGenerate generates a new CA certificate.
func runProxyCAGenerate(args []string) error {
	fs := flag.NewFlagSet("proxy ca generate", flag.ContinueOnError)

	caPath := fs.String("ca-path", "", "Path to CA certificate directory")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy ca generate [flags]

Generate a new CA certificate for HTTPS interception.

Flags:
      --ca-path  Path to CA certificate directory (required)

Examples:
  mockd proxy ca generate --ca-path ~/.mockd/ca
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *caPath == "" {
		return fmt.Errorf("--ca-path is required")
	}

	ca := proxy.NewCAManager(*caPath+"/ca.crt", *caPath+"/ca.key")
	if err := ca.Generate(); err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	fmt.Printf("CA certificate generated:\n")
	fmt.Printf("  Certificate: %s\n", ca.CertPath())
	fmt.Printf("  Private key: %s\n", ca.KeyPath())
	fmt.Println("\nTo trust this CA on macOS:")
	fmt.Printf("  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s\n", ca.CertPath())
	fmt.Println("\nTo trust this CA on Linux (Ubuntu/Debian):")
	fmt.Printf("  sudo cp %s /usr/local/share/ca-certificates/mockd-ca.crt\n", ca.CertPath())
	fmt.Println("  sudo update-ca-certificates")

	return nil
}

// splitPatterns splits a comma-separated pattern string.
func splitPatterns(s string) []string {
	if s == "" {
		return nil
	}
	var patterns []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			pattern := s[start:i]
			if pattern != "" {
				patterns = append(patterns, pattern)
			}
			start = i + 1
		}
	}
	return patterns
}
