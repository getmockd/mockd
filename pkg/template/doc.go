// Package template provides response body templating for mock responses.
// It supports variable substitution like {{now}}, {{uuid}}, {{request.body.field}}.
//
// # Built-in Variables
//
// Time-related:
//   - {{now}} - Current time in RFC3339 format
//   - {{timestamp}} - Current Unix timestamp
//
// Random values:
//   - {{uuid}} - Random UUID v4
//   - {{random}} - Random 8-character hex string
//   - {{random.string}} - Random 10-character alphanumeric string
//   - {{random.string(N)}} - Random N-character alphanumeric string
//   - {{random.int}} - Random integer 0-100
//   - {{random.int(min, max)}} - Random integer in range [min, max]
//   - {{random.float}} - Random float 0.0-1.0
//   - {{random.float(min, max)}} - Random float in range
//   - {{random.float(min, max, precision)}} - Random float with decimal precision
//
// # Request Variables
//
// Access request data with the {{request.*}} prefix:
//   - {{request.method}} - HTTP method
//   - {{request.path}} - Request path
//   - {{request.url}} - Full request URL
//   - {{request.rawBody}} - Raw request body
//   - {{request.body.field}} - Parsed JSON body field
//   - {{request.query.param}} - Query parameter value
//   - {{request.header.name}} - Request header value
//   - {{request.pathParam.name}} - Path parameter value
//
// # mTLS Variables
//
// When mTLS is enabled, client certificate data is available with {{mtls.*}}:
//   - {{mtls.cn}} - Client certificate Common Name
//   - {{mtls.o}} - First Organization
//   - {{mtls.ou}} - First Organizational Unit
//   - {{mtls.serial}} - Certificate serial number
//   - {{mtls.fingerprint}} - SHA256 fingerprint
//   - {{mtls.issuer.cn}} - Issuer Common Name
//   - {{mtls.notBefore}} - Certificate validity start (RFC3339)
//   - {{mtls.notAfter}} - Certificate validity end (RFC3339)
//   - {{mtls.san.dns}} - First DNS Subject Alternative Name
//   - {{mtls.san.email}} - First email Subject Alternative Name
//   - {{mtls.san.ip}} - First IP address Subject Alternative Name
//   - {{mtls.san.uri}} - First URI Subject Alternative Name
//   - {{mtls.verified}} - "true" or "false" if certificate was verified
//
// If no mTLS identity is present, all mtls.* variables return empty strings.
//
// # Functions
//
// Transform or provide fallback values:
//   - {{upper(value)}} or {{upper value}} - Convert to uppercase
//   - {{lower(value)}} or {{lower value}} - Convert to lowercase
//   - {{default(value, "fallback")}} or {{default value "fallback"}} - Use fallback if value is empty
//
// The default function resolves its first argument as a context path
// (request.*, mtls.*, payload.*, topic, uuid, etc.) and returns the
// fallback string if the resolved value is empty.
//
// # Sequences
//
// Auto-incrementing counters available in all contexts (HTTP, GraphQL,
// SSE, SOAP, WebSocket, MQTT):
//   - {{sequence("name")}} - Auto-incrementing counter starting at 1
//   - {{sequence("name", start)}} - Auto-incrementing counter starting at start
//
// Each named sequence is independent and persists for the lifetime of the
// engine instance.
//
// # Template Engine Boundary
//
// This package is the primary template engine for HTTP, GraphQL, SSE, SOAP,
// and WebSocket response bodies. The MQTT handler (pkg/mqtt) has its own
// independent template processing for topic patterns and message payloads,
// which handles MQTT-specific concerns like topic wildcards and QoS-aware
// substitution. Consolidating the two engines is a post-launch consideration;
// for now, changes to template syntax here do not automatically propagate
// to MQTT templates.
package template
