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
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	pwaService "github.com/moto-nrw/project-phoenix/services/pwa"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource bundles the parent-portal HTTP handlers + their deps.
type Resource struct {
	AuthService           authService.AuthService
	ParentService         parentService.Service
	RequestSharing        parentService.RequestSharingService
	CalendarService       calendarService.Service
	RequestService        enrollmentService.RequestService
	GuardianProfileLoader *usersService.GuardianProfileLoader
	SchoolService         platformSvc.SchoolService
	PushService           notificationsService.PushSubscriptionService
	PreferenceService     notificationsService.PreferenceService
	PWAUsageService       pwaService.UsageService
	db                    *bun.DB
	authRateLimiter       func(http.Handler) http.Handler
}

// SetPreferenceService injects the notification consent service.
func (rs *Resource) SetPreferenceService(service notificationsService.PreferenceService) {
	rs.PreferenceService = service
}

// SetPushService injects the Web Push subscription service (#2003).
func (rs *Resource) SetPushService(service notificationsService.PushSubscriptionService) {
	rs.PushService = service
}

// SetPWAUsageService injects the PWA standalone-usage service (#2189).
func (rs *Resource) SetPWAUsageService(service pwaService.UsageService) {
	rs.PWAUsageService = service
}

// SetAuthRateLimiter sets the rate limiter middleware for public parent auth
// endpoints. Mirrors the tenant and operator wiring in api/base.go so
// brute-force attempts return 429.
func (rs *Resource) SetAuthRateLimiter(mw func(http.Handler) http.Handler) {
	rs.authRateLimiter = mw
}

func (rs *Resource) SetCalendarService(service calendarService.Service) {
	rs.CalendarService = service
}

