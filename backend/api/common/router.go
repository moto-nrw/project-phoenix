package common

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// Middleware is the standard chi middleware shape.
type Middleware = func(http.Handler) http.Handler

// ProtectedTenantGroup registers a route group behind the standard
// JWT + tenant middleware chain (Verifier → Authenticator → TenantMiddleware)
// and hands the callback the tenant-transaction middleware for per-route use.
//
// withTx is passed to the callback instead of being applied group-wide on
// purpose: permission middleware must run before the tenant transaction is
// opened (a group-level Use would open tenant transactions on 403s), so
// routes attach it per-route via r.With(..., withTx).
func ProtectedTenantGroup(r chi.Router, db *bun.DB, fn func(r chi.Router, withTx Middleware)) {
	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(gr chi.Router) {
		gr.Use(tokenAuth.Verifier())
		gr.Use(jwt.Authenticator)
		// Write block for admin staff-view preview tokens (#2893). Directly
		// after the Authenticator: it only reads the parsed claims and must
		// reject before any transaction middleware can run.
		gr.Use(ReadOnlyPreviewMiddleware)
		gr.Use(jwt.TenantMiddleware)
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

// ProtectedSchoolGroup is the school-portal sibling of ProtectedTenantGroup
// (#2207): identical chain, but with jwt.SchoolMiddleware gating the group to
// school-scope tokens. School tokens are tenant-bound, so the tenant
// transaction middleware works unchanged — SchoolMiddleware puts the pinned
// tenant id on the context exactly like TenantMiddleware does.
func ProtectedSchoolGroup(r chi.Router, db *bun.DB, fn func(r chi.Router, withTx Middleware)) {
	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(gr chi.Router) {
		gr.Use(tokenAuth.Verifier())
		gr.Use(jwt.Authenticator)
		// Preview tokens are never school-scope, so SchoolMiddleware already
		// rejects them — this is defense-in-depth mirroring the tenant group.
		gr.Use(ReadOnlyPreviewMiddleware)
		gr.Use(jwt.SchoolMiddleware)
		gr.Use(SecurityPrincipalMiddleware)
		gr.Use(RequestSettingsCacheMiddleware)
		gr.Use(RequestIdentityCacheMiddleware)
		fn(gr, TenantTxMiddleware)
	})
}
