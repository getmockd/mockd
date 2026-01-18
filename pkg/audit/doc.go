// Package audit provides audit logging infrastructure for the mock server engine.
//
// The audit package captures detailed information about requests, responses,
// and mock matching for debugging, compliance, and observability purposes.
//
// # Basic Usage
//
// To enable audit logging, create an AuditConfig and pass it to NewLogger:
//
//	config := &audit.AuditConfig{
//		Enabled:    true,
//		Level:      audit.LevelInfo,
//		OutputFile: "/var/log/mockd-audit.log",
//	}
//
//	logger, err := audit.NewLogger(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer logger.Close()
//
//	// Log an audit entry
//	entry := audit.NewAuditEntry(audit.EventRequestReceived, "trace-123")
//	entry.WithRequest(&audit.RequestInfo{
//		Method: "GET",
//		Path:   "/api/users",
//	})
//	logger.Log(*entry)
//
// # Output Formats
//
// Audit entries are written as JSON lines (NDJSON format), making them easy
// to parse with tools like jq, or to ingest into log aggregation systems.
//
// # Logger Types
//
//   - FileLogger: Writes to a file, suitable for persistent logging
//   - StdoutLogger: Writes to stdout, suitable for containerized deployments
//   - NoOpLogger: Discards all entries, used when audit logging is disabled
//
// # Event Types
//
// The package defines standard event types for common operations:
//
//   - request.received: An HTTP request was received
//   - response.sent: An HTTP response was sent
//   - mock.matched: A mock configuration matched the request
//   - mock.not_found: No mock configuration matched the request
//   - proxy.forwarded: Request was forwarded to a proxy target
//   - error: An error occurred during processing
//
// # Thread Safety
//
// All logger implementations are safe for concurrent use from multiple
// goroutines. The FileLogger uses a mutex to serialize writes.
//
// # Extension Points
//
// The audit package supports custom extensions via registration:
//   - RegisterWriter: Register custom audit log writers (SIEM, monitoring, etc.)
//   - RegisterRedactor: Register custom PII/sensitive data redaction logic
//
// See registry.go for the extension API.
//
// # Future Enhancements
//
// Planned features include:
//   - Log rotation
//   - Async batched writes for high-throughput scenarios
package audit
