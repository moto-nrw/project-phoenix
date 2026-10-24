package studentdeletionhttp

import (
	http "net/http"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
)

// renderError writes an error response to the HTTP response writer.
// Delegates to common.RenderError which logs 5xx root causes to slog
// and hands them to the central Sentry reporting.
func renderError(w http.ResponseWriter, r *http.Request, errorResponse render.Renderer) {
	common.RenderError(w, r, errorResponse)
}
