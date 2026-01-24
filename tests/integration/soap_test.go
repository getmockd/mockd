package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/soap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// soapTestBundle groups SOAP server and related config for tests
type soapTestBundle struct {
	Handler *soap.Handler
	Server  *http.Server
	Port    int
	BaseURL string
}

// setupSOAPServer creates and starts a SOAP handler for testing
func setupSOAPServer(t *testing.T, cfg *soap.SOAPConfig) *soapTestBundle {
	t.Helper()
	port := getFreePort()

	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err, "Failed to create SOAP handler")

	mux := http.NewServeMux()
	mux.Handle(cfg.Path, handler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	return &soapTestBundle{
		Handler: handler,
		Server:  server,
		Port:    port,
		BaseURL: fmt.Sprintf("http://localhost:%d%s", port, cfg.Path),
	}
}

// soapRequest sends a SOAP 1.1 request and returns the response
func soapRequest(t *testing.T, url, soapAction, body string) (*http.Response, []byte) {
	t.Helper()
	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>%s</soap:Body>
</soap:Envelope>`, body)

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(envelope))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	if soapAction != "" {
		req.Header.Set("SOAPAction", soapAction)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	return resp, respBody
}

// soap12Request sends a SOAP 1.2 request and returns the response
func soap12Request(t *testing.T, url, action, body string) (*http.Response, []byte) {
	t.Helper()
	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>%s</soap:Body>
</soap:Envelope>`, body)

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(envelope))
	require.NoError(t, err)

	contentType := "application/soap+xml; charset=utf-8"
	if action != "" {
		contentType = fmt.Sprintf(`application/soap+xml; charset=utf-8; action="%s"`, action)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	return resp, respBody
}

// Sample WSDL for testing
const testWSDL = `<?xml version="1.0" encoding="UTF-8"?>
<definitions name="UserService"
    xmlns="http://schemas.xmlsoap.org/wsdl/"
    xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
    xmlns:tns="http://example.com/userservice"
    xmlns:xsd="http://www.w3.org/2001/XMLSchema"
    targetNamespace="http://example.com/userservice">
    
  <types>
    <xsd:schema targetNamespace="http://example.com/userservice">
      <xsd:element name="GetUserRequest">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="userId" type="xsd:string"/>
          </xsd:sequence>
        </xsd:complexType>
      </xsd:element>
      <xsd:element name="GetUserResponse">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="id" type="xsd:string"/>
            <xsd:element name="name" type="xsd:string"/>
            <xsd:element name="email" type="xsd:string"/>
          </xsd:sequence>
        </xsd:complexType>
      </xsd:element>
      <xsd:element name="CreateUserRequest">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="name" type="xsd:string"/>
            <xsd:element name="email" type="xsd:string"/>
          </xsd:sequence>
        </xsd:complexType>
      </xsd:element>
      <xsd:element name="CreateUserResponse">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="userId" type="xsd:string"/>
            <xsd:element name="status" type="xsd:string"/>
          </xsd:sequence>
        </xsd:complexType>
      </xsd:element>
      <xsd:element name="DeleteUserRequest">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="userId" type="xsd:string"/>
          </xsd:sequence>
        </xsd:complexType>
      </xsd:element>
      <xsd:element name="DeleteUserResponse">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="success" type="xsd:boolean"/>
          </xsd:sequence>
        </xsd:complexType>
      </xsd:element>
    </xsd:schema>
  </types>

  <message name="GetUserRequest">
    <part name="parameters" element="tns:GetUserRequest"/>
  </message>
  <message name="GetUserResponse">
    <part name="parameters" element="tns:GetUserResponse"/>
  </message>
  <message name="CreateUserRequest">
    <part name="parameters" element="tns:CreateUserRequest"/>
  </message>
  <message name="CreateUserResponse">
    <part name="parameters" element="tns:CreateUserResponse"/>
  </message>
  <message name="DeleteUserRequest">
    <part name="parameters" element="tns:DeleteUserRequest"/>
  </message>
  <message name="DeleteUserResponse">
    <part name="parameters" element="tns:DeleteUserResponse"/>
  </message>

  <portType name="UserServicePortType">
    <operation name="GetUser">
      <input message="tns:GetUserRequest"/>
      <output message="tns:GetUserResponse"/>
    </operation>
    <operation name="CreateUser">
      <input message="tns:CreateUserRequest"/>
      <output message="tns:CreateUserResponse"/>
    </operation>
    <operation name="DeleteUser">
      <input message="tns:DeleteUserRequest"/>
      <output message="tns:DeleteUserResponse"/>
    </operation>
  </portType>

  <binding name="UserServiceBinding" type="tns:UserServicePortType">
    <soap:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
    <operation name="GetUser">
      <soap:operation soapAction="http://example.com/GetUser"/>
      <input><soap:body use="literal"/></input>
      <output><soap:body use="literal"/></output>
    </operation>
    <operation name="CreateUser">
      <soap:operation soapAction="http://example.com/CreateUser"/>
      <input><soap:body use="literal"/></input>
      <output><soap:body use="literal"/></output>
    </operation>
    <operation name="DeleteUser">
      <soap:operation soapAction="http://example.com/DeleteUser"/>
      <input><soap:body use="literal"/></input>
      <output><soap:body use="literal"/></output>
    </operation>
  </binding>

  <service name="UserService">
    <port name="UserServicePort" binding="tns:UserServiceBinding">
      <soap:address location="http://localhost/soap/user"/>
    </port>
  </service>
</definitions>`

