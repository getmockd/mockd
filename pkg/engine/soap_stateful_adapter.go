package engine

import (
	"context"

	"github.com/getmockd/mockd/pkg/soap"
	"github.com/getmockd/mockd/pkg/stateful"
)

// soapStatefulAdapter implements soap.StatefulExecutor by delegating to stateful.Bridge.
// This adapter lives in the engine package to avoid an import cycle between soap and stateful.
type soapStatefulAdapter struct {
	bridge *stateful.Bridge
}

// newSOAPStatefulAdapter creates a new adapter wrapping the given bridge.
func newSOAPStatefulAdapter(bridge *stateful.Bridge) *soapStatefulAdapter {
	return &soapStatefulAdapter{bridge: bridge}
}

// ExecuteStateful implements soap.StatefulExecutor.
// It translates soap.StatefulRequest → stateful.OperationRequest,
// calls Bridge.Execute(), and translates stateful.OperationResult → soap.StatefulResult.
func (a *soapStatefulAdapter) ExecuteStateful(ctx context.Context, req *soap.StatefulRequest) *soap.StatefulResult {
	if a.bridge == nil || req == nil {
		return &soap.StatefulResult{
			Error: &soap.SOAPFault{
				Code:    "soap:Server",
				Message: "stateful bridge is not configured",
			},
		}
	}

	// Translate soap.StatefulRequest → stateful.OperationRequest
	opReq := &stateful.OperationRequest{
		Resource:      req.Resource,
		Action:        stateful.Action(req.Action),
		OperationName: req.OperationName,
		ResourceID:    req.ResourceID,
		Data:          req.Data,
	}

	// Translate filter
	if req.Filter != nil {
		opReq.Filter = &stateful.QueryFilter{
			Limit:   req.Filter.Limit,
			Offset:  req.Filter.Offset,
			Sort:    req.Filter.Sort,
			Order:   req.Filter.Order,
			Filters: make(map[string]string),
		}
	}

	// Execute via bridge
	result := a.bridge.Execute(ctx, opReq)

	// Translate stateful.OperationResult → soap.StatefulResult
	soapResult := &soap.StatefulResult{
		Success: result.Error == nil,
	}

	if result.Error != nil {
		soapResult.Error = errorToSOAPFault(result.Error)
		return soapResult
	}

	// Single item result
	if result.Item != nil {
		soapResult.Item = result.Item.ToJSON()
	}

	// List result
	if result.List != nil {
		soapResult.Items = append(soapResult.Items, result.List.Data...)
		soapResult.Meta = &soap.StatefulListMeta{
			Total:  result.List.Meta.Total,
			Count:  result.List.Meta.Count,
			Offset: result.List.Meta.Offset,
			Limit:  result.List.Meta.Limit,
		}
	}

	return soapResult
}

// errorToSOAPFault converts a stateful error to a SOAP fault.
// NotFound/Conflict/Validation → soap:Client (client error)
// Internal/Capacity → soap:Server (server error)
func errorToSOAPFault(err error) *soap.SOAPFault {
	code := stateful.GetErrorCode(err)

	switch code {
	case stateful.ErrCodeNotFound, stateful.ErrCodeConflict, stateful.ErrCodeValidation:
		return &soap.SOAPFault{
			Code:    "soap:Client",
			Message: err.Error(),
		}
	case stateful.ErrCodeCapacityExceeded:
		return &soap.SOAPFault{
			Code:    "soap:Server",
			Message: err.Error(),
		}
	default:
		return &soap.SOAPFault{
			Code:    "soap:Server",
			Message: err.Error(),
		}
	}
}
