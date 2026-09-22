// Package setup serves the onboarding wizard for new schools (#2832, ADR
// 0035) under /api/school-setup. Every route needs config:update: the wizard
// is for school admins, and it is the permission the steps' own pages ask for.
package setup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

type Middleware = func(http.Handler) http.Handler

// Runtime supplies delivery and request-context mechanics. The composition
// root implements it.
type Runtime struct {
	// Protected mounts the tenant group; withTx opens the tenant transaction.
	Protected func(chi.Router, func(r chi.Router, withTx Middleware))
	// RequireWrite checks config:update.
	RequireWrite Middleware
	// Actor returns the school and account of the request.
	Actor   func(context.Context) (tenantID, accountID int64)
	Respond func(w http.ResponseWriter, r *http.Request, status int, data any, message string)
	// Failure renders 400, 403, 404, 409 or 500.
	Failure func(w http.ResponseWriter, r *http.Request, status int, err error)
}

// Service is the wizard capability of Settings Platform.
type Service interface {
	Status(ctx context.Context, tenantID, accountID int64) (configSvc.SchoolSetupStatus, error)
	ConfirmBasics(ctx context.Context, tenantID, accountID int64, presenceMode string, parentAppUsed bool) error
	SetStepSkipped(ctx context.Context, tenantID, accountID int64, step configModel.SetupStep, skipped bool) error
	Complete(ctx context.Context, tenantID, accountID int64) error
	SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error
}

// Resource serves the wizard routes.
type Resource struct {
	service Service
	runtime Runtime
}

// NewResource builds the resource.
func NewResource(service Service, runtime Runtime) *Resource {
	if service == nil || runtime.Protected == nil || runtime.RequireWrite == nil || runtime.Actor == nil || runtime.Respond == nil || runtime.Failure == nil {
		panic("school setup http: service and runtime are required")
	}
	return &Resource{service: service, runtime: runtime}
}

// Router returns the routes mounted at /school-setup.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		r.Use(rs.runtime.RequireWrite)
		r.With(withTx).Get("/", rs.status)
		r.With(withTx).Put("/basics", rs.confirmBasics)
		r.With(withTx).Put("/steps/{step}", rs.skipStep)
		r.With(withTx).Post("/complete", rs.complete)
		r.With(withTx).Put("/dismissal", rs.dismiss)
	})
	return r
}

type basicsRequest struct {
	PresenceMode  string `json:"presence_mode"`
	ParentAppUsed *bool  `json:"parent_app_used"`
}

type skipRequest struct {
	Skipped bool `json:"skipped"`
}

type dismissalRequest struct {
	Dismissed bool `json:"dismissed"`
}

func (rs *Resource) status(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	status, err := rs.service.Status(r.Context(), tenantID, accountID)
	if err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.runtime.Respond(w, r, http.StatusOK, status, "")
}

func (rs *Resource) confirmBasics(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	var req basicsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
		return
	}
	if req.ParentAppUsed == nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("parent_app_used is required"))
		return
	}
	if err := rs.service.ConfirmBasics(r.Context(), tenantID, accountID, req.PresenceMode, *req.ParentAppUsed); err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

func (rs *Resource) skipStep(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	step, err := configModel.ParseSetupStep(chi.URLParam(r, "step"))
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusNotFound, err)
		return
	}
	var req skipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
		return
	}
	if err := rs.service.SetStepSkipped(r.Context(), tenantID, accountID, step, req.Skipped); err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

func (rs *Resource) complete(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	if err := rs.service.Complete(r.Context(), tenantID, accountID); err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

func (rs *Resource) dismiss(w http.ResponseWriter, r *http.Request) {
	tenantID, accountID, ok := rs.actor(w, r)
	if !ok {
		return
	}
	var req dismissalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
		return
	}
	if err := rs.service.SetDismissed(r.Context(), tenantID, accountID, req.Dismissed); err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.respondStatus(w, r, tenantID, accountID)
}

// respondStatus answers every write with the fresh wizard state, so the client
// renders one source of truth.
func (rs *Resource) respondStatus(w http.ResponseWriter, r *http.Request, tenantID, accountID int64) {
	status, err := rs.service.Status(r.Context(), tenantID, accountID)
	if err != nil {
		rs.fail(w, r, err)
		return
	}
	rs.runtime.Respond(w, r, http.StatusOK, status, "")
}

func (rs *Resource) actor(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	tenantID, accountID := rs.runtime.Actor(r.Context())
	if tenantID <= 0 || accountID <= 0 {
		rs.runtime.Failure(w, r, http.StatusForbidden, errors.New("no school account context"))
		return 0, 0, false
	}
	return tenantID, accountID, true
}

func (rs *Resource) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, configSvc.ErrSchoolSetupCompleted), errors.Is(err, configSvc.ErrSchoolSetupIncomplete):
		rs.runtime.Failure(w, r, http.StatusConflict, err)
	case errors.Is(err, configSvc.ErrInvalidPresenceMode), errors.Is(err, configSvc.ErrSetupStepNotSkippable):
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
	case errors.Is(err, configSvc.ErrPresenceModeSwitchBlocked):
		rs.runtime.Failure(w, r, http.StatusConflict, err)
	default:
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err)
	}
}
