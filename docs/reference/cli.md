# CLI Reference

Complete reference for the mockd command-line interface.

## Global Flags

These flags apply to all commands:

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-c` | Configuration file path | `mockd.json` |
| `--verbose` | `-v` | Enable verbose output | `false` |
| `--quiet` | `-q` | Suppress non-error output | `false` |
| `--help` | `-h` | Show help message | |
| `--version` | | Show version information | |

## Commands

### mockd start

Start the mock server.

```bash
mockd start [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-c` | Configuration file | `mockd.json` |
| `--port` | `-p` | Server port | `8080` |
| `--host` | | Bind address | `localhost` |
| `--https` | | Enable HTTPS | `false` |
| `--cert` | | TLS certificate file | |
| `--key` | | TLS private key file | |
| `--watch` | `-w` | Watch config for changes | `false` |
| `--admin` | | Enable admin API | `true` |
| `--admin-port` | | Admin API port | `8081` |

**Examples:**

```bash
# Basic start
mockd start

# With specific config
mockd start --config ./mocks/api.json

# Custom port
mockd start --port 3000

# With HTTPS
mockd start --https --port 8443

# Watch for config changes
mockd start --watch

# Bind to all interfaces
mockd start --host 0.0.0.0
```

---

### mockd proxy

Start the proxy server for recording or playback.

```bash
mockd proxy [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--target` | `-t` | Upstream API URL | Required |
| `--port` | `-p` | Proxy port | `8080` |
| `--record` | `-r` | Enable recording | `false` |
| `--playback` | | Enable playback | `false` |
| `--recordings` | | Recordings directory | `./recordings` |
| `--ca-cert` | | CA certificate file | |
| `--ca-key` | | CA private key file | |
| `--config` | `-c` | Proxy configuration file | |

**Examples:**

```bash
# Basic proxy
mockd proxy --target https://api.example.com

# Record traffic
mockd proxy --target https://api.example.com --record

# Playback only
mockd proxy --playback --recordings ./recordings

# Record and playback
mockd proxy --target https://api.example.com --record --playback

# With custom CA
mockd proxy --target https://api.example.com \
  --ca-cert ./certs/ca.crt \
  --ca-key ./certs/ca.key
```

---

### mockd validate

Validate a configuration file.

```bash
mockd validate [flags] <config-file>
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--strict` | Fail on warnings | `false` |
| `--format` | Output format (text, json) | `text` |

**Examples:**

```bash
# Validate config
mockd validate mocks.json

# Strict mode
mockd validate --strict mocks.json

# JSON output
mockd validate --format json mocks.json
```

---

### mockd recordings

Manage recorded API traffic.

```bash
mockd recordings <subcommand> [flags]
```

**Subcommands:**

#### recordings list

List recorded requests.

```bash
mockd recordings list [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--dir` | Recordings directory | `./recordings` |
| `--path` | Filter by path pattern | |
| `--method` | Filter by HTTP method | |
| `--format` | Output format (table, json) | `table` |

#### recordings show

Show details of a recording.

```bash
mockd recordings show <filename>
```

#### recordings delete

Delete recordings.

```bash
mockd recordings delete [flags]
```

| Flag | Description |
|------|-------------|
| `--all` | Delete all recordings |
| `--older-than` | Delete older than duration (e.g., 7d) |
| `--path` | Delete matching path pattern |

#### recordings convert

Convert recordings to mock configuration.

```bash
mockd recordings convert [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--input` | Recordings directory | `./recordings` |
| `--output` | Output config file | `mocks.json` |
| `--merge` | Merge with existing config | `false` |
| `--templatize` | Add response templates | `false` |

**Examples:**

```bash
# List recordings
mockd recordings list

# Filter by path
mockd recordings list --path "/api/users.*"

# Show recording
mockd recordings show GET_api_users_abc123.json

# Delete old recordings
mockd recordings delete --older-than 30d

# Convert to mocks
mockd recordings convert --input ./recordings --output mocks.json
```

---

### mockd cert

Manage TLS certificates.

```bash
mockd cert <subcommand> [flags]
```

**Subcommands:**

#### cert generate

Generate a server certificate.

```bash
mockd cert generate [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | Common Name (hostname) | `localhost` |
| `--san` | Subject Alternative Name (repeatable) | |
| `--days` | Validity in days | `365` |
| `--out` | Output directory | `./certs` |

#### cert generate-ca

Generate a CA certificate.

```bash
mockd cert generate-ca [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | CA name | `mockd CA` |
| `--days` | Validity in days | `3650` |
| `--out` | Output directory | `./certs` |

#### cert info

Display certificate information.

```bash
mockd cert info <cert-file>
```

**Examples:**

```bash
# Generate server cert
mockd cert generate --name localhost

# With SANs
mockd cert generate --name localhost --san "127.0.0.1" --san "myapp.local"

# Generate CA
mockd cert generate-ca --name "My Dev CA"

# Show cert info
mockd cert info ./certs/localhost.crt
```

---

### mockd version

Display version information.

```bash
mockd version
```

Output:
```
mockd version 0.1.0
Built: 2024-01-15T10:30:00Z
Go: go1.21.5
OS/Arch: linux/amd64
```

---

### mockd help

Display help for any command.

```bash
mockd help [command]
mockd <command> --help
```

**Examples:**

```bash
mockd help
mockd help start
mockd start --help
```

## Environment Variables

| Variable | Description | Equivalent Flag |
|----------|-------------|-----------------|
| `MOCKD_CONFIG` | Default config file | `--config` |
| `MOCKD_PORT` | Default port | `--port` |
| `MOCKD_HOST` | Default bind address | `--host` |
| `MOCKD_VERBOSE` | Enable verbose mode | `--verbose` |
| `MOCKD_RECORDINGS_DIR` | Recordings directory | `--recordings` |

Environment variables are overridden by command-line flags.

## Exit Codes

| Code | Description |
|------|-------------|
| `0` | Success |
| `1` | General error |
| `2` | Configuration error |
| `3` | Connection error |
| `4` | Validation error |

## Configuration File Discovery

mockd looks for configuration in this order:

1. Path specified with `--config`
2. `MOCKD_CONFIG` environment variable
3. `mockd.json` in current directory
4. `mockd.yaml` in current directory
5. `.mockd/config.json` in current directory

## See Also

- [Configuration Reference](configuration.md) - Config file format
- [Admin API Reference](admin-api.md) - Runtime management
- [Quickstart](../getting-started/quickstart.md) - Getting started guide
