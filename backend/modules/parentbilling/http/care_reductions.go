package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareReductionResource serves the rule chain of the monthly contribution
// (#4685): the care rules at /traeger/care-rules and the evidences of the
// contracts at /traeger/care-evidences. Every route sits behind the Träger
// session, needs the Träger right "Abrechnung" and runs in the Träger
// transaction of the session's Träger, which comes from the session, never
// the request.
type CareReductionResource struct {
	reductions     capability.CareReductions
	requireBilling common.Middleware
}

// NewCareReductionResource builds the resource. requireBilling admits only
// a session with the Träger right "Abrechnung".
func NewCareReductionResource(reductions capability.CareReductions, requireBilling common.Middleware) *CareReductionResource {
	if reductions == nil || requireBilling == nil {
		panic("parent billing http: the care reductions and the right check are required")
	}
	return &CareReductionResource{reductions: reductions, requireBilling: requireBilling}
}

// RuleRouter returns the routes mounted at /traeger/care-rules.
func (rs *CareReductionResource) RuleRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.rules)
		billing.Post("/", rs.createRule)
		billing.Put("/{id}", rs.updateRule)
		billing.Delete("/{id}", rs.deleteRule)
	})
	return r
}

// EvidenceRouter returns the routes mounted at /traeger/care-evidences.
func (rs *CareReductionResource) EvidenceRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.evidences)
		billing.Post("/", rs.createEvidence)
		billing.Put("/{id}", rs.updateEvidence)
		billing.Delete("/{id}", rs.deleteEvidence)
	})
	return r
}

// CareRuleRequest is the body of POST /traeger/care-rules and PUT
// /traeger/care-rules/{id}. Amounts are whole cent, shares basis points.
type CareRuleRequest struct {
	capability.CareRuleInput
}

// Bind accepts the body; the Owner validates the rule as a whole.
func (req *CareRuleRequest) Bind(*http.Request) error { return nil }

// CareEvidenceRequest is the body of POST /traeger/care-evidences and PUT
// /traeger/care-evidences/{id}.
type CareEvidenceRequest struct {
	capability.CareEvidenceInput
}

// Bind accepts the body; the Owner validates the evidence.
func (req *CareEvidenceRequest) Bind(*http.Request) error { return nil }

func (rs *CareReductionResource) rules(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	set, err := rs.reductions.CareRules(r.Context(), reviewer)
	if err != nil {
		renderCareReductionError(w, r, err, common.CodeParentBillingContributionRuleInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, set, "Care rules retrieved successfully")
}

func (rs *CareReductionResource) createRule(w http.ResponseWriter, r *http.Request) {
	rs.saveRule(w, r, 0, http.StatusCreated)
}

func (rs *CareReductionResource) updateRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.saveRule(w, r, ruleID, http.StatusOK)
}

func (rs *CareReductionResource) saveRule(w http.ResponseWriter, r *http.Request, ruleID int64, status int) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CareRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingContributionRuleInvalid))
		return
	}
	set, err := rs.reductions.SaveCareRule(r.Context(), reviewer, ruleID, req.CareRuleInput)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareReductionError(w, r, err, common.CodeParentBillingContributionRuleInvalid)
		return
	}
	common.Respond(w, r, status, set, "Care rule saved successfully")
}

func (rs *CareReductionResource) deleteRule(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	ruleID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	set, err := rs.reductions.DeleteCareRule(r.Context(), reviewer, ruleID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareReductionError(w, r, err, common.CodeParentBillingContributionRuleInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, set, "Care rule deleted successfully")
}

// evidences lists the evidences; the read is recorded per contract, so a
// failure rolls it back with the rest.
func (rs *CareReductionResource) evidences(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	set, err := rs.reductions.CareEvidences(r.Context(), reviewer)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareReductionError(w, r, err, common.CodeParentBillingCareEvidenceInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, set, "Care evidences retrieved successfully")
}

func (rs *CareReductionResource) createEvidence(w http.ResponseWriter, r *http.Request) {
	rs.saveEvidence(w, r, 0, http.StatusCreated)
}

func (rs *CareReductionResource) updateEvidence(w http.ResponseWriter, r *http.Request) {
	evidenceID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.saveEvidence(w, r, evidenceID, http.StatusOK)
}

func (rs *CareReductionResource) saveEvidence(w http.ResponseWriter, r *http.Request, evidenceID int64, status int) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CareEvidenceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareEvidenceInvalid))
		return
	}
	set, err := rs.reductions.SaveCareEvidence(r.Context(), reviewer, evidenceID, req.CareEvidenceInput)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareReductionError(w, r, err, common.CodeParentBillingCareEvidenceInvalid)
		return
	}
	common.Respond(w, r, status, set, "Care evidence saved successfully")
}

func (rs *CareReductionResource) deleteEvidence(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	evidenceID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	set, err := rs.reductions.DeleteCareEvidence(r.Context(), reviewer, evidenceID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareReductionError(w, r, err, common.CodeParentBillingCareEvidenceInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, set, "Care evidence deleted successfully")
}

// renderCareReductionError answers a refused rule or evidence; a field
// error carries the code of the object the handler writes.
func renderCareReductionError(w http.ResponseWriter, r *http.Request, err error, fallbackCode string) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, fallbackCode))
	case errors.Is(err, capability.ErrContributionRuleNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingContributionRuleNotFound))
	case errors.Is(err, capability.ErrContributionRuleOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingContributionRuleOverlap))
	case errors.Is(err, capability.ErrCareRuleInUse):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareRuleInUse))
	case errors.Is(err, capability.ErrCareEvidenceNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareEvidenceNotFound))
	case errors.Is(err, capability.ErrCareEvidenceOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareEvidenceOverlap))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
