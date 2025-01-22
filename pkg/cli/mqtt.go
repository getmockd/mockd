package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/cliconfig"
)

// RunMQTT handles the mqtt command and its subcommands.
func RunMQTT(args []string) error {
	if len(args) == 0 {
		printMQTTUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "publish":
		return runMQTTPublish(subArgs)
	case "subscribe":
		return runMQTTSubscribe(subArgs)
	case "status":
		return runMQTTStatus(subArgs)
	case "help", "--help", "-h":
		printMQTTUsage()
		return nil
	default:
		return fmt.Errorf("unknown mqtt subcommand: %s\n\nRun 'mockd mqtt --help' for usage", subcommand)
	}
}

func printMQTTUsage() {
	fmt.Print(`Usage: mockd mqtt <subcommand> [flags]

Publish and subscribe to MQTT messages for testing.

Subcommands:
  publish    Publish a message to a topic
  subscribe  Subscribe to a topic and print messages
  status     Show MQTT broker status

Run 'mockd mqtt <subcommand> --help' for more information.
`)
}

// runMQTTPublish publishes a message to an MQTT topic.
func runMQTTPublish(args []string) error {
	fs := flag.NewFlagSet("mqtt publish", flag.ContinueOnError)

	broker := fs.String("broker", "localhost:1883", "MQTT broker address")
	fs.StringVar(broker, "b", "localhost:1883", "MQTT broker address (shorthand)")

	username := fs.String("username", "", "MQTT username")
	fs.StringVar(username, "u", "", "MQTT username (shorthand)")

	password := fs.String("password", "", "MQTT password")
	fs.StringVar(password, "P", "", "MQTT password (shorthand)")

	qos := fs.Int("qos", 0, "QoS level (0, 1, or 2)")
	retain := fs.Bool("retain", false, "Retain message")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd mqtt publish [flags] <topic> <message>

Publish a message to an MQTT topic.

Arguments:
  topic      MQTT topic to publish to
  message    Message payload (or @filename for file content)

Flags:
  -b, --broker    MQTT broker address (default: localhost:1883)
  -u, --username  MQTT username
  -P, --password  MQTT password
      --qos       QoS level 0, 1, or 2 (default: 0)
      --retain    Retain message on broker

Examples:
  # Publish a simple message
  mockd mqtt publish sensors/temperature "25.5"

  # Publish with custom broker
  mockd mqtt publish -b mqtt.example.com:1883 sensors/temp "25.5"

  # Publish with authentication
  mockd mqtt publish -u user -P pass sensors/temp "25.5"

  # Publish with QoS 1 and retain
  mockd mqtt publish --qos 1 --retain sensors/temp "25.5"

  # Publish from file
  mockd mqtt publish sensors/config @config.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("topic and message are required")
	}

	topic := fs.Arg(0)
	message := fs.Arg(1)

	// Load message from file if prefixed with @
	if len(message) > 0 && message[0] == '@' {
		msgBytes, err := os.ReadFile(message[1:])
		if err != nil {
			return fmt.Errorf("failed to read message file: %w", err)
		}
		message = string(msgBytes)
	}

	// Check if mosquitto_pub is available
	mosquittoPubPath, err := exec.LookPath("mosquitto_pub")
	if err != nil {
		return printMQTTPublishInstructions(*broker, topic, message, *username, *password, *qos, *retain)
	}

	// Build mosquitto_pub command
	pubArgs := []string{"-h", extractMQTTHost(*broker), "-p", extractMQTTPort(*broker)}
	if *username != "" {
		pubArgs = append(pubArgs, "-u", *username)
	}
	if *password != "" {
		pubArgs = append(pubArgs, "-P", *password)
	}
	pubArgs = append(pubArgs, "-q", fmt.Sprintf("%d", *qos))
	if *retain {
		pubArgs = append(pubArgs, "-r")
	}
	pubArgs = append(pubArgs, "-t", topic, "-m", message)

	cmd := exec.Command(mosquittoPubPath, pubArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	fmt.Printf("Published to %s: %s\n", topic, message)
	return nil
}

