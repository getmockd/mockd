# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

### How to Report

1. Email security@getmockd.com with details of the vulnerability
2. Include steps to reproduce, if possible
3. Include the version of mockd affected

### What to Expect

- **Acknowledgment**: Within 48 hours of your report
- **Initial Assessment**: Within 7 days
- **Resolution Timeline**: Depends on severity, typically 30-90 days
- **Disclosure**: Coordinated with reporter after fix is available

### Scope

The following are in scope:
- mockd core server (this repository)
- Official Docker images
- Official Helm charts

Out of scope:
- Third-party integrations
- Self-hosted instances with custom modifications

## Security Best Practices

When running mockd in production:

1. **Do not expose the Admin API publicly** - Use firewall rules or bind to localhost
2. **Use TLS** - Enable HTTPS for all external traffic
3. **Enable mTLS** - For high-security environments
4. **Review audit logs** - Monitor for suspicious activity
5. **Keep updated** - Apply security patches promptly
