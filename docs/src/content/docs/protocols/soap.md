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

## WSDL Configuration

### Inline WSDL

Define the WSDL directly in your config:

```yaml
soap:
  path: /soap/OrderService
  wsdl: |
    <?xml version="1.0" encoding="UTF-8"?>
    <definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
                 xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
                 xmlns:xsd="http://www.w3.org/2001/XMLSchema"
                 xmlns:tns="http://example.com/orders"
                 targetNamespace="http://example.com/orders"
                 name="OrderService">
      <types>
        <xsd:schema targetNamespace="http://example.com/orders">
          <xsd:element name="GetOrder">
            <xsd:complexType>
              <xsd:sequence>
                <xsd:element name="orderId" type="xsd:string"/>
              </xsd:sequence>
            </xsd:complexType>
          </xsd:element>
        </xsd:schema>
      </types>
      <message name="GetOrderInput">
        <part name="parameters" element="tns:GetOrder"/>
      </message>
      <message name="GetOrderOutput">
        <part name="parameters" element="tns:GetOrderResponse"/>
      </message>
      <portType name="OrderPortType">
        <operation name="GetOrder">
          <input message="tns:GetOrderInput"/>
          <output message="tns:GetOrderOutput"/>
        </operation>
      </portType>
      <binding name="OrderBinding" type="tns:OrderPortType">
        <soap:binding style="document"
                      transport="http://schemas.xmlsoap.org/soap/http"/>
        <operation name="GetOrder">
          <soap:operation soapAction="http://example.com/GetOrder"/>
        </operation>
      </binding>
      <service name="OrderService">
        <port name="OrderPort" binding="tns:OrderBinding">
          <soap:address location="http://localhost:4280/soap/OrderService"/>
        </port>
      </service>
    </definitions>
```

### External WSDL File

Reference an external WSDL file:

```yaml
soap:
  path: /soap/OrderService
  wsdlFile: ./wsdl/orders.wsdl
```

The WSDL is served at `http://localhost:4280/soap/OrderService?wsdl` regardless of whether it's inline or from a file.

## Operations

Operations define how each SOAP action is handled. Each operation maps a SOAPAction header to a response.

### Basic Operation

```yaml
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

### Multiple Operations

```yaml
operations:
  GetUser:
    soapAction: "http://example.com/GetUser"
    response: |
      <GetUserResponse xmlns="http://example.com/user">
        <User>
          <Id>123</Id>
          <Name>John Doe</Name>
        </User>
      </GetUserResponse>

  CreateUser:
    soapAction: "http://example.com/CreateUser"
    response: |
      <CreateUserResponse xmlns="http://example.com/user">
        <UserId>{{uuid}}</UserId>
        <Status>Created</Status>
        <CreatedAt>{{now}}</CreatedAt>
      </CreateUserResponse>

  DeleteUser:
    soapAction: "http://example.com/DeleteUser"
    response: |
      <DeleteUserResponse xmlns="http://example.com/user">
        <Success>true</Success>
      </DeleteUserResponse>

  ListUsers:
    soapAction: "http://example.com/ListUsers"
    response: |
      <ListUsersResponse xmlns="http://example.com/user">
        <Users>
          <User><Id>1</Id><Name>Alice</Name></User>
          <User><Id>2</Id><Name>Bob</Name></User>
          <User><Id>3</Id><Name>Carol</Name></User>
        </Users>
      </ListUsersResponse>
```

### Response Delay

Simulate slow backend services:

```yaml
operations:
  GetReport:
    soapAction: "http://example.com/GetReport"
    delay: "2s"
    response: |
      <GetReportResponse xmlns="http://example.com/reports">
        <Report>
          <Id>report_001</Id>
          <Status>Complete</Status>
        </Report>
      </GetReportResponse>
