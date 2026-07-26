package common

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
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
		gr.Use(jwt.TenantMiddleware)
		fn(gr, tenant.TenantTxMiddleware(db))
	})
}
