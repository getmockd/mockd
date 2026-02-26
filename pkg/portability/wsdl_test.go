package portability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/soap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readTestWSDL(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "failed to read test WSDL %s", name)
	return data
}

// --- WI-11: Parse sample WSDL files ---

func TestWSDLImporter_ParseCalculator(t *testing.T) {
	data := readTestWSDL(t, "calculator.wsdl")
	importer := &WSDLImporter{}

	collection, err := importer.Import(data)
	require.NoError(t, err)
	require.NotNil(t, collection)

	assert.Equal(t, "1.0", collection.Version)
	assert.Equal(t, "CalculatorService", collection.Name)
	require.Len(t, collection.Mocks, 1)

	m := collection.Mocks[0]
	assert.Equal(t, mock.TypeSOAP, m.Type)
	assert.True(t, *m.Enabled)
	assert.Contains(t, m.Name, "CalculatorService")

	// SOAP spec
	require.NotNil(t, m.SOAP)
	assert.Equal(t, "/soap/calculator", m.SOAP.Path)
	assert.NotEmpty(t, m.SOAP.WSDL, "should embed original WSDL")
	assert.Contains(t, m.SOAP.WSDL, "CalculatorService")

	// Operations
	require.Len(t, m.SOAP.Operations, 2)

	addOp, ok := m.SOAP.Operations["Add"]
	require.True(t, ok, "should have Add operation")
	assert.Equal(t, "http://example.com/calculator/Add", addOp.SOAPAction)
	assert.NotEmpty(t, addOp.Response, "should have generated response XML")
	assert.Contains(t, addOp.Response, "AddResponse")

	subOp, ok := m.SOAP.Operations["Subtract"]
	require.True(t, ok, "should have Subtract operation")
	assert.Equal(t, "http://example.com/calculator/Subtract", subOp.SOAPAction)
	assert.Contains(t, subOp.Response, "SubtractResponse")
}

func TestWSDLImporter_ParseUserService(t *testing.T) {
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{}

	collection, err := importer.Import(data)
	require.NoError(t, err)
	require.NotNil(t, collection)

	require.Len(t, collection.Mocks, 1)
	m := collection.Mocks[0]
	assert.Equal(t, mock.TypeSOAP, m.Type)
	assert.Equal(t, "/soap/users", m.SOAP.Path)

	// Should have 4 operations
	require.Len(t, m.SOAP.Operations, 4)
	assert.Contains(t, m.SOAP.Operations, "GetUser")
	assert.Contains(t, m.SOAP.Operations, "CreateUser")
	assert.Contains(t, m.SOAP.Operations, "ListUsers")
	assert.Contains(t, m.SOAP.Operations, "DeleteUser")

	// Check SOAPAction values
	assert.Equal(t, "http://example.com/users/GetUser", m.SOAP.Operations["GetUser"].SOAPAction)
	assert.Equal(t, "http://example.com/users/CreateUser", m.SOAP.Operations["CreateUser"].SOAPAction)
}

func TestWSDLImporter_ParseUserService_WithXSDTypes(t *testing.T) {
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{}

	collection, err := importer.Import(data)
	require.NoError(t, err)

	// GetUserResponse should have XML generated from XSD
	getOp := collection.Mocks[0].SOAP.Operations["GetUser"]
	assert.Contains(t, getOp.Response, "GetUserResponse")
	// The response should reference the User complex type fields
	assert.Contains(t, getOp.Response, "<user>")
}

