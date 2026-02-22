package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var mqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "Manage and test MQTT endpoints",
}

var mqttAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new MQTT mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use huh interactive forms if attributes are missing
		if !cmd.Flags().Changed("topic") {
			var formTopic, formPayload string
			var formQosString = "0"

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("What MQTT topic should be mocked?").
						Placeholder("sensors/temp/#").
						Value(&formTopic).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("topic is required")
							}
							return nil
						}),
					huh.NewInput().
						Title("What payload JSON should be returned?").
						Placeholder(`{"status": "ok"}`).
						Value(&formPayload),
					huh.NewSelect[string]().
						Title("QoS Level").
						Options(
							huh.NewOption("0 - At most once", "0"),
							huh.NewOption("1 - At least once", "1"),
							huh.NewOption("2 - Exactly once", "2"),
						).
						Value(&formQosString),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			addTopic = formTopic
			addPayload = formPayload
			addQoS, _ = strconv.Atoi(formQosString)
		}
		addMockType = "mqtt"
		return runAdd(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(mqttCmd)
	mqttCmd.AddCommand(mqttAddCmd)

	mqttAddCmd.Flags().StringVar(&addTopic, "topic", "", "MQTT topic pattern")
	mqttAddCmd.Flags().StringVar(&addPayload, "payload", "", "MQTT response payload")
	mqttAddCmd.Flags().IntVar(&addQoS, "qos", 0, "MQTT QoS level (0, 1, 2)")
	mqttAddCmd.Flags().IntVar(&addMQTTPort, "mqtt-port", 1883, "MQTT broker port")

	// Add list/get/delete generic aliases
	mqttCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List MQTT mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "mqtt"
			return runList(cmd, args)
		},
	})
	mqttCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of an MQTT mock",
		RunE:  runGet,
	})
	mqttCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete an MQTT mock",
		RunE:  runDelete,
	})

	// Add existing subcommands to mqttCmd
	mqttPubCmd.Flags().StringVarP(&mqttMessage, "message", "m", "", "Message to publish")
	mqttPubCmd.Flags().IntVarP(&mqttQos, "qos", "q", 0, "QoS level (0, 1, 2)")
	mqttPubCmd.Flags().BoolVarP(&mqttRetain, "retain", "r", false, "Retain message")
	mqttPubCmd.Flags().StringVarP(&mqttUsername, "username", "u", "", "Username")
	mqttPubCmd.Flags().StringVarP(&mqttPassword, "password", "P", "", "Password")
	mqttCmd.AddCommand(mqttPubCmd)

	mqttSubCmd.Flags().IntVarP(&mqttQos, "qos", "q", 0, "QoS level (0, 1, 2)")
	mqttSubCmd.Flags().IntVarP(&mqttCount, "count", "c", 0, "Stop after receiving this many messages")
	mqttSubCmd.Flags().DurationVarP(&mqttTimeout, "timeout", "t", 0, "Stop after this duration")
	mqttSubCmd.Flags().StringVarP(&mqttUsername, "username", "u", "", "Username")
	mqttSubCmd.Flags().StringVarP(&mqttPassword, "password", "P", "", "Password")
	mqttCmd.AddCommand(mqttSubCmd)

	mqttCmd.AddCommand(mqttStatusCmd)
}

var (
	mqttMessage  string
	mqttQos      int
	mqttRetain   bool
	mqttUsername string
	mqttPassword string
	mqttCount    int
	mqttTimeout  time.Duration
)

var mqttPubCmd = &cobra.Command{
	Use:   "publish <broker> <topic> <message>",
	Short: "Publish a message to an MQTT topic",
	RunE:  runMQTTPublish,
}

var mqttSubCmd = &cobra.Command{
	Use:   "subscribe <broker> <topic>",
	Short: "Subscribe to an MQTT topic and print messages",
	RunE:  runMQTTSubscribe,
}

var mqttStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current MQTT broker status",
	RunE:  runMQTTStatus,
}

