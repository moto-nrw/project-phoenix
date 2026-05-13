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

// FieldError describes one per-field validation problem. Emitted inside
// ErrResponse.Errors so the frontend can render a form-level error list
// without a second round-trip. Both fields are required.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// ErrResponse is the error response structure
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string       `json:"status"`
	ErrorText string       `json:"error,omitempty"`
	Code      string       `json:"code,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// Render implements the render.Renderer interface for ErrResponse
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// newErrResponse builds an ErrResponse, tolerating a nil err by falling back to
// the standard HTTP status text. A nil err here means a caller violated the
// helper contract — we log a warning so the bug surfaces instead of being
// masked by a panic or a silent "unknown error" string.
func newErrResponse(status int, err error) *ErrResponse {
	text := http.StatusText(status)
	if err != nil {
		text = err.Error()
	} else {
		slog.Default().Warn("error helper called with nil error",
			slog.Int("status", status),
		)
	}
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: status,
		Status:         "error",
		ErrorText:      text,
	}
}

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return newErrResponse(http.StatusBadRequest, err)
}

// ErrorValidation returns a 400 Bad Request with a summary message and a
// per-field error list. The summary goes in the standard `error` field so
// existing frontend handlers that read `.error` continue to display a
// message; the `errors` array surfaces individual field problems for form
// rendering.
func ErrorValidation(summary string, fields []FieldError) render.Renderer {
	return &ErrResponse{
		Err:            errors.New(summary),
		HTTPStatusCode: http.StatusBadRequest,
		Status:         "error",
		ErrorText:      summary,
		Errors:         fields,
	}
}

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return newErrResponse(http.StatusUnauthorized, err)
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return newErrResponse(http.StatusForbidden, err)
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return newErrResponse(http.StatusNotFound, err)
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return newErrResponse(http.StatusInternalServerError, err)
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
	return newErrResponse(http.StatusConflict, err)
}

// ErrorConflictWithCode returns a 409 Conflict with a stable error code for frontend disambiguation.
func ErrorConflictWithCode(err error, code string) render.Renderer {
	resp := newErrResponse(http.StatusConflict, err)
	resp.Code = code
	return resp
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
	return newErrResponse(http.StatusTooManyRequests, err)
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
	return newErrResponse(http.StatusGone, err)
}
