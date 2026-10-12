// Package enrollmenthttp holds the parent-enrollment HTTP layer. PR 5 ships
// the admin form-schema CRUD; PR 7 will add public submission +
// status/edit endpoints; PR 8 admin decision endpoints.
package enrollmenthttp

import (
	"context"
	"net/http"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// GuardianProfiles loads the guardian profile an account holds in one school,
// with the children linked to it. People Directory owns the rows; the
// composition root binds the port. An account without a profile there is
// (nil, nil).
type GuardianProfiles = capability.GuardianAutofillReader

// Resource bundles the handler methods + their dependencies.
type Resource struct {
	FormSchemaService    capability.FormSchemaAdministration
	CareOfferingService  CareOfferingCatalog
	RequestService       RequestService
	CaptchaService       capability.CaptchaVerifier
	PhaseService         capability.PhaseAdministration
	PhaseExpiryService   capability.PhaseExpiryWarnings
	DecisionService      DecisionService
	ReportService        capability.Reports
	RolloverService      RolloverService
	ChangeRequestService ChangeRequestService
	DeletionService      EnrollmentDeletionService
	GuardianInvitations  GuardianInvitationRuntime
	GuardianProfiles     GuardianProfiles
	SchoolService        SchoolDirectory
	// ListExportService renders the compact per-phase registration
	// export (PDF blocks + XLSX flat table) through the Document Rendering
	// list contract. Set as a field after construction (mirrors api/rooms),
	// not via the constructor.
	ListExportService lists.DocumentRenderer
	// transactions is set by NewResource: the route steps then run in the
	// tenant and administrative transactions of the shared tenant runtime.
	// The bare Resource a route unit test builds runs runInTenantTx on the
	// request context instead and answers the public routes as not wired.
	transactions         bool
	legalDocumentRefs    legalDocumentReferenceRepository
	runInTenantTxForTest func(r *http.Request, fn func(ctx context.Context) error) error
}

// NewResource constructs the enrollment API resource. PR 7 added the
// RequestService + CaptchaService for the public submission flow.
// PR A of the phase model wires PhaseService so the public + admin
// endpoints can resolve phase rows. PR 8 wires DecisionService for the
// admin review/accept/reject UI; slice 2 also wires the
// GuardianInvitations runtime so post-approval invites can fire.
func NewResource(
	formSchemaSvc capability.FormSchemaAdministration,
	careOfferingSvc CareOfferingCatalog,
	requestSvc RequestService,
	captchaSvc capability.CaptchaVerifier,
	phaseSvc capability.PhaseAdministration,
	decisionSvc DecisionService,
	reportSvc capability.Reports,
	rolloverSvc RolloverService,
	changeRequestSvc ChangeRequestService,
	deletionSvc EnrollmentDeletionService,
	guardianInvitations GuardianInvitationRuntime,
	guardianProfiles GuardianProfiles,
	schoolService SchoolDirectory,
	legalDocumentRefs ...legalDocumentReferenceRepository,
) *Resource {
	rs := &Resource{
		FormSchemaService:    formSchemaSvc,
		CareOfferingService:  careOfferingSvc,
		RequestService:       requestSvc,
		CaptchaService:       captchaSvc,
		PhaseService:         phaseSvc,
		DecisionService:      decisionSvc,
		ReportService:        reportSvc,
		RolloverService:      rolloverSvc,
		ChangeRequestService: changeRequestSvc,
		DeletionService:      deletionSvc,
		GuardianInvitations:  guardianInvitations,
		GuardianProfiles:     guardianProfiles,
		SchoolService:        schoolService,
		transactions:         true,
	}
	if len(legalDocumentRefs) > 0 {
		rs.legalDocumentRefs = legalDocumentRefs[0]
	}
	return rs
}

// Router returns a chi router scoped to /enrollment. PR 5 added the
// admin form-schema endpoints; PR 6 adds care-offering admin CRUD +
// the public open-window endpoint.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public routes: parent-facing endpoints. No JWT — slug-gated or
	// status-token-gated in the handler. Sit outside the auth group
	// below so the JWT middleware doesn't reject anonymous requests.
	rs.mountPublicRoutes(r)

	// Authenticated admin endpoints.
	r.Group(func(r chi.Router) {
		r.Use(jwt.Authenticator)
		r.Use(common.ReadOnlyPreviewMiddleware)
		r.Use(common.TenantScopeMiddleware)
		r.Use(common.SecurityPrincipalMiddleware)

		rs.mountConfigRoutes(r)
		rs.mountPhaseRoutes(r)
		rs.mountAdminRequestRoutes(r)
		rs.mountAdminReviewRoutes(r)
	})

	return r
}

