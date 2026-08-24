// Package school holds the school-portal HTTP layer (#2207) — the fourth
// portal next to tenant, operator, and parents ("moto schule").
//
// All routes mounted by this Resource sit under /school on the API surface
// (e.g. /school/auth/login, /school/class-day). They share two properties:
//
//  1. Authenticated routes require a school-scope JWT (scope="school")
//     enforced by jwt.SchoolMiddleware. Tenant, org, platform, and parent
//     tokens are hard-rejected with 401 — the symmetric guard to the tenant
//     middleware's ScopeSchool rejection.
//  2. School tokens are tenant-bound (unlike parent tokens): the login pins
//     the school where the account holds a school-portal role, so the
//     class-day surface keeps running under the regular tenant transaction
//     and RLS.
//
// The class-day handlers themselves live in api/classday and are mounted
// here via SchoolRouter — one implementation, two scope mantles, until the
// tenant-portal mount is removed at cutover.
package school

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"

	classdayAPI "github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// Resource bundles the school-portal HTTP handlers + their deps.
type Resource struct {
	AuthService     authService.AuthService
	MFAService      authService.MFAService
	ClassDay        *classdayAPI.Resource
	authRateLimiter func(http.Handler) http.Handler
}

// NewResource builds the school-portal resource.
func NewResource(
	auth authService.AuthService,
	mfa authService.MFAService,
	classDay *classdayAPI.Resource,
) *Resource {
	return &Resource{
		AuthService: auth,
		MFAService:  mfa,
		ClassDay:    classDay,
	}
}

// SetAuthRateLimiter sets the rate limiter middleware for the public school
// auth endpoints. Mirrors the tenant, operator, and parent wiring in
// api/base.go so brute-force attempts return 429.
func (rs *Resource) SetAuthRateLimiter(mw func(http.Handler) http.Handler) {
	rs.authRateLimiter = mw
}

// Router returns the chi router scoped to /school.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	r.Route("/auth", func(r chi.Router) {
		// Public auth routes. The school login issues a school-scope JWT
		// pinned to the first school where the account holds a
		// school-portal role; token refresh and logout go through the
		// shared scope-preserving /auth/refresh and /auth/logout.
		r.Group(func(r chi.Router) {
			if rs.authRateLimiter != nil {
				r.Use(rs.authRateLimiter)
			}
			r.Post("/login", rs.login)
			r.Post("/password-reset", rs.initiatePasswordReset)
			r.Post("/password-reset/confirm", rs.resetPassword)
			// School MFA exchange: redeems ONLY challenges started with
			// the school challenge scope and mints school-scope tokens.
			r.Post("/mfa/verify", rs.mfaVerify)
			r.Post("/mfa/resend", rs.mfaResend)
		})

		// Enrollment-only routes: accept the narrow SCHOOL-scope
		// enrollment JWT the school login mints for accounts on an
		// MFA-required school that have no credential yet. Own group with
		// the dedicated enrollment authenticator so the enrollment token
		// never reaches a fully authenticated handler; the handlers pin
		// the school enrollment scope on top.
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
			r.Use(jwt.MFAEnrollmentAuthenticator)
			r.Route("/mfa/enroll", func(r chi.Router) {
				r.Post("/start", rs.mfaEnrollStart)
				r.Post("/confirm", rs.mfaEnrollConfirm)
			})
		})

		// Protected school-scope routes.
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
			r.Use(jwt.Authenticator)
			r.Use(jwt.SchoolMiddleware)

			// School switching for Lehrkraft accounts mapped to several
			// schools — the school-portal sibling of /auth/switch-tenant.
			r.Post("/switch-school", rs.switchSchool)
		})
	})

	// The class-day surface, reachable with school tokens. Same handlers
	// and permission gate as the (transitional) tenant-portal mount.
	r.Mount("/class-day", rs.ClassDay.SchoolRouter())

	return r
}