// ============================================================================
// User Story 1: Basic SOAP 1.1 Request
// ============================================================================

func TestSOAP_US1_BasicSOAP11Request(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-soap11",
		Name:    "Test SOAP 1.1 Service",
		Path:    "/soap/user",
		WSDL:    testWSDL,
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   `<GetUserResponse xmlns="http://example.com/"><id>user-123</id><name>John Doe</name><email>john@example.com</email></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Send SOAP 1.1 request
	resp, body := soapRequest(t, bundle.BaseURL, "http://example.com/GetUser",
		`<GetUser xmlns="http://example.com/"><userId>123</userId></GetUser>`)

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")

	// Verify SOAP envelope structure
	assert.Contains(t, string(body), "soap:Envelope")
	assert.Contains(t, string(body), "soap:Body")
	assert.Contains(t, string(body), "http://schemas.xmlsoap.org/soap/envelope/")

	// Verify response content
	assert.Contains(t, string(body), "<id>user-123</id>")
	assert.Contains(t, string(body), "<name>John Doe</name>")
	assert.Contains(t, string(body), "<email>john@example.com</email>")
}

func TestSOAP_US1_SOAP11WithoutSOAPAction(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-soap11-no-action",
		Name:    "Test SOAP 1.1 Without Action",
		Path:    "/soap/noaction",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>user-456</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Send SOAP 1.1 request without SOAPAction header
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser xmlns="http://example.com/"><userId>456</userId></GetUser>`)

	// Should match by operation element name
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<id>user-456</id>")
}

// ============================================================================
// User Story 2: Basic SOAP 1.2 Request
// ============================================================================

