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
//
// Authentication, transaction scoping, permission names, rendering and the
// actor lookup are supplied by the composition root through Runtime.
package absencetypes

import (
	"context"
	"errors"
	"net/http"
	"strconv"

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
	FailureInvalid      FailureKind = "invalid"
	FailureNotFound     FailureKind = "not_found"
	FailureConflict     FailureKind = "conflict"
	FailureUnauthorized FailureKind = "unauthorized"
	FailureInternal     FailureKind = "internal"
)

// Runtime carries the HTTP-platform behavior this adapter must not own. Every
// field is required, so missing production wiring fails at startup.
type Runtime struct {
	Protected     func(chi.Router, func(chi.Router, Middleware))
	Permission    func(string) Middleware
	AnyPermission func(...string) Middleware
	ParseID       func(*http.Request) (int64, error)
	ParseIDParam  func(*http.Request, string) (int64, error)
	Success       func(http.ResponseWriter, *http.Request, int, any, string)
	Failure       func(http.ResponseWriter, *http.Request, FailureKind, error)
	// ResolveActor identifies the staff member behind the request; an
	// allowance change is recorded against that person.
	ResolveActor func(context.Context) (int64, error)
	// Permission names. Reading is open to anyone who may decide absences
	// too, so the Leitung's approval views can render the school's wording.
	TimeTrackingOwn    string
	TimeTrackingManage string
	VacationApprove    string
}

// Resource bundles the dependencies for the absence-type HTTP handlers.
type Resource struct {
	types   workforce.AbsenceTypeAdministration
	runtime Runtime
}

// NewResource wires the dependencies.
func NewResource(types workforce.AbsenceTypeAdministration, runtime Runtime) *Resource {
	return &Resource{types: types, runtime: runtime}
}

// Router returns the chi sub-router for /api/absence-types.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		read := rs.runtime.AnyPermission(rs.runtime.TimeTrackingOwn, rs.runtime.TimeTrackingManage, rs.runtime.VacationApprove)
		manage := rs.runtime.Permission(rs.runtime.TimeTrackingManage)
		allowanceRead := rs.runtime.AnyPermission(rs.runtime.TimeTrackingManage, rs.runtime.VacationApprove)
		r.With(read, withTx).Get("/", rs.list)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Put("/{id}", rs.update)
		r.With(allowanceRead, withTx).Get("/{id}/allowances/{staffId}", rs.getAllowance)
		r.With(manage, withTx).Put("/{id}/allowances/{staffId}", rs.setAllowance)
	})

	return r
}

// CreateAbsenceTypeRequest is the create payload. Only the name is accepted:
// the base type is not client-controlled.
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

// SetAllowanceRequest is the payload of PUT /{id}/allowances/{staffId}.
type SetAllowanceRequest struct {
	Year         int     `json:"year"`
	EntitledDays float64 `json:"entitled_days"`
	Reason       string  `json:"reason"`
}

func toAllowanceSummaryResponse(summary workforce.AbsenceTypeAllowanceSummary) AllowanceSummaryResponse {
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

func toAbsenceTypeResponse(t workforce.StaffAbsenceType) AbsenceTypeResponse {
	return AbsenceTypeResponse{
		ID:               strconv.FormatInt(t.ID, 10),
		Name:             t.Name,
		BaseType:         t.BaseType,
		IsActive:         t.IsActive,
		AllowanceEnabled: t.AllowanceEnabled,
		OverrunPolicy:    t.OverrunPolicy,
	}
}

func toAbsenceTypeResponses(types []workforce.StaffAbsenceType) []AbsenceTypeResponse {
	out := make([]AbsenceTypeResponse, 0, len(types))
	for _, t := range types {
		out = append(out, toAbsenceTypeResponse(t))
	}
	return out
}

// classify maps a capability error to the failure kind the envelope renders:
// duplicates, reserved names, used names and exhausted allowances conflict,
// an unknown art is not found, malformed input is invalid.
func classify(err error) FailureKind {
	switch {
	case errors.Is(err, workforce.ErrAbsenceTypeNameTaken),
		errors.Is(err, workforce.ErrAbsenceTypeNameReserved),
		errors.Is(err, workforce.ErrAbsenceTypeInUse),
		errors.Is(err, workforce.ErrAbsenceTypeAllowanceExceeded):
		return FailureConflict
	case errors.Is(err, workforce.ErrAbsenceTypeNotFound):
		return FailureNotFound
	case errors.Is(err, workforce.ErrAbsenceTypeInvalid),
		errors.Is(err, workforce.ErrAbsenceTypeAllowanceInvalid):
		return FailureInvalid
	default:
		return FailureInternal
	}
}

func (rs *Resource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	rs.runtime.Failure(w, r, classify(err), err)
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	types, err := rs.types.ListAbsenceTypes(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toAbsenceTypeResponses(types), "Abwesenheitsarten geladen")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req CreateAbsenceTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	saved, err := rs.types.CreateAbsenceType(r.Context(), workforce.CreateAbsenceType{
		Name: req.Name, AllowanceEnabled: req.AllowanceEnabled, OverrunPolicy: req.OverrunPolicy,
	})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusCreated, toAbsenceTypeResponse(saved), "Abwesenheitsart erstellt")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	var req UpdateAbsenceTypeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	saved, err := rs.types.UpdateAbsenceType(r.Context(), workforce.UpdateAbsenceType{
		ID: id, Name: req.Name, IsActive: req.IsActive, AllowanceEnabled: req.AllowanceEnabled, OverrunPolicy: req.OverrunPolicy,
	})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toAbsenceTypeResponse(saved), "Abwesenheitsart aktualisiert")
}

func (rs *Resource) getAllowance(w http.ResponseWriter, r *http.Request) {
	typeID, staffID, year, err := rs.allowancePath(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	summary, err := rs.types.AllowanceSummary(r.Context(), staffID, typeID, year)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toAllowanceSummaryResponse(summary), "Kontingent geladen")
}

func (rs *Resource) setAllowance(w http.ResponseWriter, r *http.Request) {
	typeID, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	staffID, err := rs.runtime.ParseIDParam(r, "staffId")
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	actorID, err := rs.runtime.ResolveActor(r.Context())
	if err != nil || actorID <= 0 {
		if err == nil {
			err = errors.New("current staff member not found")
		}
		rs.runtime.Failure(w, r, FailureUnauthorized, err)
		return
	}
	var req SetAllowanceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err)
		return
	}
	summary, err := rs.types.SetAllowance(r.Context(), workforce.SetAbsenceTypeAllowance{
		StaffID: staffID, AbsenceTypeID: typeID, Year: req.Year,
		EntitledDays: req.EntitledDays, Reason: req.Reason, ChangedBy: actorID,
	})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toAllowanceSummaryResponse(summary), "Kontingent gespeichert")
}

func (rs *Resource) allowancePath(r *http.Request) (int64, int64, int, error) {
	typeID, err := rs.runtime.ParseID(r)
	if err != nil {
		return 0, 0, 0, err
	}
	staffID, err := rs.runtime.ParseIDParam(r, "staffId")
	if err != nil {
		return 0, 0, 0, err
	}
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		return 0, 0, 0, err
	}
	return typeID, staffID, year, nil
}
