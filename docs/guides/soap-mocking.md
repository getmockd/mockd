# SOAP/WSDL Mocking

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

## WSDL Support

Define your WSDL either inline or in an external file.

### External WSDL File

For complex services, use an external WSDL file:

```yaml
soap:
  path: /soap/UserService
  wsdlFile: ./wsdl/user-service.wsdl
```

Create `wsdl/user-service.wsdl`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
             xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
             xmlns:tns="http://example.com/user"
             xmlns:xsd="http://www.w3.org/2001/XMLSchema"
             targetNamespace="http://example.com/user"
             name="UserService">

  <types>
    <xsd:schema targetNamespace="http://example.com/user">
      <xsd:complexType name="User">
        <xsd:sequence>
          <xsd:element name="Id" type="xsd:string"/>
          <xsd:element name="Name" type="xsd:string"/>
          <xsd:element name="Email" type="xsd:string"/>
        </xsd:sequence>
      </xsd:complexType>
    </xsd:schema>
  </types>

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

  <binding name="UserBinding" type="tns:UserPortType">
    <soap:binding style="document"
                  transport="http://schemas.xmlsoap.org/soap/http"/>
    <operation name="GetUser">
      <soap:operation soapAction="http://example.com/GetUser"/>
      <input><soap:body use="literal"/></input>
      <output><soap:body use="literal"/></output>
    </operation>
  </binding>

  <service name="UserService">
    <port name="UserPort" binding="tns:UserBinding">
      <soap:address location="http://localhost:4280/soap/UserService"/>
    </port>
  </service>
</definitions>
```

### Inline WSDL

For simple services, define WSDL inline:

```yaml
soap:
  path: /soap/Calculator
  wsdl: |
    <?xml version="1.0" encoding="UTF-8"?>
    <definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
                 xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
                 xmlns:tns="http://example.com/calculator"
                 targetNamespace="http://example.com/calculator">
      <message name="AddRequest">
        <part name="a" type="xsd:int"/>
        <part name="b" type="xsd:int"/>
      </message>
      <message name="AddResponse">
        <part name="result" type="xsd:int"/>
      </message>
      <portType name="CalculatorPortType">
        <operation name="Add">
          <input message="tns:AddRequest"/>
          <output message="tns:AddResponse"/>
        </operation>
      </portType>
    </definitions>
```

### Automatic WSDL Endpoint

The WSDL is automatically served at the endpoint path with `?wsdl` suffix:

```bash
# Get WSDL
curl http://localhost:4280/soap/UserService?wsdl
```

## Operations

Operations define how SOAP requests are matched and what responses to return.

### Basic Operation

Return a static response for an operation:

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

### Operation with Delay

Simulate network latency:

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
    delay: 500ms

  SlowOperation:
    soapAction: "http://example.com/SlowOperation"
    response: |
      <SlowOperationResponse>
        <Status>Complete</Status>
      </SlowOperationResponse>
    delay: 3s
```

### Dynamic Responses with Templates

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

  GetTimestamp:
    soapAction: "http://example.com/GetTimestamp"
    response: |
      <GetTimestampResponse xmlns="http://example.com/service">
        <Timestamp>{{timestamp}}</Timestamp>
        <ISO>{{now}}</ISO>
      </GetTimestampResponse>
```

Available templates:

| Template | Description |
|----------|-------------|
| `{{uuid}}` | Random UUID |
| `{{now}}` | Current ISO timestamp |
| `{{timestamp}}` | Unix timestamp |

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

Match multiple elements:

```yaml
operations:
  GetUser:
    soapAction: "http://example.com/GetUser"
    match:
      xpath:
        "//UserId/text()": "123"
        "//Active": "true"
    response: |
      <GetUserResponse xmlns="http://example.com/user">
        <User>
          <Id>123</Id>
          <Name>Active User</Name>
          <Status>Active</Status>
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

### Multiple Operations with Different Matches

