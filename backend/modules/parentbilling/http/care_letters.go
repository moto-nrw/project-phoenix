package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The letters, the mail, the print and the Storno of a released monthly run
// (#4684, ADR 0165) under /traeger/care-billing-runs/{id}. They sit behind the
// same right "Abrechnung" and Träger transaction as the run. The kind query
// parameter names the letter of an invoice ("invoice", the default) or of its
// Storno ("cancellation"). Sending answers 202: the mail waits on Delivery's
// outbox, and the run shows its state.

// mountLetters adds the letter routes of a released run.
func (rs *CareBillingResource) mountLetters(billing chi.Router) {
	billing.Get("/{id}/letters", rs.careLetters)
	billing.Post("/{id}/letters", rs.createCareLetters)
	billing.Post("/{id}/letters/preview", rs.previewCareLetter)
	billing.Get("/{id}/letters/print", rs.printCareLetters)
	billing.Post("/{id}/letters/mail", rs.mailCareLetters)
	billing.Get("/{id}/invoices/{invoiceID}/letter", rs.careLetter)
	billing.Post("/{id}/invoices/{invoiceID}/mail", rs.mailCareLetter)
	billing.Post("/{id}/invoices/{invoiceID}/cancellation", rs.cancelCareInvoice)
}

// CareLetterRequest is the body of POST …/letters and …/letters/preview.
type CareLetterRequest struct {
	PaymentDueOn calendar.Date `json:"payment_due_on"`
	Notes        string        `json:"notes"`
}

// Bind accepts the body; the Owner checks the date and the text.
func (req *CareLetterRequest) Bind(*http.Request) error { return nil }

// CareCancellationRequest is the body of POST …/cancellation.
type CareCancellationRequest struct {
	Reason string `json:"reason"`
}

// Bind accepts the body; the Owner checks the reason.
func (req *CareCancellationRequest) Bind(*http.Request) error { return nil }

func (rs *CareBillingResource) careLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	letters, err := rs.letters.CareLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, letters, "Monthly letters retrieved successfully")
}

func (rs *CareBillingResource) createCareLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareLetterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingLettersInvalid))
		return
	}
	letters, err := rs.letters.CreateCareLetters(r.Context(), reviewer, runID, capability.CareLetterInput(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, letters, "Monthly letters created successfully")
}

func (rs *CareBillingResource) previewCareLetter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareLetterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingLettersInvalid))
		return
	}
	file, err := rs.letters.PreviewCareLetter(r.Context(), reviewer, runID, capability.CareLetterInput(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	writeLetter(w, file, "inline")
}

func (rs *CareBillingResource) printCareLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	file, err := rs.letters.PrintCareLetters(r.Context(), reviewer, runID, r.URL.Query().Get("only"))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	writeLetter(w, file, "attachment")
}

func (rs *CareBillingResource) mailCareLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	series, err := rs.letters.MailCareLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, series, "Monthly letters queued for mail")
}

// invoiceKey reads the run, the invoice and the kind of its letter.
func invoiceKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, int64, string, bool) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, 0, "", false
	}
	invoiceID, err := common.ParseIDParam(r, "invoiceID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, "", false
	}
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = capability.CareLetterInvoice
	}
	return reviewer, runID, invoiceID, kind, true
}

func (rs *CareBillingResource) careLetter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, invoiceID, kind, ok := invoiceKey(w, r)
	if !ok {
		return
	}
	file, err := rs.letters.CareLetter(r.Context(), reviewer, runID, invoiceID, kind)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	writeLetter(w, file, "inline")
}

func (rs *CareBillingResource) mailCareLetter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, invoiceID, kind, ok := invoiceKey(w, r)
	if !ok {
		return
	}
	mailing, err := rs.letters.MailCareLetter(r.Context(), reviewer, runID, invoiceID, kind)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, mailing, "Monthly letter queued for mail")
}

// cancelCareInvoice issues the Storno. Any refusal rolls the Träger
// transaction back, so no number is used without its Storno.
func (rs *CareBillingResource) cancelCareInvoice(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, invoiceID, _, ok := invoiceKey(w, r)
	if !ok {
		return
	}
	req := &CareCancellationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingCorrectionInvalid))
		return
	}
	letters, err := rs.letters.CancelCareInvoice(r.Context(), reviewer, runID, invoiceID, capability.CareCancellationInput(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareLetterError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, letters, "Monthly invoice cancelled successfully")
}

// renderCareLetterError answers a refused letter, mail or Storno of a
// monthly run; the refusals it shares with the holiday letters keep their
// codes.
func renderCareLetterError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrCareRunNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareRunNotFound))
	case errors.Is(err, capability.ErrCareInvoiceNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareInvoiceNotFound))
	case errors.Is(err, capability.ErrCareInvoiceCancelled):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareInvoiceCancelled))
	case errors.Is(err, capability.ErrCareRecipientChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareRecipientChanged))
	default:
		renderBillingError(w, r, err)
	}
}
