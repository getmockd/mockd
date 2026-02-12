package recording

import (
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/soap"
)

func TestDefaultSOAPConvertOptions(t *testing.T) {
	opts := DefaultSOAPConvertOptions()
	if !opts.Deduplicate {
		t.Error("Expected Deduplicate=true")
	}
	if opts.IncludeXPathMatch {
		t.Error("Expected IncludeXPathMatch=false")
	}
	if opts.IncludeDelay {
		t.Error("Expected IncludeDelay=false")
	}
	if !opts.PreserveFaults {
		t.Error("Expected PreserveFaults=true")
	}
}

func TestToOperationConfig_Nil(t *testing.T) {
	opts := DefaultSOAPConvertOptions()
	result := ToOperationConfig(nil, opts)
	if result != nil {
		t.Error("Expected nil for nil recording")
	}
}

func TestToOperationConfig_BasicFields(t *testing.T) {
	rec := &SOAPRecording{
		SOAPAction:   "GetUser",
		ResponseBody: "<GetUserResponse><name>Alice</name></GetUserResponse>",
	}
	opts := DefaultSOAPConvertOptions()
	result := ToOperationConfig(rec, opts)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.SOAPAction != "GetUser" {
		t.Errorf("Expected SOAPAction 'GetUser', got %q", result.SOAPAction)
	}
	if result.Response != rec.ResponseBody {
		t.Errorf("Expected response body to match, got %q", result.Response)
	}
	if result.Delay != "" {
		t.Error("Expected no delay when IncludeDelay=false")
	}
	if result.Fault != nil {
		t.Error("Expected no fault for non-fault recording")
	}
}

func TestToOperationConfig_IncludeDelay(t *testing.T) {
	rec := &SOAPRecording{
		SOAPAction:   "Slow",
		ResponseBody: "<Response/>",
		Duration:     250 * time.Millisecond,
	}
	opts := SOAPConvertOptions{IncludeDelay: true}
	result := ToOperationConfig(rec, opts)

	if result.Delay == "" {
		t.Error("Expected delay when IncludeDelay=true and Duration>0")
	}
	if result.Delay != "250ms" {
		t.Errorf("Expected delay '250ms', got %q", result.Delay)
	}
}

func TestToOperationConfig_NoDelayWhenZero(t *testing.T) {
	rec := &SOAPRecording{
		SOAPAction:   "Fast",
		ResponseBody: "<Response/>",
		Duration:     0,
	}
	opts := SOAPConvertOptions{IncludeDelay: true}
	result := ToOperationConfig(rec, opts)

	if result.Delay != "" {
		t.Errorf("Expected no delay when Duration=0, got %q", result.Delay)
	}
}

func TestToOperationConfig_PreserveFaults(t *testing.T) {
	rec := &SOAPRecording{
		SOAPAction:   "BadOp",
		ResponseBody: "<Fault/>",
		HasFault:     true,
		FaultCode:    "Server",
		FaultMessage: "Internal failure",
	}
	opts := SOAPConvertOptions{PreserveFaults: true}
	result := ToOperationConfig(rec, opts)

	if result.Fault == nil {
		t.Fatal("Expected fault to be preserved")
	}
	if result.Fault.Code != "Server" {
		t.Errorf("Expected fault code 'Server', got %q", result.Fault.Code)
	}
	if result.Fault.Message != "Internal failure" {
		t.Errorf("Expected fault message 'Internal failure', got %q", result.Fault.Message)
	}
}

func TestToOperationConfig_NoFaultWhenDisabled(t *testing.T) {
	rec := &SOAPRecording{
		ResponseBody: "<Fault/>",
		HasFault:     true,
		FaultCode:    "Server",
		FaultMessage: "fail",
	}
	opts := SOAPConvertOptions{PreserveFaults: false}
	result := ToOperationConfig(rec, opts)

	if result.Fault != nil {
		t.Error("Expected fault to be omitted when PreserveFaults=false")
	}
}

func TestToOperationConfig_XPathMatch(t *testing.T) {
	rec := &SOAPRecording{
		ResponseBody: "<Response/>",
		RequestBody: `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
			<soap:Body>
				<GetUser><id>123</id><name>Alice</name></GetUser>
			</soap:Body>
		</soap:Envelope>`,
	}
	opts := SOAPConvertOptions{IncludeXPathMatch: true}
	result := ToOperationConfig(rec, opts)

	if result.Match == nil {
		t.Fatal("Expected XPath match to be extracted")
	}
	if result.Match.XPath == nil {
		t.Fatal("Expected XPath map to be non-nil")
	}
	if v, ok := result.Match.XPath["//id"]; !ok || v != "123" {
		t.Errorf("Expected //id=123, got %q", v)
	}
	if v, ok := result.Match.XPath["//name"]; !ok || v != "Alice" {
		t.Errorf("Expected //name=Alice, got %q", v)
	}
}

