package common

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Middleware is the standard chi middleware shape.
type Middleware = func(http.Handler) http.Handler

// SessionChain is the order every session route group runs, whatever its
// portal: authenticate; block the writes of a staff-view preview token
// (#2893) before any transaction can open; admit only the group's portals;
// then derive the security principal from the admitted claims.
func SessionChain(authenticate, preview, scope, principal Middleware) []Middleware {
	return []Middleware{authenticate, preview, scope, principal}
}

// ProtectedTenantRoutes registers a session route group of a school-bound
// portal and supplies the request-scoped transaction middleware without
// retaining a database handle. The request-scoped settings memo cache
// (#2065) and identity memo cache (#2099) apply group-wide: they open no
// transaction and do no database work.
func ProtectedTenantRoutes(r chi.Router, fn func(r chi.Router, withTx Middleware), chain []Middleware, settingsCache, identityCache Middleware) {
	r.Group(func(gr chi.Router) {
		gr.Use(chain...)
		gr.Use(settingsCache, identityCache)
		fn(gr, TenantTxMiddleware)
	})
}
