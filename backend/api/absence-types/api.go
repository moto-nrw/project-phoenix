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
	"context"
	"errors"
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
	Service       activeSvc.StaffAbsenceTypeService
	actorResolver func(context.Context) (int64, error)
	db            *bun.DB
	logger        *slog.Logger
}

func (rs *Resource) SetActorResolver(resolver func(context.Context) (int64, error)) {
	rs.actorResolver = resolver
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
		allowanceRead := common.RequiresAnyPermission(permissions.TimeTrackingManage, permissions.VacationApprove)
		r.With(read, withTx).Get("/", rs.list)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Put("/{id}", rs.update)
		r.With(allowanceRead, withTx).Get("/{id}/allowances/{staffId}", rs.getAllowance)
		r.With(manage, withTx).Put("/{id}/allowances/{staffId}", rs.setAllowance)
	})

	return r
}

// CreateAbsenceTypeRequest is the create payload. Only the name is accepted:
// the base type is not client-controlled (see StaffAbsenceTypeService).
type CreateAbsenceTypeRequest struct {
	Name             string `json:"name"`
	AllowanceEnabled bool   `json:"allowance_enabled"`
	OverrunPolicy    string `json:"overrun_policy"`
}

// UpdateAbsenceTypeRequest renames and/or (de)activates. Omitted fields stay
// as they are, so a rename cannot accidentally reactivate a retired art.
type UpdateAbsenceTypeRequest struct {
	Name             *string `json:"name"`
	IsActive         *bool   `json:"is_active"`
	AllowanceEnabled *bool   `json:"allowance_enabled"`
	OverrunPolicy    *string `json:"overrun_policy"`
}

// AbsenceTypeResponse is the wire format returned to clients. BaseType tells
// the client which standard type's calculation this art inherits, so the UI can
// say so instead of leaving the school guessing.
type AbsenceTypeResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	BaseType         string `json:"base_type"`
	IsActive         bool   `json:"is_active"`
	AllowanceEnabled bool   `json:"allowance_enabled"`
	OverrunPolicy    string `json:"overrun_policy"`
}

// AllowanceSummaryResponse keeps database IDs as strings on the wire, matching
// every other frontend-facing int64 ID in the application.
type AllowanceSummaryResponse struct {
	StaffID       string  `json:"staff_id"`
	AbsenceTypeID string  `json:"absence_type_id"`
	Year          int     `json:"year"`
	EntitledDays  float64 `json:"entitled_days"`
	TakenDays     float64 `json:"taken_days"`
	ReservedDays  float64 `json:"reserved_days"`
	RemainingDays float64 `json:"remaining_days"`
}

func toAllowanceSummaryResponse(summary *activeSvc.AbsenceTypeAllowanceSummary) AllowanceSummaryResponse {
	return AllowanceSummaryResponse{
		StaffID:       strconv.FormatInt(summary.StaffID, 10),
		AbsenceTypeID: strconv.FormatInt(summary.AbsenceTypeID, 10),
		Year:          summary.Year,
		EntitledDays:  summary.EntitledDays,
		TakenDays:     summary.TakenDays,
		ReservedDays:  summary.ReservedDays,
		RemainingDays: summary.RemainingDays,
	}
}

func toAbsenceTypeResponse(t *activeModels.StaffAbsenceType) AbsenceTypeResponse {
	return AbsenceTypeResponse{
		ID:               strconv.FormatInt(t.ID, 10),
		Name:             t.Name,
		BaseType:         t.BaseType,
		IsActive:         t.IsActive,
		AllowanceEnabled: t.AllowanceEnabled,
		OverrunPolicy:    t.OverrunPolicy,
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
	{Target: activeSvc.ErrAbsenceTypeAllowanceInvalid, Render: common.ErrorInvalidRequest},
	{Target: activeSvc.ErrAbsenceTypeAllowanceExceeded, Render: common.ErrorConflict},
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
	saved, err := rs.Service.CreateAbsenceTypeWithConfig(r.Context(), req.Name, req.AllowanceEnabled, req.OverrunPolicy)
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
	saved, err := rs.Service.UpdateAbsenceTypeWithConfig(
		r.Context(), id, req.Name, req.IsActive, req.AllowanceEnabled, req.OverrunPolicy,
	)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAbsenceTypeResponse(saved), "Abwesenheitsart aktualisiert")
}

func (rs *Resource) getAllowance(w http.ResponseWriter, r *http.Request) {
	typeID, staffID, year, err := allowancePath(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.Service.GetAllowanceSummary(r.Context(), staffID, typeID, year)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAllowanceSummaryResponse(summary), "Kontingent geladen")
}

func (rs *Resource) setAllowance(w http.ResponseWriter, r *http.Request) {
	typeID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if rs.actorResolver == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("allowance actor resolver is not configured")))
		return
	}
	actorID, err := rs.actorResolver(r.Context())
	if err != nil || actorID <= 0 {
		if err == nil {
			err = errors.New("current staff member not found")
		}
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req struct {
		Year         int     `json:"year"`
		EntitledDays float64 `json:"entitled_days"`
		Reason       string  `json:"reason"`
	}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.Service.SetAllowance(r.Context(), activeSvc.SetAbsenceTypeAllowanceRequest{
		StaffID: staffID, AbsenceTypeID: typeID, Year: req.Year,
		EntitledDays: req.EntitledDays, Reason: req.Reason, ChangedBy: actorID,
	})
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAllowanceSummaryResponse(summary), "Kontingent gespeichert")
}

func allowancePath(r *http.Request) (int64, int64, int, error) {
	typeID, err := common.ParseID(r)
	if err != nil {
		return 0, 0, 0, err
	}
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		return 0, 0, 0, err
	}
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		return 0, 0, 0, err
	}
	return typeID, staffID, year, nil
}
