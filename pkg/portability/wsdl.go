package portability

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// WSDLImporter imports WSDL 1.1 service definitions and generates SOAP mock configurations.
// It parses the WSDL to extract services, port types, bindings, operations, messages, and
// XSD types, then generates mock.Mock objects with SOAP specs and optional stateful resource
// configurations.
type WSDLImporter struct {
	// Stateful controls whether the importer generates stateful resource mappings.
	// When true, operations with CRUD-like names (GetUser, CreateOrder, etc.) are
	// mapped to stateful resources with appropriate actions.
	Stateful bool
}

func init() {
	RegisterImporter(&WSDLImporter{})
}

// Format returns FormatWSDL.
func (w *WSDLImporter) Format() Format {
	return FormatWSDL
}

// Import parses a WSDL 1.1 document and generates a MockCollection with SOAP mocks.
func (w *WSDLImporter) Import(data []byte) (*config.MockCollection, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, &ImportError{
			Format:  FormatWSDL,
			Message: "failed to parse XML",
			Cause:   err,
		}
	}

	root := doc.Root()
	if root == nil {
		return nil, &ImportError{
			Format:  FormatWSDL,
			Message: "empty WSDL document",
		}
	}

	// Validate root element — must be definitions (WSDL 1.1) or description (WSDL 2.0)
	localName := root.Tag
	if localName != "definitions" && localName != "description" {
		return nil, &ImportError{
			Format:  FormatWSDL,
			Message: fmt.Sprintf("expected root element <definitions> or <description>, got <%s>", localName),
		}
	}

	if localName == "description" {
		return nil, &ImportError{
			Format:  FormatWSDL,
			Message: "WSDL 2.0 is not yet supported; please use a WSDL 1.1 document",
		}
	}

	def := parseWSDLDefinitions(root)
	return w.generateCollection(def, data)
}

// --- WSDL internal representation ---

// wsdlDefinitions is the parsed WSDL 1.1 document structure.
type wsdlDefinitions struct {
	Name            string
	TargetNamespace string
	Services        []wsdlService
	PortTypes       []wsdlPortType
	Bindings        []wsdlBinding
	Messages        []wsdlMessage
	Types           []wsdlXSDElement
}

type wsdlService struct {
	Name  string
	Ports []wsdlPort
}

type wsdlPort struct {
	Name     string
	Binding  string // QName reference to binding
	Location string // soap:address location
}

type wsdlPortType struct {
	Name       string
	Operations []wsdlOperation
}

type wsdlOperation struct {
	Name   string
	Input  string // message name (QName)
	Output string // message name (QName)
}

type wsdlBinding struct {
	Name       string
	Type       string // QName reference to portType
	Style      string // document/rpc
	Transport  string
	Operations []wsdlBindingOperation
}

type wsdlBindingOperation struct {
	Name       string
	SOAPAction string
}

type wsdlMessage struct {
	Name  string
	Parts []wsdlPart
}

type wsdlPart struct {
	Name    string
	Element string // QName reference to XSD element
	Type    string // QName reference to XSD type
}

// wsdlXSDElement represents a top-level XSD element or complex type from the <types> section.
type wsdlXSDElement struct {
	Name   string
	Fields []wsdlXSDField
}

type wsdlXSDField struct {
	Name     string
	Type     string // XSD type (xsd:string, xsd:int, etc.)
	Optional bool   // minOccurs="0" or nillable="true"
	Repeated bool   // maxOccurs="unbounded" or maxOccurs > 1
}

// --- WSDL parsing (using beevik/etree) ---

