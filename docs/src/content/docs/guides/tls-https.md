---
title: TLS/HTTPS Configuration
description: Configure HTTPS for mockd mock server and proxy modes, including certificate generation and common use cases.
---

mockd supports HTTPS for both the mock server and proxy modes. This guide covers certificate generation, configuration, and common use cases.

## Mock Server HTTPS

### Quick Start

Enable HTTPS with auto-generated self-signed certificates:

```bash
mockd start --config mocks.json --tls-auto --https-port 8443
```

mockd generates a self-signed certificate and starts HTTPS on port 8443.

### Custom Port

```bash
mockd start --config mocks.json --tls-auto --https-port 443
```

### With Your Own Certificates

```bash
mockd start --config mocks.json \
  --tls-cert ./certs/server.crt \
  --tls-key ./certs/server.key \
  --https-port 8443
```

### Configuration File

```json
{
  "server": {
    "port": 4280,
    "tls": {
      "enabled": true,
      "port": 8443,
      "certFile": "./certs/server.crt",
      "keyFile": "./certs/server.key"
    }
  },
  "mocks": [...]
}
```

This starts HTTP on port 4280 and HTTPS on port 8443. To run HTTPS only, omit `server.port` or set `httpsRedirect: true`.

## Certificate Generation

### Self-Signed (Development)

The simplest approach is to use the `--tls-auto` flag, which generates a self-signed certificate automatically:

```bash
mockd serve --config mocks.json --tls-auto --https-port 8443
```

Alternatively, generate certificates with OpenSSL:

```bash
# Generate private key
openssl genrsa -out ./certs/localhost.key 2048

# Generate self-signed certificate
openssl req -new -x509 -key ./certs/localhost.key \
  -out ./certs/localhost.crt -days 365 -subj "/CN=localhost"

# Start with your certificates
mockd serve --config mocks.json \
  --tls-cert ./certs/localhost.crt \
  --tls-key ./certs/localhost.key \
  --https-port 8443
```

With Subject Alternative Names:

```bash
openssl req -new -x509 -key ./certs/localhost.key \
  -out ./certs/localhost.crt -days 365 \
  -subj "/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,IP:::1,DNS:myapp.local"
```

### CA Certificate (For Proxy)

Generate a CA for MITM proxying with the built-in command:

```bash
mockd proxy ca generate --ca-path ./certs
```

This generates a CA certificate and key in the `./certs` directory for use with proxy HTTPS interception.

## Proxy HTTPS

### MITM Proxy Setup

For the proxy to intercept and record HTTPS traffic, clients must trust the mockd CA. Without a CA, HTTPS connections are tunneled (TCP pass-through) and cannot be recorded.

1. **Generate CA Certificate**:

```bash
mockd proxy ca generate --ca-path ./certs
```

2. **Start Proxy with HTTPS Interception**:

```bash
mockd proxy start --ca-path ./certs
```

The proxy dynamically generates per-host TLS certificates signed by your CA, decrypting traffic for recording.

3. **Install CA on Client**:

