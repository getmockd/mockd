---
title: Proxy Recording
description: Use mockd as a MITM proxy to record real API traffic and convert recordings to mock definitions.
---

mockd includes a **MITM (Man-in-the-Middle) forward proxy** that records real API traffic. Configure your HTTP client to route through the proxy, and mockd captures every request/response pair to disk. You can then convert recordings into mock definitions with `mockd convert`.

## Overview

Proxy recording is useful for:

- **Capturing real API behavior** without writing mocks by hand
- **Recording integration test fixtures** from live services
- **Creating realistic mocks** from actual API responses
- **Debugging API interactions** by inspecting traffic

## Quick Start

Start the proxy in the foreground:

```bash
mockd proxy start
```

This starts a forward proxy on port **8888** in `record` mode. Configure your HTTP client to use it:

```bash
# cURL with proxy
curl -x http://localhost:8888 http://api.example.com/users

# Or set environment variables
export http_proxy=http://localhost:8888
export https_proxy=http://localhost:8888
curl http://api.example.com/users
```

The request and response are recorded to disk. Press **Ctrl+C** to stop the proxy.

View what was captured:

```bash
mockd recordings list
```

Convert recordings to mock definitions:

```bash
mockd convert -o mocks.yaml
```

## Starting the Proxy

The proxy runs in the foreground and stops with Ctrl+C:

```bash
# Default: port 8888, record mode
mockd proxy start

# Custom port
mockd proxy start --port 9090

# Named session (for organizing recordings)
mockd proxy start --session my-api-test

# Passthrough mode (no recording, just forwarding)
mockd proxy start --mode passthrough
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `8888` | Proxy server port |
| `--mode` | `-m` | `record` | Proxy mode: `record` or `passthrough` |
| `--session` | `-s` | `default` | Recording session name |
| `--recordings-dir` | | (platform default) | Base directory for recordings |
| `--ca-path` | | | CA certificate directory (enables HTTPS interception) |
| `--include` | | | Comma-separated path patterns to include (glob) |
| `--exclude` | | | Comma-separated path patterns to exclude (glob) |
| `--include-hosts` | | | Comma-separated host patterns to include |
| `--exclude-hosts` | | | Comma-separated host patterns to exclude |

## Proxy Modes

### Record Mode (default)

Records all traffic passing through:

```bash
mockd proxy start --mode record
```

Every HTTP request/response pair is persisted to disk as it flows through the proxy.

### Passthrough Mode

Forwards traffic without recording:

```bash
mockd proxy start --mode passthrough
```

Useful for debugging or when you only need the proxy behavior without capturing data.

## HTTPS Interception

By default, HTTPS requests are tunneled (TCP pass-through) and **not recorded** because the traffic is encrypted.

To record HTTPS traffic, generate a CA certificate and configure your system to trust it:

### Generate a CA Certificate

```bash
# Generate CA cert and key
mockd proxy ca generate --ca-path ./certs

# Start proxy with HTTPS interception
mockd proxy start --ca-path ./certs
```

The proxy dynamically generates per-host TLS certificates signed by your CA, enabling it to decrypt and record HTTPS traffic.

### Trust the CA Certificate

Export the certificate for installation:

```bash
# Export to file
mockd proxy ca export --ca-path ./certs -o mockd-ca.crt

# macOS: Add to system keychain
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain mockd-ca.crt

# Linux (Debian/Ubuntu): Add to system certificates
sudo cp mockd-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

## Filtering

Control what gets recorded using include/exclude patterns with glob matching (`*` wildcard):

### Filter by Path

```bash
# Only record API paths
mockd proxy start --include "/api/*"

# Exclude health checks
mockd proxy start --exclude "/health,/metrics,/ping"
```

### Filter by Host

```bash
# Only record traffic to specific hosts
mockd proxy start --include-hosts "api.example.com,auth.example.com"

# Exclude noisy hosts
mockd proxy start --exclude-hosts "analytics.example.com,cdn.example.com"
```

### Combine Filters

```bash
mockd proxy start \
  --include-hosts "api.example.com" \
  --include "/api/*" \
  --exclude "/api/health"
```

