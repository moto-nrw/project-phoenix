// Package schoolportal holds the school-portal HTTP layer (#2207) — the fourth
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
// The class-day handlers themselves live in modules/classday/http and are mounted
// here via SchoolRouter — one implementation, two scope mantles, until the
// tenant-portal mount is removed at cutover.
package schoolportal

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	classdayAPI "github.com/moto-nrw/project-phoenix/modules/classday/http"
	notificationsAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/notifications"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// StaffMessagingRouter is the school-portal mount supplied by the application
// composition root. The school HTTP package must not depend on a sibling HTTP
// adapter.
type StaffMessagingRouter interface {
	SchoolRouter() chi.Router
}

// StaffNoticesRouter is the school-portal mount of the Tagesinformationen
// (#2208), supplied by the composition root for the same reason as
// StaffMessagingRouter.
type StaffNoticesRouter interface {
	SchoolRouter() chi.Router
}

// Resource bundles the school-portal HTTP handlers + their deps.
type Resource struct {
	AuthService    authService.AuthService
	MFAService     authService.MFAService
	ClassDay       *classdayAPI.Resource
	Timetable      *timetableAPI.Resource
	StaffMessaging StaffMessagingRouter
	StaffNotices   StaffNoticesRouter
	Notifications  *notificationsAPI.Resource
}

// NewResource builds the school-portal resource.
func NewResource(
	auth authService.AuthService,
	mfa authService.MFAService,
	classDay *classdayAPI.Resource,
	timetable *timetableAPI.Resource,
	staffMessaging StaffMessagingRouter,
	staffNotices StaffNoticesRouter,
	notifications *notificationsAPI.Resource,
) *Resource {
	return &Resource{
		AuthService:    auth,
		MFAService:     mfa,
		ClassDay:       classDay,
		Timetable:      timetable,
		StaffMessaging: staffMessaging,
		StaffNotices:   staffNotices,
		Notifications:  notifications,
	}
}

// Router returns the chi router scoped to /school without a rate limiter on
// the public auth endpoints; tests drive it directly.
func (rs *Resource) Router() chi.Router {
	return rs.RouterWithAuthRateLimiter(nil)
}

// RouterWithAuthRateLimiter returns the chi router scoped to /school with the
// given rate limiter middleware on the public school auth endpoints. Mirrors
// the tenant, operator, and parent wiring in api/base.go so brute-force
// attempts return 429; nil mounts the routes unthrottled.
func (rs *Resource) RouterWithAuthRateLimiter(authRateLimiter func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	r.Route("/auth", func(r chi.Router) {
		// Public auth routes. The school login issues a school-scope JWT
		// pinned to the first school where the account holds a
		// school-portal role; token refresh and logout go through the
		// shared scope-preserving /auth/refresh and /auth/logout.
		r.Group(func(r chi.Router) {
			if authRateLimiter != nil {
				r.Use(authRateLimiter)
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
			r.Use(common.ReadOnlyPreviewMiddleware)
			r.Use(jwt.SchoolMiddleware)
			r.Use(common.SecurityPrincipalMiddleware)

			// School switching for Lehrkraft accounts mapped to several
			// schools — the school-portal sibling of /auth/switch-tenant.
			r.Post("/switch-school", rs.switchSchool)
		})
	})

	// The class-day surface, reachable with school tokens. Same handlers
	// and permission gate as the (transitional) tenant-portal mount.
	r.Mount("/class-day", rs.ClassDay.SchoolRouter())

	// The assignment-bound supervision surface (#2527): the Betreuungsplan
	// blocks this Lehrkraft is personally planned into today, and nothing
	// else. Same operations handlers as the OGS portal, a narrower mantle —
	// see timetable.SchoolSupervisionRouter.
	if rs.Timetable != nil {
		r.Mount("/supervisions", rs.Timetable.SchoolSupervisionRouter())
	}

	// Team-Chat for Lehrkräfte (#2208): the same 1:1 conversations as the
	// OGS portal's /api/staff-messages, reached with a school token. One
	// service, one thread store — a Lehrkraft and a Betreuungskraft read the
	// same conversation from their respective portals.
	if rs.StaffMessaging != nil {
		r.Mount("/staff-messages", rs.StaffMessaging.SchoolRouter())
	}

	// Tagesinformationen for Lehrkräfte (#2208): the notices the OGS
	// leadership addressed to "alle" or "nur Lehrkräfte", read and
	// acknowledged with a school token behind staff_notices:read. Writing
	// stays in the OGS portal.
	if rs.StaffNotices != nil {
		r.Mount("/staff-notices", rs.StaffNotices.SchoolRouter())
	}

	// Own notification decisions and devices (#2208): the same handlers as
	// /api/notifications, narrowed to the school catalogue and recording
	// devices with portal "school".
	if rs.Notifications != nil {
		r.Mount("/notifications", rs.Notifications.SchoolRouter())
	}

	return r
}
