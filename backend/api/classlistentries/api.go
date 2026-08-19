// Package classlistentries manages the minimal class-list-only entries
// (#2382): children of the class cohort without an OGS record — only name and
// free-text school class. The entries surface exclusively on class lists and
// the Lehrkraft class-day view; they are deliberately invisible to the
// student search, attendance, care planning and the parent portal, which is
// guaranteed structurally (no student row exists to reference).
package classlistentries

import (
	"cmp"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource wires the class-list entry endpoints.
type Resource struct {
	Service usersService.ClassListEntryService
	db      *bun.DB
	logger  *slog.Logger
}

// NewResource creates the class-list entry resource.
func NewResource(service usersService.ClassListEntryService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, db: db, logger: logger}
}

func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// Router returns the class-list entry router.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listEntries)
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createEntry)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateEntry)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteEntry)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Post("/{id}/assign", rs.assignEntry)
	})

	return r
}

// EntryRequest is the create/update payload.
type EntryRequest struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
}

// Bind validates the payload. Fields are trimmed here so a whitespace-only
// value is rejected as the invalid request it is (400) instead of tripping
// the service-level validation later.
func (req *EntryRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.SchoolClass = strings.TrimSpace(req.SchoolClass)
	if req.FirstName == "" || req.LastName == "" || req.SchoolClass == "" {
		return errors.New("Vorname, Nachname und Klasse sind erforderlich") //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
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
// JSON strings for the same reason AssignRequest binds with `,string`:
// JavaScript clients and the Next.js proxy round JSON numbers beyond 2^53.
type EntryResponse struct {
	ID                 int64     `json:"id,string"`
	FirstName          string    `json:"first_name"`
	LastName           string    `json:"last_name"`
	SchoolClass        string    `json:"school_class"`
	CreatedAt          time.Time `json:"created_at"`
	MatchingStudentIDs []string  `json:"matching_student_ids"`
}

func entryResponse(entry *userModels.ClassListEntry, matches []int64) EntryResponse {
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
	entries, err := rs.Service.List(r.Context())
	if err != nil {
		rs.getLogger().Error("class list entries: list failed", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entryResponse(entry.Entry, entry.MatchingStudentIDs))
	}
	common.Respond(w, r, http.StatusOK, out, "Class list entries retrieved successfully")
}

func (rs *Resource) createEntry(w http.ResponseWriter, r *http.Request) {
	req := &EntryRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	entry, err := rs.Service.Create(r.Context(), usersService.ClassListEntryInput{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		SchoolClass: req.SchoolClass,
	}, accountID(r))
	if err != nil {
		rs.renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, entryResponse(entry, nil), "Class list entry created successfully")
}

func (rs *Resource) updateEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	req := &EntryRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	entry, err := rs.Service.Update(r.Context(), id, usersService.ClassListEntryInput{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		SchoolClass: req.SchoolClass,
	}, accountID(r))
	if err != nil {
		rs.renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, entryResponse(entry, nil), "Class list entry updated successfully")
}

func (rs *Resource) deleteEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	if err := rs.Service.Delete(r.Context(), id, accountID(r)); err != nil {
		rs.renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Class list entry deleted successfully")
}

func (rs *Resource) assignEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	req := &AssignRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.Assign(r.Context(), id, req.StudentID, accountID(r)); err != nil {
		rs.renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Class list entry assigned successfully")
}

func (rs *Resource) renderServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usersService.ErrClassListEntryNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, usersService.ErrClassListEntryDuplicate),
		errors.Is(err, usersService.ErrClassListEntryStudentExists),
		errors.Is(err, usersService.ErrClassListEntryStudentNotFound),
		errors.Is(err, usersService.ErrClassListEntryAssignMismatch):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		rs.getLogger().Error("class list entries: request failed", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func entryID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "id", "invalid entry ID")
}

func accountID(r *http.Request) int64 {
	return int64(jwt.ClaimsFromCtx(r.Context()).ID)
}
