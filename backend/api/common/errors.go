package common

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"net"
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

// ErrUnauthorized is the sentinel for authentication failures.
var ErrUnauthorized = errors.New("unauthorized")

const statusClientClosedRequest = 499

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
	// Details carries structured, code-specific payload that lets the frontend
	// react to a conflict without having to refetch and pattern-match local
	// state. Populated by helpers like ErrorConflictWithDetails. Keys are
	// snake_case to match the rest of the API's wire format.
	Details map[string]any `json:"details,omitempty"`
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

// ErrorInvalidRequestWithCode returns a 400 Bad Request with a stable
// error code so the frontend can map to a localized German message
// without parsing the free-form error string.
func ErrorInvalidRequestWithCode(err error, code string) render.Renderer {
	resp := newErrResponse(http.StatusBadRequest, err)
	resp.Code = code
	return resp
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

// ErrorUnauthorizedWithCode returns a 401 Unauthorized with a stable error
// code for frontend disambiguation (e.g. distinguishing wrong-password from
// account-inactive without leaking the difference in the human-readable
// message).
func ErrorUnauthorizedWithCode(err error, code string) render.Renderer {
	resp := newErrResponse(http.StatusUnauthorized, err)
	resp.Code = code
	return resp
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return newErrResponse(http.StatusForbidden, err)
}

// ErrorForbiddenWithCode returns a 403 Forbidden with a stable error code.
func ErrorForbiddenWithCode(err error, code string) render.Renderer {
	resp := newErrResponse(http.StatusForbidden, err)
	resp.Code = code
	return resp
}

// ErrorNotFound returns a 404 Not Found error response
// ErrorNotFoundWithCode returns a 404 with a stable error code.
func ErrorNotFoundWithCode(err error, code string) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      err.Error(),
		Code:           code,
	}
}

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

// ErrorInternalServerRenderer binds stable client text for declarative error
// rules while preserving each matched error as the logged cause.
func ErrorInternalServerRenderer(clientMsg string) func(error) render.Renderer {
	return func(err error) render.Renderer {
		return ErrorInternalServerWrap(clientMsg, err)
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

// ErrorConflictWithDetails returns a 409 Conflict carrying both a stable code
// and a structured details payload. Use this when the frontend needs concrete
// fields (e.g. the conflicting session_id) to drive a follow-up action — it
// removes the dependency on whichever local state happens to be loaded.
func ErrorConflictWithDetails(err error, code string, details map[string]any) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
		Code:           code,
		Details:        details,
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

// ErrorInvalidRequestMessage returns a 400 Bad Request with a user-facing
// message string. Same reason as ErrorConflictMessage: a localized sentence
// (capitalized, ending in a period) is UI copy, not a Go error string, and
// wrapping it in errors.New would violate ST1005.
func ErrorInvalidRequestMessage(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusBadRequest,
		Status:         "error",
		ErrorText:      message,
	}
}

// ErrorInvalidRequestMessageWithCode returns a 400 with user-facing copy and
// a stable domain code.
func ErrorInvalidRequestMessageWithCode(message, code string) render.Renderer {
	resp := ErrorInvalidRequestMessage(message).(*ErrResponse)
	resp.Code = code
	return resp
}

// ErrorForbiddenMessage returns a 403 Forbidden with a user-facing message
// string. See ErrorInvalidRequestMessage for why.
func ErrorForbiddenMessage(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		Status:         "error",
		ErrorText:      message,
	}
}

// ErrorForbiddenMessageWithCode returns a 403 with a user-facing message AND a
// stable code. The message reaches the browser verbatim, so it must be the
// German sentence, not the Go sentinel: the frontend renders an unrecognized
// error text as-is.
func ErrorForbiddenMessageWithCode(message, code string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		Status:         "error",
		ErrorText:      message,
		Code:           code,
	}
}

// ErrorNotFoundMessage returns a 404 with a user-facing message string. Same
// reason as ErrorInvalidRequestMessage.
func ErrorNotFoundMessage(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      message,
	}
}

// ErrorConflictMessageWithCode returns a 409 with a user-facing message AND a
// stable code, for conflicts the frontend has to branch on rather than merely
// display.
func ErrorConflictMessageWithCode(message, code string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      message,
		Code:           code,
	}
}

// ErrorTooManyRequests returns a 429 Too Many Requests error response
func ErrorTooManyRequests(err error) render.Renderer {
	return newErrResponse(http.StatusTooManyRequests, err)
}

// ErrorRequestTimeout returns a 408 Request Timeout response for request
// contexts whose deadline expired before the handler could complete.
func ErrorRequestTimeout(err error) render.Renderer {
	return newErrResponse(http.StatusRequestTimeout, err)
}

// ErrorClientClosed returns the de-facto 499 status for a client-canceled
// request. Keeping this below 500 avoids Sentry noise for disconnects.
func ErrorClientClosed(err error) render.Renderer {
	return newErrResponse(statusClientClosedRequest, err)
}

// ErrorServiceUnavailable returns a 503 Service Unavailable response. Used
// when a transient dependency (settings DB, MFA-credentials lookup, etc.)
// makes a security decision impossible and the safe behaviour is to refuse
// the request without globally locking other callers out.
func ErrorServiceUnavailable(err error) render.Renderer {
	return newErrResponse(http.StatusServiceUnavailable, err)
}

// IsTransientDatabaseError reports whether err represents a temporary database
// connectivity failure rather than a domain validation error. Callers can use
// this to retry a whole transaction once or return 503 after retry exhaustion.
func IsTransientDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		return len(code) >= 2 && code[:2] == "08"
	}

	return strings.Contains(err.Error(), "driver: bad connection")
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

// RequireDependency writes a 503 response built from unavailableErr when ok
// is false and returns ok unchanged. Handlers use it to guard optional
// service dependencies:
//
//	if !common.RequireDependency(w, r, rs.MFAService != nil, errMFAServiceUnavailable) { return }
func RequireDependency(w http.ResponseWriter, r *http.Request, ok bool, unavailableErr error) bool {
	if !ok {
		RenderError(w, r, newErrResponse(http.StatusServiceUnavailable, unavailableErr))
	}
	return ok
}
