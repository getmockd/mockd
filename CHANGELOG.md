# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Near-miss debugging** — When no mock matches a request, the 404 response now includes a `nearMisses` array with detailed field-by-field breakdown (method, path, headers, query params) showing what almost matched and why. Each near-miss includes match percentage, score, and a human-readable reason like `path matched, but method expected "GET", got "DELETE"`. Response includes `X-Mockd-Near-Misses` header with count
- **Unmatched request filtering** — `GET /requests?unmatchedOnly=true` returns only unmatched requests. CLI: `mockd logs --requests --unmatched`. MCP tool: `get_request_logs` with `unmatchedOnly: true`. Near-miss data is attached to request log entries for post-hoc debugging
- **`mockd mcp` auto-start** — MCP server now auto-starts a background daemon if no mockd server is running, so AI assistants work with zero setup. The daemon survives the MCP session and is shared across multiple sessions. Use `--data-dir` for project-scoped isolation with a separate daemon. Stop with `mockd stop`

## [0.4.4] - 2026-02-27

### Added

- **Stateful Protocol Bridge** — SOAP operations can now read/write stateful CRUD resources that were previously HTTP-only. A REST `POST /api/users` creates a user that a SOAP `GetUser` can retrieve, and vice versa. All protocols share the same in-memory state store.
- **SOAP stateful operations** — New `statefulResource` and `statefulAction` fields on SOAP operation configs wire operations directly to stateful resources with automatic XML↔map conversion and SOAP fault mapping
- **Custom operations with `expr-lang/expr`** — Define multi-step operations that compose reads, writes, and expression-evaluated transforms against stateful resources (e.g., `TransferFunds` that debits one account and credits another atomically)
- **WSDL import** — `mockd soap import <wsdl-file>` generates SOAP mock configs from WSDL 1.1 service definitions with `--stateful` flag for automatic CRUD heuristics; WSDL also supported via `mockd import service.wsdl`
- **`customOperations` top-level config** — YAML/JSON configs can define custom operations with named step pipelines (`read`, `update`, `delete`, `create`, `set`) and response expression maps
- **Cross-protocol state verification** — Integration tests proving REST↔Bridge (SOAP path) bidirectional state sharing
- **WSDL format in admin API** — `GET /formats` now includes WSDL in supported import formats
- **Config export completeness** — `Export()` now includes `statefulResources` and `customOperations` (previously only exported mocks)
- **Config merge completeness** — `MergeProjectConfigs()` now merges `customOperations` by name
- **Custom-op validation CLI improvements** — `mockd stateful custom validate` now supports `--strict` (fail on warnings) and `--check-expressions-runtime` with `--fixtures-file` for no-write runtime expression checks before registration
- **MCP tool expansion** — 16 multiplexed tools (consolidated from 19 via action-based dispatching): `manage_mock`, `manage_state`, `manage_context`, `manage_workspace`, plus 3 chaos tools (`get_chaos_config`, `set_chaos_config`, `reset_chaos_stats`), 3 verification tools (`verify_mock`, `get_mock_invocations`, `reset_verification`), custom operations tool (`manage_custom_operation`), and 2 MCP resources (`mock://chaos`, `mock://verification/{mockId}`)
- **10 built-in chaos profiles** — Pre-configured chaos scenarios: `slow-api`, `degraded`, `flaky`, `offline`, `timeout`, `rate-limited`, `mobile-3g`, `satellite`, `dns-flaky`, `overloaded`. Apply with `mockd chaos apply <name>` or `POST /chaos/profiles/{name}/apply`
- **23 new faker types** (34 total) — Internet (`ipv4`, `ipv6`, `macAddress`, `userAgent`), Finance (`creditCard` with Luhn validation, `creditCardExp`, `cvv`, `currencyCode`, `currency`, `iban`), Commerce (`price`, `productName`, `color`, `hexColor`), Identity (`ssn`, `passport`, `jobTitle`), Geo (`latitude`, `longitude`), Text (`words`, `slug`), Data (`mimeType`, `fileExtension`). Parameterized syntax: `{{faker.words(5)}}`
- **`POST /state/resources` endpoint** — Dedicated REST endpoint for creating stateful resources at runtime through all layers (engine → admin → CLI → MCP), replacing the previous `ImportConfig` wrapper
- **`mockd verify` CLI command** — Full verification command tree (`status`, `check`, `invocations`, `reset`) for asserting mock call counts and inspecting request details from the CLI. `check` returns non-zero exit codes on failure for CI scripting. Supports `--exactly`, `--at-least`, `--at-most`, and `--never` assertion flags
- **MCP Registry metadata** — `server.json` for official MCP Registry submission (`io.mockd/mockd`) with OCI container label for registry verification

