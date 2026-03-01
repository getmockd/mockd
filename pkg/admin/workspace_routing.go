package admin

import (
	"strings"
	"unicode"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// SlugifyWorkspaceName converts a workspace name to a URL-safe base path slug.
// "Payment API" → "payment-api", "Users Service v2" → "users-service-v2"
// The result does NOT include a leading slash (callers prepend "/" as needed).
func SlugifyWorkspaceName(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		case r == ' ' || r == '_' || r == '-' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	s := b.String()
	return strings.TrimRight(s, "-")
}

// WorkspaceBasePath returns the BasePath to use for a workspace, ensuring it
// has a leading "/" if non-empty.
func WorkspaceBasePath(ws *store.Workspace) string {
	if ws.BasePath == "" {
		return ""
	}
	if !strings.HasPrefix(ws.BasePath, "/") {
		return "/" + ws.BasePath
	}
	return ws.BasePath
}

// effectiveMockPath returns the engine-visible path for a mock, given the
// workspace it belongs to and the engine's root workspace ID.
//
// If the mock's workspace IS the engine's root workspace, the path is unchanged.
// Otherwise, the workspace's BasePath is prepended.
func effectiveMockPath(mockPath, workspaceID string, ws *store.Workspace, rootWorkspaceID string) string {
	if workspaceID == rootWorkspaceID {
		return mockPath
	}
	basePath := WorkspaceBasePath(ws)
	if basePath == "" {
		return mockPath
	}
	// Ensure no double slash: basePath="/payment-api", mockPath="/charge" → "/payment-api/charge"
	if strings.HasSuffix(basePath, "/") && strings.HasPrefix(mockPath, "/") {
		return basePath + mockPath[1:]
	}
	return basePath + mockPath
}

// prefixMockForEngine clones a mock and rewrites its path for the given engine.
// Returns the original mock unmodified if no prefixing is needed.
// Only path-based protocols are prefixed: HTTP, WebSocket, GraphQL, SOAP.
// Port-based protocols (gRPC, MQTT) and OAuth are left unchanged.
func prefixMockForEngine(m *mock.Mock, ws *store.Workspace, rootWorkspaceID string) *mock.Mock {
	if ws == nil || m.WorkspaceID == rootWorkspaceID || WorkspaceBasePath(ws) == "" {
		return m
	}

	// Only path-based protocols need prefixing
	switch m.Type {
	case mock.TypeHTTP, mock.TypeWebSocket, mock.TypeGraphQL, mock.TypeSOAP:
		// continue below
	default:
		// gRPC (port-based), MQTT (topic-based), OAuth (issuer-based) — no prefixing
		return m
	}

	// Clone the mock so we don't mutate the admin store's copy.
	cloned := cloneMockShallow(m)

	switch cloned.Type {
	case mock.TypeHTTP:
		if cloned.HTTP != nil && cloned.HTTP.Matcher != nil {
			matcher := *cloned.HTTP.Matcher
			if matcher.Path != "" {
				matcher.Path = effectiveMockPath(matcher.Path, m.WorkspaceID, ws, rootWorkspaceID)
			}
			if matcher.PathPattern != "" {
				// For regex patterns, prepend the base path as a literal prefix.
				// E.g., basePath="/payment-api", pattern="^/charge.*" → "^/payment-api/charge.*"
				bp := WorkspaceBasePath(ws)
				if strings.HasPrefix(matcher.PathPattern, "^") {
					matcher.PathPattern = "^" + bp + matcher.PathPattern[1:]
				} else {
					matcher.PathPattern = bp + matcher.PathPattern
				}
			}
			spec := *cloned.HTTP
			spec.Matcher = &matcher
			cloned.HTTP = &spec
		}

	case mock.TypeWebSocket:
		if cloned.WebSocket != nil && cloned.WebSocket.Path != "" {
			ws2 := *cloned.WebSocket
			ws2.Path = effectiveMockPath(ws2.Path, m.WorkspaceID, ws, rootWorkspaceID)
			cloned.WebSocket = &ws2
		}

	case mock.TypeGraphQL:
		if cloned.GraphQL != nil && cloned.GraphQL.Path != "" {
			gql := *cloned.GraphQL
			gql.Path = effectiveMockPath(gql.Path, m.WorkspaceID, ws, rootWorkspaceID)
			cloned.GraphQL = &gql
		}

	case mock.TypeSOAP:
		if cloned.SOAP != nil && cloned.SOAP.Path != "" {
			soap := *cloned.SOAP
			soap.Path = effectiveMockPath(soap.Path, m.WorkspaceID, ws, rootWorkspaceID)
			cloned.SOAP = &soap
		}
	}

	return cloned
}

// cloneMockShallow creates a shallow copy of a mock. The protocol-specific
// spec pointers still point to the same underlying data — callers that need
// to mutate a spec must copy that spec individually (which prefixMockForEngine does).
func cloneMockShallow(m *mock.Mock) *mock.Mock {
	cp := *m
	return &cp
}

// prefixMocksForEngine applies workspace base path prefixes to a slice of mocks
// for a specific engine target. Returns a new slice (original is not modified).
// workspaceMap maps workspace ID → *store.Workspace for efficient lookup.
func prefixMocksForEngine(mocks []*mock.Mock, workspaceMap map[string]*store.Workspace, rootWorkspaceID string) []*mock.Mock {
	result := make([]*mock.Mock, len(mocks))
	for i, m := range mocks {
		ws := workspaceMap[m.WorkspaceID]
		result[i] = prefixMockForEngine(m, ws, rootWorkspaceID)
	}
	return result
}

// buildWorkspaceMap creates a workspace ID → Workspace lookup map.
func buildWorkspaceMap(workspaces []*store.Workspace) map[string]*store.Workspace {
	m := make(map[string]*store.Workspace, len(workspaces))
	for _, ws := range workspaces {
		m[ws.ID] = ws
	}
	return m
}

// validateBasePath checks that a base path is valid:
//   - Must start with "/" if non-empty
//   - Must not contain query strings, fragments, or consecutive slashes
//   - Must not end with "/"
func validateBasePath(basePath string) string {
	if basePath == "" {
		return ""
	}

	bp := strings.TrimSpace(basePath)

	// Add leading slash if missing
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}

	// Strip trailing slash
	bp = strings.TrimRight(bp, "/")

	// Basic sanity: reject query strings and fragments
	if strings.ContainsAny(bp, "?#") {
		return ""
	}

	// Collapse consecutive slashes
	for strings.Contains(bp, "//") {
		bp = strings.ReplaceAll(bp, "//", "/")
	}

	return bp
}
