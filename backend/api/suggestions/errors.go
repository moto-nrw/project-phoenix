package suggestions

import (
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

// statusText adapts newErrResponse to the ErrorRule.Render shape.
func statusText(status int, text string) func(error) render.Renderer {
	return func(err error) render.Renderer { return newErrResponse(status, text, err) }
}

var errorRules = []common.ErrorRule{
	{Target: suggestionsSvc.ErrPostNotFound, Render: statusText(http.StatusNotFound, "Not Found")},
	{Target: suggestionsSvc.ErrCommentNotFound, Render: statusText(http.StatusNotFound, "Not Found")},
	{Target: suggestionsSvc.ErrForbidden, Render: statusText(http.StatusForbidden, "Forbidden")},
	{Target: suggestionsSvc.ErrInvalidData, Render: statusText(http.StatusBadRequest, "Bad Request")},
}

// ErrorRenderer returns a render.Renderer for the given error
var ErrorRenderer = common.RulesRenderer(errorRules, statusText(http.StatusInternalServerError, "Internal Server Error"))

// ErrorInvalidRequest returns an error response for invalid requests
func ErrorInvalidRequest(err error) render.Renderer {
	return newErrResponse(http.StatusBadRequest, "Invalid Request", err)
}