func TestWSDLImporter_MultipleBindingsSamePath(t *testing.T) {
	// Bug 3: WSDLs with SOAP 1.1 + SOAP 1.2 bindings on the same path should
	// produce one mock (operations merged by path), not duplicate mocks.
	wsdl := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<definitions name="MultiBindService"
             targetNamespace="http://example.com/multi"
             xmlns="http://schemas.xmlsoap.org/wsdl/"
             xmlns:tns="http://example.com/multi"
             xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
             xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/">
  <message name="DoThingInput"/>
  <message name="DoThingOutput"/>
  <portType name="MultiPort">
    <operation name="DoThing">
      <input message="tns:DoThingInput"/>
      <output message="tns:DoThingOutput"/>
    </operation>
  </portType>
  <binding name="MultiSoap11" type="tns:MultiPort">
    <soap:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
    <operation name="DoThing">
      <soap:operation soapAction="http://example.com/multi/DoThing"/>
    </operation>
  </binding>
  <binding name="MultiSoap12" type="tns:MultiPort">
    <soap12:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
    <operation name="DoThing">
      <soap12:operation soapAction="http://example.com/multi/DoThing"/>
    </operation>
  </binding>
  <service name="MultiService">
    <port name="Soap11Port" binding="tns:MultiSoap11">
      <soap:address location="http://localhost/soap/multi"/>
    </port>
    <port name="Soap12Port" binding="tns:MultiSoap12">
      <soap12:address location="http://localhost/soap/multi"/>
    </port>
  </service>
</definitions>`)

	importer := &WSDLImporter{}
	collection, err := importer.Import(wsdl)
	require.NoError(t, err)

	// Should produce exactly 1 mock (merged by path), not 2
	require.Len(t, collection.Mocks, 1, "multiple bindings on same path should merge into one mock")

	m := collection.Mocks[0]
	assert.Equal(t, "/soap/multi", m.SOAP.Path)
	assert.Contains(t, m.SOAP.Operations, "DoThing")
}

func TestWSDLImporter_ParsePrefixedWSDL(t *testing.T) {
	// user-service.wsdl uses wsdl: prefix — tests namespace-prefixed parsing
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{}

	collection, err := importer.Import(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)
	// Proves the parser handles wsdl: prefixed elements correctly
	assert.Len(t, collection.Mocks[0].SOAP.Operations, 4)
}

// --- WI-12: Generate mock config from parsed WSDL ---

func TestWSDLImporter_GeneratesValidMocks(t *testing.T) {
	data := readTestWSDL(t, "calculator.wsdl")
	importer := &WSDLImporter{}

	collection, err := importer.Import(data)
	require.NoError(t, err)

	for _, m := range collection.Mocks {
		// ID format
		assert.True(t, strings.HasPrefix(m.ID, "soap_"), "ID should have soap_ prefix")
		assert.Len(t, m.ID, 5+16, "ID should be soap_ + 16 hex chars")

		// Type
		assert.Equal(t, mock.TypeSOAP, m.Type)

		// Path
		assert.True(t, strings.HasPrefix(m.SOAP.Path, "/"), "path must start with /")

		// Each operation has either Response or Fault
		for opName, op := range m.SOAP.Operations {
			assert.True(t, op.Response != "" || op.Fault != nil,
				"operation %s must have response or fault", opName)
		}

		// Timestamps
		assert.False(t, m.CreatedAt.IsZero())
		assert.False(t, m.UpdatedAt.IsZero())
	}
}

func TestWSDLImporter_Stateful_UserService(t *testing.T) {
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{Stateful: true}

	collection, err := importer.Import(data)
	require.NoError(t, err)

	// Should generate stateful resources
	require.NotEmpty(t, collection.StatefulResources, "stateful mode should generate resources")

	// Find the "users" resource
	var usersResource *struct{ found bool }
	for _, rc := range collection.StatefulResources {
		if rc.Name == "users" {
			usersResource = &struct{ found bool }{true}
			assert.Equal(t, "/api/users", rc.BasePath)
			// Should have seed data from User complex type
			if assert.NotEmpty(t, rc.SeedData, "should have seed data from XSD") {
				seed := rc.SeedData[0]
				assert.Contains(t, seed, "id")
				assert.Contains(t, seed, "name")
				assert.Contains(t, seed, "email")
			}
			break
		}
	}
	assert.NotNil(t, usersResource, "should have 'users' stateful resource")

	// Operations should have stateful mappings
	m := collection.Mocks[0]
	getOp := m.SOAP.Operations["GetUser"]
	assert.Equal(t, "users", getOp.StatefulResource)
	assert.Equal(t, "get", getOp.StatefulAction)
	assert.Empty(t, getOp.Response, "stateful ops should not have canned response")

	createOp := m.SOAP.Operations["CreateUser"]
	assert.Equal(t, "users", createOp.StatefulResource)
	assert.Equal(t, "create", createOp.StatefulAction)

	listOp := m.SOAP.Operations["ListUsers"]
	assert.Equal(t, "users", listOp.StatefulResource)
	assert.Equal(t, "list", listOp.StatefulAction)

	deleteOp := m.SOAP.Operations["DeleteUser"]
	assert.Equal(t, "users", deleteOp.StatefulResource)
	assert.Equal(t, "delete", deleteOp.StatefulAction)
}

func TestWSDLImporter_Stateful_Calculator_NoMapping(t *testing.T) {
	// Calculator operations (Add, Subtract) don't match CRUD patterns
	data := readTestWSDL(t, "calculator.wsdl")
	importer := &WSDLImporter{Stateful: true}

	collection, err := importer.Import(data)
	require.NoError(t, err)

	// No stateful resources for non-CRUD operations
	assert.Empty(t, collection.StatefulResources, "calculator ops shouldn't generate stateful resources")

	// Operations should still have canned responses
	addOp := collection.Mocks[0].SOAP.Operations["Add"]
	assert.Empty(t, addOp.StatefulResource)
	assert.NotEmpty(t, addOp.Response)
}

// --- Format detection ---

func TestDetectFormat_WSDL_ByExtension(t *testing.T) {
	assert.Equal(t, FormatWSDL, DetectFormat([]byte("<anything/>"), "service.wsdl"))
}

func TestDetectFormat_WSDL_ByContent(t *testing.T) {
	content := `<?xml version="1.0"?><definitions name="Test" xmlns="http://schemas.xmlsoap.org/wsdl/"></definitions>`
	assert.Equal(t, FormatWSDL, DetectFormat([]byte(content), "unknown.xml"))
}

func TestDetectFormat_WSDL_PrefixedContent(t *testing.T) {
	content := `<?xml version="1.0"?><wsdl:definitions name="Test" xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/"></wsdl:definitions>`
	assert.Equal(t, FormatWSDL, DetectFormat([]byte(content), ""))
}

