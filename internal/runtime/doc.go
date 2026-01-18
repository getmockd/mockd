// Package runtime provides the control plane client for runtime mode.
//
// When mockd runs in runtime mode (--register flag), it registers with a control
// plane server and receives mock deployments to serve. This package handles:
//
// - Registration with the control plane
// - Periodic heartbeat to report health and receive commands
// - Processing deploy/undeploy commands
// - Local caching of deployed mocks
// - WebSocket sync for real-time updates (optional)
//
// Usage:
//
//	client := runtime.NewClient(runtime.Config{
//	    ControlPlaneURL: "https://api.mockd.io",
//	    Token:           "rt_xxx",
//	    Name:            "ci-runner-1",
//	    Labels:          map[string]string{"env": "ci"},
//	})
//
//	if err := client.Register(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start heartbeat loop
//	go client.HeartbeatLoop(ctx)
//
//	// Get deployments to serve
//	deployments := client.GetDeployments()
package runtime
