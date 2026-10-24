package pwa

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Resource owns the PWA route shape while the composition root supplies the
// authenticated handler and transaction middleware.
type Resource struct {
	report http.Handler
	withTx func(http.Handler) http.Handler
}

func NewResource(report http.Handler, withTx func(http.Handler) http.Handler) *Resource {
	return &Resource{report: report, withTx: withTx}
}

func (rs *Resource) Register(router chi.Router) {
	router.With(rs.withTx).Method(http.MethodPost, "/usage", rs.report)
}
