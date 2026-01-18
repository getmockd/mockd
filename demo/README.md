# mockd Demos & Integration Tests

This directory contains VHS tapes for demos and integration testing.

## Structure

```
demo/
├── tapes/           # VHS tape files
│   ├── basic.tape   # Basic mock server demo
│   ├── websocket.tape
│   └── ...
├── tools/
│   └── ws/          # WebSocket CLI helper
└── output/          # Generated gifs (gitignored)
```

## Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) - `go install github.com/charmbracelet/vhs@latest`
- `jq` - for JSON formatting in demos

## Build Tools

```bash
# Build the WebSocket helper
go build -o ws ./demo/tools/ws
```

## Run Demos

```bash
# Generate a demo gif
vhs demo/tapes/basic.tape

# Run all tapes
for tape in demo/tapes/*.tape; do vhs "$tape"; done
```

## WebSocket Helper

The `ws` tool is a minimal WebSocket client for testing:

```bash
# Send message and print response
./ws ws://localhost:4280/ws/echo "Hello"

# Interactive mode (stdin -> ws -> stdout)
./ws ws://localhost:4280/ws/chat -i

# Listen mode (print incoming messages)
./ws ws://localhost:4280/ws/events -l

# Pipe input
echo "ping" | ./ws ws://localhost:4280/ws/chat
```

## Using as Integration Tests

The VHS tapes can be run as integration tests since they exercise the full CLI and server:

```bash
# Run a tape and check for errors
vhs demo/tapes/basic.tape 2>&1 | grep -i error && exit 1 || echo "PASS"
```
