package activities

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

// ErrorResponse represents an HTTP error response
type ErrorResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string `json:"status"`
	ErrorText string `json:"error,omitempty"`
	Code      string `json:"code,omitempty"`
}

const errorCodeOnlySupervisorReplacementRequired = "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED"

// Render implements the render.Renderer interface
func (e *ErrorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorRenderer renders an error to an HTTP response
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific activity service error
	if actErr, ok := err.(*activities.ActivityError); ok {
		// Map specific activity service errors to appropriate HTTP status codes
		switch {
		case errors.Is(actErr, activities.ErrCategoryNotFound):
			return ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrGroupNotFound):
			return ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrScheduleNotFound):
			return ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrSupervisorNotFound):
			return ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrEnrollmentNotFound):
			return ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrGroupFull):
			return ErrorConflict(actErr)
		case errors.Is(actErr, activities.ErrAlreadyEnrolled):
			return ErrorConflict(actErr)
		case errors.Is(actErr, activities.ErrOnlySupervisorRequiresReplacement):
			return ErrorConflictWithCode(actErr, errorCodeOnlySupervisorReplacementRequired)
		case errors.Is(actErr, activities.ErrNotEnrolled):
			return ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrSystemActivityProtected):
			return ErrorForbidden(actErr)
		case errors.Is(actErr, activities.ErrGroupClosed):
			return ErrorForbidden(actErr)
		case errors.Is(actErr, activities.ErrInvalidAttendanceStatus):
			return ErrorInvalidRequest(actErr)
		case errors.Is(actErr, activities.ErrStaffNotFound):
			return ErrorNotFound(actErr)
		default:
			return ErrorInternalServer(actErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
}

// ErrorInvalidRequest returns a 400 Bad Request error
func ErrorInvalidRequest(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorForbidden returns a 403 Forbidden error
func ErrorForbidden(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorNotFound returns a 404 Not Found error
func ErrorNotFound(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorConflict returns a 409 Conflict error
func ErrorConflict(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorConflictWithCode returns a 409 Conflict error with a stable error code.
func ErrorConflictWithCode(err error, code string) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
		Code:           code,
	}
}

// ErrorInternalServer returns a 500 Internal Server Error
func ErrorInternalServer(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}
