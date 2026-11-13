//go:build testhooks

package studentdeletionhttp

import (
	"net/http"

	"github.com/go-chi/render"
)

// StudentDeletionErrorRendererForTest classifies err the way the delete
// routes do.
func (rs *Resource) StudentDeletionErrorRendererForTest(err error) render.Renderer {
	return rs.studentDeletionErrorRenderer(err)
}

// WithdrawalDeletionErrorRendererForTest classifies err the way the care-exit
// routes do.
func (rs *Resource) WithdrawalDeletionErrorRendererForTest(err error) render.Renderer {
	return rs.withdrawalDeletionErrorRenderer(err)
}

// RenderErrorForTest writes errorResponse the way the routes do.
func RenderErrorForTest(w http.ResponseWriter, r *http.Request, errorResponse render.Renderer) {
	renderError(w, r, errorResponse)
}
