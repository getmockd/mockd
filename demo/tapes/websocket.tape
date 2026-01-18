# WebSocket mockd demo
# Run with: vhs demo/tapes/websocket.tape
# Requires: demo/tools/ws binary (go build -o ws ./demo/tools/ws)

Output demo/output/websocket.gif
Set Shell bash
Set FontSize 14
Set Width 1200
Set Height 800
Set Theme "Catppuccin Mocha"

# Setup
Type "# WebSocket Mocking Demo"
Enter
Sleep 1s

Type "# Build tools"
Enter
Sleep 500ms
Type "go build -o mockd ./cmd/mockd && go build -o ws ./demo/tools/ws"
Enter
Sleep 2s

# Create config with WebSocket endpoints (single line JSON)
Type "# Create WebSocket config"
Enter
Sleep 500ms
Type `echo '{"version":"1.0","websocketEndpoints":[{"path":"/ws/echo","echoMode":true},{"path":"/ws/chat","matchers":[{"match":{"type":"exact","value":"ping"},"response":{"type":"text","value":"pong"}}]}]}' > /tmp/ws.json`
Enter
Sleep 1s

# Start server
Type "# Start mockd with WebSocket config"
Enter
Sleep 500ms
Type "./mockd start --config /tmp/ws.json &"
Enter
Sleep 2s

# Test echo
Type "# Test WebSocket echo endpoint"
Enter
Sleep 500ms
Type `./ws ws://localhost:4280/ws/echo "Hello WebSocket!"`
Enter
Sleep 1s

# Test pattern matching
Type "# Test pattern matching - 'ping' -> 'pong'"
Enter
Sleep 500ms
Type `./ws ws://localhost:4280/ws/chat "ping"`
Enter
Sleep 1s

# View stats
Type "# View WebSocket stats"
Enter
Sleep 500ms
Type "curl -s localhost:4290/admin/ws/stats | jq ."
Enter
Sleep 2s

# Cleanup
Type "pkill mockd"
Enter
Sleep 500ms
Type "# Done!"
Enter
Sleep 2s