### Fixed

- **SOAP validation rejected stateful operations** — Validator required `response` or `fault` on every SOAP operation, blocking stateful-only operations that get their response from the stateful resource. Now accepts `statefulResource` as a valid alternative.
- **Custom operations silently ignored** — `loadCollection()` parsed `customOperations` from config but never registered them on the stateful Bridge. They now wire through correctly.
- **StatefulAction validation** — Added validation that `statefulAction` is a valid CRUD action and that `statefulResource` and `statefulAction` are set together (both or neither)
- **MCP drift — GetStats endpoint** — `get_server_status` was calling `/stats` instead of `/status`, returning empty data. Fixed to call the correct endpoint
- **MCP drift — toggle_mock atomicity** — `toggle_mock` was using GET+PUT workaround instead of `PATCH /mocks/{id}`. Fixed to use atomic PATCH
- **Chaos config JSON shape** — MCP `set_chaos_config` was sending flat fields (`latencyMinMs`, `errorRate`) but admin API expects nested typed structs (`latency.min`, `errorRate.probability`). Fixed to build correct nested shape. Profile activation uses `POST /chaos/profiles/{name}/apply` endpoint
- **Data race in engine handler tests** — Added mutex to `mockEngineServer` in `engine_handlers_test.go` to eliminate race condition in CI

### Dependencies

- Added `github.com/expr-lang/expr` v1.17.8 for expression evaluation in custom operations (zero transitive dependencies, ~0.5MB)

## [0.4.0] - 2026-02-24

### Added

- **Auto-generate mock IDs** — config files no longer require `id` fields; `fillMockDefaults()` generates deterministic IDs and infers `type` from populated spec fields (DX-1)
- **Flexible `response.body`** — YAML/JSON config now accepts objects and arrays directly for `response.body` (auto-marshaled to JSON string); custom `UnmarshalJSON`/`UnmarshalYAML` on both `mock.HTTPResponse` and `config.HTTPResponse` (DX-5)
- **Import from stdin** — `mockd import -` or `cat mocks.yaml | mockd import` (DX-2)
- **Import directories** — `mockd import ./mocks/` recursively loads `.yaml`, `.yml`, and `.json` files (DX-3)
- **`--stateful` flag on `mockd add`** — `mockd add http --path /api/users --stateful` creates a stateful CRUD resource in one command (DX-4)
- **Doc config regression test** — `tests/unit/doc_config_test.go` scans all doc `.md` files, extracts YAML/JSON config blocks, and validates them through `config.ParseYAML`/`ParseJSON`

### Fixed

- **24 documentation bugs** found by live-testing every example across all 30 doc files against a running mockd server:
  - `tls-https.md`: `mockd cert generate` and `mockd cert generate-ca` commands don't exist; replaced with `--tls-auto` flag and OpenSSL commands; fixed `--https`/`--cert`/`--key` → `--tls-auto`/`--tls-cert`/`--tls-key`/`--https-port`
  - `observability.md`: Prometheus metric names corrected (`mockd_http_requests_total` → `mockd_requests_total`, `mockd_http_request_duration_seconds_bucket` → `mockd_request_duration_seconds_bucket`); added 3 missing metrics
  - `validation.md`: Error response format corrected to include `location`, `code`, `received`, `expected`, `hint` fields
  - `response-templating.md`: Removed 7 non-existent template functions; added documentation for actual functions (faker.*, sequence(), uuid.short, etc.)
  - `mqtt.md`: All `mqtt subscribe` examples were missing required `broker:port` positional argument
  - `replay-modes.md`: Replay start response field `replayId` → `sessionId`; status `config` object → top-level `mode`
  - `stream-recording.md`: Start response format, recording `formatVersion` → `version`, stats field names corrected
  - `grpc.md`: `-I` flag → `--import`; `grpc list` output format corrected
  - `websocket.md`: Non-upgrade response returns 404, not 400
  - `sharing-mocks.md`: Non-existent `--keepalive` flag; `--mqtt` tunnel-quic only
  - `stateful-mocking.md`: List response format (plain array → paginated `{data, meta}`); IDs (integers → UUIDs)
  - `chaos-engineering.md`: `PUT /chaos` body format (flat → nested objects)
  - `mock-verification.md`: 3 wrong response field names
  - `troubleshooting.md`: `--limit` → `--lines`; missing `--requests` flag
  - Plus fixes in concepts.md, request-matching.md, admin-api.md, configuration.md, json-schema.md, crud-api.md, integration-testing.md, sse.md, cli.md, proxy-recording.md, basic-mocks.md, import-export.md
