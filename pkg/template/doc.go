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
//   - {{mtls.verified}} - "true" or "false" if certificate was verified
//
// If no mTLS identity is present, all mtls.* variables return empty strings.
package template
