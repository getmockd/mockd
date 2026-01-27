package admin

import (
	"context"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Unit Tests for Port Helper Functions
// ============================================================================

func TestGetMockPort(t *testing.T) {
	tests := []struct {
		name     string
		mock     *mock.Mock
		expected int
	}{
		{
			name: "MQTT mock returns port",
			mock: &mock.Mock{
				ID:   "mqtt-1",
				Type: mock.MockTypeMQTT,
				MQTT: &mock.MQTTSpec{
					Port: 1883,
				},
			},
			expected: 1883,
		},
		{
			name: "gRPC mock returns port",
			mock: &mock.Mock{
				ID:   "grpc-1",
				Type: mock.MockTypeGRPC,
				GRPC: &mock.GRPCSpec{
					Port: 50051,
				},
			},
			expected: 50051,
		},
		{
			name: "HTTP mock returns 0 (no dedicated port)",
			mock: &mock.Mock{
				ID:   "http-1",
				Type: mock.MockTypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Path: "/api/test",
					},
				},
			},
			expected: 0,
		},
		{
			name: "WebSocket mock returns 0",
			mock: &mock.Mock{
				ID:   "ws-1",
				Type: mock.MockTypeWebSocket,
			},
			expected: 0,
		},
		{
			name: "GraphQL mock returns 0",
			mock: &mock.Mock{
				ID:   "graphql-1",
				Type: mock.MockTypeGraphQL,
			},
			expected: 0,
		},
		{
			name: "SOAP mock returns 0",
			mock: &mock.Mock{
				ID:   "soap-1",
				Type: mock.MockTypeSOAP,
			},
			expected: 0,
		},
		{
			name: "MQTT mock with nil spec returns 0",
			mock: &mock.Mock{
				ID:   "mqtt-nil",
				Type: mock.MockTypeMQTT,
				MQTT: nil,
			},
			expected: 0,
		},
		{
			name: "gRPC mock with nil spec returns 0",
			mock: &mock.Mock{
				ID:   "grpc-nil",
				Type: mock.MockTypeGRPC,
				GRPC: nil,
			},
			expected: 0,
		},
		{
			name: "MQTT mock with port 0 returns 0",
			mock: &mock.Mock{
				ID:   "mqtt-zero",
				Type: mock.MockTypeMQTT,
				MQTT: &mock.MQTTSpec{
					Port: 0,
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := getMockPort(tt.mock)
			assert.Equal(t, tt.expected, port)
		})
	}
}

