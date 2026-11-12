package carecontracthttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// ChangeResource serves /traeger/contract-changes (#5979): the Träger-Büro
// reads the change of the booked care the parents of a contract's child
// asked for and approves or rejects it. Every route requires the right
// "Verträge verwalten"; the owner opens the Träger and the school
// transaction itself.
type ChangeResource struct {
	changes         carecontract.ContractChangeDecisions
	requireContract common.Middleware
}

// NewChangeResource binds the routes to the owner and the right check.
func NewChangeResource(changes carecontract.ContractChangeDecisions, requireContract common.Middleware) *ChangeResource {
	if changes == nil || requireContract == nil {
		panic("care contract http: the change decisions and the right check are required")
	}
	return &ChangeResource{changes: changes, requireContract: requireContract}
}

// Router returns the routes mounted at /traeger/contract-changes.
func (rs *ChangeResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		open := r.With(rs.requireContract)
		open.Get("/{id}", rs.list)
		open.Post("/{id}/requests/{requestId}/decide", rs.decide)
	})
	return r
}

func (rs *ChangeResource) list(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	var effectiveFrom *calendar.Date
	if value := r.URL.Query().Get("effective_from"); value != "" {
		date, err := calendar.ParseDate(value)
		if err != nil {
			RenderChangeError(w, r, carecontract.ErrChangeInvalid)
			return
		}
		effectiveFrom = &date
	}
	changes, err := rs.changes.ContractChanges(r.Context(), reviewer, id, effectiveFrom)
	if err != nil {
		RenderChangeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, changes, "Contract changes retrieved successfully")
}

// ChangeDecisionRequest is the body of a decision.
type ChangeDecisionRequest struct {
	carecontract.ChangeDecisionInput
}

// Bind accepts the body; the owner checks the decision.
func (req *ChangeDecisionRequest) Bind(*http.Request) error { return nil }

func (rs *ChangeResource) decide(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	requestID, err := common.ParseIDParam(r, "requestId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var body ChangeDecisionRequest
	if err := render.Bind(r, &body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	changes, err := rs.changes.DecideContractChange(r.Context(), reviewer, id, requestID, body.ChangeDecisionInput)
	if err != nil {
		RenderChangeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, changes, "Contract change decided")
}

// changeConflictRules map the refusals the office can act on to their
// codes; they keep the codes the school's decision answers with.
var changeConflictRules = []struct {
	err  error
	code string
}{
	{carecontract.ErrContractNotActive, common.CodeCareContractChangeContractNotActive},
	{carecontract.ErrChangeDecided, common.CodeStudentsChangeRequestNotPending},
	{carecontract.ErrChangeStale, common.CodeStudentsChangeRequestStale},
	{carecontract.ErrChangePast, common.CodeStudentsRequestPast},
	{carecontract.ErrChangeEndsCare, common.CodeCareContractChangeEndsCare},
	{carecontract.ErrChangeCapacityFull, common.CodeStudentsOfferingChangeCapacityFull},
	{carecontract.ErrChangeNotApplicable, common.CodeCareContractChangeNotApplicable},
}

// RenderChangeError renders a failure of the office's decision.
func RenderChangeError(w http.ResponseWriter, r *http.Request, err error) {
	for _, rule := range changeConflictRules {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, rule.code))
			return
		}
	}
	switch {
	case errors.Is(err, carecontract.ErrChangeNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeCareContractChangeNotFound))
	case errors.Is(err, carecontract.ErrChangeReasonRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeStudentsReasonRequired))
	case errors.Is(err, carecontract.ErrChangeDateOutOfRange):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeStudentsOfferingChangeDateOutOfRange))
	case errors.Is(err, carecontract.ErrChangeInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareContractFieldInvalid))
	default:
		renderError(w, r, err)
	}
}
