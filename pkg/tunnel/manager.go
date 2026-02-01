// Package tunnel provides tunnel client implementations for exposing local mock servers.
// This file contains the TunnelManager which manages the lifecycle of a QUIC tunnel
// connection based on configuration from the admin API.

package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/tunnel/protocol"
	quicclient "github.com/getmockd/mockd/pkg/tunnel/quic"
)

// TunnelManager manages a single tunnel connection for an engine.
// It is owned by the admin API and handles the lifecycle of connecting/disconnecting
// to/from the relay based on TunnelConfig changes.
type TunnelManager struct {
	// Connection state
	client    *quicclient.Client
	config    *store.TunnelConfig
	handler   http.Handler
	relayAddr string
	insecure  bool
	mu        sync.Mutex
	cancel    context.CancelFunc
	running   bool

	// Callbacks for updating admin state
	onStatusChange func(status, publicURL, sessionID, transport string)

	logger *slog.Logger
}

// TunnelManagerConfig configures the TunnelManager.
type TunnelManagerConfig struct {
	// Handler is the HTTP handler that serves mock requests
	Handler http.Handler

	// RelayAddr is the relay server address (default: relay.mockd.io:443)
	RelayAddr string

	// Insecure skips TLS verification (for development)
	Insecure bool

	// Logger for operational logging
	Logger *slog.Logger

	// OnStatusChange is called when the tunnel status changes.
	// Called with (status, publicURL, sessionID, transport).
	OnStatusChange func(status, publicURL, sessionID, transport string)
}

// NewTunnelManager creates a new tunnel manager.
func NewTunnelManager(cfg *TunnelManagerConfig) *TunnelManager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	relayAddr := cfg.RelayAddr
	if relayAddr == "" {
		relayAddr = "relay.mockd.io:443"
	}

	return &TunnelManager{
		handler:        cfg.Handler,
		relayAddr:      relayAddr,
		insecure:       cfg.Insecure,
		logger:         logger,
		onStatusChange: cfg.OnStatusChange,
	}
}

// Enable starts a tunnel connection based on the given config.
// If a tunnel is already running, it is stopped first.
func (m *TunnelManager) Enable(cfg *store.TunnelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing connection if any
	m.stopLocked()

	if cfg == nil || !cfg.Enabled {
		return nil
	}

	m.config = cfg
	m.notifyStatus("connecting", "", "", "")

	// Build token from config or environment
	token := "" // Will be provided by admin config
	if cfg.Auth != nil && cfg.Auth.Type == "token" {
		token = cfg.Auth.Token
	}

	// Create QUIC client
	client := quicclient.NewClient(&quicclient.ClientConfig{
		RelayAddr:   m.relayAddr,
		Token:       token,
		LocalPort:   0, // Not needed when using handler
		Handler:     m.handler,
		TLSInsecure: m.insecure,
		Logger:      m.logger,
	})

	client.OnConnect = func(publicURL string) {
		m.logger.Info("tunnel connected",
			"publicURL", publicURL,
			"subdomain", client.Subdomain(),
		)
		m.notifyStatus("connected", publicURL, client.SessionID(), "quic")
	}

	client.OnDisconnect = func(err error) {
		if err != nil {
			m.logger.Warn("tunnel disconnected", "error", err)
		} else {
			m.logger.Info("tunnel disconnected cleanly")
		}
		m.notifyStatus("disconnected", "", "", "")
	}

	client.OnGoaway = func(payload protocol.GoawayPayload) {
		m.logger.Info("received GOAWAY, will reconnect",
			"reason", payload.Reason,
			"drain_timeout_ms", payload.DrainTimeoutMs,
		)
		// Reconnect in background after a short delay
		go m.reconnectAfterGoaway(payload)
	}

	// Connect in background
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.client = client
	m.running = true

	go m.connectAndRun(ctx, client)

	return nil
}

// Disable stops the tunnel connection.
func (m *TunnelManager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
	m.notifyStatus("disconnected", "", "", "")
}

// stopLocked stops the tunnel connection. Must be called with m.mu held.
func (m *TunnelManager) stopLocked() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.client != nil {
		_ = m.client.Close()
		m.client = nil
	}
	m.running = false
	m.config = nil
}

// IsRunning returns true if a tunnel is currently active.
func (m *TunnelManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running && m.client != nil && m.client.IsConnected()
}

