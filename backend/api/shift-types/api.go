// Package shifttypes exposes admin CRUD for tenant-defined shift types
// (Schichtarten, #1836) used to label planned staff shifts in the Dienstplan.
// A shift type carries a name, a hex color, an optional description and an
// active flag; the color makes different duties distinguishable in the week
// grid. Guarded by time_tracking:manage, same as staff shifts.
//
// Authentication, transaction scoping, permission names, rendering and the
// rollback marker are supplied by the composition root through Runtime; the
// handlers call the public Workforce shift-type administration contract.
package shifttypes

import (
	"context"
	"errors"
	"net/http"

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
	FailureConflict FailureKind = "conflict"
	FailureInternal FailureKind = "internal"
)

// Runtime carries the HTTP-platform behavior this adapter must not own. Every
// field is required, so missing production wiring fails at startup.
type Runtime struct {
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Permission func(string) Middleware
	ParseID    func(*http.Request) (int64, error)
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, FailureKind, error)
	// MarkRollback discards the request's tenant transaction so a rejected
	// category mapping never commits the shift-type write without it.
	MarkRollback func(context.Context)
	// TimeTrackingManage is the permission every route requires.
	TimeTrackingManage string
}

// Resource bundles the dependencies for the shift-type HTTP handlers.
type Resource struct {
	types   workforce.ShiftTypeAdministration
	runtime Runtime
}

// NewResource wires the dependencies.
func NewResource(types workforce.ShiftTypeAdministration, runtime Runtime) *Resource {
	return &Resource{types: types, runtime: runtime}
}

// Router returns the chi sub-router for /api/shift-types.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		manage := rs.runtime.Permission(rs.runtime.TimeTrackingManage)
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

func toShiftTypeResponse(t workforce.ShiftType) ShiftTypeResponse {
	return ShiftTypeResponse{
		ID:          t.ID,
		Name:        t.Name,
		Color:       t.Color,
		Description: t.Description,
		IsActive:    t.IsActive,
	}
}

func toShiftTypeResponses(types []workforce.ShiftType) []ShiftTypeResponse {
	out := make([]ShiftTypeResponse, 0, len(types))
	for _, t := range types {
		out = append(out, toShiftTypeResponse(t))
	}
	return out
}

func toShiftTypeInput(id int64, req ShiftTypeRequest) workforce.ShiftTypeInput {
	return workforce.ShiftTypeInput{
		ID:          id,
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
		IsActive:    req.IsActive,
		CategoryIDs: req.CategoryIDs,
	}
}

// failureRule pairs a capability error with the failure kind the envelope
// renders for it; anything not listed is an internal failure.
type failureRule struct {
	Target error
	Kind   FailureKind
}

var failureRules = []failureRule{
	{Target: workforce.ErrShiftTypeNameTaken, Kind: FailureConflict},
	{Target: workforce.ErrShiftTypeNotFound, Kind: FailureNotFound},
	{Target: workforce.ErrInvalidShiftType, Kind: FailureInvalid},
	{Target: workforce.ErrShiftTypeCategoryUnknown, Kind: FailureInvalid},
}

func classify(err error) FailureKind {
	for _, rule := range failureRules {
		if errors.Is(err, rule.Target) {
			return rule.Kind
		}
	}
	return FailureInternal
}

// renderError maps a capability failure. Unknown/cross-tenant category IDs
// are client input errors (400) and must roll back the whole request so the
// shift-type write does not persist without its mapping.
func (rs *Resource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, workforce.ErrShiftTypeCategoryUnknown) {
		rs.runtime.MarkRollback(r.Context())
	}
	rs.runtime.Failure(w, r, classify(err), err)
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	types, err := rs.types.ListShiftTypes(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toShiftTypeResponses(types), "Shift types retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req ShiftTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	saved, err := rs.types.CreateShiftType(r.Context(), toShiftTypeInput(0, req))
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusCreated, toShiftTypeResponse(saved), "Shift type created")
}

func (rs *Resource) createDefaults(w http.ResponseWriter, r *http.Request) {
	types, err := rs.types.CreateDefaultShiftTypes(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toShiftTypeResponses(types), "Default shift types created")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	var req ShiftTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	// A PUT that omits is_active must not silently flip the stored state; the
	// capability preserves the current value when the field is nil.
	saved, err := rs.types.UpdateShiftType(r.Context(), toShiftTypeInput(id, req))
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toShiftTypeResponse(saved), "Shift type updated")
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	if err := rs.types.DeleteShiftType(r.Context(), id); err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, map[string]any{"id": id}, "Shift type deleted")
}
