// Package util provides shared helpers for safe file-path validation and
// log-body truncation used across mockd packages.
//
//   - SafeFilePath / SafeFilePathAllowAbsolute — reject path-traversal attempts
//   - TruncateBody — cap request/response bodies for safe logging
package util