// runMQTTPublish publishes a message to an MQTT topic.
func runMQTTPublish(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("broker address and topic are required")
	}

	broker := args[0]
	topic := args[1]

	// Message can be positional or flag
	msg := mqttMessage
	if msg == "" && len(args) >= 3 {
		msg = args[2]
	}

	if msg == "" {
		return errors.New("message is required (via -m flag or argument)")
	}

	// Load message from file if prefixed with @
	if len(msg) > 0 && msg[0] == '@' {
		msgBytes, err := os.ReadFile(msg[1:])
		if err != nil {
			return fmt.Errorf("failed to read message file: %w", err)
		}
		msg = string(msgBytes)
	}

	//nolint:misspell // mosquitto is the correct name of the MQTT broker software
	// Check if mosquitto_pub is available
	//nolint:misspell // mosquitto is the correct name of the MQTT broker software
	mosquittoPubPath, err := exec.LookPath("mosquitto_pub")
	if err != nil {
		return printMQTTPublishInstructions(broker, topic, msg, mqttUsername, mqttPassword, mqttQos, mqttRetain)
	}

	//nolint:misspell // mosquitto is MQTT broker software name
	// Build mosquitto_pub command
	pubArgs := []string{"-h", extractMQTTHost(broker), "-p", extractMQTTPort(broker)}
	if mqttUsername != "" {
		pubArgs = append(pubArgs, "-u", mqttUsername)
	}
	if mqttPassword != "" {
		pubArgs = append(pubArgs, "-P", mqttPassword)
	}
	pubArgs = append(pubArgs, "-q", strconv.Itoa(mqttQos))
	if mqttRetain {
		pubArgs = append(pubArgs, "-r")
	}
	pubArgs = append(pubArgs, "-t", topic, "-m", msg)

	execCmd := exec.Command(mosquittoPubPath, pubArgs...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	fmt.Printf("Published to %s: %s\n", topic, msg)
	return nil
}

// runMQTTSubscribe subscribes to an MQTT topic and prints messages.
func runMQTTSubscribe(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("topic is required, optionally provide broker as first arg")
	}

	broker := "localhost:1883"
	topic := args[0]
	if len(args) >= 2 {
		broker = args[0]
		topic = args[1]
	}

	// Check if mosquitto_sub is available
	//nolint:misspell // mosquitto is the correct name of the MQTT broker software
	mosquittoSubPath, err := exec.LookPath("mosquitto_sub")
	if err != nil {
		return printMQTTSubscribeInstructions(broker, topic, mqttUsername, mqttPassword, mqttQos, mqttCount, mqttTimeout)
	}

	//nolint:misspell // mosquitto is the correct name of the MQTT broker software
	// Build mosquitto_sub command
	subArgs := []string{"-h", extractMQTTHost(broker), "-p", extractMQTTPort(broker)}
	if mqttUsername != "" {
		subArgs = append(subArgs, "-u", mqttUsername)
	}
	if mqttPassword != "" {
		subArgs = append(subArgs, "-P", mqttPassword)
	}
	subArgs = append(subArgs, "-q", strconv.Itoa(mqttQos))
	if mqttCount > 0 {
		subArgs = append(subArgs, "-C", strconv.Itoa(mqttCount))
	}
	subArgs = append(subArgs, "-v", "-t", topic)

	fmt.Printf("Subscribed to %s (Ctrl+C to stop)\n", topic)

	cmdSub := exec.Command(mosquittoSubPath, subArgs...)
	cmdSub.Stdout = os.Stdout
	cmdSub.Stderr = os.Stderr

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start command in background
	if err := cmdSub.Start(); err != nil {
		return fmt.Errorf("failed to start subscription: %w", err)
	}

	// Setup timeout if specified
	done := make(chan error)
	go func() {
		done <- cmdSub.Wait()
	}()

	if mqttTimeout > 0 {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("subscription error: %w", err)
			}
		case <-sigChan:
			_ = cmdSub.Process.Kill()
		case <-time.After(mqttTimeout):
			_ = cmdSub.Process.Kill()
			fmt.Println("\nTimeout reached")
		}
	} else {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("subscription error: %w", err)
			}
		case <-sigChan:
			_ = cmdSub.Process.Kill()
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
//
//nolint:misspell // mosquitto is MQTT broker software name, not a typo
func printMQTTPublishInstructions(broker, topic, message, username, password string, qos int, retain bool) error {
	fmt.Println("mosquitto_pub is not installed. To publish MQTT messages, install mosquitto-clients:")
	fmt.Println()
	fmt.Println("  # macOS")
	fmt.Println("  brew install mosquitto") //nolint:misspell
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

	return errors.New("mosquitto_pub not found")
}

// printMQTTSubscribeInstructions prints instructions when mosquitto_sub is not available.
//
//nolint:misspell // mosquitto is MQTT broker software name, not a typo
func printMQTTSubscribeInstructions(broker, topic, username, password string, qos, count int, _ time.Duration) error {
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

	return errors.New("mosquitto_sub not found")
}

// runMQTTStatus shows the current MQTT broker status.
func runMQTTStatus(cmd *cobra.Command, args []string) error {
	// Get MQTT status from admin API
	client := NewAdminClientWithAuth(adminURL)
	status, err := client.GetMQTTStatus()
	if err != nil {
		return fmt.Errorf("failed to get MQTT status: %s", FormatConnectionError(err))
	}

	printResult(status, func() {
		running, _ := status["running"].(bool)
		if !running {
			fmt.Println("MQTT broker: not running")
			return
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
	})
	return nil
}
