---
title: SOAP/WSDL Mocking
description: Mock SOAP/XML web services with WSDL support, XPath matching, and fault handling
---

SOAP mocking enables you to create mock SOAP/XML web service endpoints for testing enterprise integrations and legacy systems. Configure WSDL-based services with operation mocking, XPath request matching, and SOAP fault handling.

## Overview

mockd's SOAP support includes:

- **WSDL support** - Define services inline or from external WSDL files
- **Operation mocking** - Return mock responses for SOAP operations
- **XPath matching** - Conditional responses based on request content
- **SOAP faults** - Return fault responses for error testing
- **Namespace handling** - Full XML namespace support
- **Template support** - Dynamic responses with variables
- **Automatic WSDL serving** - WSDL endpoint at `?wsdl` suffix

## Quick Start

Create a minimal SOAP mock:

```yaml
version: "1.0"

mocks:
  - id: my-soap-service
    name: User Service
    type: soap
    enabled: true
    soap:
      path: /soap/UserService
      wsdl: |
        <?xml version="1.0" encoding="UTF-8"?>
        <definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
                     xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
                     xmlns:tns="http://example.com/user"
                     targetNamespace="http://example.com/user">
          <message name="GetUserRequest">
            <part name="userId" type="xsd:string"/>
          </message>
          <message name="GetUserResponse">
            <part name="user" type="tns:User"/>
          </message>
          <portType name="UserPortType">
            <operation name="GetUser">
              <input message="tns:GetUserRequest"/>
              <output message="tns:GetUserResponse"/>
            </operation>
          </portType>
        </definitions>

      operations:
        GetUser:
          soapAction: "http://example.com/GetUser"
          response: |
            <GetUserResponse xmlns="http://example.com/user">
              <User>
                <Id>123</Id>
                <Name>John Doe</Name>
                <Email>john@example.com</Email>
              </User>
            </GetUserResponse>
```

Start the server and test:

```bash
# Start mockd
mockd serve --config mockd.yaml

# Call the SOAP service
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://example.com/GetUser" \
  -d '<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <UserId>123</UserId>
    </GetUser>
  </soap:Body>
</soap:Envelope>'

# Get the WSDL
curl http://localhost:4280/soap/UserService?wsdl
```

## Configuration

### Full Configuration Reference

```yaml
mocks:
  - id: soap-service
    name: My SOAP Service
    type: soap
    enabled: true
    soap:
      # Endpoint path (required)
      path: /soap/MyService

      # WSDL definition - use either inline or file
      wsdl: |
        <?xml version="1.0" encoding="UTF-8"?>
        <definitions xmlns="http://schemas.xmlsoap.org/wsdl/">
          ...
        </definitions>
      # OR
      wsdlFile: ./wsdl/service.wsdl

      # Operation configurations
      operations:
        OperationName:
          soapAction: "http://example.com/OperationName"
          response: |
            <OperationNameResponse>...</OperationNameResponse>
          delay: "100ms"
          match:
            xpath:
              "//ElementName/text()": "value"
          fault:
            code: soap:Client
            message: "Error message"
            detail: "<ErrorCode>...</ErrorCode>"
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | SOAP endpoint path (e.g., `/soap/UserService`) |
| `wsdl` | string | Inline WSDL definition |
| `wsdlFile` | string | Path to external WSDL file |
| `operations` | map | Operation configurations by operation name |

### Operation Fields

| Field | Type | Description |
|-------|------|-------------|
| `soapAction` | string | SOAPAction header value for matching |
| `response` | string | XML response body |
| `delay` | string | Response delay (e.g., `"100ms"`, `"2s"`) |
| `match` | object | XPath-based request matching |
| `fault` | object | SOAP fault response configuration |

## XPath Matching

Use XPath expressions to match specific request elements and return conditional responses.

### Basic XPath Match

```yaml
operations:
  GetUser:
    soapAction: "http://example.com/GetUser"
    match:
      xpath:
        "//UserId/text()": "123"
    response: |
      <GetUserResponse xmlns="http://example.com/user">
        <User>
          <Id>123</Id>
          <Name>John Doe</Name>
          <Email>john@example.com</Email>
        </User>
      </GetUserResponse>
```

### XPath Patterns

| Pattern | Description | Example |
|---------|-------------|---------|
| `//Element` | Select element anywhere | `"//UserId"` |
| `/Root/Child` | Absolute path | `"/Envelope/Body/Request"` |
| `//Element/text()` | Element text content | `"//UserId/text()"` |
| `//Element[@attr]` | Element with attribute | `"//User[@active]"` |
| `//Element[@attr='x']` | Specific attribute value | `"//User[@type='admin']"` |

## Fault Responses

Return SOAP fault responses for error testing.

### Basic Fault

```yaml
operations:
  GetUser:
    soapAction: "http://example.com/GetUser"
    match:
      xpath:
        "//UserId/text()": "invalid"
    fault:
      code: soap:Client
      message: "Invalid user ID format"
```

This generates:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Client</faultcode>
      <faultstring>Invalid user ID format</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>
```

### Common SOAP Fault Codes

| Code | Description | Use Case |
|------|-------------|----------|
| `soap:Client` | Client-side error | Invalid request, missing data |
| `soap:Server` | Server-side error | Internal errors, service unavailable |
| `soap:MustUnderstand` | Header processing error | Required header not understood |
| `soap:VersionMismatch` | SOAP version mismatch | Wrong SOAP version |

## Dynamic Responses with Templates

Use template expressions in responses:

```yaml
operations:
  CreateUser:
    soapAction: "http://example.com/CreateUser"
    response: |
      <CreateUserResponse xmlns="http://example.com/user">
        <User>
          <Id>{{uuid}}</Id>
          <Name>New User</Name>
          <CreatedAt>{{now}}</CreatedAt>
        </User>
      </CreateUserResponse>
```

Available templates:

| Template | Description |
|----------|-------------|
| `{{uuid}}` | Random UUID |
| `{{now}}` | Current ISO timestamp |
| `{{timestamp}}` | Unix timestamp |

## Testing

### Test with curl

Basic SOAP request:

```bash
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: text/xml; charset=utf-8" \
  -H "SOAPAction: http://example.com/GetUser" \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <UserId>123</UserId>
    </GetUser>
  </soap:Body>
</soap:Envelope>'
```

### Test with SOAP Clients

mockd works with standard SOAP tools and libraries:

- **SoapUI** - Import the WSDL from `http://localhost:4280/soap/UserService?wsdl`
- **Postman** - Use SOAP request type with WSDL import
- **Python zeep** - Point to the WSDL endpoint
- **.NET WCF** - Add service reference using the WSDL URL
- **Java JAX-WS** - Generate client stubs from WSDL

### Python zeep Example

```python
from zeep import Client

client = Client('http://localhost:4280/soap/UserService?wsdl')
result = client.service.GetUser(UserId='123')
print(result)
```

## Next Steps

- [Response Templating](/guides/response-templating/) - Dynamic response values
- [Request Matching](/guides/request-matching/) - HTTP-level matching
- [TLS/HTTPS](/guides/tls-https/) - Secure SOAP endpoints
