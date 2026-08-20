// Package enrollment holds the parent-enrollment HTTP layer. PR 5 ships
// the admin form-schema CRUD; PR 7 will add public submission +
// status/edit endpoints; PR 8 admin decision endpoints.
package enrollment

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource bundles the handler methods + their dependencies.
type Resource struct {
	FormSchemaService         enrollmentService.FormSchemaService
	CareOfferingService       enrollmentService.CareOfferingService
	RequestService            enrollmentService.RequestService
	CaptchaService            *enrollmentService.CaptchaService
	PhaseService              enrollmentService.PhaseService
	DecisionService           enrollmentService.DecisionService
	ReportService             enrollmentService.ReportService
	RolloverService           enrollmentService.RolloverService
	ChangeRequestService      enrollmentService.ChangeRequestService
	DeletionService           enrollmentService.EnrollmentDeletionService
	GuardianInvitationService authService.GuardianInvitationService
	GuardianProfileLoader     *usersService.GuardianProfileLoader
	SchoolService             platformSvc.SchoolService
	// ListExportService renders the compact per-phase registration
	// export (PDF blocks + XLSX flat table). Set as a field after
	// construction (mirrors api/rooms), not via the constructor.
	ListExportService    *listexport.RendererService
	db                   *bun.DB
	legalDocumentRefs    legalDocumentReferenceRepository
	runInTenantTxForTest func(r *http.Request, fn func(ctx context.Context) error) error
}

