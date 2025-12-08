# Installation

mockd is distributed as a single binary with no external dependencies. Choose the installation method that works best for your environment.

## Binary Download

Download the latest release for your platform:

=== "Linux (x86_64)"

    ```bash
    curl -sSL https://github.com/getmockd/mockd/releases/latest/download/mockd-linux-amd64 -o mockd
    chmod +x mockd
    sudo mv mockd /usr/local/bin/
    ```

=== "Linux (ARM64)"

    ```bash
    curl -sSL https://github.com/getmockd/mockd/releases/latest/download/mockd-linux-arm64 -o mockd
    chmod +x mockd
    sudo mv mockd /usr/local/bin/
    ```

=== "macOS (Intel)"

    ```bash
    curl -sSL https://github.com/getmockd/mockd/releases/latest/download/mockd-darwin-amd64 -o mockd
    chmod +x mockd
    sudo mv mockd /usr/local/bin/
    ```

=== "macOS (Apple Silicon)"

    ```bash
    curl -sSL https://github.com/getmockd/mockd/releases/latest/download/mockd-darwin-arm64 -o mockd
    chmod +x mockd
    sudo mv mockd /usr/local/bin/
    ```

=== "Windows"

    ```powershell
    # Download from GitHub releases
    Invoke-WebRequest -Uri "https://github.com/getmockd/mockd/releases/latest/download/mockd-windows-amd64.exe" -OutFile "mockd.exe"

    # Add to PATH or move to a directory in your PATH
    ```

Verify the installation:

```bash
mockd --version
```

## Go Install

If you have Go 1.21+ installed:

```bash
go install github.com/getmockd/mockd/cmd/mockd@latest
```

This installs mockd to your `$GOPATH/bin` directory. Make sure it's in your `PATH`.

## Docker

Pull and run the official Docker image:

```bash
# Pull the latest image
docker pull ghcr.io/getmockd/mockd:latest

# Run with a local mocks directory
docker run -p 8080:8080 -v $(pwd)/mocks:/mocks ghcr.io/getmockd/mockd

# Run with a specific config file
docker run -p 8080:8080 -v $(pwd)/mocks.json:/mocks.json ghcr.io/getmockd/mockd start --config /mocks.json
```

### Docker Compose

```yaml
version: '3.8'
services:
  mockd:
    image: ghcr.io/getmockd/mockd:latest
    ports:
      - "8080:8080"
    volumes:
      - ./mocks:/mocks
    command: start --config /mocks/config.json
```

## Build from Source

Clone and build the project:

```bash
git clone https://github.com/getmockd/mockd.git
cd mockd
go build -o mockd ./cmd/mockd
```

## Verify Installation

After installation, verify mockd is working:

```bash
# Check version
mockd --version

# Show help
mockd --help

# Start a simple mock server (creates default config if none exists)
mockd start
```

## Next Steps

- [Quickstart](quickstart.md) - Create your first mock API
- [Core Concepts](concepts.md) - Learn how mockd works
