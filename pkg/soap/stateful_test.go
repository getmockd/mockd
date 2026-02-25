package soap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

// mockStatefulExecutor is a test implementation of StatefulExecutor.
type mockStatefulExecutor struct {
	lastRequest *StatefulRequest
	result      *StatefulResult
}

func (m *mockStatefulExecutor) ExecuteStateful(_ context.Context, req *StatefulRequest) *StatefulResult {
	m.lastRequest = req
	if m.result != nil {
		return m.result
	}
	return &StatefulResult{
		Success: true,
		Item: map[string]interface{}{
			"id":   "123",
			"name": "Test User",
		},
	}
}

// Helper to create a SOAP 1.1 envelope
func soapEnvelope11(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>` + body + `</soap:Body>
</soap:Envelope>`
}

func TestStateful_GetOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":    "user-1",
				"name":  "Alice",
				"email": "alice@example.com",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"GetUser": {
				StatefulResource: "users",
				StatefulAction:   "get",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<GetUser><id>user-1</id></GetUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// Verify the response contains the user data as XML
	if !strings.Contains(respStr, "<name>Alice</name>") {
		t.Errorf("expected response to contain <name>Alice</name>, got: %s", respStr)
	}
	if !strings.Contains(respStr, "<email>alice@example.com</email>") {
		t.Errorf("expected response to contain email, got: %s", respStr)
	}
	if !strings.Contains(respStr, "<id>user-1</id>") {
		t.Errorf("expected response to contain id, got: %s", respStr)
	}
	if !strings.Contains(respStr, "Envelope") {
		t.Errorf("expected response to be a SOAP envelope, got: %s", respStr)
	}

	// Verify the executor received the correct request
	if executor.lastRequest == nil {
		t.Fatal("expected executor to receive a request")
	}
	if executor.lastRequest.Resource != "users" {
		t.Errorf("expected resource 'users', got %q", executor.lastRequest.Resource)
	}
	if executor.lastRequest.Action != StatefulActionGet {
		t.Errorf("expected action 'get', got %q", executor.lastRequest.Action)
	}
	if executor.lastRequest.ResourceID != "user-1" {
		t.Errorf("expected resourceID 'user-1', got %q", executor.lastRequest.ResourceID)
	}
}

func TestStateful_ListOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Items: []map[string]interface{}{
				{"id": "u1", "name": "Alice"},
				{"id": "u2", "name": "Bob"},
			},
			Meta: &StatefulListMeta{
				Total:  2,
				Count:  2,
				Offset: 0,
				Limit:  100,
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"ListUsers": {
				StatefulResource: "users",
				StatefulAction:   "list",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<ListUsers/>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	if !strings.Contains(respStr, "<name>Alice</name>") {
		t.Errorf("expected response to contain Alice, got: %s", respStr)
	}
	if !strings.Contains(respStr, "<name>Bob</name>") {
		t.Errorf("expected response to contain Bob, got: %s", respStr)
	}
	if !strings.Contains(respStr, "<total>2</total>") {
		t.Errorf("expected response to contain total=2, got: %s", respStr)
	}
	if !strings.Contains(respStr, "<usersResponse>") {
		t.Errorf("expected response wrapped in <usersResponse>, got: %s", respStr)
	}

	// Verify filter was set
	if executor.lastRequest.Action != StatefulActionList {
		t.Errorf("expected action 'list', got %q", executor.lastRequest.Action)
	}
}

func TestStateful_CreateOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":    "new-id",
				"name":  "Charlie",
				"email": "charlie@example.com",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"CreateUser": {
				StatefulResource: "users",
				StatefulAction:   "create",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<CreateUser><name>Charlie</name><email>charlie@example.com</email></CreateUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Verify the executor received the data
	if executor.lastRequest.Action != StatefulActionCreate {
		t.Errorf("expected action 'create', got %q", executor.lastRequest.Action)
	}
	if executor.lastRequest.Data == nil {
		t.Fatal("expected data to be set")
	}
	if executor.lastRequest.Data["name"] != "Charlie" {
		t.Errorf("expected name 'Charlie', got %v", executor.lastRequest.Data["name"])
	}
}

func TestStateful_UpdateOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":   "user-1",
				"name": "Alice Updated",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"UpdateUser": {
				StatefulResource: "users",
				StatefulAction:   "update",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<UpdateUser><id>user-1</id><name>Alice Updated</name></UpdateUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	if executor.lastRequest.Action != StatefulActionUpdate {
		t.Errorf("expected action 'update', got %q", executor.lastRequest.Action)
	}
	if executor.lastRequest.ResourceID != "user-1" {
		t.Errorf("expected resourceID 'user-1', got %q", executor.lastRequest.ResourceID)
	}
	// ID should be removed from data for updates
	if _, hasID := executor.lastRequest.Data["id"]; hasID {
		t.Error("expected 'id' to be removed from data for update operations")
	}
	if executor.lastRequest.Data["name"] != "Alice Updated" {
		t.Errorf("expected name 'Alice Updated', got %v", executor.lastRequest.Data["name"])
	}
}

func TestStateful_DeleteOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"DeleteUser": {
				StatefulResource: "users",
				StatefulAction:   "delete",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<DeleteUser><id>user-1</id></DeleteUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	if !strings.Contains(respStr, "<success>true</success>") {
		t.Errorf("expected success element, got: %s", respStr)
	}
	if executor.lastRequest.Action != StatefulActionDelete {
		t.Errorf("expected action 'delete', got %q", executor.lastRequest.Action)
	}
	if executor.lastRequest.ResourceID != "user-1" {
		t.Errorf("expected resourceID 'user-1', got %q", executor.lastRequest.ResourceID)
	}
}

func TestStateful_NotFound_Returns_SOAP11_Fault(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Error: &SOAPFault{
				Code:    "soap:Client",
				Message: `resource "users" item "nonexistent" not found`,
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"GetUser": {
				StatefulResource: "users",
				StatefulAction:   "get",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<GetUser><id>nonexistent</id></GetUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 (SOAP fault), got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	if !strings.Contains(respStr, "<faultcode>soap:Client</faultcode>") {
		t.Errorf("expected soap:Client faultcode, got: %s", respStr)
	}
	if !strings.Contains(respStr, "not found") {
		t.Errorf("expected 'not found' in fault string, got: %s", respStr)
	}
}

func TestStateful_Conflict_Returns_SOAP12_Fault(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Error: &SOAPFault{
				Code:    "soap:Client",
				Message: `resource "users" item "dup-id" already exists`,
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"CreateUser": {
				StatefulResource: "users",
				StatefulAction:   "create",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	// SOAP 1.2 envelope
	body := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body><CreateUser><id>dup-id</id><name>Test</name></CreateUser></soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 (SOAP fault), got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// SOAP 1.2 maps soap:Client → soap:Sender
	if !strings.Contains(respStr, "soap:Sender") {
		t.Errorf("expected soap:Sender fault code (SOAP 1.2), got: %s", respStr)
	}
	if !strings.Contains(respStr, "already exists") {
		t.Errorf("expected 'already exists' in fault reason, got: %s", respStr)
	}
}

func TestStateful_MixedMode_CannedAndStateful(t *testing.T) {
	// Tests SS-13: Some SOAP ops are stateful, others are canned (coexistence)
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":   "user-1",
				"name": "Alice",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/service",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"GetUser": {
				StatefulResource: "users",
				StatefulAction:   "get",
			},
			"GetVersion": {
				Response: "<Version>1.0.0</Version>",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	// Test stateful operation
	body1 := soapEnvelope11(`<GetUser><id>user-1</id></GetUser>`)
	req1 := httptest.NewRequest("POST", "/soap/service", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	resp1 := w1.Result()
	defer func() { _ = resp1.Body.Close() }()
	body1Resp, _ := io.ReadAll(resp1.Body)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("stateful op: expected 200, got %d: %s", resp1.StatusCode, string(body1Resp))
	}
	if !strings.Contains(string(body1Resp), "<name>Alice</name>") {
		t.Errorf("stateful op: expected Alice, got: %s", string(body1Resp))
	}

	// Test canned operation
	body2 := soapEnvelope11(`<GetVersion/>`)
	req2 := httptest.NewRequest("POST", "/soap/service", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	resp2 := w2.Result()
	defer func() { _ = resp2.Body.Close() }()
	body2Resp, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("canned op: expected 200, got %d: %s", resp2.StatusCode, string(body2Resp))
	}
	if !strings.Contains(string(body2Resp), "<Version>1.0.0</Version>") {
		t.Errorf("canned op: expected Version 1.0.0, got: %s", string(body2Resp))
	}
}

func TestStateful_NoExecutor_FallsThrough(t *testing.T) {
	// When StatefulResource is set but no executor is configured,
	// it should fall through to canned response
	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"GetUser": {
				StatefulResource: "users",
				StatefulAction:   "get",
				Response:         "<User><id>fallback</id></User>",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	// Note: no executor set

	body := soapEnvelope11(`<GetUser><id>1</id></GetUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBody), "<id>fallback</id>") {
		t.Errorf("expected fallback canned response, got: %s", string(respBody))
	}
}

