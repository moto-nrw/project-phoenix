package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// BillingCancellationRequest is the body of POST
// /traeger/billing-runs/{id}/documents/{documentID}/cancellation: the
// reason of the Storno (#4690).
type BillingCancellationRequest struct {
	Reason string `json:"reason"`
}

// Bind accepts the body; the Owner validates the reason.
func (req *BillingCancellationRequest) Bind(*http.Request) error { return nil }

// BillingCreditRequest is the body of POST
// /traeger/billing-runs/{id}/documents/{documentID}/credits: the child, the
// amount in whole cent and the reason of the Gutschrift (#4690).
type BillingCreditRequest struct {
	ChildID     int64  `json:"child_id"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}

// Bind accepts the body; the Owner validates the credit.
func (req *BillingCreditRequest) Bind(*http.Request) error { return nil }

// documentKey reads the session, the run and the invoice.
func documentKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, int64, bool) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, 0, false
	}
	documentID, err := common.ParseIDParam(r, "documentID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	return reviewer, runID, documentID, true
}

// cancel issues the Storno of an invoice. Any refusal rolls the Träger
// transaction back, so no number is used without its document.
func (rs *BillingRunResource) cancel(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, documentID, ok := documentKey(w, r)
	if !ok {
		return
	}
	req := &BillingCancellationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingDocumentCorrectionInvalid))
		return
	}
	document, err := rs.letters.CancelBillingDocument(r.Context(), reviewer, runID, documentID, capability.BillingCancellationInput{Reason: req.Reason})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderDocumentCorrectionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, document, "Invoice cancelled successfully")
}

// credit issues a Gutschrift for one child of an invoice, like cancel.
func (rs *BillingRunResource) credit(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, documentID, ok := documentKey(w, r)
	if !ok {
		return
	}
	req := &BillingCreditRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingDocumentCorrectionInvalid))
		return
	}
	document, err := rs.letters.CreditBillingDocument(r.Context(), reviewer, runID, documentID, capability.BillingCreditInput(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderDocumentCorrectionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, document, "Invoice credited successfully")
}

// renderDocumentCorrectionError answers a refused Storno or Gutschrift; the
// refusals of the run keep the codes of the billing runs.
func renderDocumentCorrectionError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingDocumentCorrectionInvalid))
	case errors.Is(err, capability.ErrBillingDocumentNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingBillingDocumentNotFound))
	case errors.Is(err, capability.ErrInvoiceCancelled):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingInvoiceCancelled))
	case errors.Is(err, capability.ErrInvoiceCredited):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingInvoiceCredited))
	default:
		renderBillingError(w, r, err)
	}
}
