// Package absencetypes exposes the school's own Abwesenheitsarten (#2403):
// wiederverwendbare Bezeichnungen a school adds next to the five standard types
// (Urlaub, Krank, Fortbildung, Sonstige, Freizeitausgleich), which stay code
// constants and are neither listed nor changeable here.
//
// Guarded by time_tracking:manage — the same permission that already governs
// entering and deleting staff absences.
//
// There is deliberately no DELETE: an art that was used has to stay readable on
// its historical absences, so retirement is `is_active: false`.
package absencetypes

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/uptrace/bun"
)

// Resource bundles the dependencies for the absence-type HTTP handlers.
type Resource struct {
	Service activeSvc.StaffAbsenceTypeService
	db      *bun.DB
	logger  *slog.Logger
}

// NewResource wires the dependencies.
func NewResource(service activeSvc.StaffAbsenceTypeService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, db: db, logger: logger}
}

// Router returns the chi sub-router for /api/absence-types.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		// Reading is open to anyone who may decide absences too, so the
		// Leitung's approval views can render the school's own wording.
		read := common.RequiresAnyPermission(permissions.TimeTrackingOwn, permissions.TimeTrackingManage, permissions.VacationApprove)
		manage := common.RequiresPermission(permissions.TimeTrackingManage)
		r.With(read, withTx).Get("/", rs.list)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Put("/{id}", rs.update)
	})

	return r
}

// CreateAbsenceTypeRequest is the create payload. Only the name is accepted:
// the base type is not client-controlled (see StaffAbsenceTypeService).
type CreateAbsenceTypeRequest struct {
	Name string `json:"name"`
}

// UpdateAbsenceTypeRequest renames and/or (de)activates. Omitted fields stay
// as they are, so a rename cannot accidentally reactivate a retired art.
type UpdateAbsenceTypeRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

// AbsenceTypeResponse is the wire format returned to clients. BaseType tells
// the client which standard type's calculation this art inherits, so the UI can
// say so instead of leaving the school guessing.
type AbsenceTypeResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BaseType string `json:"base_type"`
	IsActive bool   `json:"is_active"`
}

func toAbsenceTypeResponse(t *activeModels.StaffAbsenceType) AbsenceTypeResponse {
	return AbsenceTypeResponse{
		ID:       strconv.FormatInt(t.ID, 10),
		Name:     t.Name,
		BaseType: t.BaseType,
		IsActive: t.IsActive,
	}
}

func toAbsenceTypeResponses(types []*activeModels.StaffAbsenceType) []AbsenceTypeResponse {
	out := make([]AbsenceTypeResponse, 0, len(types))
	for _, t := range types {
		out = append(out, toAbsenceTypeResponse(t))
	}
	return out
}

var errorRules = []common.ErrorRule{
	{Target: activeSvc.ErrAbsenceTypeNameTaken, Render: common.ErrorConflict},
	{Target: activeSvc.ErrAbsenceTypeNameReserved, Render: common.ErrorConflict},
	{Target: activeSvc.ErrAbsenceTypeInUse, Render: common.ErrorConflict},
	{Target: activeSvc.ErrAbsenceTypeNotFound, Render: common.ErrorNotFound},
	{Target: activeSvc.ErrAbsenceTypeInvalid, Render: common.ErrorInvalidRequest},
}

var errorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)

func renderServiceError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, errorRenderer(err))
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	types, err := rs.Service.ListAbsenceTypes(r.Context())
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAbsenceTypeResponses(types), "Abwesenheitsarten geladen")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req CreateAbsenceTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.Service.CreateAbsenceType(r.Context(), req.Name)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toAbsenceTypeResponse(saved), "Abwesenheitsart erstellt")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req UpdateAbsenceTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.Service.UpdateAbsenceType(r.Context(), id, req.Name, req.IsActive)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAbsenceTypeResponse(saved), "Abwesenheitsart aktualisiert")
}