- README examples corrected (chaos, stateful, proxy, import sections)

## [0.3.3] - 2026-02-23

### Added

- **Positional type argument** for `mockd add` — `mockd add http --path /api` now works alongside `mockd add --type http` and `mockd http add` syntax forms
- `--mutation` shorthand flag for GraphQL `add` commands (both `mockd add graphql` and `mockd graphql add`)
- `--action` flag alias for `--operation` on the unified `add` command (SOAP consistency)
- `--json` output standardization across all non-streaming CLI commands via `printResult`/`printList` helpers
- 18 `--json` contract tests ensuring structured output compliance
- Comprehensive docs overhaul: expanded all 7 protocol pages with CLI `add` sections, new chaos engineering and import/export guides, rewritten quickstart, expanded troubleshooting

### Fixed

- `mockd ps` now handles both PID file formats (serve/start vs up/down) without crashing
- Duplicate workspace name creation now correctly returns 409 Conflict
- Engine startup rollback on HTTP/HTTPS listen failures no longer leaks resources
- gRPC docs output example corrected (was showing merge format for initial creation)
- Broken links in sharing-mocks guide fixed
- Installation docs updated with Homebrew tap and get.mockd.io script

### Changed

- README overhauled with comparison table, demo GIF, and tighter structure (369→212 lines)
- goreleaser homepage corrected to mockd.io, description updated
- Homebrew auto-update job enabled in release workflow

## [0.3.2] - 2026-02-22

### Fixed

- **MQTT broker deadlock** — `Broker.Stop()` held the write lock while calling `simulator.Stop()`, which waits for goroutines that need the read lock via mochi hooks. Moved simulator shutdown outside the lock so in-flight publishes can drain
- Benchmark workflow port collision on CI runners (moved to non-colliding ports 14280/14290)
- Binary size test threshold bumped to 45 MB for Cobra + Charmbracelet huh dependencies

## [0.3.1] - 2026-02-22

### Added

- Full Cobra CLI migration — all commands use `spf13/cobra` with `charmbracelet/huh` interactive forms (zero remaining `flag.NewFlagSet` or `DisableFlagParsing`)
- Native Go e2e and integration test suites replacing BATS scripts
- Engine heartbeat protocol for `up` orchestration with split registration
- Dynamic version detection via `debug.ReadBuildInfo`

### Fixed

- **30+ engine/admin/protocol bug fixes** — chaos probability, SSE race, file store lock, MQTT ACL, validation middleware, nil guards, error codes
- Admin-url flag shadowing removed across 15+ CLI subcommands
- Engine startup teardown leak, client.go URL concatenation, workspace port validation
- SSE `recordingHookFactory` race condition (added mutex)
- Admin pagination negative offset/limit handling
- Workspace 409 conflict handling in `up.go`
- ~55 `time.Sleep` readiness waits replaced with polling helpers in tests
- HTTP client errors no longer swallowed in 32 test helpers
- Resolved all golangci-lint warnings (11→0 issues): perfsprint, bodyclose, copyloopvar, gocritic, unparam, gocyclo

### Changed

- `start --admin-url` renamed to `--register-url` to avoid shadowing root persistent flag
- CLI `serve` and `start` follow Docker patterns: `serve` = foreground, `start` = background/detached, `stop` = shutdown
- Removed dead code: `export.go` reorderArgs, SSE Graceful dead field, stale CLI bridge functions
- Moved `parseOptionalBool` to shared `query_parse.go`

## [0.3.0] - 2026-02-20

### Added

