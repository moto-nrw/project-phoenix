package users_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDirectory records the calls the adapter makes and answers from a
// small in-memory map so every handler path can be driven without a DB.
type fakeDirectory struct {
	peopledirectory.Capability
	persons  map[int64]peopledirectory.Person
	searched peopledirectory.PersonFilter
	created  peopledirectory.CreatePerson
	updated  peopledirectory.UpdatePerson
	deleted  int64
	err      error
}

func (f *fakeDirectory) SearchPersons(_ context.Context, filter peopledirectory.PersonFilter) ([]peopledirectory.Person, error) {
	f.searched = filter
	if f.err != nil {
		return nil, f.err
	}
	result := make([]peopledirectory.Person, 0, len(f.persons))
	for _, person := range f.persons {
		result = append(result, person)
	}
	return result, nil
}

func (f *fakeDirectory) FindPerson(_ context.Context, id int64) (peopledirectory.Person, error) {
	if f.err != nil {
		return peopledirectory.Person{}, f.err
	}
	person, ok := f.persons[id]
	if !ok {
		return peopledirectory.Person{}, peopledirectory.ErrPersonNotFound
	}
	return person, nil
}

func (f *fakeDirectory) CreatePerson(_ context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	f.created = input
	if f.err != nil {
		return peopledirectory.Person{}, f.err
	}
	return peopledirectory.Person{ID: 42, FirstName: input.FirstName, LastName: input.LastName, TagID: input.TagID, AccountID: input.AccountID}, nil
}

func (f *fakeDirectory) UpdatePerson(_ context.Context, input peopledirectory.UpdatePerson) (peopledirectory.Person, error) {
	f.updated = input
	if f.err != nil {
		return peopledirectory.Person{}, f.err
	}
	return peopledirectory.Person{ID: input.ID, FirstName: input.FirstName, LastName: input.LastName, TagID: input.TagID, AccountID: input.AccountID}, nil
}

func (f *fakeDirectory) DeletePerson(_ context.Context, id int64) error {
	f.deleted = id
	return f.err
}

type response struct {
	Status     string          `json:"status"`
	Data       json.RawMessage `json:"data,omitempty"`
	Message    string          `json:"message,omitempty"`
	Pagination *struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
		Total    int `json:"total"`
	} `json:"pagination,omitempty"`
}

func (response) Render(http.ResponseWriter, *http.Request) error { return nil }

type errorResponse struct {
	StatusCode int    `json:"-"`
	Status     string `json:"status"`
	Kind       string `json:"kind"`
	Error      string `json:"error"`
}

func (e errorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.StatusCode)
	return nil
}

type harness struct {
	directory *fakeDirectory
	router    chi.Router
	observed  []string
	accounts  map[int64]string
	lookups   [][]int64
	tags      map[string]bool
	permitted map[string]bool
	txCalls   int
}

func newHarness(t *testing.T, directory *fakeDirectory) *harness {
	t.Helper()
	h := &harness{directory: directory, accounts: map[int64]string{}, tags: map[string]bool{}, permitted: map[string]bool{}}
	statusOf := map[usersHTTP.FailureKind]int{
		usersHTTP.FailureInvalidRequest: http.StatusBadRequest,
		usersHTTP.FailureNotFound:       http.StatusNotFound,
		usersHTTP.FailureConflict:       http.StatusConflict,
		usersHTTP.FailureInternal:       http.StatusInternalServerError,
	}
	resource := usersHTTP.NewResource(directory, usersHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, usersHTTP.Middleware)) {
			register(router, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					h.txCalls++
					next.ServeHTTP(w, r)
				})
			})
		},
		Permission: func(permission string) usersHTTP.Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !h.permitted[permission] {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
				})
			}
		},
		ParsePagination: func(r *http.Request) (int, int) { return 1, 20 },
		Success: func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
			payload, _ := json.Marshal(data)
			render.Status(r, status)
			_ = render.Render(w, r, response{Status: "success", Data: payload, Message: message})
		},
		SuccessPaginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination usersHTTP.Pagination, message string) {
			payload, _ := json.Marshal(data)
			render.Status(r, status)
			_ = render.Render(w, r, response{Status: "success", Data: payload, Message: message, Pagination: &struct {
				Page     int `json:"page"`
				PageSize int `json:"page_size"`
				Total    int `json:"total"`
			}{pagination.Page, pagination.PageSize, pagination.Total}})
		},
		NoContent: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
		Failure: func(w http.ResponseWriter, r *http.Request, kind usersHTTP.FailureKind, err error) {
			_ = render.Render(w, r, errorResponse{StatusCode: statusOf[kind], Status: "error", Kind: string(kind), Error: err.Error()})
		},
		AccountEmails: func(_ context.Context, ids []int64) (map[int64]string, error) {
			h.lookups = append(h.lookups, ids)
			emails := make(map[int64]string, len(ids))
			for _, id := range ids {
				if email, found := h.accounts[id]; found {
					emails[id] = email
				}
			}
			return emails, nil
		},
		TagExists: func(_ context.Context, tag string) (bool, error) { return h.tags[tag], nil },
		ObserveResponse: func(status int, code string) {
			h.observed = append(h.observed, http.StatusText(status)+":"+code)
		},
	})
	h.router = chi.NewRouter()
	h.router.Mount("/users", resource.Router())
	return h
}

