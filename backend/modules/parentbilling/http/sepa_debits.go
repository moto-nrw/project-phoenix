package parentbillinghttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SEPADebitResource serves the SEPA direct debit file of an issued run
// (#4697, ADR 0181) at /traeger/sepa-debits/{kind}/{id}: kind care-runs is
// a monthly run, holiday-runs a holiday run. Every route sits behind the
// Träger session, needs the Träger right "Abrechnung" and runs in the
// Träger transaction of the session's Träger. Only the file carries bank
// data, and nobody caches it.
type SEPADebitResource struct {
	debits         capability.SEPADebits
	requireBilling common.Middleware
}

// NewSEPADebitResource builds the resource. requireBilling admits only a
// session with the Träger right "Abrechnung".
func NewSEPADebitResource(debits capability.SEPADebits, requireBilling common.Middleware) *SEPADebitResource {
	if debits == nil || requireBilling == nil {
		panic("parent billing http: the SEPA debits and the right check are required")
	}
	return &SEPADebitResource{debits: debits, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/sepa-debits.
func (rs *SEPADebitResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/{kind}/{id}", rs.run)
		billing.Get("/{kind}/{id}/file", rs.file)
		billing.Post("/{kind}/{id}/submission", rs.submit)
	})
	return r
}

// debitRunKinds maps the path segment to the kind of run.
var debitRunKinds = map[string]string{
	"care-runs":    capability.DebitRunCare,
	"holiday-runs": capability.DebitRunHoliday,
}

// SEPADebitSubmissionRequest is the body of POST …/submission: the message
// ID of the file the office submitted to its bank.
type SEPADebitSubmissionRequest struct {
	MessageID string `json:"message_id"`
}

// Bind requires the message ID.
func (req *SEPADebitSubmissionRequest) Bind(*http.Request) error {
	if strings.TrimSpace(req.MessageID) == "" {
		return errors.New("message_id is required")
	}
	return nil
}

func debitRunKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, string, int64, bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return capability.HolidayReviewer{}, "", 0, false
	}
	kind, known := debitRunKinds[chi.URLParam(r, "kind")]
	id, err := common.ParseIDParam(r, "id")
	if !known || err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(capability.ErrSEPADebitRunNotFound,
			common.CodeParentBillingSepaDebitRunNotFound))
		return capability.HolidayReviewer{}, "", 0, false
	}
	return reviewer, kind, id, true
}

func (rs *SEPADebitResource) run(w http.ResponseWriter, r *http.Request) {
	reviewer, kind, id, ok := debitRunKey(w, r)
	if !ok {
		return
	}
	run, err := rs.debits.SEPADebitRun(r.Context(), reviewer, kind, id)
	if err != nil {
		renderDebitError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, run, "SEPA debits retrieved successfully")
}

// file answers the PAIN.008 file; it leaves the server only in this answer
// and is never stored or cached.
func (rs *SEPADebitResource) file(w http.ResponseWriter, r *http.Request) {
	reviewer, kind, id, ok := debitRunKey(w, r)
	if !ok {
		return
	}
	file, err := rs.debits.SEPADebitFile(r.Context(), reviewer, kind, id)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderDebitError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *SEPADebitResource) submit(w http.ResponseWriter, r *http.Request) {
	reviewer, kind, id, ok := debitRunKey(w, r)
	if !ok {
		return
	}
	req := &SEPADebitSubmissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	run, err := rs.debits.SubmitSEPADebits(r.Context(), reviewer, kind, id, req.MessageID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderDebitError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, run, "SEPA debits submitted successfully")
}

// renderDebitError answers a refused file; no answer names bank data.
func renderDebitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrSEPADebitRunNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingSepaDebitRunNotFound))
	case errors.Is(err, capability.ErrSEPADebitsEmpty):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaDebitsEmpty))
	case errors.Is(err, capability.ErrSEPADebitsChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaDebitsChanged))
	case errors.Is(err, capability.ErrSEPACreditorAccountMissing):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaCreditorAccountMissing))
	default:
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
	}
}