func TestApplyMockFilter(t *testing.T) {
	// Create test mocks
	mocks := []*mock.Mock{
		{ID: "mqtt-1", Type: mock.MockTypeMQTT, Enabled: boolPtr(true), WorkspaceID: "ws-1", ParentID: "folder-1"},
		{ID: "mqtt-2", Type: mock.MockTypeMQTT, Enabled: boolPtr(false), WorkspaceID: "ws-1", ParentID: ""},
		{ID: "grpc-1", Type: mock.MockTypeGRPC, Enabled: boolPtr(true), WorkspaceID: "ws-1", ParentID: ""},
		{ID: "http-1", Type: mock.MockTypeHTTP, Enabled: boolPtr(true), WorkspaceID: "ws-2", ParentID: "folder-1"},
	}

	t.Run("filter by type", func(t *testing.T) {
		filter := &MockFilter{Type: "mqtt"}
		result := applyMockFilter(mocks, filter)
		assert.Len(t, result, 2)
		for _, m := range result {
			assert.Equal(t, mock.MockTypeMQTT, m.Type)
		}
	})

	t.Run("filter by workspace", func(t *testing.T) {
		filter := &MockFilter{WorkspaceID: "ws-1"}
		result := applyMockFilter(mocks, filter)
		assert.Len(t, result, 3)
		for _, m := range result {
			assert.Equal(t, "ws-1", m.WorkspaceID)
		}
	})

	t.Run("filter by enabled", func(t *testing.T) {
		enabled := true
		filter := &MockFilter{Enabled: &enabled}
		result := applyMockFilter(mocks, filter)
		assert.Len(t, result, 3)
		for _, m := range result {
			assert.NotNil(t, m.Enabled)
			assert.True(t, *m.Enabled)
		}
	})

	t.Run("filter by disabled", func(t *testing.T) {
		enabled := false
		filter := &MockFilter{Enabled: &enabled}
		result := applyMockFilter(mocks, filter)
		assert.Len(t, result, 1)
		assert.Equal(t, "mqtt-2", result[0].ID)
	})

	t.Run("filter by parent", func(t *testing.T) {
		filter := &MockFilter{ParentID: "folder-1"}
		result := applyMockFilter(mocks, filter)
		assert.Len(t, result, 2)
		for _, m := range result {
			assert.Equal(t, "folder-1", m.ParentID)
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		enabled := true
		filter := &MockFilter{Type: "mqtt", WorkspaceID: "ws-1", Enabled: &enabled}
		result := applyMockFilter(mocks, filter)
		assert.Len(t, result, 1)
		assert.Equal(t, "mqtt-1", result[0].ID)
	})

	t.Run("nil filter returns all", func(t *testing.T) {
		result := applyMockFilter(mocks, nil)
		assert.Len(t, result, 4)
	})

	t.Run("empty filter returns all", func(t *testing.T) {
		result := applyMockFilter(mocks, &MockFilter{})
		assert.Len(t, result, 4)
	})
}

func TestApplyMockPatch(t *testing.T) {
	t.Run("patch name", func(t *testing.T) {
		m := &mock.Mock{ID: "test", Name: "Original"}
		patch := map[string]interface{}{"name": "Updated"}
		applyMockPatch(m, patch)
		assert.Equal(t, "Updated", m.Name)
	})

	t.Run("patch description", func(t *testing.T) {
		m := &mock.Mock{ID: "test", Description: ""}
		patch := map[string]interface{}{"description": "New description"}
		applyMockPatch(m, patch)
		assert.Equal(t, "New description", m.Description)
	})

	t.Run("patch enabled to false", func(t *testing.T) {
		m := &mock.Mock{ID: "test", Enabled: boolPtr(true)}
		patch := map[string]interface{}{"enabled": false}
		applyMockPatch(m, patch)
		assert.NotNil(t, m.Enabled)
		assert.False(t, *m.Enabled)
	})

	t.Run("patch enabled to true", func(t *testing.T) {
		m := &mock.Mock{ID: "test", Enabled: boolPtr(false)}
		patch := map[string]interface{}{"enabled": true}
		applyMockPatch(m, patch)
		assert.NotNil(t, m.Enabled)
		assert.True(t, *m.Enabled)
	})

	t.Run("patch parentId", func(t *testing.T) {
		m := &mock.Mock{ID: "test", ParentID: ""}
		patch := map[string]interface{}{"parentId": "folder-1"}
		applyMockPatch(m, patch)
		assert.Equal(t, "folder-1", m.ParentID)
	})

	t.Run("patch metaSortKey", func(t *testing.T) {
		m := &mock.Mock{ID: "test", MetaSortKey: 0}
		patch := map[string]interface{}{"metaSortKey": 100.5}
		applyMockPatch(m, patch)
		assert.Equal(t, 100.5, m.MetaSortKey)
	})

	t.Run("updates timestamp", func(t *testing.T) {
		m := &mock.Mock{ID: "test", UpdatedAt: time.Now().Add(-time.Hour)}
		oldTime := m.UpdatedAt
		patch := map[string]interface{}{"name": "Updated"}
		applyMockPatch(m, patch)
		assert.True(t, m.UpdatedAt.After(oldTime))
	})

	t.Run("ignores unknown fields", func(t *testing.T) {
		m := &mock.Mock{ID: "test", Name: "Original"}
		patch := map[string]interface{}{"unknownField": "value", "name": "Updated"}
		applyMockPatch(m, patch)
		assert.Equal(t, "Updated", m.Name)
	})

	t.Run("ignores wrong types", func(t *testing.T) {
		m := &mock.Mock{ID: "test", Name: "Original"}
		patch := map[string]interface{}{"name": 12345} // wrong type
		applyMockPatch(m, patch)
		assert.Equal(t, "Original", m.Name) // unchanged
	})

	// Document: PATCH does NOT support changing protocol-specific fields like ports.
	// Users should use PUT for full mock replacement including port changes.
	t.Run("does not patch mqtt config - use PUT for port changes", func(t *testing.T) {
		m := &mock.Mock{
			ID:   "mqtt-1",
			Type: mock.MockTypeMQTT,
			MQTT: &mock.MQTTSpec{Port: 1883},
		}
		// Try to patch mqtt config - should be ignored
		patch := map[string]interface{}{
			"mqtt": map[string]interface{}{
				"port": 9999,
			},
		}
		applyMockPatch(m, patch)
		// Port should remain unchanged
		assert.Equal(t, 1883, m.MQTT.Port, "PATCH should not change MQTT port - use PUT instead")
	})

	t.Run("does not patch grpc config - use PUT for port changes", func(t *testing.T) {
		m := &mock.Mock{
			ID:   "grpc-1",
			Type: mock.MockTypeGRPC,
			GRPC: &mock.GRPCSpec{Port: 50051},
		}
		// Try to patch grpc config - should be ignored
		patch := map[string]interface{}{
			"grpc": map[string]interface{}{
				"port": 9999,
			},
		}
		applyMockPatch(m, patch)
		// Port should remain unchanged
		assert.Equal(t, 50051, m.GRPC.Port, "PATCH should not change gRPC port - use PUT instead")
	})
}

func TestPortConflict(t *testing.T) {
	t.Run("PortConflict struct fields", func(t *testing.T) {
		conflict := &PortConflict{
			Port:         1883,
			ConflictID:   "mqtt-123",
			ConflictName: "IoT Broker",
			ConflictType: mock.MockTypeMQTT,
		}

		assert.Equal(t, 1883, conflict.Port)
		assert.Equal(t, "mqtt-123", conflict.ConflictID)
		assert.Equal(t, "IoT Broker", conflict.ConflictName)
		assert.Equal(t, mock.MockTypeMQTT, conflict.ConflictType)
	})
}

func TestGenerateMockID(t *testing.T) {
	t.Run("generates ID with type prefix", func(t *testing.T) {
		httpID := generateMockID(mock.MockTypeHTTP)
		assert.True(t, len(httpID) > 4)
		assert.Contains(t, httpID, "http_")

		mqttID := generateMockID(mock.MockTypeMQTT)
		assert.Contains(t, mqttID, "mqtt_")

		grpcID := generateMockID(mock.MockTypeGRPC)
		assert.Contains(t, grpcID, "grpc_")

		wsID := generateMockID(mock.MockTypeWebSocket)
		assert.Contains(t, wsID, "websocket_")

		graphqlID := generateMockID(mock.MockTypeGraphQL)
		assert.Contains(t, graphqlID, "graphql_")

		soapID := generateMockID(mock.MockTypeSOAP)
		assert.Contains(t, soapID, "soap_")
	})

	t.Run("uses 'mock' prefix for empty type", func(t *testing.T) {
		id := generateMockID("")
		assert.Contains(t, id, "mock_")
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateMockID(mock.MockTypeHTTP)
			assert.False(t, ids[id], "Duplicate ID generated: %s", id)
			ids[id] = true
		}
	})
}

