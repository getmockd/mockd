# TLS/HTTPS Configuration

mockd supports HTTPS for both the mock server and proxy modes. This guide covers certificate generation, configuration, and common use cases.

## Mock Server HTTPS

### Quick Start

Enable HTTPS with auto-generated certificates:

```bash
mockd start --config mocks.json --https
```

mockd generates a self-signed certificate and starts on port 8443.

### Custom Port

```bash
mockd start --config mocks.json --https --port 443
```

### With Your Own Certificates

```bash
mockd start --config mocks.json \
  --cert ./certs/server.crt \
  --key ./certs/server.key
```

### Configuration File

```json
{
  "server": {
    "port": 8443,
    "tls": {
      "enabled": true,
      "certFile": "./certs/server.crt",
      "keyFile": "./certs/server.key"
    }
  },
  "mocks": [...]
}
```

## Certificate Generation

### Self-Signed (Development)

Generate a self-signed certificate:

```bash
mockd cert generate --name localhost --days 365
```

Output:
```
Generated certificate: ./certs/localhost.crt
Generated private key: ./certs/localhost.key
```

With Subject Alternative Names:

```bash
mockd cert generate \
  --name localhost \
  --san "127.0.0.1" \
  --san "::1" \
  --san "myapp.local"
```

### CA Certificate (For Proxy)

Generate a CA for MITM proxying:

```bash
mockd cert generate-ca --name "mockd CA" --days 3650
```

Output:
```
Generated CA certificate: ./certs/mockd-ca.crt
Generated CA private key: ./certs/mockd-ca.key
```

## Proxy HTTPS

### MITM Proxy Setup

For the proxy to intercept HTTPS traffic, clients must trust the mockd CA.

1. **Generate CA Certificate**:

```bash
mockd cert generate-ca
```

2. **Start Proxy**:

```bash
mockd proxy --target https://api.example.com \
  --ca-cert ./certs/mockd-ca.crt \
  --ca-key ./certs/mockd-ca.key
```

3. **Install CA on Client**:

See [Installing CA Certificates](#installing-ca-certificates) below.

### Proxy Configuration

```json
{
  "proxy": {
    "target": "https://api.example.com",
    "tls": {
      "caCertFile": "./certs/mockd-ca.crt",
      "caKeyFile": "./certs/mockd-ca.key",
      "certCacheDir": "./certs/generated"
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `caCertFile` | CA certificate for signing |
| `caKeyFile` | CA private key |
| `certCacheDir` | Cache for generated certs |

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
mockd cert generate --name localhost --san "myapp.local"
```

### Certificate Expired

**Symptom**: `CERT_HAS_EXPIRED` error

**Solution**: Regenerate with longer validity:

```bash
mockd cert generate --name localhost --days 3650
```

### Permission Denied (Port 443)

**Symptom**: Cannot bind to port 443

**Solution**: Use a high port or grant capability:

```bash
# Use high port
mockd start --https --port 8443

# Or grant capability (Linux)
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

- [Proxy Recording](proxy-recording.md) - HTTPS proxy setup
- [CLI Reference](../reference/cli.md) - Certificate commands
- [Configuration Reference](../reference/configuration.md) - Full TLS options