func parseWSDLDefinitions(root *etree.Element) *wsdlDefinitions {
	def := &wsdlDefinitions{
		Name:            root.SelectAttrValue("name", ""),
		TargetNamespace: root.SelectAttrValue("targetNamespace", ""),
	}

	// Parse messages
	for _, msgEl := range findElements(root, "message") {
		msg := wsdlMessage{Name: msgEl.SelectAttrValue("name", "")}
		for _, partEl := range findElements(msgEl, "part") {
			msg.Parts = append(msg.Parts, wsdlPart{
				Name:    partEl.SelectAttrValue("name", ""),
				Element: stripPrefix(partEl.SelectAttrValue("element", "")),
				Type:    stripPrefix(partEl.SelectAttrValue("type", "")),
			})
		}
		def.Messages = append(def.Messages, msg)
	}

	// Parse portTypes
	for _, ptEl := range findElements(root, "portType") {
		pt := wsdlPortType{Name: ptEl.SelectAttrValue("name", "")}
		for _, opEl := range findElements(ptEl, "operation") {
			op := wsdlOperation{Name: opEl.SelectAttrValue("name", "")}
			if inp := findElement(opEl, "input"); inp != nil {
				op.Input = stripPrefix(inp.SelectAttrValue("message", ""))
			}
			if out := findElement(opEl, "output"); out != nil {
				op.Output = stripPrefix(out.SelectAttrValue("message", ""))
			}
			pt.Operations = append(pt.Operations, op)
		}
		def.PortTypes = append(def.PortTypes, pt)
	}

	// Parse bindings
	for _, bindEl := range findElements(root, "binding") {
		b := wsdlBinding{
			Name: bindEl.SelectAttrValue("name", ""),
			Type: stripPrefix(bindEl.SelectAttrValue("type", "")),
		}
		// soap:binding
		if soapBind := findElementNS(bindEl, "binding"); soapBind != nil {
			b.Style = soapBind.SelectAttrValue("style", "document")
			b.Transport = soapBind.SelectAttrValue("transport", "")
		}
		// binding operations
		for _, opEl := range findElements(bindEl, "operation") {
			bop := wsdlBindingOperation{Name: opEl.SelectAttrValue("name", "")}
			if soapOp := findElementNS(opEl, "operation"); soapOp != nil {
				bop.SOAPAction = soapOp.SelectAttrValue("soapAction", "")
			}
			b.Operations = append(b.Operations, bop)
		}
		def.Bindings = append(def.Bindings, b)
	}

	// Parse services
	for _, svcEl := range findElements(root, "service") {
		svc := wsdlService{Name: svcEl.SelectAttrValue("name", "")}
		for _, portEl := range findElements(svcEl, "port") {
			p := wsdlPort{
				Name:    portEl.SelectAttrValue("name", ""),
				Binding: stripPrefix(portEl.SelectAttrValue("binding", "")),
			}
			if addr := findElementNS(portEl, "address"); addr != nil {
				p.Location = addr.SelectAttrValue("location", "")
			}
			svc.Ports = append(svc.Ports, p)
		}
		def.Services = append(def.Services, svc)
	}

	// Parse types (XSD elements and complex types)
	for _, typesEl := range findElements(root, "types") {
		for _, schemaEl := range findElements(typesEl, "schema") {
			def.Types = append(def.Types, parseXSDSchema(schemaEl)...)
		}
	}

	return def
}

// parseXSDSchema extracts top-level elements and complex types from an XSD schema.
func parseXSDSchema(schema *etree.Element) []wsdlXSDElement {
	var elements []wsdlXSDElement

	// Parse top-level xsd:element entries
	for _, elem := range findElements(schema, "element") {
		name := elem.SelectAttrValue("name", "")
		if name == "" {
			continue
		}
		xsdEl := wsdlXSDElement{Name: name}

		// Check for inline complexType
		if ct := findElement(elem, "complexType"); ct != nil {
			xsdEl.Fields = parseComplexTypeFields(ct)
		}

		elements = append(elements, xsdEl)
	}

	// Parse top-level xsd:complexType entries
	for _, ct := range findElements(schema, "complexType") {
		name := ct.SelectAttrValue("name", "")
		if name == "" {
			continue
		}
		xsdEl := wsdlXSDElement{
			Name:   name,
			Fields: parseComplexTypeFields(ct),
		}
		elements = append(elements, xsdEl)
	}

	return elements
}

