package admin

import (
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

func TestSlugifyWorkspaceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "Payment API", "payment-api"},
		{"version suffix", "Users Service v2", "users-service-v2"},
		{"already slugged", "payment-api", "payment-api"},
		{"single word", "Default", "default"},
		{"extra spaces", "  My  Workspace  ", "my-workspace"},
		{"underscores", "my_workspace_test", "my-workspace-test"},
		{"dots", "api.v2.staging", "api-v2-staging"},
		{"mixed separators", "My_Cool.API v3", "my-cool-api-v3"},
		{"numbers only", "123", "123"},
		{"special chars", "Hello@World!", "helloworld"},
		{"unicode", "Ünïcödë", "ünïcödë"},
		{"empty", "", ""},
		{"only spaces", "   ", ""},
		{"trailing dash", "foo-", "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlugifyWorkspaceName(tt.input)
			if got != tt.expected {
				t.Errorf("SlugifyWorkspaceName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestWorkspaceBasePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		expected string
	}{
		{"empty (root)", "", ""},
		{"with leading slash", "/payment-api", "/payment-api"},
		{"without leading slash", "payment-api", "/payment-api"},
		{"nested", "/api/v2", "/api/v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &store.Workspace{BasePath: tt.basePath}
			got := WorkspaceBasePath(ws)
			if got != tt.expected {
				t.Errorf("WorkspaceBasePath(%q) = %q, want %q", tt.basePath, got, tt.expected)
			}
		})
	}
}

func TestEffectiveMockPath(t *testing.T) {
	rootWS := &store.Workspace{ID: "local", BasePath: ""}
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}
	usersWS := &store.Workspace{ID: "ws_usr", BasePath: "users"} // no leading slash

	tests := []struct {
		name            string
		mockPath        string
		workspaceID     string
		ws              *store.Workspace
		rootWorkspaceID string
		expected        string
	}{
		{"root workspace - no prefix", "/payments/charge", "local", rootWS, "local", "/payments/charge"},
		{"non-root - gets prefix", "/payments/charge", "ws_pay", paymentWS, "local", "/payment-api/payments/charge"},
		{"non-root is root on this engine", "/payments/charge", "ws_pay", paymentWS, "ws_pay", "/payments/charge"},
		{"no leading slash on basePath", "/api/users", "ws_usr", usersWS, "local", "/users/api/users"},
		{"root path", "/", "ws_pay", paymentWS, "local", "/payment-api/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveMockPath(tt.mockPath, tt.workspaceID, tt.ws, tt.rootWorkspaceID)
			if got != tt.expected {
				t.Errorf("effectiveMockPath(%q, %q, ws{%q}, %q) = %q, want %q",
					tt.mockPath, tt.workspaceID, tt.ws.BasePath, tt.rootWorkspaceID, got, tt.expected)
			}
		})
	}
}

func TestPrefixMockForEngine_HTTP(t *testing.T) {
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}

	original := &mock.Mock{
		ID:          "mock_1",
		Type:        mock.TypeHTTP,
		WorkspaceID: "ws_pay",
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/payments/charge",
			},
		},
	}

	// Non-root workspace → should prefix
	result := prefixMockForEngine(original, paymentWS, "local")

	if result.HTTP.Matcher.Path != "/payment-api/payments/charge" {
		t.Errorf("expected prefixed path, got %q", result.HTTP.Matcher.Path)
	}
	// Original should be unmodified
	if original.HTTP.Matcher.Path != "/payments/charge" {
		t.Errorf("original mock was mutated: %q", original.HTTP.Matcher.Path)
	}
	// ID should be the same
	if result.ID != original.ID {
		t.Errorf("mock ID changed: %q → %q", original.ID, result.ID)
	}
}

func TestPrefixMockForEngine_RootWorkspace(t *testing.T) {
	rootWS := &store.Workspace{ID: "local", BasePath: ""}

	original := &mock.Mock{
		ID:          "mock_1",
		Type:        mock.TypeHTTP,
		WorkspaceID: "local",
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
		},
	}

	// Root workspace → no prefix
	result := prefixMockForEngine(original, rootWS, "local")

	if result.HTTP.Matcher.Path != "/api/users" {
		t.Errorf("root workspace mock should not be prefixed, got %q", result.HTTP.Matcher.Path)
	}
	// Should return the SAME pointer (no clone needed)
	if result != original {
		t.Errorf("root workspace mock should return same pointer (no clone)")
	}
}

func TestPrefixMockForEngine_NonRootIsRootOnEngine(t *testing.T) {
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}

	original := &mock.Mock{
		ID:          "mock_1",
		Type:        mock.TypeHTTP,
		WorkspaceID: "ws_pay",
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/charge",
			},
		},
	}

	// ws_pay is root on THIS engine → no prefix
	result := prefixMockForEngine(original, paymentWS, "ws_pay")

	if result.HTTP.Matcher.Path != "/charge" {
		t.Errorf("mock from root workspace should not be prefixed, got %q", result.HTTP.Matcher.Path)
	}
}

func TestPrefixMockForEngine_WebSocket(t *testing.T) {
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}

	original := &mock.Mock{
		ID:          "mock_ws",
		Type:        mock.TypeWebSocket,
		WorkspaceID: "ws_pay",
		WebSocket: &mock.WebSocketSpec{
			Path: "/ws/events",
		},
	}

	result := prefixMockForEngine(original, paymentWS, "local")

	if result.WebSocket.Path != "/payment-api/ws/events" {
		t.Errorf("WebSocket path should be prefixed, got %q", result.WebSocket.Path)
	}
}