- `--cors-origins` flag on `serve` command
- `--rate-limit` flag on `serve` command
- `--no-persist` flag on `serve` command
- `--watch` flag on `serve` command
- `--match-body-contains` flag on `mockd http add` command
- `--path-pattern` flag on `mockd http add` command
- `mockd oauth add` command with `--issuer`, `--client-id`, `--client-secret`, `--oauth-user`, `--oauth-password`
- Protocol utility commands: `grpc call`, `soap call`, `soap validate`, `mqtt publish`, `mqtt subscribe`, `mqtt status`, `graphql query`, `graphql validate`
- `{{random.string(N)}}` template function
- `{{mtls.san.ip}}` and `{{mtls.san.uri}}` template variables
- `{{sequence("name", start)}}` works in all contexts (not just MQTT)
- MCP stateful tools now work through admin API
- Stateful item-level CRUD admin endpoints
- 20+ new tests (chaos, template, mTLS)

### Fixed

- SSE template expressions now resolve (was returning literal strings)
- OpenAPI body validation errors now include field paths
- Health endpoint zero timestamp in Docker (startTime race condition)
- Install script version display uses installed binary instead of PATH lookup
- Content-Type auto-detection: JSON bodies get `application/json` instead of `text/plain`
- `bodyFile` relative path resolution (resolves relative to config file directory)
- Validation mode "warn" now adds warning headers instead of blocking
- Validation mode "permissive" now skips validation entirely
- Stateful capacity error returns 507 instead of 500
- Chaos probability values clamped to [0.0, 1.0]
- Chaos per-path rules now properly preempt global rules
- SSE rate limit headers now sent when configured
- Unknown CLI command now shows helpful error with available commands
- Port range validation (0-65535) prevents misleading OS errors
- `--match-query` now accepts both `key=value` and `key:value` formats
- `{{default}}` template function now properly resolves context values

### Changed

- Template engine `New()` always initializes a SequenceStore

## [0.2.9] - 2026-02-19

### Added

- CLI UX improvements: `mockd <protocol> add` upserts by method+path, `mockd list -w`, `mockd delete --path/--method/--yes`, `mockd rm` alias
- README overhauled

### Changed

- Homebrew tap renamed to `getmockd/homebrew-tap`
- Helm chart bumped

## [0.2.8] - 2026-02-15

### Fixed

- 5 test fixes (envelope unwrapping, SOAP WSDL, E2E import)

### Notes

- 53 commits pushed to origin, all CI green
- First public push

## [0.2.7] - 2026-02-12

### Fixed

- Validation body double-read elimination (halves peak memory)
- 9 P3 cosmetic fixes: MCP version wiring, timestamps, seed IDs, export options, variable shadow, log IDs, modulo bias, Insomnia export

### Added

- Protocol interface documentation
- Template engine boundary documentation

## [0.2.6] - 2026-02-12

### Added

- 59 recording tests (WebSocket, SOAP handler, SOAP converter)
- 39 tests across various subsystems

### Fixed

- 12 P3/cosmetic fixes and dead code cleanup

### Notes

- Marketing audit on mockd-ui

## [0.2.5] - 2026-02-12

### Fixed

- 22 bug fixes across admin, portability, CLI, MCP, tracing, stateful

### Added

- 67 new tests (chaos, portability, stateful, CLI config)

## [0.2.4] - 2026-02-04

### Added

- **MCP server overhaul** — 16 multiplexed tools across all protocols with session-scoped context switching, chaos engineering, mock verification, stateful resource management, and both stdio and HTTP transports (`mockd mcp` / `mockd serve --mcp`)
- **OpenRouter AI provider** — First-class support via `MOCKD_AI_PROVIDER=openrouter` with google/gemini-2.5-flash default. Access 200+ models through a single API key
- **AI mock validation pipeline** — Generated mocks are now validated before export, with invalid mocks skipped and warnings printed
- **Multi-protocol QUIC tunnel** (`mockd tunnel-quic`) — Expose local mocks to the internet through a single QUIC connection. All 7 protocols tunneled through port 443 with zero configuration
- **gRPC tunnel support** — Native HTTP/2 with trailer forwarding via streaming chunked body framing
- **MQTT tunnel support** — MQTT connections routed via TLS ALPN on port 443
- **WebSocket tunnel support** — Bidirectional WebSocket frames proxied through QUIC
- **Tunnel authentication** — Protect tunnel URLs with token auth, HTTP Basic Auth, or IP allowlists
- **Field-level validation** — Validate request bodies with type checking, constraints, patterns, and formats
- **OAuth token introspection** (`POST /introspect`) — RFC 7662 compliant
- **YAML export for proxy recordings**
- **SSE rate limiting**