// parseComplexTypeFields extracts fields from a complexType's sequence.
func parseComplexTypeFields(ct *etree.Element) []wsdlXSDField {
	var fields []wsdlXSDField

	seq := findElement(ct, "sequence")
	if seq == nil {
		// Try <all> as alternative to <sequence>
		seq = findElement(ct, "all")
	}
	if seq == nil {
		return fields
	}

	for _, elem := range findElements(seq, "element") {
		field := wsdlXSDField{
			Name:     elem.SelectAttrValue("name", ""),
			Type:     stripPrefix(elem.SelectAttrValue("type", "")),
			Optional: elem.SelectAttrValue("minOccurs", "1") == "0",
			Repeated: elem.SelectAttrValue("maxOccurs", "1") == "unbounded",
		}
		if elem.SelectAttrValue("nillable", "") == "true" {
			field.Optional = true
		}
		fields = append(fields, field)
	}

	return fields
}

// --- Mock generation ---

func (w *WSDLImporter) generateCollection(def *wsdlDefinitions, originalWSDL []byte) (*config.MockCollection, error) {
	now := time.Now()
	enabled := true

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    def.Name,
		Metadata: &config.CollectionMetadata{
			Name:        def.Name,
			Description: "Imported from WSDL: " + def.Name,
			Tags:        []string{"soap", "wsdl-import"},
		},
		Mocks: make([]*config.MockConfiguration, 0),
	}

	// Build message lookup for response element resolution
	messageMap := make(map[string]*wsdlMessage, len(def.Messages))
	for i := range def.Messages {
		messageMap[def.Messages[i].Name] = &def.Messages[i]
	}

	// Build XSD type lookup for sample response generation
	typeMap := make(map[string]*wsdlXSDElement, len(def.Types))
	for i := range def.Types {
		typeMap[def.Types[i].Name] = &def.Types[i]
	}

	// Build SOAPAction lookup from bindings
	soapActionMap := buildSOAPActionMap(def)

	// Build a portType lookup
	portTypeMap := make(map[string]*wsdlPortType, len(def.PortTypes))
	for i := range def.PortTypes {
		portTypeMap[def.PortTypes[i].Name] = &def.PortTypes[i]
	}

	// Generate one mock per unique path.
	// When a WSDL defines multiple bindings (SOAP 1.1, SOAP 1.2, HTTP POST) for the
	// same service, they often share the same path. We merge their operations into a
	// single mock to avoid ambiguous routing on the same endpoint.
	mockByPath := make(map[string]*config.MockConfiguration)

	for _, svc := range def.Services {
		for _, port := range svc.Ports {
			// Resolve the binding → portType chain
			binding := findBinding(def, port.Binding)
			if binding == nil {
				continue
			}
			pt := portTypeMap[binding.Type]
			if pt == nil {
				continue
			}

			// Determine path from soap:address or fallback
			path := extractPath(port.Location, svc.Name)

			// Reuse existing mock for this path, or create a new one
			m, exists := mockByPath[path]
			if !exists {
				m = &config.MockConfiguration{
					ID:      generateSOAPImportID(),
					Type:    mock.TypeSOAP,
					Name:    svc.Name + " - " + port.Name,
					Enabled: &enabled,
					SOAP: &mock.SOAPSpec{
						Path:       path,
						WSDL:       string(originalWSDL),
						Operations: make(map[string]mock.OperationConfig),
					},
					CreatedAt: now,
					UpdatedAt: now,
				}
				mockByPath[path] = m
				collection.Mocks = append(collection.Mocks, m)
			}

			// Merge operations — first binding wins for each operation name
			for _, op := range pt.Operations {
				if _, have := m.SOAP.Operations[op.Name]; have {
					continue // already registered by an earlier binding
				}
				opConfig := mock.OperationConfig{
					SOAPAction: soapActionMap[op.Name],
				}

				// Generate sample response XML from XSD types
				responseXML := w.generateResponseXML(op, messageMap, typeMap)
				opConfig.Response = responseXML

				// Stateful mapping heuristic
				if w.Stateful {
					resource, action := inferStatefulMapping(op.Name)
					if resource != "" {
						opConfig.StatefulResource = resource
						opConfig.StatefulAction = action
						opConfig.Response = "" // stateful handler generates response
					}
				}

				m.SOAP.Operations[op.Name] = opConfig
			}
		}
	}

	// If stateful mode, also generate StatefulResource configs
	if w.Stateful {
		collection.StatefulResources = w.inferStatefulResources(def, typeMap)
	}

	if len(collection.Mocks) == 0 {
		return nil, &ImportError{
			Format:  FormatWSDL,
			Message: "no services found in WSDL document",
		}
	}

	return collection, nil
}