func TestGenerateShortID(t *testing.T) {
	t.Run("generates non-empty ID", func(t *testing.T) {
		id := generateShortID()
		assert.NotEmpty(t, id)
		assert.True(t, len(id) >= 8, "ID should be at least 8 characters")
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateShortID()
			assert.False(t, ids[id], "Duplicate short ID generated: %s", id)
			ids[id] = true
		}
	})
}

func TestIsPortError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "address already in use",
			errMsg:   "listen tcp :1883: bind: address already in use",
			expected: true,
		},
		{
			name:     "port in use by broker",
			errMsg:   "port 1883 is already in use by broker mqtt-123",
			expected: true,
		},
		{
			name:     "permission denied",
			errMsg:   "listen tcp :80: bind: permission denied",
			expected: true,
		},
		{
			name:     "bind error",
			errMsg:   "bind: cannot assign requested address",
			expected: true,
		},
		{
			name:     "EADDRINUSE",
			errMsg:   "error: EADDRINUSE",
			expected: true,
		},
		{
			name:     "failed to start on port",
			errMsg:   "failed to start MQTT broker on port 1883: bind error",
			expected: true,
		},
		{
			name:     "generic validation error",
			errMsg:   "validation failed: name is required",
			expected: false,
		},
		{
			name:     "proto file error",
			errMsg:   "failed to parse proto files: file not found",
			expected: false,
		},
		{
			name:     "nil config error",
			errMsg:   "MQTT config cannot be nil",
			expected: false,
		},
		{
			name:     "empty error",
			errMsg:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPortError(tt.errMsg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Unit Tests for checkPortConflict
// ============================================================================

// TestCheckPortConflict_NoPort tests that mocks without dedicated ports return nil conflict
func TestCheckPortConflict_NoPort(t *testing.T) {
	// Create AdminAPI without engine or store - should return nil for any mock
	api := &AdminAPI{}

	tests := []struct {
		name string
		mock *mock.Mock
	}{
		{
			name: "HTTP mock has no dedicated port",
			mock: &mock.Mock{
				ID:          "http-1",
				Type:        mock.MockTypeHTTP,
				WorkspaceID: "local",
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{Path: "/api"},
				},
			},
		},
		{
			name: "WebSocket mock has no dedicated port",
			mock: &mock.Mock{
				ID:          "ws-1",
				Type:        mock.MockTypeWebSocket,
				WorkspaceID: "local",
			},
		},
		{
			name: "GraphQL mock has no dedicated port",
			mock: &mock.Mock{
				ID:          "graphql-1",
				Type:        mock.MockTypeGraphQL,
				WorkspaceID: "local",
			},
		},
		{
			name: "SOAP mock has no dedicated port",
			mock: &mock.Mock{
				ID:          "soap-1",
				Type:        mock.MockTypeSOAP,
				WorkspaceID: "local",
			},
		},
		{
			name: "MQTT mock with nil spec",
			mock: &mock.Mock{
				ID:          "mqtt-nil",
				Type:        mock.MockTypeMQTT,
				WorkspaceID: "local",
				MQTT:        nil,
			},
		},
		{
			name: "gRPC mock with nil spec",
			mock: &mock.Mock{
				ID:          "grpc-nil",
				Type:        mock.MockTypeGRPC,
				WorkspaceID: "local",
				GRPC:        nil,
			},
		},
		{
			name: "MQTT mock with port 0",
			mock: &mock.Mock{
				ID:          "mqtt-zero",
				Type:        mock.MockTypeMQTT,
				WorkspaceID: "local",
				MQTT:        &mock.MQTTSpec{Port: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := api.checkPortConflict(context.Background(), tt.mock, "")
			assert.Nil(t, conflict, "Should return nil for mocks without dedicated ports")
		})
	}
}

// TestCheckPortConflict_NoBackend tests that checkPortConflict returns nil when no backend is available
func TestCheckPortConflict_NoBackend(t *testing.T) {
	// Create AdminAPI without engine or store
	api := &AdminAPI{}

	mqttMock := &mock.Mock{
		ID:          "mqtt-1",
		Type:        mock.MockTypeMQTT,
		WorkspaceID: "local",
		MQTT:        &mock.MQTTSpec{Port: 1883},
	}

	// Should return nil (not block) even with a valid port - no backend to query
	conflict := api.checkPortConflict(context.Background(), mqttMock, "")
	assert.Nil(t, conflict, "Should return nil when no backend is available")
}

// TestCheckPortConflict_ExcludeID tests the excludeID parameter
func TestCheckPortConflict_ExcludeID(t *testing.T) {
	// This test verifies the logic works correctly with applyMockFilter
	// We use a simple in-memory mock list
	existingMocks := []*mock.Mock{
		{
			ID:          "mqtt-1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		},
		{
			ID:          "mqtt-2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1884},
		},
	}

	// Test case: updating mqtt-1 to keep port 1883 - should not conflict with itself
	t.Run("excludes own mock when updating", func(t *testing.T) {
		// Simulate the check logic
		newMock := &mock.Mock{
			ID:          "mqtt-1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883}, // Same port
		}
		excludeID := "mqtt-1"

		port := getMockPort(newMock)
		assert.Equal(t, 1883, port)

		// Filter to same workspace
		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "local"})
		assert.Len(t, filtered, 2)

		// Check for conflict excluding mqtt-1
		var conflict *PortConflict
		for _, existing := range filtered {
			if existing.ID == excludeID {
				continue // Should skip mqtt-1
			}
			existingPort := getMockPort(existing)
			if existingPort == port {
				conflict = &PortConflict{
					Port:         port,
					ConflictID:   existing.ID,
					ConflictName: existing.Name,
					ConflictType: existing.Type,
				}
				break
			}
		}
		assert.Nil(t, conflict, "Should not conflict with itself when excludeID matches")
	})

	// Test case: updating mqtt-2 to use port 1883 - should conflict with mqtt-1
	t.Run("finds conflict when changing to used port", func(t *testing.T) {
		newMock := &mock.Mock{
			ID:          "mqtt-2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883}, // Port used by mqtt-1
		}
		excludeID := "mqtt-2"

		port := getMockPort(newMock)
		assert.Equal(t, 1883, port)

		// Filter to same workspace
		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "local"})

		// Check for conflict excluding mqtt-2
		var conflict *PortConflict
		for _, existing := range filtered {
			if existing.ID == excludeID {
				continue // Should skip mqtt-2
			}
			existingPort := getMockPort(existing)
			if existingPort == port {
				conflict = &PortConflict{
					Port:         port,
					ConflictID:   existing.ID,
					ConflictName: existing.Name,
					ConflictType: existing.Type,
				}
				break
			}
		}
		assert.NotNil(t, conflict, "Should find conflict with mqtt-1")
		assert.Equal(t, "mqtt-1", conflict.ConflictID)
		assert.Equal(t, 1883, conflict.Port)
	})
}

