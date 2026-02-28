---
title: JSON Schema Reference
description: JSON Schema for mockd configuration validation, including editor setup and programmatic validation examples.
---

mockd provides a JSON Schema (Draft-07) for configuration validation. Use this schema with your editor for instant autocompletion and validation of `mockd.yaml` and `mockd.json` config files.

## Schema URL

```
https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json
```

The schema covers all 7 protocols (HTTP, GraphQL, gRPC, WebSocket, MQTT, SOAP, OAuth), stateful resources, custom operations, chaos config, and server settings.

## Editor Setup

### VS Code

Add to your `.vscode/settings.json`:

```json
{
  "json.schemas": [
    {
      "fileMatch": ["mockd.json", "mockd.yaml", "mockd.yml", "mocks.json", "mocks.yaml"],
      "url": "https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json"
    }
  ],
  "yaml.schemas": {
    "https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json": ["mockd.yaml", "mockd.yml", "mocks.yaml"]
  }
}
```

Or add directly in your JSON config file:

```json
{
  "$schema": "https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json",
  "mocks": []
}
```

For YAML config files, add a schema comment at the top:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json
mocks:
  - type: http
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        body: '[]'
```

### JetBrains IDEs

1. Open Settings -> Languages & Frameworks -> Schemas and DTDs -> JSON Schema Mappings
2. Add new mapping with URL: `https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json`
3. Set file pattern: `mockd.json`, `mockd.yaml`, `mocks.json`, `mocks.yaml`

### Vim/Neovim (with coc.nvim or nvim-lspconfig)

Add to `coc-settings.json`:

```json
{
  "json.schemas": [
    {
      "fileMatch": ["mockd.json", "mocks.json"],
      "url": "https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json"
    }
  ]
}
```

## Schema Definition

### Root Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "mockd Configuration",
  "type": "object",
  "properties": {
    "version": { "type": "string" },
    "name": { "type": "string" },
    "server": { "$ref": "#/definitions/server" },
    "mocks": {
      "type": "array",
      "items": { "$ref": "#/definitions/mock" }
    },
    "statefulResources": {
      "type": "array",
      "items": { "$ref": "#/definitions/statefulResource" }
    }
  }
}
```

### Server Definition

```json
{
  "definitions": {
    "server": {
      "type": "object",
      "properties": {
        "port": {
          "type": "integer",
          "minimum": 1,
          "maximum": 65535,
          "default": 4280
        },
        "host": {
          "type": "string",
          "default": "localhost"
        },
        "adminEnabled": {
          "type": "boolean",
          "default": true
        },
        "adminPort": {
          "type": "integer",
          "minimum": 1,
          "maximum": 65535,
          "default": 4290
        },
        "tls": { "$ref": "#/definitions/tls" },
        "cors": { "$ref": "#/definitions/cors" }
      }
    }
  }
}
```

### Mock Definition

A mock wraps protocol-specific config under a type key. The `id` and `type`
fields are auto-generated if omitted in config files.

```json
{
  "definitions": {
    "mock": {
      "type": "object",
      "properties": {
        "id": {
          "type": "string",
          "description": "Unique mock ID (auto-generated if omitted)"
        },
        "type": {
          "type": "string",
          "enum": ["http", "graphql", "grpc", "websocket", "mqtt", "soap", "oauth"],
          "description": "Protocol type (inferred from spec field if omitted)"
        },
        "name": {
          "type": "string",
          "description": "Human-readable name for the mock"
        },
        "enabled": {
          "type": "boolean",
          "default": true,
          "description": "Whether this mock is active"
        },
        "priority": {
          "type": "integer",
          "default": 0,
          "description": "Match priority (higher = matched first)"
        },
        "http": { "$ref": "#/definitions/httpSpec" },
        "graphql": { "$ref": "#/definitions/graphqlSpec" },
        "grpc": { "$ref": "#/definitions/grpcSpec" },
        "websocket": { "$ref": "#/definitions/websocketSpec" },
        "mqtt": { "$ref": "#/definitions/mqttSpec" },
        "soap": { "$ref": "#/definitions/soapSpec" },
        "oauth": { "$ref": "#/definitions/oauthSpec" }
      }
    },
    "httpSpec": {
      "type": "object",
      "description": "HTTP mock specification",
      "properties": {
        "matcher": { "$ref": "#/definitions/requestMatcher" },
        "response": { "$ref": "#/definitions/response" },
        "priority": { "type": "integer", "default": 0 }
      }
    }
  }
}
```

