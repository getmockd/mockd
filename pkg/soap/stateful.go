package soap

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

// StatefulAction represents a CRUD action for stateful operations.
type StatefulAction string

const (
	StatefulActionGet    StatefulAction = "get"
	StatefulActionList   StatefulAction = "list"
	StatefulActionCreate StatefulAction = "create"
	StatefulActionUpdate StatefulAction = "update"
	StatefulActionPatch  StatefulAction = "patch"
	StatefulActionDelete StatefulAction = "delete"
	StatefulActionCustom StatefulAction = "custom"
)

// StatefulRequest is a protocol-agnostic request to perform a CRUD operation.
// The SOAP handler builds this from the SOAP envelope and operation config,
// and the StatefulExecutor (provided by the engine) executes it against the store.
type StatefulRequest struct {
	// Resource is the stateful resource name (e.g., "users")
	Resource string
	// Action is the CRUD action to perform
	Action StatefulAction
	// OperationName is the protocol-level operation name (e.g., SOAP operation "TransferFunds").
	// For custom actions, this is used to look up the registered custom operation.
	OperationName string
	// ResourceID is the item ID for single-item operations
	ResourceID string
	// Data is the deserialized request payload
	Data map[string]interface{}
	// Filter contains query parameters for list operations
	Filter *StatefulFilter
}

// StatefulFilter contains pagination/filter parameters for list operations.
type StatefulFilter struct {
	Limit  int
	Offset int
	Sort   string
	Order  string
}

// StatefulResult is the protocol-agnostic result from a stateful operation.
type StatefulResult struct {
	// Success indicates whether the operation succeeded.
	Success bool
	// Item is the single item result (for get/create/update/patch).
	Item map[string]interface{}
	// Items is the list result (for list).
	Items []map[string]interface{}
	// Meta contains pagination metadata for list results.
	Meta *StatefulListMeta
	// Error is the fault to return, if any.
	Error *SOAPFault
}

// StatefulListMeta contains pagination metadata.
type StatefulListMeta struct {
	Total  int
	Count  int
	Offset int
	Limit  int
}

// StatefulExecutor is the interface that the engine provides to execute
// stateful operations. This decouples the SOAP package from pkg/stateful
// (avoiding import cycles). The engine wires in a concrete implementation
// that delegates to stateful.Bridge.
type StatefulExecutor interface {
	ExecuteStateful(ctx context.Context, req *StatefulRequest) *StatefulResult
}

// SetStatefulExecutor configures the stateful executor for this handler.
// When set, operations with StatefulResource/StatefulAction fields will
// route through the executor instead of returning canned responses.
func (h *Handler) SetStatefulExecutor(executor StatefulExecutor) {
	h.statefulExecutor = executor
}

// GetStatefulExecutor returns the stateful executor, if configured.
func (h *Handler) GetStatefulExecutor() StatefulExecutor {
	return h.statefulExecutor
}

// handleStatefulOperation executes a stateful CRUD operation and returns
// the SOAP response body XML. Returns (nil, nil) if the operation is not stateful.
// opName is the SOAP operation name (e.g., "TransferFunds"), needed for custom operation lookup.
func (h *Handler) handleStatefulOperation(opName string, opConfig *OperationConfig, doc *etree.Document) ([]byte, *SOAPFault) {
	if opConfig.StatefulResource == "" || h.statefulExecutor == nil {
		return nil, nil
	}

	// Build the StatefulRequest from the SOAP body
	req := buildStatefulRequest(opName, opConfig, doc)

	// Execute via the executor (provided by the engine, backed by stateful.Bridge)
	result := h.statefulExecutor.ExecuteStateful(context.Background(), req)

	// Check for errors — return as SOAP fault
	if result.Error != nil {
		return nil, result.Error
	}

	// Build XML response from result.
	// For custom operations, use the operation name as the response wrapper (e.g., <TransferFundsResponse>).
	// For CRUD operations, use the resource name (e.g., <userResponse>).
	responseName := opConfig.StatefulResource
	if req.Action == StatefulActionCustom && opName != "" {
		responseName = opName
	}

	var responseXML []byte
	switch {
	case result.Items != nil:
		responseXML = listResultToXML(result, responseName)
	case result.Item != nil:
		responseXML = mapToXML(result.Item, responseName)
	default:
		// Delete/custom success — return a simple success element
		responseXML = []byte(fmt.Sprintf("<%sResponse><success>true</success></%sResponse>",
			responseName, responseName))
	}

	return responseXML, nil
}

