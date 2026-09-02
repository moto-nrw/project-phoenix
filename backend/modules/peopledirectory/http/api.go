// Package users exposes the staff People Directory HTTP adapter under
// /api/users. It talks to the public capability only; rendering, auth,
// pagination and the foreign lookups it needs are injected by the root.
package users

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type Middleware = func(http.Handler) http.Handler

// FailureKind selects the response shape the root renders for an error; the
// wire format stays with the shared renderer so it matches every other
// staff endpoint.
type FailureKind string

const (
	FailureInvalidRequest FailureKind = "invalid_request"
	FailureNotFound       FailureKind = "not_found"
	FailureConflict       FailureKind = "conflict"
	FailureInternal       FailureKind = "internal"
)

type Pagination struct {
	Page     int
	PageSize int
	Total    int
}

// Runtime carries everything the adapter must not own: the protected route
// group, permission middleware, response rendering, and the two foreign
// lookups (accounts, RFID cards) the person form validates against.
type Runtime struct {
	Protected        func(chi.Router, func(chi.Router, Middleware))
	Permission       func(string) Middleware
	ParsePagination  func(*http.Request) (page, pageSize int)
	Success          func(http.ResponseWriter, *http.Request, int, any, string)
	SuccessPaginated func(http.ResponseWriter, *http.Request, int, any, Pagination, string)
	NoContent        func(http.ResponseWriter, *http.Request)
	Failure          func(http.ResponseWriter, *http.Request, FailureKind, error)
	// AccountEmail returns the e-mail of an account, or found=false.
	AccountEmail func(context.Context, int64) (email string, found bool, err error)
	// TagExists reports whether an RFID card with that identifier exists.
	TagExists       func(context.Context, string) (bool, error)
	ObserveResponse func(int, string)
}

type Resource struct {
	directory peopledirectory.Capability
	runtime   Runtime
}

func NewResource(directory peopledirectory.Capability, runtime Runtime) *Resource {
	if directory == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.ParsePagination == nil ||
		runtime.Success == nil || runtime.SuccessPaginated == nil || runtime.NoContent == nil || runtime.Failure == nil ||
		runtime.AccountEmail == nil || runtime.TagExists == nil || runtime.ObserveResponse == nil {
		panic("users HTTP: all dependencies are required")
	}
	return &Resource{directory: directory, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(rs.runtime.Permission(permissions.UsersRead), withTx).Get("/", rs.listPersons)
		protected.With(rs.runtime.Permission(permissions.UsersRead), withTx).Get("/{id}", rs.getPerson)
		protected.With(rs.runtime.Permission(permissions.UsersCreate), withTx).Post("/", rs.createPerson)
		protected.With(rs.runtime.Permission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updatePerson)
		protected.With(rs.runtime.Permission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deletePerson)
	})
	return router
}

// PersonResponse is the wire shape of one directory entry.
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

// PersonRequest is the create and update payload.
type PersonRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TagID     string `json:"tag_id,omitempty"`
	AccountID int64  `json:"account_id,omitempty"`
}

func (request *PersonRequest) Bind(*http.Request) error {
	if request.FirstName == "" {
		return errors.New("first name is required")
	}
	if request.LastName == "" {
		return errors.New("last name is required")
	}
	return nil
}

func (rs *Resource) listPersons(w http.ResponseWriter, r *http.Request) {
	page, pageSize := rs.runtime.ParsePagination(r)
	filter := peopledirectory.PersonFilter{
		FirstNamePrefix: r.URL.Query().Get("first_name"),
		LastNamePrefix:  r.URL.Query().Get("last_name"),
		TagID:           r.URL.Query().Get("tag_id"),
		Page:            page,
		PageSize:        pageSize,
	}
	persons, err := rs.directory.SearchPersons(r.Context(), filter)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	responses, err := rs.personResponses(r.Context(), persons)
	if err != nil {
		rs.failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	rs.runtime.SuccessPaginated(w, r, http.StatusOK, responses, Pagination{Page: page, PageSize: pageSize, Total: len(responses)}, "Persons retrieved successfully")
	rs.runtime.ObserveResponse(http.StatusOK, "none")
}

func (rs *Resource) getPerson(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseID(w, r)
	if !ok {
		return
	}
	person, err := rs.directory.FindPerson(r.Context(), id)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.respondPerson(w, r, http.StatusOK, person, "Person retrieved successfully")
}

func (rs *Resource) createPerson(w http.ResponseWriter, r *http.Request) {
	request := &PersonRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	input := peopledirectory.CreatePerson{FirstName: request.FirstName, LastName: request.LastName}
	if request.TagID != "" {
		input.TagID = &request.TagID
	}
	if request.AccountID != 0 {
		input.AccountID = &request.AccountID
	}
	if !rs.referencesExist(w, r, input.TagID, input.AccountID) {
		return
	}
	person, err := rs.directory.CreatePerson(r.Context(), input)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.respondPerson(w, r, http.StatusCreated, person, "Person created successfully")
}

func (rs *Resource) updatePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseID(w, r)
	if !ok {
		return
	}
	existing, err := rs.directory.FindPerson(r.Context(), id)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	request := &PersonRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	// An empty tag or account keeps the stored link; only a supplied value
	// re-points it (the previous handler merged the same way).
	input := peopledirectory.UpdatePerson{
		ID: id, FirstName: request.FirstName, LastName: request.LastName,
		Birthday: existing.Birthday, TagID: existing.TagID, AccountID: existing.AccountID,
	}
	var changedTag *string
	var changedAccount *int64
	if request.TagID != "" && (existing.TagID == nil || *existing.TagID != peopledirectory.NormalizeTagID(request.TagID)) {
		input.TagID = &request.TagID
		changedTag = &request.TagID
	}
	if request.AccountID != 0 && (existing.AccountID == nil || *existing.AccountID != request.AccountID) {
		input.AccountID = &request.AccountID
		changedAccount = &request.AccountID
	}
	if !rs.referencesExist(w, r, changedTag, changedAccount) {
		return
	}
	person, err := rs.directory.UpdatePerson(r.Context(), input)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.respondPerson(w, r, http.StatusOK, person, "Person updated successfully")
}

