// Package schoolportal holds the school-portal HTTP layer (#2207) — the fourth
// portal next to tenant, operator, and parents ("moto schule").
//
// All routes mounted by this Resource sit under /school on the API surface
// (e.g. /school/auth/login, /school/class-day). They share two properties:
//
//  1. Authenticated routes require a school-scope JWT (scope="school")
//     enforced by sessiontoken.SchoolMiddleware. Tenant, org, platform, and parent
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
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// StaffMessagingRouter is the school-portal mount supplied by the application
// composition root. The school HTTP package must not depend on a sibling HTTP
// adapter.
type StaffMessagingRouter interface {
	SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// StaffNoticesRouter is the school-portal mount of the Tagesinformationen
// (#2208), supplied by the composition root for the same reason as
// StaffMessagingRouter.
type StaffNoticesRouter interface {
	SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// SchoolRouter is a school-scoped route resource supplied by Root.
// The portal host does not import the HTTP adapters it mounts.
type SchoolRouter interface {
	SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// SupervisionsRouter is the assignment-bound supervision surface Root binds.
type SupervisionsRouter interface {
	SchoolSupervisionRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// NotificationRouter is the notification surface, whose own runtime already
// carries its session route assembly.
type NotificationRouter interface {
	SchoolRouter() chi.Router
}

// Resource bundles the school-portal HTTP handlers + their deps.
type Resource struct {
	// TenantContext is the enrollment-audit context Root binds from native runtime.
	TenantContext func(context.Context, int64) context.Context
	AuthService   AuthRuntime
	MFAService    MFARuntime
	// Resets is the reset runtime the composition root binds to Identity &
	// Access (#3332); the zero value leaves the reset routes answering 500.
	Resets         PasswordResetRuntime
	ClassDay       SchoolRouter
	Timetable      SupervisionsRouter
	StaffMessaging StaffMessagingRouter
	StaffNotices   StaffNoticesRouter
	Notifications  NotificationRouter
	// Calendar is the calendar of moto schule (#4798); nil leaves it
	// unmounted.
	Calendar SchoolRouter
}

// NewResource builds the school-portal resource. An incomplete resets
// runtime leaves the public reset routes answering 500, exactly as a service
// composed without the reset dependencies did.
func NewResource(
	auth AuthRuntime,
	mfa MFARuntime,
	resets PasswordResetRuntime,
	classDay SchoolRouter,
	timetable SupervisionsRouter,
	staffMessaging StaffMessagingRouter,
	staffNotices StaffNoticesRouter,
	notifications NotificationRouter,
	tenantContext func(context.Context, int64) context.Context,
) *Resource {
	return &Resource{
		TenantContext:  tenantContext,
		AuthService:    auth,
		MFAService:     mfa,
		Resets:         resets,
		ClassDay:       classDay,
		Timetable:      timetable,
		StaffMessaging: staffMessaging,
		StaffNotices:   staffNotices,
		Notifications:  notifications,
	}
}

// RouterWithAuthRateLimiter returns the chi router scoped to /school with the
// given rate limiter middleware on the public school auth endpoints. Mirrors
// the tenant, operator, and parent wiring in api/base.go so brute-force
// attempts return 429; nil mounts the routes unthrottled.
func (rs *Resource) RouterWithAuthRateLimiter(authRateLimiter func(http.Handler) http.Handler, configureSession func(chi.Router), protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	rs.mountAuthRoutes(r, authRateLimiter, configureSession)
	// The class-day surface, reachable with school tokens. Same handlers
	// and permission gate as the (transitional) tenant-portal mount.
	r.Mount("/class-day", rs.ClassDay.SchoolRouter(protectedSchoolRoutes))

	// The assignment-bound supervision surface (#2527): the Betreuungsplan
	// blocks this Lehrkraft is personally planned into today, and nothing
	// else. Same operations handlers as the OGS portal, a narrower mantle —
	// see timetable.SchoolSupervisionRouter.
	if rs.Timetable != nil {
		r.Mount("/supervisions", rs.Timetable.SchoolSupervisionRouter(protectedSchoolRoutes))
	}

	// Team-Chat for Lehrkräfte (#2208): the same 1:1 conversations as the
	// OGS portal's /api/staff-messages, reached with a school token. One
	// service, one thread store — a Lehrkraft and a Betreuungskraft read the
	// same conversation from their respective portals.
	if rs.StaffMessaging != nil {
		r.Mount("/staff-messages", rs.StaffMessaging.SchoolRouter(protectedSchoolRoutes))
	}

	// Tagesinformationen for Lehrkräfte (#2208): the notices the OGS
	// leadership addressed to "alle" or "nur Lehrkräfte", read and
	// acknowledged with a school token behind staff_notices:read. Writing
	// stays in the OGS portal.
	if rs.StaffNotices != nil {
		r.Mount("/staff-notices", rs.StaffNotices.SchoolRouter(protectedSchoolRoutes))
	}

	// Own notification decisions and devices (#2208): the same handlers as
	// /api/notifications, narrowed to the school catalogue and recording
	// devices with portal "school".
	if rs.Notifications != nil {
		r.Mount("/notifications", rs.Notifications.SchoolRouter())
	}

	// The shared calendar (#4798): appointments for Lehrkräfte, closing days
	// and holidays, the own Schultermine and the subscription feed, behind
	// calendar:school. Same calendar service as /api/calendar.
	if rs.Calendar != nil {
		r.Mount("/calendar", rs.Calendar.SchoolRouter(protectedSchoolRoutes))
	}

	return r
}

func (rs *Resource) mountAuthRoutes(r chi.Router, authRateLimiter func(http.Handler) http.Handler, configureSession func(chi.Router)) {
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
			r.Use(sessiontoken.MFAEnrollmentAuthenticator)
			r.Route("/mfa/enroll", func(r chi.Router) {
				r.Post("/start", rs.mfaEnrollStart)
				r.Post("/confirm", rs.mfaEnrollConfirm)
			})
		})

		// Protected school-scope routes.
		r.Group(func(r chi.Router) {
			configureSession(r)

			// School switching for Lehrkraft accounts mapped to several
			// schools — the school-portal sibling of /auth/switch-tenant.
			r.Post("/switch-school", rs.switchSchool)
		})
	})

}
