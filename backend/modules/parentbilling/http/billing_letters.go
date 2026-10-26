package parentbillinghttp

import (
	"fmt"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BillingLettersRequest is the body of POST
// /traeger/billing-runs/{id}/letters and of its preview: the payment date
// (YYYY-MM-DD), the free text for the parents and the emergency numbers of
// the hosting OGS.
type BillingLettersRequest struct {
	PaymentDueOn      calendar.Date                        `json:"payment_due_on"`
	Notes             string                               `json:"notes"`
	EmergencyContacts []capability.BillingEmergencyContact `json:"emergency_contacts"`
}

// Bind accepts the body; the Owner validates the letters.
func (req *BillingLettersRequest) Bind(*http.Request) error { return nil }

func (req *BillingLettersRequest) input() capability.BillingLetterInput {
	return capability.BillingLetterInput{
		PaymentDueOn: req.PaymentDueOn, Notes: req.Notes, EmergencyContacts: req.EmergencyContacts,
	}
}

// lettersRequest reads the session, the run and the body of the letters.
func lettersRequest(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, capability.BillingLetterInput, bool) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, capability.BillingLetterInput{}, false
	}
	req := &BillingLettersRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingBillingLettersInvalid))
		return capability.HolidayReviewer{}, 0, capability.BillingLetterInput{}, false
	}
	return reviewer, runID, req.input(), true
}

// createLetters renders and stores the letter of every invoice of the run.
// Any refusal rolls the Träger transaction back, so no run keeps half of
// its letters.
func (rs *BillingRunResource) createLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, input, ok := lettersRequest(w, r)
	if !ok {
		return
	}
	letters, err := rs.letters.CreateBillingLetters(r.Context(), reviewer, runID, input)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, letters, "Billing letters created successfully")
}

// previewLetter answers the letter of the first invoice as a PDF without
// storing it.
func (rs *BillingRunResource) previewLetter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, input, ok := lettersRequest(w, r)
	if !ok {
		return
	}
	file, err := rs.letters.PreviewBillingLetter(r.Context(), reviewer, runID, input)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	writeLetter(w, file, "inline")
}

// letter answers the stored letter of an invoice of the run.
func (rs *BillingRunResource) letter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	documentID, err := common.ParseIDParam(r, "documentID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	file, err := rs.letters.BillingLetter(r.Context(), reviewer, runID, documentID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	writeLetter(w, file, "attachment")
}

// writeLetter answers a PDF that no cache keeps.
func writeLetter(w http.ResponseWriter, file capability.BillingLetterFile, disposition string) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
