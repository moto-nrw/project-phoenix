// Package traegercataloghttp serves the Träger catalog of the Träger portal
// (ADR 0195): the office keeps its Träger phases and offerings in the Träger
// transaction (withTx) and previews and rolls them out through the workflow,
// which opens its transactions itself.
package traegercataloghttp

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
)

// Resource serves /traeger/catalog.
type Resource struct {
	phases        enrollment.TraegerPhases
	offerings     careplan.TraegerOfferings
	catalog       traegercatalog.Catalog
	requireManage common.Middleware
}

// NewResource builds the resource; every collaborator is required.
func NewResource(phases enrollment.TraegerPhases, offerings careplan.TraegerOfferings, catalog traegercatalog.Catalog, requireManage common.Middleware) *Resource {
	if phases == nil || offerings == nil || catalog == nil || requireManage == nil {
		panic("träger catalog HTTP: phases, offerings, catalog and the permission are required")
	}
	return &Resource{phases: phases, offerings: offerings, catalog: catalog, requireManage: requireManage}
}

// Router mounts the routes: the office keeps its catalog in the Träger
// transaction; preview and rollout run without it, because they open the
// Träger and the school transactions one after the other.
func (rs *Resource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		keep := r.With(rs.requireManage, withTx)
		keep.Get("/phases", rs.listPhases)
		keep.Post("/phases", rs.createPhase)
		keep.Get("/phases/{id}", rs.getPhase)
		keep.Put("/phases/{id}", rs.updatePhase)
		keep.Post("/phases/{id}/withdraw", rs.withdrawPhase)
		keep.Post("/phases/{id}/offerings", rs.createOffering)
		keep.Put("/offerings/{id}", rs.updateOffering)
		keep.Post("/offerings/{id}/withdraw", rs.withdrawOffering)
		roll := r.With(rs.requireManage)
		roll.Post("/phases/{id}/preview", rs.preview)
		roll.Post("/phases/{id}/rollout", rs.rollout)
	})
	return r
}

// PhaseDetail is a Träger phase with its offerings.
type PhaseDetail struct {
	enrollment.TraegerPhase
	Offerings []careplan.TraegerOffering `json:"offerings"`
}

// PhaseRequest is the office's draft of a Träger phase.
type PhaseRequest struct {
	IsActive                  *bool      `json:"is_active"`
	Name                      string     `json:"name"`
	Kind                      string     `json:"kind"`
	ServiceStartDate          string     `json:"service_start_date"`
	ServiceEndDate            string     `json:"service_end_date"`
	EnrollmentOpenAt          *time.Time `json:"enrollment_open_at"`
	EnrollmentCloseAt         *time.Time `json:"enrollment_close_at"`
	Audience                  string     `json:"audience"`
	CareOverflowMode          string     `json:"care_overflow_mode"`
	CareOfferingSelectionMode string     `json:"care_offering_selection_mode"`
	AllSchools                bool       `json:"all_schools"`
	SchoolIDs                 []int64    `json:"school_ids"`
}

// Bind accepts the request; the owner validates it.
func (*PhaseRequest) Bind(*http.Request) error { return nil }

func (req *PhaseRequest) input() enrollment.TraegerPhaseInput {
	return enrollment.TraegerPhaseInput{
		Fields: enrollment.TraegerPhaseFields{
			IsActive: req.IsActive, Name: req.Name, Kind: req.Kind, ServiceStartDate: enrollment.Date(req.ServiceStartDate),
			ServiceEndDate: enrollment.Date(req.ServiceEndDate), EnrollmentOpenAt: req.EnrollmentOpenAt,
			EnrollmentCloseAt: req.EnrollmentCloseAt, Audience: req.Audience, CareOverflowMode: req.CareOverflowMode,
			CareOfferingSelectionMode: req.CareOfferingSelectionMode,
		},
		AllSchools: req.AllSchools, SchoolIDs: req.SchoolIDs,
	}
}

// OfferingRequest is the office's draft of a Träger care offering.
type OfferingRequest struct {
	careplan.TraegerOfferingDraft
}

// Bind accepts the request; the owner validates it.
func (*OfferingRequest) Bind(*http.Request) error { return nil }

// RolloutRequest confirms a preview by its fingerprint.
type RolloutRequest struct {
	Fingerprint string `json:"fingerprint"`
}

// Bind requires the fingerprint.
func (req *RolloutRequest) Bind(*http.Request) error {
	if req.Fingerprint == "" {
		return errors.New("fingerprint is required")
	}
	return nil
}

func (rs *Resource) listPhases(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorOf(w, r)
	if !ok {
		return
	}
	phases, err := rs.phases.ListTraegerPhases(r.Context(), actor.OrganizationID)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, phases, "Träger phases retrieved successfully")
}

func (rs *Resource) getPhase(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	rs.respondPhase(w, r, actor, id, http.StatusOK)
}

func (rs *Resource) respondPhase(w http.ResponseWriter, r *http.Request, actor traegercatalog.Actor, id int64, status int) {
	phase, err := rs.phases.TraegerPhase(r.Context(), actor.OrganizationID, id)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	offerings, err := rs.offerings.ListTraegerOfferings(r.Context(), actor.OrganizationID, id)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, status, PhaseDetail{TraegerPhase: phase, Offerings: offerings}, "Träger phase retrieved successfully")
}

