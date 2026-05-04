package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

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

// =============================================================================
// EXPORTED HANDLER METHODS FOR TESTING
// =============================================================================

// ListPersonsHandler returns the listPersons handler for testing
func (rs *Resource) ListPersonsHandler() http.HandlerFunc {
	return rs.listPersons
}

// GetPersonHandler returns the getPerson handler for testing
func (rs *Resource) GetPersonHandler() http.HandlerFunc {
	return rs.getPerson
}

// CreatePersonHandler returns the createPerson handler for testing
func (rs *Resource) CreatePersonHandler() http.HandlerFunc {
	return rs.createPerson
}

// UpdatePersonHandler returns the updatePerson handler for testing
func (rs *Resource) UpdatePersonHandler() http.HandlerFunc {
	return rs.updatePerson
}

// DeletePersonHandler returns the deletePerson handler for testing
func (rs *Resource) DeletePersonHandler() http.HandlerFunc {
	return rs.deletePerson
}