// NewResource builds the parent-portal resource.
func NewResource(
	auth authService.AuthService,
	parent parentService.Service,
	requestSvc enrollmentService.RequestService,
	guardianProfileLoader *usersService.GuardianProfileLoader,
	schoolService platformSvc.SchoolService,
	db *bun.DB,
) *Resource {
	var sharing parentService.RequestSharingService
	if parent != nil {
		var ok bool
		sharing, ok = parent.(parentService.RequestSharingService)
		if !ok {
			panic("parent resource requires a request sharing service")
		}
	}
	return &Resource{
		AuthService:           auth,
		ParentService:         parent,
		RequestSharing:        sharing,
		RequestService:        requestSvc,
		GuardianProfileLoader: guardianProfileLoader,
		SchoolService:         schoolService,
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
		r.Post("/password-reset", rs.initiatePasswordReset)
		r.Post("/password-reset/confirm", rs.resetPassword)
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
		// Push subscription management for the parent's own devices
		// (#2003). Registers across every active tenant mapping.
		r.Route("/me/push", func(r chi.Router) {
			r.Get("/public-key", rs.getPushPublicKey)
			r.Post("/subscriptions", rs.subscribePush)
			r.Delete("/subscriptions", rs.unsubscribePush)
		})

		// PWA standalone-usage report (#2189): fans out to every
		// active tenant mapping, account id from claims only.
		r.Post("/me/pwa-usage", rs.reportPWAUsage)

		// Which notifications this guardian agreed to. Applied to every
		// school the family belongs to, the same fan-out the push
		// registration above uses.
		r.Route("/me/notification-preferences", func(r chi.Router) {
			r.Get("/", rs.listNotificationPreferences)
			r.Delete("/", rs.deleteAllNotificationPreferences)
			r.Put("/{type}", rs.setNotificationPreference)
		})

		r.Get("/me/profile", rs.getMyProfile)
		r.Put("/me/profile", rs.updateMyProfile)
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
		r.Get("/me/calendar", rs.listMyCalendar)
		r.Get("/me/calendar/appointments/{appointmentId}/overview", rs.calendarAppointmentOverview)
		r.Get("/me/calendar/appointments/{appointmentId}/ics", rs.calendarAppointmentICS)
		r.Get("/me/calendar/feed", rs.calendarFeedURL)
		r.Post("/me/calendar/feed/rotate", rs.rotateCalendarFeed)
		r.Post("/me/calendar/recipients/{recipientId}/response", rs.respondToCalendarInvitation)

		// Per-school autofill payload for the embedded enrollment
		// form. Tenant resolved from the {tenantSlug} path segment;
		// account from the parent JWT. Returns guardian fields from
		// the guardian_profile in that tenant (or claims if none) +
		// any students already linked to the guardian profile.
		r.Get("/enrollments/{tenantSlug}/profile", rs.getEnrollmentProfile)

		// Authenticated form-load bootstrap for the embedded enrollment
		// form. Mirrors the anonymous public /form-bootstrap endpoint but
		// runs the enrollee phase gate, so audience-restricted
		// (linked_parents) phases load here for a logged-in guardian while
		// the public path refuses them (#1663). Tenant from {tenantSlug},
		// account from the parent JWT; captcha skipped.
		r.Get("/enrollments/{tenantSlug}/bootstrap/{phaseId}", rs.getEnrollmentBootstrap)

		// Authenticated submit. Stamps guardian_account_id on the
		// resulting enrollment.requests row, skips captcha (parent
		// already authenticated), and forwards to the same
		// RequestService.Submit the public path uses.
		r.Post("/enrollments/{tenantSlug}/submit", rs.submitParentEnrollment)

		// Per-child write features. The {studentId} is validated against
		// the calling account's guardian links inside the service — the
		// account id always comes from the JWT, never the URL/body.
		//   - sick-note: report the child sick for one or more dates
		//   - care-exception: set or clear a one-day pickup time
		r.Get("/me/children/{studentId}/features", rs.getChildFeatures)
		r.Get("/me/children/{studentId}/today", rs.getChildTodayStatus)
		r.Get("/me/children/{studentId}/meal-plan", rs.getChildMealPlan)
		r.Get("/me/children/{studentId}/sick-note", rs.listSickDays)
		r.Post("/me/children/{studentId}/sick-note", rs.submitSickNote)
		// Excused-absence approval requests (#1845): pending/decided requests the
		// parent can view, and withdraw their own still-pending one.
		r.Get("/me/children/{studentId}/excused-requests", rs.listExcusedRequests)
		r.Delete("/me/children/{studentId}/excused-requests/{requestId}", rs.withdrawExcusedRequest)
		r.Get("/me/children/{studentId}/request-sharing/{requestType}/{requestId}", rs.getRequestSharing)
		r.Put("/me/children/{studentId}/request-sharing/{requestType}/{requestId}", rs.setRequestSharing)
		r.Get("/me/children/{studentId}/request-sharing-options", rs.getRequestSharingOptions)

		// Parent-OGS messaging — chat model. One continuous conversation per
		// child with the OGS (no subject). The list aggregates the guardian's
		// conversations across all their children; the per-child routes read
		// the conversation (marking it read) and post a message (creating the
		// conversation on the first message). Gated by operations.parent_notes_enabled.
		r.Get("/me/messages", rs.listMessageThreads)
		r.Get("/me/messages/unread-count", rs.unreadMessageCount)
		r.Get("/me/messages/children/{studentId}/threads", rs.listChildThreads)
		r.Get("/me/messages/children/{studentId}", rs.getChildConversation)
		r.Post("/me/messages/children/{studentId}", rs.postChildMessage)
		// Parent-news feed (#1669) — read-only broadcast announcements the
		// guardian is targeted by across all their children's (news-enabled)
		// schools. The guardian can mark one read, or acknowledge one that
		// requires confirmation. Audience + visibility are enforced server-side
		// from the JWT account; no tenant or audience selector is trusted from
		// the client.
		r.Get("/me/news", rs.listAnnouncements)
		r.Get("/me/news/unread-count", rs.unreadAnnouncementCount)
		r.Post("/me/news/{announcementId}/read", rs.markAnnouncementRead)
		r.Post("/me/news/{announcementId}/acknowledge", rs.acknowledgeAnnouncement)
		// Poll answer, one child per call (#1371).
		r.Post("/me/news/{announcementId}/respond", rs.respondToAnnouncement)
		r.Get("/me/children/{studentId}/care-exception", rs.listCareExceptions)
		r.Post("/me/children/{studentId}/care-exception", rs.submitCareException)
		r.Delete("/me/children/{studentId}/care-exception", rs.deleteCareException)
		r.Get("/me/children/{studentId}/pickup-change-requests", rs.listPickupChangeRequests)
		r.Delete("/me/children/{studentId}/pickup-change-requests/{requestId}", rs.withdrawPickupChangeRequest)

		// Permanent weekly care plan (#1803) — read view on the Stammdaten
		// page plus the change-request lifecycle (create / withdraw). Staff
		// decide the requests on the central Änderungsanfragen page; the chat
		// only receives notification pills.
		r.Get("/me/children/{studentId}/care-schedule", rs.getChildCareSchedule)
		r.Post("/me/children/{studentId}/care-schedule/requests", rs.createCareScheduleRequest)
		r.Post("/me/children/{studentId}/care-schedule/requests/{requestId}/withdraw", rs.withdrawCareScheduleRequest)

		// Booked care offerings (#1665): read view plus the
		// post-enrollment change-request lifecycle. Approved requests are
		// applied by the enrollment domain on a chosen effective date.
		r.Get("/me/children/{studentId}/care-offerings", rs.getChildCareOfferings)
		r.Get("/me/children/{studentId}/care-offerings/catalog", rs.getChildOfferingCatalog)
		r.Post("/me/children/{studentId}/care-offerings/requests", rs.createOfferingChangeRequest)
		r.Post("/me/children/{studentId}/care-offerings/requests/{requestId}/withdraw", rs.withdrawOfferingChangeRequest)

		// Stammdaten — structured view of the child's master data plus the
		// calling guardian's own contact data. Track A direct edits apply
		// immediately and are audited; Track B change requests (name,
		// birthday, permanent Gehzeit) are added in a later step.
		r.Get("/me/children/{studentId}/master-data", rs.getMasterData)
		r.Patch("/me/children/{studentId}/master-data/{target}/{field}", rs.updateMasterDataField)
		r.Get("/me/children/{studentId}/master-data/requests", rs.listMasterDataRequests)
		r.Post("/me/children/{studentId}/master-data/requests", rs.submitMasterDataRequest)

		// Related accounts — see who has access to the child, invite a
		// further guardian by email (gated by guardians.parent_invite_mode),
		// and remove an account's access (gated by guardians.parent_can_remove;
		// the primary guardian is protected). Ownership of {studentId} is
		// verified in the service against the calling account's guardian links.
		r.Get("/me/children/{studentId}/related-accounts", rs.listRelatedAccounts)
		r.Post("/me/children/{studentId}/related-accounts", rs.inviteRelatedAccount)
		r.Delete("/me/children/{studentId}/related-accounts/{guardianProfileId}", rs.removeRelatedAccount)

		// Guardian contact + pickup info — list every guardian of the child
		// with contact and pickup detail, edit a contact-only guardian's
		// contact data (parent_portal.guardian.edit), and manage the per-child
		// pickup/emergency flags (parent_portal.pickup.manage). Both writes are
		// authorized in the service against the calling account's guardian
		// permissions; a guardian with their own portal account can never have
		// their personal data edited by another parent.
		r.Get("/me/children/{studentId}/guardians", rs.listChildGuardians)
		r.Post("/me/children/{studentId}/guardians", rs.createGuardianContact)
		r.Put("/me/children/{studentId}/guardians/{guardianProfileId}/contact", rs.updateGuardianContact)
		r.Put("/me/children/{studentId}/guardians/{guardianProfileId}/pickup", rs.updateGuardianRelationship)

	})

	return r
}