func (rs *Resource) createPhase(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorOf(w, r)
	if !ok {
		return
	}
	req := &PhaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	phase, err := rs.phases.CreateTraegerPhase(r.Context(), actor.OrganizationID, req.input())
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	rs.respondPhase(w, r, actor, phase.ID, http.StatusCreated)
}

func (rs *Resource) updatePhase(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	req := &PhaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.phases.UpdateTraegerPhase(r.Context(), actor.OrganizationID, id, req.input()); err != nil {
		renderCatalogError(w, r, err)
		return
	}
	rs.respondPhase(w, r, actor, id, http.StatusOK)
}

func (rs *Resource) withdrawPhase(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	if _, err := rs.phases.WithdrawTraegerPhase(r.Context(), actor.OrganizationID, id); err != nil {
		renderCatalogError(w, r, err)
		return
	}
	rs.respondPhase(w, r, actor, id, http.StatusOK)
}

func (rs *Resource) createOffering(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	req := &OfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	offering, err := rs.offerings.CreateTraegerOffering(r.Context(), actor.OrganizationID, id, req.TraegerOfferingDraft)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, offering, "Träger care offering created successfully")
}

func (rs *Resource) updateOffering(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	req := &OfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	offering, err := rs.offerings.UpdateTraegerOffering(r.Context(), actor.OrganizationID, id, req.TraegerOfferingDraft)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, offering, "Träger care offering updated successfully")
}

func (rs *Resource) withdrawOffering(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	offering, err := rs.offerings.WithdrawTraegerOffering(r.Context(), actor.OrganizationID, id)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, offering, "Träger care offering withdrawn successfully")
}

func (rs *Resource) preview(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	preview, err := rs.catalog.Preview(r.Context(), actor, id)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, preview, "Träger phase preview retrieved successfully")
}

func (rs *Resource) rollout(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := actorAndID(w, r)
	if !ok {
		return
	}
	req := &RolloutRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rollout, err := rs.catalog.Rollout(r.Context(), actor, id, req.Fingerprint)
	if err != nil {
		renderCatalogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, rollout, "Träger phase rolled out")
}

// actorOf is the Träger office of the session; the Träger never comes
// from the request.
func actorOf(w http.ResponseWriter, r *http.Request) (traegercatalog.Actor, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("träger session required")))
		return traegercatalog.Actor{}, false
	}
	return traegercatalog.Actor{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

func actorAndID(w http.ResponseWriter, r *http.Request) (traegercatalog.Actor, int64, bool) {
	actor, ok := actorOf(w, r)
	if !ok {
		return actor, 0, false
	}
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return actor, 0, false
	}
	return actor, id, true
}

type codedError interface{ ErrorCode() string }

var catalogErrorRules = []common.ErrorRule{
	{Target: enrollment.ErrTraegerPhaseNotFound, Render: notFound(common.CodeEnrollmentTraegerPhaseNotFound)},
	{Target: careplan.ErrTraegerOfferingNotFound, Render: notFound(common.CodeEnrollmentTraegerOfferingNotFound)},
	{Target: enrollment.ErrTraegerPhaseWithdrawn, Render: conflict(common.CodeEnrollmentTraegerWithdrawn)},
	{Target: careplan.ErrTraegerOfferingWithdrawn, Render: conflict(common.CodeEnrollmentTraegerWithdrawn)},
	{Target: traegercatalog.ErrPreviewStale, Render: conflict(common.CodeEnrollmentTraegerPreviewStale)},
	{Target: enrollment.ErrTraegerSchoolOutsideTraeger, Render: invalid(common.CodeEnrollmentTraegerSchoolOutside)},
	{Match: func(err error) bool { var coded codedError; return errors.As(err, &coded) }, Render: func(err error) render.Renderer {
		var coded codedError
		errors.As(err, &coded)
		return common.ErrorInvalidRequestWithCode(err, coded.ErrorCode())
	}},
	{Target: enrollment.ErrInvalidPhase, Render: invalid(common.CodeEnrollmentTraegerCatalogInvalid)},
	{Target: careplan.ErrCareOfferingConfigInvalid, Render: invalid(common.CodeEnrollmentTraegerCatalogInvalid)},
	{Target: careplan.ErrInvalidCareOffering, Render: invalid(common.CodeEnrollmentTraegerCatalogInvalid)},
	{Target: careplan.ErrCareOfferingPickupTimesRequired, Render: invalid(common.CodeEnrollmentCareOfferingPickupTimesRequired)},
	{Target: careplan.ErrCareOfferingDaysRequired, Render: invalid(common.CodeEnrollmentCareOfferingDaysRequired)},
}

var renderCatalogFailure = common.RulesRenderer(catalogErrorRules, common.ErrorInternalServer)

// renderCatalogError discards the Träger transaction of a refused write and
// answers with the refusal's code.
func renderCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	common.RenderError(w, r, renderCatalogFailure(err))
}

func notFound(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorNotFoundWithCode(err, code) }
}

func conflict(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorConflictWithCode(err, code) }
}

func invalid(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorInvalidRequestWithCode(err, code) }
}