func TestSOAP_US2_BasicSOAP12Request(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-soap12",
		Name:    "Test SOAP 1.2 Service",
		Path:    "/soap/v12",
		WSDL:    testWSDL,
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   `<GetUserResponse xmlns="http://example.com/"><id>user-789</id><name>Jane Smith</name><email>jane@example.com</email></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Send SOAP 1.2 request
	resp, body := soap12Request(t, bundle.BaseURL, "http://example.com/GetUser",
		`<GetUser xmlns="http://example.com/"><userId>789</userId></GetUser>`)

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/soap+xml")

	// Verify SOAP 1.2 envelope structure
	assert.Contains(t, string(body), "soap:Envelope")
	assert.Contains(t, string(body), "http://www.w3.org/2003/05/soap-envelope")

	// Verify response content
	assert.Contains(t, string(body), "<id>user-789</id>")
	assert.Contains(t, string(body), "<name>Jane Smith</name>")
}

func TestSOAP_US2_SOAP12ActionInContentType(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-soap12-action-ct",
		Name:    "Test SOAP 1.2 Action in Content-Type",
		Path:    "/soap/v12action",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"CreateUser": {
				SOAPAction: "http://example.com/CreateUser",
				Response:   `<CreateUserResponse xmlns="http://example.com/"><userId>new-user-001</userId><status>created</status></CreateUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// SOAP 1.2 action is in Content-Type header
	resp, body := soap12Request(t, bundle.BaseURL, "http://example.com/CreateUser",
		`<CreateUser xmlns="http://example.com/"><name>New User</name><email>new@example.com</email></CreateUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<userId>new-user-001</userId>")
	assert.Contains(t, string(body), "<status>created</status>")
}

// ============================================================================
// User Story 3: WSDL Serving
// ============================================================================

func TestSOAP_US3_WSDLServing(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-wsdl",
		Name:    "Test WSDL Service",
		Path:    "/soap/wsdl",
		WSDL:    testWSDL,
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Request WSDL with ?wsdl query param
	resp, err := http.Get(bundle.BaseURL + "?wsdl")
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")

	// Verify WSDL content
	assert.Contains(t, string(body), "definitions")
	assert.Contains(t, string(body), "UserService")
	assert.Contains(t, string(body), "GetUser")
	assert.Contains(t, string(body), "CreateUser")
}

func TestSOAP_US3_WSDLNotAvailable(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-no-wsdl",
		Name:    "Test No WSDL Service",
		Path:    "/soap/nowsdl",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Request WSDL when not configured
	resp, err := http.Get(bundle.BaseURL + "?wsdl")
	require.NoError(t, err)
	resp.Body.Close()

	// Should return 404
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSOAP_US3_WSDLCaseInsensitive(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-wsdl-case",
		Name:    "Test WSDL Case Insensitive",
		Path:    "/soap/wsdlcase",
		WSDL:    testWSDL,
		Enabled: true,
	}

	bundle := setupSOAPServer(t, cfg)

	// Test various case combinations
	testCases := []string{"?wsdl", "?WSDL", "?Wsdl", "?WsDl"}

	for _, query := range testCases {
		t.Run(query, func(t *testing.T) {
			resp, err := http.Get(bundle.BaseURL + query)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// ============================================================================
// User Story 4: Operation Matching
// ============================================================================

func TestSOAP_US4_OperationMatchingBySOAPAction(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-op-match",
		Name:    "Test Operation Matching",
		Path:    "/soap/ops",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   `<GetUserResponse><type>get</type><id>user-123</id></GetUserResponse>`,
			},
			"CreateUser": {
				SOAPAction: "http://example.com/CreateUser",
				Response:   `<CreateUserResponse><type>create</type><id>new-user</id></CreateUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Test GetUser operation
	resp1, body1 := soapRequest(t, bundle.BaseURL, "http://example.com/GetUser",
		`<GetUser><userId>123</userId></GetUser>`)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	assert.Contains(t, string(body1), "<type>get</type>")
	assert.Contains(t, string(body1), "<id>user-123</id>")

	// Test CreateUser operation
	resp2, body2 := soapRequest(t, bundle.BaseURL, "http://example.com/CreateUser",
		`<CreateUser><name>New User</name></CreateUser>`)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Contains(t, string(body2), "<type>create</type>")
	assert.Contains(t, string(body2), "<id>new-user</id>")
}

func TestSOAP_US4_OperationMatchingByElementName(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-op-elem",
		Name:    "Test Operation by Element",
		Path:    "/soap/elem",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><source>element-match</source></GetUserResponse>`,
			},
			"DeleteUser": {
				Response: `<DeleteUserResponse><deleted>true</deleted></DeleteUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Match by element name
	resp1, body1 := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>123</userId></GetUser>`)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	assert.Contains(t, string(body1), "<source>element-match</source>")

	resp2, body2 := soapRequest(t, bundle.BaseURL, "",
		`<DeleteUser><userId>456</userId></DeleteUser>`)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Contains(t, string(body2), "<deleted>true</deleted>")
}

func TestSOAP_US4_UnknownOperation(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-unknown-op",
		Name:    "Test Unknown Operation",
		Path:    "/soap/unknown",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Request unknown operation
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<UnknownOperation><data>test</data></UnknownOperation>`)

	// Should return SOAP Fault
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, string(body), "Fault")
	assert.Contains(t, string(body), "Unknown operation")
}

// ============================================================================
// User Story 5: XPath Matching
// ============================================================================

