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

// CostShareRouter returns the routes mounted at /traeger/care-cost-shares
// (#4687): the cost bearer shares of the contracts. Like the evidences,
// every route sits behind the Träger session, needs the Träger right
// "Abrechnung" and runs in the Träger transaction of the session's Träger.
func (rs *CareReductionResource) CostShareRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.costShares)
		billing.Post("/", rs.createCostShare)
		billing.Put("/{id}", rs.updateCostShare)
		billing.Delete("/{id}", rs.deleteCostShare)
	})
	return r
}

// CareCostShareRequest is the body of POST /traeger/care-cost-shares and
// PUT /traeger/care-cost-shares/{id}. A share is basis points, an amount
// whole cent.
type CareCostShareRequest struct {
	capability.CareCostShareInput
}

// Bind accepts the body; the Owner validates the share.
func (req *CareCostShareRequest) Bind(*http.Request) error { return nil }

func (rs *CareReductionResource) costShares(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	set, err := rs.reductions.CareCostShares(r.Context(), reviewer)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCostShareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, set, "Care cost shares retrieved successfully")
}

func (rs *CareReductionResource) createCostShare(w http.ResponseWriter, r *http.Request) {
	rs.saveCostShare(w, r, 0, http.StatusCreated)
}

func (rs *CareReductionResource) updateCostShare(w http.ResponseWriter, r *http.Request) {
	shareID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if shareID <= 0 {
		renderCostShareError(w, r, capability.ErrCareCostShareNotFound)
		return
	}
	rs.saveCostShare(w, r, shareID, http.StatusOK)
}

func (rs *CareReductionResource) saveCostShare(w http.ResponseWriter, r *http.Request, shareID int64, status int) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CareCostShareRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareCostShareInvalid))
		return
	}
	set, err := rs.reductions.SaveCareCostShare(r.Context(), reviewer, shareID, req.CareCostShareInput)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCostShareError(w, r, err)
		return
	}
	common.Respond(w, r, status, set, "Care cost share saved successfully")
}

func (rs *CareReductionResource) deleteCostShare(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	shareID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	set, err := rs.reductions.DeleteCareCostShare(r.Context(), reviewer, shareID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCostShareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, set, "Care cost share deleted successfully")
}

// renderCostShareError answers a refused share.
func renderCostShareError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareCostShareInvalid))
	case errors.Is(err, capability.ErrCareCostShareNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareCostShareNotFound))
	case errors.Is(err, capability.ErrCareCostShareOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareCostShareOverlap))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
