package common

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Middleware is the standard chi middleware shape.
type Middleware = func(http.Handler) http.Handler

// ProtectedTenantRoutes registers the tenant security chain and supplies the
// request-scoped transaction middleware without retaining a database handle.
func ProtectedTenantRoutes(r chi.Router, fn func(r chi.Router, withTx Middleware), authenticate, preview, scope, principal, settingsCache, identityCache Middleware) {

	r.Group(func(gr chi.Router) {
		gr.Use(authenticate)
		// Write block for admin staff-view preview tokens (#2893). Directly
		// after the Authenticator: it only reads the parsed claims and must
		// reject before any transaction middleware can run.
		gr.Use(preview)
		gr.Use(scope)
		gr.Use(principal)
		// Request-scoped settings memo cache (issue #2065) and identity memo
		// cache (issue #2099). Unlike withTx these ARE applied group-wide:
		// they open no transaction and do no DB work, so running them on
		// requests that later 403 costs one map allocation each.
		gr.Use(settingsCache)
		gr.Use(identityCache)
		fn(gr, TenantTxMiddleware)
	})
}

// ProtectedParentGroup registers a route group behind the parents-portal
// chain (Verifier → Authenticator → ParentMiddleware).
//
// There is no tenant middleware and no tenant transaction: a parent token is
// deliberately cross-tenant, because a guardian's children can attend several
// schools. Every handler in such a group must therefore resolve the school
// from the resource it was asked for, and open its own transaction for that
// school — never from the token.
func ProtectedParentGroup(r chi.Router, fn func(r chi.Router), authenticate, scope Middleware) {

	r.Group(func(gr chi.Router) {
		gr.Use(authenticate)
		gr.Use(scope)
		fn(gr)
	})
}

// ProtectedSchoolRoutes registers the school-portal security chain and
// supplies the request-scoped transaction middleware without retaining a
// database handle.
func ProtectedSchoolRoutes(r chi.Router, fn func(r chi.Router, withTx Middleware), authenticate, preview, scope, principal, settingsCache, identityCache Middleware) {

	r.Group(func(gr chi.Router) {
		gr.Use(authenticate)
		// Preview tokens are never school-scope, so SchoolMiddleware already
		// rejects them — this is defense-in-depth mirroring the tenant group.
		gr.Use(preview)
		gr.Use(scope)
		gr.Use(principal)
		gr.Use(settingsCache)
		gr.Use(identityCache)
		fn(gr, TenantTxMiddleware)
	})
}
