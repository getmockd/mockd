# Proxy Recording

mockd can act as a MITM (Man-in-the-Middle) proxy to record real API traffic. Recorded requests and responses are automatically saved as mocks that can be replayed later.

## Overview

Proxy recording is useful for:

- **Capturing production API behavior** without manual mock creation
- **Recording integration test fixtures** from real services
- **Creating realistic mocks** from actual API responses
- **Debugging API interactions** by inspecting traffic

## Quick Start

Start the proxy targeting an upstream API:

```bash
mockd proxy --target https://api.example.com --record
```

Point your application to the proxy:

```bash
# Application makes request through proxy
curl http://localhost:4280/api/users
# Proxied to https://api.example.com/api/users
```

The request and response are recorded. View recordings:

```bash
mockd recordings list
```

## Proxy Modes

### Record Mode

Record all traffic passing through:

```bash
mockd proxy --target https://api.example.com --record
```

Recordings are saved to `./recordings/` by default.

### Playback Mode

Replay recorded responses without contacting upstream:

```bash
mockd proxy --playback --recordings ./recordings
```

### Record and Playback

Record new requests, replay known ones:

```bash
mockd proxy --target https://api.example.com --record --playback
```

Priority:
1. Check recordings for matching request
2. If found, replay recorded response
3. If not found, proxy to upstream and record

## Configuration

### Basic Proxy Config

```json
{
  "proxy": {
    "target": "https://api.example.com",
    "record": true,
    "recordingsDir": "./recordings"
  }
}
```

Start with config:

```bash
mockd proxy --config proxy-config.json
```

### Full Configuration

```json
{
  "proxy": {
    "target": "https://api.example.com",
    "port": 4280,
    "record": true,
    "playback": true,
    "recordingsDir": "./recordings",
    "recordFormat": "json",
    "stripHeaders": [
      "Authorization",
      "Cookie"
    ],
    "rewriteHost": true
  }
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `target` | Upstream API URL | Required |
| `port` | Proxy port | `4280` |
| `record` | Enable recording | `false` |
| `playback` | Enable playback | `false` |
| `recordingsDir` | Where to save recordings | `./recordings` |
| `recordFormat` | Output format (json, yaml) | `json` |
| `stripHeaders` | Headers to remove from recordings | `[]` |
| `rewriteHost` | Rewrite Host header to target | `true` |

## Recording Format

Each recorded request creates a file:

```
recordings/
├── GET_api_users_abc123.json
├── POST_api_users_def456.json
└── GET_api_users_1_ghi789.json
```

Recording content:

```json
{
  "recorded_at": "2024-01-15T10:30:00Z",
  "request": {
    "method": "GET",
    "path": "/api/users",
    "headers": {
      "Accept": "application/json"
    },
    "query": {}
  },
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": {
      "users": [
        {"id": 1, "name": "Alice"}
      ]
    }
  }
}
```

## HTTPS/TLS

### Auto-generated Certificates

mockd generates CA certificates automatically:

```bash
mockd proxy --target https://api.example.com
# Generates ./certs/mockd-ca.crt
```

Install the CA certificate in your system or browser to avoid TLS warnings.

### Custom Certificates

Use your own certificates:

```bash
mockd proxy --target https://api.example.com \
  --ca-cert ./my-ca.crt \
  --ca-key ./my-ca.key
```

### Certificate Configuration

```json
{
  "proxy": {
    "target": "https://api.example.com",
    "tls": {
      "caCertFile": "./certs/ca.crt",
      "caKeyFile": "./certs/ca.key",
      "certDir": "./certs/generated"
    }
  }
}
```

## Filtering

### Record Only Certain Paths

```json
{
  "proxy": {
    "target": "https://api.example.com",
    "record": true,
    "recordFilter": {
      "includePaths": [
        "/api/users.*",
        "/api/posts.*"
      ],
      "excludePaths": [
        "/api/health",
        "/api/metrics"
      ]
    }
  }
}
```

### Filter by Method

```json
{
  "proxy": {
    "recordFilter": {
      "methods": ["GET", "POST"]
    }
  }
}
```

### Filter by Status Code

Record only successful responses:

```json
{
  "proxy": {
    "recordFilter": {
      "statusCodes": [200, 201, 204]
    }
  }
}
```

## Sensitive Data

### Strip Headers

Remove sensitive headers from recordings:

```json
{
  "proxy": {
    "stripHeaders": [
      "Authorization",
      "Cookie",
      "X-API-Key",
      "X-Auth-Token"
    ]
  }
}
```

### Redact Body Fields

Mask sensitive data in recorded bodies:

```json
{
  "proxy": {
    "redact": {
      "bodyFields": [
        "password",
        "secret",
        "$.user.ssn"
      ],
      "replacement": "[REDACTED]"
    }
  }
}
```

## Converting to Mocks

Convert recordings to a mock configuration:

```bash
mockd recordings convert --input ./recordings --output mocks.json
```

Options:

```bash
# Convert with response templates
mockd recordings convert --input ./recordings --output mocks.json --templatize

# Merge with existing mocks
mockd recordings convert --input ./recordings --output mocks.json --merge
```

## CLI Commands

### Start Proxy

```bash
# Basic proxy with recording
mockd proxy --target https://api.example.com --record

# Playback only
mockd proxy --playback --recordings ./recordings

# Custom port
mockd proxy --target https://api.example.com --port 4290

# With config file
mockd proxy --config proxy.json
```

### Manage Recordings

```bash
# List recordings
mockd recordings list

# List with filter
mockd recordings list --path "/api/users.*"

# Show recording details
mockd recordings show GET_api_users_abc123.json

# Delete recordings
mockd recordings delete --older-than 7d

# Convert to mocks
mockd recordings convert --input ./recordings --output mocks.json
```

## Example Workflow

### 1. Record Production Traffic

```bash
# Start proxy recording
mockd proxy --target https://production-api.example.com --record

# Run your application tests through the proxy
API_URL=http://localhost:4280 npm test
```

### 2. Review Recordings

```bash
mockd recordings list
# Output:
# GET_api_users_abc123.json (200, 1.2kb)
# POST_api_users_def456.json (201, 0.3kb)
# GET_api_users_1_ghi789.json (200, 0.5kb)
```

### 3. Convert to Mocks

```bash
mockd recordings convert --input ./recordings --output mocks.json
```

### 4. Use for Testing

```bash
mockd start --config mocks.json
# Tests now run against recorded responses
```

## Proxy vs Mock Server

| Feature | Proxy Mode | Mock Server |
|---------|------------|-------------|
| Upstream dependency | Required | Not needed |
| Real responses | Yes | No (mocked) |
| Recording | Built-in | N/A |
| Offline operation | Playback only | Yes |
| Best for | Capturing real behavior | Development/testing |

## Next Steps

- [TLS/HTTPS Configuration](tls-https.md) - Certificate management
- [CLI Reference](../reference/cli.md) - All proxy commands
- [Configuration Reference](../reference/configuration.md) - Full schema
