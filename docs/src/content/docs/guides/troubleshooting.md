---
title: Troubleshooting Guide
description: Solutions for common issues when using mockd, including mock matching problems, port conflicts, and performance optimization.
---

This guide covers common issues you may encounter when using mockd and how to resolve them.

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

# View request logs to see what's being received
mockd logs

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

# Use a different port
mockd start --port 3000
```

### Solutions

- Kill the existing process using the port
- Choose a different port with the `--port` flag
- Check if another mockd instance is already running

## Server Won't Start

If the server fails to start:

- Check config file syntax with `mockd config validate`
- Run `mockd doctor` for diagnostics
- Check permissions on data directory
- Verify mock files are valid YAML/JSON

### Common Causes

| Issue | Solution |
|-------|----------|
| Invalid YAML syntax | Use a YAML linter to validate files |
| Missing required fields | Check mock schema requirements |
| Permission denied | Ensure write access to data directory |
| Invalid port number | Use a port between 1024-65535 |

## gRPC Issues

When working with gRPC mocks:

- Proto files must be valid and parseable
- Reflection must be enabled for grpcurl to work
- Service and method names are case-sensitive
- Check that proto import paths are correct

### Debugging gRPC

```bash
# Test gRPC reflection
grpcurl -plaintext localhost:4280 list

# Check specific service methods
grpcurl -plaintext localhost:4280 describe your.service.Name
```

## WebSocket Issues

For WebSocket connection problems:

- Check subprotocol matching between client and mock
- Verify upgrade headers are correct
- Ensure the WebSocket path matches the mock configuration
- Check for proxy/firewall issues that may block WebSocket upgrades

### WebSocket Debugging Tips

- Use browser DevTools Network tab to inspect WebSocket handshake
- Check the `Sec-WebSocket-Protocol` header if using subprotocols
- Verify the connection URL scheme (`ws://` vs `wss://`)

## Performance Issues

If mockd is running slowly:

- Check mock count (large lists slow down matching)
- Enable metrics endpoint for monitoring
- Consider using more specific matchers
- Review regex patterns for efficiency

### Performance Optimization

```bash
# Check current mock count
mockd list | wc -l

# Enable metrics for monitoring
mockd start --metrics

# View performance metrics
curl http://localhost:4280/__mockd/metrics
```

### Best Practices

- Use exact path matching when possible instead of regex
- Group related mocks into separate files
- Remove unused or disabled mocks
- Use path parameters instead of regex for dynamic segments

## Getting Help

If you're still experiencing issues:

1. Run `mockd doctor` for a comprehensive diagnostic report
2. Check the logs with `mockd logs --level debug`
3. Search existing issues on GitHub
4. Open a new issue with diagnostic output and reproduction steps