// NewResource constructs the enrollment API resource. PR 7 added the
// RequestService + CaptchaService for the public submission flow.
// PR A of the phase model wires PhaseService so the public + admin
// endpoints can resolve phase rows. PR 8 wires DecisionService for the
// admin review/accept/reject UI; slice 2 also wires the
// GuardianInvitationService so post-approval invites can fire.
func NewResource(
	formSchemaSvc enrollmentService.FormSchemaService,
	careOfferingSvc enrollmentService.CareOfferingService,
	requestSvc enrollmentService.RequestService,
	captchaSvc *enrollmentService.CaptchaService,
	phaseSvc enrollmentService.PhaseService,
	decisionSvc enrollmentService.DecisionService,
	reportSvc enrollmentService.ReportService,
	rolloverSvc enrollmentService.RolloverService,
	changeRequestSvc enrollmentService.ChangeRequestService,
	deletionSvc enrollmentService.EnrollmentDeletionService,
	guardianInvitationSvc authService.GuardianInvitationService,
	guardianProfileLoader *usersService.GuardianProfileLoader,
	schoolService platformSvc.SchoolService,
	db *bun.DB,
	legalDocumentRefs ...legalDocumentReferenceRepository,
) *Resource {
	rs := &Resource{
		FormSchemaService:         formSchemaSvc,
		CareOfferingService:       careOfferingSvc,
		RequestService:            requestSvc,
		CaptchaService:            captchaSvc,
		PhaseService:              phaseSvc,
		DecisionService:           decisionSvc,
		ReportService:             reportSvc,
		RolloverService:           rolloverSvc,
		ChangeRequestService:      changeRequestSvc,
		DeletionService:           deletionSvc,
		GuardianInvitationService: guardianInvitationSvc,
		GuardianProfileLoader:     guardianProfileLoader,
		SchoolService:             schoolService,
		db:                        db,
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

	// Authenticated admin endpoints.
	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)

		r.Route("/schema", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getActiveSchema)
			r.With(authorize.RequiresPermission("config:read")).Get("/versions", rs.listSchemaVersions)
			r.With(authorize.RequiresPermission("config:read")).Get("/preview", rs.getSchemaPreviewBootstrap)
			r.With(authorize.RequiresPermission("config:read")).Get("/{id}", rs.getSchemaByID)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.publishSchema)
			r.With(authorize.RequiresPermission("config:manage")).Put("/{id}", rs.updateSchema)
			r.With(authorize.RequiresPermission("config:manage")).Patch("/{id}", rs.renameSchema)
			r.With(authorize.RequiresPermission("config:manage")).Delete("/{id}", rs.deleteSchema)
		})
		r.With(authorize.RequiresPermission("config:manage")).Post("/legal-documents", rs.uploadLegalDocument)
		r.With(authorize.RequiresPermission("config:manage")).Delete("/legal-documents/{filename}", rs.deleteLegalDocument)

		r.Route("/care-offerings", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.listCareOfferings)
			// Static segment, registered before the {id} sub-router so chi
			// matches it as a literal path rather than an offering id.
			r.With(authorize.RequiresPermission("config:read")).Get("/booking-stats", rs.listCareOfferingBookingStats)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.createCareOffering)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getCareOffering)
				r.With(authorize.RequiresPermission("config:manage")).Put("/", rs.updateCareOffering)
				r.With(authorize.RequiresPermission("config:manage")).Delete("/", rs.deleteCareOffering)
				r.With(authorize.RequiresPermission("config:manage")).Post("/clone", rs.cloneCareOffering)
				// Angebots-Gehzeit rollout (#2290): preview feeds the
				// confirmation dialog, POST executes with per-child opt-outs.
				r.With(authorize.RequiresPermission("config:manage")).Get("/pickup-rollout", rs.previewCareOfferingPickupRollout)
				r.With(authorize.RequiresPermission("config:manage")).Post("/pickup-rollout", rs.rolloutCareOfferingPickupTimes)
			})
		})

		r.Route("/phases", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.listPhases)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.createPhase)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getPhase)
				r.With(authorize.RequiresPermission("config:manage")).Put("/", rs.updatePhase)
				// delete-impact previews the blast radius for the
				// confirmation modal; same permission as the delete it
				// precedes.
				r.With(authorize.RequiresPermission("config:manage")).Get("/delete-impact", rs.getPhaseDeleteImpact)
				r.With(authorize.RequiresPermission("config:manage")).Delete("/", rs.deletePhase)
				// Rollover (phase renewal). createRollover carries
				// approved enrollments from this phase forward into a
				// new phase; listRolloverReview surfaces children that
				// landed in pending_admin_review on a phase created
				// FROM this phase. Both require config:manage.
				r.With(authorize.RequiresPermission("config:manage")).Post("/rollover", rs.createRollover)
				// Read-only dry run for the rollover form (#2251); same
				// permission as the create it precedes.
				r.With(authorize.RequiresPermission("config:manage")).Get("/rollover-preview", rs.previewRollover)
				r.With(authorize.RequiresPermission("config:read")).Get("/review", rs.listRolloverReview)
				r.With(authorize.RequiresPermission("config:manage")).Get("/manual-bootstrap", rs.getManualEnrollmentBootstrap)
				r.With(authorize.RequiresPermission("config:manage")).Post("/late-invites", rs.createLateInvite)
				r.With(authorize.RequiresPermission("config:manage")).Post("/manual-approved-enrollments", rs.createManualApprovedEnrollment)
				// Compact export of every registration in the phase
				// (PDF for print, XLSX for data). Gated config:manage
				// (not config:read like the review list): one call
				// bundles every guardian + child's full PII into a file
				// that leaves the RLS-protected system, so it sits at
				// the GDPR/admin tier alongside rollover. Admins hold
				// config:manage via the admin:* wildcard.
				r.With(authorize.RequiresPermission("config:manage")).Post("/export", rs.exportPhaseRegistrations)
			})
		})

		// Rollover review decisions live alongside the admin requests
		// surface so reviewers don't have to keep switching contexts.
		r.Route("/admin/request-children", func(r chi.Router) {
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:manage")).Post("/rollover-review", rs.decideRolloverReview)
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
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.listAdminRequests)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:manage")).Get("/", rs.getAdminRequest)
				r.With(authorize.RequiresPermission("config:manage")).Get("/delete-impact", rs.getAdminRequestDeleteImpact)
				r.With(authorize.RequiresPermission("config:manage")).Delete("/", rs.deleteAdminRequest)
				r.With(authorize.RequiresPermission("config:manage")).Post("/restore", rs.restoreAdminRequest)
				r.With(authorize.RequiresPermission("config:manage")).Get("/children/{childId}/delete-impact", rs.getAdminChildDeleteImpact)
				r.With(authorize.RequiresPermission("config:manage")).Delete("/children/{childId}", rs.deleteAdminChild)
				r.With(authorize.RequiresPermission("config:manage")).Post("/children/{childId}/decide", rs.decideAdminChild)
				r.With(authorize.RequiresPermission("config:manage")).Put("/children/{childId}/data-correction", rs.correctAdminChildData)
				r.With(authorize.RequiresPermission("config:manage")).Put("/children/{childId}/offerings", rs.updateAdminChildOfferings)
				r.With(authorize.RequiresPermission("config:manage")).Get("/children/{childId}/offering-adjustments", rs.listAdminChildOfferingAdjustments)
			})
		})
		r.Route("/admin/change-requests", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:manage")).Get("/", rs.listAdminChangeRequests)
			// Anmeldungsänderungen in the shared display format of the request
			// module, open or history, keyset-paginated (#2435).
			r.With(authorize.RequiresPermission("config:manage")).Get("/list", rs.listChangeRequestReviewEntries)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:manage")).Get("/", rs.getAdminChangeRequest)
				r.With(authorize.RequiresPermission("config:manage")).Post("/question", rs.askChangeRequestQuestion)
				r.With(authorize.RequiresPermission("config:manage")).Post("/approve", rs.approveChangeRequest)
				r.With(authorize.RequiresPermission("config:manage")).Post("/reject", rs.rejectChangeRequest)
			})
		})
		r.Route("/admin/reports", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/care-usage", rs.getCareUsageReport)
			r.With(authorize.RequiresPermission("config:manage")).Post("/care-usage/export", rs.exportCareUsageReport)
			r.With(authorize.RequiresAllPermissions(permissions.ConfigManage, permissions.UsersRead)).Post("/class-roster/export", rs.exportClassRosterReport)
		})
		r.Route("/admin/students/{studentId}/requests", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:manage")).Get("/", rs.listAdminRequestsByStudent)
			r.With(authorize.RequiresPermission("config:manage")).Post("/export", rs.exportStudentEnrollmentRequests)
		})
	})

	return r
}