func TestToOperationConfig_NoXPathWhenDisabled(t *testing.T) {
	rec := &SOAPRecording{
		ResponseBody: "<Response/>",
		RequestBody:  `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Op><id>1</id></Op></soap:Body></soap:Envelope>`,
	}
	opts := SOAPConvertOptions{IncludeXPathMatch: false}
	result := ToOperationConfig(rec, opts)

	if result.Match != nil {
		t.Error("Expected no match when IncludeXPathMatch=false")
	}
}

// --- extractXPathMatch tests ---

func TestExtractXPathMatch_ValidEnvelope(t *testing.T) {
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
		<soap:Body>
			<GetUser><userId>42</userId></GetUser>
		</soap:Body>
	</soap:Envelope>`

	match := extractXPathMatch(body)
	if match == nil {
		t.Fatal("Expected non-nil match")
	}
	if v, ok := match.XPath["//userId"]; !ok || v != "42" {
		t.Errorf("Expected //userId=42, got %v", match.XPath)
	}
}

func TestExtractXPathMatch_InvalidXML(t *testing.T) {
	match := extractXPathMatch("not xml at all")
	if match != nil {
		t.Error("Expected nil for invalid XML")
	}
}

func TestExtractXPathMatch_EmptyBody(t *testing.T) {
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
		<soap:Body></soap:Body>
	</soap:Envelope>`

	match := extractXPathMatch(body)
	if match != nil {
		t.Error("Expected nil for empty body")
	}
}

