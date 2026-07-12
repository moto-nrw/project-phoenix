package feedback

import (
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

// statusText adapts newErrResponse to the ErrorRule.Render shape.
func statusText(status int, text string) func(error) render.Renderer {
	return func(err error) render.Renderer { return newErrResponse(status, text, err) }
}

var errorRules = []common.ErrorRule{
	{Target: feedbackSvc.ErrEntryNotFound, Render: statusText(http.StatusNotFound, "Resource Not Found")},
	{Target: feedbackSvc.ErrInvalidEntryData, Render: statusText(http.StatusBadRequest, "Invalid Feedback Data")},
	{Target: feedbackSvc.ErrInvalidDateRange, Render: statusText(http.StatusBadRequest, "Invalid Date Range")},
	{Target: feedbackSvc.ErrStudentNotFound, Render: statusText(http.StatusNotFound, "Student Not Found")},
}

// ErrorRenderer returns a render.Renderer for the given error
var ErrorRenderer = common.RulesRenderer(errorRules, statusText(http.StatusInternalServerError, "Internal Server Error"))

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
