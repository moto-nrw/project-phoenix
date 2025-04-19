package activity

import (
	"net/http"

	"github.com/go-chi/render"
)

// ErrResponse renderer type for handling all sorts of errors.
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	StatusText string `json:"status"`
	AppCode    int64  `json:"code,omitempty"`
	ErrorText  string `json:"error,omitempty"`
}

// Render sets the application-specific error code in AppCode.
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrInvalidRequest returns status 422 Unprocessable Entity including error message.
func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnprocessableEntity,
		StatusText:     "Invalid request.",
		ErrorText:      err.Error(),
	}
}

// ErrInternalServerError returns status 500 Internal Server Error.
func ErrInternalServerError(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal server error.",
		ErrorText:      err.Error(),
	}
}

// ErrNotFound returns status 404 Not Found for the request.
var ErrNotFound = &ErrResponse{
	HTTPStatusCode: http.StatusNotFound,
	StatusText:     "Resource not found.",
}

// ErrForbidden returns status 403 Forbidden.
var ErrForbidden = &ErrResponse{
	HTTPStatusCode: http.StatusForbidden,
	StatusText:     "Access denied.",
}

// ErrConflict returns status 409 Conflict for the request.
var ErrConflict = &ErrResponse{
	HTTPStatusCode: http.StatusConflict,
	StatusText:     "Resource conflict.",
}
