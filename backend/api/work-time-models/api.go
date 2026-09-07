// Package worktimemodels serves the established /api/work-time-models
// contract through the Workforce capability. A template captures a
// Soll-Stunden pattern (Mo-Fr per rotation week) that admins assign to staff;
// the per-staff binding lives on /api/staff/{id}/schedule with mode=template.
//
// Authentication, transaction scoping, rendering and the post-commit
// notification are supplied by the composition root.
package worktimemodels

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Middleware is the HTTP middleware shape the composition root supplies.
type Middleware = func(http.Handler) http.Handler

// FailureKind classifies a handler failure so the composition root can render
// it with the project's shared error envelope.
type FailureKind string

const (
	FailureInvalid  FailureKind = "invalid"
	FailureNotFound FailureKind = "not_found"
	FailureInternal FailureKind = "internal"
)

// wireClockLayout is the HH:MM start time the API has always accepted and
// returned; the capability itself stores a full wall clock.
const wireClockLayout = "15:04"

// Runtime carries the HTTP-platform behavior this adapter must not own. Every
// field is required, so missing production wiring fails at startup.
type Runtime struct {
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Permission func(string) Middleware
	ParseID    func(*http.Request) (int64, error)
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, FailureKind, error)
	// Manage is the permission every route requires.
	Manage string
	// NotifyChanged invalidates the time-account views after a template edit
	// rewrote the assigned staff schedules. It runs only after the write
	// succeeded.
	NotifyChanged func(context.Context)
}

// Resource bundles the dependencies needed by the work-time-model handlers.
type Resource struct {
	models  workforce.Capability
	runtime Runtime
}

// NewResource wires the dependencies.
func NewResource(models workforce.Capability, runtime Runtime) *Resource {
	return &Resource{models: models, runtime: runtime}
}

// Router returns the chi sub-router for /api/work-time-models.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		manage := rs.runtime.Permission(rs.runtime.Manage)
		r.With(manage, withTx).Get("/", rs.list)
		r.With(manage, withTx).Get("/{id}", rs.get)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Put("/{id}", rs.update)
		r.With(manage, withTx).Delete("/{id}", rs.delete)
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
	models, err := rs.models.ListWorkTimeModels(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err)
		return
	}
	out := make([]ModelResponse, 0, len(models))
	for _, m := range models {
		out = append(out, toResponse(m))
	}
	rs.runtime.Success(w, r, http.StatusOK, out, "Work time models retrieved")
}

func (rs *Resource) get(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	model, err := rs.models.FindWorkTimeModel(r.Context(), id)
	if err != nil {
		rs.runtime.Failure(w, r, FailureNotFound, errors.New("model not found"))
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toResponse(model), "Work time model retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req ModelRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	fields, err := buildFields(req)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	saved, err := rs.models.CreateWorkTimeModel(r.Context(), workforce.CreateWorkTimeModel{WorkTimeModelFields: fields})
	if err != nil {
		rs.renderSaveError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusCreated, toResponse(saved), "Work time model created")
}

// update rewrites the template and, in the same transaction, the schedule
// snapshots of every staff member bound to it. The time-account views are
// invalidated only once both writes succeeded.
func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	var req ModelRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	fields, err := buildFields(req)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	saved, err := rs.models.UpdateWorkTimeModel(r.Context(), workforce.UpdateWorkTimeModel{ID: id, WorkTimeModelFields: fields})
	if err != nil {
		rs.renderSaveError(w, r, err)
		return
	}
	if err := rs.models.RefreshAssignedStaffSchedules(r.Context(), id); err != nil {
		rs.renderSaveError(w, r, err)
		return
	}
	rs.runtime.NotifyChanged(r.Context())
	rs.runtime.Success(w, r, http.StatusOK, toResponse(saved), "Work time model updated")
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	if err := rs.models.DeleteWorkTimeModel(r.Context(), id); err != nil {
		rs.runtime.Failure(w, r, FailureNotFound, errors.New("model not found or in use"))
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, map[string]any{"id": id}, "Work time model deleted")
}

// renderSaveError maps a create/update capability error to its HTTP response:
// caller-input validation failures become 400, everything else 500.
func (rs *Resource) renderSaveError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, workforce.ErrInvalidWorkTime) {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	rs.runtime.Failure(w, r, FailureInternal, err)
}

// buildFields binds the wire request into capability input. It only parses the
// wire format (anchor date, per-entry start time) and drops zero-minute
// entries; the business rules are enforced by the Workforce capability.
func buildFields(req ModelRequest) (workforce.WorkTimeModelFields, error) {
	anchor := ""
	if req.RotationAnchorDate != "" {
		parsed, err := time.Parse(workforce.DateLayout, req.RotationAnchorDate)
		if err != nil || parsed.Format(workforce.DateLayout) != req.RotationAnchorDate {
			return workforce.WorkTimeModelFields{}, errors.New("rotation_anchor_date must be YYYY-MM-DD")
		}
		anchor = req.RotationAnchorDate
	}
	fields := workforce.WorkTimeModelFields{
		Name:               req.Name,
		RotationLength:     req.RotationLength,
		RotationAnchorDate: anchor,
		Entries:            make([]workforce.WorkTimeModelEntry, 0, len(req.Entries)),
	}
	for _, e := range req.Entries {
		if e.TargetMinutes == 0 {
			continue
		}
		startTime, err := parseOptionalStartTime(e.StartTime)
		if err != nil {
			return workforce.WorkTimeModelFields{}, err
		}
		fields.Entries = append(fields.Entries, workforce.WorkTimeModelEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     startTime,
		})
	}
	return fields, nil
}

func parseOptionalStartTime(raw *string) (string, error) {
	if raw == nil || *raw == "" {
		return "", nil
	}
	parsed, err := time.Parse(wireClockLayout, *raw)
	if err != nil {
		return "", errors.New("start_time must be HH:MM")
	}
	return parsed.Format(workforce.ClockLayout), nil
}

func formatOptionalStartTime(value string) *string {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(workforce.ClockLayout, value)
	if err != nil {
		return nil
	}
	formatted := parsed.Format(wireClockLayout)
	return &formatted
}

func toResponse(m workforce.WorkTimeModel) ModelResponse {
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
		RotationAnchorDate: rotationAnchor(m.RotationAnchorDate),
		Entries:            entries,
		WeeklyTotals:       totals,
	}
}

// rotationAnchor preserves the legacy zero-date rendering of a template whose
// anchor column is somehow empty.
func rotationAnchor(value string) string {
	if value == "" {
		return "0000-00-00"
	}
	return value
}