// mountPublicRoutes registers the parent-facing routes.
func (rs *Resource) mountPublicRoutes(r chi.Router) {
	r.Get("/phases/public/{tenantSlug}", rs.listPublicPhases)
	r.Get("/care-offerings/public/{tenantSlug}/{phaseId}", rs.listPublicCareOfferings)
	r.Get("/form-bootstrap/public/{tenantSlug}/{phaseId}", rs.publicFormBootstrap)
	r.Get("/schema/public/{tenantSlug}/{phaseId}", rs.listPublicActiveSchema)
	r.Get("/captcha-config/{tenantSlug}", rs.publicCaptchaConfig)
	r.Get("/legal/{tenantSlug}/{phaseId}", rs.publicLegalTexts)
	r.Get("/legal/{tenantSlug}", rs.publicLegalTexts)
	r.Post("/{tenantSlug}/submit", rs.submitEnrollment)
	r.Get("/requests/{statusToken}", rs.getStatus)
	r.Get("/requests/{statusToken}/edit-bootstrap", rs.getEditBootstrap)
	r.Patch("/requests/{statusToken}", rs.patchStatus)
	r.Put("/requests/{statusToken}", rs.replaceStatus)
	r.Get("/requests/{statusToken}/change-requests", rs.listPublicChangeRequests)
	r.Post("/requests/{statusToken}/change-requests", rs.createChangeRequest)
	r.Post("/requests/{statusToken}/change-requests/{changeRequestId}/messages", rs.replyToChangeRequest)
	r.Post("/requests/{statusToken}/withdraw", rs.withdrawStatus)
	r.Post("/requests/{statusToken}/confirm-renewal", rs.confirmRenewal)
}

// mountConfigRoutes registers the form schemas, the legal documents and the
// care-offering catalog.
func (rs *Resource) mountConfigRoutes(r chi.Router) {
	r.Route("/schema", func(r chi.Router) {
		r.With(common.RequiresPermission("config:read")).Get("/", rs.getActiveSchema)
		r.With(common.RequiresPermission("config:read")).Get("/versions", rs.listSchemaVersions)
		r.With(common.RequiresPermission("config:read")).Get("/preview", rs.getSchemaPreviewBootstrap)
		r.With(common.RequiresPermission("config:read")).Get("/{id}", rs.getSchemaByID)
		r.With(common.RequiresPermission("config:manage")).Post("/", rs.publishSchema)
		r.With(common.RequiresPermission("config:manage")).Put("/{id}", rs.updateSchema)
		r.With(common.RequiresPermission("config:manage")).Patch("/{id}", rs.renameSchema)
		r.With(common.RequiresPermission("config:manage")).Delete("/{id}", rs.deleteSchema)
	})
	r.With(common.RequiresPermission("config:manage")).Post("/legal-documents", rs.uploadLegalDocument)
	r.With(common.RequiresPermission("config:manage")).Delete("/legal-documents/{filename}", rs.deleteLegalDocument)

	r.Route("/care-offerings", func(r chi.Router) {
		r.With(common.RequiresPermission("config:read")).Get("/", rs.listCareOfferings)
		// Static segment, registered before the {id} sub-router so chi
		// matches it as a literal path rather than an offering id.
		r.With(common.RequiresPermission("config:read")).Get("/booking-stats", rs.listCareOfferingBookingStats)
		r.With(common.RequiresPermission("config:manage")).Post("/", rs.createCareOffering)
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission("config:read")).Get("/", rs.getCareOffering)
			r.With(common.RequiresPermission("config:manage")).Put("/", rs.updateCareOffering)
			r.With(common.RequiresPermission("config:manage")).Delete("/", rs.deleteCareOffering)
			r.With(common.RequiresPermission("config:manage")).Post("/clone", rs.cloneCareOffering)
		})
	})
}

