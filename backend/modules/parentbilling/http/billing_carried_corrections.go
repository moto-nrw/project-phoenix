package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BillingCarriedCorrectionRequest is the body of PUT
// /traeger/billing-runs/{id}/carried-corrections/{positionID}: the new amount
// of the child in whole cent and the reason (#5663).
type BillingCarriedCorrectionRequest struct {
	TotalCents int64  `json:"total_cents"`
	Reason     string `json:"reason"`
}

// Bind accepts the body; the Owner validates the correction.
func (req *BillingCarriedCorrectionRequest) Bind(*http.Request) error { return nil }

// carriedKey reads the session, the run and the proposal.
func carriedKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, int64, bool) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, 0, false
	}
	positionID, err := common.ParseIDParam(r, "positionID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	return reviewer, runID, positionID, true
}

// removeCarried removes a proposed correction from the preview. Any refusal
// rolls the Träger transaction back.
func (rs *BillingRunResource) removeCarried(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, positionID, ok := carriedKey(w, r)
	if !ok {
		return
	}
	detail, err := rs.runs.RemoveCarriedCorrection(r.Context(), reviewer, runID, positionID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCarriedCorrectionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Carried correction removed successfully")
}

// changeCarried replaces a proposed correction with a correction of the run,
// like removeCarried.
func (rs *BillingRunResource) changeCarried(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, positionID, ok := carriedKey(w, r)
	if !ok {
		return
	}
	req := &BillingCarriedCorrectionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingCorrectionInvalid))
		return
	}
	detail, err := rs.runs.ChangeCarriedCorrection(r.Context(), reviewer, runID, positionID, capability.BillingCarriedCorrectionChange(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCarriedCorrectionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Carried correction changed successfully")
}

// renderCarriedCorrectionError answers a refused change of a proposal; the
// refusals of the run keep the codes of the billing runs.
func renderCarriedCorrectionError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, capability.ErrCarriedCorrectionNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCarriedCorrectionNotFound))
		return
	}
	renderBillingError(w, r, err)
}
