package users

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Constants for response messages (S1192 - avoid duplicate string literals)
const (
	msgPersonRetrieved = "Person retrieved successfully"
)

// PersonResponse represents a simplified person response
type PersonResponse struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email,omitempty"`
	TagID     string    `json:"tag_id,omitempty"`
	AccountID int64     `json:"account_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PersonRequest represents a person creation/update request
type PersonRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TagID     string `json:"tag_id,omitempty"`
	AccountID int64  `json:"account_id,omitempty"`
}

// Bind validates the person request
func (req *PersonRequest) Bind(_ *http.Request) error {
	// Basic validation
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}
	// Note: TagID and AccountID are optional - they can be linked later
	// This aligns with service layer validation (see person_service.go:188-189)
	return nil
}

// newPersonResponse creates a person response from a person model
func newPersonResponse(person *users.Person) PersonResponse {
	response := PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}

	if person.TagID != nil {
		response.TagID = *person.TagID
	}

	if person.AccountID != nil {
		response.AccountID = *person.AccountID
	}

	// If account information is available
	if person.Account != nil {
		response.Email = person.Account.Email
	}

	return response
}

// listPersons handles listing all persons with optional filtering
func (rs *Resource) listPersons(w http.ResponseWriter, r *http.Request) {
	// Create query options with filters
	queryOptions := base.NewQueryOptions()
	filter := base.NewFilter()

	// Add filters from query parameters
	if firstName := r.URL.Query().Get("first_name"); firstName != "" {
		filter.ILike("first_name", firstName+"%")
	}

	if lastName := r.URL.Query().Get("last_name"); lastName != "" {
		filter.ILike("last_name", lastName+"%")
	}

	if tagID := r.URL.Query().Get("tag_id"); tagID != "" {
		filter.Equal("tag_id", tagID)
	}

	// Add pagination
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)
	queryOptions.Filter = filter

	// Get persons from service
	persons, err := rs.PersonService.List(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Convert to response objects
	responses := make([]PersonResponse, len(persons))
	for i, person := range persons {
		responses[i] = newPersonResponse(person)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Persons retrieved successfully")
}

// getPerson handles getting a person by ID
func (rs *Resource) getPerson(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Get person from service
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), msgPersonRetrieved)
}

// createPerson handles creating a new person
func (rs *Resource) createPerson(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &PersonRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Create person model
	person := &users.Person{
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	// Set optional fields
	if req.TagID != "" {
		tagID := req.TagID
		person.TagID = &tagID
	}

	if req.AccountID != 0 {
		accountID := req.AccountID
		person.AccountID = &accountID
	}

	// Create person using service
	tenantID := tenant.FromContext(r.Context())
	err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.PersonService.Create(ctx, person)
	})
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newPersonResponse(person), "Person created successfully")
}

// updatePerson handles updating an existing person
func (rs *Resource) updatePerson(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Get existing person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Parse request
	req := &PersonRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Update person fields
	person.FirstName = req.FirstName
	person.LastName = req.LastName

	// Update optional fields
	if req.TagID != "" {
		tagID := req.TagID
		person.TagID = &tagID
	}

	if req.AccountID != 0 {
		accountID := req.AccountID
		person.AccountID = &accountID
	}

	// Update person using service
	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.PersonService.Update(ctx, person)
	})
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Person updated successfully")
}

// deletePerson handles deleting a person
func (rs *Resource) deletePerson(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Delete person using service
	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.PersonService.Delete(ctx, id)
	})
	if err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Person kann nicht gelöscht werden: Person hat verknüpfte Personal-, Kinder- oder Kontodaten"))
			return
		}
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.RespondNoContent(w, r)
}