// mountPhaseRoutes registers the phases with their rollover, responses,
// late invites, manual enrollments and export.
func (rs *Resource) mountPhaseRoutes(r chi.Router) {
	r.Route("/phases", func(r chi.Router) {
		r.With(common.RequiresPermission("config:read")).Get("/", rs.listPhases)
		// The warnings ask for a successor phase, which config:manage
		// creates; a lead role without the admin wildcard reads them too (#3469).
		r.With(common.RequiresPermission("config:manage")).Get("/expiry-warnings", rs.listPhaseExpiryWarnings)
		r.With(common.RequiresPermission("config:manage")).Post("/", rs.createPhase)
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission("config:read")).Get("/", rs.getPhase)
			r.With(common.RequiresPermission("config:manage")).Put("/", rs.updatePhase)
			// delete-impact previews the blast radius for the
			// confirmation modal; same permission as the delete it
			// precedes.
			r.With(common.RequiresPermission("config:manage")).Get("/delete-impact", rs.getPhaseDeleteImpact)
			r.With(common.RequiresPermission("config:manage")).Delete("/", rs.deletePhase)
			// Rollover (phase renewal). createRollover carries
			// approved enrollments from this phase forward into a
			// new phase; listRolloverReview surfaces children that
			// landed in pending_admin_review on a phase created
			// FROM this phase. Both require config:manage.
			r.With(common.RequiresPermission("config:manage")).Post("/rollover", rs.createRollover)
			// Read-only dry run for the rollover form (#2251); same
			// permission as the create it precedes.
			r.With(common.RequiresPermission("config:manage")).Get("/rollover-preview", rs.previewRollover)
			r.With(common.RequiresPermission("config:read")).Get("/review", rs.listRolloverReview)
			// Response of the existing children to this phase (#3379):
			// who answered, who is still missing. Same tier as the
			// request list, which already shows these children by name.
			r.With(common.RequiresPermission("config:read")).Get("/responses", rs.getPhaseResponseOverview)
			r.With(common.RequiresPermission("config:manage")).Get("/manual-bootstrap", rs.getManualEnrollmentBootstrap)
			r.With(common.RequiresPermission("config:manage")).Post("/late-invites", rs.createLateInvite)
			r.With(common.RequiresPermission("config:manage")).Post("/manual-approved-enrollments", rs.createManualApprovedEnrollment)
			// Compact export of every registration in the phase
			// (PDF for print, XLSX for data). Gated config:manage
			// (not config:read like the review list): one call
			// bundles every guardian + child's full PII into a file
			// that leaves the RLS-protected system, so it sits at
			// the GDPR/admin tier alongside rollover. Admins hold
			// config:manage via the admin:* wildcard.
			r.With(common.RequiresPermission("config:manage")).Post("/export", rs.exportPhaseRegistrations)
		})
	})
}

