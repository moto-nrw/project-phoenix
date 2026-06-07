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
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource bundles the dependencies needed by the work-time-model HTTP handlers.
type Resource struct {
	Repo   config.WorkTimeModelRepository
	db     *bun.DB
	logger *slog.Logger
}

// NewResource wires the dependencies.
func NewResource(repo config.WorkTimeModelRepository, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Repo: repo, db: db, logger: logger}
}

// Router returns the chi sub-router for /api/work-time-models.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/", rs.list)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}", rs.get)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/", rs.create)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}", rs.update)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/{id}", rs.delete)
	})

	return r
}

// EntryRequest is the wire-format for an entry in create/update bodies.
type EntryRequest struct {
	WeekIndex     int `json:"week_index"`
	DayOfWeek     int `json:"day_of_week"`
	TargetMinutes int `json:"target_minutes"`
}

// EntryResponse is the wire-format for an entry returned to clients.
type EntryResponse struct {
	WeekIndex     int `json:"week_index"`
	DayOfWeek     int `json:"day_of_week"`
	TargetMinutes int `json:"target_minutes"`
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
	models, err := rs.Repo.List(r.Context())
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
	m, err := rs.Repo.FindByID(r.Context(), id)
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
	if err := rs.Repo.Create(r.Context(), model, entries); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	saved, err := rs.Repo.FindByID(r.Context(), model.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
	if err := rs.Repo.Update(r.Context(), model, entries); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	saved, err := rs.Repo.FindByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
	if err := rs.Repo.Delete(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("model not found or in use")))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"id": id}, "Work time model deleted")
}

func buildModelAndEntries(req ModelRequest) (*config.WorkTimeModel, []*config.WorkTimeModelEntry, error) {
	if req.Name == "" {
		return nil, nil, errors.New("name is required")
	}
	if req.RotationLength < 1 || req.RotationLength > config.WorkTimeModelMaxRotation {
		return nil, nil, errors.New("rotation_length must be between 1 and 4")
	}
	if req.RotationAnchorDate == "" {
		return nil, nil, errors.New("rotation_anchor_date is required")
	}
	anchor, err := time.Parse("2006-01-02", req.RotationAnchorDate)
	if err != nil {
		return nil, nil, errors.New("rotation_anchor_date must be YYYY-MM-DD")
	}
	model := &config.WorkTimeModel{
		Name:               req.Name,
		RotationLength:     req.RotationLength,
		RotationAnchorDate: anchor,
	}
	entries := make([]*config.WorkTimeModelEntry, 0, len(req.Entries))
	for _, e := range req.Entries {
		if e.TargetMinutes <= 0 {
			continue
		}
		entries = append(entries, &config.WorkTimeModelEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
		})
	}
	return model, entries, nil
}

func toResponse(m *config.WorkTimeModel) ModelResponse {
	totals := make([]int, m.RotationLength)
	entries := make([]EntryResponse, 0, len(m.Entries))
	for _, e := range m.Entries {
		entries = append(entries, EntryResponse{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
		})
		if e.WeekIndex >= 0 && e.WeekIndex < m.RotationLength {
			totals[e.WeekIndex] += e.TargetMinutes
		}
	}
	return ModelResponse{
		ID:                 m.ID,
		Name:               m.Name,
		RotationLength:     m.RotationLength,
		RotationAnchorDate: m.RotationAnchorDate.Format("2006-01-02"),
		Entries:            entries,
		WeeklyTotals:       totals,
	}
}
