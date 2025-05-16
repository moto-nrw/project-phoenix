package users

import (
	"log"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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

// PersonProfileResponse represents a full person profile response
type PersonProfileResponse struct {
	PersonResponse
	RFIDCard *RFIDCardResponse `json:"rfid_card,omitempty"`
	Account  *AccountResponse  `json:"account,omitempty"`
}

// RFIDCardResponse represents an RFID card response
type RFIDCardResponse struct {
	TagID     string    `json:"tag_id"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AccountResponse represents a simplified account response
type AccountResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active"`
}

// PersonRequest represents a person creation/update request
type PersonRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TagID     string `json:"tag_id,omitempty"`
	AccountID int64  `json:"account_id,omitempty"`
}

// RFIDLinkRequest represents an RFID card link request
type RFIDLinkRequest struct {
	TagID string `json:"tag_id"`
}

// AccountLinkRequest represents an account link request
type AccountLinkRequest struct {
	AccountID int64 `json:"account_id"`
}

// Bind validates the person request
func (req *PersonRequest) Bind(r *http.Request) error {
	// Basic validation
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}
	// At least one of TagID or AccountID should be provided
	if req.TagID == "" && req.AccountID == 0 {
		return errors.New("either tag ID or account ID must be provided")
	}
	return nil
}

// Bind validates the RFID link request
func (req *RFIDLinkRequest) Bind(r *http.Request) error {
	if req.TagID == "" {
		return errors.New("tag ID is required")
	}
	return nil
}

// Bind validates the account link request
func (req *AccountLinkRequest) Bind(r *http.Request) error {
	if req.AccountID == 0 {
		return errors.New("account ID is required")
	}
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

// newPersonProfileResponse creates a full person profile response
func newPersonProfileResponse(person *users.Person) PersonProfileResponse {
	response := PersonProfileResponse{
		PersonResponse: newPersonResponse(person),
	}

	// Add RFID card if available
	if person.RFIDCard != nil {
		response.RFIDCard = &RFIDCardResponse{
			TagID:     person.RFIDCard.ID,
			IsActive:  person.RFIDCard.Active,
			CreatedAt: person.RFIDCard.CreatedAt,
			UpdatedAt: person.RFIDCard.UpdatedAt,
		}
	}

	// Add account if available
	if person.Account != nil {
		response.Account = &AccountResponse{
			ID:       person.Account.ID,
			Email:    person.Account.Email,
			IsActive: person.Account.Active,
		}
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
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	queryOptions.WithPagination(page, pageSize)
	queryOptions.Filter = filter

	// Get persons from service
	persons, err := rs.PersonService.List(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response objects
	responses := make([]PersonResponse, len(persons))
	for i, person := range persons {
		responses[i] = newPersonResponse(person)
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Persons retrieved successfully")
}

// getPerson handles getting a person by ID
func (rs *Resource) getPerson(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person from service
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Person retrieved successfully")
}

// getPersonByTag handles getting a person by RFID tag ID
func (rs *Resource) getPersonByTag(w http.ResponseWriter, r *http.Request) {
	// Get tag ID from URL
	tagID := chi.URLParam(r, "tagId")
	if tagID == "" {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("tag ID is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person from service
	person, err := rs.PersonService.FindByTagID(r.Context(), tagID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Person retrieved successfully")
}

// getPersonByAccount handles getting a person by account ID
func (rs *Resource) getPersonByAccount(w http.ResponseWriter, r *http.Request) {
	// Parse account ID from URL
	accountID, err := strconv.ParseInt(chi.URLParam(r, "accountId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person from service
	person, err := rs.PersonService.FindByAccountID(r.Context(), accountID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Person retrieved successfully")
}

// searchPersons handles searching for persons by name
func (rs *Resource) searchPersons(w http.ResponseWriter, r *http.Request) {
	// Get search parameters
	firstName := r.URL.Query().Get("first_name")
	lastName := r.URL.Query().Get("last_name")

	// At least one search parameter is required
	if firstName == "" && lastName == "" {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("at least one search parameter is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Search for persons by name
	persons, err := rs.PersonService.FindByName(r.Context(), firstName, lastName)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response objects
	responses := make([]PersonResponse, len(persons))
	for i, person := range persons {
		responses[i] = newPersonResponse(person)
	}

	common.Respond(w, r, http.StatusOK, responses, "Persons retrieved successfully")
}

// createPerson handles creating a new person
func (rs *Resource) createPerson(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &PersonRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, common.ErrorInvalidRequest(err))
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
	if err := rs.PersonService.Create(r.Context(), person); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newPersonResponse(person), "Person created successfully")
}

// updatePerson handles updating an existing person
func (rs *Resource) updatePerson(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get existing person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &PersonRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, common.ErrorInvalidRequest(err))
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
	if err := rs.PersonService.Update(r.Context(), person); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Person updated successfully")
}

// deletePerson handles deleting a person
func (rs *Resource) deletePerson(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete person using service
	if err := rs.PersonService.Delete(r.Context(), id); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// linkRFID handles linking a person to an RFID card
func (rs *Resource) linkRFID(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &RFIDLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Link RFID card to person
	if err := rs.PersonService.LinkToRFIDCard(r.Context(), id, req.TagID); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "RFID card linked successfully")
}

// unlinkRFID handles unlinking an RFID card from a person
func (rs *Resource) unlinkRFID(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Unlink RFID card from person
	if err := rs.PersonService.UnlinkFromRFIDCard(r.Context(), id); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "RFID card unlinked successfully")
}

// linkAccount handles linking a person to an account
func (rs *Resource) linkAccount(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &AccountLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Link account to person
	if err := rs.PersonService.LinkToAccount(r.Context(), id, req.AccountID); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Account linked successfully")
}

// unlinkAccount handles unlinking an account from a person
func (rs *Resource) unlinkAccount(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Unlink account from person
	if err := rs.PersonService.UnlinkFromAccount(r.Context(), id); err != nil {
		render.Render(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Account unlinked successfully")
}

// getFullProfile handles getting a person's full profile
func (rs *Resource) getFullProfile(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get full profile from service
	person, err := rs.PersonService.GetFullProfile(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonProfileResponse(person), "Person profile retrieved successfully")
}
