# Multi-Protocol Demo

This demo showcases mockd's ability to mock multiple protocols simultaneously:

- **HTTP** - REST APIs with templating
- **WebSocket** - Real-time bidirectional communication
- **SSE** - Server-Sent Events streaming
- **gRPC** - Protocol buffers with streaming
- **MQTT** - IoT pub/sub messaging
- **GraphQL** - Query/mutation API
- **SOAP** - XML web services

## Quick Start

```bash
# Start the mock server
mockd serve -c examples/multi-protocol-demo/config.json

# Or with verbose logging
mockd serve -c examples/multi-protocol-demo/config.json -v
```

## Endpoints

### HTTP (port 4280)

```bash
# Get products
curl http://localhost:4280/api/products

# Get single product
curl http://localhost:4280/api/products/prod-001

# Login
curl -X POST http://localhost:4280/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user","password":"pass"}'

# Protected endpoint (with token)
curl http://localhost:4280/api/protected \
  -H "Authorization: Bearer eyJ..."

# Metrics
curl http://localhost:4280/metrics

# Webhook receiver
curl -X POST http://localhost:4280/webhooks/events \
  -H "Content-Type: application/json" \
  -d '{"event":"order.created","data":{}}'
```

### SSE (port 4280)

```bash
# AI chat streaming (OpenAI-compatible)
curl -X POST http://localhost:4280/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'

# Stock ticker
curl http://localhost:4280/sse/stocks
```

### WebSocket (port 4280)

```bash
# Trading WebSocket
websocat ws://localhost:4280/ws/trading
# Send: {"action":"subscribe","channel":"AAPL"}

# Collaborative editor
websocat ws://localhost:4280/ws/collab
# Send: {"type":"join","document":"doc-001"}

# IoT device control
websocat ws://localhost:4280/ws/iot
# Send: {"command":"status","deviceId":"device-001"}
```

### GraphQL (port 4280)

```bash
# Products query
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"{ products { id name price } }"}'

# Add to cart mutation
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { addToCart(userId: \"1\", productId: \"prod-001\", quantity: 2) { id quantity } }"}'
```

### gRPC (ports 50051, 50052)

```bash
# Order service (requires grpcurl)
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"customer_id":"cust-123"}' localhost:50051 order.OrderService/CreateOrder

# Stock ticker streaming
grpcurl -plaintext -d '{"symbol":"AAPL"}' localhost:50052 stocks.StockService/StreamQuotes
```

### SOAP (port 4280)

```bash
# Process payment
curl -X POST http://localhost:4280/soap/payment \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://payments.example.com/ProcessPayment" \
  -d '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
    <soap:Body>
      <ProcessPayment xmlns="http://payments.example.com/">
        <Amount>99.99</Amount>
        <Currency>USD</Currency>
      </ProcessPayment>
    </soap:Body>
  </soap:Envelope>'

# ERP inventory
curl -X POST http://localhost:4280/soap/erp \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://erp.example.com/GetInventory" \
  -d '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
    <soap:Body>
      <GetInventory xmlns="http://erp.example.com/"/>
    </soap:Body>
  </soap:Envelope>'
```

### MQTT (port 1883)

```bash
# Subscribe to sensor data (requires mosquitto_sub)
mosquitto_sub -h localhost -p 1883 -u device -P device123 -t "sensors/#"

# Publish to factory machine
mosquitto_pub -h localhost -p 1883 -u admin -P admin123 \
  -t "factory/machines/machine-001/status" \
  -m '{"status":"running","temp":45}'

# Fleet tracking
mosquitto_sub -h localhost -p 1883 -u admin -P admin123 -t "fleet/vehicles/+/location"
```

## Admin API (port 4290)

```bash
# View request log
curl http://localhost:4290/requests

# Filter by protocol
curl "http://localhost:4290/requests?protocol=websocket"

# Clear logs
curl -X DELETE http://localhost:4290/requests
```

## Demo Scenarios

### E-Commerce Flow
1. Browse products (HTTP)
2. Add to cart (GraphQL mutation)
3. Create order (gRPC)
4. Process payment (SOAP)
5. Track shipment (WebSocket real-time updates)

### IoT Monitoring
1. Connect devices (MQTT)
2. Subscribe to sensors (MQTT pub/sub)
3. Control devices (WebSocket commands)
4. View telemetry (SSE streaming)

### Trading Platform
1. Get quotes (gRPC unary)
2. Stream real-time prices (gRPC server streaming)
3. Execute trades (WebSocket bidirectional)
4. Receive confirmations (SSE events)