Define the same operation multiple times with different matches:

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
        </User>
      </GetUserResponse>

  GetUser:
    soapAction: "http://example.com/GetUser"
    match:
      xpath:
        "//UserId/text()": "456"
    response: |
      <GetUserResponse xmlns="http://example.com/user">
        <User>
          <Id>456</Id>
          <Name>Jane Smith</Name>
        </User>
      </GetUserResponse>

  GetUser:
    soapAction: "http://example.com/GetUser"
    match:
      xpath:
        "//UserId/text()": "999"
    fault:
      code: soap:Client
      message: "User not found"
```

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

Include detailed error information:

```yaml
operations:
  GetUser:
    soapAction: "http://example.com/GetUser"
    match:
      xpath:
        "//UserId/text()": "999"
    fault:
      code: soap:Client
      message: "User not found"
      detail: |
        <ErrorInfo xmlns="http://example.com/errors">
          <Code>USER_NOT_FOUND</Code>
          <UserId>999</UserId>
          <Timestamp>{{now}}</Timestamp>
        </ErrorInfo>
```

This generates:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Client</faultcode>
      <faultstring>User not found</faultstring>
      <detail>
        <ErrorInfo xmlns="http://example.com/errors">
          <Code>USER_NOT_FOUND</Code>
          <UserId>999</UserId>
          <Timestamp>2024-01-15T10:00:00Z</Timestamp>
        </ErrorInfo>
      </detail>
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

### Server-side Fault Example

```yaml
operations:
  ProcessOrder:
    soapAction: "http://example.com/ProcessOrder"
    match:
      xpath:
        "//ProductId/text()": "OUT_OF_STOCK"
    fault:
      code: soap:Server
      message: "Service temporarily unavailable"
      detail: |
        <ServiceError xmlns="http://example.com/errors">
          <Code>SERVICE_UNAVAILABLE</Code>
          <RetryAfter>300</RetryAfter>
        </ServiceError>
```

## Examples

### Calculator Service

A simple calculator SOAP service:

```yaml
mocks:
  - id: calculator-soap
    name: Calculator Service
    type: soap
    enabled: true
    soap:
      path: /soap/Calculator
      wsdl: |
        <?xml version="1.0" encoding="UTF-8"?>
        <definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
                     xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
                     xmlns:tns="http://example.com/calculator"
                     xmlns:xsd="http://www.w3.org/2001/XMLSchema"
                     targetNamespace="http://example.com/calculator"
                     name="CalculatorService">

          <message name="AddRequest">
            <part name="a" type="xsd:int"/>
            <part name="b" type="xsd:int"/>
          </message>
          <message name="AddResponse">
            <part name="result" type="xsd:int"/>
          </message>
          <message name="SubtractRequest">
            <part name="a" type="xsd:int"/>
            <part name="b" type="xsd:int"/>
          </message>
          <message name="SubtractResponse">
            <part name="result" type="xsd:int"/>
          </message>

          <portType name="CalculatorPortType">
            <operation name="Add">
              <input message="tns:AddRequest"/>
              <output message="tns:AddResponse"/>
            </operation>
            <operation name="Subtract">
              <input message="tns:SubtractRequest"/>
              <output message="tns:SubtractResponse"/>
            </operation>
          </portType>

          <binding name="CalculatorBinding" type="tns:CalculatorPortType">
            <soap:binding style="document"
                          transport="http://schemas.xmlsoap.org/soap/http"/>
            <operation name="Add">
              <soap:operation soapAction="http://example.com/calculator/Add"/>
            </operation>
            <operation name="Subtract">
              <soap:operation soapAction="http://example.com/calculator/Subtract"/>
            </operation>
          </binding>

          <service name="CalculatorService">
            <port name="CalculatorPort" binding="tns:CalculatorBinding">
              <soap:address location="http://localhost:4280/soap/Calculator"/>
            </port>
          </service>
        </definitions>

      operations:
        Add:
          soapAction: "http://example.com/calculator/Add"
          response: |
            <AddResponse xmlns="http://example.com/calculator">
              <Result>15</Result>
            </AddResponse>

        Subtract:
          soapAction: "http://example.com/calculator/Subtract"
          response: |
            <SubtractResponse xmlns="http://example.com/calculator">
              <Result>5</Result>
            </SubtractResponse>
```

Test the calculator:

```bash
# Add operation
curl -X POST http://localhost:4280/soap/Calculator \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://example.com/calculator/Add" \
  -d '<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <Add xmlns="http://example.com/calculator">
      <A>10</A>
      <B>5</B>
    </Add>
  </soap:Body>