func (h *harness) do(t *testing.T, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder) response {
	t.Helper()
	var body response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var body errorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func TestListPersonsForwardsFiltersAndPaginates(t *testing.T) {
	t.Parallel()
	tag := "ABC"
	var accountID int64 = 99
	var secondAccountID int64 = 100
	directory := &fakeDirectory{persons: map[int64]peopledirectory.Person{
		7: {ID: 7, FirstName: "Mia", LastName: "Muster", TagID: &tag, AccountID: &accountID},
		8: {ID: 8, FirstName: "Noah", LastName: "Muster", AccountID: &secondAccountID},
	}}
	h := newHarness(t, directory)
	h.permitted["users:read"] = true
	h.accounts[accountID] = "mia@example.com"
	h.accounts[secondAccountID] = "noah@example.com"

	recorder := h.do(t, http.MethodGet, "/users?first_name=Mi&last_name=Mu&tag_id=ABC", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, peopledirectory.PersonFilter{FirstNamePrefix: "Mi", LastNamePrefix: "Mu", TagID: "ABC", Page: 1, PageSize: 20}, directory.searched)
	body := decode(t, recorder)
	require.NotNil(t, body.Pagination)
	assert.Equal(t, 2, body.Pagination.Total)
	assert.Contains(t, string(body.Data), `"tag_id":"ABC"`)
	assert.Contains(t, string(body.Data), `"email":"mia@example.com"`)
	assert.Contains(t, string(body.Data), `"email":"noah@example.com"`)
	require.Len(t, h.lookups, 1)
	assert.ElementsMatch(t, []int64{accountID, secondAccountID}, h.lookups[0])
	assert.Equal(t, 1, h.txCalls, "the protected group wraps the handler in the injected transaction middleware")
}

func TestListPersonsWithoutPermissionIsForbidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t, &fakeDirectory{persons: map[int64]peopledirectory.Person{}})

	recorder := h.do(t, http.MethodGet, "/users", nil)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Zero(t, h.txCalls)
}

func TestGetPersonAddsAccountEmailAndMapsNotFound(t *testing.T) {
	t.Parallel()
	var accountID int64 = 99
	directory := &fakeDirectory{persons: map[int64]peopledirectory.Person{7: {ID: 7, FirstName: "Mia", LastName: "Muster", AccountID: &accountID}}}
	h := newHarness(t, directory)
	h.permitted["users:read"] = true
	h.accounts[accountID] = "mia@example.com"

	recorder := h.do(t, http.MethodGet, "/users/7", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, string(decode(t, recorder).Data), `"email":"mia@example.com"`)

	missing := h.do(t, http.MethodGet, "/users/8", nil)
	assert.Equal(t, http.StatusNotFound, missing.Code)
	assert.Equal(t, "person not found", decodeError(t, missing).Error)

	invalid := h.do(t, http.MethodGet, "/users/abc", nil)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
	assert.Equal(t, "invalid person ID", decodeError(t, invalid).Error)
	assert.Contains(t, h.observed, "Not Found:not_found")
	assert.Contains(t, h.observed, "Bad Request:invalid_parameters")
}