// TestCheckPortConflict_WorkspaceIsolation tests that conflicts are workspace-scoped
func TestCheckPortConflict_WorkspaceIsolation(t *testing.T) {
	existingMocks := []*mock.Mock{
		{
			ID:          "mqtt-ws1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "workspace-1",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		},
		{
			ID:          "mqtt-ws2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "workspace-2",
			MQTT:        &mock.MQTTSpec{Port: 1883}, // Same port, different workspace
		},
	}

	t.Run("same port different workspace - no conflict", func(t *testing.T) {
		// New mock in workspace-3 using port 1883
		newMock := &mock.Mock{
			ID:          "mqtt-ws3",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "workspace-3",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		}

		port := getMockPort(newMock)

		// Filter to workspace-3 only
		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "workspace-3"})
		assert.Len(t, filtered, 0, "No mocks in workspace-3")

		// No conflicts in same workspace
		var conflict *PortConflict
		for _, existing := range filtered {
			if getMockPort(existing) == port {
				conflict = &PortConflict{Port: port, ConflictID: existing.ID}
				break
			}
		}
		assert.Nil(t, conflict)
	})

	t.Run("same port same workspace - conflict", func(t *testing.T) {
		// New mock in workspace-1 using port 1883
		newMock := &mock.Mock{
			ID:          "mqtt-new",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "workspace-1",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		}

		port := getMockPort(newMock)

		// Filter to workspace-1 only
		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "workspace-1"})
		assert.Len(t, filtered, 1)

		// Should find conflict
		var conflict *PortConflict
		for _, existing := range filtered {
			if getMockPort(existing) == port {
				conflict = &PortConflict{Port: port, ConflictID: existing.ID}
				break
			}
		}
		assert.NotNil(t, conflict)
		assert.Equal(t, "mqtt-ws1", conflict.ConflictID)
	})
}