### Request Matcher Definition

```json
{
  "definitions": {
    "requestMatcher": {
      "type": "object",
      "properties": {
        "method": {
          "type": "string",
          "enum": ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"],
          "description": "HTTP method to match"
        },
        "path": {
          "type": "string",
          "description": "URL path pattern (supports {param} syntax)"
        },
        "headers": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "description": "Header matchers (exact match or glob patterns with *)"
        },
        "queryParams": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "description": "Query parameter matchers (exact match)"
        },
        "bodyEquals": {
          "type": "string",
          "description": "Exact body match (full string comparison)"
        },
        "bodyContains": {
          "type": "string",
          "description": "Substring body match"
        },
        "bodyPattern": {
          "type": "string",
          "description": "Regex body match"
        },
        "bodyJsonPath": {
          "type": "object",
          "additionalProperties": {},
          "description": "JSONPath condition matchers (e.g., {\"$.user.role\": \"admin\"})"
        }
      }
    }
  }
}
```

### Response Definition

```json
{
  "definitions": {
    "response": {
      "type": "object",
      "properties": {
        "statusCode": {
          "type": "integer",
          "minimum": 100,
          "maximum": 599,
          "default": 200,
          "description": "HTTP status code"
        },
        "headers": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "description": "Response headers"
        },
        "body": {
          "description": "Response body (string, object, or array)"
        },
        "bodyFile": {
          "type": "string",
          "description": "Load body from file path"
        },
        "delayMs": {
          "type": "integer",
          "minimum": 0,
          "default": 0,
          "description": "Response delay in milliseconds"
        }
      }
    }
  }
}
```

### Stateful Resources Definition

```json
{
  "definitions": {
    "statefulResources": {
      "type": "array",
      "items": { "$ref": "#/definitions/statefulResource" }
    },
    "statefulResource": {
      "type": "object",
      "required": ["name", "basePath"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Resource name (e.g., users, products)"
        },
        "basePath": {
          "type": "string",
          "description": "Base path for CRUD endpoints (e.g., /api/users)"
        },
        "idField": {
          "type": "string",
          "default": "id",
          "description": "Field used as the unique identifier"
        },
        "parentField": {
          "type": "string",
          "default": "",
          "description": "Optional parent field for nested resources"
        },
        "seedData": {
          "type": "array",
          "items": { "type": "object" },
          "description": "Initial data to populate the resource"
        }
      }
    }
  }
}
```

> **Note:** Resource definitions and seed data are persisted to the admin file store. Runtime data (CRUD operations) is in-memory only and resets to seed data on restart.

## Validation

### CLI Validation

```bash
mockd validate mocks.json
```

### Programmatic Validation

Using Node.js with ajv:

```javascript
const Ajv = require('ajv');
const schema = require('./mockd-schema.json');
const config = require('./mocks.json');

const ajv = new Ajv();
const validate = ajv.compile(schema);
const valid = validate(config);

if (!valid) {
  console.error(validate.errors);
}
```

Using Python with jsonschema:

```python
import json
from jsonschema import validate, ValidationError

with open('mockd-schema.json') as f:
    schema = json.load(f)

with open('mocks.json') as f:
    config = json.load(f)

try:
    validate(instance=config, schema=schema)
    print("Configuration is valid")
except ValidationError as e:
    print(f"Validation error: {e.message}")
```

## Custom Schema Extensions

Add custom properties with `x-` prefix:

```json
{
  "mocks": [
    {
      "x-team": "backend",
      "x-version": "2.0",
      "type": "http",
      "http": {
        "matcher": {"method": "GET", "path": "/api/data"},
        "response": {"statusCode": 200, "body": "{}"}
      }
    }
  ]
}
```

Custom properties are ignored by mockd but preserved in the config.

## Full Schema File

The complete schema is available at:

- **URL**: `https://raw.githubusercontent.com/getmockd/mockd/main/schema/mockd.schema.json`
- **Repository**: `schema/mockd.schema.json`

## See Also

- [Configuration Reference](/reference/configuration) - Config options
- [Quickstart](/getting-started/quickstart) - Getting started