func TestExtractXPathMatch_NoSimpleElements(t *testing.T) {
	// Body with nested elements but no simple text content
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
		<soap:Body>
			<Op><nested><deep>value</deep></nested></Op>
		</soap:Body>
	</soap:Envelope>`

	match := extractXPathMatch(body)
	// The regex should still find <deep>value</deep>
	if match == nil {
		t.Fatal("Expected match for nested simple element")
	}
	if _, ok := match.XPath["//deep"]; !ok {
		t.Errorf("Expected //deep to be found, got %v", match.XPath)
	}
}

// --- ToSOAPConfig tests ---

func TestToSOAPConfig_EmptySlice(t *testing.T) {
	result := ToSOAPConfig(nil, DefaultSOAPConvertOptions())
	if result != nil {
		t.Error("Expected nil for empty recordings")
	}

	result = ToSOAPConfig([]*SOAPRecording{}, DefaultSOAPConvertOptions())
	if result != nil {
		t.Error("Expected nil for empty slice")
	}
}

func TestToSOAPConfig_SingleRecording(t *testing.T) {
	recs := []*SOAPRecording{
		{
			Endpoint:     "/soap/users",
			Operation:    "GetUser",
			SOAPAction:   "GetUser",
			ResponseBody: "<GetUserResponse><name>Bob</name></GetUserResponse>",
		},
	}
	result := ToSOAPConfig(recs, DefaultSOAPConvertOptions())

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Path != "/soap/users" {
		t.Errorf("Expected path '/soap/users', got %q", result.Path)
	}
	if !result.Enabled {
		t.Error("Expected Enabled=true")
	}
	if len(result.Operations) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(result.Operations))
	}
	op, ok := result.Operations["GetUser"]
	if !ok {
		t.Fatal("Expected 'GetUser' operation")
	}
	if op.SOAPAction != "GetUser" {
		t.Errorf("Expected SOAPAction 'GetUser', got %q", op.SOAPAction)
	}
}

func TestToSOAPConfig_DeduplicateFirstWins(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "first"},
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "second"},
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "third"},
	}
	result := ToSOAPConfig(recs, SOAPConvertOptions{Deduplicate: true})

	op := result.Operations["Op"]
	if op.Response != "first" {
		t.Errorf("Expected first recording with Deduplicate=true, got %q", op.Response)
	}
}

func TestToSOAPConfig_NoDeduplicate_LastWins(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "first"},
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "second"},
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "third"},
	}
	result := ToSOAPConfig(recs, SOAPConvertOptions{Deduplicate: false})

	op := result.Operations["Op"]
	if op.Response != "third" {
		t.Errorf("Expected last recording with Deduplicate=false, got %q", op.Response)
	}
}

func TestToSOAPConfig_MultipleOperations(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "GetUser", ResponseBody: "user"},
		{Endpoint: "/soap", Operation: "GetOrder", ResponseBody: "order"},
		{Endpoint: "/soap", Operation: "GetProduct", ResponseBody: "product"},
	}
	result := ToSOAPConfig(recs, DefaultSOAPConvertOptions())

	if len(result.Operations) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(result.Operations))
	}
	for _, name := range []string{"GetUser", "GetOrder", "GetProduct"} {
		if _, ok := result.Operations[name]; !ok {
			t.Errorf("Expected operation %q to exist", name)
		}
	}
}

// --- ConvertSOAPRecordings tests ---

func TestConvertSOAPRecordings_Empty(t *testing.T) {
	result := ConvertSOAPRecordings(nil, DefaultSOAPConvertOptions())

	if result == nil {
		t.Fatal("Expected non-nil result even for empty input")
	}
	if result.Total != 0 {
		t.Errorf("Expected Total=0, got %d", result.Total)
	}
	if result.OperationCount != 0 {
		t.Errorf("Expected OperationCount=0, got %d", result.OperationCount)
	}
	if result.Config != nil {
		t.Error("Expected nil Config for empty input")
	}
}

func TestConvertSOAPRecordings_WithStats(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op1", ResponseBody: "r1"},
		{Endpoint: "/soap", Operation: "Op2", ResponseBody: "r2"},
		{Endpoint: "/soap", Operation: "Op1", ResponseBody: "r1-dup"},
	}
	result := ConvertSOAPRecordings(recs, DefaultSOAPConvertOptions())

	if result.Total != 3 {
		t.Errorf("Expected Total=3, got %d", result.Total)
	}
	if result.OperationCount != 2 {
		t.Errorf("Expected OperationCount=2, got %d", result.OperationCount)
	}
}

func TestConvertSOAPRecordings_FaultWarning(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "r", HasFault: true, FaultCode: "Server"},
	}
	// PreserveFaults=false should produce a warning
	opts := SOAPConvertOptions{PreserveFaults: false}
	result := ConvertSOAPRecordings(recs, opts)

	foundWarning := false
	for _, w := range result.Warnings {
		if w == "Some recordings contained SOAP faults which were not preserved" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected fault warning when PreserveFaults=false and faults exist")
	}
}

func TestConvertSOAPRecordings_NoFaultWarningWhenPreserved(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "r", HasFault: true, FaultCode: "Server"},
	}
	opts := SOAPConvertOptions{PreserveFaults: true}
	result := ConvertSOAPRecordings(recs, opts)

	for _, w := range result.Warnings {
		if w == "Some recordings contained SOAP faults which were not preserved" {
			t.Error("Should not warn about faults when PreserveFaults=true")
		}
	}
}

func TestConvertSOAPRecordings_MultiEndpointWarning(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap/users", Operation: "GetUser", ResponseBody: "r1"},
		{Endpoint: "/soap/orders", Operation: "GetOrder", ResponseBody: "r2"},
	}
	result := ConvertSOAPRecordings(recs, DefaultSOAPConvertOptions())

	foundWarning := false
	for _, w := range result.Warnings {
		if w == "Multiple endpoints detected; only the first endpoint path was used" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected multi-endpoint warning")
	}
}

// --- MergeSOAPConfigs tests ---

func TestMergeSOAPConfigs_NilBase(t *testing.T) {
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "new"},
	}
	result := MergeSOAPConfigs(nil, recs, DefaultSOAPConvertOptions())

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if _, ok := result.Operations["Op"]; !ok {
		t.Error("Expected 'Op' operation")
	}
}

func TestMergeSOAPConfigs_EmptyRecordings(t *testing.T) {
	base := &soap.SOAPConfig{
		Path:       "/soap",
		Operations: map[string]soap.OperationConfig{"Existing": {Response: "old"}},
	}
	result := MergeSOAPConfigs(base, nil, DefaultSOAPConvertOptions())

	if result != base {
		t.Error("Expected base to be returned unchanged")
	}
}

func TestMergeSOAPConfigs_AddsNewOperations(t *testing.T) {
	base := &soap.SOAPConfig{
		Path:       "/soap",
		Operations: map[string]soap.OperationConfig{"Existing": {Response: "old"}},
	}
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "New", ResponseBody: "new-response"},
	}
	result := MergeSOAPConfigs(base, recs, DefaultSOAPConvertOptions())

	if len(result.Operations) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(result.Operations))
	}
	if _, ok := result.Operations["Existing"]; !ok {
		t.Error("Expected 'Existing' to remain")
	}
	if _, ok := result.Operations["New"]; !ok {
		t.Error("Expected 'New' to be added")
	}
}

func TestMergeSOAPConfigs_OverwritesExisting(t *testing.T) {
	base := &soap.SOAPConfig{
		Path:       "/soap",
		Operations: map[string]soap.OperationConfig{"Op": {Response: "old"}},
	}
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "updated"},
	}
	result := MergeSOAPConfigs(base, recs, DefaultSOAPConvertOptions())

	op := result.Operations["Op"]
	if op.Response != "updated" {
		t.Errorf("Expected overwritten response 'updated', got %q", op.Response)
	}
}

func TestMergeSOAPConfigs_NilBaseOperations(t *testing.T) {
	base := &soap.SOAPConfig{
		Path: "/soap",
		// Operations is nil
	}
	recs := []*SOAPRecording{
		{Endpoint: "/soap", Operation: "Op", ResponseBody: "response"},
	}
	result := MergeSOAPConfigs(base, recs, DefaultSOAPConvertOptions())

	if result.Operations == nil {
		t.Fatal("Expected operations map to be initialized")
	}
	if _, ok := result.Operations["Op"]; !ok {
		t.Error("Expected 'Op' to be added")
	}
}