func TestCreatePersonValidatesBodyAndForeignReferences(t *testing.T) {
	t.Parallel()
	directory := &fakeDirectory{persons: map[int64]peopledirectory.Person{}}
	h := newHarness(t, directory)
	h.permitted["users:create"] = true
	h.accounts[5] = "five@example.com"
	h.tags["AABB"] = true

	missingName := h.do(t, http.MethodPost, "/users", map[string]any{"last_name": "Only"})
	assert.Equal(t, http.StatusBadRequest, missingName.Code)
	assert.Equal(t, "first name is required", decodeError(t, missingName).Error)

	unknownAccount := h.do(t, http.MethodPost, "/users", map[string]any{"first_name": "A", "last_name": "B", "account_id": 6})
	assert.Equal(t, http.StatusNotFound, unknownAccount.Code)
	assert.Equal(t, "account not found", decodeError(t, unknownAccount).Error)

	unknownTag := h.do(t, http.MethodPost, "/users", map[string]any{"first_name": "A", "last_name": "B", "tag_id": "zz:zz"})
	assert.Equal(t, http.StatusNotFound, unknownTag.Code)
	assert.Equal(t, "RFID card not found", decodeError(t, unknownTag).Error)

	created := h.do(t, http.MethodPost, "/users", map[string]any{"first_name": "A", "last_name": "B", "account_id": 5, "tag_id": "aa:bb"})
	require.Equal(t, http.StatusCreated, created.Code)
	require.NotNil(t, directory.created.AccountID)
	assert.EqualValues(t, 5, *directory.created.AccountID)
	assert.Contains(t, string(decode(t, created).Data), `"email":"five@example.com"`)

	plain := h.do(t, http.MethodPost, "/users", map[string]any{"first_name": "No", "last_name": "Links"})
	assert.Equal(t, http.StatusCreated, plain.Code)
	assert.Nil(t, directory.created.AccountID)
	assert.Nil(t, directory.created.TagID)
}

func TestUpdatePersonKeepsStoredLinksWhenBodyOmitsThem(t *testing.T) {
	t.Parallel()
	var accountID int64 = 5
	tag := "AABB"
	directory := &fakeDirectory{persons: map[int64]peopledirectory.Person{7: {ID: 7, FirstName: "Old", LastName: "Name", AccountID: &accountID, TagID: &tag}}}
	h := newHarness(t, directory)
	h.permitted["users:update"] = true
	h.accounts[5] = "five@example.com"
	h.accounts[6] = "six@example.com"

	recorder := h.do(t, http.MethodPut, "/users/7", map[string]any{"first_name": "New", "last_name": "Name"})
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, directory.updated.AccountID)
	assert.EqualValues(t, 5, *directory.updated.AccountID)
	require.NotNil(t, directory.updated.TagID)
	assert.Equal(t, "AABB", *directory.updated.TagID)

	repointed := h.do(t, http.MethodPut, "/users/7", map[string]any{"first_name": "New", "last_name": "Name", "account_id": 6})
	require.Equal(t, http.StatusOK, repointed.Code)
	assert.EqualValues(t, 6, *directory.updated.AccountID)
	assert.Contains(t, string(decode(t, repointed).Data), `"email":"six@example.com"`)

	unknownAccount := h.do(t, http.MethodPut, "/users/7", map[string]any{"first_name": "New", "last_name": "Name", "account_id": 9})
	assert.Equal(t, http.StatusNotFound, unknownAccount.Code)

	missing := h.do(t, http.MethodPut, "/users/8", map[string]any{"first_name": "New", "last_name": "Name"})
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestDeletePersonRespondsNoContentAndMapsModuleErrors(t *testing.T) {
	t.Parallel()
	directory := &fakeDirectory{persons: map[int64]peopledirectory.Person{}}
	h := newHarness(t, directory)
	h.permitted["users:delete"] = true

	recorder := h.do(t, http.MethodDelete, "/users/7", nil)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.EqualValues(t, 7, directory.deleted)

	directory.err = peopledirectory.ErrPersonNotFound
	missing := h.do(t, http.MethodDelete, "/users/7", nil)
	assert.Equal(t, http.StatusNotFound, missing.Code)

	directory.err = errors.New("storage down")
	failed := h.do(t, http.MethodDelete, "/users/7", nil)
	assert.Equal(t, http.StatusInternalServerError, failed.Code)
	assert.Equal(t, "storage down", decodeError(t, failed).Error)
	assert.Contains(t, h.observed, "Internal Server Error:internal_error")
}

func TestNewResourceRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { usersHTTP.NewResource(nil, usersHTTP.Runtime{}) })
}
