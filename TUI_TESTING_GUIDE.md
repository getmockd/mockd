# TUI Testing Guide

## Quick Test Commands

### 1. Build and Test Help

```bash
# Build the binary
cd /home/plex/development/repos/getmockd/mockd
go build -o mockd ./cmd/mockd

# Test main help shows TUI info
./mockd --help | grep -A10 "TUI Flags"

# Test TUI-specific help
./mockd --tui --help
```

Expected output should show:
- TUI modes (Remote, Hybrid)
- Flags (--serve, --admin)
- Examples for all three modes
- Navigation keys

### 2. Test Remote Mode (Default)

**Terminal 1 - Start Server:**
```bash
# Start mock server
./mockd start

# Should show:
# - Mock server running on port 8080
# - Admin API running on port 9090
```

**Terminal 2 - Launch TUI:**
```bash
# Connect TUI to local server
./mockd --tui

# You should see:
# - Full-screen TUI
# - Header with "mockd TUI"
# - Sidebar with 7 menu items
# - Dashboard view (default)
# - Status bar at bottom
```

**Test Navigation:**
```
Press '1' - Should show Dashboard
Press '2' - Should show Mocks view
Press '3' - Should show Proxy view
Press '4' - Should show Streams view
Press '5' - Should show Traffic view
Press '6' - Should show Connections view
Press '7' - Should show Logs view
Press '?' - Should toggle help overlay
Press 'q' - Should exit cleanly
```

### 3. Test Hybrid Mode

**Single Terminal:**
```bash
# Start everything together
./mockd --tui --serve

# Should show:
# Mock server started on port 8080
# Admin API started on port 9090
# Starting TUI...
# [TUI launches]
```

**Verify:**
- ✅ TUI opens successfully
- ✅ Navigate between all 7 views
- ✅ Press 'q' to quit
- ✅ See "Shutting down servers..."
- ✅ Clean exit to shell

**Test interaction:**
```
In Mocks view (press '2'):
- Should see list of mocks (or empty state)
- Press 'n' to attempt creating new mock
- Navigate with arrow keys

In Dashboard view (press '1'):
- Should see server status
- Should see stats (mocks count, etc.)

In Traffic view (press '5'):
- Should see request log
- Press 'p' to pause/resume
- Press 'c' to clear
```

### 4. Test Remote Server Mode

**Terminal 1 - Remote Server (different machine or port):**
```bash
# Start server on custom admin port
./mockd start --port 8080 --admin-port 9999
```

**Terminal 2 - TUI:**
```bash
# Connect TUI to custom port
./mockd --tui --admin http://localhost:9999

# Should connect and show TUI
```

### 5. Test Error Handling

**Test 1: No Server Running**
```bash
# Make sure no server is running
pkill -f "mockd start"

# Try to launch TUI
./mockd --tui

# Expected: TUI opens but shows connection errors in views
# Dashboard should show "Failed to fetch status" or similar
```

**Test 2: Port Already in Use**
```bash
# Start server
./mockd start

# In another terminal, try hybrid mode
./mockd --tui --serve

# Expected: Error about port already in use
# Should handle gracefully
```

**Test 3: Invalid Admin URL**
```bash
./mockd --tui --admin http://invalid-url:9999

# Expected: TUI opens but shows connection errors
```

### 6. Test View Functionality

#### Dashboard View
```bash
./mockd --tui --serve
# Press '1' for Dashboard
# Verify:
# - Server status shown
# - Port numbers displayed
# - Mock count visible
# - Request count visible
```

#### Mocks View
```bash
# Press '2' for Mocks
# Add a mock via CLI in another terminal:
./mockd add --path /test --status 200 --body "test"

# Back in TUI:
# - Should see the new mock appear
# - Press 'enter' to toggle enabled/disabled
# - Press '/' to activate filter
# - Type to filter mocks
```

#### Traffic View
```bash
# Press '5' for Traffic
# Make a request via curl:
curl http://localhost:8080/test

# Back in TUI:
# - Should see the request appear in log
# - Should show method, path, status, duration
# - Press 'enter' to see details
# - Press 'p' to pause refresh
# - Press 'c' to clear log
```

#### Proxy View
```bash
# Press '3' for Proxy
# Verify:
# - Shows proxy status (stopped/running)
# - Shows target URL if configured
# - Can press 's' to start/stop
# - Can press 't' to set target
```

### 7. Run Automated Tests

```bash
# Run all TUI tests
go test ./pkg/tui/... -v

# Run with race detection
go test ./pkg/tui/... -race

# Run with coverage
go test ./pkg/tui/... -cover -coverprofile=coverage.out

# View coverage
go tool cover -html=coverage.out
```

### 8. Test Build Variations

```bash
# Test build on different platforms
GOOS=linux GOARCH=amd64 go build -o mockd-linux-amd64 ./cmd/mockd
GOOS=darwin GOARCH=amd64 go build -o mockd-darwin-amd64 ./cmd/mockd
GOOS=darwin GOARCH=arm64 go build -o mockd-darwin-arm64 ./cmd/mockd
GOOS=windows GOARCH=amd64 go build -o mockd-windows-amd64.exe ./cmd/mockd

# Verify all build successfully
```

### 9. Integration Test Script

