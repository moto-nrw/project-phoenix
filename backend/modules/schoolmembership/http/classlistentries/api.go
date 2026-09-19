// Package classlistentries exposes the class-list-only entries (#2382) under
// /api/class-list-entries: children of the class cohort without an OGS
// record, only name and free-text class. The adapter reads and writes through
// the School Membership capability — the duplicate guards, the student-match
// hint, the display order and the audit trail all live with that owner;
// rendering and auth are injected by the root.
package classlistentries

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// Middleware is the chi middleware shape the root hands in.
type Middleware = func(http.Handler) http.Handler

// FailureKind selects the response shape the root renders for an error. It is
// the owner's own classification: the adapter has no second opinion about
// what a refused class-list write means.
type FailureKind = schoolmembership.ClassListEntryFailure

const (
	FailureInvalidRequest = schoolmembership.ClassListEntryFailureInvalidRequest
	FailureNotFound       = schoolmembership.ClassListEntryFailureNotFound
	FailureInternal       = schoolmembership.ClassListEntryFailureInternal
)

// Runtime carries everything the adapter must not own: the protected route
// group, permission middleware, response rendering and the caller's identity.
type Runtime struct {
	// Protected wraps the router in the authenticated tenant group and hands
	// back the per-request transaction middleware.
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Permission func(string) Middleware

	Success         func(http.ResponseWriter, *http.Request, int, any, string)
	Failure         func(http.ResponseWriter, *http.Request, FailureKind, error)
	ObserveResponse func(int, string)

	// CurrentAccountID is 0 when the token carries no account.
	CurrentAccountID func(context.Context) int64

	Log *slog.Logger
}

// Resource is the class-list entry HTTP adapter.
type Resource struct {
	entries schoolmembership.ClassListEntries
	runtime Runtime
}

// NewResource panics when a dependency is missing: a nil closure would only
// surface as a nil-pointer panic on the first request that needs it.
func NewResource(entries schoolmembership.ClassListEntries, runtime Runtime) *Resource {
	if entries == nil || runtime.Protected == nil || runtime.Permission == nil ||
		runtime.Success == nil || runtime.Failure == nil || runtime.ObserveResponse == nil ||
		runtime.CurrentAccountID == nil || runtime.Log == nil {
		panic("class list entries HTTP: all dependencies are required")
	}
	return &Resource{entries: entries, runtime: runtime}
}

// Router mounts the routes on their own protected router.
func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(rs.runtime.Permission(permissions.UsersRead), withTx).Get("/", rs.listEntries)
		protected.With(rs.runtime.Permission(permissions.UsersCreate), withTx).Post("/", rs.createEntry)
		protected.With(rs.runtime.Permission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateEntry)
		protected.With(rs.runtime.Permission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteEntry)
		protected.With(rs.runtime.Permission(permissions.UsersDelete), withTx).Post("/{id}/assign", rs.assignEntry)
	})
	return router
}

// EntryRequest is the create/update payload.
type EntryRequest struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
}