func TestStateful_WithSOAPAction(t *testing.T) {
	// Stateful operations should also work with SOAPAction-based routing
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":   "order-1",
				"item": "Widget",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/orders",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"GetOrder": {
				SOAPAction:       "urn:GetOrder",
				StatefulResource: "orders",
				StatefulAction:   "get",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<GetOrder><id>order-1</id></GetOrder>`)
	req := httptest.NewRequest("POST", "/soap/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"urn:GetOrder"`)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBody), "<item>Widget</item>") {
		t.Errorf("expected Widget in response, got: %s", string(respBody))
	}
}

func TestStateful_PatchOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"id":    "user-1",
				"name":  "Alice",
				"email": "newemail@example.com",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap",
		Path:    "/soap/users",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"PatchUser": {
				StatefulResource: "users",
				StatefulAction:   "patch",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<PatchUser><id>user-1</id><email>newemail@example.com</email></PatchUser>`)
	req := httptest.NewRequest("POST", "/soap/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	if executor.lastRequest.Action != StatefulActionPatch {
		t.Errorf("expected action 'patch', got %q", executor.lastRequest.Action)
	}
	if executor.lastRequest.ResourceID != "user-1" {
		t.Errorf("expected resourceID 'user-1', got %q", executor.lastRequest.ResourceID)
	}
}

// Test XML-to-map conversion

func TestXMLElementToMap_SimpleFields(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<CreateUser><name>Alice</name><email>alice@example.com</email></CreateUser>`); err != nil {
		t.Fatal(err)
	}
	root := doc.Root()

	result := xmlElementToMap(root)

	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", result["name"])
	}
	if result["email"] != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %v", result["email"])
	}
}

