package classlistentries_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	classListHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http/classlistentries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMembership answers the listing from memory so every handler path can
// be driven without a database.
type fakeMembership struct {
	schoolmembership.Query
	entries    []schoolmembership.ClassListEntry
	listErr    error
	lastFilter *schoolmembership.ClassListEntryFilter
}

func (f *fakeMembership) ListClassListEntries(_ context.Context, filter schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, error) {
	f.lastFilter = &filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]schoolmembership.ClassListEntry(nil), f.entries...), nil
}

type response struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
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

// harness records every runtime call the adapter makes and serves the routes
// through a real chi router.
type harness struct {
	membership *fakeMembership
	router     chi.Router
	observed   []string
	txCalls    int
	permitted  map[string]bool
	accountID  int64

	matches   map[string][]int64
	created   []classListHTTP.EntryInput
	updated   map[int64]classListHTTP.EntryInput
	deleted   []int64
	assigned  map[int64]int64
	actors    []int64
	writeErr  error
	writeFail []error
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		membership: &fakeMembership{},
		permitted:  map[string]bool{},
		accountID:  77,
		matches:    map[string][]int64{},
		updated:    map[int64]classListHTTP.EntryInput{},
		assigned:   map[int64]int64{},
	}
	resource := classListHTTP.NewResource(h.membership, classListHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, classListHTTP.Middleware)) {
			register(router, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					h.txCalls++
					next.ServeHTTP(w, r)
				})
			})
		},
		Permission: func(permission string) classListHTTP.Middleware {
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
		Success: func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
			render.Status(r, status)
			payload, err := json.Marshal(data)
			require.NoError(t, err)
			_ = render.Render(w, r, response{Status: "success", Data: payload, Message: message})
		},
		Failure: func(w http.ResponseWriter, r *http.Request, kind classListHTTP.FailureKind, err error) {
			_ = render.Render(w, r, errorResponse{StatusCode: classListHTTP.StatusOf(kind), Status: "error", Kind: string(kind), Error: err.Error()})
		},
		ObserveResponse: func(status int, code string) {
			h.observed = append(h.observed, http.StatusText(status)+"/"+code)
		},
		WriteFailure: func(w http.ResponseWriter, r *http.Request, err error) {
			h.writeFail = append(h.writeFail, err)
			_ = render.Render(w, r, errorResponse{StatusCode: http.StatusTeapot, Status: "error", Kind: "delegated", Error: err.Error()})
		},
		CurrentAccountID: func(context.Context) int64 { return h.accountID },
		Order: func(entries []schoolmembership.ClassListEntry) {
			// Reverse the input so the test can see the closure ran.
			for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
				entries[i], entries[j] = entries[j], entries[i]
			}
		},
		MatchingStudentIDs: func(_ context.Context, input classListHTTP.EntryInput) ([]int64, error) {
			return h.matches[input.FirstName+"|"+input.LastName+"|"+input.SchoolClass], nil
		},
		Create: func(_ context.Context, input classListHTTP.EntryInput, actorID int64) (schoolmembership.ClassListEntry, error) {
			h.created = append(h.created, input)
			h.actors = append(h.actors, actorID)
			if h.writeErr != nil {
				return schoolmembership.ClassListEntry{}, h.writeErr
			}
			return schoolmembership.ClassListEntry{ID: 9007199254740993, FirstName: input.FirstName, LastName: input.LastName, SchoolClass: input.SchoolClass, CreatedAt: time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC)}, nil
		},
		Update: func(_ context.Context, id int64, input classListHTTP.EntryInput, actorID int64) (schoolmembership.ClassListEntry, error) {
			h.updated[id] = input
			h.actors = append(h.actors, actorID)
			if h.writeErr != nil {
				return schoolmembership.ClassListEntry{}, h.writeErr
			}
			return schoolmembership.ClassListEntry{ID: id, FirstName: input.FirstName, LastName: input.LastName, SchoolClass: input.SchoolClass}, nil
		},
		Delete: func(_ context.Context, id int64, actorID int64) error {
			h.deleted = append(h.deleted, id)
			h.actors = append(h.actors, actorID)
			return h.writeErr
		},
		Assign: func(_ context.Context, entryID, studentID, actorID int64) error {
			h.assigned[entryID] = studentID
			h.actors = append(h.actors, actorID)
			return h.writeErr
		},
		Log: slog.Default(),
	})
	h.router = resource.Router()
	return h
}

func (h *harness) do(method, target, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestRoutesAreGatedByTheirPermissions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	assert.Equal(t, http.StatusForbidden, h.do(http.MethodGet, "/", "").Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodPost, "/", `{}`).Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodPut, "/1", `{}`).Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodDelete, "/1", "").Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodPost, "/1/assign", `{}`).Code)
	assert.Zero(t, h.txCalls, "a refused request never opens the transaction")
}

