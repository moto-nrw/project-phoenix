// Package worktimemodels exposes tenant-level CRUD for work-time templates.
// A template captures a Soll-Stunden pattern (Mo-Fr per rotation week) that
// admins assign to staff. Per-staff binding lives on /api/staff/{id}/schedule
// with mode=template.
package worktimemodels

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/uptrace/bun"
)

// Resource bundles the dependencies needed by the work-time-model HTTP handlers.
type Resource struct {
	Service *configSvc.WorkTimeModelService
	db      *bun.DB
	logger  *slog.Logger
}

// NewResource wires the dependencies.
func NewResource(service *configSvc.WorkTimeModelService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, db: db, logger: logger}
}

// Router returns the chi sub-router for /api/work-time-models.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		r.With(common.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/", rs.list)
		r.With(common.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}", rs.get)
		r.With(common.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/", rs.create)
		r.With(common.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}", rs.update)
		r.With(common.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/{id}", rs.delete)
	})

	return r
}

// EntryRequest is the wire-format for an entry in create/update bodies.
type EntryRequest struct {
	WeekIndex     int     `json:"week_index"`
	DayOfWeek     int     `json:"day_of_week"`
	TargetMinutes int     `json:"target_minutes"`
	StartTime     *string `json:"start_time,omitempty"`
}

// EntryResponse is the wire-format for an entry returned to clients.
type EntryResponse struct {
	WeekIndex     int     `json:"week_index"`
	DayOfWeek     int     `json:"day_of_week"`
	TargetMinutes int     `json:"target_minutes"`
	StartTime     *string `json:"start_time,omitempty"`
}

// ModelRequest is the create/update payload.
type ModelRequest struct {
	Name               string         `json:"name"`
	RotationLength     int            `json:"rotation_length"`
	RotationAnchorDate string         `json:"rotation_anchor_date"`
	Entries            []EntryRequest `json:"entries"`
}

// ModelResponse mirrors the model with its entries.
type ModelResponse struct {
	ID                 int64           `json:"id"`
	Name               string          `json:"name"`
	RotationLength     int             `json:"rotation_length"`
	RotationAnchorDate string          `json:"rotation_anchor_date"`
	Entries            []EntryResponse `json:"entries"`
	WeeklyTotals       []int           `json:"weekly_totals"`
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	models, err := rs.Service.ListModels(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]ModelResponse, 0, len(models))
	for _, m := range models {
		out = append(out, toResponse(m))
	}
	common.Respond(w, r, http.StatusOK, out, "Work time models retrieved")
}

func (rs *Resource) get(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	m, err := rs.Service.GetModel(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("model not found")))
		return
	}
	common.Respond(w, r, http.StatusOK, toResponse(m), "Work time model retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req ModelRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	model, entries, err := buildModelAndEntries(req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.Service.CreateModel(r.Context(), model, entries)
	if err != nil {
		rs.renderSaveError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toResponse(saved), "Work time model created")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req ModelRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	model, entries, err := buildModelAndEntries(req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	model.ID = id
	saved, err := rs.Service.UpdateModel(r.Context(), model, entries)
	if err != nil {
		rs.renderSaveError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toResponse(saved), "Work time model updated")
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.DeleteModel(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("model not found or in use")))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"id": id}, "Work time model deleted")
}

// renderSaveError maps a create/update service error to its HTTP response:
// caller-input validation failures become 400, everything else 500.
func (rs *Resource) renderSaveError(w http.ResponseWriter, r *http.Request, err error) {
	var vErr *configSvc.WorkTimeModelValidationError
	if errors.As(err, &vErr) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// buildModelAndEntries binds the wire request into domain structs. It only
// parses the wire format (anchor date, per-entry start time) and drops
// zero-minute entries; the business rules are enforced by the service via
// WorkTimeModelService.ValidateModelWithEntries.
func buildModelAndEntries(req ModelRequest) (*config.WorkTimeModel, []*config.WorkTimeModelEntry, error) {
	anchor := timezone.Date{}
	if req.RotationAnchorDate != "" {
		parsed, err := timezone.ParseDate(req.RotationAnchorDate)
		if err != nil {
			return nil, nil, errors.New("rotation_anchor_date must be YYYY-MM-DD")
		}
		anchor = parsed
	}
	model := &config.WorkTimeModel{
		Name:               req.Name,
		RotationLength:     req.RotationLength,
		RotationAnchorDate: anchor,
	}
	entries := make([]*config.WorkTimeModelEntry, 0, len(req.Entries))
	for _, e := range req.Entries {
		if e.TargetMinutes == 0 {
			continue
		}
		startTime, err := parseOptionalStartTime(e.StartTime)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, &config.WorkTimeModelEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     startTime,
		})
	}
	return model, entries, nil
}

func parseOptionalStartTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("15:04", *raw)
	if err != nil {
		return nil, errors.New("start_time must be HH:MM")
	}
	wallClock := timezone.NormalizeWallClock(parsed)
	return &wallClock, nil
}

func formatOptionalStartTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := timezone.NormalizeWallClock(*value).Format("15:04")
	return &formatted
}

func toResponse(m *config.WorkTimeModel) ModelResponse {
	totals := make([]int, m.RotationLength)
	entries := make([]EntryResponse, 0, len(m.Entries))
	for _, e := range m.Entries {
		entries = append(entries, EntryResponse{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     formatOptionalStartTime(e.StartTime),
		})
		if e.WeekIndex >= 0 && e.WeekIndex < m.RotationLength {
			totals[e.WeekIndex] += e.TargetMinutes
		}
	}
	return ModelResponse{
		ID:                 m.ID,
		Name:               m.Name,
		RotationLength:     m.RotationLength,
		RotationAnchorDate: m.RotationAnchorDate.String(),
		Entries:            entries,
		WeeklyTotals:       totals,
	}
}