// runMQTTSubscribe subscribes to an MQTT topic and prints messages.
func runMQTTSubscribe(args []string) error {
	fs := flag.NewFlagSet("mqtt subscribe", flag.ContinueOnError)

	broker := fs.String("broker", "localhost:1883", "MQTT broker address")
	fs.StringVar(broker, "b", "localhost:1883", "MQTT broker address (shorthand)")

	username := fs.String("username", "", "MQTT username")
	fs.StringVar(username, "u", "", "MQTT username (shorthand)")

	password := fs.String("password", "", "MQTT password")
	fs.StringVar(password, "P", "", "MQTT password (shorthand)")

	qos := fs.Int("qos", 0, "QoS level (0, 1, or 2)")
	count := fs.Int("count", 0, "Number of messages to receive (0 = unlimited)")
	fs.IntVar(count, "n", 0, "Number of messages (shorthand)")

	timeout := fs.Duration("timeout", 0, "Timeout duration (0 = no timeout)")
	fs.DurationVar(timeout, "t", 0, "Timeout (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd mqtt subscribe [flags] <topic>

Subscribe to an MQTT topic and print received messages.

Arguments:
  topic    MQTT topic to subscribe to (supports wildcards: +, #)

Flags:
  -b, --broker    MQTT broker address (default: localhost:1883)
  -u, --username  MQTT username
  -P, --password  MQTT password
      --qos       QoS level 0, 1, or 2 (default: 0)
  -n, --count     Number of messages to receive (0 = unlimited)
  -t, --timeout   Timeout duration (e.g., 30s, 5m)

Examples:
  # Subscribe to a topic
  mockd mqtt subscribe sensors/temperature

  # Subscribe with wildcard
  mockd mqtt subscribe "sensors/#"

  # Receive only 5 messages
  mockd mqtt subscribe -n 5 sensors/temperature

  # Subscribe with timeout
  mockd mqtt subscribe -t 30s sensors/temperature

  # Subscribe with authentication
  mockd mqtt subscribe -u user -P pass sensors/temperature
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("topic is required")
	}

	topic := fs.Arg(0)

	// Check if mosquitto_sub is available
	mosquittoSubPath, err := exec.LookPath("mosquitto_sub")
	if err != nil {
		return printMQTTSubscribeInstructions(*broker, topic, *username, *password, *qos, *count, *timeout)
	}

	// Build mosquitto_sub command
	subArgs := []string{"-h", extractMQTTHost(*broker), "-p", extractMQTTPort(*broker)}
	if *username != "" {
		subArgs = append(subArgs, "-u", *username)
	}
	if *password != "" {
		subArgs = append(subArgs, "-P", *password)
	}
	subArgs = append(subArgs, "-q", fmt.Sprintf("%d", *qos))
	if *count > 0 {
		subArgs = append(subArgs, "-C", fmt.Sprintf("%d", *count))
	}
	subArgs = append(subArgs, "-v", "-t", topic)

	fmt.Printf("Subscribed to %s (Ctrl+C to stop)\n", topic)

	cmd := exec.Command(mosquittoSubPath, subArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start command in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start subscription: %w", err)
	}

	// Setup timeout if specified
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	if *timeout > 0 {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("subscription error: %w", err)
			}
		case <-sigChan:
			cmd.Process.Kill()
		case <-time.After(*timeout):
			cmd.Process.Kill()
			fmt.Println("\nTimeout reached")
		}
	} else {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("subscription error: %w", err)
			}
		case <-sigChan:
			cmd.Process.Kill()
		}
	}

	return nil
}

// extractMQTTHost extracts the host from a broker address.
func extractMQTTHost(broker string) string {
	for i := 0; i < len(broker); i++ {
		if broker[i] == ':' {
			return broker[:i]
		}
	}
	return broker
}

// extractMQTTPort extracts the port from a broker address.
func extractMQTTPort(broker string) string {
	for i := 0; i < len(broker); i++ {
		if broker[i] == ':' {
			return broker[i+1:]
		}
	}
	return "1883"
}