func (rs *Resource) deletePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseID(w, r)
	if !ok {
		return
	}
	if err := rs.directory.DeletePerson(r.Context(), id); err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.runtime.NoContent(w, r)
	rs.runtime.ObserveResponse(http.StatusNoContent, "none")
}

// referencesExist validates the foreign links a person form may carry. The
// directory owns persons only, so accounts and RFID cards are checked
// through the root-injected lookups before the write.
func (rs *Resource) referencesExist(w http.ResponseWriter, r *http.Request, tagID *string, accountID *int64) bool {
	if accountID != nil {
		_, found, err := rs.runtime.AccountEmail(r.Context(), *accountID)
		if err != nil {
			rs.failure(w, r, FailureInternal, err, "internal_error")
			return false
		}
		if !found {
			rs.failure(w, r, FailureNotFound, errors.New("account not found"), "account_not_found")
			return false
		}
	}
	if tagID != nil {
		found, err := rs.runtime.TagExists(r.Context(), peopledirectory.NormalizeTagID(*tagID))
		if err != nil {
			rs.failure(w, r, FailureInternal, err, "internal_error")
			return false
		}
		if !found {
			rs.failure(w, r, FailureNotFound, errors.New("RFID card not found"), "rfid_card_not_found")
			return false
		}
	}
	return true
}

func (rs *Resource) respondPerson(w http.ResponseWriter, r *http.Request, status int, person peopledirectory.Person, message string) {
	response, err := rs.personResponse(r.Context(), person)
	if err != nil {
		rs.failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	rs.runtime.Success(w, r, status, response, message)
	rs.runtime.ObserveResponse(status, "none")
}

func (rs *Resource) personResponses(ctx context.Context, persons []peopledirectory.Person) ([]PersonResponse, error) {
	responses := make([]PersonResponse, 0, len(persons))
	for _, person := range persons {
		response, err := rs.personResponse(ctx, person)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (rs *Resource) personResponse(ctx context.Context, person peopledirectory.Person) (PersonResponse, error) {
	email := ""
	if person.AccountID != nil {
		value, found, err := rs.runtime.AccountEmail(ctx, *person.AccountID)
		if err != nil {
			return PersonResponse{}, err
		}
		if found {
			email = value
		}
	}
	return newPersonResponse(person, email), nil
}

func (rs *Resource) parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		rs.failure(w, r, FailureInvalidRequest, errors.New("invalid person ID"), "invalid_parameters")
		return 0, false
	}
	return id, true
}

func (rs *Resource) moduleFailure(w http.ResponseWriter, r *http.Request, err error) {
	rs.failure(w, r, classifyModuleError(err), err, peopledirectory.ErrorCode(err))
}

func (rs *Resource) failure(w http.ResponseWriter, r *http.Request, kind FailureKind, err error, code string) {
	rs.runtime.Failure(w, r, kind, err)
	rs.runtime.ObserveResponse(statusOf(kind), code)
}

func classifyModuleError(err error) FailureKind {
	switch {
	case errors.Is(err, peopledirectory.ErrPersonNotFound):
		return FailureNotFound
	case errors.Is(err, peopledirectory.ErrInvalidPerson):
		return FailureInvalidRequest
	case errors.Is(err, peopledirectory.ErrTagConflict), errors.Is(err, peopledirectory.ErrAccountConflict):
		return FailureConflict
	default:
		return FailureInternal
	}
}

func statusOf(kind FailureKind) int {
	switch kind {
	case FailureInvalidRequest:
		return http.StatusBadRequest
	case FailureNotFound:
		return http.StatusNotFound
	case FailureConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func newPersonResponse(person peopledirectory.Person, email string) PersonResponse {
	response := PersonResponse{
		ID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
		Email: email, CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt,
	}
	if person.TagID != nil {
		response.TagID = *person.TagID
	}
	if person.AccountID != nil {
		response.AccountID = *person.AccountID
	}
	return response
}
