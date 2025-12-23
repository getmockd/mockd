package client_test

import (
	"fmt"
	"log"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
)

func ExampleClient_GetHealth() {
	c := client.NewDefaultClient()
	health, err := c.GetHealth()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Status: %s\n", health.Status)
}

func ExampleClient_ListMocks() {
	c := client.NewDefaultClient()
	mocks, err := c.ListMocks()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d mocks\n", len(mocks))
}

func ExampleClient_CreateMock() {
	c := client.NewDefaultClient()

	newMock := &config.MockConfiguration{
		Name: "Example API",
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/example",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"message": "Hello from mockd"}`,
		},
		Enabled: true,
	}

	created, err := c.CreateMock(newMock)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created mock with ID: %s\n", created.ID)
}

func ExampleClient_ToggleMock() {
	c := client.NewDefaultClient()

	// Disable a mock
	mock, err := c.ToggleMock("mock-id", false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Mock enabled: %t\n", mock.Enabled)
}

func ExampleClient_GetTraffic() {
	c := client.NewDefaultClient()

	// Get recent traffic
	filter := &client.RequestLogFilter{
		Limit: 10,
	}
	entries, err := c.GetTraffic(filter)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		fmt.Printf("%s %s\n", entry.Method, entry.Path)
	}
}

func ExampleClient_ListStreamRecordings() {
	c := client.NewDefaultClient()

	// List WebSocket recordings
	filter := &client.StreamRecordingFilter{
		Protocol: "websocket",
		Limit:    50,
	}
	recordings, err := c.ListStreamRecordings(filter)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d WebSocket recordings\n", len(recordings))
}

func ExampleClient_StartReplay() {
	c := client.NewDefaultClient()

	// Start replay in synchronized mode at 2x speed
	sessionID, err := c.StartReplay(
		"recording-id",
		"synchronized",
		0.5, // 2x speed (0.5 = half the delay)
		true,
		30000,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Started replay session: %s\n", sessionID)

	// Get status
	status, err := c.GetReplayStatus(sessionID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Replay progress: %d/%d frames\n", status.CurrentFrame, status.TotalFrames)
}

func ExampleClient_GetProxyStatus() {
	c := client.NewDefaultClient()

	status, err := c.GetProxyStatus()
	if err != nil {
		log.Fatal(err)
	}

	if status.Running {
		fmt.Printf("Proxy is running on port %d in %s mode\n", status.Port, status.Mode)
		fmt.Printf("Recording count: %d\n", status.RecordingCount)
	} else {
		fmt.Println("Proxy is not running")
	}
}

func ExampleClient_Ping() {
	c := client.NewDefaultClient()

	if err := c.Ping(); err != nil {
		log.Printf("Server is not reachable: %v", err)
	} else {
		log.Println("Server is healthy")
	}
}
