package parentbillinghttp

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The mail and the print of the confirmation letters of a released run
// (#4670, ADR 0123). Sending answers 202: the mail waits on Delivery's
// outbox, and the run shows its state.

// mailings answers the mail state of every family of the run.
func (rs *BillingRunResource) mailings(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	mailings, err := rs.mail.LetterMailings(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, mailings, "Billing letter mails retrieved successfully")
}

// mailAll mails the letter of every family that has an address and no
// mail yet.
func (rs *BillingRunResource) mailAll(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	series, err := rs.mail.MailBillingLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, series, "Billing letters queued for mail")
}

// mailOne mails the letter of one family, first or again.
func (rs *BillingRunResource) mailOne(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	documentID, err := common.ParseIDParam(r, "documentID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	mailing, err := rs.mail.MailBillingLetter(r.Context(), reviewer, runID, documentID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, mailing, "Billing letter queued for mail")
}

// printLetters answers every letter of the run in one PDF.
func (rs *BillingRunResource) printLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	file, err := rs.mail.PrintBillingLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	writeLetter(w, file, "attachment")
}
