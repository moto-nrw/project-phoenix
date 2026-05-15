package groups

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// ErrorResponse represents an HTTP error response
type ErrorResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string `json:"status"`
	ErrorText string `json:"error,omitempty"`
}

// Render implements the render.Renderer interface
func (e *ErrorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorRenderer renders an error to an HTTP response.
//
// User-facing branches render the inner sentinel directly so the JSON
// `error` field carries just the (German) message instead of EducationError's
// "education: {Op}: …" prefix. The 500 fallback keeps the wrapped eduErr so
// server logs still capture the operation context. Same pattern as
// api/rooms/errors.go.
func ErrorRenderer(err error) render.Renderer {
	if eduErr, ok := err.(*education.EducationError); ok {
		inner := eduErr.Unwrap()
		switch inner {
		case education.ErrGroupNotFound:
			return ErrorNotFound(inner)
		case education.ErrDuplicateGroup:
			return ErrorConflict(inner)
		case education.ErrRoomNotFound:
			return ErrorInvalidRequest(inner)
		case education.ErrTeacherNotFound:
			return ErrorInvalidRequest(inner)
		case education.ErrGroupTeacherNotFound:
			return ErrorNotFound(inner)
		case education.ErrGroupHasStudents:
			return ErrorConflict(inner)
		default:
			return ErrorInternalServer(eduErr)
		}
	}

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

// ErrorInternalServer returns a 500 Internal Server Error
func ErrorInternalServer(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}
