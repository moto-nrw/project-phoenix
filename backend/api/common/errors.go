package common

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/render"
	"github.com/uptrace/bun/driver/pgdriver"
)

// RenderError renders an error response and logs any render failures.
// For server errors (5xx), it also logs the root cause to slog and reports
// the error to Sentry so that failures are visible in both Grafana and Sentry.
func RenderError(w http.ResponseWriter, r *http.Request, renderer render.Renderer) {
	if errResp, ok := renderer.(*ErrResponse); ok && errResp.HTTPStatusCode >= 500 && errResp.Err != nil {
		slog.Default().ErrorContext(r.Context(), "server error",
			slog.Int("status", errResp.HTTPStatusCode),
			slog.String("error", errResp.Err.Error()),
		)
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.CaptureException(errResp.Err)
		} else {
			sentry.CaptureException(errResp.Err)
		}
	}
	if err := render.Render(w, r, renderer); err != nil {
		slog.Default().Error("error rendering error response", slog.String("error", err.Error()))
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
	Code      string `json:"code,omitempty"`
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

// ErrorInternalServerWrap returns a 500 response with a stable client-facing message
// while preserving the full error chain for Sentry and slog.
// Use this instead of ErrorInternalServer when the original error contains
// internal details (DB errors, stack traces) that must not leak to clients.
func ErrorInternalServerWrap(clientMsg string, cause error) render.Renderer {
	return &ErrResponse{
		Err:            fmt.Errorf("%s: %w", clientMsg, cause),
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      clientMsg,
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

// ErrorConflictWithCode returns a 409 Conflict with a stable error code for frontend disambiguation.
func ErrorConflictWithCode(err error, code string) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
		Code:           code,
	}
}

// ErrorConflictMessage returns a 409 Conflict with a user-facing message string.
// Use this instead of ErrorConflict(errors.New(...)) for localized messages
// that would violate Go's lowercase error string convention (ST1005).
func ErrorConflictMessage(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      message,
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

// IsConstraintViolation checks if an error is a PostgreSQL constraint violation
// that indicates the entity cannot be deleted due to dependencies.
// Primary check uses typed pgdriver.Error with SQLSTATE codes (23503 = FK, 23502 = NOT NULL).
// Fallback string matching covers errors that have been wrapped and lost the original type.
func IsConstraintViolation(err error) bool {
	if err == nil {
		return false
	}

	// Primary: typed pgdriver.Error with structured SQLSTATE code
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C') // SQLSTATE code
		return code == "23503" || code == "23502"
	}

	// Fallback: string matching for wrapped errors that lost the pgdriver.Error type
	msg := err.Error()
	return strings.Contains(msg, "violates foreign key constraint") ||
		strings.Contains(msg, "violates not-null constraint")
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
