# mockd Examples

This directory contains example configurations and programs demonstrating mockd usage.

## Go Examples

### basic-usage/

Demonstrates programmatic mock server creation and configuration.

```bash
cd basic-usage
go run main.go
```

This example creates a mock server on port 4280 with user CRUD endpoints and an admin API on port 4290.

Test it:
```bash
curl http://localhost:4280/api/users
curl -X POST http://localhost:4280/api/users
curl http://localhost:4280/api/users/123
```

### with-config-file/

Demonstrates loading mocks from a JSON configuration file.

```bash
cd with-config-file
go run main.go mocks.json
```

Add `--save-on-exit` to save configuration changes on shutdown.

## Configuration Files

| File | Description |
|------|-------------|
| `basic-mock.json` | Simple mock configuration with GET and POST endpoints |
| `advanced-matching.json` | Complex matching with headers, query params, and body |
| `admin-api-usage.sh` | Shell script demonstrating admin API usage |

## Running Examples

### Using Configuration Files

```bash
cd with-config-file
go run main.go ../basic-mock.json
```

### Admin API

```bash
# Start a server, then run the admin API examples
./admin-api-usage.sh
```

## Configuration Format

All JSON configuration files follow this structure:

```json
{
  "version": "1.0",
  "name": "Collection Name",
  "mocks": [
    {
      "id": "unique-id",
      "name": "Human readable name",
      "priority": 10,
      "matcher": {
        "method": "GET",
        "path": "/api/endpoint"
      },
      "response": {
        "statusCode": 200,
        "body": "{\"data\": \"value\"}"
      }
    }
  ]
}
```

See the [Data Model](../specs/001-local-mock-engine/data-model.md) for full schema documentation.