See [Installing CA Certificates](#installing-ca-certificates) below.

### Export CA Certificate

```bash
# Export to a file for distribution
mockd proxy ca export --ca-path ./certs -o mockd-ca.crt
```

## Installing CA Certificates

### macOS

```bash
# Add to system keychain
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain \
  ./certs/mockd-ca.crt

# Or for current user only
security add-trusted-cert -r trustRoot \
  -k ~/Library/Keychains/login.keychain \
  ./certs/mockd-ca.crt
```

### Linux

```bash
# Copy certificate
sudo cp ./certs/mockd-ca.crt /usr/local/share/ca-certificates/mockd-ca.crt

# Update certificate store
sudo update-ca-certificates
```

### Windows

```powershell
# Import to Trusted Root Certification Authorities
Import-Certificate -FilePath .\certs\mockd-ca.crt `
  -CertStoreLocation Cert:\LocalMachine\Root
```

### Node.js

```bash
export NODE_EXTRA_CA_CERTS=./certs/mockd-ca.crt
node app.js
```

### Python (requests)

```python
import requests
requests.get('https://localhost:8443', verify='./certs/mockd-ca.crt')
```

### curl

```bash
curl --cacert ./certs/mockd-ca.crt https://localhost:8443/api/users
```

### Docker

Mount the CA certificate:

```bash
docker run -v $(pwd)/certs/mockd-ca.crt:/etc/ssl/certs/mockd-ca.crt \
  myapp
```

## TLS Options

### Minimum TLS Version

```json
{
  "server": {
    "tls": {
      "enabled": true,
      "minVersion": "1.2"
    }
  }
}
```

Supported: `1.0`, `1.1`, `1.2`, `1.3`

### Cipher Suites

```json
{
  "server": {
    "tls": {
      "enabled": true,
      "cipherSuites": [
        "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
        "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
      ]
    }
  }
}
```

### Client Certificate Authentication (mTLS)

Require client certificates:

```json
{
  "server": {
    "tls": {
      "enabled": true,
      "certFile": "./certs/server.crt",
      "keyFile": "./certs/server.key",
      "clientAuth": "require",
      "clientCAs": ["./certs/client-ca.crt"]
    }
  }
}
```

Client auth modes:
- `none` - No client cert required
- `request` - Request but don't require
- `require` - Require valid client cert

## Mixed HTTP/HTTPS

Serve both protocols:

```json
{
  "server": {
    "port": 4280,
    "tls": {
      "enabled": true,
      "port": 8443,
      "certFile": "./certs/server.crt",
      "keyFile": "./certs/server.key"
    }
  }
}
```

Both endpoints serve the same mocks:
- `http://localhost:4280`
- `https://localhost:8443`

## HTTPS Redirect

Redirect HTTP to HTTPS:

```json
{
  "server": {
    "port": 4280,
    "httpsRedirect": true,
    "tls": {
      "enabled": true,
      "port": 8443
    }
  }
}
```

## Common Issues

### Certificate Not Trusted

**Symptom**: `CERT_AUTHORITY_INVALID` or similar errors

**Solution**: Install the CA certificate as described above, or use `--insecure` flags for testing:

```bash
curl -k https://localhost:8443/api/users
```

### Certificate Hostname Mismatch

**Symptom**: `HOSTNAME_MISMATCH` error

**Solution**: Generate certificate with correct SANs:

```bash
openssl req -new -x509 -key ./certs/localhost.key \
  -out ./certs/localhost.crt -days 365 \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:myapp.local,IP:127.0.0.1"
```

Or use `--tls-auto` which generates a cert for `localhost` automatically.

### Certificate Expired

**Symptom**: `CERT_HAS_EXPIRED` error

**Solution**: Regenerate with longer validity:

```bash
openssl req -new -x509 -key ./certs/localhost.key \
  -out ./certs/localhost.crt -days 3650 -subj "/CN=localhost"
```

### Permission Denied (Port 443)

**Symptom**: Cannot bind to port 443

**Solution**: Use a high port or grant capability:

```bash
# Use high port (recommended)
mockd start --tls-auto --https-port 8443

# Or grant capability (Linux) to bind port 443
sudo setcap 'cap_net_bind_service=+ep' $(which mockd)
```

## Security Considerations

1. **Never use self-signed certs in production** - They're for development only

2. **Protect private keys** - Restrict file permissions:
   ```bash
   chmod 600 ./certs/*.key
   ```

3. **Short-lived certificates** - Use shorter validity for development certs

4. **Don't commit certs** - Add to `.gitignore`:
   ```
   certs/
   *.crt
   *.key
   *.pem
   ```

## Next Steps

- [Proxy Recording](/guides/proxy-recording/) - HTTPS proxy setup
- [CLI Reference](/reference/cli/) - Certificate commands
- [Configuration Reference](/reference/configuration/) - Full TLS options