```

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

### Multiple XPath Conditions

Match on multiple elements simultaneously:

```yaml
operations:
  SearchUsers:
    soapAction: "http://example.com/SearchUsers"
    match:
      xpath:
        "//Department/text()": "Engineering"
        "//Status/text()": "active"
    response: |
      <SearchUsersResponse xmlns="http://example.com/user">
        <Users>
          <User><Name>Alice</Name><Department>Engineering</Department></User>
          <User><Name>Bob</Name><Department>Engineering</Department></User>
        </Users>
      </SearchUsersResponse>
```

### Conditional Responses with Multiple Mocks

Return different responses for different XPath matches by creating multiple mocks:

```yaml
mocks:
  # Match user 123
  - id: soap-user-123
    type: soap
    enabled: true
    soap:
      path: /soap/UserService
      operations:
        GetUser:
          soapAction: "http://example.com/GetUser"
          match:
            xpath:
              "//UserId/text()": "123"
          response: |
            <GetUserResponse xmlns="http://example.com/user">
              <User><Id>123</Id><Name>John Doe</Name></User>
            </GetUserResponse>

  # Match user 456
  - id: soap-user-456
    type: soap
    enabled: true
    soap:
      path: /soap/UserService
      operations:
        GetUser:
          soapAction: "http://example.com/GetUser"
          match:
            xpath:
              "//UserId/text()": "456"
          response: |
            <GetUserResponse xmlns="http://example.com/user">
              <User><Id>456</Id><Name>Jane Smith</Name></User>
            </GetUserResponse>

  # Not found — no XPath match, acts as fallback
  - id: soap-user-not-found
    type: soap
    enabled: true
    soap:
      path: /soap/UserService
      operations:
        GetUser:
          soapAction: "http://example.com/GetUser"
          fault:
            code: soap:Client
            message: "User not found"
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

### Fault with Detail

Include structured error details:

```yaml
operations:
  TransferFunds:
    soapAction: "http://example.com/TransferFunds"
    match:
      xpath:
        "//Amount/text()": "0"
    fault:
      code: soap:Client
      message: "Invalid transfer amount"
      detail: |
        <TransferError xmlns="http://example.com/errors">
          <ErrorCode>INVALID_AMOUNT</ErrorCode>
          <MinAmount>0.01</MinAmount>
          <MaxAmount>1000000.00</MaxAmount>
        </TransferError>
```

### Server-Side Fault

Simulate backend failures:

```yaml
operations:
  ProcessPayment:
    soapAction: "http://example.com/ProcessPayment"
    fault:
      code: soap:Server
      message: "Payment gateway unavailable"
      detail: |
        <ServiceError xmlns="http://example.com/errors">
          <RetryAfter>30</RetryAfter>
        </ServiceError>
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

## Examples

### Payment Processing Service

```yaml
version: "1.0"

mocks:
  - id: payment-soap
    name: Payment Service
    type: soap
    enabled: true
    soap:
      path: /soap/PaymentService
      operations:
        ProcessPayment:
          soapAction: "http://example.com/ProcessPayment"
          response: |
            <ProcessPaymentResponse xmlns="http://example.com/payment">
              <TransactionId>{{uuid}}</TransactionId>
              <Status>APPROVED</Status>
              <Amount>99.99</Amount>
              <Currency>USD</Currency>
              <Timestamp>{{now}}</Timestamp>
            </ProcessPaymentResponse>

        RefundPayment:
          soapAction: "http://example.com/RefundPayment"
          delay: "500ms"
          response: |
            <RefundPaymentResponse xmlns="http://example.com/payment">
              <RefundId>{{uuid}}</RefundId>
              <Status>PROCESSED</Status>
              <Timestamp>{{now}}</Timestamp>
            </RefundPaymentResponse>

        GetTransaction:
          soapAction: "http://example.com/GetTransaction"
          response: |
            <GetTransactionResponse xmlns="http://example.com/payment">
              <Transaction>
                <Id>txn_001</Id>
                <Amount>99.99</Amount>
                <Currency>USD</Currency>
                <Status>COMPLETED</Status>
                <CardLast4>4242</CardLast4>
                <CreatedAt>2024-01-15T10:00:00Z</CreatedAt>
              </Transaction>
            </GetTransactionResponse>