func TestSOAP_US5_XPathMatching(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-xpath",
		Name:    "Test XPath Matching",
		Path:    "/soap/xpath",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Match: &soap.SOAPMatch{
					XPath: map[string]string{
						"//userId": "123",
					},
				},
				Response: `<GetUserResponse><id>user-123</id><matched>xpath-condition</matched></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Request with matching userId
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>123</userId></GetUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<id>user-123</id>")
	assert.Contains(t, string(body), "<matched>xpath-condition</matched>")
}

func TestSOAP_US5_XPathNoMatch(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-xpath-nomatch",
		Name:    "Test XPath No Match",
		Path:    "/soap/xpathno",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Match: &soap.SOAPMatch{
					XPath: map[string]string{
						"//userId": "123",
					},
				},
				Response: `<GetUserResponse><id>user-123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Request with non-matching userId - should return unknown operation fault
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>456</userId></GetUser>`)

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, string(body), "Fault")
	assert.Contains(t, string(body), "Unknown operation")
}

func TestSOAP_US5_MultipleXPathConditions(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-xpath-multi",
		Name:    "Test Multiple XPath",
		Path:    "/soap/xpathmulti",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Match: &soap.SOAPMatch{
					XPath: map[string]string{
						"//userId": "123",
						"//role":   "admin",
					},
				},
				Response: `<GetUserResponse><access>full</access></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Both conditions must match
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>123</userId><role>admin</role></GetUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<access>full</access>")
}

// ============================================================================
// User Story 6: SOAP Fault
// ============================================================================

func TestSOAP_US6_SOAPFault11(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-fault11",
		Name:    "Test SOAP 1.1 Fault",
		Path:    "/soap/fault11",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Fault: &soap.SOAPFault{
					Code:    "soap:Client",
					Message: "User not found",
					Detail:  "<errorCode>USER_404</errorCode>",
				},
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>nonexistent</userId></GetUser>`)

	// Verify fault response
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")

	// SOAP 1.1 fault structure
	assert.Contains(t, string(body), "<faultcode>soap:Client</faultcode>")
	assert.Contains(t, string(body), "<faultstring>User not found</faultstring>")
	assert.Contains(t, string(body), "<detail><errorCode>USER_404</errorCode></detail>")
}

func TestSOAP_US6_SOAPFault12(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-fault12",
		Name:    "Test SOAP 1.2 Fault",
		Path:    "/soap/fault12",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Fault: &soap.SOAPFault{
					Code:    "soap:Client",
					Message: "Invalid request format",
				},
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	resp, body := soap12Request(t, bundle.BaseURL, "",
		`<GetUser><userId>invalid</userId></GetUser>`)

	// Verify fault response
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/soap+xml")

	// SOAP 1.2 fault structure (Client -> Sender)
	assert.Contains(t, string(body), "soap:Sender")
	assert.Contains(t, string(body), "Invalid request format")
	assert.Contains(t, string(body), "soap:Text")
}

func TestSOAP_US6_ServerFault(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-server-fault",
		Name:    "Test Server Fault",
		Path:    "/soap/serverfault",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Fault: &soap.SOAPFault{
					Code:    "soap:Server",
					Message: "Internal server error",
				},
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// SOAP 1.1 Server fault
	resp1, body1 := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>123</userId></GetUser>`)
	assert.Equal(t, http.StatusInternalServerError, resp1.StatusCode)
	assert.Contains(t, string(body1), "<faultcode>soap:Server</faultcode>")

	// SOAP 1.2 Server -> Receiver fault
	resp2, body2 := soap12Request(t, bundle.BaseURL, "",
		`<GetUser><userId>123</userId></GetUser>`)
	assert.Equal(t, http.StatusInternalServerError, resp2.StatusCode)
	assert.Contains(t, string(body2), "soap:Receiver")
}

// ============================================================================
// User Story 7: Namespace Handling
// ============================================================================

func TestSOAP_US7_NamespaceInRequest(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-namespace",
		Name:    "Test Namespace Handling",
		Path:    "/soap/ns",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<tns:GetUserResponse xmlns:tns="http://example.com/userservice"><tns:id>user-ns</tns:id></tns:GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Request with namespace prefix
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<tns:GetUser xmlns:tns="http://example.com/userservice"><tns:userId>123</tns:userId></tns:GetUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "user-ns")
}