func TestListReadsThroughTheCapabilityAndAddsTheMatchHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:read"] = true
	h.membership.entries = []schoolmembership.ClassListEntry{
		{ID: 1, FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"},
		{ID: 9007199254740993, FirstName: "Ben", LastName: "Berg", SchoolClass: "2b"},
	}
	h.matches["Ben|Berg|2b"] = []int64{9007199254740995}

	rec := h.do(http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, "Class list entries retrieved successfully")
	// IDs travel as JSON strings, lossless beyond 2^53.
	assert.Contains(t, body, `"id":"9007199254740993"`)
	assert.Contains(t, body, `"matching_student_ids":["9007199254740995"]`)
	assert.Contains(t, body, `"matching_student_ids":[]`, "no hint is an empty list, never null")
	assert.Less(t, strings.Index(body, "Berg"), strings.Index(body, "Aalders"), "the injected order closure decides the listing order")
	require.NotNil(t, h.membership.lastFilter)
	assert.Equal(t, schoolmembership.ClassListEntryFilter{}, *h.membership.lastFilter, "the listing reads every entry of the tenant")
	assert.Equal(t, 1, h.txCalls)
	assert.Equal(t, []string{"OK/none"}, h.observed)
}

func TestListFailureIsInternal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:read"] = true
	h.membership.listErr = errors.New("database gone")

	rec := h.do(http.MethodGet, "/", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, []string{"Internal Server Error/internal_error"}, h.observed)
}

func TestCreateTrimsAndDelegatesWithTheCallersAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:create"] = true

	rec := h.do(http.MethodPost, "/", `{"first_name":" Zoe ","last_name":"Aalders","school_class":" 1a "}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, h.created, 1)
	assert.Equal(t, classListHTTP.EntryInput{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"}, h.created[0])
	assert.Equal(t, []int64{77}, h.actors)
	assert.Contains(t, rec.Body.String(), `"id":"9007199254740993"`)
	assert.Contains(t, rec.Body.String(), "Class list entry created successfully")
	assert.Equal(t, []string{"Created/none"}, h.observed)
}

func TestWhitespaceOnlyFieldsAreInvalidRequests(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:create"] = true
	h.permitted["users:update"] = true

	rec := h.do(http.MethodPost, "/", `{"first_name":"  ","last_name":"Aalders","school_class":"1a"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "erforderlich")
	assert.Empty(t, h.created, "validation stops the request before the write flow")

	rec = h.do(http.MethodPut, "/5", `{"first_name":"Zoe","last_name":"","school_class":"1a"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Empty(t, h.updated)
}

func TestInvalidEntryIDsAreRejectedBeforeAnyFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:update"] = true
	h.permitted["users:delete"] = true

	for _, target := range []string{"/0", "/abc", "/-4"} {
		assert.Equal(t, http.StatusBadRequest, h.do(http.MethodPut, target, `{"first_name":"Zoe","last_name":"Aalders","school_class":"1a"}`).Code, target)
		assert.Equal(t, http.StatusBadRequest, h.do(http.MethodDelete, target, "").Code, target)
		assert.Equal(t, http.StatusBadRequest, h.do(http.MethodPost, target+"/assign", `{"student_id":"3"}`).Code, target)
	}
	assert.Empty(t, h.updated)
	assert.Empty(t, h.deleted)
	assert.Empty(t, h.assigned)
	assert.Contains(t, h.observed, "Bad Request/invalid_parameters")
}

func TestUpdateDeleteAndAssignDelegateToTheRuntime(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:update"] = true
	h.permitted["users:delete"] = true

	rec := h.do(http.MethodPut, "/9007199254740993", `{"first_name":"Zoe","last_name":"Aalders","school_class":"1b"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, classListHTTP.EntryInput{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1b"}, h.updated[9007199254740993])
	assert.Contains(t, rec.Body.String(), "Class list entry updated successfully")

	rec = h.do(http.MethodDelete, "/12", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{12}, h.deleted)
	assert.Contains(t, rec.Body.String(), "Class list entry deleted successfully")

	// student_id binds from the quoted wire value, lossless beyond 2^53.
	rec = h.do(http.MethodPost, "/12/assign", `{"student_id":"9007199254740995"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(9007199254740995), h.assigned[12])
	assert.Contains(t, rec.Body.String(), "Class list entry assigned successfully")

	rec = h.do(http.MethodPost, "/12/assign", `{"student_id":"0"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "student_id ist erforderlich")

	assert.Equal(t, []int64{77, 77, 77}, h.actors)
}

func TestWriteFailuresAreRenderedByTheRoot(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:create"] = true
	h.permitted["users:update"] = true
	h.permitted["users:delete"] = true
	h.writeErr = errors.New("Ein Eintrag mit diesem Namen existiert in dieser Klasse bereits")

	for _, request := range []struct{ method, target, body string }{
		{http.MethodPost, "/", `{"first_name":"Zoe","last_name":"Aalders","school_class":"1a"}`},
		{http.MethodPut, "/3", `{"first_name":"Zoe","last_name":"Aalders","school_class":"1a"}`},
		{http.MethodDelete, "/3", ""},
		{http.MethodPost, "/3/assign", `{"student_id":"4"}`},
	} {
		rec := h.do(request.method, request.target, request.body)
		assert.Equal(t, http.StatusTeapot, rec.Code, "%s %s must be rendered by the root's classifier", request.method, request.target)
	}
	require.Len(t, h.writeFail, 4)
	for _, err := range h.writeFail {
		assert.ErrorIs(t, err, h.writeErr)
	}
}

func TestNewResourceRefusesMissingDependencies(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { classListHTTP.NewResource(nil, classListHTTP.Runtime{}) })
}