```

### Weather Service with XPath Matching

```yaml
version: "1.0"

mocks:
  - id: weather-soap-nyc
    name: Weather Service - NYC
    type: soap
    enabled: true
    soap:
      path: /soap/WeatherService
      operations:
        GetWeather:
          soapAction: "http://example.com/GetWeather"
          match:
            xpath:
              "//City/text()": "New York"
          response: |
            <GetWeatherResponse xmlns="http://example.com/weather">
              <Weather>
                <City>New York</City>
                <Temperature>72</Temperature>
                <Unit>Fahrenheit</Unit>
                <Condition>Partly Cloudy</Condition>
                <Humidity>65</Humidity>
              </Weather>
            </GetWeatherResponse>

  - id: weather-soap-london
    name: Weather Service - London
    type: soap
    enabled: true
    soap:
      path: /soap/WeatherService
      operations:
        GetWeather:
          soapAction: "http://example.com/GetWeather"
          match:
            xpath:
              "//City/text()": "London"
          response: |
            <GetWeatherResponse xmlns="http://example.com/weather">
              <Weather>
                <City>London</City>
                <Temperature>18</Temperature>
                <Unit>Celsius</Unit>
                <Condition>Rainy</Condition>
                <Humidity>80</Humidity>
              </Weather>
            </GetWeatherResponse>

  - id: weather-soap-default
    name: Weather Service - Default
    type: soap
    enabled: true
    soap:
      path: /soap/WeatherService
      operations:
        GetWeather:
          soapAction: "http://example.com/GetWeather"
          fault:
            code: soap:Client
            message: "City not found. Supported cities: New York, London"
```

### Enterprise Integration with Multiple Services

```yaml
version: "1.0"

mocks:
  - id: crm-soap
    name: CRM Service
    type: soap
    enabled: true
    soap:
      path: /soap/CRMService
      operations:
        GetCustomer:
          soapAction: "urn:crm:GetCustomer"
          response: |
            <GetCustomerResponse xmlns="urn:crm">
              <Customer>
                <Id>CUST-001</Id>
                <Name>Acme Corp</Name>
                <Type>Enterprise</Type>
                <Status>Active</Status>
                <AccountManager>John Smith</AccountManager>
              </Customer>
            </GetCustomerResponse>

        CreateLead:
          soapAction: "urn:crm:CreateLead"
          response: |
            <CreateLeadResponse xmlns="urn:crm">
              <LeadId>{{uuid}}</LeadId>
              <Status>New</Status>
              <CreatedAt>{{now}}</CreatedAt>
            </CreateLeadResponse>

  - id: inventory-soap
    name: Inventory Service
    type: soap
    enabled: true
    soap:
      path: /soap/InventoryService
      operations:
        CheckStock:
          soapAction: "urn:inventory:CheckStock"
          response: |
            <CheckStockResponse xmlns="urn:inventory">
              <Item>
                <SKU>WIDGET-001</SKU>
                <InStock>true</InStock>
                <Quantity>250</Quantity>
                <Warehouse>US-EAST-1</Warehouse>
              </Item>
            </CheckStockResponse>

        ReserveStock:
          soapAction: "urn:inventory:ReserveStock"
          delay: "200ms"
          response: |
            <ReserveStockResponse xmlns="urn:inventory">
              <ReservationId>{{uuid}}</ReservationId>
              <Status>Reserved</Status>
              <ExpiresAt>{{now}}</ExpiresAt>
            </ReserveStockResponse>
```

## CLI Commands

### Add a SOAP Mock

Create SOAP mocks directly from the command line using `mockd soap add`:

```bash
# Simple operation
mockd soap add --path /soap/weather --action GetWeather \
  --response '<Temp>72</Temp>'