</soap:Envelope>'
```

### Enterprise User Management API

A complete enterprise user management SOAP service:

```yaml
mocks:
  - id: enterprise-user-api
    name: Enterprise User Management
    type: soap
    enabled: true
    soap:
      path: /soap/UserManagement
      wsdlFile: ./wsdl/user-management.wsdl

      operations:
        # Get user by ID
        GetUser:
          soapAction: "http://enterprise.example.com/user/GetUser"
          match:
            xpath:
              "//UserId/text()": "emp001"
          response: |
            <GetUserResponse xmlns="http://enterprise.example.com/user">
              <User>
                <UserId>emp001</UserId>
                <FirstName>John</FirstName>
                <LastName>Doe</LastName>
                <Email>john.doe@enterprise.com</Email>
                <Department>Engineering</Department>
                <Role>Senior Developer</Role>
                <Status>Active</Status>
                <CreatedAt>2020-01-15T09:00:00Z</CreatedAt>
              </User>
            </GetUserResponse>
          delay: 50ms

        # Get user - not found case
        GetUser:
          soapAction: "http://enterprise.example.com/user/GetUser"
          match:
            xpath:
              "//UserId/text()": "unknown"
          fault:
            code: soap:Client
            message: "User not found"
            detail: |
              <UserError xmlns="http://enterprise.example.com/errors">
                <Code>USER_NOT_FOUND</Code>
                <Message>No user exists with the specified ID</Message>
              </UserError>

        # Create new user
        CreateUser:
          soapAction: "http://enterprise.example.com/user/CreateUser"
          response: |
            <CreateUserResponse xmlns="http://enterprise.example.com/user">
              <User>
                <UserId>{{uuid}}</UserId>
                <FirstName>New</FirstName>
                <LastName>User</LastName>
                <Status>Pending</Status>
                <CreatedAt>{{now}}</CreatedAt>
              </User>
              <Message>User created successfully. Awaiting approval.</Message>
            </CreateUserResponse>
          delay: 100ms

        # Update user
        UpdateUser:
          soapAction: "http://enterprise.example.com/user/UpdateUser"
          response: |
            <UpdateUserResponse xmlns="http://enterprise.example.com/user">
              <Success>true</Success>
              <UpdatedAt>{{now}}</UpdatedAt>
            </UpdateUserResponse>

        # Delete user - protected user case
        DeleteUser:
          soapAction: "http://enterprise.example.com/user/DeleteUser"
          match:
            xpath:
              "//UserId/text()": "admin"
          fault:
            code: soap:Client
            message: "Cannot delete protected user"
            detail: |
              <UserError xmlns="http://enterprise.example.com/errors">
                <Code>PROTECTED_USER</Code>
                <Message>System administrator accounts cannot be deleted</Message>
              </UserError>

        # Delete user - success case
        DeleteUser:
          soapAction: "http://enterprise.example.com/user/DeleteUser"
          response: |
            <DeleteUserResponse xmlns="http://enterprise.example.com/user">
              <Success>true</Success>
              <DeletedAt>{{now}}</DeletedAt>
            </DeleteUserResponse>

        # List users
        ListUsers:
          soapAction: "http://enterprise.example.com/user/ListUsers"
          response: |
            <ListUsersResponse xmlns="http://enterprise.example.com/user">
              <Users>
                <User>
                  <UserId>emp001</UserId>
                  <FirstName>John</FirstName>
                  <LastName>Doe</LastName>
                  <Department>Engineering</Department>
                </User>
                <User>
                  <UserId>emp002</UserId>
                  <FirstName>Jane</FirstName>
                  <LastName>Smith</LastName>
                  <Department>Marketing</Department>
                </User>
                <User>
                  <UserId>emp003</UserId>
                  <FirstName>Bob</FirstName>
                  <LastName>Johnson</LastName>
                  <Department>Sales</Department>
                </User>
              </Users>
              <TotalCount>3</TotalCount>
            </ListUsersResponse>
          delay: 100ms

        # Search users
        SearchUsers:
          soapAction: "http://enterprise.example.com/user/SearchUsers"
          match:
            xpath:
              "//Department/text()": "Engineering"
          response: |
            <SearchUsersResponse xmlns="http://enterprise.example.com/user">
              <Users>
                <User>
                  <UserId>emp001</UserId>
                  <FirstName>John</FirstName>
                  <LastName>Doe</LastName>
                  <Department>Engineering</Department>
                </User>
              </Users>
              <TotalCount>1</TotalCount>
            </SearchUsersResponse>

        # Authenticate user
        Authenticate:
          soapAction: "http://enterprise.example.com/user/Authenticate"
          match:
            xpath:
              "//Username/text()": "admin"
              "//Password/text()": "secret"
          response: |
            <AuthenticateResponse xmlns="http://enterprise.example.com/user">
              <Success>true</Success>
              <Token>eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...</Token>
              <ExpiresAt>{{now}}</ExpiresAt>
            </AuthenticateResponse>

        # Authenticate user - failure
        Authenticate:
          soapAction: "http://enterprise.example.com/user/Authenticate"
          fault:
            code: soap:Client
            message: "Authentication failed"
            detail: |
              <AuthError xmlns="http://enterprise.example.com/errors">
                <Code>INVALID_CREDENTIALS</Code>
                <Message>Invalid username or password</Message>
                <AttemptsRemaining>2</AttemptsRemaining>
              </AuthError>
