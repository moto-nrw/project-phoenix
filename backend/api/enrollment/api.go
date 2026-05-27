// Package enrollment holds the parent-enrollment HTTP layer. PR 5 ships
// the admin form-schema CRUD; PR 7 will add public submission +
// status/edit endpoints; PR 8 admin decision endpoints.
package enrollment

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource bundles the handler methods + their dependencies.
type Resource struct {
	FormSchemaService         enrollmentService.FormSchemaService
	CareOfferingService       enrollmentService.CareOfferingService
	RequestService            enrollmentService.RequestService
	CaptchaService            enrollmentService.CaptchaService
	PhaseService              enrollmentService.PhaseService
	DecisionService           enrollmentService.DecisionService
	RolloverService           enrollmentService.RolloverService
	GuardianInvitationService authService.GuardianInvitationService
	GuardianProfileLoader     usersService.GuardianProfileLoader
	SchoolRepo                platformModels.SchoolRepository
	PhaseRepo                 enrollmentModels.PhaseRepository
	db                        *bun.DB
}

// NewResource constructs the enrollment API resource. PR 7 added the
// RequestService + CaptchaService for the public submission flow.
// PR A of the phase model wires PhaseService + PhaseRepo so the public
// + admin endpoints can resolve phase rows. PR 8 wires DecisionService
// for the admin review/accept/reject UI; slice 2 also wires the
// GuardianInvitationService so post-approval invites can fire.
func NewResource(
	formSchemaSvc enrollmentService.FormSchemaService,
	careOfferingSvc enrollmentService.CareOfferingService,
	requestSvc enrollmentService.RequestService,
	captchaSvc enrollmentService.CaptchaService,
	phaseSvc enrollmentService.PhaseService,
	decisionSvc enrollmentService.DecisionService,
	rolloverSvc enrollmentService.RolloverService,
	guardianInvitationSvc authService.GuardianInvitationService,
	guardianProfileLoader usersService.GuardianProfileLoader,
	schoolRepo platformModels.SchoolRepository,
	phaseRepo enrollmentModels.PhaseRepository,
	db *bun.DB,
) *Resource {
	return &Resource{
		FormSchemaService:         formSchemaSvc,
		CareOfferingService:       careOfferingSvc,
		RequestService:            requestSvc,
		CaptchaService:            captchaSvc,
		PhaseService:              phaseSvc,
		DecisionService:           decisionSvc,
		RolloverService:           rolloverSvc,
		GuardianInvitationService: guardianInvitationSvc,
		GuardianProfileLoader:     guardianProfileLoader,
		SchoolRepo:                schoolRepo,
		PhaseRepo:                 phaseRepo,
		db:                        db,
	}
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
	r.Get("/schema/public/{tenantSlug}/{phaseId}", rs.listPublicActiveSchema)
	r.Get("/captcha-config/{tenantSlug}", rs.publicCaptchaConfig)
	r.Get("/legal/{tenantSlug}", rs.publicLegalTexts)
	r.Post("/{tenantSlug}/submit", rs.submitEnrollment)
	r.Get("/requests/{statusToken}", rs.getStatus)
	r.Patch("/requests/{statusToken}", rs.patchStatus)
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
			r.With(authorize.RequiresPermission("config:read")).Get("/{id}", rs.getSchemaByID)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.publishSchema)
			r.With(authorize.RequiresPermission("config:manage")).Put("/{id}", rs.updateSchema)
			r.With(authorize.RequiresPermission("config:manage")).Delete("/{id}", rs.deleteSchema)
		})

		r.Route("/care-offerings", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.listCareOfferings)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.createCareOffering)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getCareOffering)
				r.With(authorize.RequiresPermission("config:manage")).Put("/", rs.updateCareOffering)
				r.With(authorize.RequiresPermission("config:manage")).Delete("/", rs.deleteCareOffering)
				r.With(authorize.RequiresPermission("config:manage")).Post("/clone", rs.cloneCareOffering)
			})
		})

		r.Route("/phases", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.listPhases)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.createPhase)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getPhase)
				r.With(authorize.RequiresPermission("config:manage")).Put("/", rs.updatePhase)
				r.With(authorize.RequiresPermission("config:manage")).Delete("/", rs.deletePhase)
				// Rollover (phase renewal). createRollover carries
				// approved enrollments from this phase forward into a
				// new phase; listRolloverReview surfaces children that
				// landed in pending_admin_review on a phase created
				// FROM this phase. Both require config:manage.
				r.With(authorize.RequiresPermission("config:manage")).Post("/rollover", rs.createRollover)
				r.With(authorize.RequiresPermission("config:read")).Get("/review", rs.listRolloverReview)
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

		// PR 8 admin review surface. config:read for browse,
		// config:manage to apply decisions. Decision writes audit
		// reviewed_by/reviewed_at on each child row.
		r.Route("/admin/requests", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.listAdminRequests)
			r.Route("/{id}", func(r chi.Router) {
				r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getAdminRequest)
				r.With(authorize.RequiresPermission("config:manage")).Post("/children/{childId}/decide", rs.decideAdminChild)
			})
		})
	})

	return r
}