func TestParseFormat_WSDL(t *testing.T) {
	assert.Equal(t, FormatWSDL, ParseFormat("wsdl"))
	assert.Equal(t, FormatWSDL, ParseFormat("WSDL"))
}

func TestFormatWSDL_Properties(t *testing.T) {
	assert.True(t, FormatWSDL.IsValid())
	assert.True(t, FormatWSDL.CanImport())
	assert.False(t, FormatWSDL.CanExport())
	assert.Equal(t, "wsdl", FormatWSDL.String())
}

// --- Operation name heuristics (WI-05) ---

func TestInferStatefulMapping(t *testing.T) {
	tests := []struct {
		opName   string
		resource string
		action   string
	}{
		{"GetUser", "users", "get"},
		{"FindOrder", "orders", "get"},
		{"FetchProduct", "products", "get"},
		{"RetrieveAccount", "accounts", "get"},
		{"CreateUser", "users", "create"},
		{"AddItem", "items", "create"},
		{"InsertRecord", "records", "create"},
		{"UpdateUser", "users", "update"},
		{"ModifyOrder", "orders", "update"},
		{"DeleteUser", "users", "delete"},
		{"RemoveItem", "items", "delete"},
		{"ListUsers", "users", "list"},
		{"SearchProducts", "products", "list"},
		{"GetAllOrders", "orders", "list"},
		{"FindAllCustomers", "customers", "list"},
		// Already plural
		{"GetUsers", "users", "get"},
		{"ListProducts", "products", "list"},
		// No match
		{"Add", "", ""},
		{"Calculate", "", ""},
		{"Subtract", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.opName, func(t *testing.T) {
			resource, action := inferStatefulMapping(tt.opName)
			assert.Equal(t, tt.resource, resource, "resource for %s", tt.opName)
			assert.Equal(t, tt.action, action, "action for %s", tt.opName)
		})
	}
}

