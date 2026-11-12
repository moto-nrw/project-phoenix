package parentbillinghttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// The stored originals of the Jahresbescheide and their delivery (#6347)
// under /traeger/care-notice-runs/{id}. They sit behind the same right
// "Abrechnung" and Träger transaction as the run. A notice is named by its
// contract, as page 1 of the Bescheid is. Sending answers 202: the mail
// waits on Delivery's outbox, and the run shows its state.

// mountLetters adds the routes of the originals of a released run.
func (rs *CareNoticeResource) mountLetters(billing chi.Router) {
	billing.Get("/{id}/letters", rs.noticeLetters)
	billing.Post("/{id}/letters", rs.createNoticeLetters)
	billing.Get("/{id}/letters/print", rs.printNoticeLetters)
	billing.Post("/{id}/letters/mail", rs.mailNoticeLetters)
	billing.Get("/{id}/notices/{contractID}/pdf", rs.noticePDF)
	billing.Post("/{id}/notices/{contractID}/mail", rs.mailNoticeLetter)
}

func (rs *CareNoticeResource) noticeLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	letters, err := rs.letters.CareNoticeLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, letters, "Fee notice letters retrieved successfully")
}

func (rs *CareNoticeResource) createNoticeLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	letters, err := rs.letters.CreateCareNoticeLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, letters, "Fee notice letters stored successfully")
}

func (rs *CareNoticeResource) printNoticeLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	file, err := rs.letters.PrintCareNoticeLetters(r.Context(), reviewer, runID, r.URL.Query().Get("only"))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	writeLetter(w, file, "attachment")
}

func (rs *CareNoticeResource) mailNoticeLetters(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	series, err := rs.letters.MailCareNoticeLetters(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, series, "Fee notices queued for mail")
}

// noticeKey reads the run and the contract of its notice.
func noticeKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, int64, bool) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, 0, false
	}
	contractID, err := common.ParseIDParam(r, "contractID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	return reviewer, runID, contractID, true
}

func (rs *CareNoticeResource) noticePDF(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, contractID, ok := noticeKey(w, r)
	if !ok {
		return
	}
	file, err := rs.letters.CareNoticePDF(r.Context(), reviewer, runID, contractID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	writeLetter(w, file, "inline")
}

func (rs *CareNoticeResource) mailNoticeLetter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, contractID, ok := noticeKey(w, r)
	if !ok {
		return
	}
	mailing, err := rs.letters.MailCareNoticeLetter(r.Context(), reviewer, runID, contractID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, mailing, "Fee notice queued for mail")
}
