package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
)

// ErrResponse renderer type for handling all sorts of errors.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int    `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

// errorMapping defines how specific errors map to HTTP responses
type errorMapping struct {
	err        error
	statusCode int
	statusText string
}

// errorMappings is the table of error-to-response mappings
var errorMappings = []errorMapping{
	// Not Found errors
	{activeSvc.ErrActiveGroupNotFound, http.StatusNotFound, "Active Group Not Found"},
	{activeSvc.ErrVisitNotFound, http.StatusNotFound, "Visit Not Found"},
	{activeSvc.ErrGroupSupervisorNotFound, http.StatusNotFound, "Group Supervisor Not Found"},
	{activeSvc.ErrCombinedGroupNotFound, http.StatusNotFound, "Combined Group Not Found"},
	{activeSvc.ErrGroupMappingNotFound, http.StatusNotFound, "Group Mapping Not Found"},

	// Bad Request errors
	{activeSvc.ErrInvalidData, http.StatusBadRequest, "Invalid Data"},
	{activeSvc.ErrActiveGroupAlreadyEnded, http.StatusBadRequest, "Active Group Already Ended"},
	{activeSvc.ErrVisitAlreadyEnded, http.StatusBadRequest, "Visit Already Ended"},
	{activeSvc.ErrSupervisionAlreadyEnded, http.StatusBadRequest, "Supervision Already Ended"},
	{activeSvc.ErrCombinedGroupAlreadyEnded, http.StatusBadRequest, "Combined Group Already Ended"},
	{activeSvc.ErrGroupAlreadyInCombination, http.StatusBadRequest, "Group Already In Combination"},
	{activeSvc.ErrStudentAlreadyInGroup, http.StatusBadRequest, "Student Already In Group"},
	{activeSvc.ErrStudentAlreadyActive, http.StatusBadRequest, "Student Already Has Active Visit"},
	{activeSvc.ErrStaffAlreadySupervising, http.StatusBadRequest, "Staff Already Supervising This Group"},
	{activeSvc.ErrCannotDeleteActiveGroup, http.StatusBadRequest, "Cannot Delete Active Group With Active Visits"},
	{activeSvc.ErrInvalidTimeRange, http.StatusBadRequest, "Invalid Time Range"},

	// Conflict errors
	{activeSvc.ErrRoomConflict, http.StatusConflict, "Room Conflict"},
}

// Render sets the specific error code for the response
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorRenderer returns a render.Renderer for the given error
func ErrorRenderer(err error) render.Renderer {
	// Check error mappings table
	for _, m := range errorMappings {
		if errors.Is(err, m.err) {
			return &ErrResponse{
				Err:            err,
				HTTPStatusCode: m.statusCode,
				StatusText:     m.statusText,
				ErrorText:      err.Error(),
			}
		}
	}

	// Default to internal server error
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal Server Error",
		ErrorText:      err.Error(),
	}
}

// ErrorInvalidRequest returns an ErrResponse for invalid requests
func ErrorInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Invalid Request",
		ErrorText:      err.Error(),
	}
}

// ErrorInternalServer returns an ErrResponse for server errors
func ErrorInternalServer(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal Server Error",
		ErrorText:      err.Error(),
	}
}

// ErrorForbidden returns an ErrResponse for forbidden actions
func ErrorForbidden(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "Forbidden",
		ErrorText:      err.Error(),
	}
}

// ErrorUnauthorized returns an ErrResponse for unauthorized actions
func ErrorUnauthorized(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Unauthorized",
		ErrorText:      err.Error(),
	}
}