func TestSOAP_US7_MultipleNamespaces(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-multi-ns",
		Name:    "Test Multiple Namespaces",
		Path:    "/soap/multins",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse xmlns="http://example.com/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
					<id>multi-ns-user</id>
					<status xsi:nil="false">active</status>
				</GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser xmlns="http://example.com/" xmlns:custom="http://custom.example.com/">
			<userId>123</userId>
			<custom:metadata>test</custom:metadata>
		</GetUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "multi-ns-user")
}

// ============================================================================
// User Story 8: Templating with XPath Variables
// ============================================================================

func TestSOAP_US8_XPathTemplating(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-xpath-template",
		Name:    "Test XPath Templating",
		Path:    "/soap/tpl",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse>
					<requestedId>{{xpath://GetUser/userId}}</requestedId>
					<requestedName>{{xpath://GetUser/name}}</requestedName>
				</GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>ABC123</userId><name>Test User</name></GetUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<requestedId>ABC123</requestedId>")
	assert.Contains(t, string(body), "<requestedName>Test User</requestedName>")
}

func TestSOAP_US8_XPathTemplatingNestedElements(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-xpath-nested",
		Name:    "Test XPath Nested Templating",
		Path:    "/soap/tplnested",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"CreateUser": {
				Response: `<CreateUserResponse>
					<userId>new-{{xpath://CreateUser/user/id}}</userId>
					<email>{{xpath://CreateUser/user/email}}</email>
				</CreateUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<CreateUser>
			<user>
				<id>user-001</id>
				<email>new@example.com</email>
			</user>
		</CreateUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<userId>new-user-001</userId>")
	assert.Contains(t, string(body), "<email>new@example.com</email>")
}

func TestSOAP_US8_XPathMissingValue(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-xpath-missing",
		Name:    "Test XPath Missing Value",
		Path:    "/soap/tplmissing",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse>
					<requestedId>{{xpath://GetUser/userId}}</requestedId>
					<missingField>{{xpath://GetUser/nonexistent}}</missingField>
				</GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<GetUser><userId>ABC123</userId></GetUser>`)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<requestedId>ABC123</requestedId>")
	// Missing XPath should result in empty string
	assert.Contains(t, string(body), "<missingField></missingField>")
}

// ============================================================================
// User Story 9: Delays
// ============================================================================

func TestSOAP_US9_ResponseDelay(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-delay",
		Name:    "Test Delay",
		Path:    "/soap/delay",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"SlowOperation": {
				Delay:    "100ms",
				Response: `<SlowOperationResponse><status>completed</status></SlowOperationResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	start := time.Now()
	resp, body := soapRequest(t, bundle.BaseURL, "",
		`<SlowOperation><data>test</data></SlowOperation>`)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<status>completed</status>")
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(80), "Should have delay of at least 80ms")
}

func TestSOAP_US9_NoDelay(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-no-delay",
		Name:    "Test No Delay",
		Path:    "/soap/nodelay",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"FastOperation": {
				Response: `<FastOperationResponse><status>fast</status></FastOperationResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	start := time.Now()
	resp, _ := soapRequest(t, bundle.BaseURL, "",
		`<FastOperation><data>test</data></FastOperation>`)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, elapsed.Milliseconds(), int64(50), "Should be fast without delay")
}

func TestSOAP_US9_DelayInSeconds(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-delay-sec",
		Name:    "Test Delay Seconds",
		Path:    "/soap/delaysec",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"SlowOp": {
				Delay:    "150ms",
				Response: `<SlowOpResponse><done>true</done></SlowOpResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	start := time.Now()
	resp, _ := soapRequest(t, bundle.BaseURL, "",
		`<SlowOp/>`)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(130), "Should have delay")
}

// ============================================================================
// User Story 10: Multiple Operations in Same Mock
// ============================================================================

func TestSOAP_US10_MultipleOperations(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-multi-ops",
		Name:    "Test Multiple Operations",
		Path:    "/soap/multiops",
		WSDL:    testWSDL,
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   `<GetUserResponse><operation>get</operation><id>{{xpath://GetUser/userId}}</id></GetUserResponse>`,
			},
			"CreateUser": {
				SOAPAction: "http://example.com/CreateUser",
				Response:   `<CreateUserResponse><operation>create</operation><userId>new-user</userId></CreateUserResponse>`,
			},
			"DeleteUser": {
				SOAPAction: "http://example.com/DeleteUser",
				Response:   `<DeleteUserResponse><operation>delete</operation><success>true</success></DeleteUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Test GetUser
	resp1, body1 := soapRequest(t, bundle.BaseURL, "http://example.com/GetUser",
		`<GetUser><userId>user-001</userId></GetUser>`)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	assert.Contains(t, string(body1), "<operation>get</operation>")
	assert.Contains(t, string(body1), "<id>user-001</id>")

	// Test CreateUser
	resp2, body2 := soapRequest(t, bundle.BaseURL, "http://example.com/CreateUser",
		`<CreateUser><name>New User</name><email>new@example.com</email></CreateUser>`)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Contains(t, string(body2), "<operation>create</operation>")
	assert.Contains(t, string(body2), "<userId>new-user</userId>")

	// Test DeleteUser
	resp3, body3 := soapRequest(t, bundle.BaseURL, "http://example.com/DeleteUser",
		`<DeleteUser><userId>user-001</userId></DeleteUser>`)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	assert.Contains(t, string(body3), "<operation>delete</operation>")
	assert.Contains(t, string(body3), "<success>true</success>")
}

// ============================================================================
// Additional Edge Cases
// ============================================================================

func TestSOAP_MethodNotAllowed(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-method",
		Name:    "Test Method",
		Path:    "/soap/method",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// GET should not be allowed for SOAP operations (only for WSDL)
	resp, err := http.Get(bundle.BaseURL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestSOAP_InvalidXML(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-invalid-xml",
		Name:    "Test Invalid XML",
		Path:    "/soap/invalid",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Send invalid XML
	req, err := http.NewRequest(http.MethodPost, bundle.BaseURL, strings.NewReader("not valid xml"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/xml")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, string(body), "Fault")
}

func TestSOAP_MissingEnvelope(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-missing-envelope",
		Name:    "Test Missing Envelope",
		Path:    "/soap/noenvelope",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Send XML without SOAP envelope
	req, err := http.NewRequest(http.MethodPost, bundle.BaseURL,
		strings.NewReader(`<?xml version="1.0"?><GetUser><userId>123</userId></GetUser>`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/xml")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, string(body), "Fault")
	assert.Contains(t, string(body), "Envelope")
}

func TestSOAP_EmptyBody(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-empty-body",
		Name:    "Test Empty Body",
		Path:    "/soap/empty",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	// Send envelope with empty body
	resp, body := soapRequest(t, bundle.BaseURL, "", "")

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, string(body), "Fault")
}

func TestSOAP_ConcurrentRequests(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-concurrent",
		Name:    "Test Concurrent",
		Path:    "/soap/concurrent",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>concurrent-user</id></GetUserResponse>`,
			},
		},
	}

	bundle := setupSOAPServer(t, cfg)

	numRequests := 20
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(reqNum int) {
			resp, _ := soapRequest(t, bundle.BaseURL, "",
				fmt.Sprintf(`<GetUser><userId>%d</userId></GetUser>`, reqNum))
			results <- resp.StatusCode
		}(i)
	}

	successCount := 0
	for i := 0; i < numRequests; i++ {
		if <-results == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
}

func TestSOAP_HandlerMetadata(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-metadata",
		Name:    "Test Metadata Service",
		Path:    "/soap/meta",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				Response: `<GetUserResponse><id>123</id></GetUserResponse>`,
			},
		},
	}

	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err)

	// Test metadata
	meta := handler.Metadata()
	assert.Equal(t, "test-metadata", meta.ID)
	assert.Equal(t, "Test Metadata Service", meta.Name)
	assert.Equal(t, "soap", string(meta.Protocol))

	// Test ID
	assert.Equal(t, "test-metadata", handler.ID())

	// Test Pattern
	assert.Equal(t, "/soap/meta", handler.Pattern())
}

func TestSOAP_HealthCheck(t *testing.T) {
	cfg := &soap.SOAPConfig{
		ID:      "test-health",
		Name:    "Test Health",
		Path:    "/soap/health",
		Enabled: true,
	}

	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err)

	health := handler.Health(context.Background())
	assert.Equal(t, "healthy", string(health.Status))
}
