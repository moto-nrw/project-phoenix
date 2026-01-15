package users

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the users API resource
type Resource struct {
	PersonService usersSvc.PersonService
}

// NewResource creates a new users resource
func NewResource(personService usersSvc.PersonService) *Resource {
	return &Resource{
		PersonService: personService,
	}
}

// Router returns a configured router for user endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations only require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/", rs.listPersons)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}", rs.getPerson)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/by-tag/{tagId}", rs.getPersonByTag)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/search", rs.searchPersons)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/by-account/{accountId}", rs.getPersonByAccount)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/rfid-cards/available", rs.listAvailableRFIDCards)

		// Write operations require specific permissions
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.createPerson)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updatePerson)
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deletePerson)

		// Special operations
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}/rfid", rs.linkRFID)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Delete("/{id}/rfid", rs.unlinkRFID)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}/account", rs.linkAccount)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Delete("/{id}/account", rs.unlinkAccount)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/profile", rs.getFullProfile)
	})

	return r
}