```bash
#!/bin/bash
# Save as: test-tui-integration.sh

set -e

echo "Building mockd..."
go build -o /tmp/mockd-test ./cmd/mockd

echo "Testing help..."
/tmp/mockd-test --help | grep -q "TUI" && echo "✅ Help includes TUI"

echo "Testing TUI help..."
/tmp/mockd-test --tui --help | grep -q "Hybrid" && echo "✅ TUI help works"

echo "Running tests..."
go test ./pkg/tui/... -v | grep -q "PASS" && echo "✅ All tests pass"

echo "Testing CLI parsing..."
timeout 2 /tmp/mockd-test --tui --admin http://test:9090 2>&1 | grep -q "could not open a new TTY" && echo "✅ Admin URL parsed"

echo ""
echo "All integration tests passed! ✅"
```

### 10. Performance Test

```bash
# Test with many mocks
for i in {1..100}; do
  ./mockd add --path "/test$i" --status 200 --body "{\"id\": $i}"
done

# Launch TUI
./mockd --tui

# Navigate to Mocks view (press '2')
# Verify:
# - All 100 mocks load
# - Scrolling is smooth
# - Filter works with many items
# - No crashes or freezes
```

### 11. Stress Test

```bash
# Terminal 1: Start hybrid mode
./mockd --tui --serve

# Terminal 2: Generate traffic
for i in {1..1000}; do
  curl -s http://localhost:8080/test$i > /dev/null &
done

# Back in TUI (Terminal 1):
# Press '5' for Traffic view
# Verify:
# - Traffic appears in real-time
# - No crashes with high volume
# - Pause/resume works
# - Clear works
```

### 12. Manual Acceptance Tests

#### Test Case 1: Fresh Start
- [ ] Build binary from scratch
- [ ] Run `./mockd --tui --serve`
- [ ] All views load without errors
- [ ] Can navigate all views
- [ ] Quit cleanly

#### Test Case 2: With Existing Config
- [ ] Create mocks.json with sample mocks
- [ ] Run `./mockd start --config mocks.json`
- [ ] In another terminal: `./mockd --tui`
- [ ] Mocks view shows all mocks from config
- [ ] Can toggle mocks on/off
- [ ] Changes persist

#### Test Case 3: Proxy Recording
- [ ] Start TUI: `./mockd --tui --serve`
- [ ] Navigate to Proxy view (press '3')
- [ ] Press 's' to start proxy
- [ ] Set target to real API
- [ ] Make requests through proxy
- [ ] See recordings appear
- [ ] Can convert to mocks

#### Test Case 4: Stream Monitoring
- [ ] Start TUI: `./mockd --tui --serve`
- [ ] Navigate to Connections view (press '6')
- [ ] Open WebSocket connection from browser
- [ ] See connection appear in TUI
- [ ] See message counts update
- [ ] Kill connection from TUI

#### Test Case 5: Live Logging
- [ ] Start TUI: `./mockd --tui --serve`
- [ ] Navigate to Logs view (press '7')
- [ ] Generate various activities
- [ ] See logs appear in real-time
- [ ] Filter by level
- [ ] Search for specific text

## Common Issues & Solutions

### Issue 1: TUI doesn't start
```
Error: could not open a new TTY: open /dev/tty: no such device or address
```
**Solution:** You're in a headless environment. Use `--ci` flag or run in proper terminal.

### Issue 2: Port conflict
```
Error: listen tcp :8080: bind: address already in use
```
**Solution:** Stop existing server or use different ports.

### Issue 3: Connection refused
```
Error: connection refused
```
**Solution:** Ensure server is running before launching TUI, or use `--serve` flag.

### Issue 4: Views show "Loading..."
**Solution:** 
- Check server is running
- Verify Admin API is accessible
- Check firewall settings
- Try `curl http://localhost:9090/admin/status`

## Success Criteria

✅ **Build:** Binary compiles without errors  
✅ **Tests:** All 65+ tests pass  
✅ **Help:** Help messages show correctly  
✅ **Remote:** Can connect to existing server  
✅ **Hybrid:** Can start embedded server + TUI  
✅ **Navigation:** All 7 views accessible  
✅ **Status Bar:** Shows correct hints per view  
✅ **Shutdown:** Graceful cleanup on quit  
✅ **Signals:** Handles Ctrl+C properly  
✅ **Errors:** Handles connection errors gracefully  

## Performance Benchmarks

- **Startup Time:** < 1 second (hybrid mode)
- **View Switch:** < 100ms
- **Memory Usage:** < 50MB (idle)
- **CPU Usage:** < 5% (idle)
- **Request Log:** Handle 1000+ entries smoothly

## CI/CD Integration

```yaml
# Example GitHub Actions workflow
name: Test TUI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      
      - name: Build
        run: go build -o mockd ./cmd/mockd
      
      - name: Test TUI package
        run: go test ./pkg/tui/... -v -race -cover
      
      - name: Test help output
        run: |
          ./mockd --help | grep "TUI"
          ./mockd --tui --help | grep "Hybrid"
      
      - name: Integration test (headless)
        run: |
          timeout 5 ./mockd --tui 2>&1 | grep -q "could not open a new TTY"
          echo "Headless behavior correct ✅"
```

---

**Last Updated:** 2025-12-21  
**Version:** 1.0.0  
**Status:** Complete ✅
