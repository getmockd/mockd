# mockd

**Fast, lightweight API mocking for development and testing**

<div class="hero">
<p class="tagline">
Create realistic mock APIs in seconds. No external dependencies. Works offline.
</p>
</div>

<div class="cta-buttons">
<a href="getting-started/quickstart/" class="md-button md-button--primary">Get Started</a>
<a href="getting-started/installation/" class="md-button">Installation</a>
</div>

---

## Why mockd?

<div class="feature-grid">
<div class="feature-card">
<h3>Zero Dependencies</h3>
<p>Single binary written in Go. No runtime dependencies, no configuration servers, no Docker required for basic usage.</p>
</div>

<div class="feature-card">
<h3>Instant Setup</h3>
<p>Define mocks in JSON and start serving immediately. Your first mock API runs in under 5 minutes.</p>
</div>

<div class="feature-card">
<h3>Stateful Mocking</h3>
<p>Simulate real CRUD APIs with persistent state. Create, update, delete resources and see changes reflected across requests.</p>
</div>

<div class="feature-card">
<h3>Proxy Recording</h3>
<p>Record real API traffic through the MITM proxy and replay responses as mocks. Perfect for capturing production behavior.</p>
</div>

<div class="feature-card">
<h3>Flexible Matching</h3>
<p>Match requests by path, method, headers, query params, and body content. Use exact matches or regex patterns.</p>
</div>

<div class="feature-card">
<h3>Developer First</h3>
<p>Works offline, runs locally, integrates with any HTTP client. Use for development, testing, or CI/CD pipelines.</p>
</div>
</div>

---

## Quick Install

=== "Binary (Linux/macOS)"

    ```bash
    curl -sSL https://github.com/getmockd/mockd/releases/latest/download/mockd-$(uname -s)-$(uname -m) -o mockd
    chmod +x mockd
    ./mockd --version
    ```

=== "Go Install"

    ```bash
    go install github.com/getmockd/mockd/cmd/mockd@latest
    ```

=== "Docker"

    ```bash
    docker run -p 4280:4280 -v $(pwd)/mocks:/mocks ghcr.io/getmockd/mockd
    ```

---

## Your First Mock

Create a file called `mocks.json`:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/users"
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
          ]
        }
      }
    }
  ]
}
```

Start the server:

```bash
mockd start --config mocks.json
```

Test it:

```bash
curl http://localhost:4280/api/users
```

---

## Use Cases

### Development

Mock external APIs during frontend development. No need to wait for backend teams or deal with rate limits.

### Testing

Create predictable test fixtures for integration and end-to-end tests. Control every response.

### CI/CD

Run fast, isolated tests in CI pipelines without external dependencies.

### Prototyping

Quickly prototype API designs before implementing the real backend.

---

## Learn More

- **[Installation](getting-started/installation.md)** - Download and install mockd
- **[Quickstart](getting-started/quickstart.md)** - Run your first mock server
- **[Core Concepts](getting-started/concepts.md)** - Understand how mockd works
- **[Request Matching](guides/request-matching.md)** - Configure matching rules
- **[Stateful Mocking](guides/stateful-mocking.md)** - Simulate CRUD APIs
