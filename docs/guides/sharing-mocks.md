# Sharing Mocks Publicly

This guide covers how to expose your local mockd server to the internet, enabling teammates, clients, or external services to access your mocks.

## Overview

mockd supports several ways to share your mocks publicly:

| Method | Cost | Protocols | Best For |
|--------|------|-----------|----------|
| [mockd tunnel (built-in)](#mockd-tunnel) | Free | All 7 protocols | Recommended for all users |
| [Third-party tunnels](#third-party-tunnels) | Free | HTTP, WebSocket, SSE | Alternative if UDP/QUIC is blocked |
| [Self-hosted relay](#self-hosted-relay) | Your infra | All protocols | Enterprise, air-gapped environments |

## Protocol Support

mockd's built-in tunnel supports **all seven protocols** through a single QUIC connection on port 443:

| Protocol | Tunnel Support | How It Works |
|----------|---------------|--------------|
| HTTP/HTTPS | Yes | Standard HTTPS |
| gRPC | Yes | Native HTTP/2 with trailers (not gRPC-web) |
| WebSocket | Yes | Upgrade proxied, bidirectional streaming |
| MQTT | Yes | TLS ALPN routing (`mqtt`) on port 443 |
| SSE | Yes | Streaming responses |
| GraphQL | Yes | Over HTTP |
| SOAP | Yes | Over HTTP |

## mockd Tunnel

mockd includes a built-in QUIC tunnel that exposes your local mocks to the internet with a single command. No signup required for anonymous tunnels (2-hour session, 100MB bandwidth).

### Quick Start

```bash
# Start your mock server
mockd serve --config mocks.yaml

# In another terminal, expose it to the internet
mockd tunnel-quic --port 4280

# Output:
# Tunnel connected!
#   HTTP:  https://a1b2c3d4.tunnel.mockd.io -> http://localhost:4280
#   Auth:  none (tunnel URL is public)
```

Your mocks are now accessible at `https://a1b2c3d4.tunnel.mockd.io`.

### Multi-Protocol Tunneling

All protocols are tunneled automatically. For MQTT, specify the broker port:

```bash
# Tunnel HTTP + MQTT
mockd tunnel-quic --port 4280 --mqtt 1883

# Output:
# Tunnel connected!
#   HTTP:  https://a1b2c3d4.tunnel.mockd.io -> http://localhost:4280
#   MQTT:  mqtts://a1b2c3d4.tunnel.mockd.io:443 -> localhost:1883 (ALPN: mqtt)

# Tunnel a gRPC server directly
mockd tunnel-quic --port 50051

# Test gRPC through the tunnel
grpcurl -d '{"name": "World"}' a1b2c3d4.tunnel.mockd.io:443 helloworld.Greeter/SayHello

# Test MQTT through the tunnel (requires TLS ALPN client)
mosquitto_pub -h a1b2c3d4.tunnel.mockd.io -p 443 --alpn mqtt \
  --capath /etc/ssl/certs -t test/hello -m "Hello!"
```

### Tunnel Authentication

Protect your tunnel from unauthorized access:

```bash
# Require bearer token
mockd tunnel-quic --port 4280 --auth-token secret123

# Require HTTP Basic Auth
mockd tunnel-quic --port 4280 --auth-basic admin:password

# Restrict by IP range
mockd tunnel-quic --port 4280 --allow-ips "10.0.0.0/8,192.168.1.0/24"
```

### Use Cases

- **Webhook development**: Expose mocks to receive webhooks from Stripe, GitHub, etc.
- **Team sharing**: Share mocks with remote teammates without deploying
- **Client demos**: Show API mocks to stakeholders with a public URL
- **CI/CD integration**: Use tunneled endpoints in integration test pipelines
- **Mobile testing**: Test mobile apps against mocks on a real device

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

## Tunnel Tiers

| Tier | Session Duration | Bandwidth | Subdomain | Signup |
|------|-----------------|-----------|-----------|--------|
| Anonymous | 2 hours | 100 MB | Random | No |
| Free | 8 hours | 1 GB | Random | Yes |
| Pro ($12/mo) | 24 hours | 5 GB/mo | Custom | Yes |
| Team ($29/mo) | Unlimited | 50 GB/mo | Custom + domain | Yes |

Anonymous tunnels require no signup or token — just run `mockd tunnel-quic --port 4280`.

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
- **Exposed**: The local port you specify with `--port` (HTTP, gRPC, WebSocket, SSE)
- **Exposed**: MQTT broker ports specified with `--mqtt` (via TLS ALPN)
- **NOT exposed**: Admin API (port 4290) — unless explicitly tunneled
- **NOT exposed**: Other local services

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

- [CLI Reference: tunnel-quic](../reference/cli.md#mockd-tunnel-quic)
- [CLI Reference: tunnel](../reference/cli.md#mockd-tunnel)
- [gRPC Mocking](grpc-mocking.md)
- [MQTT Mocking](mqtt-mocking.md)
- [WebSocket Mocking](websocket-mocking.md)
- [SSE Streaming](sse-streaming.md)
