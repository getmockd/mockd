---
title: Troubleshooting Guide
description: Solutions for common issues when using mockd, including mock matching problems, port conflicts, and protocol-specific debugging.
---

This guide covers common issues you may encounter when using mockd and how to resolve them.

## Quick Diagnostics

Before diving into specific issues, run these commands:

```bash
# Full diagnostic report
mockd doctor

# Check server health
mockd health

# Show what's running
mockd status

# List loaded mocks
mockd list

# View recent request logs
mockd logs --limit 10
```

## Mock Not Matching

Common reasons why your mock might not be matching:

1. **Method mismatch** - Check GET vs POST vs PUT
2. **Path mismatch** - Check exact path, trailing slashes
3. **Mock not enabled** - Check `enabled: true`
4. **Priority/ordering** - More specific mocks should come first
5. **Headers required** - Some mocks require specific headers

### Debug Steps

```bash
# Check what mocks are loaded
mockd list

# Show full IDs and paths without truncation
mockd list --no-truncate

# View request logs to see what's being received
mockd logs

# Delete a problematic mock by path
mockd delete --path /api/users --method GET

# Use doctor to diagnose issues
mockd doctor
```

### Checklist

- Verify the HTTP method matches exactly (case-sensitive)
- Check for trailing slashes in paths (`/api/users` vs `/api/users/`)
- Confirm the mock file has `enabled: true` or the field is omitted (defaults to true)
- Review mock priority if you have overlapping patterns
- Check if the mock requires specific headers that aren't being sent

## Port Already in Use

If mockd fails to start with a "port already in use" error:

```bash
# Check what's using the port (macOS/Linux)
lsof -i :4280

# Check what's using the port (Windows)
netstat -ano | findstr :4280

# Check all mockd default ports
mockd ports

# Use a different port
mockd serve --port 3000
```

### Default Ports

| Port | Service | Override Flag |
|------|---------|---------------|
| 4280 | Mock server (HTTP, GraphQL, WebSocket, SOAP, SSE) | `--port` |
| 4290 | Admin API | `--admin-port` |
| 50051 | gRPC (configurable per mock) | In YAML config |
| 1883 | MQTT (configurable per mock) | In YAML config |

### Solutions

- Kill the existing process using the port
- Choose a different port with the `--port` flag
- Check if another mockd instance is already running: `mockd ps`
- Stop a running instance: `mockd stop`

## Server Won't Start

If the server fails to start:

- Check config file syntax with `mockd validate mockd.yaml`
- Run `mockd doctor` for diagnostics
- Check permissions on data directory
- Verify mock files are valid YAML/JSON

### Common Causes

| Issue | Solution |
|-------|----------|
| Invalid YAML syntax | Run `mockd validate mockd.yaml` |
| Missing required fields | Check mock schema requirements |
| Permission denied | Ensure write access to data directory |
| Invalid port number | Use a port between 1024-65535 |
| Proto file not found | Check `protoFile` path is relative to config |

## HTTP Issues

### Response Body Not Matching Expected

```bash
# Check the exact mock configuration
mockd get <mock-id> --json

# Check request logs for what was actually matched
mockd logs --limit 5
```

### Wrong Mock Matched

When multiple mocks could match, priority matters:

1. Mocks with more specific paths win (`/api/users/1` > `/api/users/{id}`)
2. Mocks with more matchers win (path + headers > path only)
3. Earlier mocks in config win if priority is equal

```bash
# List all mocks and their paths
mockd list --no-truncate
```

### Request Body Matching Not Working

- Ensure `Content-Type` header is sent with the request
- JSON body matching requires valid JSON in both request and matcher
- Check for extra whitespace or field ordering differences

## GraphQL Issues

### Query Returns Empty Data

- Verify the operation name matches a resolver in your config
- Check that the schema defines the query type you're calling
- Ensure introspection is enabled if your client requires it

```bash
# Test a GraphQL query directly
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ users { id name } }"}'

# Check introspection
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { types { name } } }"}'
```

### Schema Validation Errors

```bash
# Validate a schema file
mockd graphql validate schema.graphql
```

### Resolver Not Found

- Resolver keys must be `Type.field` format: `Query.users`, `Mutation.createUser`
- Field names are case-sensitive
- Check that the schema defines the operation you're resolving

## gRPC Issues

### Proto File Errors

- Proto files must be valid protobuf syntax
- Check import paths if your proto references other files
- Verify the package name matches your service configuration

```bash
# List services from a proto file
mockd grpc list api.proto

# Verify gRPC server is running
grpcurl -plaintext localhost:50051 list
```

### "Service not found" Errors

- Service names must be fully qualified: `package.ServiceName` (e.g., `users.UserService`)
- Method names are case-sensitive
- Check that the proto file path is correct relative to your config file

### Reflection Not Working

