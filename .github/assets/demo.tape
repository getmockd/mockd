# mockd demo — 7 protocols, one binary
# Run: mockd stop 2>/dev/null; rm -f ~/.local/share/mockd/data.json; vhs demo.tape
# Requires: ports 4280/4290 free

Output demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1000
Set Height 600
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 40ms
Set CursorBlink false

# ── Install ────────────────────────────────────────────────────────

Type "# Install mockd"
Enter
Sleep 400ms

Type "curl -sSL https://get.mockd.io | INSTALL_DIR=~/.local/bin sh"
Enter
Sleep 8s

# ── Start the server ───────────────────────────────────────────────

Type "mockd start -d"
Enter
Sleep 3s

# ── Stateful CRUD API in one command ───────────────────────────────

Type "# Stateful CRUD API — one command"
Enter
Sleep 400ms

Type "mockd add http --path /api/users --stateful users"
Enter
Sleep 1.5s

Type `curl -s -X POST localhost:4280/api/users -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@test.com"}' | jq`
Enter
Sleep 2s

Type "curl -s localhost:4280/api/users | jq"
Enter
Sleep 2s

# ── GraphQL mock ───────────────────────────────────────────────────

Type "# GraphQL — just as easy"
Enter
Sleep 400ms

Type `mockd add graphql --path /graphql --operation GetUser --response '{"id":"1","name":"Alice"}'`
Enter
Sleep 1.5s

Type `curl -s localhost:4280/graphql -H 'Content-Type: application/json' -d '{"query":"{ GetUser }"}' | jq`
Enter
Sleep 2s

# ── Import from cURL ──────────────────────────────────────────────

Type "# Import from OpenAPI, Postman, HAR, WireMock, or cURL"
Enter
Sleep 400ms

Type `mockd import "curl -X GET https://api.example.com/health"`
Enter
Sleep 1.5s

# ── See everything ─────────────────────────────────────────────────

Type "mockd list"
Enter
Sleep 2.5s

# ── Chaos engineering ──────────────────────────────────────────────

Type "# Chaos engineering — simulate real-world failures"
Enter
Sleep 400ms

Type "mockd chaos apply flaky"
Enter
Sleep 1.5s

# ── Clean up ───────────────────────────────────────────────────────

Type "mockd chaos disable"
Enter
Sleep 1s

Type "mockd stop"
Enter
Sleep 1s

Type "# mockd.io"
Enter
Sleep 2s
