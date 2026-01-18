# Basic mockd demo
# Run with: vhs demo/tapes/basic.tape

Output demo/output/basic.gif
Set Shell bash
Set FontSize 14
Set Width 1200
Set Height 800
Set Theme "Catppuccin Mocha"

# Build mockd
Type "# Build mockd"
Enter
Sleep 500ms
Type "go build -o mockd ./cmd/mockd"
Enter
Sleep 2s

# Start server in background
Type "# Start the mock server"
Enter
Sleep 500ms
Type "./mockd start --port 4280 --admin-port 4290 &"
Enter
Sleep 2s

# Check health
Type "# Check server health"
Enter
Sleep 500ms
Type "curl -s localhost:4290/health | jq ."
Enter
Sleep 1s

# Create a mock
Type "# Create a mock endpoint"
Enter
Sleep 500ms
Type `curl -s -X POST localhost:4290/mocks -H 'Content-Type: application/json' -d '{"name": "Get Users", "matcher": {"method": "GET", "path": "/api/users"}, "response": {"statusCode": 200, "body": "[{\"id\": 1, \"name\": \"Alice\"}]"}}' | jq .`
Enter
Sleep 2s

# Hit the mock
Type "# Hit the mock endpoint"
Enter
Sleep 500ms
Type "curl -s localhost:4280/api/users | jq ."
Enter
Sleep 2s

# Template variables
Type "# Create a mock with template variables"
Enter
Sleep 500ms
Type `curl -s -X POST localhost:4290/mocks -H 'Content-Type: application/json' -d '{"name": "Echo", "matcher": {"method": "POST", "path": "/echo"}, "response": {"statusCode": 200, "body": "{\"id\": \"{{request.body.id}}\", \"uuid\": \"{{uuid}}\"}"}}' | jq .name`
Enter
Sleep 1s

Type "# Test template variable substitution"
Enter
Sleep 500ms
Type `curl -s -X POST localhost:4280/echo -H 'Content-Type: application/json' -d '{"id": 42}' | jq .`
Enter
Sleep 2s

# Cleanup
Type "pkill mockd"
Enter
Sleep 500ms
Type "# Done!"
Enter
Sleep 2s
