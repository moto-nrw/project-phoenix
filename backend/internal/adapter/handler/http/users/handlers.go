package users

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

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

// getPersonByTag handles getting a person by RFID tag ID
func (rs *Resource) getPersonByTag(w http.ResponseWriter, r *http.Request) {
	// Get tag ID from URL
	tagID := chi.URLParam(r, "tagId")
	if tagID == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tag ID is required")))
		return
	}

	// Get person from service
	person, err := rs.PersonService.FindByTagID(r.Context(), tagID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), msgPersonRetrieved)
}

// getPersonByAccount handles getting a person by account ID
func (rs *Resource) getPersonByAccount(w http.ResponseWriter, r *http.Request) {
	// Parse account ID from URL
	accountID, err := common.ParseIDParam(r, "accountId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid account ID")))
		return
	}

	// Get person from service
	person, err := rs.PersonService.FindByAccountID(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), msgPersonRetrieved)
}

// searchPersons handles searching for persons by name
func (rs *Resource) searchPersons(w http.ResponseWriter, r *http.Request) {
	// Get search parameters
	firstName := r.URL.Query().Get("first_name")
	lastName := r.URL.Query().Get("last_name")

	// At least one search parameter is required
	if firstName == "" && lastName == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("at least one search parameter is required")))
		return
	}

	// Search for persons by name
	persons, err := rs.PersonService.FindByName(r.Context(), firstName, lastName)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
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
	if err := rs.PersonService.Create(r.Context(), person); err != nil {
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
	if err := rs.PersonService.Update(r.Context(), person); err != nil {
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
	if err := rs.PersonService.Delete(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// linkRFID handles linking a person to an RFID card
func (rs *Resource) linkRFID(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Parse request
	req := &RFIDLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Link RFID card to person
	if err := rs.PersonService.LinkToRFIDCard(r.Context(), id, req.TagID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "RFID card linked successfully")
}

// unlinkRFID handles unlinking an RFID card from a person
func (rs *Resource) unlinkRFID(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Unlink RFID card from person
	if err := rs.PersonService.UnlinkFromRFIDCard(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "RFID card unlinked successfully")
}

// linkAccount handles linking a person to an account
func (rs *Resource) linkAccount(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Parse request
	req := &AccountLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Link account to person
	if err := rs.PersonService.LinkToAccount(r.Context(), id, req.AccountID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Account linked successfully")
}

// unlinkAccount handles unlinking an account from a person
func (rs *Resource) unlinkAccount(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Unlink account from person
	if err := rs.PersonService.UnlinkFromAccount(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get updated person
	person, err := rs.PersonService.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonResponse(person), "Account unlinked successfully")
}

// getFullProfile handles getting a person's full profile
func (rs *Resource) getFullProfile(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidPersonID)))
		return
	}

	// Get full profile from service
	person, err := rs.PersonService.GetFullProfile(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPersonProfileResponse(person), "Person profile retrieved successfully")
}

// listAvailableRFIDCards handles listing RFID cards that are not assigned to any person
func (rs *Resource) listAvailableRFIDCards(w http.ResponseWriter, r *http.Request) {
	// Get available RFID cards from service
	cards, err := rs.PersonService.ListAvailableRFIDCards(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Convert to response objects
	responses := make([]RFIDCardResponse, len(cards))
	for i, card := range cards {
		responses[i] = RFIDCardResponse{
			TagID:     card.ID,
			IsActive:  card.Active,
			CreatedAt: card.CreatedAt,
			UpdatedAt: card.UpdatedAt,
		}
	}

	common.Respond(w, r, http.StatusOK, responses, "Available RFID cards retrieved successfully")
}
