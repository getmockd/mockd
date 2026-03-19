<p align="center">
  <a href="https://mockd.io"><img src="https://mockd.io/logo-dark.svg" alt="mockd" width="200"></a>
</p>

<h3 align="center">One binary. Seven protocols. Zero dependencies.</h3>

<p align="center">
  Mock HTTP, gRPC, GraphQL, WebSocket, MQTT, SSE, and SOAP from a single CLI tool.<br>
  Import OpenAPI specs. Build digital twins. Let AI agents create mocks for you.
</p>

<p align="center">
  <a href="https://github.com/getmockd/mockd/actions/workflows/ci.yaml"><img src="https://github.com/getmockd/mockd/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
  <a href="https://github.com/getmockd/mockd/releases"><img src="https://img.shields.io/github/v/release/getmockd/mockd?include_prereleases" alt="Release"></a>
  <a href="https://github.com/getmockd/mockd/stargazers"><img src="https://img.shields.io/github/stars/getmockd/mockd?style=social" alt="Stars"></a>
  <a href="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
</p>

<p align="center">
  <a href="https://mockd.io">Website</a> &middot;
  <a href="https://mockd.io/docs">Docs</a> &middot;
  <a href="https://github.com/getmockd/mockd-samples">Samples</a> &middot;
  <a href="https://github.com/getmockd/mockd/blob/main/CONTRIBUTING.md">Contributing</a>
</p>

---

## Quick Start

```bash
# Install
curl -sSL https://get.mockd.io | sh

# Start + create a stateful CRUD API in one command
mockd start
mockd add http --path /api/users --stateful users

# It works immediately
curl -X POST localhost:4280/api/users -d '{"name":"Alice","email":"alice@test.com"}'
# → {"id":"a1b2c3","name":"Alice","email":"alice@test.com"}

curl localhost:4280/api/users
# → {"data":[{"id":"a1b2c3","name":"Alice","email":"alice@test.com"}],"meta":{"total":1}}
```

<details>
<summary><strong>More install options</strong></summary>

```bash
brew install getmockd/tap/mockd                                          # Homebrew
docker run -p 4280:4280 -p 4290:4290 ghcr.io/getmockd/mockd:latest      # Docker
go install github.com/getmockd/mockd/cmd/mockd@latest                    # Go
```

