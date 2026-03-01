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

	t.Run("exact collision: same effective path across workspaces", func(t *testing.T) {
		// Root has /api/status, ws_pay has /status → effective /payment-api/status.
		// These differ. But if root has /payment-api/status and ws_pay creates /status,
		// both resolve to /payment-api/status.
		// Use mocks within the same workspace to test pure exact collision.
		sameWsMap := map[string]*store.Workspace{
			"local": {ID: "local", Name: "Default", BasePath: ""},
			"ws_a":  {ID: "ws_a", Name: "Service A", BasePath: "/svc-a"},
			"ws_b":  {ID: "ws_b", Name: "Service B", BasePath: "/svc-b"},
		}
		existing := []*mock.Mock{
			{
				ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "ws_a",
				HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/status"}},
			},
		}
		// ws_b mock whose effective path /svc-b/status differs from ws_a's /svc-a/status
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "ws_b",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/status"}},
		}
		collision := checkRouteCollision(newMock, sameWsMap["ws_b"], existing, sameWsMap, "local")
		if collision != nil {
			t.Errorf("different basePaths should not collide, got %+v", collision)
		}
	})

	t.Run("no collision: different methods", func(t *testing.T) {
		// Two non-root workspaces with identical paths but different methods
		sameWsMap := map[string]*store.Workspace{
			"local": {ID: "local", Name: "Default", BasePath: ""},
			"ws_a":  {ID: "ws_a", Name: "Service A", BasePath: "/svc-a"},
		}
		existing := []*mock.Mock{
			{
				ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
				HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/health"}},
			},
		}
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/health"}},
		}
		collision := checkRouteCollision(newMock, sameWsMap["local"], existing, sameWsMap, "local")
		if collision != nil {
			t.Errorf("expected no collision (different methods), got %+v", collision)
		}
	})

	t.Run("no collision: different effective paths", func(t *testing.T) {
		existing := []*mock.Mock{
			{
				ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
				HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/health"}},
			},
		}
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/ready"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], existing, wsMap, "local")
		if collision != nil {
			t.Errorf("expected no collision, got %+v", collision)
		}
	})

	t.Run("skip self on update", func(t *testing.T) {
		// Simulating an update: same ID as existing mock, path unchanged
		existing := []*mock.Mock{
			{
				ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
				HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/health"}},
			},
		}
		newMock := &mock.Mock{
			ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/health"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], existing, wsMap, "local")
		if collision != nil {
			t.Errorf("should skip self, got collision with %s", collision.ExistingMockID)
		}
	})

	t.Run("gRPC mock skipped", func(t *testing.T) {
		existing := []*mock.Mock{}
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

func TestCheckRouteCollision_NamespaceShadowing(t *testing.T) {
	wsMap := map[string]*store.Workspace{
		"local":  {ID: "local", Name: "Default", BasePath: ""},
		"ws_pay": {ID: "ws_pay", Name: "Payment API", BasePath: "/payment-api"},
		"ws_usr": {ID: "ws_usr", Name: "Users", BasePath: "/users"},
	}

	// No existing mocks — clean slate for Direction A tests
	noMocks := []*mock.Mock{}

	t.Run("root mock with named param invading namespace", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/{id}"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], noMocks, wsMap, "local")
		if collision == nil {
			t.Fatal("expected namespace collision for /payment-api/{id}, got nil")
		}
		if collision.WorkspaceID != "ws_pay" {
			t.Errorf("collision should reference ws_pay, got %s", collision.WorkspaceID)
		}
	})

	t.Run("root mock with wildcard invading namespace", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/*"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], noMocks, wsMap, "local")
		if collision == nil {
			t.Fatal("expected namespace collision for /payment-api/*, got nil")
		}
	})

	t.Run("root mock at exact basePath", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], noMocks, wsMap, "local")
		if collision == nil {
			t.Fatal("expected namespace collision for /payment-api (exact basePath), got nil")
		}
	})

	t.Run("root mock deep inside namespace", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/v2/charge"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], noMocks, wsMap, "local")
		if collision == nil {
			t.Fatal("expected namespace collision for /payment-api/v2/charge, got nil")
		}
	})

	t.Run("root mock with similar prefix is fine", func(t *testing.T) {
		// "/payment-apiv2/status" does NOT start with "/payment-api/"
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-apiv2/status"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], noMocks, wsMap, "local")
		if collision != nil {
			t.Errorf("expected no collision for /payment-apiv2/status, got %+v", collision)
		}
	})

	t.Run("root mock outside any namespace is fine", func(t *testing.T) {
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/health"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], noMocks, wsMap, "local")
		if collision != nil {
			t.Errorf("expected no collision for /health, got %+v", collision)
		}
	})

	t.Run("direction B: non-root mock blocked by existing root wildcard", func(t *testing.T) {
		// Root already has a wildcard covering the /users namespace
		existingWithRootWildcard := []*mock.Mock{
			{
				ID: "m_root_wc", Type: mock.TypeHTTP, WorkspaceID: "local",
				HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users/{id}"}},
			},
		}
		// Now try to create a mock in the users workspace
		newMock := &mock.Mock{
			ID: "m_new", Type: mock.TypeHTTP, WorkspaceID: "ws_usr",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/profile"}},
		}
		collision := checkRouteCollision(newMock, wsMap["ws_usr"], existingWithRootWildcard, wsMap, "local")
		if collision == nil {
			t.Fatal("expected namespace collision (direction B), got nil")
		}
		if collision.ExistingMockID != "m_root_wc" {
			t.Errorf("collision should reference m_root_wc, got %s", collision.ExistingMockID)
		}
	})

	t.Run("self-update at same path allowed despite namespace", func(t *testing.T) {
		// m1 already exists in root at /payment-api/status (legacy data).
		// Updating m1 with the same path should be allowed.
		existingWithInvader := []*mock.Mock{
			{
				ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
				HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/status"}},
			},
		}
		newMock := &mock.Mock{
			ID: "m1", Type: mock.TypeHTTP, WorkspaceID: "local",
			HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/payment-api/status"}},
		}
		collision := checkRouteCollision(newMock, wsMap["local"], existingWithInvader, wsMap, "local")
		if collision != nil {
			t.Errorf("self-update at same path should be allowed, got %+v", collision)
		}
	})
}

func TestPathInvades(t *testing.T) {
	tests := []struct {
		name          string
		effectivePath string
		basePath      string
		expected      bool
	}{
		{"exact basePath", "/payment-api", "/payment-api", true},
		{"sub-path", "/payment-api/status", "/payment-api", true},
		{"deep sub-path", "/payment-api/v2/charge", "/payment-api", true},
		{"wildcard", "/payment-api/*", "/payment-api", true},
		{"named param", "/payment-api/{id}", "/payment-api", true},
		{"similar prefix", "/payment-apiv2/status", "/payment-api", false},
		{"different path", "/health", "/payment-api", false},
		{"root path", "/", "/payment-api", false},
		{"partial overlap", "/payment", "/payment-api", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathInvades(tt.effectivePath, tt.basePath)
			if got != tt.expected {
				t.Errorf("pathInvades(%q, %q) = %v, want %v", tt.effectivePath, tt.basePath, got, tt.expected)
			}
		})
	}
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