# With a specific SOAPAction
mockd soap add --path /soap/users --action GetUser \
  --response '<User><Id>123</Id><Name>John</Name></User>'
```

Output:

```
Created mock: soap_4b349e0c7719f577
  Type: soap
  Path: /soap/weather
  Operation: GetWeather
```

#### Add Command Flags

| Flag | Description |
|------|-------------|
| `--path` | SOAP endpoint path (required) |
| `--action` | SOAP operation/action name (required) |
| `--response` | XML response body |
| `--stateful-resource` | Stateful resource name (e.g., `users`) |
| `--stateful-action` | Stateful action: `list`, `get`, `create`, `update`, `delete`, `custom` |
| `--admin-url` | Admin API URL (default: `http://localhost:4290`) |

:::note
The CLI flag is `--action`, not `--operation`. This matches the SOAPAction header terminology.
:::

#### Stateful SOAP via CLI

Wire a SOAP operation directly to a stateful resource from the command line:

```bash
# First, create a stateful resource (if one doesn't exist)
mockd stateful add users --path /api/users

# Wire SOAP operations to the resource
mockd soap add --path /soap --action ListUsers --stateful-resource users --stateful-action list
mockd soap add --path /soap --action GetUser --stateful-resource users --stateful-action get
mockd soap add --path /soap --action CreateUser --stateful-resource users --stateful-action create
```

`--stateful-resource` and `--stateful-action` must be used together. When set, the SOAP operation reads/writes from the named stateful resource instead of returning a canned response.

For complex SOAP mocks with WSDL definitions, XPath matching, or multiple operations, use a YAML config file instead of the CLI.

### List SOAP Mocks

```bash
# List all mocks (includes SOAP)
mockd list

# Filter to SOAP mocks
mockd list --type soap

# JSON output
mockd list --type soap --json
```

### Delete a SOAP Mock

```bash
mockd delete soap_4b349e0c7719f577
```

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

### Verify WSDL Endpoint

```bash
# Fetch WSDL
curl http://localhost:4280/soap/UserService?wsdl

# Verify WSDL returns XML
curl -sI http://localhost:4280/soap/UserService?wsdl | grep Content-Type
# Content-Type: text/xml; charset=utf-8
```

### Test Fault Responses

Trigger a SOAP fault by sending a request that matches fault conditions:

```bash
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://example.com/GetUser" \
  -d '<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <UserId>invalid</UserId>
    </GetUser>
  </soap:Body>
</soap:Envelope>'

# Returns SOAP Fault with HTTP 500
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

### Java Example

```java
import javax.xml.ws.Service;
import java.net.URL;
import javax.xml.namespace.QName;

URL wsdlUrl = new URL("http://localhost:4280/soap/UserService?wsdl");
QName serviceName = new QName("http://example.com/user", "UserService");
Service service = Service.create(wsdlUrl, serviceName);
UserPortType port = service.getPort(UserPortType.class);
GetUserResponse response = port.getUser("123");
```

### Integration Tests (Go)

```go
package main

import (
    "bytes"
    "io"
    "net/http"
    "strings"
    "testing"
)

func TestSOAPMock(t *testing.T) {
    soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <UserId>123</UserId>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

    req, _ := http.NewRequest("POST",
        "http://localhost:4280/soap/UserService",
        bytes.NewBufferString(soapRequest))
    req.Header.Set("Content-Type", "text/xml; charset=utf-8")
    req.Header.Set("SOAPAction", "http://example.com/GetUser")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }

    body, _ := io.ReadAll(resp.Body)
    if !strings.Contains(string(body), "John Doe") {
        t.Error("Response does not contain expected user name")
    }
}
```

## Testing Tips

### Verify SOAPAction Header Matching

SOAP mocks match on the `SOAPAction` header. Ensure your client sends it:

```bash
# This will match
curl -X POST http://localhost:4280/soap/UserService \
  -H "SOAPAction: http://example.com/GetUser" \
  -H "Content-Type: text/xml" \
  -d @request.xml