func TestXMLElementToMap_NestedElements(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<CreateUser><name>Alice</name><address><city>NYC</city><zip>10001</zip></address></CreateUser>`); err != nil {
		t.Fatal(err)
	}
	root := doc.Root()

	result := xmlElementToMap(root)

	addr, ok := result["address"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected address to be a map, got %T", result["address"])
	}
	if addr["city"] != "NYC" {
		t.Errorf("expected city=NYC, got %v", addr["city"])
	}
}

func TestXMLElementToMap_RepeatedElements(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<ListItems><tag>go</tag><tag>mockd</tag><tag>soap</tag></ListItems>`); err != nil {
		t.Fatal(err)
	}
	root := doc.Root()

	result := xmlElementToMap(root)

	tags, ok := result["tag"].([]interface{})
	if !ok {
		t.Fatalf("expected tag to be a slice, got %T", result["tag"])
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}

func TestXMLElementToMap_EmptyElement(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<Empty/>`); err != nil {
		t.Fatal(err)
	}

	result := xmlElementToMap(doc.Root())
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestMapToXML_Simple(t *testing.T) {
	data := map[string]interface{}{
		"id":   "123",
		"name": "Alice",
	}
	xml := string(mapToXML(data, "users"))

	if !strings.Contains(xml, "<userResponse>") {
		t.Errorf("expected <userResponse> wrapper, got: %s", xml)
	}
	if !strings.Contains(xml, "<id>123</id>") {
		t.Errorf("expected <id>123</id>, got: %s", xml)
	}
	if !strings.Contains(xml, "<name>Alice</name>") {
		t.Errorf("expected <name>Alice</name>, got: %s", xml)
	}
}

func TestMapToXML_SpecialChars(t *testing.T) {
	data := map[string]interface{}{
		"name": "A & B <Corp>",
	}
	xml := string(mapToXML(data, "companies"))

	if !strings.Contains(xml, "&amp;") {
		t.Errorf("expected escaped ampersand, got: %s", xml)
	}
	if !strings.Contains(xml, "&lt;Corp&gt;") {
		t.Errorf("expected escaped angle brackets, got: %s", xml)
	}
}

func TestMapToXML_NestedMap(t *testing.T) {
	data := map[string]interface{}{
		"id": "1",
		"address": map[string]interface{}{
			"city": "NYC",
			"zip":  "10001",
		},
	}
	xml := string(mapToXML(data, "users"))

	if !strings.Contains(xml, "<address>") {
		t.Errorf("expected nested <address>, got: %s", xml)
	}
	if !strings.Contains(xml, "<city>NYC</city>") {
		t.Errorf("expected <city>NYC</city>, got: %s", xml)
	}
}

func TestListResultToXML(t *testing.T) {
	result := &StatefulResult{
		Items: []map[string]interface{}{
			{"id": "u1", "name": "Alice"},
			{"id": "u2", "name": "Bob"},
		},
		Meta: &StatefulListMeta{
			Total:  2,
			Count:  2,
			Offset: 0,
			Limit:  100,
		},
	}
	xml := string(listResultToXML(result, "users"))

	if !strings.Contains(xml, "<usersResponse>") {
		t.Errorf("expected <usersResponse> wrapper, got: %s", xml)
	}
	if !strings.Contains(xml, "<user>") {
		t.Errorf("expected <user> elements, got: %s", xml)
	}
	if !strings.Contains(xml, "<total>2</total>") {
		t.Errorf("expected <total>2</total>, got: %s", xml)
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"orders", "order"},
		{"addresses", "address"},
		{"categories", "category"},
		{"class", "class"},   // "ss" ending should not be stripped
		{"data", "data"},     // doesn't end in 's'
		{"person", "person"}, // doesn't end in 's'
	}
	for _, tt := range tests {
		got := singularize(tt.input)
		if got != tt.expected {
			t.Errorf("singularize(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBuildStatefulRequest_GetAction(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(soapEnvelope11(`<GetUser><id>user-1</id></GetUser>`)); err != nil {
		t.Fatal(err)
	}

	opConfig := &OperationConfig{
		StatefulResource: "users",
		StatefulAction:   "get",
	}

	req := buildStatefulRequest("GetUser", opConfig, doc)

	if req.Resource != "users" {
		t.Errorf("expected resource 'users', got %q", req.Resource)
	}
	if req.Action != StatefulActionGet {
		t.Errorf("expected action 'get', got %q", req.Action)
	}
	if req.OperationName != "GetUser" {
		t.Errorf("expected operationName 'GetUser', got %q", req.OperationName)
	}
	if req.ResourceID != "user-1" {
		t.Errorf("expected resourceID 'user-1', got %q", req.ResourceID)
	}
	if req.Data != nil {
		t.Errorf("expected nil data for get, got %v", req.Data)
	}
}

func TestBuildStatefulRequest_CreateAction(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(soapEnvelope11(`<CreateUser><name>Alice</name><email>a@b.com</email></CreateUser>`)); err != nil {
		t.Fatal(err)
	}

	opConfig := &OperationConfig{
		StatefulResource: "users",
		StatefulAction:   "create",
	}

	req := buildStatefulRequest("CreateUser", opConfig, doc)

	if req.Action != StatefulActionCreate {
		t.Errorf("expected action 'create', got %q", req.Action)
	}
	if req.Data["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got %v", req.Data["name"])
	}
	if req.Data["email"] != "a@b.com" {
		t.Errorf("expected email 'a@b.com', got %v", req.Data["email"])
	}
}

func TestBuildStatefulRequest_ListAction(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(soapEnvelope11(`<ListUsers><limit>10</limit><offset>5</offset></ListUsers>`)); err != nil {
		t.Fatal(err)
	}

	opConfig := &OperationConfig{
		StatefulResource: "users",
		StatefulAction:   "list",
	}

	req := buildStatefulRequest("ListUsers", opConfig, doc)

	if req.Action != StatefulActionList {
		t.Errorf("expected action 'list', got %q", req.Action)
	}
	if req.Filter == nil {
		t.Fatal("expected filter to be set")
	}
	if req.Filter.Limit != 10 {
		t.Errorf("expected limit=10, got %d", req.Filter.Limit)
	}
	if req.Filter.Offset != 5 {
		t.Errorf("expected offset=5, got %d", req.Filter.Offset)
	}
	if req.Data != nil {
		t.Errorf("expected nil data for list, got %v", req.Data)
	}
}

// --- CO-14: SOAP custom operation integration test ---
// This test verifies that a SOAP operation configured with StatefulAction="custom"
// correctly extracts the XML body as data and passes it through to the executor,
// and that the result is rendered back as XML.

func TestStateful_CustomOperation(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: true,
			Item: map[string]interface{}{
				"sourceBalance": float64(700),
				"destBalance":   float64(800),
				"transferId":    "txn-abc-123",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap-custom",
		Path:    "/soap/banking",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"TransferFunds": {
				StatefulResource: "TransferFunds", // operation name for custom
				StatefulAction:   "custom",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<TransferFunds>
		<sourceId>acc-1</sourceId>
		<destId>acc-2</destId>
		<amount>300</amount>
	</TransferFunds>`)
	req := httptest.NewRequest("POST", "/soap/banking", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// Verify the executor received the correct action and data
	if executor.lastRequest == nil {
		t.Fatal("executor was not called")
	}
	if executor.lastRequest.Action != StatefulActionCustom {
		t.Errorf("expected action 'custom', got %q", executor.lastRequest.Action)
	}
	if executor.lastRequest.Resource != "TransferFunds" {
		t.Errorf("expected resource 'TransferFunds', got %q", executor.lastRequest.Resource)
	}
	// Verify input data was extracted from XML
	if executor.lastRequest.Data == nil {
		t.Fatal("expected data to be non-nil")
	}
	if sourceID, ok := executor.lastRequest.Data["sourceId"]; !ok || sourceID != "acc-1" {
		t.Errorf("expected sourceId=acc-1, got %v", executor.lastRequest.Data["sourceId"])
	}
	if destID, ok := executor.lastRequest.Data["destId"]; !ok || destID != "acc-2" {
		t.Errorf("expected destId=acc-2, got %v", executor.lastRequest.Data["destId"])
	}
	if amount, ok := executor.lastRequest.Data["amount"]; !ok || amount != "300" {
		t.Errorf("expected amount=300, got %v", executor.lastRequest.Data["amount"])
	}

	// Verify response contains the result data as XML
	if !strings.Contains(respStr, "sourceBalance") {
		t.Errorf("response should contain sourceBalance: %s", respStr)
	}
	if !strings.Contains(respStr, "700") {
		t.Errorf("response should contain 700: %s", respStr)
	}
	if !strings.Contains(respStr, "destBalance") {
		t.Errorf("response should contain destBalance: %s", respStr)
	}
	if !strings.Contains(respStr, "txn-abc-123") {
		t.Errorf("response should contain transfer ID: %s", respStr)
	}
}

func TestStateful_CustomOperation_Fault(t *testing.T) {
	executor := &mockStatefulExecutor{
		result: &StatefulResult{
			Success: false,
			Error: &SOAPFault{
				Code:    "soap:Client",
				Message: "custom operation 'TransferFunds' not registered",
			},
		},
	}

	handler, err := NewHandler(&SOAPConfig{
		ID:      "test-soap-custom-fault",
		Path:    "/soap/banking",
		Enabled: true,
		Operations: map[string]OperationConfig{
			"TransferFunds": {
				StatefulResource: "TransferFunds",
				StatefulAction:   "custom",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetStatefulExecutor(executor)

	body := soapEnvelope11(`<TransferFunds>
		<sourceId>acc-1</sourceId>
		<destId>acc-2</destId>
		<amount>300</amount>
	</TransferFunds>`)
	req := httptest.NewRequest("POST", "/soap/banking", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// Should return a SOAP fault
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for fault, got %d", resp.StatusCode)
	}
	if !strings.Contains(respStr, "soap:Client") {
		t.Errorf("response should contain fault code: %s", respStr)
	}
	if !strings.Contains(respStr, "not registered") {
		t.Errorf("response should contain fault message: %s", respStr)
	}
}
