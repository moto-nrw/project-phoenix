package common

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Middleware is the standard chi middleware shape.
type Middleware = func(http.Handler) http.Handler

// The scope gates of the token adapter, bound to the tenant runtime. Each
// accepts exactly one portal's tokens and must follow jwt.Authenticator.

// TenantScopeMiddleware rejects parent and school tokens and binds the tenant.
func TenantScopeMiddleware(next http.Handler) http.Handler {
	return jwt.TenantMiddleware(tenant.ClaimScope{})(next)
}

// ParentScopeMiddleware accepts parents-portal tokens only and binds no tenant.
func ParentScopeMiddleware(next http.Handler) http.Handler {
	return jwt.ParentMiddleware(tenant.ClaimScope{})(next)
}

// SchoolScopeMiddleware accepts school-portal tokens only and binds their school.
func SchoolScopeMiddleware(next http.Handler) http.Handler {
	return jwt.SchoolMiddleware(tenant.ClaimScope{})(next)
}

// ProtectedTenantRoutes registers the tenant security chain and supplies the
// request-scoped transaction middleware without retaining a database handle.
func ProtectedTenantRoutes(r chi.Router, fn func(r chi.Router, withTx Middleware)) {

	r.Group(func(gr chi.Router) {
		gr.Use(jwt.Authenticator)
		// Write block for admin staff-view preview tokens (#2893). Directly
		// after the Authenticator: it only reads the parsed claims and must
		// reject before any transaction middleware can run.
		gr.Use(ReadOnlyPreviewMiddleware)
		gr.Use(TenantScopeMiddleware)
		gr.Use(SecurityPrincipalMiddleware)
		// Request-scoped settings memo cache (issue #2065) and identity memo
		// cache (issue #2099). Unlike withTx these ARE applied group-wide:
		// they open no transaction and do no DB work, so running them on
		// requests that later 403 costs one map allocation each.
		gr.Use(RequestSettingsCacheMiddleware)
		gr.Use(RequestIdentityCacheMiddleware)
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
func ProtectedParentGroup(r chi.Router, fn func(r chi.Router)) {

	r.Group(func(gr chi.Router) {
		gr.Use(jwt.Authenticator)
		gr.Use(ParentScopeMiddleware)
		fn(gr)
	})
}

// ProtectedSchoolRoutes registers the school-portal security chain and
// supplies the request-scoped transaction middleware without retaining a
// database handle.
func ProtectedSchoolRoutes(r chi.Router, fn func(r chi.Router, withTx Middleware)) {

	r.Group(func(gr chi.Router) {
		gr.Use(jwt.Authenticator)
		// Preview tokens are never school-scope, so SchoolMiddleware already
		// rejects them — this is defense-in-depth mirroring the tenant group.
		gr.Use(ReadOnlyPreviewMiddleware)
		gr.Use(SchoolScopeMiddleware)
		gr.Use(SecurityPrincipalMiddleware)
		gr.Use(RequestSettingsCacheMiddleware)
		gr.Use(RequestIdentityCacheMiddleware)
		fn(gr, TenantTxMiddleware)
	})
}
