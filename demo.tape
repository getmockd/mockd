# mockd demo — 7 protocols, one binary
# Run: vhs demo.tape
# Requires: mockd binary in PATH, ports 4280/4290 free

Output demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1000
Set Height 600
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 40ms
Set CursorBlink false

# ── Show it's a single binary ──────────────────────────────────────

Type "# One binary. Seven protocols."
Enter
Sleep 600ms

Type "mockd version"
Enter
Sleep 1.5s

# ── Start the server ───────────────────────────────────────────────

Type "mockd start -d"
Enter
Sleep 2s

# ── HTTP mock ──────────────────────────────────────────────────────

Type "# Mock an HTTP API"
Enter
Sleep 400ms

Type `mockd add http --path /api/users --body '[{"name":"Alice"},{"name":"Bob"}]'`
Enter
Sleep 1.5s

Type "curl -s localhost:4280/api/users | jq"
Enter
Sleep 1.5s

# ── GraphQL mock ───────────────────────────────────────────────────

Type "# Mock a GraphQL API"
Enter
Sleep 400ms

Type `mockd add graphql --path /graphql --operation hello --response '"world"'`
Enter
Sleep 1.5s

Type `curl -s localhost:4280/graphql -d '{"query":"{ hello }"}' | jq`
Enter
Sleep 1.5s

# ── WebSocket mock ─────────────────────────────────────────────────

Type "# Mock a WebSocket endpoint"
Enter
Sleep 400ms

Type "mockd add websocket --path /ws/chat --echo"
Enter
Sleep 1.5s

# ── Import from cURL ──────────────────────────────────────────────

Type "# Import from Postman, OpenAPI, WireMock, HAR, or cURL"
Enter
Sleep 400ms

Type `mockd import "curl -X GET https://api.example.com/health"`
Enter
Sleep 1.5s

# ── See everything ─────────────────────────────────────────────────

Type "mockd list"
Enter
Sleep 2s

# ── Chaos engineering ──────────────────────────────────────────────

Type "# Chaos engineering — test how your app handles failures"
Enter
Sleep 400ms

Type "mockd chaos enable --latency 200ms --error-rate 0.1"
Enter
Sleep 1.5s

# ── Clean up ───────────────────────────────────────────────────────

Type "mockd stop"
Enter
Sleep 1s

Type "# mockd.io"
Enter
Sleep 2s