**Filter precedence:**
1. If the request matches **any** exclude pattern → not recorded
2. If include patterns exist and the request matches **none** → not recorded
3. Otherwise → recorded

## Recording Storage

Recordings are organized by session and host:

```
~/.local/share/mockd/recordings/
├── default-20260224-143000/
│   ├── meta.json
│   ├── api.example.com/
│   │   ├── rec_a1b2c3d4.json
│   │   └── rec_e5f6a7b8.json
│   └── auth.example.com/
│       └── rec_c9d0e1f2.json
├── my-session-20260225-091500/
│   ├── meta.json
│   └── ...
└── latest -> default-20260224-143000/
```

The `latest` symlink always points to the most recent session. The default storage location is platform-specific:

| Platform | Default Path |
|----------|-------------|
| macOS | `~/Library/Application Support/mockd/recordings/` |
| Linux | `~/.local/share/mockd/recordings/` |
| Windows | `%LOCALAPPDATA%/mockd/recordings/` |

Override with `--recordings-dir` or the `XDG_DATA_HOME` environment variable.

## Managing Recordings

### List Sessions

```bash
mockd recordings sessions
```

### List Recordings

```bash
# From the latest session
mockd recordings list

# From a specific session
mockd recordings list --session my-api-test

# Filter by method or host
mockd recordings list --method GET
mockd recordings list --host api.example.com
```

### Export Recordings

```bash
# Export to JSON
mockd recordings export -o recordings.json

# From a specific session
mockd recordings export --session my-api-test -o recordings.json
```

### Import Recordings

```bash
mockd recordings import --input recordings.json --session imported
```

### Clear Recordings

```bash
# Clear a specific session
mockd recordings clear --session my-api-test --force

# Clear all sessions
mockd recordings clear --force
```

## Converting to Mocks

The `mockd convert` command transforms recorded traffic into mock definitions:

```bash
# Convert latest session, output to stdout
mockd convert

# Save to file
mockd convert -o mocks.yaml

# Convert a specific session
mockd convert --session my-api-test -o mocks.yaml
```

### Smart Matching

Detect dynamic path segments and convert them to path parameters:

```bash
# Turns /users/123 into /users/{id}
mockd convert --smart-match -o mocks.yaml
```

### Filtering During Conversion

```bash
# Only GET and POST requests
mockd convert --method GET,POST

# Only successful responses
mockd convert --status 2xx

# Only specific hosts
mockd convert --include-hosts "api.example.com"

# Only specific paths
mockd convert --path-filter "/api/*"
```

### Duplicate Handling

When multiple recordings match the same endpoint:

```bash
# Keep first occurrence (default)
mockd convert --duplicates first

# Keep last occurrence
mockd convert --duplicates last

# Keep all occurrences
mockd convert --duplicates all
```

## Example Workflow

### 1. Record Real API Traffic

```bash
# Start the proxy
mockd proxy start --session api-capture --port 8888

# In another terminal, run your app through the proxy
http_proxy=http://localhost:8888 npm test

# Stop the proxy with Ctrl+C
```

### 2. Review Recordings

```bash
mockd recordings list --session api-capture
# ID       METHOD  HOST               PATH           STATUS  DURATION
# a1b2c3d4 GET     api.example.com    /api/users     200     150ms
# e5f6a7b8 POST    api.example.com    /api/users     201     89ms
# c9d0e1f2 GET     api.example.com    /api/users/1   200     45ms
```

### 3. Convert to Mocks

```bash
mockd convert --session api-capture --smart-match -o mocks.yaml
```

### 4. Use the Mocks

```bash
mockd serve --config mocks.yaml
# Your tests now run against captured responses — no external dependency needed
```

## Proxy vs Mock Server

| Feature | Proxy Recording | Mock Server |
|---------|----------------|-------------|
| Upstream dependency | Required (for recording) | Not needed |
| Real responses | Yes | No (mocked) |
| Recording | Built-in | N/A |
| Offline operation | No | Yes |
| Best for | Capturing real behavior | Development and testing |

## Next Steps

- [TLS/HTTPS Configuration](/guides/tls-https/) — Certificate management for mock serving
- [CLI Reference](/reference/cli/) — All available commands
- [Configuration Reference](/reference/configuration/) — Full config schema
