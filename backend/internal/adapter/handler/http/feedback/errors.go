package feedback

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
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
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
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
	case errors.Is(err, feedbackSvc.ErrEntryNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Resource Not Found"

	case errors.Is(err, feedbackSvc.ErrInvalidEntryData):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Invalid Feedback Data"

	case errors.Is(err, feedbackSvc.ErrInvalidDateRange):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.StatusText = "Invalid Date Range"

	case errors.Is(err, feedbackSvc.ErrStudentNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.StatusText = "Student Not Found"
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