// generateResponseXML generates sample XML response body for an operation.
func (w *WSDLImporter) generateResponseXML(op wsdlOperation, messages map[string]*wsdlMessage, types map[string]*wsdlXSDElement) string {
	// Find the output message
	msg := messages[op.Output]
	if msg == nil {
		// Fallback: generate a simple wrapper
		return fmt.Sprintf("<%sResponse><result>ok</result></%sResponse>", op.Name, op.Name)
	}

	// Find the element referenced by the message part
	var elementName string
	for _, part := range msg.Parts {
		if part.Element != "" {
			elementName = part.Element
			break
		}
	}

	if elementName == "" {
		return fmt.Sprintf("<%sResponse><result>ok</result></%sResponse>", op.Name, op.Name)
	}

	// Look up the XSD element definition
	xsdEl := types[elementName]
	if xsdEl == nil || len(xsdEl.Fields) == 0 {
		return fmt.Sprintf("<%s><result>ok</result></%s>", elementName, elementName)
	}

	// Generate XML from XSD fields
	return generateXMLFromFields(elementName, xsdEl.Fields, types)
}

// generateXMLFromFields generates sample XML from a list of XSD fields.
func generateXMLFromFields(wrapperName string, fields []wsdlXSDField, types map[string]*wsdlXSDElement) string {
	var b strings.Builder
	b.WriteString("<" + wrapperName + ">")

	for _, f := range fields {
		// Check if the field type is a known complex type
		if ct := types[f.Type]; ct != nil && len(ct.Fields) > 0 {
			b.WriteString(generateXMLFromFields(f.Name, ct.Fields, types))
		} else {
			b.WriteString("<" + f.Name + ">")
			b.WriteString(sampleValueForXSDType(f.Type))
			b.WriteString("</" + f.Name + ">")
		}
	}

	b.WriteString("</" + wrapperName + ">")
	return b.String()
}

// sampleValueForXSDType returns a sample value for a given XSD type.
func sampleValueForXSDType(xsdType string) string {
	switch xsdType {
	case "string", "xsd:string":
		return "sample"
	case "int", "xsd:int", "integer", "xsd:integer", "long", "xsd:long", "short", "xsd:short":
		return "0"
	case "float", "xsd:float", "double", "xsd:double", "decimal", "xsd:decimal":
		return "0.0"
	case "boolean", "xsd:boolean":
		return "true"
	case "date", "xsd:date":
		return "2026-01-01"
	case "dateTime", "xsd:dateTime":
		return "2026-01-01T00:00:00Z"
	default:
		return "sample"
	}
}

// --- Stateful mapping heuristics (WI-05) ---

