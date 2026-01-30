package suggestions

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
)

// ErrResponse renderer type for handling errors
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	StatusText string `json:"status"`
	ErrorText  string `json:"error,omitempty"`
}

// Render sets the specific error code for the response
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorRenderer returns a render.Renderer for the given error
func ErrorRenderer(err error) render.Renderer {
	renderer := &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal Server Error",
		ErrorText:      err.Error(),
	}

	switch {
	case errors.Is(err, suggestionsSvc.ErrPostNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Not Found"
	case errors.Is(err, suggestionsSvc.ErrForbidden):
		renderer.HTTPStatusCode = http.StatusForbidden
		renderer.StatusText = "Forbidden"
	case errors.Is(err, suggestionsSvc.ErrInvalidData):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Bad Request"
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