### Changed

- AI generation prompt now requests `{param}` path syntax (not Express-style `:param`)
- AI provider default `maxTokens` increased from 500 to 4096 to prevent truncated responses
- Unified admin API create endpoint now validates mocks before storing
- Chaos API `ErrorRateConfig` uses `statusCodes` array and `defaultCode`
- `/engines` endpoint includes local engine in response

### Fixed

- **MQTT broker shutdown deadlock** — `Stop()` held the broker mutex while `server.Close()` triggered hook callbacks that also needed the mutex. Fixed with atomic stopping flag and lock release before close
- **AI code fence parsing** — LLMs wrapping JSON in markdown fences now stripped before parsing
- **Express-style path params** — AI-generated `:param` paths automatically normalized to `{param}`
- **Graceful degradation** — MCP tools and CLI commands now return actionable error messages when mockd server is unreachable
- **CVE patches** — Upgraded quic-go v0.49→v0.57 and x/crypto v0.44→v0.47
- Chaos injection CLI no longer nests config under a `global` key the API ignores

### Removed

- **gRPC recording** — Only recorded traffic from mockd's own server. External service recording deferred pending community demand
- Duplicate docs workflow file

### Security

- quic-go and x/crypto upgraded to resolve known CVEs

## [0.2.0] - 2026-01-21

### Added

- gRPC/MQTT port merging: automatically merge services/topics when creating mocks on the same port
- Port conflict detection with actionable error messages
- `mockd ports` command to list all ports in use
- CLI merge output shows added and total services/topics
- Metrics path normalization for UUIDs, MongoDB ObjectIDs, and numeric IDs
- Shared test helpers for port allocation stability

### Changed

- Version reset to 0.2.0 to reflect pre-release status
- Improved CLI help text for gRPC and MQTT flags (documents merge behavior)

### Fixed

- CLI handling of merge responses (HTTP 200 vs 201)
- Bulk create and update handlers properly detect merge targets as conflicts
- Integration test port allocation stability

## [0.1.0] - 2026-01-17

### Added

- Multi-protocol mock server support: HTTP, WebSocket, gRPC, MQTT, SSE, GraphQL, SOAP
- CLI with 30+ commands for mock management
- Admin API for runtime mock configuration
- Proxy recording mode for capturing real API traffic
- Stateful mocking for simulating CRUD operations
- Chaos engineering features: latency injection, error rates, timeouts
- mTLS support with certificate matching
- OpenTelemetry tracing integration
- Prometheus metrics endpoint
- OAuth mock provider for testing auth flows
- MCP server for AI agent integration
- Shell completion support for bash, zsh, and fish
- Import/export support for OpenAPI, Postman, WireMock, HAR, and cURL formats
- Docker container support
- Helm chart for Kubernetes deployment
- kubectl-style context management for switching between mockd deployments
- Workspace CLI commands for organizing mocks into logical groups

### Security

- Config file permissions restricted to `0600` (owner read/write only)
- Config directory permissions restricted to `0700`
- Auth tokens masked in JSON output

### Notes

- Initial public release (pre-1.0)
- Licensed under Apache 2.0

[Unreleased]: https://github.com/getmockd/mockd/compare/v0.4.4...HEAD
[0.4.4]: https://github.com/getmockd/mockd/compare/v0.4.0...v0.4.4
[0.4.0]: https://github.com/getmockd/mockd/compare/v0.3.3...v0.4.0
[0.3.3]: https://github.com/getmockd/mockd/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/getmockd/mockd/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/getmockd/mockd/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/getmockd/mockd/compare/v0.2.9...v0.3.0
[0.2.9]: https://github.com/getmockd/mockd/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/getmockd/mockd/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/getmockd/mockd/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/getmockd/mockd/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/getmockd/mockd/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/getmockd/mockd/compare/v0.2.0...v0.2.4
[0.2.0]: https://github.com/getmockd/mockd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/getmockd/mockd/releases/tag/v0.1.0