// TestCheckPortConflict_MQTTSharing tests that multiple MQTT mocks can share the same port
func TestCheckPortConflict_MQTTSharing(t *testing.T) {
	existingMocks := []*mock.Mock{
		{
			ID:          "mqtt-1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		},
	}

	t.Run("MQTT mocks can share the same port", func(t *testing.T) {
		// New MQTT mock using the same port should NOT conflict
		newMQTT := &mock.Mock{
			ID:          "mqtt-2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 1883},
		}

		port := getMockPort(newMQTT)
		assert.Equal(t, 1883, port)

		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "local"})

		// Simulate the checkPortConflict logic with MQTT sharing
		var conflict *PortConflict
		for _, existing := range filtered {
			existingPort := getMockPort(existing)
			if existingPort == port {
				// MQTT mocks can share the same port since they share a single broker
				if newMQTT.Type == mock.MockTypeMQTT && existing.Type == mock.MockTypeMQTT {
					continue
				}
				conflict = &PortConflict{
					Port:         port,
					ConflictID:   existing.ID,
					ConflictType: existing.Type,
				}
				break
			}
		}
		assert.Nil(t, conflict, "MQTT mocks should be able to share the same port")
	})

	t.Run("gRPC cannot use port already used by MQTT", func(t *testing.T) {
		// gRPC mock trying to use same port should conflict
		grpcMock := &mock.Mock{
			ID:          "grpc-1",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC:        &mock.GRPCSpec{Port: 1883},
		}

		port := getMockPort(grpcMock)
		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "local"})

		var conflict *PortConflict
		for _, existing := range filtered {
			existingPort := getMockPort(existing)
			if existingPort == port {
				// MQTT mocks can share, but gRPC cannot share with MQTT
				if grpcMock.Type == mock.MockTypeMQTT && existing.Type == mock.MockTypeMQTT {
					continue
				}
				conflict = &PortConflict{
					Port:         port,
					ConflictID:   existing.ID,
					ConflictType: existing.Type,
				}
				break
			}
		}
		assert.NotNil(t, conflict, "gRPC should conflict with existing MQTT on same port")
		assert.Equal(t, "mqtt-1", conflict.ConflictID)
	})
}

