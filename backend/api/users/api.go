package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
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
// Note: Authentication is handled by tenant middleware in base.go when TENANT_AUTH_ENABLED=true
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Read operations only require person:read permission
	r.With(tenant.RequiresPermission("person:read")).Get("/", rs.listPersons)
	r.With(tenant.RequiresPermission("person:read")).Get("/{id}", rs.getPerson)
	r.With(tenant.RequiresPermission("person:read")).Get("/by-tag/{tagId}", rs.getPersonByTag)
	r.With(tenant.RequiresPermission("person:read")).Get("/search", rs.searchPersons)
	r.With(tenant.RequiresPermission("person:read")).Get("/by-account/{accountId}", rs.getPersonByAccount)
	r.With(tenant.RequiresPermission("person:read")).Get("/rfid-cards/available", rs.listAvailableRFIDCards)

	// Write operations require specific permissions
	r.With(tenant.RequiresPermission("person:create")).Post("/", rs.createPerson)
	r.With(tenant.RequiresPermission("person:update")).Put("/{id}", rs.updatePerson)
	r.With(tenant.RequiresPermission("person:delete")).Delete("/{id}", rs.deletePerson)

	// Special operations
	r.With(tenant.RequiresPermission("person:update")).Put("/{id}/rfid", rs.linkRFID)
	r.With(tenant.RequiresPermission("person:update")).Delete("/{id}/rfid", rs.unlinkRFID)
	r.With(tenant.RequiresPermission("person:update")).Put("/{id}/account", rs.linkAccount)
	r.With(tenant.RequiresPermission("person:update")).Delete("/{id}/account", rs.unlinkAccount)
	r.With(tenant.RequiresPermission("person:read")).Get("/{id}/profile", rs.getFullProfile)

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

// GetPersonByTagHandler returns the getPersonByTag handler for testing
func (rs *Resource) GetPersonByTagHandler() http.HandlerFunc {
	return rs.getPersonByTag
}

// SearchPersonsHandler returns the searchPersons handler for testing
func (rs *Resource) SearchPersonsHandler() http.HandlerFunc {
	return rs.searchPersons
}

// GetPersonByAccountHandler returns the getPersonByAccount handler for testing
func (rs *Resource) GetPersonByAccountHandler() http.HandlerFunc {
	return rs.getPersonByAccount
}

// ListAvailableRFIDCardsHandler returns the listAvailableRFIDCards handler for testing
func (rs *Resource) ListAvailableRFIDCardsHandler() http.HandlerFunc {
	return rs.listAvailableRFIDCards
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

// LinkRFIDHandler returns the linkRFID handler for testing
func (rs *Resource) LinkRFIDHandler() http.HandlerFunc {
	return rs.linkRFID
}

// UnlinkRFIDHandler returns the unlinkRFID handler for testing
func (rs *Resource) UnlinkRFIDHandler() http.HandlerFunc {
	return rs.unlinkRFID
}

// LinkAccountHandler returns the linkAccount handler for testing
func (rs *Resource) LinkAccountHandler() http.HandlerFunc {
	return rs.linkAccount
}

// UnlinkAccountHandler returns the unlinkAccount handler for testing
func (rs *Resource) UnlinkAccountHandler() http.HandlerFunc {
	return rs.unlinkAccount
}

// GetFullProfileHandler returns the getFullProfile handler for testing
func (rs *Resource) GetFullProfileHandler() http.HandlerFunc {
	return rs.getFullProfile
}
