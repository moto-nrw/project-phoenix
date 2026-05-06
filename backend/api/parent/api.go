// Package parent holds the cross-tenant parent-portal HTTP layer.
//
// All routes mounted by this Resource sit under /parent on the API
// surface (e.g. /parent/auth/login, /parent/me/children). They share
// two properties:
//
//  1. Authenticated routes require a parent-scope JWT (scope="parent")
//     enforced by jwt.ParentMiddleware. Tenant + operator tokens are
//     hard-rejected with 401 — this is the symmetric guard to the
//     tenant middleware's ScopeParent rejection.
//  2. Public routes (just /auth/login for now) issue parent-scope
//     tokens that are not bound to any single tenant. The login
//     refuses accounts that don't have a guardian role on at least
//     one school they're mapped to.
//
// PR 9 commit 4 adds /auth/login. Commit 5 will add /me/children.
package parent

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// Resource bundles the parent-portal HTTP handlers + their deps.
type Resource struct {
	AuthService authService.AuthService
	db          *bun.DB
}

// NewResource builds the parent-portal resource.
func NewResource(auth authService.AuthService, db *bun.DB) *Resource {
	return &Resource{
		AuthService: auth,
		db:          db,
	}
}

// Router returns the chi router scoped to /parent.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	// Public auth routes. parent-credentials login issues a
	// parent-scope JWT that's NOT bound to any tenant_id; per-action
	// tenant resolution happens at downstream parent endpoints from
	// the URL or the picked child.
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", rs.login)
	})

	// Authenticated parent routes — all require scope=parent.
	// jwt.ParentMiddleware rejects tenant + operator tokens.
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.ParentMiddleware)

		// Routes added by commit 5 (cross-tenant children list, etc.)
	})

	return r
}