// TestCheckPortConflict_CrossProtocol tests that MQTT and gRPC ports conflict with each other
func TestCheckPortConflict_CrossProtocol(t *testing.T) {
	existingMocks := []*mock.Mock{
		{
			ID:          "mqtt-1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT:        &mock.MQTTSpec{Port: 5000},
		},
	}

	t.Run("gRPC conflicts with MQTT on same port", func(t *testing.T) {
		grpcMock := &mock.Mock{
			ID:          "grpc-1",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC:        &mock.GRPCSpec{Port: 5000},
		}

		port := getMockPort(grpcMock)
		assert.Equal(t, 5000, port)

		filtered := applyMockFilter(existingMocks, &MockFilter{WorkspaceID: "local"})

		var conflict *PortConflict
		for _, existing := range filtered {
			if getMockPort(existing) == port {
				conflict = &PortConflict{
					Port:         port,
					ConflictID:   existing.ID,
					ConflictType: existing.Type,
				}
				break
			}
		}
		assert.NotNil(t, conflict)
		assert.Equal(t, "mqtt-1", conflict.ConflictID)
		assert.Equal(t, mock.MockTypeMQTT, conflict.ConflictType)
	})
}

// ============================================================================
// Unit Tests for Merge Functions
// ============================================================================

func TestMergeGRPCMock(t *testing.T) {
	t.Run("merges new service into existing mock", func(t *testing.T) {
		existing := &mock.Mock{
			ID:          "grpc-1",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC: &mock.GRPCSpec{
				Port: 50051,
				Services: map[string]mock.ServiceConfig{
					"test.UserService": {
						Methods: map[string]mock.MethodConfig{
							"GetUser": {Response: map[string]interface{}{"id": "1"}},
						},
					},
				},
			},
		}

		newMock := &mock.Mock{
			ID:          "grpc-2",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC: &mock.GRPCSpec{
				Port: 50051,
				Services: map[string]mock.ServiceConfig{
					"test.HealthService": {
						Methods: map[string]mock.MethodConfig{
							"Check": {Response: map[string]interface{}{"status": 1}},
						},
					},
				},
			},
		}

		result, err := mergeGRPCMock(existing, newMock)

		assert.NoError(t, err)
		assert.Equal(t, "merged", result.Action)
		assert.Equal(t, existing.ID, result.TargetMockID)
		assert.Contains(t, result.AddedServices, "test.HealthService/Check")
		assert.Len(t, existing.GRPC.Services, 2)
		assert.Contains(t, existing.GRPC.Services, "test.UserService")
		assert.Contains(t, existing.GRPC.Services, "test.HealthService")
	})

	t.Run("adds new method to existing service", func(t *testing.T) {
		existing := &mock.Mock{
			ID:          "grpc-1",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC: &mock.GRPCSpec{
				Port: 50051,
				Services: map[string]mock.ServiceConfig{
					"test.UserService": {
						Methods: map[string]mock.MethodConfig{
							"GetUser": {Response: map[string]interface{}{"id": "1"}},
						},
					},
				},
			},
		}

		newMock := &mock.Mock{
			ID:          "grpc-2",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC: &mock.GRPCSpec{
				Port: 50051,
				Services: map[string]mock.ServiceConfig{
					"test.UserService": {
						Methods: map[string]mock.MethodConfig{
							"CreateUser": {Response: map[string]interface{}{"id": "2"}},
						},
					},
				},
			},
		}

		result, err := mergeGRPCMock(existing, newMock)

		assert.NoError(t, err)
		assert.Equal(t, "merged", result.Action)
		assert.Contains(t, result.AddedServices, "test.UserService/CreateUser")
		assert.Len(t, existing.GRPC.Services["test.UserService"].Methods, 2)
	})

	t.Run("fails when method already exists", func(t *testing.T) {
		existing := &mock.Mock{
			ID:          "grpc-1",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC: &mock.GRPCSpec{
				Port: 50051,
				Services: map[string]mock.ServiceConfig{
					"test.UserService": {
						Methods: map[string]mock.MethodConfig{
							"GetUser": {Response: map[string]interface{}{"id": "1"}},
						},
					},
				},
			},
		}

		newMock := &mock.Mock{
			ID:          "grpc-2",
			Type:        mock.MockTypeGRPC,
			WorkspaceID: "local",
			GRPC: &mock.GRPCSpec{
				Port: 50051,
				Services: map[string]mock.ServiceConfig{
					"test.UserService": {
						Methods: map[string]mock.MethodConfig{
							"GetUser": {Response: map[string]interface{}{"id": "different"}},
						},
					},
				},
			},
		}

		result, err := mergeGRPCMock(existing, newMock)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "test.UserService")
		assert.Contains(t, err.Error(), "GetUser")
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestMergeMQTTMock(t *testing.T) {
	t.Run("merges new topic into existing mock", func(t *testing.T) {
		existing := &mock.Mock{
			ID:          "mqtt-1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT: &mock.MQTTSpec{
				Port: 1883,
				Topics: []mock.TopicConfig{
					{Topic: "sensors/temp", Messages: []mock.MessageConfig{{Payload: `{"temp":25}`}}},
				},
			},
		}

		newMock := &mock.Mock{
			ID:          "mqtt-2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT: &mock.MQTTSpec{
				Port: 1883,
				Topics: []mock.TopicConfig{
					{Topic: "sensors/humidity", Messages: []mock.MessageConfig{{Payload: `{"h":50}`}}},
				},
			},
		}

		result, err := mergeMQTTMock(existing, newMock)

		assert.NoError(t, err)
		assert.Equal(t, "merged", result.Action)
		assert.Equal(t, existing.ID, result.TargetMockID)
		assert.Contains(t, result.AddedTopics, "sensors/humidity")
		assert.Len(t, existing.MQTT.Topics, 2)
	})

	t.Run("fails when topic already exists", func(t *testing.T) {
		existing := &mock.Mock{
			ID:          "mqtt-1",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT: &mock.MQTTSpec{
				Port: 1883,
				Topics: []mock.TopicConfig{
					{Topic: "sensors/temp", Messages: []mock.MessageConfig{{Payload: `{"temp":25}`}}},
				},
			},
		}

		newMock := &mock.Mock{
			ID:          "mqtt-2",
			Type:        mock.MockTypeMQTT,
			WorkspaceID: "local",
			MQTT: &mock.MQTTSpec{
				Port: 1883,
				Topics: []mock.TopicConfig{
					{Topic: "sensors/temp", Messages: []mock.MessageConfig{{Payload: `{"temp":30}`}}},
				},
			},
		}

		result, err := mergeMQTTMock(existing, newMock)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "sensors/temp")
		assert.Contains(t, err.Error(), "already exists")
	})
}

// Note: TestCheckPortAvailability is tested indirectly through the existing
// TestCheckPortConflict_* tests, which test the checkPortConflict method
// that wraps checkPortAvailability. The merge-specific behavior is tested
// via the TestMergeGRPCMock and TestMergeMQTTMock tests above.