// Bind validates the payload. Fields are trimmed here so a whitespace-only
// value is rejected as the invalid request it is (400) instead of tripping
// the owner-level validation later.
func (req *EntryRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.SchoolClass = strings.TrimSpace(req.SchoolClass)
	if req.FirstName == "" || req.LastName == "" || req.SchoolClass == "" {
		return errors.New("Vorname, Nachname und Klasse sind erforderlich") //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

func (req *EntryRequest) fields() schoolmembership.ClassListEntryFields {
	return schoolmembership.ClassListEntryFields{
		FirstName: req.FirstName, LastName: req.LastName, SchoolClass: req.SchoolClass,
	}
}

// AssignRequest names the student an entry is resolved into. StudentID binds
// via `,string` (the wire value is a quoted decimal): JavaScript clients and
// the Next.js proxy round JSON numbers beyond 2^53, so 64-bit IDs must travel
// as strings to stay lossless.
type AssignRequest struct {
	StudentID int64 `json:"student_id,string"`
}

// Bind validates the payload.
func (req *AssignRequest) Bind(_ *http.Request) error {
	if req.StudentID <= 0 {
		return errors.New("student_id ist erforderlich") //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// EntryResponse is one entry on the wire. MatchingStudentIDs carries the IDs
// of regular students sharing name and class — the hint for a deliberate
// "Zuordnen" resolution, never an automatic merge. All IDs are serialized as
// JSON strings for the same reason AssignRequest binds with `,string`.
type EntryResponse struct {
	ID                 int64     `json:"id,string"`
	FirstName          string    `json:"first_name"`
	LastName           string    `json:"last_name"`
	SchoolClass        string    `json:"school_class"`
	CreatedAt          time.Time `json:"created_at"`
	MatchingStudentIDs []string  `json:"matching_student_ids"`
}

func entryResponse(entry schoolmembership.ClassListEntry, matches []int64) EntryResponse {
	matchIDs := make([]string, 0, len(matches))
	for _, id := range matches {
		matchIDs = append(matchIDs, strconv.FormatInt(id, 10))
	}
	return EntryResponse{
		ID:                 entry.ID,
		FirstName:          entry.FirstName,
		LastName:           entry.LastName,
		SchoolClass:        entry.SchoolClass,
		CreatedAt:          entry.CreatedAt,
		MatchingStudentIDs: matchIDs,
	}
}

func (rs *Resource) listEntries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entries, err := rs.entries.ListClassListEntriesInDisplayOrder(ctx, schoolmembership.ClassListEntryFilter{})
	if err != nil {
		rs.runtime.Log.Error("class list entries: list failed", "error", err.Error())
		rs.failure(w, r, FailureInternal, err, schoolmembership.ErrorCode(err))
		return
	}
	out := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		matches, err := rs.entries.MatchingStudentIDs(ctx, schoolmembership.ClassListEntryFields{
			FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: entry.SchoolClass,
		})
		if err != nil {
			rs.runtime.Log.Error("class list entries: list failed", "error", err.Error())
			rs.failure(w, r, FailureInternal, err, "internal_error")
			return
		}
		out = append(out, entryResponse(entry, matches))
	}
	rs.respond(w, r, http.StatusOK, out, "Class list entries retrieved successfully")
}

func (rs *Resource) createEntry(w http.ResponseWriter, r *http.Request) {
	req := &EntryRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	entry, err := rs.entries.AddClassListEntry(r.Context(), schoolmembership.AddClassListEntry{
		ClassListEntryFields: req.fields(), ChangedBy: rs.runtime.CurrentAccountID(r.Context()),
	})
	if err != nil {
		rs.writeFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusCreated, entryResponse(entry, nil), "Class list entry created successfully")
}

func (rs *Resource) updateEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseEntryID(w, r)
	if !ok {
		return
	}
	req := &EntryRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	entry, err := rs.entries.ReviseClassListEntry(r.Context(), schoolmembership.ReviseClassListEntry{
		ID: id, ClassListEntryFields: req.fields(), ChangedBy: rs.runtime.CurrentAccountID(r.Context()),
	})
	if err != nil {
		rs.writeFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, entryResponse(entry, nil), "Class list entry updated successfully")
}

func (rs *Resource) deleteEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseEntryID(w, r)
	if !ok {
		return
	}
	err := rs.entries.RemoveClassListEntry(r.Context(), schoolmembership.RemoveClassListEntry{
		ID: id, ChangedBy: rs.runtime.CurrentAccountID(r.Context()),
	})
	if err != nil {
		rs.writeFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, nil, "Class list entry deleted successfully")
}

func (rs *Resource) assignEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseEntryID(w, r)
	if !ok {
		return
	}
	req := &AssignRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	err := rs.entries.ResolveClassListEntry(r.Context(), schoolmembership.ResolveClassListEntry{
		ID: id, StudentID: req.StudentID, ChangedBy: rs.runtime.CurrentAccountID(r.Context()),
	})
	if err != nil {
		rs.writeFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, nil, "Class list entry assigned successfully")
}

func (rs *Resource) parseEntryID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		rs.failure(w, r, FailureInvalidRequest, errors.New("invalid entry ID"), "invalid_parameters")
		return 0, false
	}
	return id, true
}

func (rs *Resource) respond(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	rs.runtime.Success(w, r, status, data, message)
	rs.runtime.ObserveResponse(status, "none")
}

// writeFailure renders the outcome of an administration command: unknown
// entry -> 404, a refused name, class or resolve target -> 400, else 500.
// Only the 500 is logged; the others are the documented answers of the
// screen, not incidents.
func (rs *Resource) writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	kind := schoolmembership.ClassifyClassListEntryFailure(err)
	if kind == FailureInternal {
		rs.runtime.Log.Error("class list entries: request failed", "error", err.Error())
	}
	rs.failure(w, r, kind, err, string(kind))
}

func (rs *Resource) failure(w http.ResponseWriter, r *http.Request, kind FailureKind, err error, code string) {
	rs.runtime.Failure(w, r, kind, err)
	rs.runtime.ObserveResponse(StatusOf(kind), code)
}

// StatusOf is the HTTP status the root renders for a failure kind.
func StatusOf(kind FailureKind) int {
	switch kind {
	case FailureInvalidRequest:
		return http.StatusBadRequest
	case FailureNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
