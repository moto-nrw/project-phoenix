package parent

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (rs *Resource) mountAccountRoutes(r chi.Router) {
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
}

func (rs *Resource) mountCalendarRoutes(r chi.Router) {
	r.Get("/me/calendar", rs.listMyCalendar)
	r.Get("/me/calendar/appointments/{appointmentId}/overview", rs.calendarAppointmentOverview)
	r.Get("/me/calendar/appointments/{appointmentId}/ics", rs.calendarAppointmentICS)
	r.Get("/me/calendar/feed", rs.calendarFeedURL)
	r.Post("/me/calendar/feed/rotate", rs.rotateCalendarFeed)
	r.Post("/me/calendar/recipients/{recipientId}/response", rs.respondToCalendarInvitation)
}

func (rs *Resource) mountEnrollmentRoutes(r chi.Router) {
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
	// already authenticated), and forwards to the same owner
	// submission the public path uses.
	r.Post("/enrollments/{tenantSlug}/submit", rs.submitParentEnrollment)
	r.Post("/enrollments/{tenantSlug}/uploads", rs.uploadParentEnrollment)
}

func (rs *Resource) mountChildStatusRoutes(r chi.Router) {
	// Per-child write features. The {studentId} is validated against
	// the calling account's guardian links inside the service — the
	// account id always comes from the JWT, never the URL/body.
	//   - sick-note: report the child sick for one or more dates
	//   - care-exception: set or clear a one-day pickup time
	r.Get("/me/children/{studentId}/features", rs.getChildFeatures)
	r.Get("/me/children/{studentId}/today", rs.getChildTodayStatus)
	r.Get("/me/children/{studentId}/consents", rs.getChildConsents)
	r.Put("/me/children/{studentId}/consents/photo", rs.grantPhotoConsent)
	r.Delete("/me/children/{studentId}/consents/photo", rs.withdrawPhotoConsent)
	r.Get("/me/children/{studentId}/meal-plan", rs.getChildMealPlan)
	r.Get("/me/children/{studentId}/meal-participation", rs.getMealParticipation)
	r.Put("/me/children/{studentId}/meal-participation", rs.replaceMealParticipation)
	r.Put("/me/children/{studentId}/meal-participation/{date}", rs.setMealParticipationDay)
	r.Put("/me/children/{studentId}/meal-participation/weeks/{date}", rs.setMealParticipationWeek)
	r.Delete("/me/children/{studentId}/meal-participation/{date}", rs.clearMealParticipationDay)
	r.Get("/me/children/{studentId}/sick-note", rs.listSickDays)
	r.Post("/me/children/{studentId}/sick-note", rs.submitSickNote)
	// Excused-absence approval requests (#1845): pending/decided requests the
	// parent can view, and edit their own still-pending one (#2267:
	// withdrawal was replaced by editing).
	r.Get("/me/children/{studentId}/excused-requests", rs.listExcusedRequests)
	// Guardian edit of an open request (#2267): the parent corrects their
	// own pending request instead of withdrawing and refiling it, so the
	// request keeps its id and the co-guardians it was shared with. Every
	// body carries expected_version; a mismatch is 409
	// change_request_stale.
	r.Put("/me/children/{studentId}/excused-requests/{requestId}", rs.editExcusedRequest)
	r.Get("/me/children/{studentId}/requests/{requestType}/{requestId}/events", rs.listRequestEvents)
	r.Get("/me/children/{studentId}/request-sharing/{requestType}/{requestId}", rs.getRequestSharing)
	r.Put("/me/children/{studentId}/request-sharing/{requestType}/{requestId}", rs.setRequestSharing)
	r.Get("/me/children/{studentId}/request-sharing-options", rs.getRequestSharingOptions)
}