// Status returns the current tunnel status info.
func (m *TunnelManager) Status() (connected bool, publicURL, sessionID, transport string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil || !m.running {
		return false, "", "", ""
	}

	return m.client.IsConnected(),
		m.client.PublicURL(),
		m.client.SessionID(),
		"quic"
}

// Stats returns tunnel statistics.
func (m *TunnelManager) Stats() *store.TunnelStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil
	}

	uptime := ""
	if m.config != nil && m.config.ConnectedAt != nil {
		uptime = time.Since(*m.config.ConnectedAt).Round(time.Second).String()
	}

	return &store.TunnelStats{
		RequestsServed: m.client.RequestCount(),
		Uptime:         uptime,
	}
}

// connectAndRun connects and runs the tunnel client. Used by Enable and reconnect.
func (m *TunnelManager) connectAndRun(ctx context.Context, client *quicclient.Client) {
	if err := client.Connect(ctx); err != nil {
		if ctx.Err() != nil {
			return // Context cancelled, expected
		}
		m.logger.Error("tunnel connect failed", "error", err)
		m.notifyStatus("error", "", "", "")
		return
	}

	// Run until cancelled or disconnected
	if err := client.Run(ctx); err != nil && err != context.Canceled {
		m.logger.Error("tunnel run error", "error", err)
		m.notifyStatus("error", "", "", "")
	}
}

// reconnectAfterGoaway handles reconnection after receiving a GOAWAY message.
// Uses exponential backoff with a short initial delay.
func (m *TunnelManager) reconnectAfterGoaway(payload protocol.GoawayPayload) {
	_ = payload // payload reserved for future use (e.g., reconnect hints from relay)

	m.mu.Lock()
	if !m.running || m.config == nil {
		m.mu.Unlock()
		return
	}
	cfg := m.config
	m.mu.Unlock()

	// Wait a short delay before reconnecting to give the relay time to shut down
	// and a new instance to come up
	baseDelay := 500 * time.Millisecond
	maxDelay := 30 * time.Second
	delay := baseDelay

	for attempt := 1; attempt <= 10; attempt++ {
		m.logger.Info("reconnecting after GOAWAY", "attempt", attempt, "delay", delay)
		time.Sleep(delay)

		m.mu.Lock()
		if !m.running {
			m.mu.Unlock()
			return // Manager was disabled while we were waiting
		}

		// Stop old client
		if m.client != nil {
			_ = m.client.Close()
		}

		// Build token from saved config
		token := ""
		if cfg.Auth != nil && cfg.Auth.Type == "token" {
			token = cfg.Auth.Token
		}

		// Create fresh client
		client := quicclient.NewClient(&quicclient.ClientConfig{
			RelayAddr:   m.relayAddr,
			Token:       token,
			LocalPort:   0,
			Handler:     m.handler,
			TLSInsecure: m.insecure,
			Logger:      m.logger,
		})

		client.OnConnect = func(publicURL string) {
			m.logger.Info("tunnel reconnected",
				"publicURL", publicURL,
				"subdomain", client.Subdomain(),
			)
			m.notifyStatus("connected", publicURL, client.SessionID(), "quic")
		}

		client.OnDisconnect = func(err error) {
			if err != nil {
				m.logger.Warn("tunnel disconnected after reconnect", "error", err)
			}
			m.notifyStatus("disconnected", "", "", "")
		}

		client.OnGoaway = func(p protocol.GoawayPayload) {
			go m.reconnectAfterGoaway(p)
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		m.client = client
		m.mu.Unlock()

		m.notifyStatus("connecting", "", "", "")
		m.connectAndRun(ctx, client)

		// If we get here, the connection ended. Check if it was intentional.
		m.mu.Lock()
		if !m.running {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		// Exponential backoff
		delay = delay * 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	m.logger.Error("failed to reconnect after GOAWAY, max attempts reached")
	m.notifyStatus("error", "", "", "")
}

// Close shuts down the tunnel manager.
func (m *TunnelManager) Close() error {
	m.Disable()
	return nil
}

// notifyStatus calls the status change callback if set.
func (m *TunnelManager) notifyStatus(status, publicURL, sessionID, transport string) {
	if m.onStatusChange != nil {
		m.onStatusChange(status, publicURL, sessionID, transport)
	}
}

// String returns a human-readable description.
func (m *TunnelManager) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return "TunnelManager(inactive)"
	}
	if m.client != nil && m.client.IsConnected() {
		return fmt.Sprintf("TunnelManager(connected, url=%s)", m.client.PublicURL())
	}
	return "TunnelManager(connecting)"
}
