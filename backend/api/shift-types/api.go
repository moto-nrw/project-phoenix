// Package shifttypes exposes admin CRUD for tenant-defined shift types
// (Schichtarten, #1836) used to label planned staff shifts in the Dienstplan.
// A shift type carries a name, a hex color, an optional description and an
// active flag; the color makes different duties distinguishable in the week
// grid. Guarded by time_tracking:manage, same as staff shifts.
package shifttypes

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CategoryLinker syncs the optional Kategorie↔Schichtart mapping (#1837
// follow-up). The FK lives on activities.categories, so the write is owned by
// the activities service; this consumer-defined interface avoids importing that
// package's concrete type. Runs inside the request's tenant transaction.
type CategoryLinker interface {
	SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error
}

// Resource bundles the dependencies for the shift-type HTTP handlers.
type Resource struct {
	Service        scheduleSvc.ShiftTypeService
	CategoryLinker CategoryLinker
	db             *bun.DB
	logger         *slog.Logger
}

// NewResource wires the dependencies.
func NewResource(service scheduleSvc.ShiftTypeService, categoryLinker CategoryLinker, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, CategoryLinker: categoryLinker, db: db, logger: logger}
}

// Router returns the chi sub-router for /api/shift-types.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		manage := common.RequiresPermission(permissions.TimeTrackingManage)
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
	// CategoryIDs is the optional set of Timetable-Kategorien mapped to this
	// shift type (#1837 follow-up). When present (including an empty array) it
	// replaces the current mapping for this shift type; omitted (nil) leaves it
	// untouched.
	CategoryIDs []int64 `json:"category_ids"`
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
	if err := rs.syncCategoryLinks(r, saved.ID, req.CategoryIDs); err != nil {
		rs.renderSyncCategoryError(w, r, err)
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
	// A PUT that omits is_active must not silently flip the stored state — e.g.
	// a client editing only name/color would otherwise reactivate an inactive
	// shift type via buildShiftType's create default. Preserve the current value.
	if req.IsActive == nil {
		existing, err := rs.Service.GetShiftType(r.Context(), id)
		if err != nil {
			renderServiceError(w, r, err)
			return
		}
		shiftType.IsActive = existing.IsActive
	}
	saved, err := rs.Service.UpdateShiftType(r.Context(), shiftType)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	if err := rs.syncCategoryLinks(r, saved.ID, req.CategoryIDs); err != nil {
		rs.renderSyncCategoryError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toShiftTypeResponse(saved), "Shift type updated")
}

// syncCategoryLinks applies the optional Kategorie↔Schichtart mapping after a
// shift type is created/updated. A nil categoryIDs (field omitted) leaves the
// mapping untouched; a present slice (including empty) replaces it. Runs in the
// same tenant transaction as the shift-type write.
func (rs *Resource) syncCategoryLinks(r *http.Request, shiftTypeID int64, categoryIDs []int64) error {
	if categoryIDs == nil || rs.CategoryLinker == nil {
		return nil
	}
	return rs.CategoryLinker.SetCategoryShiftTypeLinks(r.Context(), shiftTypeID, categoryIDs)
}

// renderSyncCategoryError maps a category-link failure. Unknown/cross-tenant
// category IDs are client input errors (400) and must roll back the whole
// request so the shift-type write does not persist without its mapping; any
// other error is a 500.
func (rs *Resource) renderSyncCategoryError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, activitiesModels.ErrUnknownCategoryIDs) {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
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
