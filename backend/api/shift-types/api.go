// Package shifttypes exposes admin CRUD for tenant-defined shift types
// (Schichtarten, #1836) used to label planned staff shifts in the Dienstplan.
// A shift type carries a name, a hex color, an optional description and an
// active flag; the color makes different duties distinguishable in the week
// grid. Guarded by time_tracking:manage, same as staff shifts.
package shifttypes

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource bundles the dependencies for the shift-type HTTP handlers.
type Resource struct {
	Service scheduleSvc.ShiftTypeService
	db      *bun.DB
	logger  *slog.Logger
}

// NewResource wires the dependencies.
func NewResource(service scheduleSvc.ShiftTypeService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, db: db, logger: logger}
}

// Router returns the chi sub-router for /api/shift-types.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		manage := authorize.RequiresPermission(permissions.TimeTrackingManage)
		r.With(manage, withTx).Get("/", rs.list)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Post("/defaults", rs.createDefaults)
		r.With(manage, withTx).Put("/{id}", rs.update)
		r.With(manage, withTx).Delete("/{id}", rs.delete)
	})

	return r
}

// ShiftTypeRequest is the create/update payload. IsActive defaults to true when
// omitted on create.
type ShiftTypeRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active"`
}

// ShiftTypeResponse is the wire format returned to clients.
type ShiftTypeResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	IsActive    bool   `json:"is_active"`
}

func toShiftTypeResponse(t *scheduleModels.ShiftType) ShiftTypeResponse {
	return ShiftTypeResponse{
		ID:          t.ID,
		Name:        t.Name,
		Color:       t.Color,
		Description: t.Description,
		IsActive:    t.IsActive,
	}
}

func toShiftTypeResponses(types []*scheduleModels.ShiftType) []ShiftTypeResponse {
	out := make([]ShiftTypeResponse, 0, len(types))
	for _, t := range types {
		out = append(out, toShiftTypeResponse(t))
	}
	return out
}

func buildShiftType(req ShiftTypeRequest) *scheduleModels.ShiftType {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	return &scheduleModels.ShiftType{
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
		IsActive:    isActive,
	}
}

func renderServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrShiftTypeNameTaken):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, scheduleSvc.ErrShiftTypeNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, scheduleSvc.ErrShiftTypeInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	types, err := rs.Service.ListShiftTypes(r.Context())
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toShiftTypeResponses(types), "Shift types retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req ShiftTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.Service.CreateShiftType(r.Context(), buildShiftType(req))
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toShiftTypeResponse(saved), "Shift type created")
}

func (rs *Resource) createDefaults(w http.ResponseWriter, r *http.Request) {
	types, err := rs.Service.CreateDefaultShiftTypes(r.Context())
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toShiftTypeResponses(types), "Default shift types created")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req ShiftTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	shiftType := buildShiftType(req)
	shiftType.ID = id
	saved, err := rs.Service.UpdateShiftType(r.Context(), shiftType)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toShiftTypeResponse(saved), "Shift type updated")
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.DeleteShiftType(r.Context(), id); err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"id": id}, "Shift type deleted")
}