func TestPrefixMockForEngine_GRPC_NoPrefix(t *testing.T) {
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}

	original := &mock.Mock{
		ID:          "mock_grpc",
		Type:        mock.TypeGRPC,
		WorkspaceID: "ws_pay",
		GRPC: &mock.GRPCSpec{
			Port: 50051,
		},
	}

	// gRPC is port-based, should not be modified
	result := prefixMockForEngine(original, paymentWS, "local")

	if result != original {
		t.Errorf("gRPC mock should not be cloned/modified")
	}
}

func TestPrefixMockForEngine_MQTT_NoPrefix(t *testing.T) {
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}

	original := &mock.Mock{
		ID:          "mock_mqtt",
		Type:        mock.TypeMQTT,
		WorkspaceID: "ws_pay",
		MQTT: &mock.MQTTSpec{
			Port: 1883,
		},
	}

	result := prefixMockForEngine(original, paymentWS, "local")

	if result != original {
		t.Errorf("MQTT mock should not be cloned/modified")
	}
}

func TestPrefixMockForEngine_PathPattern(t *testing.T) {
	paymentWS := &store.Workspace{ID: "ws_pay", BasePath: "/payment-api"}

	original := &mock.Mock{
		ID:          "mock_regex",
		Type:        mock.TypeHTTP,
		WorkspaceID: "ws_pay",
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				PathPattern: "^/payments/.*",
			},
		},
	}

	result := prefixMockForEngine(original, paymentWS, "local")

	expected := "^/payment-api/payments/.*"
	if result.HTTP.Matcher.PathPattern != expected {
		t.Errorf("PathPattern should be prefixed, got %q, want %q", result.HTTP.Matcher.PathPattern, expected)
	}
}

func TestPrefixMocksForEngine(t *testing.T) {
	wsMap := map[string]*store.Workspace{
		"local":  {ID: "local", BasePath: ""},
		"ws_pay": {ID: "ws_pay", BasePath: "/payment-api"},
	}

	mocks := []*mock.Mock{
		{
			ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Path: "/health"}},
		},
		{
			ID: "m2", Type: mock.TypeHTTP, WorkspaceID: "ws_pay",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Path: "/charge"}},
		},
	}

	result := prefixMocksForEngine(mocks, wsMap, "local")

	if len(result) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(result))
	}
	if result[0].HTTP.Matcher.Path != "/health" {
		t.Errorf("root workspace mock path should be unchanged, got %q", result[0].HTTP.Matcher.Path)
	}
	if result[1].HTTP.Matcher.Path != "/payment-api/charge" {
		t.Errorf("non-root mock path should be prefixed, got %q", result[1].HTTP.Matcher.Path)
	}
	// Originals should be unmodified
	if mocks[1].HTTP.Matcher.Path != "/charge" {
		t.Errorf("original mock was mutated")
	}
}

func TestCheckRouteCollision(t *testing.T) {
	wsMap := map[string]*store.Workspace{
		"local":  {ID: "local", Name: "Default", BasePath: ""},
		"ws_pay": {ID: "ws_pay", Name: "Payment API", BasePath: "/payment-api"},
	}

	existing := []*mock.Mock{
		{
			ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/status"}},
		},
		{
			ID: "m2", Type: mock.TypeHTTP, WorkspaceID: "ws_pay",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/charge"}},
		},
	}

	t.Run("collision: default /payment-api/status vs payment-api /status", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "ws_pay",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/status"}},
		}
		collision := checkRouteCollision(newMock, wsMap["ws_pay"], existing, wsMap, "local")
		if collision == nil {
			t.Fatal("expected collision, got nil")
		}
		if collision.ExistingMockID != "m1" {
			t.Errorf("collision should be with m1, got %s", collision.ExistingMockID)
		}
		if collision.EffectivePath != "/payment-api/status" {
			t.Errorf("effective path should be /payment-api/status, got %s", collision.EffectivePath)
		}
	})

	t.Run("no collision: different methods", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "ws_pay",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/status"}},
		}
		collision := checkRouteCollision(newMock, wsMap["ws_pay"], existing, wsMap, "local")
		if collision != nil {
			t.Errorf("expected no collision (different methods), got %+v", collision)
		}
	})

	t.Run("no collision: different effective paths", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "ws_pay",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/health"}},
		}
		collision := checkRouteCollision(newMock, wsMap["ws_pay"], existing, wsMap, "local")
		if collision != nil {
			t.Errorf("expected no collision, got %+v", collision)
		}
	})

	t.Run("skip self on update", func(t *testing.T) {
		// Simulating an update: same ID as existing mock
		newMock := &mock.Mock{
			ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/status"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], existing, wsMap, "local")
		if collision != nil {
			t.Errorf("should skip self, got collision with %s", collision.ExistingMockID)
		}
	})

	t.Run("gRPC mock skipped", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_grpc", Type: mock.TypeGRPC, WorkspaceID: "ws_pay",
			GRPC: &mock.GRPCSpec{Port: 50051},
		}
		collision := checkRouteCollision(newMock, wsMap["ws_pay"], existing, wsMap, "local")
		if collision != nil {
			t.Errorf("gRPC should never collide on path, got %+v", collision)
		}
	})
}

func TestValidateBasePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"valid", "/payment-api", "/payment-api"},
		{"no leading slash", "payment-api", "/payment-api"},
		{"trailing slash", "/payment-api/", "/payment-api"},
		{"double slash", "/payment//api", "/payment/api"},
		{"query string rejected", "/api?foo=bar", ""},
		{"fragment rejected", "/api#section", ""},
		{"spaces trimmed", "  /api  ", "/api"},
		{"nested path", "/api/v2/payments", "/api/v2/payments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateBasePath(tt.input)
			if got != tt.expected {
				t.Errorf("validateBasePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