# This will NOT match (missing SOAPAction)
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: text/xml" \
  -d @request.xml
```

### Test Error Handling

Create mocks that return SOAP faults to verify your client's error handling:

```yaml
operations:
  # Timeout simulation
  SlowOperation:
    soapAction: "http://example.com/SlowOp"
    delay: "30s"
    response: |
      <SlowOpResponse><Status>done</Status></SlowOpResponse>

  # Server error
  FailingOperation:
    soapAction: "http://example.com/FailOp"
    fault:
      code: soap:Server
      message: "Internal service error"
```

### Debug with Request Logs

Use mockd's request log to see incoming SOAP requests:

```bash
# View recent requests
curl http://localhost:4290/logs?limit=5

# Or via CLI
mockd logs --limit 5
```

### Use with CI/CD

Start mockd in the background for integration tests:

```bash
# Start in background
mockd start -d --config soap-mocks.yaml

# Run your SOAP client tests
./run-soap-tests.sh

# Stop when done
mockd stop
```

## Stateful SOAP Operations

SOAP operations can be wired to stateful CRUD resources, enabling shared state between REST and SOAP protocols. A REST `POST /api/users` creates a user that a SOAP `GetUser` can retrieve — and vice versa.

### Configuration

Add `statefulResource` and `statefulAction` to any SOAP operation:

```yaml
version: "1.0"

statefulResources:
  - name: users
    basePath: /api/users
    seedData:
      - { id: "1", name: "Alice", email: "alice@example.com" }

mocks:
  - type: soap
    name: User SOAP Service
    soap:
      path: /soap/UserService
      operations:
        GetUser:
          soapAction: "http://example.com/GetUser"
          statefulResource: users
          statefulAction: get

        ListUsers:
          soapAction: "http://example.com/ListUsers"
          statefulResource: users
          statefulAction: list

        CreateUser:
          soapAction: "http://example.com/CreateUser"
          statefulResource: users
          statefulAction: create

        UpdateUser:
          soapAction: "http://example.com/UpdateUser"
          statefulResource: users
          statefulAction: update

        DeleteUser:
          soapAction: "http://example.com/DeleteUser"
          statefulResource: users
          statefulAction: delete
```

### Supported Actions

| Action | Description | Request Data | Response |
|--------|-------------|--------------|----------|
| `get` | Retrieve single item | ID extracted from XML | Item as XML |
| `list` | List all items | Optional filters | Items wrapped in XML |
| `create` | Create new item | Fields from XML body | Created item as XML |
| `update` | Replace item (PUT) | ID + fields from XML | Updated item as XML |
| `patch` | Partial update | ID + partial fields | Updated item as XML |
| `delete` | Remove item | ID extracted from XML | Empty response |
| `custom` | Multi-step operation | Defined by steps | Expression-built response |

### How It Works

1. SOAP request arrives and is routed to the matching operation
2. XML body is parsed and converted to a `map[string]interface{}`
3. The stateful Bridge executes the CRUD action against the shared store
4. The result is converted back to XML and wrapped in a SOAP envelope
5. Errors map to SOAP faults (e.g., not-found → `soap:Client`)

### Cross-Protocol State Sharing

The key insight: stateful resources are **protocol-agnostic**. The same in-memory store backs both HTTP REST and SOAP:

```bash
# Create via REST
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'
# → {"id": "abc-123", "name": "Alice", ...}

# Retrieve the same user via SOAP
curl -X POST http://localhost:4280/soap/UserService \
  -H "SOAPAction: http://example.com/GetUser" \
  -H "Content-Type: text/xml" \
  -d '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
        <soap:Body><GetUser><Id>abc-123</Id></GetUser></soap:Body>
      </soap:Envelope>'
# → SOAP envelope with user data
```

## WSDL Import

Generate SOAP mock configurations from existing WSDL files.

### Using the SOAP Import Command

```bash
# Basic import — generates static response mocks
mockd soap import service.wsdl

