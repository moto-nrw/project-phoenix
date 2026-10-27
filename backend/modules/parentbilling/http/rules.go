// Package parentbillinghttp serves the Elternabrechnung in the Träger-Portal
// and the contribution of the public holiday registration (#4664, ADR 0115).
package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RuleResource serves the contribution rules of a Träger at
// /traeger/contribution-rules. Every route sits behind the Träger session,
// needs the Träger right "Abrechnung" and runs in the Träger transaction of
// the session's Träger (ADR 0105). The Träger comes from the session, never
// from the request.
type RuleResource struct {
	rules          capability.ContributionRules
	requireBilling common.Middleware
}

// NewRuleResource builds the resource. requireBilling admits only a session
// with the Träger right "Abrechnung"; the root binds it to the right
// Identity & Access bundles into the Träger roles.
func NewRuleResource(rules capability.ContributionRules, requireBilling common.Middleware) *RuleResource {
	if rules == nil || requireBilling == nil {
		panic("parent billing http: the contribution rules and the right check are required")
	}
	return &RuleResource{rules: rules, requireBilling: requireBilling}
}

// TraegerRouter returns the routes mounted at /traeger/contribution-rules.
func (rs *RuleResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.list)
		billing.Post("/", rs.create)
		billing.Put("/{id}", rs.update)
		billing.Delete("/{id}", rs.delete)
	})
	return r
}

// ContributionRuleRequest is the body of POST /traeger/contribution-rules
// and PUT /traeger/contribution-rules/{id}. Amounts are whole cent, shares
// basis points (10000 = 100 %).
type ContributionRuleRequest struct {
	Service             string         `json:"service"`
	Kind                string         `json:"kind"`
	ValidFrom           calendar.Date  `json:"valid_from"`
	ValidUntil          *calendar.Date `json:"valid_until"`
	SiblingFromChild    *int           `json:"sibling_from_child"`
	DiscountBasisPoints *int           `json:"discount_basis_points"`
	DiscountCents       *int64         `json:"discount_cents"`
	Note                string         `json:"note"`
}

// Bind accepts the body; the Owner validates the rule as a whole.
func (req *ContributionRuleRequest) Bind(*http.Request) error { return nil }

func (req *ContributionRuleRequest) input() capability.ContributionRuleInput {
	return capability.ContributionRuleInput(*req)
}

// traegerOf returns the Träger of the session, or renders 401.
func traegerOf(w http.ResponseWriter, r *http.Request) (int64, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, false
	}
	return principal.OrganizationID(), true
}

func (rs *RuleResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	rules, err := rs.rules.ListContributionRules(r.Context(), organizationID)
	if err != nil {
		renderRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, rules, "Contribution rules retrieved successfully")
}

func (rs *RuleResource) create(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	req := &ContributionRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rule, err := rs.rules.CreateContributionRule(r.Context(), organizationID, req.input())
	if err != nil {
		renderRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, rule, "Contribution rule created successfully")
}

func (rs *RuleResource) update(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	ruleID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &ContributionRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rule, err := rs.rules.UpdateContributionRule(r.Context(), organizationID, ruleID, req.input())
	if err != nil {
		renderRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, rule, "Contribution rule updated successfully")
}

func (rs *RuleResource) delete(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	ruleID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.rules.DeleteContributionRule(r.Context(), organizationID, ruleID); err != nil {
		renderRuleError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// renderRuleError answers a refused rule.
func renderRuleError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingContributionRuleInvalid))
	case errors.Is(err, capability.ErrContributionRuleNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingContributionRuleNotFound))
	case errors.Is(err, capability.ErrContributionRuleOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingContributionRuleOverlap))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
