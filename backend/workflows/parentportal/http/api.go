// Package parent holds the cross-tenant parent-portal HTTP layer.
//
// All routes mounted by this Resource sit under /parent on the API
// surface (e.g. /parent/auth/login, /parent/me/children). They share
// two properties:
//
//  1. Authenticated routes require a parent-scope JWT (scope="parent")
//     enforced by sessiontoken.ParentMiddleware. Tenant + operator tokens are
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
	"github.com/go-chi/render"

	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Resource bundles the parent-portal HTTP handlers + their deps.
type Resource struct {
	// Auth is the login runtime the composition root binds to Identity &
	// Access (#3364); the zero value leaves the login route answering 500.
	Auth LoginRuntime
	// Resets is the reset runtime the composition root binds to Identity &
	// Access (#3332); the zero value leaves the reset routes answering 500.
	Resets                PasswordResetRuntime
	ParentService         PortalService
	RequestSharing        parentService.RequestSharingService
	CalendarService       calendarService.ParentCalendar
	EnrollmentForms       EnrollmentForms
	GuardianProfileLoader GuardianProfileLoader
	SchoolService         SchoolDirectory
	PushService           PushSubscriptions
	PreferenceService     NotificationPreferences
	PWAUsageService       PWAUsage
	// Reports renders the Erklärung proof PDF (#3430).
	Reports ReportRenderer
	// SupportedLocale is bound to the shared language catalog by the root.
	SupportedLocale func(string) bool
}

// ResourceConfig lists the parent-portal collaborators. The composition root
// supplies all of them at construction; tests set only the ones their routes
// reach.
type ResourceConfig struct {
	// Auth is the login runtime the composition root binds (#3364).
	Auth LoginRuntime
	// Resets is the reset runtime the composition root binds (#3332).
	Resets   PasswordResetRuntime
	Parent   PortalService
	Calendar calendarService.ParentCalendar
	// Enrollment is the public form flow the composition root binds (#2734).
	Enrollment            EnrollmentForms
	GuardianProfileLoader GuardianProfileLoader
	Schools               SchoolDirectory
	// Push is the Web Push subscription service (#2003).
	Push PushSubscriptions
	// Preferences is the notification consent service.
	Preferences NotificationPreferences
	// PWAUsage is the PWA standalone-usage service (#2189).
	PWAUsage PWAUsage
	// Reports renders the Erklärung proof PDF (#3430).
	Reports ReportRenderer
	// SupportedLocale is bound to the shared language catalog by the root.
	SupportedLocale func(string) bool
}

// NewResource builds the parent-portal resource.
func NewResource(cfg ResourceConfig) *Resource {
	var sharing parentService.RequestSharingService
	if cfg.Parent != nil {
		var ok bool
		sharing, ok = cfg.Parent.(parentService.RequestSharingService)
		if !ok {
			panic("parent resource requires a request sharing service")
		}
	}
	return &Resource{
		Auth:                  cfg.Auth,
		Resets:                cfg.Resets,
		ParentService:         cfg.Parent,
		RequestSharing:        sharing,
		CalendarService:       cfg.Calendar,
		EnrollmentForms:       cfg.Enrollment,
		GuardianProfileLoader: cfg.GuardianProfileLoader,
		SchoolService:         cfg.Schools,
		PushService:           cfg.Push,
		PreferenceService:     cfg.Preferences,
		PWAUsageService:       cfg.PWAUsage,
		Reports:               cfg.Reports,
		SupportedLocale:       cfg.SupportedLocale,
	}
}

// RouterWithAuthRateLimiter returns the chi router scoped to /parent with the
// given rate limiter middleware on the public parent auth endpoints. Mirrors
// the tenant and operator wiring in api/base.go so brute-force attempts return
// 429; nil mounts the routes unthrottled.
func (rs *Resource) RouterWithAuthRateLimiter(authRateLimiter func(http.Handler) http.Handler, configureSession func(chi.Router)) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public auth routes. parent-credentials login issues a
	// parent-scope JWT that's NOT bound to any tenant_id; per-action
	// tenant resolution happens at downstream parent endpoints from
	// the URL or the picked child.
	r.Route("/auth", func(r chi.Router) {
		if authRateLimiter != nil {
			r.Use(authRateLimiter)
		}
		r.Post("/login", rs.login)
		r.Post("/password-reset", rs.initiatePasswordReset)
		r.Post("/password-reset/confirm", rs.resetPassword)
	})

	// Authenticated parent routes — all require scope=parent.
	// sessiontoken.ParentMiddleware rejects tenant + operator tokens.
	r.Group(func(r chi.Router) {
		configureSession(r)

		rs.mountAccountRoutes(r)
		rs.mountCalendarRoutes(r)
		rs.mountEnrollmentRoutes(r)
		rs.mountChildStatusRoutes(r)
		rs.mountCommunicationRoutes(r, authRateLimiter)
		rs.mountChildCareRoutes(r)
		rs.mountChildMasterDataRoutes(r)

	})

	return r
}