func TestNormalizeResourceName(t *testing.T) {
	assert.Equal(t, "users", normalizeResourceName("User"))
	assert.Equal(t, "orders", normalizeResourceName("Order"))
	assert.Equal(t, "products", normalizeResourceName("Products")) // already plural
	assert.Equal(t, "items", normalizeResourceName("Items"))
	assert.Equal(t, "orderitems", normalizeResourceName("OrderItem"))
}

// --- Error cases ---

func TestWSDLImporter_EmptyDocument(t *testing.T) {
	importer := &WSDLImporter{}
	_, err := importer.Import([]byte(""))
	assert.Error(t, err)
}

func TestWSDLImporter_InvalidXML(t *testing.T) {
	importer := &WSDLImporter{}
	_, err := importer.Import([]byte("not xml at all"))
	assert.Error(t, err)
	// etree may parse non-XML as empty doc or fail — either way we get an error
}

func TestWSDLImporter_WrongRootElement(t *testing.T) {
	importer := &WSDLImporter{}
	_, err := importer.Import([]byte(`<?xml version="1.0"?><html><body>not wsdl</body></html>`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected root element")
}

func TestWSDLImporter_WSDL20_NotSupported(t *testing.T) {
	importer := &WSDLImporter{}
	_, err := importer.Import([]byte(`<?xml version="1.0"?><description xmlns="http://www.w3.org/ns/wsdl"></description>`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WSDL 2.0")
}

func TestWSDLImporter_NoServices(t *testing.T) {
	importer := &WSDLImporter{}
	// Valid WSDL structure but no services
	wsdl := `<?xml version="1.0"?>
<definitions name="Empty" xmlns="http://schemas.xmlsoap.org/wsdl/">
  <message name="TestInput"/>
  <portType name="TestPortType">
    <operation name="Test">
      <input message="tns:TestInput"/>
    </operation>
  </portType>
</definitions>`
	_, err := importer.Import([]byte(wsdl))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no services found")
}

// --- Registry integration ---

func TestWSDLImporter_RegisteredInRegistry(t *testing.T) {
	importer := GetImporter(FormatWSDL)
	assert.NotNil(t, importer, "WSDL importer should be registered")
	assert.Equal(t, FormatWSDL, importer.Format())
}

// --- XSD type inference (WI-06) ---

func TestSampleValueForXSDType(t *testing.T) {
	assert.Equal(t, "sample", sampleValueForXSDType("string"))
	assert.Equal(t, "0", sampleValueForXSDType("int"))
	assert.Equal(t, "0", sampleValueForXSDType("xsd:int"))
	assert.Equal(t, "0.0", sampleValueForXSDType("float"))
	assert.Equal(t, "true", sampleValueForXSDType("boolean"))
	assert.Equal(t, "2026-01-01", sampleValueForXSDType("date"))
	assert.Equal(t, "2026-01-01T00:00:00Z", sampleValueForXSDType("dateTime"))
}

// --- Response XML generation (WI-07) ---

func TestGenerateXMLFromFields(t *testing.T) {
	types := map[string]*wsdlXSDElement{}
	fields := []wsdlXSDField{
		{Name: "id", Type: "string"},
		{Name: "count", Type: "int"},
		{Name: "active", Type: "boolean"},
	}

	xml := generateXMLFromFields("TestResponse", fields, types)

	assert.Contains(t, xml, "<TestResponse>")
	assert.Contains(t, xml, "</TestResponse>")
	assert.Contains(t, xml, "<id>sample</id>")
	assert.Contains(t, xml, "<count>0</count>")
	assert.Contains(t, xml, "<active>true</active>")
}

func TestGenerateXMLFromFields_NestedComplexType(t *testing.T) {
	types := map[string]*wsdlXSDElement{
		"Address": {
			Name: "Address",
			Fields: []wsdlXSDField{
				{Name: "street", Type: "string"},
				{Name: "city", Type: "string"},
			},
		},
	}
	fields := []wsdlXSDField{
		{Name: "name", Type: "string"},
		{Name: "address", Type: "Address"},
	}

	xml := generateXMLFromFields("UserResponse", fields, types)

	assert.Contains(t, xml, "<UserResponse>")
	assert.Contains(t, xml, "<name>sample</name>")
	assert.Contains(t, xml, "<address><street>sample</street><city>sample</city></address>")
}

// --- Metadata ---

func TestWSDLImporter_CollectionMetadata(t *testing.T) {
	data := readTestWSDL(t, "calculator.wsdl")
	importer := &WSDLImporter{}

	collection, err := importer.Import(data)
	require.NoError(t, err)

	require.NotNil(t, collection.Metadata)
	assert.Contains(t, collection.Metadata.Description, "WSDL")
	assert.Contains(t, collection.Metadata.Tags, "soap")
	assert.Contains(t, collection.Metadata.Tags, "wsdl-import")
}

// ==========================================================================
// WI-13: Integration tests — import WSDL → build SOAP handler → call endpoint
// ==========================================================================

// testStatefulExecutor is a simple mock executor for integration testing.
type testStatefulExecutor struct {
	lastReq *soap.StatefulRequest
	result  *soap.StatefulResult
}

func (e *testStatefulExecutor) ExecuteStateful(_ context.Context, req *soap.StatefulRequest) *soap.StatefulResult {
	e.lastReq = req
	if e.result != nil {
		return e.result
	}
	return &soap.StatefulResult{Success: true, Item: map[string]interface{}{"status": "ok"}}
}

// mockToSOAPConfig converts a mock.Mock SOAP spec to a soap.SOAPConfig, mirroring
// the conversion done by engine.MockManager.registerSOAPMock.
func mockToSOAPConfig(m *mock.Mock) *soap.SOAPConfig {
	cfg := &soap.SOAPConfig{
		ID:       m.ID,
		Name:     m.Name,
		Path:     m.SOAP.Path,
		WSDL:     m.SOAP.WSDL,
		WSDLFile: m.SOAP.WSDLFile,
		Enabled:  m.Enabled == nil || *m.Enabled,
	}
	if m.SOAP.Operations != nil {
		cfg.Operations = make(map[string]soap.OperationConfig)
		for name, op := range m.SOAP.Operations {
			soapOp := soap.OperationConfig{
				SOAPAction:       op.SOAPAction,
				Response:         op.Response,
				Delay:            op.Delay,
				StatefulResource: op.StatefulResource,
				StatefulAction:   op.StatefulAction,
			}
			if op.Fault != nil {
				soapOp.Fault = &soap.SOAPFault{
					Code:    op.Fault.Code,
					Message: op.Fault.Message,
					Detail:  op.Fault.Detail,
				}
			}
			if op.Match != nil {
				soapOp.Match = &soap.SOAPMatch{
					XPath: op.Match.XPath,
				}
			}
			cfg.Operations[name] = soapOp
		}
	}
	return cfg
}

// soap11Envelope wraps a body string in a SOAP 1.1 envelope.
func soap11Envelope(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>` + body + `</soap:Body>
</soap:Envelope>`
}

// callSOAPHandler sends a SOAP 1.1 POST request to the handler and returns
// the status code and response body string.
func callSOAPHandler(t *testing.T, handler http.Handler, path, soapAction, bodyXML string) (int, string) {
	t.Helper()
	envelope := soap11Envelope(bodyXML)
	req := httptest.NewRequest("POST", path, strings.NewReader(envelope))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	if soapAction != "" {
		req.Header.Set("SOAPAction", `"`+soapAction+`"`)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(respBody)
}

// TestWSDLIntegration_Calculator_CannedResponses imports calculator.wsdl,
// creates a SOAP handler from the imported config, and calls both operations.
func TestWSDLIntegration_Calculator_CannedResponses(t *testing.T) {
	// Step 1: Import the WSDL
	data := readTestWSDL(t, "calculator.wsdl")
	importer := &WSDLImporter{}
	collection, err := importer.Import(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	// Step 2: Convert to SOAP handler config
	m := collection.Mocks[0]
	cfg := mockToSOAPConfig(m)
	require.Len(t, cfg.Operations, 2, "expected 2 operations (Add, Subtract)")

	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err, "failed to create SOAP handler from imported config")

	// Step 3: Call the Add operation
	addAction := cfg.Operations["Add"].SOAPAction
	status, body := callSOAPHandler(t, handler, cfg.Path, addAction, `<Add><a>3</a><b>4</b></Add>`)
	assert.Equal(t, http.StatusOK, status, "Add should return 200")
	assert.Contains(t, body, "Envelope", "response should be a SOAP envelope")
	assert.Contains(t, body, "AddResponse", "response should contain AddResponse XML")

	// Step 4: Call the Subtract operation
	subAction := cfg.Operations["Subtract"].SOAPAction
	status, body = callSOAPHandler(t, handler, cfg.Path, subAction, `<Subtract><a>10</a><b>3</b></Subtract>`)
	assert.Equal(t, http.StatusOK, status, "Subtract should return 200")
	assert.Contains(t, body, "SubtractResponse", "response should contain SubtractResponse XML")

	// Step 5: Verify WSDL serving
	wsdlReq := httptest.NewRequest("GET", cfg.Path+"?wsdl", nil)
	wsdlW := httptest.NewRecorder()
	handler.ServeHTTP(wsdlW, wsdlReq)

	wsdlResp := wsdlW.Result()
	defer func() { _ = wsdlResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, wsdlResp.StatusCode, "WSDL should be served")
	wsdlBody, _ := io.ReadAll(wsdlResp.Body)
	assert.Contains(t, string(wsdlBody), "CalculatorService", "WSDL should contain service name")
}

// TestWSDLIntegration_UserService_CannedResponses imports user-service.wsdl
// (non-stateful mode) and verifies all 4 operations return canned XML responses.
func TestWSDLIntegration_UserService_CannedResponses(t *testing.T) {
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{} // non-stateful
	collection, err := importer.Import(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	m := collection.Mocks[0]
	cfg := mockToSOAPConfig(m)
	require.Len(t, cfg.Operations, 4, "expected 4 operations")

	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err)

	// GetUser — should return generated response XML based on XSD
	status, body := callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["GetUser"].SOAPAction, `<GetUser><id>1</id></GetUser>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "GetUserResponse", "should contain GetUserResponse element")

	// CreateUser
	status, body = callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["CreateUser"].SOAPAction, `<CreateUser><name>Alice</name></CreateUser>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "CreateUserResponse", "should contain CreateUserResponse element")

	// ListUsers
	status, body = callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["ListUsers"].SOAPAction, `<ListUsers/>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "ListUsersResponse", "should contain ListUsersResponse element")

	// DeleteUser
	status, body = callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["DeleteUser"].SOAPAction, `<DeleteUser><id>1</id></DeleteUser>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "DeleteUserResponse", "should contain DeleteUserResponse element")
}

// TestWSDLIntegration_UserService_StatefulMode imports user-service.wsdl with
// --stateful, wires in a mock executor, and verifies operations route through
// the stateful bridge instead of returning canned responses.
func TestWSDLIntegration_UserService_StatefulMode(t *testing.T) {
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{Stateful: true}
	collection, err := importer.Import(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	m := collection.Mocks[0]
	cfg := mockToSOAPConfig(m)

	// Verify stateful config was applied
	getOp := cfg.Operations["GetUser"]
	assert.Equal(t, "users", getOp.StatefulResource)
	assert.Equal(t, "get", getOp.StatefulAction)
	assert.Empty(t, getOp.Response, "stateful ops should not have canned response")

	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err)

	// Wire up a mock executor
	executor := &testStatefulExecutor{
		result: &soap.StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":    "user-42",
				"name":  "Integration User",
				"email": "integ@example.com",
			},
		},
	}
	handler.SetStatefulExecutor(executor)

	// Call GetUser — should route through stateful executor, not canned response
	status, body := callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["GetUser"].SOAPAction, `<GetUser><id>user-42</id></GetUser>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "Envelope", "should be a SOAP envelope")
	assert.Contains(t, body, "<name>Integration User</name>", "should contain stateful user data")
	assert.Contains(t, body, "<email>integ@example.com</email>")
	assert.Contains(t, body, "<id>user-42</id>")

	// Verify executor received correct request
	require.NotNil(t, executor.lastReq)
	assert.Equal(t, "users", executor.lastReq.Resource)
	assert.Equal(t, soap.StatefulActionGet, executor.lastReq.Action)
	assert.Equal(t, "user-42", executor.lastReq.ResourceID)

	// Call ListUsers — executor returns list
	executor.result = &soap.StatefulResult{
		Success: true,
		Items: []map[string]interface{}{
			{"id": "u1", "name": "Alice"},
			{"id": "u2", "name": "Bob"},
		},
		Meta: &soap.StatefulListMeta{Total: 2, Count: 2, Offset: 0, Limit: 100},
	}

	status, body = callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["ListUsers"].SOAPAction, `<ListUsers/>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "<name>Alice</name>")
	assert.Contains(t, body, "<name>Bob</name>")
	assert.Contains(t, body, "<total>2</total>")
	assert.Equal(t, soap.StatefulActionList, executor.lastReq.Action)

	// Call CreateUser — executor returns created item
	executor.result = &soap.StatefulResult{
		Success: true,
		Item: map[string]interface{}{
			"id":    "new-user",
			"name":  "Charlie",
			"email": "charlie@example.com",
		},
	}

	status, body = callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["CreateUser"].SOAPAction, `<CreateUser><name>Charlie</name><email>charlie@example.com</email></CreateUser>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "<name>Charlie</name>")
	assert.Equal(t, soap.StatefulActionCreate, executor.lastReq.Action)
	assert.Equal(t, "Charlie", executor.lastReq.Data["name"])

	// Call DeleteUser — executor returns success
	executor.result = &soap.StatefulResult{Success: true}

	status, body = callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["DeleteUser"].SOAPAction, `<DeleteUser><id>user-42</id></DeleteUser>`)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "<success>true</success>")
	assert.Equal(t, soap.StatefulActionDelete, executor.lastReq.Action)
	assert.Equal(t, "user-42", executor.lastReq.ResourceID)
}