// inferStatefulMapping maps a WSDL operation name to a stateful resource and action.
// Uses common CRUD naming patterns: Get/Find → get, Create/Add → create, etc.
func inferStatefulMapping(operationName string) (resource string, action string) {
	name := operationName
	lower := strings.ToLower(name)

	// Try prefix-based patterns (longer/more-specific prefixes first)
	prefixes := []struct {
		prefix string
		action string
	}{
		{"GetAll", "list"},
		{"FindAll", "list"},
		{"FetchAll", "list"},
		{"RetrieveAll", "list"},
		{"Get", "get"},
		{"Find", "get"},
		{"Fetch", "get"},
		{"Retrieve", "get"},
		{"Create", "create"},
		{"Add", "create"},
		{"Insert", "create"},
		{"New", "create"},
		{"Update", "update"},
		{"Modify", "update"},
		{"Edit", "update"},
		{"Delete", "delete"},
		{"Remove", "delete"},
		{"Destroy", "delete"},
		{"List", "list"},
		{"Search", "list"},
	}

	for _, p := range prefixes {
		if strings.HasPrefix(name, p.prefix) {
			remainder := name[len(p.prefix):]
			if remainder == "" {
				continue
			}
			resource = normalizeResourceName(remainder)
			action = p.action
			return
		}
	}

	// Try suffix-based patterns (e.g., UserGet, OrderCreate)
	suffixes := []struct {
		suffix string
		action string
	}{
		{"List", "list"},
		{"All", "list"},
	}
	for _, s := range suffixes {
		if strings.HasSuffix(name, s.suffix) {
			remainder := name[:len(name)-len(s.suffix)]
			if remainder == "" {
				continue
			}
			resource = normalizeResourceName(remainder)
			action = s.action
			return
		}
	}

	// Fallback: if the name contains known verbs at the start (case-insensitive)
	_ = lower
	return "", ""
}

// normalizeResourceName converts a PascalCase entity name to a lowercase plural resource name.
// Examples: "User" → "users", "OrderItem" → "orderitems", "Products" → "products"
func normalizeResourceName(name string) string {
	lower := strings.ToLower(name)
	// Simple pluralization: add "s" if not already plural
	if !strings.HasSuffix(lower, "s") {
		lower += "s"
	}
	return lower
}

// inferStatefulResources generates StatefulResourceConfig entries from WSDL operations.
func (w *WSDLImporter) inferStatefulResources(def *wsdlDefinitions, types map[string]*wsdlXSDElement) []*config.StatefulResourceConfig {
	resourceSet := make(map[string]*config.StatefulResourceConfig)

	for _, pt := range def.PortTypes {
		for _, op := range pt.Operations {
			resource, _ := inferStatefulMapping(op.Name)
			if resource == "" {
				continue
			}
			if _, exists := resourceSet[resource]; exists {
				continue
			}

			rc := &config.StatefulResourceConfig{
				Name:     resource,
				BasePath: "/api/" + resource,
			}

			// Try to infer seed data schema from XSD types
			seedData := w.inferSeedData(op, resource, def, types)
			if len(seedData) > 0 {
				rc.SeedData = seedData
			}

			resourceSet[resource] = rc
		}
	}

	// Sort for deterministic output
	resources := make([]*config.StatefulResourceConfig, 0, len(resourceSet))
	names := make([]string, 0, len(resourceSet))
	for name := range resourceSet {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		resources = append(resources, resourceSet[name])
	}

	return resources
}

// inferSeedData generates sample seed data from XSD types associated with the operation.
func (w *WSDLImporter) inferSeedData(_ wsdlOperation, resource string, _ *wsdlDefinitions, types map[string]*wsdlXSDElement) []map[string]interface{} {
	// Look for a response type that contains the entity fields
	// E.g., GetUserResponse → User type → fields
	for _, xsdEl := range types {
		lower := strings.ToLower(xsdEl.Name)
		// Match entity types (not Request/Response wrappers)
		if strings.HasSuffix(lower, "request") || strings.HasSuffix(lower, "response") {
			continue
		}

		// Check if this type name matches the singular form of the resource
		singular := strings.TrimSuffix(resource, "s")
		if strings.ToLower(xsdEl.Name) == singular && len(xsdEl.Fields) > 0 {
			// Generate one sample seed item
			item := make(map[string]interface{})
			for _, f := range xsdEl.Fields {
				item[f.Name] = sampleGoValueForXSDType(f.Type)
			}
			// Override ID with a descriptive value
			if _, hasID := item["id"]; hasID {
				item["id"] = singular + "-1"
			}
			return []map[string]interface{}{item}
		}
	}

	return nil
}

