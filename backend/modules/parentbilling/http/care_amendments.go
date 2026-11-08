package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// CareAmendmentRequest is the body of POST …/{id}/amendments (#6197): one
// change of one invoice of a calculated run, or the revocation of one.
// ContractID and Service name the invoice, Fingerprint the review the
// change was made on; Kind decides which of the other fields count.
type CareAmendmentRequest struct {
	ContractID     int64                         `json:"contract_id"`
	Service        string                        `json:"service"`
	Kind           string                        `json:"kind"`
	Fingerprint    string                        `json:"fingerprint"`
	Reason         string                        `json:"reason"`
	TotalCents     *int64                        `json:"total_cents"`
	Label          string                        `json:"label"`
	AmountCents    *int64                        `json:"amount_cents"`
	PositionKey    string                        `json:"position_key"`
	Recipient      *capability.CareRecipientName `json:"recipient"`
	Address        *capability.PostalAddress     `json:"address"`
	RevokesID      int64                         `json:"revokes_id"`
	RevokesCarried bool                          `json:"revokes_carried"`
}

// Bind accepts the body; the Owner checks the change against the invoice.
func (req *CareAmendmentRequest) Bind(*http.Request) error { return nil }

// amend answers POST …/{id}/amendments with the run after the change.
func (rs *CareBillingResource) amend(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareAmendmentRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareAmendmentInvalid))
		return
	}
	detail, err := rs.billing.AmendCareInvoice(r.Context(), reviewer, runID, capability.CareAmendmentInput{
		ContractID: req.ContractID, Service: req.Service, Kind: req.Kind, Fingerprint: req.Fingerprint, Reason: req.Reason,
		TotalCents: req.TotalCents, Label: req.Label, AmountCents: req.AmountCents, PositionKey: req.PositionKey,
		Recipient: req.Recipient, Address: req.Address, RevokesID: req.RevokesID, RevokesCarried: req.RevokesCarried,
	})
	var fields validation.Errors
	if errors.As(err, &fields) {
		err = amendmentInvalid{err}
	}
	rs.respondReview(w, r, detail, err, "Invoice changed successfully")
}

// amendmentInvalid marks a refused change of an invoice whose fields are
// wrong, so it answers with the code of the change rather than of the run.
type amendmentInvalid struct{ error }

func (e amendmentInvalid) Unwrap() error { return e.error }
