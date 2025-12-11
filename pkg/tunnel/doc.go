// Package tunnel provides a WebSocket tunnel client for exposing local mockd
// instances to the internet via a relay server.
//
// The tunnel client establishes an outbound WebSocket connection to the relay
// server, authenticates with a JWT token, and receives incoming HTTP requests
// which are forwarded to the local mock engine.
//
// Example usage:
//
//	cfg := &tunnel.Config{
//		RelayURL:   "wss://relay.mockd.io/tunnel",
//		Token:      "your-jwt-token",
//		Subdomain:  "my-mocks",
//	}
//
//	client, err := tunnel.NewClient(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if err := client.Connect(context.Background()); err != nil {
//		log.Fatal(err)
//	}
//
//	// Client is now connected and forwarding requests
//	// Use client.Disconnect() to stop
package tunnel