// sampleGoValueForXSDType returns a Go-typed sample value for seed data.
func sampleGoValueForXSDType(xsdType string) interface{} {
	switch xsdType {
	case "string", "xsd:string":
		return "sample"
	case "int", "xsd:int", "integer", "xsd:integer", "long", "xsd:long", "short", "xsd:short":
		return 0
	case "float", "xsd:float", "double", "xsd:double", "decimal", "xsd:decimal":
		return 0.0
	case "boolean", "xsd:boolean":
		return true
	default:
		return "sample"
	}
}

// --- Helper functions ---

// generateSOAPImportID generates a unique ID for an imported SOAP mock.
func generateSOAPImportID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "soap_" + hex.EncodeToString(b)
}

// extractPath extracts the URL path from a soap:address location, or generates a fallback.
func extractPath(location, serviceName string) string {
	if location != "" {
		if u, err := url.Parse(location); err == nil && u.Path != "" {
			return u.Path
		}
	}
	// Fallback: generate from service name
	return "/soap/" + strings.ToLower(serviceName)
}

// buildSOAPActionMap builds a map of operation name → SOAPAction from all bindings.
func buildSOAPActionMap(def *wsdlDefinitions) map[string]string {
	m := make(map[string]string)
	for _, b := range def.Bindings {
		for _, op := range b.Operations {
			if op.SOAPAction != "" {
				m[op.Name] = op.SOAPAction
			}
		}
	}
	return m
}

// findBinding returns the binding with the given name.
func findBinding(def *wsdlDefinitions, name string) *wsdlBinding {
	for i := range def.Bindings {
		if def.Bindings[i].Name == name {
			return &def.Bindings[i]
		}
	}
	return nil
}

// findElements returns all direct child elements matching the local name (ignoring namespace prefix).
func findElements(parent *etree.Element, localName string) []*etree.Element {
	var results []*etree.Element
	for _, child := range parent.ChildElements() {
		tag := child.Tag
		// Strip namespace prefix (e.g., "wsdl:message" → "message")
		if idx := strings.IndexByte(tag, ':'); idx >= 0 {
			tag = tag[idx+1:]
		}
		if tag == localName {
			results = append(results, child)
		}
	}
	return results
}

// findElement returns the first direct child element matching the local name.
func findElement(parent *etree.Element, localName string) *etree.Element {
	elems := findElements(parent, localName)
	if len(elems) > 0 {
		return elems[0]
	}
	return nil
}

// findElementNS finds an element by local name in any SOAP namespace.
// etree stores the namespace URI in child.Space and the local name in child.Tag,
// so we match on Tag (local name) and Space (namespace URI).
func findElementNS(parent *etree.Element, localName string) *etree.Element {
	for _, child := range parent.ChildElements() {
		if child.Tag == localName && isSOAPNamespace(child.Space) {
			return child
		}
	}
	return nil
}

// isSOAPNamespace returns true if the namespace is a SOAP binding namespace.
// etree stores the namespace prefix in Space (e.g., "soap"), not the full URI.
func isSOAPNamespace(ns string) bool {
	switch ns {
	// Namespace prefixes (etree default behavior)
	case "soap", "soap12", "wsoap":
		return true
	// Full namespace URIs (if etree resolves them)
	case "http://schemas.xmlsoap.org/wsdl/soap/",
		"http://schemas.xmlsoap.org/wsdl/soap12/",
		"http://www.w3.org/ns/wsdl/soap":
		return true
	default:
		return false
	}
}

// stripPrefix removes a namespace prefix from a QName (e.g., "tns:Foo" → "Foo").
func stripPrefix(qname string) string {
	if idx := strings.IndexByte(qname, ':'); idx >= 0 {
		return qname[idx+1:]
	}
	return qname
}
