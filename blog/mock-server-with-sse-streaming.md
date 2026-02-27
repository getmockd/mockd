---
title: Mock Server with SSE Streaming Support
published: false
description: Server-Sent Events are great for real-time features. Testing them is less great. Here's how to set up a mock SSE endpoint in under a minute.
tags: sse, testing, javascript, devtools
canonical_url: https://mockd.io/blog/mock-server-with-sse-streaming
---

Server-Sent Events are one of those technologies that's simple in concept and annoying in practice. The spec is elegant — the server sends `text/event-stream` data over a regular HTTP connection. Your browser reconnects automatically if the connection drops. No WebSocket upgrade, no special protocol.

But testing SSE is a pain. You need a server that holds connections open and sends events over time. Most mock servers don't do this. They return a response and close the connection. An SSE stream, by definition, doesn't close until you tell it to.

## A mock SSE endpoint

```bash
mockd add http --path /events --sse \
  --sse-event 'message:{"text": "hello"}' \
  --sse-event 'message:{"text": "world"}'
```

That creates an SSE endpoint on `http://localhost:4280/events`. Connect with curl to see the stream:

```bash
curl -N http://localhost:4280/events
```

```
retry:3000

event:message
data:{"text":"hello"}

event:message
data:{"text":"world"}

```

That's a valid SSE stream. The `retry:3000` tells the browser to reconnect after 3 seconds if the connection drops. Each event has a `type` (`message`) and `data` (your JSON). A blank line separates events — that's how SSE works.

## Typed events

SSE events have a `type` field that clients can filter on. This is useful when one endpoint streams different kinds of data:

```yaml
# mockd.yaml
version: "1.0"

mocks:
  - name: Stock Prices
    type: http
    http:
      matcher:
        method: GET
        path: /stock/stream
      sse:
        events:
          - type: price
            id: "1"
            data:
              symbol: AAPL
              price: 178.50
          - type: price
            id: "2"
            data:
              symbol: GOOG
              price: 141.20
          - type: alert
            id: "3"
            data:
              message: "Market closing in 30 minutes"
        timing:
          fixedDelay: 500
        lifecycle:
          maxEvents: 3
```

```bash
mockd serve --config mockd.yaml
```

Now curl it:

```bash
curl -N http://localhost:4280/stock/stream
```

```
retry:3000

event:price
id:1
data:{"price":178.5,"symbol":"AAPL"}

event:price
id:2
data:{"price":141.2,"symbol":"GOOG"}

event:alert
id:3
data:{"message":"Market closing in 30 minutes"}

```

On the client side, you listen for specific event types:

```javascript
const es = new EventSource("/stock/stream");

es.addEventListener("price", (e) => {
  const data = JSON.parse(e.data);
  console.log(`${data.symbol}: $${data.price}`);
});

es.addEventListener("alert", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Alert: ${data.message}`);
});
```

The `id` field enables resumption. If the connection drops, the browser sends `Last-Event-ID: 3` on reconnect, so the server knows where the client left off. Not all mock scenarios need this, but it's there when you do.

## Why is SSE testing hard?

The fundamental issue: SSE is a long-lived connection. Most testing approaches assume request-response semantics — you send a request, you get a response, you're done. SSE doesn't work that way.

Here's what "testing SSE" usually looks like in practice:

**Option 1: Mock the EventSource constructor.** You override `EventSource` in your test environment and manually fire events. This tests your event handling code but not the actual connection behavior.

**Option 2: Write a test server.** You write a small Express/Fastify server that sends `text/event-stream` responses. This works, but now you're maintaining a test server alongside your tests.

**Option 3: Skip it.** Test the rest of the app and hope the SSE part works in staging.

What I wanted was a middle ground: a server that sends real SSE events, is configurable without code, and dies when I don't need it anymore.

## Timing control

The `timing.fixedDelay` field controls how many milliseconds between events. This matters for testing:

- **Fast (10ms):** When you just want to verify your client handles multiple events. Tests run fast.
- **Slow (1000ms+):** When you're testing UI behavior — loading states, progressive rendering, or how your dashboard handles a stream of updates.

```yaml
timing:
  fixedDelay: 1000  # one event per second
```

## Finite vs infinite streams

By default, mockd sends all events in the list and then closes the connection. The `lifecycle.maxEvents` field controls how many total events to send.

For testing reconnection logic, you might want the stream to close after a few events so the `EventSource` triggers its reconnect behavior:

```yaml
lifecycle:
  maxEvents: 3  # close after 3 events
```

## Combining SSE with other mocks

This is where mockd being multi-protocol helps. Most real-time features combine SSE with REST endpoints:

```yaml
version: "1.0"

mocks:
  # REST endpoint to fetch initial data
  - name: Get Orders
    type: http
    http:
      matcher:
        method: GET
        path: /api/orders
      response:
        statusCode: 200
        body: '[{"id": "order_1", "status": "pending"}]'

  # SSE endpoint for real-time updates
  - name: Order Updates
    type: http
    http:
      matcher:
        method: GET
        path: /api/orders/stream
      sse:
        events:
          - type: status_change
            data:
              orderId: order_1
              status: processing
          - type: status_change
            data:
              orderId: order_1
              status: shipped
        timing:
          fixedDelay: 2000
```

Your frontend fetches `/api/orders` on load, then opens an `EventSource` on `/api/orders/stream` for live updates. Both endpoints are served by the same mockd instance.

## Limitations

- **No `Last-Event-ID` resumption.** The mock server doesn't track where a disconnected client left off. It always starts from the beginning. Fine for testing; not a real event store.
- **No conditional events.** You can't send different events based on query parameters or headers in the SSE stream (though you can use separate paths with different matchers).
- **No infinite generators yet.** If you need a stream that runs forever with changing data (like a simulated stock ticker), you're limited to a finite event list with repeat. A proper generator system is on the roadmap.

Want one of these? [Open an issue](https://github.com/getmockd/mockd/issues) — feature requests directly shape what gets built next. And if mockd saved you from writing a test SSE server, a [star on GitHub](https://github.com/getmockd/mockd) helps other developers find it.

## Links

- **GitHub:** [github.com/getmockd/mockd](https://github.com/getmockd/mockd) (Apache 2.0)
- **SSE docs:** [docs.mockd.io/protocols/sse](https://docs.mockd.io/protocols/sse/)
- **Install:** `brew install getmockd/tap/mockd` or `curl -fsSL https://get.mockd.io | sh`