```bash
# Test gRPC reflection
grpcurl -plaintext localhost:50051 list

# If reflection is disabled, use the proto file directly
grpcurl -plaintext -proto ./service.proto \
  localhost:50051 package.Service/Method
```

Ensure `reflection: true` is set in your gRPC config.

### Port Conflicts Between gRPC Mocks

Multiple gRPC mocks on the same port are automatically merged. If you see conflicts:

```bash
# Check which mocks are on port 50051
mockd list --type grpc
```

## WebSocket Issues

### Connection Refused

- Verify the WebSocket path matches your mock: `/ws` vs `/ws/chat`
- Ensure you're using the correct scheme: `ws://` (not `http://`)
- Check that mockd is running on the expected port

```bash
# Test WebSocket endpoint exists
curl -i http://localhost:4280/ws

# Connect with mockd CLI
mockd websocket connect ws://localhost:4280/ws
```

### Subprotocol Mismatch

If your client requires a specific subprotocol:

```yaml
websocket:
  path: /ws
  subprotocols:
    - chat.v1
  requireSubprotocol: true  # Rejects connections without matching subprotocol
```

Check the `Sec-WebSocket-Protocol` header in your client connection.

### Messages Not Matching

- Matchers are evaluated in order — first match wins
- Check `matchType`: `exact`, `contains`, `prefix`, `regex`, `json`
- For JSON matching, verify the JSONPath expression is correct

### Connection Drops

- Enable heartbeat/keepalive in your WebSocket mock config
- Check `idleTimeout` — connections are closed after inactivity
- Some proxies/firewalls drop idle WebSocket connections

```yaml
websocket:
  heartbeat:
    enabled: true
    interval: "30s"
    timeout: "10s"
```

## MQTT Issues

### Can't Connect to Broker

- Default MQTT port is **1883** (not 4280)
- Check if authentication is required

```bash
# Test connection with mosquitto
mosquitto_sub -h localhost -p 1883 -t "test/#" -v

# With authentication
mosquitto_sub -h localhost -p 1883 -u user -P pass -t "test/#"
```

### Not Receiving Messages

- Check topic name spelling and wildcards (`+` for single level, `#` for multi-level)
- Verify QoS level — QoS 0 messages may be lost
- Check if retained messages are configured

```bash
# Subscribe to all topics
mockd mqtt subscribe "#"

# Subscribe with specific QoS
mockd mqtt subscribe --qos 1 "sensors/#"
```

### Authentication Denied

- Verify username/password match the `auth.users` config
- Check ACL rules — the user may not have access to the requested topic
- ACL access levels: `read`, `write`, `readwrite`/`all`

## SOAP Issues

### "No Operation Matched"

- Verify the `SOAPAction` header matches the configured `soapAction` value
- SOAPAction matching is exact (case-sensitive)

```bash
# Include the SOAPAction header
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://example.com/GetUser" \
  -d @request.xml
```

### WSDL Not Serving

- Access the WSDL by appending `?wsdl` to the endpoint URL
- Check that the `wsdl` or `wsdlFile` field is set in your config

```bash
curl http://localhost:4280/soap/UserService?wsdl
```

### XPath Matching Not Working

- XPath expressions are evaluated against the SOAP body (inside `<soap:Body>`)
- Use `//Element/text()` to match element text content
- Namespace prefixes in XPath may need to match the request

## SSE Issues

### Stream Ends Immediately

- Check the `lifecycle.maxEvents` setting — it may be too low
- Verify `lifecycle.timeout` is long enough for your use case
- Ensure your client sends `Accept: text/event-stream`

```bash
# Test SSE connection
curl -N -H "Accept: text/event-stream" http://localhost:4280/events
```

### Events Not Repeating

- Set `timing.repeat` or configure lifecycle for continuous streams
- Check if `maxEvents` is limiting the number of events sent

### OpenAI Template Issues

- The `openai-chat` template expects `POST` method
- Verify `templateParams.tokens` is an array of strings
- Check that `includeDone: true` is set if your client expects `[DONE]`

## Performance Issues

If mockd is running slowly:

- Check mock count (large lists slow down matching)
- Consider using more specific matchers
- Review regex patterns for efficiency

### Performance Optimization

```bash
# Check current mock count and status
mockd status

# Show all ports in use
mockd ports
```

### Best Practices

- Use exact path matching when possible instead of regex
- Group related mocks into separate files
- Remove unused or disabled mocks
- Use path parameters (`{id}`) instead of regex for dynamic segments

## Getting Help

If you're still experiencing issues:

1. Run `mockd doctor` for a comprehensive diagnostic report
2. Check the logs with `mockd logs`
3. Search existing issues on [GitHub](https://github.com/getmockd/mockd/issues)
4. Open a new issue with diagnostic output and reproduction steps

Include this information when reporting issues:

```bash
mockd version
mockd doctor
mockd status --json
```
