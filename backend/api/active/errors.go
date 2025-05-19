package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// ErrResponse renderer type for handling all sorts of errors.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int    `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

// Render sets the specific error code for the response
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorRenderer returns a render.Renderer for the given error
func ErrorRenderer(err error) render.Renderer {
	// Default to internal server error
	renderer := &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal Server Error",
		ErrorText:      err.Error(),
	}

	// Handle specific error types
	switch {
	case errors.Is(err, activeSvc.ErrActiveGroupNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Active Group Not Found"

	case errors.Is(err, activeSvc.ErrVisitNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Visit Not Found"

	case errors.Is(err, activeSvc.ErrGroupSupervisorNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Group Supervisor Not Found"

	case errors.Is(err, activeSvc.ErrCombinedGroupNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Combined Group Not Found"

	case errors.Is(err, activeSvc.ErrGroupMappingNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Group Mapping Not Found"

	case errors.Is(err, activeSvc.ErrInvalidData):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Invalid Data"

	case errors.Is(err, activeSvc.ErrActiveGroupAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Active Group Already Ended"

	case errors.Is(err, activeSvc.ErrVisitAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Visit Already Ended"

	case errors.Is(err, activeSvc.ErrSupervisionAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Supervision Already Ended"

	case errors.Is(err, activeSvc.ErrCombinedGroupAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Combined Group Already Ended"

	case errors.Is(err, activeSvc.ErrGroupAlreadyInCombination):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Group Already In Combination"

	case errors.Is(err, activeSvc.ErrStudentAlreadyInGroup):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Student Already In Group"

	case errors.Is(err, activeSvc.ErrStudentAlreadyActive):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Student Already Has Active Visit"

	case errors.Is(err, activeSvc.ErrStaffAlreadySupervising):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Staff Already Supervising This Group"

	case errors.Is(err, activeSvc.ErrCannotDeleteActiveGroup):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Cannot Delete Active Group With Active Visits"

	case errors.Is(err, activeSvc.ErrInvalidTimeRange):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Invalid Time Range"
	}

	return renderer
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