// printMQTTPublishInstructions prints instructions when mosquitto_pub is not available.
func printMQTTPublishInstructions(broker, topic, message, username, password string, qos int, retain bool) error {
	fmt.Println("mosquitto_pub is not installed. To publish MQTT messages, install mosquitto-clients:")
	fmt.Println()
	fmt.Println("  # macOS")
	fmt.Println("  brew install mosquitto")
	fmt.Println()
	fmt.Println("  # Ubuntu/Debian")
	fmt.Println("  sudo apt install mosquitto-clients")
	fmt.Println()
	fmt.Println("Or use this command directly:")
	fmt.Println()

	cmd := fmt.Sprintf("mosquitto_pub -h %s -p %s -t '%s' -m '%s'",
		extractMQTTHost(broker), extractMQTTPort(broker), topic, message)
	if username != "" {
		cmd += fmt.Sprintf(" -u '%s'", username)
	}
	if password != "" {
		cmd += fmt.Sprintf(" -P '%s'", password)
	}
	if qos > 0 {
		cmd += fmt.Sprintf(" -q %d", qos)
	}
	if retain {
		cmd += " -r"
	}

	fmt.Println("  " + cmd)

	return fmt.Errorf("mosquitto_pub not found")
}

// printMQTTSubscribeInstructions prints instructions when mosquitto_sub is not available.
func printMQTTSubscribeInstructions(broker, topic, username, password string, qos, count int, timeout time.Duration) error {
	fmt.Println("mosquitto_sub is not installed. To subscribe to MQTT topics, install mosquitto-clients:")
	fmt.Println()
	fmt.Println("  # macOS")
	fmt.Println("  brew install mosquitto")
	fmt.Println()
	fmt.Println("  # Ubuntu/Debian")
	fmt.Println("  sudo apt install mosquitto-clients")
	fmt.Println()
	fmt.Println("Or use this command directly:")
	fmt.Println()

	cmd := fmt.Sprintf("mosquitto_sub -h %s -p %s -t '%s' -v",
		extractMQTTHost(broker), extractMQTTPort(broker), topic)
	if username != "" {
		cmd += fmt.Sprintf(" -u '%s'", username)
	}
	if password != "" {
		cmd += fmt.Sprintf(" -P '%s'", password)
	}
	if qos > 0 {
		cmd += fmt.Sprintf(" -q %d", qos)
	}
	if count > 0 {
		cmd += fmt.Sprintf(" -C %d", count)
	}

	fmt.Println("  " + cmd)

	return fmt.Errorf("mosquitto_sub not found")
}

// runMQTTStatus shows the current MQTT broker status.
func runMQTTStatus(args []string) error {
	fs := flag.NewFlagSet("mqtt status", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd mqtt status [flags]

Show MQTT broker status.

Flags:
      --admin-url   Admin API base URL (default: http://localhost:4290)
      --json        Output in JSON format

Examples:
  mockd mqtt status
  mockd mqtt status --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get MQTT status from admin API
	client := NewAdminClientWithAuth(*adminURL)
	status, err := client.GetMQTTStatus()
	if err != nil {
		return fmt.Errorf("failed to get MQTT status: %s", FormatConnectionError(err))
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	// Pretty print status
	running, _ := status["running"].(bool)
	if !running {
		fmt.Println("MQTT broker: not running")
		return nil
	}

	fmt.Println("MQTT broker: running")

	if port, ok := status["port"].(float64); ok {
		fmt.Printf("  Port: %d\n", int(port))
	}

	if clientCount, ok := status["clientCount"].(float64); ok {
		fmt.Printf("  Connected clients: %d\n", int(clientCount))
	}

	if topicCount, ok := status["topicCount"].(float64); ok {
		fmt.Printf("  Configured topics: %d\n", int(topicCount))
	}

	if tlsEnabled, ok := status["tlsEnabled"].(bool); ok && tlsEnabled {
		fmt.Println("  TLS: enabled")
	}

	if authEnabled, ok := status["authEnabled"].(bool); ok && authEnabled {
		fmt.Println("  Authentication: enabled")
	}

	return nil
}
