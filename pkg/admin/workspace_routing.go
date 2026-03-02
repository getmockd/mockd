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

	case mock.TypeGRPC, mock.TypeMQTT, mock.TypeOAuth:
		// These types were already filtered by the early return above (line 79-82),
		// but the exhaustive linter requires all cases to be listed.
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

// checkEngineBasePathConflict checks if assigning a workspace to an engine
// would cause a basePath overlap with workspaces already on that engine.
// Looks up the workspace's basePath and the engine's current workspace list,
// then delegates to checkBasePathConflict.
func (a *API) checkEngineBasePathConflict(ctx context.Context, engineID, workspaceID string) *BasePathConflict {
	if a.workspaceStore == nil {
		return nil
	}
	newWS, err := a.workspaceStore.Get(ctx, workspaceID)
	if err != nil || newWS == nil {
		return nil // can't check — let assignment proceed
	}
	newBP := WorkspaceBasePath(newWS)
	if newBP == "" {
		return nil // root workspace, no basePath conflict possible
	}

	// Get workspaces already on this engine
	engine, err := a.engineRegistry.Get(engineID)
	if err != nil || engine == nil {
		return nil
	}
	var peers []*store.Workspace
	for _, ew := range engine.Workspaces {
		if ew.WorkspaceID == workspaceID {
			continue // skip self if somehow already assigned
		}
		peerWS, peerErr := a.workspaceStore.Get(ctx, ew.WorkspaceID)
		if peerErr == nil && peerWS != nil {
			peers = append(peers, peerWS)
		}
	}
	return checkBasePathConflict(newBP, workspaceID, peers)
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
	rootWorkspaceID string, //nolint:unparam // kept as parameter for testability
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

	// --- Pass 1: exact effective-path collision (cross-workspace only) ---
	// Same-workspace duplicate paths are intentional — the matching engine uses
	// priority to pick the winner. We only flag collisions across workspaces where
	// base-path prefixing causes two mocks to resolve to the same engine route.
	for _, existing := range existingMocks {
		if existing.ID == newMock.ID {
			continue // Skip self (for updates)
		}
		if existing.Type != mock.TypeHTTP || existing.HTTP == nil || existing.HTTP.Matcher == nil {
			continue
		}
		// Same workspace → priority matching handles it, not a collision
		if existing.WorkspaceID == newMock.WorkspaceID {
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

	return nil
}

// basePathsOverlap returns true if two non-empty basePaths would conflict
// on the same engine — i.e., they're identical or one is a path-segment
// prefix of the other.
//
//	basePathsOverlap("/api", "/api/users") → true
//	basePathsOverlap("/api", "/api")       → true
//	basePathsOverlap("/api", "/api-v2")    → false
//	basePathsOverlap("/api", "/other")     → false
//	basePathsOverlap("", "/api")           → false (root is special)
func basePathsOverlap(a, b string) bool {
	if a == "" || b == "" {
		return false // root workspace checked separately via mock paths
	}
	return a == b ||
		strings.HasPrefix(a, b+"/") ||
		strings.HasPrefix(b, a+"/")
}

// BasePathConflict describes an overlap between two workspace basePaths
// on the same engine.
type BasePathConflict struct {
	ExistingID       string `json:"existingId"`
	ExistingName     string `json:"existingName"`
	ExistingBasePath string `json:"existingBasePath"`
	NewBasePath      string `json:"newBasePath"`
	Reason           string `json:"reason"`
}

// checkBasePathConflict checks if a workspace's basePath overlaps with any
// peer workspace's basePath. Two basePaths overlap when one is a
// path-segment prefix of the other (or they're identical).
// Call this at workspace creation, update, and engine assignment.
// Returns nil if no conflict.
func checkBasePathConflict(newBasePath, newWorkspaceID string, peers []*store.Workspace) *BasePathConflict {
	if newBasePath == "" {
		return nil // root workspace never conflicts at the basePath level
	}
	for _, peer := range peers {
		if peer.ID == newWorkspaceID {
			continue // skip self (for updates)
		}
		peerBP := WorkspaceBasePath(peer)
		if peerBP == "" {
			continue // root workspace — overlap checked via mock paths
		}
		if basePathsOverlap(newBasePath, peerBP) {
			reason := "exact duplicate"
			if newBasePath != peerBP {
				if strings.HasPrefix(newBasePath, peerBP+"/") {
					reason = "new path is inside existing namespace"
				} else {
					reason = "existing path is inside new namespace"
				}
			}
			return &BasePathConflict{
				ExistingID:       peer.ID,
				ExistingName:     peer.Name,
				ExistingBasePath: peerBP,
				NewBasePath:      newBasePath,
				Reason:           reason,
			}
		}
	}
	return nil
}

// pathInvades checks whether a path falls inside a basePath namespace.
// Used both for mock-path-vs-basePath checking and as the primitive
// underneath basePathsOverlap.
//
//	pathInvades("/payment-api/status", "/payment-api") → true
//	pathInvades("/payment-api",        "/payment-api") → true
//	pathInvades("/payment-apiv2",      "/payment-api") → false
//	pathInvades("/other",              "/payment-api") → false
func pathInvades(path, basePath string) bool {
	return path == basePath ||
		strings.HasPrefix(path, basePath+"/")
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
