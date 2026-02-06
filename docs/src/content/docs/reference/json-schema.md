---
title: JSON Schema Reference
description: JSON Schema for mockd configuration validation, including editor setup and programmatic validation examples.
---

mockd provides JSON Schema for configuration validation. Use this schema with your editor for autocompletion and validation.

## Schema URL

```
https://getmockd.github.io/mockd/schema/config.json
```

## Editor Setup

### VS Code

Add to your `.vscode/settings.json`:

```json
{
  "json.schemas": [
    {
      "fileMatch": ["mockd.json", "mocks.json", "**/mocks/*.json"],
      "url": "https://getmockd.github.io/mockd/schema/config.json"
    }
  ]
}
```

Or add directly in your config file:

```json
{
  "$schema": "https://getmockd.github.io/mockd/schema/config.json",
  "mocks": [...]
}
```

### JetBrains IDEs

1. Open Settings -> Languages & Frameworks -> Schemas and DTDs -> JSON Schema Mappings
2. Add new mapping with URL: `https://getmockd.github.io/mockd/schema/config.json`
3. Set file pattern: `mockd.json`, `mocks.json`

### Vim/Neovim (with coc.nvim)

Add to `coc-settings.json`:

```json
{
  "json.schemas": [
    {
      "fileMatch": ["mockd.json", "mocks.json"],
      "url": "https://getmockd.github.io/mockd/schema/config.json"
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
    "server": { "$ref": "#/definitions/server" },
    "mocks": {
      "type": "array",
      "items": { "$ref": "#/definitions/mock" }
    },
    "stateful": { "$ref": "#/definitions/stateful" },
    "proxy": { "$ref": "#/definitions/proxy" }
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

```json
{
  "definitions": {
    "mock": {
      "type": "object",
      "required": ["matcher", "response"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Human-readable name for the mock"
        },
        "description": {
          "type": "string",
          "description": "Description of what this mock does"
        },
        "priority": {
          "type": "integer",
          "default": 0,
          "description": "Match priority (lower = higher)"
        },
        "matcher": { "$ref": "#/definitions/requestMatcher" },
        "response": { "$ref": "#/definitions/response" }
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
          "description": "Header matchers (values are regex patterns)"
        },
        "query": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "description": "Query parameter matchers"
        },
        "body": {
          "description": "Exact body match (any JSON value)"
        },
        "bodyContains": {
          "type": "object",
          "description": "Partial body match"
        },
        "bodyMatch": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "description": "JSONPath matchers"
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
      "matcher": {...},
      "response": {...}
    }
  ]
}
```

Custom properties are ignored by mockd but preserved in the config.

## Full Schema File

The complete schema is available at:

- **URL**: `https://getmockd.github.io/mockd/schema/config.json`
- **Repository**: `docs/schema/config.json`

## See Also

- [Configuration Reference](/reference/configuration) - Config options
- [Quickstart](/getting-started/quickstart) - Getting started
