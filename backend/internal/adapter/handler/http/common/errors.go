package common

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

// RenderError renders an error response and logs any render failures.
// This helper consolidates the common pattern of rendering errors and
// logging render failures, reducing code duplication across handlers.
func RenderError(w http.ResponseWriter, r *http.Request, renderer render.Renderer) {
	if err := render.Render(w, r, renderer); err != nil {
		if logger.Logger != nil {
			logger.Logger.WithField("error", err).Error("Error rendering error response")
		}
	}
}

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
	ErrConflict         = errors.New("resource conflict")
	ErrTooManyRequests  = errors.New("too many requests")
	ErrGone             = errors.New("resource no longer available")
)

// LogRenderError is the format string for logging render errors
const LogRenderError = "Error rendering error response: %v"

// Validation error messages
const (
	MsgInvalidGroupID         = "invalid group ID"
	MsgInvalidStudentID       = "invalid student ID"
	MsgInvalidStaffID         = "invalid staff ID"
	MsgInvalidActivityID      = "invalid activity ID"
	MsgInvalidRoleID          = "invalid role ID"
	MsgInvalidAccountID       = "invalid account ID"
	MsgInvalidPermissionID    = "invalid permission ID"
	MsgInvalidParentAccountID = "invalid parent account ID"
	MsgInvalidSettingID       = "invalid setting ID"
	MsgInvalidRoomID          = "invalid room ID"
	MsgInvalidWeekday         = "invalid weekday"
	MsgInvalidPersonID        = "invalid person ID"
)

// Not found messages
const (
	MsgGroupNotFound = "group not found"
	MsgStaffNotFound = "staff member not found"
)

// Date format constants
const (
	DateFormatISO = "2006-01-02"
)

// ErrResponse is the error response structure
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string `json:"status"`
	ErrorText string `json:"error,omitempty"`
}

// Render implements the render.Renderer interface for ErrResponse
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorConflict returns a 409 Conflict error response
func ErrorConflict(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorTooManyRequests returns a 429 Too Many Requests error response
func ErrorTooManyRequests(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusTooManyRequests,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorGone returns a 410 Gone error response
func ErrorGone(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusGone,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}
