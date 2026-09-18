package classlistentries_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// fakeEntries answers the class-list capability from memory so every handler
// path can be driven without a database, and records what the adapter asked
// the owner to do.
type fakeEntries struct {
	entries    []schoolmembership.ClassListEntry
	listErr    error
	lastFilter *schoolmembership.ClassListEntryFilter

	matches  map[string][]int64
	matchErr error

	added    []schoolmembership.AddClassListEntry
	revised  map[int64]schoolmembership.ReviseClassListEntry
	removed  []schoolmembership.RemoveClassListEntry
	resolved map[int64]schoolmembership.ResolveClassListEntry
	actors   []int64
	writeErr error
}

func (f *fakeEntries) ListClassListEntriesInDisplayOrder(_ context.Context, filter schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, error) {
	f.lastFilter = &filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]schoolmembership.ClassListEntry(nil), f.entries...), nil
}

func (f *fakeEntries) MatchingStudentIDs(_ context.Context, fields schoolmembership.ClassListEntryFields) ([]int64, error) {
	if f.matchErr != nil {
		return nil, f.matchErr
	}
	return f.matches[fields.FirstName+"|"+fields.LastName+"|"+fields.SchoolClass], nil
}

func (f *fakeEntries) AddClassListEntry(_ context.Context, input schoolmembership.AddClassListEntry) (schoolmembership.ClassListEntry, error) {
	f.added = append(f.added, input)
	f.actors = append(f.actors, input.ChangedBy)
	if f.writeErr != nil {
		return schoolmembership.ClassListEntry{}, f.writeErr
	}
	return schoolmembership.ClassListEntry{
		ID: 9007199254740993, FirstName: input.FirstName, LastName: input.LastName,
		SchoolClass: input.SchoolClass, CreatedAt: time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeEntries) ReviseClassListEntry(_ context.Context, input schoolmembership.ReviseClassListEntry) (schoolmembership.ClassListEntry, error) {
	f.revised[input.ID] = input
	f.actors = append(f.actors, input.ChangedBy)
	if f.writeErr != nil {
		return schoolmembership.ClassListEntry{}, f.writeErr
	}
	return schoolmembership.ClassListEntry{
		ID: input.ID, FirstName: input.FirstName, LastName: input.LastName, SchoolClass: input.SchoolClass,
	}, nil
}

func (f *fakeEntries) RemoveClassListEntry(_ context.Context, input schoolmembership.RemoveClassListEntry) error {
	f.removed = append(f.removed, input)
	f.actors = append(f.actors, input.ChangedBy)
	return f.writeErr
}

func (f *fakeEntries) ResolveClassListEntry(_ context.Context, input schoolmembership.ResolveClassListEntry) error {
	f.resolved[input.ID] = input
	f.actors = append(f.actors, input.ChangedBy)
	return f.writeErr
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

// harness serves the routes through a real chi router over the fake owner.
type harness struct {
	entries   *fakeEntries
	router    chi.Router
	observed  []string
	txCalls   int
	permitted map[string]bool
	accountID int64
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		entries: &fakeEntries{
			matches:  map[string][]int64{},
			revised:  map[int64]schoolmembership.ReviseClassListEntry{},
			resolved: map[int64]schoolmembership.ResolveClassListEntry{},
		},
		permitted: map[string]bool{},
		accountID: 77,
	}
	resource := classListHTTP.NewResource(h.entries, classListHTTP.Runtime{
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
		CurrentAccountID: func(context.Context) int64 { return h.accountID },
		Log:              slog.Default(),
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
	h.entries.entries = []schoolmembership.ClassListEntry{
		{ID: 9007199254740993, FirstName: "Ben", LastName: "Berg", SchoolClass: "2b"},
		{ID: 1, FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"},
	}
	h.entries.matches["Ben|Berg|2b"] = []int64{9007199254740995}

	rec := h.do(http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, "Class list entries retrieved successfully")
	// IDs travel as JSON strings, lossless beyond 2^53.
	assert.Contains(t, body, `"id":"9007199254740993"`)
	assert.Contains(t, body, `"matching_student_ids":["9007199254740995"]`)
	assert.Contains(t, body, `"matching_student_ids":[]`, "no hint is an empty list, never null")
	assert.Less(t, strings.Index(body, "Berg"), strings.Index(body, "Aalders"), "the owner's display order is rendered unchanged")
	require.NotNil(t, h.entries.lastFilter)
	assert.Equal(t, schoolmembership.ClassListEntryFilter{}, *h.entries.lastFilter, "the listing reads every entry of the tenant")
	assert.Equal(t, 1, h.txCalls)
	assert.Equal(t, []string{"OK/none"}, h.observed)
}

func TestListFailureIsInternal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:read"] = true
	h.entries.listErr = errors.New("database gone")

	rec := h.do(http.MethodGet, "/", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, []string{"Internal Server Error/internal_error"}, h.observed)
}

func TestMatchHintFailureIsInternal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:read"] = true
	h.entries.entries = []schoolmembership.ClassListEntry{{ID: 1, FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"}}
	h.entries.matchErr = errors.New("directory gone")

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
	require.Len(t, h.entries.added, 1)
	assert.Equal(t, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"},
		ChangedBy:            77,
	}, h.entries.added[0])
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
	assert.Empty(t, h.entries.added, "validation stops the request before the write flow")

	rec = h.do(http.MethodPut, "/5", `{"first_name":"Zoe","last_name":"","school_class":"1a"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Empty(t, h.entries.revised)
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
	assert.Empty(t, h.entries.revised)
	assert.Empty(t, h.entries.removed)
	assert.Empty(t, h.entries.resolved)
	assert.Contains(t, h.observed, "Bad Request/invalid_parameters")
}

func TestUpdateDeleteAndAssignDelegateToTheOwner(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.permitted["users:update"] = true
	h.permitted["users:delete"] = true

	rec := h.do(http.MethodPut, "/9007199254740993", `{"first_name":"Zoe","last_name":"Aalders","school_class":"1b"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, schoolmembership.ReviseClassListEntry{
		ID:                   9007199254740993,
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1b"},
		ChangedBy:            77,
	}, h.entries.revised[9007199254740993])
	assert.Contains(t, rec.Body.String(), "Class list entry updated successfully")

	rec = h.do(http.MethodDelete, "/12", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []schoolmembership.RemoveClassListEntry{{ID: 12, ChangedBy: 77}}, h.entries.removed)
	assert.Contains(t, rec.Body.String(), "Class list entry deleted successfully")

	// student_id binds from the quoted wire value, lossless beyond 2^53.
	rec = h.do(http.MethodPost, "/12/assign", `{"student_id":"9007199254740995"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, schoolmembership.ResolveClassListEntry{ID: 12, StudentID: 9007199254740995, ChangedBy: 77}, h.entries.resolved[12])
	assert.Contains(t, rec.Body.String(), "Class list entry assigned successfully")

	rec = h.do(http.MethodPost, "/12/assign", `{"student_id":"0"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "student_id ist erforderlich")

	assert.Equal(t, []int64{77, 77, 77}, h.entries.actors)
}

// The adapter classifies through the owner's error contract: the German
// refusals are 400s, an unknown entry is a 404 and anything else a 500.
func TestWriteFailuresAreClassifiedByTheOwnerContract(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		err    error
		status int
		kind   string
	}{
		{"duplicate entry", schoolmembership.ErrClassListEntryDuplicate, http.StatusBadRequest, "invalid_request"},
		{"student of the same name", schoolmembership.ErrClassListEntryStudentExists, http.StatusBadRequest, "invalid_request"},
		{"unknown assign target", schoolmembership.ErrClassListEntryStudentNotFound, http.StatusBadRequest, "invalid_request"},
		{"mismatched assign target", schoolmembership.ErrClassListEntryAssignMismatch, http.StatusBadRequest, "invalid_request"},
		{"unknown entry", schoolmembership.ErrClassListEntryNotFound, http.StatusNotFound, "not_found"},
		{"anything else", errors.New("database gone"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, request := range []struct{ method, target, body string }{
				{http.MethodPost, "/", `{"first_name":"Zoe","last_name":"Aalders","school_class":"1a"}`},
				{http.MethodPut, "/3", `{"first_name":"Zoe","last_name":"Aalders","school_class":"1a"}`},
				{http.MethodDelete, "/3", ""},
				{http.MethodPost, "/3/assign", `{"student_id":"4"}`},
			} {
				h := newHarness(t)
				h.permitted["users:create"] = true
				h.permitted["users:update"] = true
				h.permitted["users:delete"] = true
				h.entries.writeErr = tt.err

				rec := h.do(request.method, request.target, request.body)
				assert.Equal(t, tt.status, rec.Code, "%s %s", request.method, request.target)
				assert.Contains(t, rec.Body.String(), tt.err.Error(), "the owner's message is rendered verbatim")
				assert.Equal(t, []string{fmt.Sprintf("%s/%s", http.StatusText(tt.status), tt.kind)}, h.observed)
			}
		})
	}
}

func TestNewResourceRefusesMissingDependencies(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { classListHTTP.NewResource(nil, classListHTTP.Runtime{}) })
}
