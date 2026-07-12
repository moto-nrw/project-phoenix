package feedback

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
)

// newErrResponse builds a common.ErrResponse carrying this package's
// historical human-readable status classification (e.g. "Invalid Feedback
// Data") instead of api/common's literal "error". Wire bytes pinned by
// wire_format_test.go (issue #575 B1).
func newErrResponse(status int, statusText string, err error) *common.ErrResponse {
	return &common.ErrResponse{
		Err:            err,
		HTTPStatusCode: status,
		Status:         statusText,
		ErrorText:      err.Error(),
	}
}

// ErrorRenderer returns a render.Renderer for the given error
func ErrorRenderer(err error) render.Renderer {
	// Default to internal server error
	renderer := newErrResponse(http.StatusInternalServerError, "Internal Server Error", err)

	// Handle specific error types
	switch {
	case errors.Is(err, feedbackSvc.ErrEntryNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Resource Not Found"

	case errors.Is(err, feedbackSvc.ErrInvalidEntryData):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Invalid Feedback Data"

	case errors.Is(err, feedbackSvc.ErrInvalidDateRange):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Invalid Date Range"

	case errors.Is(err, feedbackSvc.ErrStudentNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Student Not Found"
	}

	return renderer
}

// ErrorInvalidRequest returns an error response for invalid requests
func ErrorInvalidRequest(err error) render.Renderer {
	return newErrResponse(http.StatusBadRequest, "Invalid Request", err)
}

// ErrorInternalServer returns an error response for server errors
func ErrorInternalServer(err error) render.Renderer {
	return newErrResponse(http.StatusInternalServerError, "Internal Server Error", err)
}

// ErrorForbidden returns an error response for forbidden requests
func ErrorForbidden(err error) render.Renderer {
	return newErrResponse(http.StatusForbidden, "Forbidden", err)
}
