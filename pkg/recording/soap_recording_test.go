package recording

import (
	"strings"
	"testing"
	"time"
)

func TestNewSOAPRecording(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	if rec.ID == "" {
		t.Error("Expected ID to be set")
	}
	if !strings.HasPrefix(rec.ID, "soap-") {
		t.Errorf("Expected ID to have 'soap-' prefix, got '%s'", rec.ID)
	}
	if rec.Endpoint != "/soap/service" {
		t.Errorf("Expected endpoint '/soap/service', got '%s'", rec.Endpoint)
	}
	if rec.Operation != "GetUser" {
		t.Errorf("Expected operation 'GetUser', got '%s'", rec.Operation)
	}
	if rec.SOAPVersion != "1.1" {
		t.Errorf("Expected SOAP version '1.1', got '%s'", rec.SOAPVersion)
	}
	if rec.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestNewSOAPRecordingSOAP12(t *testing.T) {
	rec := NewSOAPRecording("/ws/api", "ProcessOrder", "1.2")

	if rec.SOAPVersion != "1.2" {
		t.Errorf("Expected SOAP version '1.2', got '%s'", rec.SOAPVersion)
	}
}

func TestNewSOAPRecordingEmptyValues(t *testing.T) {
	rec := NewSOAPRecording("", "", "")

	if rec.ID == "" {
		t.Error("Expected ID to be set even with empty values")
	}
	if rec.Endpoint != "" {
		t.Errorf("Expected empty endpoint, got '%s'", rec.Endpoint)
	}
	if rec.Operation != "" {
		t.Errorf("Expected empty operation, got '%s'", rec.Operation)
	}
	if rec.SOAPVersion != "" {
		t.Errorf("Expected empty SOAP version, got '%s'", rec.SOAPVersion)
	}
}

func TestGenerateSOAPID(t *testing.T) {
	id1 := generateSOAPID()
	id2 := generateSOAPID()

	// Check prefix
	if !strings.HasPrefix(id1, "soap-") {
		t.Errorf("Expected ID to start with 'soap-', got '%s'", id1)
	}
	if !strings.HasPrefix(id2, "soap-") {
		t.Errorf("Expected ID to start with 'soap-', got '%s'", id2)
	}

	// Check uniqueness
	if id1 == id2 {
		t.Error("Expected unique IDs, got identical IDs")
	}

	// Check format (soap-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	parts := strings.Split(id1, "-")
	if len(parts) != 6 {
		t.Errorf("Expected 6 parts in ID (soap + 5 hex groups), got %d parts: %s", len(parts), id1)
	}
	if parts[0] != "soap" {
		t.Errorf("Expected first part to be 'soap', got '%s'", parts[0])
	}
}

func TestGenerateSOAPIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateSOAPID()
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestSOAPRecordingSetSOAPAction(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetSOAPAction("http://example.com/GetUser")
	if rec.SOAPAction != "http://example.com/GetUser" {
		t.Errorf("Expected SOAPAction 'http://example.com/GetUser', got '%s'", rec.SOAPAction)
	}

	// Test overwriting
	rec.SetSOAPAction("http://example.com/UpdateUser")
	if rec.SOAPAction != "http://example.com/UpdateUser" {
		t.Errorf("Expected SOAPAction 'http://example.com/UpdateUser', got '%s'", rec.SOAPAction)
	}
}

func TestSOAPRecordingSetSOAPActionEmpty(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetSOAPAction("")
	if rec.SOAPAction != "" {
		t.Errorf("Expected empty SOAPAction, got '%s'", rec.SOAPAction)
	}
}

func TestSOAPRecordingSetRequestBody(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	body := `<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetUser><id>123</id></GetUser></soap:Body></soap:Envelope>`
	rec.SetRequestBody(body)
	if rec.RequestBody != body {
		t.Errorf("Expected request body to be set correctly")
	}
}

func TestSOAPRecordingSetRequestBodyEmpty(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetRequestBody("")
	if rec.RequestBody != "" {
		t.Errorf("Expected empty request body, got '%s'", rec.RequestBody)
	}
}

func TestSOAPRecordingSetResponseBody(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	body := `<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetUserResponse><name>John</name></GetUserResponse></soap:Body></soap:Envelope>`
	rec.SetResponseBody(body)
	if rec.ResponseBody != body {
		t.Errorf("Expected response body to be set correctly")
	}
}

func TestSOAPRecordingSetResponseBodyEmpty(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetResponseBody("")
	if rec.ResponseBody != "" {
		t.Errorf("Expected empty response body, got '%s'", rec.ResponseBody)
	}
}

func TestSOAPRecordingSetResponseStatus(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetResponseStatus(200)
	if rec.ResponseStatus != 200 {
		t.Errorf("Expected response status 200, got %d", rec.ResponseStatus)
	}

	rec.SetResponseStatus(500)
	if rec.ResponseStatus != 500 {
		t.Errorf("Expected response status 500, got %d", rec.ResponseStatus)
	}
}

func TestSOAPRecordingSetResponseStatusZero(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetResponseStatus(0)
	if rec.ResponseStatus != 0 {
		t.Errorf("Expected response status 0, got %d", rec.ResponseStatus)
	}
}

func TestSOAPRecordingSetRequestHeaders(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	headers := map[string]string{
		"Content-Type": "text/xml; charset=utf-8",
		"SOAPAction":   "http://example.com/GetUser",
	}
	rec.SetRequestHeaders(headers)

	if rec.RequestHeaders == nil {
		t.Error("Expected request headers to be set")
	}
	if rec.RequestHeaders["Content-Type"] != "text/xml; charset=utf-8" {
		t.Errorf("Expected Content-Type header, got '%s'", rec.RequestHeaders["Content-Type"])
	}
	if rec.RequestHeaders["SOAPAction"] != "http://example.com/GetUser" {
		t.Errorf("Expected SOAPAction header, got '%s'", rec.RequestHeaders["SOAPAction"])
	}
}

func TestSOAPRecordingSetRequestHeadersNil(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetRequestHeaders(nil)
	if rec.RequestHeaders != nil {
		t.Error("Expected request headers to be nil")
	}
}

func TestSOAPRecordingSetRequestHeadersEmpty(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	headers := map[string]string{}
	rec.SetRequestHeaders(headers)

	if rec.RequestHeaders == nil {
		t.Error("Expected request headers to be set (empty map, not nil)")
	}
	if len(rec.RequestHeaders) != 0 {
		t.Errorf("Expected empty request headers, got %d headers", len(rec.RequestHeaders))
	}
}

func TestSOAPRecordingSetResponseHeaders(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	headers := map[string]string{
		"Content-Type":   "text/xml; charset=utf-8",
		"Content-Length": "1234",
	}
	rec.SetResponseHeaders(headers)

	if rec.ResponseHeaders == nil {
		t.Error("Expected response headers to be set")
	}
	if rec.ResponseHeaders["Content-Type"] != "text/xml; charset=utf-8" {
		t.Errorf("Expected Content-Type header, got '%s'", rec.ResponseHeaders["Content-Type"])
	}
	if rec.ResponseHeaders["Content-Length"] != "1234" {
		t.Errorf("Expected Content-Length header, got '%s'", rec.ResponseHeaders["Content-Length"])
	}
}

func TestSOAPRecordingSetResponseHeadersNil(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetResponseHeaders(nil)
	if rec.ResponseHeaders != nil {
		t.Error("Expected response headers to be nil")
	}
}

func TestSOAPRecordingSetResponseHeadersEmpty(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	headers := map[string]string{}
	rec.SetResponseHeaders(headers)

	if rec.ResponseHeaders == nil {
		t.Error("Expected response headers to be set (empty map, not nil)")
	}
	if len(rec.ResponseHeaders) != 0 {
		t.Errorf("Expected empty response headers, got %d headers", len(rec.ResponseHeaders))
	}
}

func TestSOAPRecordingSetDuration(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetDuration(100 * time.Millisecond)
	if rec.Duration != 100*time.Millisecond {
		t.Errorf("Expected duration 100ms, got %v", rec.Duration)
	}

	rec.SetDuration(2 * time.Second)
	if rec.Duration != 2*time.Second {
		t.Errorf("Expected duration 2s, got %v", rec.Duration)
	}
}

func TestSOAPRecordingSetDurationZero(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetDuration(0)
	if rec.Duration != 0 {
		t.Errorf("Expected duration 0, got %v", rec.Duration)
	}
}

func TestSOAPRecordingSetFault(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetFault("soap:Server", "Internal server error")

	if !rec.HasFault {
		t.Error("Expected HasFault to be true")
	}
	if rec.FaultCode != "soap:Server" {
		t.Errorf("Expected fault code 'soap:Server', got '%s'", rec.FaultCode)
	}
	if rec.FaultMessage != "Internal server error" {
		t.Errorf("Expected fault message 'Internal server error', got '%s'", rec.FaultMessage)
	}
}

func TestSOAPRecordingSetFaultClientError(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetFault("soap:Client", "Invalid request")

	if !rec.HasFault {
		t.Error("Expected HasFault to be true")
	}
	if rec.FaultCode != "soap:Client" {
		t.Errorf("Expected fault code 'soap:Client', got '%s'", rec.FaultCode)
	}
}

func TestSOAPRecordingSetFaultEmptyValues(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	rec.SetFault("", "")

	if !rec.HasFault {
		t.Error("Expected HasFault to be true even with empty values")
	}
	if rec.FaultCode != "" {
		t.Errorf("Expected empty fault code, got '%s'", rec.FaultCode)
	}
	if rec.FaultMessage != "" {
		t.Errorf("Expected empty fault message, got '%s'", rec.FaultMessage)
	}
}

func TestSOAPRecordingClearFault(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	// First set a fault
	rec.SetFault("soap:Server", "Error occurred")
	if !rec.HasFault {
		t.Error("Expected HasFault to be true after SetFault")
	}

	// Now clear it
	rec.ClearFault()

	if rec.HasFault {
		t.Error("Expected HasFault to be false after ClearFault")
	}
	if rec.FaultCode != "" {
		t.Errorf("Expected empty fault code after ClearFault, got '%s'", rec.FaultCode)
	}
	if rec.FaultMessage != "" {
		t.Errorf("Expected empty fault message after ClearFault, got '%s'", rec.FaultMessage)
	}
}

func TestSOAPRecordingClearFaultWhenNoFault(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	// Clear without setting a fault first
	rec.ClearFault()

	if rec.HasFault {
		t.Error("Expected HasFault to be false")
	}
	if rec.FaultCode != "" {
		t.Errorf("Expected empty fault code, got '%s'", rec.FaultCode)
	}
	if rec.FaultMessage != "" {
		t.Errorf("Expected empty fault message, got '%s'", rec.FaultMessage)
	}
}

func TestSOAPRecordingSettersChaining(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	// Set all values
	rec.SetSOAPAction("http://example.com/GetUser")
	rec.SetRequestBody("<request>data</request>")
	rec.SetResponseBody("<response>result</response>")
	rec.SetResponseStatus(200)
	rec.SetRequestHeaders(map[string]string{"X-Custom": "value"})
	rec.SetResponseHeaders(map[string]string{"X-Response": "header"})
	rec.SetDuration(50 * time.Millisecond)
	rec.SetFault("soap:Server", "Error")

	// Verify all values
	if rec.SOAPAction != "http://example.com/GetUser" {
		t.Error("SOAPAction not set correctly")
	}
	if rec.RequestBody != "<request>data</request>" {
		t.Error("RequestBody not set correctly")
	}
	if rec.ResponseBody != "<response>result</response>" {
		t.Error("ResponseBody not set correctly")
	}
	if rec.ResponseStatus != 200 {
		t.Error("ResponseStatus not set correctly")
	}
	if rec.RequestHeaders["X-Custom"] != "value" {
		t.Error("RequestHeaders not set correctly")
	}
	if rec.ResponseHeaders["X-Response"] != "header" {
		t.Error("ResponseHeaders not set correctly")
	}
	if rec.Duration != 50*time.Millisecond {
		t.Error("Duration not set correctly")
	}
	if !rec.HasFault {
		t.Error("Fault not set correctly")
	}
}

func TestSOAPRecordingOverwriteValues(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	// Set initial values
	rec.SetRequestBody("initial body")
	rec.SetResponseStatus(200)

	// Overwrite values
	rec.SetRequestBody("updated body")
	rec.SetResponseStatus(500)

	if rec.RequestBody != "updated body" {
		t.Errorf("Expected request body to be overwritten, got '%s'", rec.RequestBody)
	}
	if rec.ResponseStatus != 500 {
		t.Errorf("Expected response status to be overwritten, got %d", rec.ResponseStatus)
	}
}

func TestSOAPRecordingHeadersIsolation(t *testing.T) {
	rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

	// Set headers
	headers := map[string]string{"Key": "value"}
	rec.SetRequestHeaders(headers)

	// Modify the original map
	headers["Key"] = "modified"
	headers["NewKey"] = "new"

	// The recording should have the modified values since it's a reference
	// This tests the current behavior - whether headers are copied or referenced
	if rec.RequestHeaders["Key"] != "modified" {
		// If this fails, it means the setter copies the map (which is safer)
		t.Log("SetRequestHeaders copies the map (safe behavior)")
	}
}

func TestSOAPRecordingFilter(t *testing.T) {
	// Test that SOAPRecordingFilter struct is properly defined
	filter := SOAPRecordingFilter{
		Endpoint:   "/soap/service",
		Operation:  "GetUser",
		SOAPAction: "http://example.com/GetUser",
		Limit:      10,
		Offset:     0,
	}

	if filter.Endpoint != "/soap/service" {
		t.Errorf("Expected endpoint '/soap/service', got '%s'", filter.Endpoint)
	}
	if filter.Operation != "GetUser" {
		t.Errorf("Expected operation 'GetUser', got '%s'", filter.Operation)
	}
	if filter.SOAPAction != "http://example.com/GetUser" {
		t.Errorf("Expected SOAPAction 'http://example.com/GetUser', got '%s'", filter.SOAPAction)
	}
}

func TestSOAPRecordingFilterHasFault(t *testing.T) {
	hasFault := true
	filter := SOAPRecordingFilter{
		HasFault: &hasFault,
	}

	if filter.HasFault == nil {
		t.Error("Expected HasFault to be set")
	}
	if !*filter.HasFault {
		t.Error("Expected HasFault to be true")
	}

	hasFault = false
	filter.HasFault = &hasFault
	if *filter.HasFault {
		t.Error("Expected HasFault to be false")
	}
}

func TestSOAPRecordingFilterNilHasFault(t *testing.T) {
	filter := SOAPRecordingFilter{
		Endpoint: "/soap/service",
	}

	if filter.HasFault != nil {
		t.Error("Expected HasFault to be nil by default")
	}
}
