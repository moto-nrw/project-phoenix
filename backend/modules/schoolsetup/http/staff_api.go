package schoolsetuphttp

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
)

// StaffService is the first-steps capability of a care worker (#3748).
type StaffService = schoolsetup.StaffOnboardingService

// StaffResource serves the first steps of a care worker under
// /api/staff-onboarding. The routes need no permission: every person reads and
// writes only their own progress, keyed by the account of the token, and every
// page a tour leads to checks its own permissions.
type StaffResource struct {
	service StaffService
	runtime Runtime
}

// NewStaffResource builds the resource. RequireWrite is not used.
func NewStaffResource(service StaffService, runtime Runtime) *StaffResource {
	if service == nil || runtime.Protected == nil || runtime.Actor == nil || runtime.Respond == nil || runtime.Failure == nil {
		panic("staff onboarding http: service and runtime are required")
	}
	return &StaffResource{service: service, runtime: runtime}
}

// Router returns the routes mounted at /staff-onboarding.
func (rs *StaffResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		r.With(withTx).Get("/", rs.status)
		r.With(withTx).Put("/steps/{step}", rs.setStepState)
		r.With(withTx).Put("/dismissal", rs.dismiss)
	})
	return r
}

type stepStateRequest struct {
	State string `json:"state"`
}

func (rs *StaffResource) status(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

func (rs *StaffResource) setStepState(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	var req stepStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
		return
	}
	step := chi.URLParam(r, "step")
	if err := rs.service.SetStaffStepState(r.Context(), tenantID, accountID, step, req.State); err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

func (rs *StaffResource) dismiss(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	var req dismissalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
		return
	}
	if err := rs.service.SetStaffDismissed(r.Context(), tenantID, accountID, req.Dismissed); err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

// respondStatus answers every write with the fresh state, so the client
// renders one source of truth.
func (rs *StaffResource) respondStatus(w http.ResponseWriter, r *http.Request, tenantID, accountID int64) {
	status, err := rs.service.StaffStatus(r.Context(), tenantID, accountID)
	if err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.runtime.Respond(w, r, http.StatusOK, status, "")
}

func (rs *StaffResource) actor(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	tenantID, accountID := rs.runtime.Actor(r.Context())
	if tenantID <= 0 || accountID <= 0 {
		rs.runtime.Failure(w, r, http.StatusForbidden, errors.New("no school account context"))
		return 0, 0, false
	}
	return tenantID, accountID, true
}

func (rs *StaffResource) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, schoolsetup.ErrUnknownStep):
		rs.runtime.Failure(w, r, http.StatusNotFound, err)
	case errors.Is(err, schoolsetup.ErrUnknownStepState):
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
	default:
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err)
	}
}
