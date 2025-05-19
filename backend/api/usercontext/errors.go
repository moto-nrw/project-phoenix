package usercontext

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// ErrorRenderer maps usercontext service errors to HTTP error responses
func ErrorRenderer(err error) render.Renderer {
	// Unwrap the error to check if it's a usercontext error
	var ucErr *usercontext.UserContextError
	if errors.As(err, &ucErr) {
		// Check for specific error types
		switch {
		case errors.Is(err, usercontext.ErrUserNotAuthenticated):
			return common.ErrorUnauthorized(err)
		case errors.Is(err, usercontext.ErrUserNotAuthorized):
			return common.ErrorForbidden(err)
		case errors.Is(err, usercontext.ErrUserNotFound):
			return common.ErrorNotFound(err)
		case errors.Is(err, usercontext.ErrUserNotLinkedToPerson):
			return common.ErrorNotFound(err)
		case errors.Is(err, usercontext.ErrUserNotLinkedToStaff):
			return common.ErrorNotFound(err)
		case errors.Is(err, usercontext.ErrUserNotLinkedToTeacher):
			return common.ErrorNotFound(err)
		case errors.Is(err, usercontext.ErrGroupNotFound):
			return common.ErrorNotFound(err)
		case errors.Is(err, usercontext.ErrNoActiveGroups):
			return common.ErrorNotFound(err)
		case errors.Is(err, usercontext.ErrInvalidOperation):
			return common.ErrorInvalidRequest(err)
		default:
			// For general service errors, return internal server error
			return &common.ErrResponse{
				Err:            err,
				HTTPStatusCode: http.StatusInternalServerError,
				Status:         "error",
				ErrorText:      err.Error(),
			}
		}
	}

	// If it's not a usercontext error, just return a generic error
	return &common.ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}
