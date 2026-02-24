---
title: Import & Export
description: Import mocks from OpenAPI, Postman, WireMock, HAR, and cURL. Export to share or migrate.
---

mockd can import mock definitions from formats you probably already have — OpenAPI specs, Postman collections, WireMock stubs, HAR files from your browser, and even cURL commands.

## Supported Import Formats

| Format | File Extension | Auto-Detected | Description |
|--------|---------------|---------------|-------------|
| OpenAPI | `.yaml`, `.json` | Yes | OpenAPI 3.x and Swagger 2.0 specs |
| Postman | `.json` | Yes | Postman Collection v2.0 and v2.1 |
| WireMock | directory | Yes | WireMock mapping JSON files |
| HAR | `.har` | Yes | HTTP Archive files (from browser DevTools) |
| cURL | — | No | cURL command strings |
| mockd | `.yaml`, `.json` | Yes | mockd's own config format |

## Importing

### From OpenAPI Specs

Import an OpenAPI 3.x or Swagger 2.0 specification. mockd creates one mock per path+method combination, using example values from the spec:

```bash
mockd import openapi.yaml
```

```
Parsed 12 mocks from openapi.yaml (format: openapi)
Imported 12 mocks to server
```

Verify what was created:

```bash
mockd list
```

If your spec doesn't have example values, mockd generates placeholder responses based on the schema types.

### From Postman Collections

Import a Postman Collection (v2.0 or v2.1 format). Export your collection from Postman first (Collection → Export → Collection v2.1):

```bash
mockd import my-api.postman_collection.json
```

Postman environment variables in requests are preserved as literal strings (e.g., `{{baseUrl}}`). You may need to adjust paths after import.

### From WireMock

If you're migrating from WireMock, point mockd at a directory containing WireMock mapping files:

```bash
mockd import ./wiremock-mappings/
```

mockd reads all `.json` files in the directory and converts WireMock's request matching and response definitions to mockd format.

### From HAR Files

Record API traffic in your browser (DevTools → Network → Export HAR), then import it:

```bash
mockd import recorded-traffic.har
```

This creates mocks for every request/response pair captured in the HAR file. Useful for quickly creating mocks that match real API behavior.

### From cURL Commands

Convert a cURL command directly into a mock:

```bash
mockd import --format curl 'curl -X POST https://api.example.com/orders -H "Content-Type: application/json" -d {"item":"widget","qty":5}'
```

```
Parsed 1 mocks from curl command (format: curl)
Imported 1 mocks to server
```

The `--format curl` flag is required since cURL commands can't be auto-detected from file content.

### Dry Run

Preview what would be imported without actually applying it:

```bash
mockd import --dry-run openapi.yaml
```

This parses and validates the file, showing you what mocks would be created, without changing anything on the server.

### Merge vs Replace

By default, imported mocks are **merged** with existing mocks. To replace all existing mocks with the imported ones:

```bash
mockd import --replace openapi.yaml
```

## Exporting

### To mockd YAML

Export your current mock configuration:

```bash
mockd export --format yaml > mocks-backup.yaml
```

### To mockd JSON

```bash
mockd export --format json > mocks-backup.json
```

### To OpenAPI

Generate an OpenAPI 3.0 spec from your current mocks:

```bash
mockd export --format openapi > api-spec.yaml
```

This is useful for documenting the API your mocks represent, or for sharing the spec with frontend teams.

## Common Workflows

### Migrate from WireMock

```bash
# Import WireMock stubs
mockd import ./wiremock-mappings/

# Verify everything looks right
mockd list

# Export as mockd config for future use
mockd export --format yaml > mockd.yaml
```

### Capture Real Traffic → Mock

```bash
# Start the MITM proxy (records traffic to disk)
mockd proxy start --port 8888

# Configure your app to use the proxy, then run it
http_proxy=http://localhost:8888 npm test

# Stop recording with Ctrl+C, then convert to mocks
mockd convert -o mocks.yaml
```

### Import from Browser

1. Open your browser's DevTools → Network tab
2. Use your application normally
3. Right-click in the Network tab → **Save all as HAR**
4. Import into mockd:

```bash
mockd import network-traffic.har
```

### CI/CD Pipeline

Use import in your test pipeline to load mocks from version-controlled specs:

```bash
# In your CI script
mockd serve &
sleep 2
mockd import ./test-fixtures/api-spec.yaml
pytest tests/
```

### Round-Trip: Export → Edit → Import

```bash
# Export current state
mockd export --format yaml > mocks.yaml

# Edit the file (add mocks, change responses, etc.)
vim mocks.yaml

# Re-import (replace mode to get a clean state)
mockd import --replace mocks.yaml
```

## Format Detection

mockd auto-detects the format of imported files based on content:

- Files with `openapi` or `swagger` keys → OpenAPI
- Files with `info.schema` matching Postman patterns → Postman Collection
- Files with `log.entries` → HAR
- Files with `request.url` + `response` at top level → WireMock
- Files with `mocks` array or `version: "1.0"` → mockd native format
- Directories → scanned for WireMock JSON mappings

You can override auto-detection with `--format`:

```bash
mockd import --format openapi ambiguous-file.json
```

## Using --json

Import and export commands support `--json` for scripting:

```bash
mockd import --json openapi.yaml
```

```json
{
  "imported": 12,
  "format": "openapi",
  "source": "openapi.yaml"
}
```