// TestWSDLIntegration_StatefulErrorReturnsFault verifies that when the stateful
// executor returns an error, the handler produces a proper SOAP fault.
func TestWSDLIntegration_StatefulErrorReturnsFault(t *testing.T) {
	data := readTestWSDL(t, "user-service.wsdl")
	importer := &WSDLImporter{Stateful: true}
	collection, err := importer.Import(data)
	require.NoError(t, err)

	cfg := mockToSOAPConfig(collection.Mocks[0])
	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err)

	executor := &testStatefulExecutor{
		result: &soap.StatefulResult{
			Error: &soap.SOAPFault{
				Code:    "soap:Client",
				Message: `resource "users" item "nonexistent" not found`,
			},
		},
	}
	handler.SetStatefulExecutor(executor)

	status, body := callSOAPHandler(t, handler, cfg.Path,
		cfg.Operations["GetUser"].SOAPAction, `<GetUser><id>nonexistent</id></GetUser>`)
	assert.Equal(t, http.StatusInternalServerError, status, "SOAP fault should return 500")
	assert.Contains(t, body, "<faultcode>soap:Client</faultcode>")
	assert.Contains(t, body, "not found")
}

// TestWSDLIntegration_UnknownOperation verifies that calling an operation not
// present in the imported WSDL returns a proper SOAP fault.
func TestWSDLIntegration_UnknownOperation(t *testing.T) {
	data := readTestWSDL(t, "calculator.wsdl")
	importer := &WSDLImporter{}
	collection, err := importer.Import(data)
	require.NoError(t, err)

	cfg := mockToSOAPConfig(collection.Mocks[0])
	handler, err := soap.NewHandler(cfg)
	require.NoError(t, err)

	// Call a non-existent operation
	status, body := callSOAPHandler(t, handler, cfg.Path, "", `<Multiply><a>2</a><b>3</b></Multiply>`)
	assert.Equal(t, http.StatusInternalServerError, status, "unknown op should return SOAP fault")
	assert.Contains(t, body, "Multiply", "fault should reference the operation name")
}
