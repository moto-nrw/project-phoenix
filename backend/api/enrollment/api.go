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
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// Resource bundles the handler methods + their dependencies.
type Resource struct {
	FormSchemaService   enrollmentService.FormSchemaService
	CareOfferingService enrollmentService.CareOfferingService
	SchoolRepo          platformModels.SchoolRepository
	db                  *bun.DB
}

// NewResource constructs the enrollment API resource.
func NewResource(formSchemaSvc enrollmentService.FormSchemaService, careOfferingSvc enrollmentService.CareOfferingService, schoolRepo platformModels.SchoolRepository, db *bun.DB) *Resource {
	return &Resource{
		FormSchemaService:   formSchemaSvc,
		CareOfferingService: careOfferingSvc,
		SchoolRepo:          schoolRepo,
		db:                  db,
	}
}

// Router returns a chi router scoped to /enrollment. PR 5 added the
// admin form-schema endpoints; PR 6 adds care-offering admin CRUD +
// the public open-window endpoint.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public route: parent-facing care offerings. No JWT — slug-gated
	// in the handler. Sits outside the auth group below so the JWT
	// middleware doesn't reject anonymous requests.
	r.Get("/care-offerings/public/{tenantSlug}", rs.listPublicCareOfferings)

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
	})

	return r
}