func (rs *Resource) mountCommunicationRoutes(r chi.Router, authRateLimiter func(http.Handler) http.Handler) {
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
	// Group threads (#4784): what the OGS wrote to a group, class or AG of
	// the guardian's children. Answers go to the child's conversation above.
	r.Get("/me/messages/group-threads", rs.listGroupThreads)
	r.Get("/me/messages/group-threads/{groupThreadId}", rs.getGroupThread)
	// Parent-news feed (#1669): announcements the guardian is targeted by
	// across their children's news-enabled schools, to read, acknowledge,
	// answer or declare on. Audience and visibility come from the JWT
	// account; no tenant or audience selector is trusted from the client.
	r.Get("/me/news", rs.listAnnouncements)
	r.Get("/me/news/unread-count", rs.unreadAnnouncementCount)
	r.Post("/me/news/{announcementId}/read", rs.markAnnouncementRead)
	r.Post("/me/news/{announcementId}/acknowledge", rs.acknowledgeAnnouncement)
	rs.mountDeclarationRoutes(r, authRateLimiter)
	rs.mountPickupPermissionRoutes(r)
	// Poll answer, one child per call (#1371).
	r.Post("/me/news/{announcementId}/respond", rs.respondToAnnouncement)
	// Survey (#4781), one answer per guardian account.
	r.Get("/me/news/{announcementId}/survey", rs.getAnnouncementSurvey)
	r.Post("/me/news/{announcementId}/survey", rs.submitAnnouncementSurvey)
}

func (rs *Resource) mountChildCareRoutes(r chi.Router) {
	r.Get("/me/children/{studentId}/care-exception", rs.listCareExceptions)
	r.Post("/me/children/{studentId}/care-exception", rs.submitCareException)
	r.Delete("/me/children/{studentId}/care-exception", rs.deleteCareException)
	r.Get("/me/children/{studentId}/pickup-change-requests", rs.listPickupChangeRequests)
	r.Put("/me/children/{studentId}/pickup-change-requests/{requestId}", rs.editPickupChangeRequest)

	// Permanent weekly care plan (#1803) — read view on the Stammdaten
	// page plus the change-request lifecycle (create / edit). Staff
	// decide the requests on the central Änderungsanfragen page; the chat
	// only receives notification pills.
	r.Get("/me/children/{studentId}/care-schedule", rs.getChildCareSchedule)
	r.Post("/me/children/{studentId}/care-schedule/requests", rs.createCareScheduleRequest)
	r.Put("/me/children/{studentId}/care-schedule/requests/{requestId}", rs.editCareScheduleRequest)

	// Booked care offerings (#1665): read view plus the
	// post-enrollment change-request lifecycle. Approved requests are
	// applied by the enrollment domain on a chosen effective date.
	r.Get("/me/children/{studentId}/care-offerings", rs.getChildCareOfferings)
	r.Get("/me/children/{studentId}/care-offerings/catalog", rs.getChildOfferingCatalog)
	r.Post("/me/children/{studentId}/care-offerings/requests", rs.createOfferingChangeRequest)
	r.Put("/me/children/{studentId}/care-offerings/requests/{requestId}", rs.editOfferingChangeRequest)

	// Kurse (#3075, SH 4.3). Ein Kurs ist eine AG, die über ein
	// Betreuungsangebot erreichbar ist; die Anfrage ist deshalb dieselbe
	// Änderungsanfrage und wird in der bestehenden Freigabeansicht
	// entschieden. Gated by enrollment.parent_course_requests_enabled.
	r.Get("/me/children/{studentId}/courses", rs.getChildCourses)
	r.Post("/me/children/{studentId}/courses/requests", rs.createCourseRequest)
	r.Post("/me/children/{studentId}/courses/requests/{requestId}/withdraw", rs.withdrawCourseRequest)
}

func (rs *Resource) mountChildMasterDataRoutes(r chi.Router) {
	// Stammdaten — structured view of the child's master data plus the
	// calling guardian's own contact data. Track A direct edits apply
	// immediately and are audited; Track B change requests (name,
	// birthday, permanent Gehzeit) are added in a later step.
	r.Get("/me/children/{studentId}/master-data", rs.getMasterData)
	r.Patch("/me/children/{studentId}/master-data/{target}/{field}", rs.updateMasterDataField)
	r.Get("/me/children/{studentId}/master-data/requests", rs.listMasterDataRequests)
	r.Post("/me/children/{studentId}/master-data/requests", rs.submitMasterDataRequest)
	r.Put("/me/children/{studentId}/master-data/requests/{requestId}", rs.editMasterDataRequest)

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
}