```

### Banking Integration Service

A banking SOAP service demonstrating financial transactions:

```yaml
mocks:
  - id: banking-soap
    name: Banking Integration Service
    type: soap
    enabled: true
    soap:
      path: /soap/Banking
      wsdlFile: ./wsdl/banking.wsdl

      operations:
        GetAccountBalance:
          soapAction: "http://bank.example.com/GetAccountBalance"
          match:
            xpath:
              "//AccountNumber/text()": "1234567890"
          response: |
            <GetAccountBalanceResponse xmlns="http://bank.example.com/banking">
              <Account>
                <AccountNumber>1234567890</AccountNumber>
                <AccountType>Checking</AccountType>
                <Balance>5432.10</Balance>
                <Currency>USD</Currency>
                <AvailableBalance>5232.10</AvailableBalance>
                <AsOf>{{now}}</AsOf>
              </Account>
            </GetAccountBalanceResponse>

        GetAccountBalance:
          soapAction: "http://bank.example.com/GetAccountBalance"
          match:
            xpath:
              "//AccountNumber/text()": "0000000000"
          fault:
            code: soap:Client
            message: "Account not found"
            detail: |
              <BankingError xmlns="http://bank.example.com/errors">
                <ErrorCode>ACCOUNT_NOT_FOUND</ErrorCode>
                <ErrorMessage>The specified account does not exist</ErrorMessage>
              </BankingError>

        TransferFunds:
          soapAction: "http://bank.example.com/TransferFunds"
          response: |
            <TransferFundsResponse xmlns="http://bank.example.com/banking">
              <TransactionId>TXN-{{uuid}}</TransactionId>
              <Status>Completed</Status>
              <Timestamp>{{now}}</Timestamp>
              <ConfirmationNumber>CNF-{{timestamp}}</ConfirmationNumber>
            </TransferFundsResponse>
          delay: 200ms

        TransferFunds:
          soapAction: "http://bank.example.com/TransferFunds"
          match:
            xpath:
              "//Amount/text()": "1000000"
          fault:
            code: soap:Server
            message: "Transfer limit exceeded"
            detail: |
              <BankingError xmlns="http://bank.example.com/errors">
                <ErrorCode>LIMIT_EXCEEDED</ErrorCode>
                <ErrorMessage>Daily transfer limit exceeded</ErrorMessage>
                <MaxAllowed>50000.00</MaxAllowed>
              </BankingError>

        GetTransactionHistory:
          soapAction: "http://bank.example.com/GetTransactionHistory"
          response: |
            <GetTransactionHistoryResponse xmlns="http://bank.example.com/banking">
              <Transactions>
                <Transaction>
                  <TransactionId>TXN-001</TransactionId>
                  <Type>Debit</Type>
                  <Amount>150.00</Amount>
                  <Description>Online Purchase</Description>
                  <Date>2024-01-14T15:30:00Z</Date>
                </Transaction>
                <Transaction>
                  <TransactionId>TXN-002</TransactionId>
                  <Type>Credit</Type>
                  <Amount>2500.00</Amount>
                  <Description>Direct Deposit</Description>
                  <Date>2024-01-15T09:00:00Z</Date>
                </Transaction>
              </Transactions>
            </GetTransactionHistoryResponse>
          delay: 150ms