# Stateful import — detects CRUD operations and wires to stateful resources
mockd soap import service.wsdl --stateful

# Output to a specific file
mockd soap import service.wsdl -o mocks.yaml

# Output as JSON
mockd soap import service.wsdl --format json
```

### Using the General Import Command

WSDL files are auto-detected by the general import command:

```bash
mockd import service.wsdl
```

### Stateful Heuristics

With the `--stateful` flag, the importer detects CRUD patterns in operation names:

| Pattern | Detected Action |
|---------|----------------|
| `GetUser`, `FindOrder` | `get` |
| `ListUsers`, `SearchOrders` | `list` |
| `CreateUser`, `AddOrder` | `create` |
| `UpdateUser`, `ModifyOrder` | `update` |
| `DeleteUser`, `RemoveOrder` | `delete` |

The importer generates both the `statefulResources` definitions and the SOAP operations with `statefulResource`/`statefulAction` fields pre-filled.

## Custom Operations

Custom operations compose multiple reads, writes, and expression-evaluated transforms against stateful resources. This enables complex mock scenarios like fund transfers.

### Configuration

```yaml
customOperations:
  - name: TransferFunds
    consistency: atomic
    steps:
      - type: read
        resource: accounts
        id: "input.sourceId"
        as: source
      - type: read
        resource: accounts
        id: "input.destId"
        as: dest
      - type: update
        resource: accounts
        id: "input.sourceId"
        set:
          balance: "source.balance - input.amount"
      - type: update
        resource: accounts
        id: "input.destId"
        set:
          balance: "dest.balance + input.amount"
    response:
      status: '"completed"'
      newSourceBalance: "source.balance - input.amount"
      newDestBalance: "dest.balance + input.amount"
```

### Referencing from SOAP

```yaml
operations:
  TransferFunds:
    soapAction: "http://example.com/TransferFunds"
    statefulResource: TransferFunds   # Name of the custom operation
    statefulAction: custom
```

### Managing Custom Operations via CLI

Custom operations can also be managed and executed directly from the CLI — useful for testing, scripting, and AI agent workflows:

```bash
# Validate before registering (optional but recommended)
mockd stateful custom validate --file transfer.yaml --check-resources
mockd stateful custom validate --file transfer.yaml \
  --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}' \
  --check-expressions-runtime \
  --fixtures-file transfer-fixtures.json

# Register a custom operation from a YAML file
mockd stateful custom add --file transfer.yaml

# Or inline as JSON
mockd stateful custom add --definition '{"name":"TransferFunds","steps":[...]}'

# List registered operations
mockd stateful custom list

# Execute directly (no HTTP/SOAP request needed)
mockd stateful custom run TransferFunds --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'

# Wire to an HTTP endpoint too
mockd http add -m POST --path /api/transfer --stateful-operation TransferFunds
```

See [`mockd stateful custom`](/reference/cli/#mockd-stateful-custom) in the CLI reference for the full command set.

### Step Types

| Step | Description |
|------|-------------|
| `read` | Read an item from a resource, store in a named variable |
| `create` | Create a new item in a resource |
| `update` | Update an item using expression-evaluated fields |
| `delete` | Delete an item from a resource |
| `set` | Set a context variable to an expression result |

### Expression Language

Custom operations use [expr-lang/expr](https://github.com/expr-lang/expr) for expressions. The environment includes:

- `input` — the request data (parsed from SOAP XML, GraphQL variables, etc.)
- Named variables from prior `read` steps (e.g., `source.balance`)
- All Go arithmetic, comparison, and string operators

## Next Steps

- [Stateful Mocking](/guides/stateful-mocking/) - Complete stateful resource guide
- [Response Templating](/guides/response-templating/) - Dynamic response values
- [Import/Export](/guides/import-export/) - Import existing SOAP mocks
- [Chaos Engineering](/guides/chaos-engineering/) - Simulate failures
- [Configuration Reference](/reference/configuration/) - Full configuration schema
