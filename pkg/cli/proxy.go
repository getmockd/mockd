package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/proxy"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/store"
)

// SessionMeta is the metadata written to meta.json for each recording session.
type SessionMeta struct {
	Name           string   `json:"name"`
	StartTime      string   `json:"startTime"`
	EndTime        string   `json:"endTime,omitempty"`
	Port           int      `json:"port"`
	Mode           string   `json:"mode"`
	RecordingCount int      `json:"recordingCount"`
	Hosts          []string `json:"hosts,omitempty"`
	Filters        *struct {
		IncludePaths []string `json:"includePaths,omitempty"`
		ExcludePaths []string `json:"excludePaths,omitempty"`
		IncludeHosts []string `json:"includeHosts,omitempty"`
		ExcludeHosts []string `json:"excludeHosts,omitempty"`
	} `json:"filters,omitempty"`
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
  start     Start the proxy server (foreground, Ctrl+C to stop)
  ca        Manage CA certificate

Run 'mockd proxy <subcommand> --help' for more information.
`)
}

// runProxyStart starts the proxy server in the foreground.
// Recordings are written to disk continuously as traffic is captured.
func runProxyStart(args []string) error {
	fs := flag.NewFlagSet("proxy start", flag.ContinueOnError)

	port := fs.Int("port", 8888, "Proxy server port")
	fs.IntVar(port, "p", 8888, "Proxy server port (shorthand)")

	mode := fs.String("mode", "record", "Proxy mode: record or passthrough")
	fs.StringVar(mode, "m", "record", "Proxy mode (shorthand)")

	session := fs.String("session", "", "Recording session name")
	fs.StringVar(session, "s", "", "Recording session name (shorthand)")

	recordingsDir := fs.String("recordings-dir", "", "Base directory for recordings (default: ~/.local/share/mockd/recordings)")

	caPath := fs.String("ca-path", "", "Path to CA certificate directory")

	includePaths := fs.String("include", "", "Comma-separated path patterns to include")
	excludePaths := fs.String("exclude", "", "Comma-separated path patterns to exclude")
	includeHosts := fs.String("include-hosts", "", "Comma-separated host patterns to include")
	excludeHosts := fs.String("exclude-hosts", "", "Comma-separated host patterns to exclude")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy start [flags]

Start the MITM proxy server for recording API traffic.
Recordings are written to disk as traffic flows through the proxy.
Press Ctrl+C to stop. Use 'mockd recordings list' to view captured traffic.

Flags:
  -p, --port            Proxy server port (default: 8888)
  -m, --mode            Proxy mode: record or passthrough (default: record)
  -s, --session         Recording session name (default: auto-generated)
      --recordings-dir  Base directory for recordings
      --ca-path         Path to CA certificate directory (enables HTTPS recording)
      --include         Comma-separated path patterns to include
      --exclude         Comma-separated path patterns to exclude
      --include-hosts   Comma-separated host patterns to include
      --exclude-hosts   Comma-separated host patterns to exclude

Examples:
  # Start proxy in record mode
  mockd proxy start

  # Start with named session and HTTPS support
  mockd proxy start --session stripe-api --ca-path ~/.mockd/ca

  # Record only specific hosts
  mockd proxy start --session my-api --include-hosts "api.example.com,*.stripe.com"
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

	// Determine session name and directory
	sessionName := *session
	if sessionName == "" {
		sessionName = "default"
	}
	timestamp := time.Now().Format("20060102-150405")
	sessionDirName := sessionName + "-" + timestamp

	// Determine recordings base directory
	baseDir := *recordingsDir
	if baseDir == "" {
		baseDir = store.DefaultRecordingsDir()
	}
	sessionDir := filepath.Join(baseDir, sessionDirName)

	// Create session directory
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Build filter config
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

	// Write initial meta.json
	meta := SessionMeta{
		Name:      sessionName,
		StartTime: time.Now().Format(time.RFC3339),
		Port:      *port,
		Mode:      *mode,
	}
	if *includePaths != "" || *excludePaths != "" || *includeHosts != "" || *excludeHosts != "" {
		meta.Filters = &struct {
			IncludePaths []string `json:"includePaths,omitempty"`
			ExcludePaths []string `json:"excludePaths,omitempty"`
			IncludeHosts []string `json:"includeHosts,omitempty"`
			ExcludeHosts []string `json:"excludeHosts,omitempty"`
		}{
			IncludePaths: filter.IncludePaths,
			ExcludePaths: filter.ExcludePaths,
			IncludeHosts: filter.IncludeHosts,
			ExcludeHosts: filter.ExcludeHosts,
		}
	}
	if err := writeSessionMeta(sessionDir, &meta); err != nil {
		return fmt.Errorf("failed to write session metadata: %w", err)
	}

	// Create CA manager (optional, enables HTTPS MITM)
	var ca *proxy.CAManager
	if *caPath != "" {
		ca = proxy.NewCAManager(*caPath+"/ca.crt", *caPath+"/ca.key")
		if err := ca.EnsureCA(); err != nil {
			return fmt.Errorf("failed to initialize CA: %w", err)
		}
	}

	// Create in-memory store (for summary on exit)
	memStore := recording.NewStore()
	memStore.CreateSession(sessionName, nil)

	// Create proxy with disk persistence
	logger := log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	p := proxy.New(proxy.Options{
		Mode:      proxyMode,
		Store:     memStore,
		DiskDir:   sessionDir,
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

	// Print startup info
	fmt.Printf("Proxy server running on http://localhost:%d\n", *port)
	fmt.Printf("Mode: %s\n", proxyMode)
	fmt.Printf("Session: %s\n", sessionName)
	fmt.Printf("Recordings: %s\n", sessionDir)
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

	// Update meta.json with final stats
	hosts := discoverHosts(sessionDir)
	recordings, total := memStore.ListRecordings(recording.RecordingFilter{})
	meta.EndTime = time.Now().Format(time.RFC3339)
	meta.RecordingCount = total
	meta.Hosts = hosts
	if err := writeSessionMeta(sessionDir, &meta); err != nil {
		output.Warn("failed to update session metadata: %v", err)
	}

	// Update "latest" symlink
	updateLatestSymlink(baseDir, sessionDirName)

	// Print summary
	fmt.Println("Proxy stopped")
	if total > 0 {
		fmt.Printf("\nCaptured %d recordings in %s\n", total, sessionDir)
		for _, r := range recordings {
			fmt.Printf("  %s %s %s (%d)\n", r.Request.Method, r.Request.Host, r.Request.Path, r.Response.StatusCode)
		}
		fmt.Printf("\nUse 'mockd recordings list --session %s' to view\n", sessionDirName)
		fmt.Println("Use 'mockd convert --session " + sessionDirName + "' to generate mocks")
	} else {
		fmt.Println("\nNo recordings captured")
	}

	return nil
}

// writeSessionMeta writes the meta.json file for a session directory.
func writeSessionMeta(sessionDir string, meta *SessionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sessionDir, "meta.json"), data, 0600)
}

// discoverHosts scans a session directory for host subdirectories.
func discoverHosts(sessionDir string) []string {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil
	}

	var hosts []string
	for _, e := range entries {
		if e.IsDir() {
			hosts = append(hosts, e.Name())
		}
	}
	return hosts
}

// updateLatestSymlink creates or updates a "latest" symlink pointing to the session directory.
func updateLatestSymlink(baseDir, sessionDirName string) {
	latestLink := filepath.Join(baseDir, "latest")

	// Remove existing symlink (ignore errors — may not exist)
	_ = os.Remove(latestLink)

	// Create new symlink (relative path so it's portable)
	if err := os.Symlink(sessionDirName, latestLink); err != nil {
		// Symlinks may not be supported (e.g., some Windows configs).
		// Fall back to writing the session name to a "latest" file.
		_ = os.WriteFile(latestLink, []byte(sessionDirName), 0600)
	}
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

	outputPath := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(outputPath, "o", "", "Output file path (shorthand)")

	caPath := fs.String("ca-path", "", "Path to CA certificate directory")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd proxy ca export [flags]

Export the CA certificate for trust installation.

Flags:
  -o, --output   Output file path (default: stdout)
      --ca-path  Path to CA certificate directory

Examples:
  # Export to stdout
  mockd proxy ca export --ca-path ~/.mockd/ca

  # Export to file
  mockd proxy ca export --ca-path ~/.mockd/ca -o ca.crt
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *caPath == "" {
		return errors.New("--ca-path is required")
	}

	ca := proxy.NewCAManager(*caPath+"/ca.crt", *caPath+"/ca.key")
	if err := ca.Load(); err != nil {
		return fmt.Errorf("failed to load CA: %w", err)
	}

	certPEM, err := ca.CACertPEM()
	if err != nil {
		return fmt.Errorf("failed to export CA certificate: %w", err)
	}

	if *outputPath == "" {
		fmt.Print(string(certPEM))
	} else {
		if err := os.WriteFile(*outputPath, certPEM, 0644); err != nil {
			return fmt.Errorf("failed to write certificate: %w", err)
		}
		fmt.Printf("CA certificate exported to: %s\n", *outputPath)
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
		return errors.New("--ca-path is required")
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