```

## CLI Commands

### Validate WSDL

Validate a WSDL file before deployment:

```bash
mockd soap validate service.wsdl
```

```
WSDL valid: service.wsdl
  Services: 1
  Operations: 4
  Messages: 8
```

Validate with full path:

```bash
mockd soap validate ./wsdl/user-management.wsdl
```

### Call SOAP Operations

Test SOAP operations against a running endpoint:

```bash
# Call an operation with auto-generated body
mockd soap call http://localhost:4280/soap/UserService GetUser

# Call with SOAPAction header
mockd soap call http://localhost:4280/soap/UserService GetUser \
  -a "http://example.com/GetUser"

# Call with custom body
mockd soap call http://localhost:4280/soap/UserService GetUser \
  -b '<GetUser xmlns="http://example.com/"><UserId>123</UserId></GetUser>'

# Call with body from file
mockd soap call http://localhost:4280/soap/UserService GetUser \
  -b @request.xml

# Use SOAP 1.2
mockd soap call http://localhost:4280/soap/UserService GetUser --soap12

# Add custom headers
mockd soap call http://localhost:4280/soap/UserService GetUser \
  -H "Authorization:Bearer token123"
```

### Call Command Options

| Flag | Description |
|------|-------------|
| `-a, --action` | SOAPAction header value |
| `-b, --body` | SOAP body content (XML) or `@filename` |
| `-H, --header` | Additional headers (`key:value,key2:value2`) |
| `--soap12` | Use SOAP 1.2 (default: SOAP 1.1) |
| `--pretty` | Pretty print output (default: true) |
| `--timeout` | Request timeout in seconds (default: 30) |

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

### Test SOAP 1.2

SOAP 1.2 uses different content type and namespace:

```bash
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: application/soap+xml; charset=utf-8" \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <UserId>123</UserId>
    </GetUser>
  </soap:Body>
</soap:Envelope>'
```

### Test Fault Responses

Trigger a fault by matching the error condition:

```bash
curl -X POST http://localhost:4280/soap/UserService \
  -H "Content-Type: text/xml" \
  -H "SOAPAction: http://example.com/GetUser" \
  -d '<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <UserId>999</UserId>
    </GetUser>
  </soap:Body>
</soap:Envelope>'
```

### Test WSDL Retrieval

```bash
# Get WSDL document
curl http://localhost:4280/soap/UserService?wsdl

# Save WSDL to file
curl -o service.wsdl http://localhost:4280/soap/UserService?wsdl
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

### Test Latency Simulation

Use delays to test timeout handling:

```yaml
operations:
  SlowOperation:
    soapAction: "http://example.com/SlowOperation"
    response: |
      <SlowOperationResponse>
        <Status>Complete</Status>
      </SlowOperationResponse>
    delay: 5s  # Test client timeout behavior
```

## Namespace Handling

SOAP services typically use XML namespaces. mockd handles these in XPath matching and response generation.

### XPath with Namespaces

When matching elements with namespaces, use local-name() or namespace prefixes:

```yaml
operations:
  GetUser:
    match:
      xpath:
        "//ns:UserId": "123"
```

Or match by element name only:

```yaml
operations:
  GetUser:
    match:
      xpath:
        "//UserId/text()": "123"
```

### Response Namespaces

Always include proper namespace declarations in responses:

```yaml
operations:
  GetUser:
    response: |
      <GetUserResponse xmlns="http://example.com/user"
                       xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
        <User>
          <Id>123</Id>
          <Name>John Doe</Name>
        </User>
      </GetUserResponse>
```

## Next Steps

- [Response Templating](response-templating.md) - Dynamic response values
- [Request Matching](request-matching.md) - HTTP-level matching
- [TLS/HTTPS](tls-https.md) - Secure SOAP endpoints
