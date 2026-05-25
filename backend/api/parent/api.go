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
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource bundles the parent-portal HTTP handlers + their deps.
type Resource struct {
	AuthService           authService.AuthService
	ParentService         parentService.Service
	RequestService        enrollmentService.RequestService
	GuardianProfileLoader usersService.GuardianProfileLoader
	SchoolRepo            platformModels.SchoolRepository
	AccountTenantRepo     authModels.AccountTenantRepository
	db                    *bun.DB
	authRateLimiter       func(http.Handler) http.Handler
}

// SetAuthRateLimiter sets the rate limiter middleware for the public parent
// login endpoint. Mirrors the tenant and operator wiring in api/base.go so
// brute-force attempts return 429 — which the frontend NextAuth provider
// translates into the localized "Zu viele Anmeldeversuche" error code.
func (rs *Resource) SetAuthRateLimiter(mw func(http.Handler) http.Handler) {
	rs.authRateLimiter = mw
}

// NewResource builds the parent-portal resource.
func NewResource(
	auth authService.AuthService,
	parent parentService.Service,
	requestSvc enrollmentService.RequestService,
	guardianProfileLoader usersService.GuardianProfileLoader,
	schoolRepo platformModels.SchoolRepository,
	accountTenantRepo authModels.AccountTenantRepository,
	db *bun.DB,
) *Resource {
	return &Resource{
		AuthService:           auth,
		ParentService:         parent,
		RequestService:        requestSvc,
		GuardianProfileLoader: guardianProfileLoader,
		SchoolRepo:            schoolRepo,
		AccountTenantRepo:     accountTenantRepo,
		db:                    db,
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
		if rs.authRateLimiter != nil {
			r.Use(rs.authRateLimiter)
		}
		r.Post("/login", rs.login)
	})

	// Authenticated parent routes — all require scope=parent.
	// jwt.ParentMiddleware rejects tenant + operator tokens.
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.ParentMiddleware)

		// Cross-tenant children list — every student the parent is
		// linked to, across every active tenant mapping. Account id
		// is read from claims, never from URL or body.
		r.Get("/me/children", rs.listMyChildren)

		// Cross-tenant enrollable schools list — every (school, open
		// phase) the parent could enroll a new child at, with a flag
		// for schools the parent already has children at. Powers the
		// "Neue Anmeldung" school picker in the parents app.
		r.Get("/me/enrollable-schools", rs.listEnrollableSchools)

		// Cross-tenant enrollment-request list — every
		// enrollment.requests row owned by this parent (matched by
		// guardian_account_id), joined to phase/school/children.
		// Powers the "Anmeldungen in Bearbeitung" section on the
		// dashboard.
		r.Get("/me/enrollments", rs.listMyEnrollments)

		// Per-school autofill payload for the embedded enrollment
		// form. Tenant resolved from the {tenantSlug} path segment;
		// account from the parent JWT. Returns guardian fields from
		// the guardian_profile in that tenant (or claims if none) +
		// any students already linked to the guardian profile.
		r.Get("/enrollments/{tenantSlug}/profile", rs.getEnrollmentProfile)

		// Authenticated submit. Stamps guardian_account_id on the
		// resulting enrollment.requests row, skips captcha (parent
		// already authenticated), and forwards to the same
		// RequestService.Submit the public path uses.
		r.Post("/enrollments/{tenantSlug}/submit", rs.submitParentEnrollment)
	})

	return r
}
