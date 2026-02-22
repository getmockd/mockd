package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

func mapCreateMockAddError(err error, log *slog.Logger, operation string) (int, string, string) {
	switch {
	case errors.Is(err, engineclient.ErrDuplicate):
		return http.StatusConflict, "conflict", ErrMsgConflict
	case errors.Is(err, engineclient.ErrNotFound):
		return http.StatusNotFound, "not_found", ErrMsgNotFound
	}

	errMsg := err.Error()
	switch {
	case isPortError(errMsg):
		return http.StatusConflict, "port_unavailable", "Failed to start mock: the port may be in use by another process"
	case isValidationError(errMsg):
		return http.StatusBadRequest, "validation_error", errMsg
	default:
		return http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, log, operation)
	}
}

func mapStreamAddError(err error, log *slog.Logger) (int, string, string) {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "endpointPath is required"):
		return http.StatusBadRequest, "validation_error", errMsg
	case strings.Contains(errMsg, "unsupported protocol"),
		strings.Contains(errMsg, "unexpected config type"):
		if log != nil {
			log.Error("stream add-to-server failed", "error", err)
		}
		return http.StatusInternalServerError, "add_error", ErrMsgInternalError
	default:
		return mapCreateMockAddError(err, log, "add stream mock to engine")
	}
}