// mountAdminRequestRoutes registers the rollover review decisions, the
// autofill profile and the admin request surface.
func (rs *Resource) mountAdminRequestRoutes(r chi.Router) {
	// Rollover review decisions live alongside the admin requests
	// surface so reviewers don't have to keep switching contexts.
	r.Route("/admin/request-children", func(r chi.Router) {
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission("config:manage")).Post("/rollover-review", rs.decideRolloverReview)
		})
	})

	// Autofill payload for the public enrollment form. Any
	// authenticated session can hit it — we don't require a
	// specific permission. Non-guardian sessions get the auth
	// claims as guardian fields and an empty children list, so
	// the frontend can still cleanly render the form.
	r.Get("/me/profile", rs.getMyProfile)

	// PR 8 admin review surface. config:read for queue browse;
	// config:manage for detail, export and decisions because those
	// expose or mutate full enrollment PII. Decision writes audit
	// reviewed_by/reviewed_at on each child row.
	r.Route("/admin/requests", func(r chi.Router) {
		r.With(common.RequiresPermission("config:read")).Get("/", rs.listAdminRequests)
		// Per-account read state (#3778): the badge on the Anmeldungen
		// section, same permission as the section.
		r.With(common.RequiresPermission("config:manage")).Get("/unread-count", rs.unreadAdminRequestCount)
		r.With(common.RequiresPermission("config:manage")).Post("/mark-all-read", rs.markAllAdminRequestsRead)
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission("config:manage")).Get("/", rs.getAdminRequest)
			r.With(common.RequiresPermission("config:manage")).Put("/read", rs.markAdminRequestRead)
			r.With(common.RequiresPermission("config:manage")).Delete("/read", rs.markAdminRequestUnread)
			r.With(common.RequiresPermission("config:manage")).Get("/delete-impact", rs.getAdminRequestDeleteImpact)
			r.With(common.RequiresPermission("config:manage")).Delete("/", rs.deleteAdminRequest)
			r.With(common.RequiresPermission("config:manage")).Post("/restore", rs.restoreAdminRequest)
			r.With(common.RequiresPermission("config:manage")).Get("/children/{childId}/delete-impact", rs.getAdminChildDeleteImpact)
			r.With(common.RequiresPermission("config:manage")).Delete("/children/{childId}", rs.deleteAdminChild)
			r.With(common.RequiresPermission("config:manage")).Post("/children/{childId}/decide", rs.decideAdminChild)
			r.With(common.RequiresPermission("config:manage")).Put("/children/{childId}/data-correction", rs.correctAdminChildData)
			r.With(common.RequiresPermission("config:manage")).Put("/children/{childId}/offerings", rs.updateAdminChildOfferings)
			r.With(common.RequiresPermission("config:manage")).Get("/children/{childId}/offering-adjustments", rs.listAdminChildOfferingAdjustments)
		})
	})
}

// mountAdminReviewRoutes registers the change-request review, the reports
// and a student's requests.
func (rs *Resource) mountAdminReviewRoutes(r chi.Router) {
	r.Route("/admin/change-requests", func(r chi.Router) {
		r.With(common.RequiresPermission("config:manage")).Get("/", rs.listAdminChangeRequests)
		// Anmeldungsänderungen in the shared display format of the request
		// module, open or history, keyset-paginated (#2435).
		r.With(common.RequiresPermission("config:manage")).Get("/list", rs.listChangeRequestReviewEntries)
		r.With(common.RequiresPermission("config:manage")).Get("/pending-count", rs.pendingChangeRequestReviewCount)
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission("config:manage")).Get("/", rs.getAdminChangeRequest)
			r.With(common.RequiresPermission("config:manage")).Post("/question", rs.askChangeRequestQuestion)
			r.With(common.RequiresPermission("config:manage")).Post("/approve", rs.approveChangeRequest)
			r.With(common.RequiresPermission("config:manage")).Post("/reject", rs.rejectChangeRequest)
		})
	})
	r.Route("/admin/reports", func(r chi.Router) {
		r.With(common.RequiresPermission("config:read")).Get("/care-usage", rs.getCareUsageReport)
		r.With(common.RequiresPermission("config:manage")).Post("/care-usage/export", rs.exportCareUsageReport)
		r.With(common.RequiresAllPermissions(permissions.ConfigManage, permissions.UsersRead)).Post("/class-roster/export", rs.exportClassRosterReport)
	})
	r.Route("/admin/students/{studentId}/requests", func(r chi.Router) {
		r.With(common.RequiresPermission("config:manage")).Get("/", rs.listAdminRequestsByStudent)
		r.With(common.RequiresPermission("config:manage")).Post("/export", rs.exportStudentEnrollmentRequests)
	})
}
