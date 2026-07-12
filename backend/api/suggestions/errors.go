package suggestions

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
)

// newErrResponse builds a common.ErrResponse carrying this package's
// historical human-readable status classification (e.g. "Not Found")
// instead of api/common's literal "error". Wire bytes pinned by
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
	renderer := newErrResponse(http.StatusInternalServerError, "Internal Server Error", err)

	switch {
	case errors.Is(err, suggestionsSvc.ErrPostNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Not Found"
	case errors.Is(err, suggestionsSvc.ErrCommentNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Not Found"
	case errors.Is(err, suggestionsSvc.ErrForbidden):
		renderer.HTTPStatusCode = http.StatusForbidden
		renderer.Status = "Forbidden"
	case errors.Is(err, suggestionsSvc.ErrInvalidData):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Bad Request"
	}

	return renderer
}

// ErrorInvalidRequest returns an error response for invalid requests
func ErrorInvalidRequest(err error) render.Renderer {
	return newErrResponse(http.StatusBadRequest, "Invalid Request", err)
}
