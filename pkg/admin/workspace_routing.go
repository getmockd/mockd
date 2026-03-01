package admin

import (
	"context"
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

// checkMockRouteCollision is an API-level convenience that loads all workspaces
// and mocks, then delegates to checkRouteCollision.
func (a *API) checkMockRouteCollision(ctx context.Context, m *mock.Mock) *RouteCollision {
	if a.workspaceStore == nil || a.dataStore == nil {
		return nil
	}
	wsList, err := a.workspaceStore.List(ctx)
	if err != nil {
		return nil
	}
	wsMap := buildWorkspaceMap(wsList)

	newWS := wsMap[m.WorkspaceID]
	if newWS == nil {
		// Unknown workspace — can't check, allow it through
		return nil
	}

	allMocks, err := a.dataStore.Mocks().List(ctx, nil)
	if err != nil {
		return nil
	}

	return checkRouteCollision(m, newWS, allMocks, wsMap, store.DefaultWorkspaceID)
}

// RouteCollision describes a conflict between two mocks that would resolve
// to the same method+path on the engine after workspace base path prefixing.
type RouteCollision struct {
	ExistingMockID   string `json:"existingMockId"`
	ExistingMockName string `json:"existingMockName,omitempty"`
	WorkspaceID      string `json:"workspaceId"`
	WorkspaceName    string `json:"workspaceName,omitempty"`
	EffectivePath    string `json:"effectivePath"`
	Method           string `json:"method"`
}

// checkRouteCollision checks if a new mock (with its workspace) would collide
// with any existing mock on the engine after base path prefixing.
//
// Two levels of protection:
//  1. Exact collision — two mocks resolve to the same method+effective path.
//  2. Namespace shadowing — a root workspace mock uses a path (including
//     wildcards or named params) that falls inside another workspace's
//     reserved basePath namespace, or vice-versa.
//
// Returns nil if no collision. Only checks HTTP mocks (path-based).
func checkRouteCollision(
	newMock *mock.Mock,
	newWorkspace *store.Workspace,
	existingMocks []*mock.Mock,
	workspaceMap map[string]*store.Workspace,
	rootWorkspaceID string,
) *RouteCollision {
	if newMock.Type != mock.TypeHTTP || newMock.HTTP == nil || newMock.HTTP.Matcher == nil {
		return nil // Only HTTP has method+path collisions
	}

	newMethod := strings.ToUpper(newMock.HTTP.Matcher.Method)
	newPath := newMock.HTTP.Matcher.Path
	if newPath == "" {
		return nil
	}

	// Compute the effective engine path for the new mock
	newEffective := effectiveMockPath(newPath, newMock.WorkspaceID, newWorkspace, rootWorkspaceID)

	// --- Pass 1: exact effective-path collision ---
	for _, existing := range existingMocks {
		if existing.ID == newMock.ID {
			continue // Skip self (for updates)
		}
		if existing.Type != mock.TypeHTTP || existing.HTTP == nil || existing.HTTP.Matcher == nil {
			continue
		}
		existMethod := strings.ToUpper(existing.HTTP.Matcher.Method)
		existPath := existing.HTTP.Matcher.Path
		if existPath == "" || existMethod != newMethod {
			continue
		}

		existWS := workspaceMap[existing.WorkspaceID]
		existEffective := effectiveMockPath(existPath, existing.WorkspaceID, existWS, rootWorkspaceID)

		if existEffective == newEffective {
			wsName := ""
			if existWS != nil {
				wsName = existWS.Name
			}
			return &RouteCollision{
				ExistingMockID:   existing.ID,
				ExistingMockName: existing.Name,
				WorkspaceID:      existing.WorkspaceID,
				WorkspaceName:    wsName,
				EffectivePath:    existEffective,
				Method:           existMethod,
			}
		}
	}

	// --- Pass 2: namespace shadowing ---
	//
	// A root-workspace mock whose effective path falls inside (or covers) a
	// non-root workspace's basePath namespace would silently shadow that
	// workspace's routes. Detect and reject.
	//
	// Skip this check if the mock already exists at the same effective path
	// (self-update with no path change). We find the existing effective path
	// by scanning the mock list for a match on ID.
	if newMock.ID != "" {
		for _, existing := range existingMocks {
			if existing.ID != newMock.ID {
				continue
			}
			if existing.Type != mock.TypeHTTP || existing.HTTP == nil || existing.HTTP.Matcher == nil {
				break
			}
			existWS := workspaceMap[existing.WorkspaceID]
			existEffective := effectiveMockPath(
				existing.HTTP.Matcher.Path, existing.WorkspaceID, existWS, rootWorkspaceID,
			)
			if existEffective == newEffective {
				// Path unchanged — allow the update without namespace re-check.
				return nil
			}
			break
		}
	}

	// Direction A: new mock is in root → check if its path invades any workspace's basePath.
	// Direction B: new mock is in a non-root workspace → check if any existing root
	//              mock's path invades the new workspace's basePath namespace.
	if newMock.WorkspaceID == rootWorkspaceID {
		// Direction A: root mock encroaching on a reserved namespace.
		for _, ws := range workspaceMap {
			bp := WorkspaceBasePath(ws)
			if bp == "" || ws.ID == rootWorkspaceID {
				continue
			}
			if pathInvades(newEffective, bp) {
				return &RouteCollision{
					WorkspaceID:   ws.ID,
					WorkspaceName: ws.Name,
					EffectivePath: newEffective,
					Method:        newMethod,
				}
			}
		}
	} else {
		// Direction B: non-root mock — check existing root mocks don't
		// already shadow our namespace.
		myBasePath := WorkspaceBasePath(newWorkspace)
		if myBasePath != "" {
			for _, existing := range existingMocks {
				if existing.ID == newMock.ID {
					continue
				}
				if existing.WorkspaceID != rootWorkspaceID {
					continue
				}
				if existing.Type != mock.TypeHTTP || existing.HTTP == nil || existing.HTTP.Matcher == nil {
					continue
				}
				existPath := existing.HTTP.Matcher.Path
				if existPath == "" {
					continue
				}
				existEffective := effectiveMockPath(existPath, existing.WorkspaceID, workspaceMap[existing.WorkspaceID], rootWorkspaceID)
				if pathInvades(existEffective, myBasePath) {
					return &RouteCollision{
						ExistingMockID:   existing.ID,
						ExistingMockName: existing.Name,
						WorkspaceID:      existing.WorkspaceID,
						WorkspaceName:    "Default",
						EffectivePath:    existEffective,
						Method:           strings.ToUpper(existing.HTTP.Matcher.Method),
					}
				}
			}
		}
	}

	return nil
}

// pathInvades checks whether an effective engine path falls inside a
// workspace's reserved basePath namespace. This catches:
//   - Exact basePath match: "/payment-api" invades "/payment-api"
//   - Literal sub-path:     "/payment-api/status" invades "/payment-api"
//   - Named params:         "/payment-api/{id}" invades "/payment-api"
//   - Wildcards:            "/payment-api/*" invades "/payment-api"
//   - Deeper nesting:       "/payment-api/v2/charge" invades "/payment-api"
//
// It works on the raw path string before the engine's matcher runs, so it
// correctly catches patterns like "{param}" and "*" that would match
// arbitrary values at runtime.
func pathInvades(effectivePath, basePath string) bool {
	return effectivePath == basePath ||
		strings.HasPrefix(effectivePath, basePath+"/")
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
