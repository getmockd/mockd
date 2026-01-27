# Sharing Mocks Publicly

This guide covers how to expose your local mockd server to the internet, enabling teammates, clients, or external services to access your mocks.

## Overview

mockd supports several ways to share your mocks publicly:

| Method | Cost | Protocols | Best For |
|--------|------|-----------|----------|
| [Third-party tunnels](#third-party-tunnels) | Free | HTTP, WebSocket, SSE | Quick testing, OSS users |
| [mockd Cloud Relay](#mockd-cloud-relay) | Paid | HTTP, WebSocket, SSE | Production use, custom domains |
| [Self-hosted relay](#self-hosted-relay) | Your infra | All protocols | Enterprise, air-gapped environments |

## Protocol Support

### Currently Supported (HTTP-based)

These protocols work over standard HTTPS (port 443) and are fully supported by all relay methods:

- **HTTP/HTTPS** - REST APIs, webhooks
- **WebSocket** - Real-time bidirectional communication  
- **SSE** - Server-sent events, streaming responses
- **GraphQL** - Query/mutation endpoints (runs over HTTP)
- **SOAP** - XML web services (runs over HTTP)

### Future Consideration (TCP-based)

These protocols require raw TCP connections on custom ports. Support will be considered based on community demand:

- **gRPC** - Requires HTTP/2 or dedicated port (typically 50051)
- **MQTT** - Requires broker port (typically 1883/8883)

> **Want TCP protocol support?** [Open an issue](https://github.com/getmockd/mockd/issues) or join the discussion on Discord. We're evaluating approaches like gRPC-web proxying and TCP-over-WebSocket tunneling.

## Third-Party Tunnels

For quick testing or if you're using the OSS version, you can use free third-party tunnel services.

### localtunnel (Recommended for Testing)

[localtunnel](https://localtunnel.me) is free, requires no signup, and supports HTTP/WebSocket/SSE.

```bash
# Install
npm install -g localtunnel

# Start mockd
mockd serve

# In another terminal, create tunnel
lt --port 4280

# Output: your url is: https://random-name.loca.lt
```

**Testing your tunnel:**

```bash
# Add a mock
mockd add --path /api/users --body '{"users": [{"id": 1, "name": "Alice"}]}'

# Test via tunnel (note the bypass header)
curl -H "bypass-tunnel-reminder: true" https://random-name.loca.lt/api/users
```

**WebSocket through localtunnel:**

```javascript
// localtunnel supports WebSocket upgrade
const ws = new WebSocket('wss://random-name.loca.lt/ws/chat');
ws.onopen = () => ws.send('Hello from tunnel!');
```

### ngrok

[ngrok](https://ngrok.com) offers a free tier with signup. Better reliability than localtunnel.

```bash
# Install (see ngrok.com/download)
# Configure auth token (one-time)
ngrok config add-authtoken YOUR_TOKEN

# Start tunnel
ngrok http 4280
```

**ngrok features:**
- Stable URLs (paid)
- Request inspection dashboard
- Custom domains (paid)

### Cloudflare Tunnel

If you have a Cloudflare account with a domain, [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) provides free, unlimited HTTP tunneling.

```bash
# Install cloudflared
# See: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/

# Quick tunnel (temporary URL)
cloudflared tunnel --url http://localhost:4280

# Named tunnel (persistent, requires setup)
cloudflared tunnel create mockd
cloudflared tunnel route dns mockd mocks.yourdomain.com
cloudflared tunnel run mockd
```

### Comparison

| Feature | localtunnel | ngrok (free) | Cloudflare Tunnel |
|---------|-------------|--------------|-------------------|
| Cost | Free | Free (limited) | Free |
| Signup required | No | Yes | Yes (+ domain) |
| Stable URLs | No | No (paid only) | Yes |
| WebSocket | Yes | Yes | Yes |
| SSE | Yes | Yes | Yes |
| TCP tunnels | No | Paid only | No |
| Request inspection | No | Yes | Yes |

## mockd Cloud Relay

> **Coming Soon** - The mockd Cloud Relay provides a managed tunnel service with custom domains, team sharing, and usage analytics.

### Features (Planned)

- **Custom subdomains**: `your-name.mockd.dev`
- **Custom domains**: `mocks.yourcompany.com`
- **Team sharing**: Share tunnel access with teammates
- **Usage analytics**: Request counts, bandwidth
- **Auto-reconnect**: Survives network interruptions

### Usage

```bash
# Login to mockd cloud
mockd auth login

# Start tunnel
mockd tunnel

# Output:
# Tunnel connected!
# Public URL: https://your-name.mockd.dev
# Local server: http://localhost:4280
```

### Pricing

| Tier | Tunnel Access | Bandwidth | Custom Domain |
|------|---------------|-----------|---------------|
| Free | No | - | - |
| Pro | Yes | 5 GB/mo | No |
| Team | Yes | 50 GB/mo | Yes |

## Self-Hosted Relay

For enterprise users or those needing full control, you can run your own relay server.

### Docker Compose

```yaml
version: '3.8'
services:
  mockd-relay:
    image: ghcr.io/getmockd/relay:latest
    ports:
      - "80:80"
      - "443:443"
    environment:
      - MOCKD_DOMAIN=relay.yourcompany.com
      - MOCKD_TLS_EMAIL=admin@yourcompany.com
    volumes:
      - caddy_data:/data
      
volumes:
  caddy_data:
```

### Connecting to Self-Hosted Relay

```bash
mockd tunnel --relay wss://relay.yourcompany.com/tunnel --token YOUR_TOKEN
```

### Kubernetes / Helm

```bash
helm repo add mockd https://charts.mockd.dev
helm install mockd-relay mockd/relay \
  --set domain=relay.yourcompany.com \
  --set tls.email=admin@yourcompany.com
```

## Security Considerations

### Protecting Your Tunnel

By default, tunnels are public. Add authentication for sensitive mocks:

```bash
# Require bearer token
mockd tunnel --auth-token secret123

# Require HTTP Basic Auth  
mockd tunnel --auth-basic admin:password

# Restrict by IP
mockd tunnel --allow-ips "10.0.0.0/8,192.168.1.0/24"
```

### What Gets Exposed

When you create a tunnel:
- **Exposed**: All enabled mocks on your HTTP port (default 4280)
- **NOT exposed**: Admin API (port 4290) - unless explicitly tunneled
- **NOT exposed**: Other protocol ports (gRPC, MQTT) - HTTP tunnel only

### Best Practices

1. **Use authentication** for any non-trivial testing
2. **Disable mocks** you don't want publicly accessible
3. **Monitor usage** via mockd logs or cloud dashboard
4. **Stop tunnels** when not actively needed
5. **Use short-lived tunnels** for demos rather than persistent ones

## Troubleshooting

### Tunnel connects but requests fail

Check that mockd is running and accessible locally:

```bash
curl http://localhost:4280/health
```

### WebSocket connections drop

Some tunnel providers have idle timeouts. Enable keepalive:

```bash
# In your WebSocket mock config
mockd add --type websocket --path /ws --keepalive 30
```

### SSE stream cuts off

Ensure your tunnel provider supports long-lived connections. localtunnel and ngrok both support SSE.

### "No mock matched" errors

The tunnel is working but no mock matches the request. Check your mock configuration:

```bash
mockd list
```

## See Also

- [CLI Reference: cloud commands](../reference/cli.md#cloud-commands)
- [WebSocket Mocking](websocket-mocking.md)
- [SSE Streaming](sse-streaming.md)
