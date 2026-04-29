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
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// Resource bundles the handler methods + their dependencies.
type Resource struct {
	FormSchemaService enrollmentService.FormSchemaService
	db                *bun.DB
}

// NewResource constructs the enrollment API resource.
func NewResource(formSchemaSvc enrollmentService.FormSchemaService, db *bun.DB) *Resource {
	return &Resource{
		FormSchemaService: formSchemaSvc,
		db:                db,
	}
}

// Router returns a chi router scoped to /enrollment. PR 5 only registers
// the admin form-schema endpoints; later PRs add their own routes here.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// All enrollment admin endpoints require a tenant JWT + the
	// config:update permission (same as the operations-tab settings).
	// Sensitive operations (publishing a new schema) elevate to
	// config:manage.
	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		// Hook in the standard JWT chain. The base.go API setup wraps
		// this router into the global tenant tx middleware via the
		// /api mount.
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)

		r.Route("/schema", func(r chi.Router) {
			r.With(authorize.RequiresPermission("config:read")).Get("/", rs.getActiveSchema)
			r.With(authorize.RequiresPermission("config:read")).Get("/versions", rs.listSchemaVersions)
			r.With(authorize.RequiresPermission("config:read")).Get("/{id}", rs.getSchemaByID)
			r.With(authorize.RequiresPermission("config:manage")).Post("/", rs.publishSchema)
		})
	})

	return r
}
