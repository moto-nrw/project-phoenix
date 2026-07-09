package users

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the users API resource
type Resource struct {
	PersonService usersSvc.PersonService
	db            *bun.DB
}

// NewResource creates a new users resource
func NewResource(personService usersSvc.PersonService, db *bun.DB) *Resource {
	return &Resource{
		PersonService: personService,
		db:            db,
	}
}

// Router returns a configured router for user endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Read operations only require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listPersons)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getPerson)

		// Write operations require specific permissions
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createPerson)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updatePerson)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deletePerson)
	})

	return r
}