Pre-built binaries for Linux, macOS, and Windows on the [Releases](https://github.com/getmockd/mockd/releases) page.
</details>

## Why mockd?

Every other mock tool makes you choose: pick one protocol, install a runtime, bolt on extensions. mockd doesn't.

| | mockd | WireMock | Mockoon | json-server | Prism | MockServer |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| **Single binary, no runtime** | :white_check_mark: | :x: JVM | :x: Node | :x: Node | :x: Node | :x: JVM |
| **HTTP + gRPC + GraphQL + WS** | :white_check_mark: | 🔌 Ext | :x: | :x: | :x: | Partial |
| **MQTT + SSE + SOAP + OAuth** | :white_check_mark: | :x: | :x: | :x: | :x: | :x: |
| **Stateful CRUD** | :white_check_mark: | :x: | :white_check_mark: | :white_check_mark: | :x: | :x: |
| **Import OpenAPI/Postman/HAR** | :white_check_mark: | :white_check_mark: | :white_check_mark: | :x: | :white_check_mark: | :white_check_mark: |
| **Chaos engineering** | :white_check_mark: | :white_check_mark: | :x: | :x: | :x: | :x: |
| **MCP server (AI-native)** | :white_check_mark: | :x: | :x: | :x: | :x: | :x: |
| **Cloud tunnel sharing** | :white_check_mark: | :x: | :white_check_mark: | :x: | :x: | :x: |
| **Built-in web dashboard** | :white_check_mark: | :x: | :white_check_mark: | :x: | :x: | :x: |

> 🔌 **Ext** = available via separate extension, not bundled. mockd includes everything in a single binary.

## Digital Twins

Import a real API spec, bind it to stateful tables, and get a mock that passes the real SDK:

```yaml
# mockd.yaml — Stripe digital twin
version: "1.0"
imports:
  - path: stripe-openapi.yaml
    as: stripe
tables:
  - name: customers
    idStrategy: prefix
    idPrefix: "cus_"
    seedData:
      - { id: "cus_1", name: "Acme Corp", email: "billing@acme.com" }
extend:
  - { mock: stripe.GetCustomers, table: customers, action: list }
  - { mock: stripe.PostCustomers, table: customers, action: create }
  - { mock: stripe.GetCustomersCustomer, table: customers, action: get }
  - { mock: stripe.PostCustomersCustomer, table: customers, action: update }
  - { mock: stripe.DeleteCustomersCustomer, table: customers, action: delete }
```

```bash
mockd start -c mockd.yaml --no-auth
curl -X POST localhost:4280/v1/customers -d "name=Test&email=test@corp.com"
# → {"id":"cus_a1b2c3","object":"customer","name":"Test","email":"test@corp.com"}
```

**Validated with real SDKs:**
- Stripe: **49/49** `stripe-go` SDK tests pass
- Twilio: **13/13** `twilio-go` SDK tests pass
- OpenAI: `openai` Python SDK verified (models, assistants, chat completions)

See [mockd-samples](https://github.com/getmockd/mockd-samples) for complete digital twin configs.

## AI-Native (MCP)

mockd includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) server with **18 tools**. AI agents can create mocks, manage state, import specs, and verify contracts without touching the CLI:

```json
{
  "mcpServers": {
    "mockd": { "command": "mockd", "args": ["mcp"] }
  }
}
```

Works in Claude Desktop, Cursor, Windsurf, and any MCP-compatible editor. Tools cover mock CRUD, stateful resources, chaos injection, request logs, verification, workspaces, and import/export.

## Features

<details>
<summary><strong>Multi-Protocol Mocking</strong> — 7 protocols, unified CLI</summary>

| Protocol | Port | Example |
|----------|------|---------|
| HTTP/HTTPS | 4280 | `mockd add http --path /api/hello --body '{"msg":"hi"}'` |
| gRPC | 50051 | `mockd add grpc --proto svc.proto --service Greeter --rpc-method Greet` |
| GraphQL | 4280 | `mockd add graphql --path /graphql --operation hello` |
| WebSocket | 4280 | `mockd add websocket --path /ws --echo` |
| MQTT | 1883 | `mockd add mqtt --topic sensors/temp --payload '{"temp":72}'` |
| SSE | 4280 | `mockd add http --path /events --sse --sse-event 'data: hello'` |
| SOAP | 4280 | `mockd add soap --path /soap --operation GetWeather --response '<OK/>'` |
</details>

<details>
<summary><strong>Import & Export</strong> — OpenAPI, Postman, HAR, WireMock, cURL, WSDL</summary>

```bash
mockd import openapi.yaml           # OpenAPI 3.x / Swagger 2.0
mockd import collection.json        # Postman collections
mockd import recording.har          # HAR files
mockd import wiremock-mapping.json  # WireMock stubs
mockd import service.wsdl           # WSDL → SOAP mocks
mockd import "curl -X GET https://api.example.com/users"  # cURL commands
mockd export --format yaml > mocks.yaml
```
</details>

<details>
<summary><strong>Chaos Engineering</strong> — latency, errors, circuit breakers</summary>

```bash
mockd chaos apply flaky       # 30% error rate
mockd chaos apply slow-api    # 200-800ms latency
mockd chaos apply offline     # 100% 503 errors
mockd chaos disable
```
</details>

<details>
<summary><strong>Cloud Tunnel</strong> — share local mocks instantly</summary>

```bash
mockd tunnel
# → https://a1b2c3d4.tunnel.mockd.io → http://localhost:4280
```

All 7 protocols multiplexed through a single secure connection on port 443. Works behind NAT and firewalls.
</details>

<details>
<summary><strong>Workspaces</strong> — isolated mock environments</summary>

```bash
mockd workspace create -n "Payment API" --use
mockd import stripe-openapi.yaml
mockd workspace create -n "Comms API" --use
mockd import twilio-openapi.yaml
# Mocks, state, and logs are fully isolated per workspace
```
</details>

<details>
<summary><strong>Proxy Recording</strong> — record real traffic, replay as mocks</summary>

```bash
mockd proxy start --port 8888
# Configure your app to use http://localhost:8888 as proxy
# Traffic is recorded, then replay with:
mockd import recordings/session.json
```
</details>

<details>
<summary><strong>Web Dashboard</strong> — manage mocks visually</summary>

Release builds serve a web UI from the admin port (`http://localhost:4290`). VS Code-style editor, command palette, mock tree with folders, request log viewer, and near-miss debugging.
</details>

## Mockd Cloud

mockd works fully offline with no account required. For teams that want shared environments:

- **Persistent cloud mocks** — deploy mock environments your whole team can hit
- **Team management** — shared workspaces with access controls
- **Cloud tunnels** — authenticated tunnels with custom domains

Coming soon. [Join the waitlist](https://mockd.io/cloud).

## Documentation

Full guides, API reference, and config docs at **[mockd.io](https://mockd.io)**.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for setup.

## License

[Apache License 2.0](LICENSE) — free for commercial use.