// buildStatefulRequest translates SOAP body XML into a StatefulRequest.
// opName is the SOAP operation name, used as the custom operation lookup key when action is "custom".
func buildStatefulRequest(opName string, opConfig *OperationConfig, doc *etree.Document) *StatefulRequest {
	action := StatefulAction(opConfig.StatefulAction)

	req := &StatefulRequest{
		Resource:      opConfig.StatefulResource,
		Action:        action,
		OperationName: opName,
	}

	// Extract the SOAP body content
	body := findSOAPBody(doc)
	if body == nil {
		return req
	}

	// Get the first child element of Body (the operation element)
	children := body.ChildElements()
	if len(children) == 0 {
		return req
	}

	opElement := children[0]

	// Extract data from the operation element's children
	data := xmlElementToMap(opElement)

	// For get/update/patch/delete: extract the ID from the data
	switch action {
	case StatefulActionGet, StatefulActionUpdate, StatefulActionPatch, StatefulActionDelete:
		if id, ok := data["id"]; ok {
			req.ResourceID = fmt.Sprintf("%v", id)
			// For get/delete, we don't need data; for update/patch, keep data but remove id
			if action == StatefulActionGet || action == StatefulActionDelete {
				data = nil
			} else {
				delete(data, "id")
			}
		}
	case StatefulActionCreate:
		// Create: data is the entire payload (id may be auto-generated)
		if id, ok := data["id"]; ok {
			req.ResourceID = fmt.Sprintf("%v", id)
		}
	case StatefulActionList:
		// List: extract filter params from data if present
		filter := &StatefulFilter{
			Limit:  100,
			Offset: 0,
			Sort:   "createdAt",
			Order:  "desc",
		}
		if limit, ok := data["limit"]; ok {
			if v, err := toInt(limit); err == nil {
				filter.Limit = v
			}
		}
		if offset, ok := data["offset"]; ok {
			if v, err := toInt(offset); err == nil {
				filter.Offset = v
			}
		}
		if sortField, ok := data["sort"]; ok {
			filter.Sort = fmt.Sprintf("%v", sortField)
		}
		if order, ok := data["order"]; ok {
			filter.Order = fmt.Sprintf("%v", order)
		}
		req.Filter = filter
		data = nil
	case StatefulActionCustom:
		// Custom: pass all XML body fields through as-is.
		// These become the "input" map for the custom operation executor.
	}

	req.Data = data
	return req
}

// findSOAPBody finds the Body element in a SOAP envelope document.
func findSOAPBody(doc *etree.Document) *etree.Element {
	if doc == nil {
		return nil
	}
	// Try direct path
	body := doc.FindElement("//Body")
	if body != nil {
		return body
	}
	// Try with local-name (handles namespace prefixes)
	return doc.FindElement("//*[local-name()='Body']")
}

// xmlElementToMap converts an etree.Element's children into a map[string]interface{}.
// Nested elements are recursively converted. Leaf elements become string values.
// Repeated elements with the same tag become slices.
func xmlElementToMap(elem *etree.Element) map[string]interface{} {
	if elem == nil {
		return nil
	}

	result := make(map[string]interface{})
	children := elem.ChildElements()

	if len(children) == 0 {
		return result
	}

	// Track which keys have multiple values (for repeated elements)
	counts := make(map[string]int)
	for _, child := range children {
		counts[child.Tag]++
	}

	for _, child := range children {
		key := child.Tag
		grandChildren := child.ChildElements()

		var value interface{}
		if len(grandChildren) == 0 {
			// Leaf element: use text content
			value = child.Text()
		} else {
			// Nested element: recurse
			value = xmlElementToMap(child)
		}

		if counts[key] > 1 {
			// Multiple elements with same tag -> build a slice
			if existing, ok := result[key]; ok {
				if slice, ok := existing.([]interface{}); ok {
					result[key] = append(slice, value)
				} else {
					result[key] = []interface{}{existing, value}
				}
			} else {
				result[key] = []interface{}{value}
			}
		} else {
			result[key] = value
		}
	}

	return result
}

// mapToXML converts a map[string]interface{} to XML bytes wrapped in a response element.
func mapToXML(data map[string]interface{}, resourceName string) []byte {
	var buf bytes.Buffer
	singular := singularize(resourceName)
	buf.WriteString("<" + singular + "Response>")
	writeMapAsXML(&buf, data)
	buf.WriteString("</" + singular + "Response>")
	return buf.Bytes()
}

// listResultToXML converts a StatefulResult with Items to XML bytes with a list wrapper.
func listResultToXML(result *StatefulResult, resourceName string) []byte {
	var buf bytes.Buffer
	singular := singularize(resourceName)

	buf.WriteString("<" + resourceName + "Response>")

	// Items
	for _, item := range result.Items {
		buf.WriteString("<" + singular + ">")
		writeMapAsXML(&buf, item)
		buf.WriteString("</" + singular + ">")
	}

	// Meta
	if result.Meta != nil {
		buf.WriteString("<meta>")
		buf.WriteString("<total>" + strconv.Itoa(result.Meta.Total) + "</total>")
		buf.WriteString("<count>" + strconv.Itoa(result.Meta.Count) + "</count>")
		buf.WriteString("<offset>" + strconv.Itoa(result.Meta.Offset) + "</offset>")
		buf.WriteString("<limit>" + strconv.Itoa(result.Meta.Limit) + "</limit>")
		buf.WriteString("</meta>")
	}

	buf.WriteString("</" + resourceName + "Response>")
	return buf.Bytes()
}

// writeMapAsXML writes map entries as XML elements to a buffer.
// Keys are sorted for deterministic output.
func writeMapAsXML(buf *bytes.Buffer, data map[string]interface{}) {
	if data == nil {
		return
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := data[key]
		writeValueAsXML(buf, key, value)
	}
}

// writeValueAsXML writes a single value as an XML element.
func writeValueAsXML(buf *bytes.Buffer, key string, value interface{}) {
	switch v := value.(type) {
	case map[string]interface{}:
		buf.WriteString("<" + key + ">")
		writeMapAsXML(buf, v)
		buf.WriteString("</" + key + ">")
	case []interface{}:
		for _, item := range v {
			writeValueAsXML(buf, key, item)
		}
	case nil:
		// Omit nil values
	default:
		buf.WriteString("<" + key + ">")
		buf.WriteString(escapeXML(fmt.Sprintf("%v", v)))
		buf.WriteString("</" + key + ">")
	}
}

// singularize performs a simple pluralization removal.
// "users" → "user", "orders" → "order", "addresses" → "address"
func singularize(s string) string {
	if strings.HasSuffix(s, "ses") {
		return strings.TrimSuffix(s, "es")
	}
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// toInt converts an interface{} to int.
func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}
