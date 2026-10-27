package parentbillinghttp

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BillingRunResource serves the billing runs of a Träger at
// /traeger/billing-runs and its number ranges at /traeger/number-ranges
// (#4668, ADR 0119). Every route sits behind the Träger session, needs the
// Träger right "Abrechnung" and runs in the Träger transaction of the
// session's Träger. The Träger comes from the session, never from the
// request. The confirmation letters of a released run sit below the run
// (#4669, ADR 0121), with their mail and their print (#4670, ADR 0123).
type BillingRunResource struct {
	runs           capability.BillingRuns
	letters        capability.BillingLetterService
	mail           capability.BillingLetterMailService
	requireBilling common.Middleware
}

// NewBillingRunResource builds the resource. requireBilling admits only a
// session with the Träger right "Abrechnung".
func NewBillingRunResource(runs capability.BillingRuns, letters capability.BillingLetterService, mail capability.BillingLetterMailService,
	requireBilling common.Middleware) *BillingRunResource {
	if runs == nil || letters == nil || mail == nil || requireBilling == nil {
		panic("parent billing http: the billing runs, the letters, their mail and the right check are required")
	}
	return &BillingRunResource{runs: runs, letters: letters, mail: mail, requireBilling: requireBilling}
}

// RunRouter returns the routes mounted at /traeger/billing-runs.
func (rs *BillingRunResource) RunRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.list)
		billing.Post("/", rs.start)
		billing.Get("/periods", rs.periods)
		billing.Get("/{id}", rs.detail)
		billing.Post("/{id}/corrections", rs.correct)
		billing.Post("/{id}/release", rs.release)
		billing.Get("/{id}/export", rs.export)
		billing.Post("/{id}/letters", rs.createLetters)
		billing.Post("/{id}/letters/preview", rs.previewLetter)
		billing.Get("/{id}/documents/{documentID}/letter", rs.letter)
		billing.Get("/{id}/letters/print", rs.printLetters)
		billing.Get("/{id}/mailings", rs.mailings)
		billing.Post("/{id}/mailings", rs.mailAll)
		billing.Post("/{id}/documents/{documentID}/mailings", rs.mailOne)
	})
	return r
}

// NumberRangeRouter returns the routes mounted at /traeger/number-ranges.
func (rs *BillingRunResource) NumberRangeRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.numberRanges)
		billing.Put("/{service}", rs.setNumberRange)
	})
	return r
}

// BillingRunRequest is the body of POST /traeger/billing-runs.
type BillingRunRequest struct {
	HolidayPeriodID int64 `json:"holiday_period_id"`
}

// Bind accepts the body; the Owner checks the period.
func (req *BillingRunRequest) Bind(*http.Request) error { return nil }

// BillingCorrectionRequest is the body of POST
// /traeger/billing-runs/{id}/corrections: the new amount of one child in
// whole cent and the reason.
type BillingCorrectionRequest struct {
	RegistrationID int64  `json:"registration_id"`
	ChildID        int64  `json:"child_id"`
	TotalCents     int64  `json:"total_cents"`
	Reason         string `json:"reason"`
}

// Bind accepts the body; the Owner validates the correction.
func (req *BillingCorrectionRequest) Bind(*http.Request) error { return nil }

// BillingReleaseRequest is the body of POST
// /traeger/billing-runs/{id}/release: the number of invoices and the sum of
// the preview the Abrechnung checked.
type BillingReleaseRequest struct {
	DocumentCount int   `json:"document_count"`
	TotalCents    int64 `json:"total_cents"`
}

// Bind accepts the body.
func (req *BillingReleaseRequest) Bind(*http.Request) error { return nil }

// NumberRangeRequest is the body of PUT /traeger/number-ranges/{service}.
type NumberRangeRequest struct {
	Pattern string `json:"pattern"`
}

// Bind accepts the body; the Owner validates the pattern.
func (req *NumberRangeRequest) Bind(*http.Request) error { return nil }

func (rs *BillingRunResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	runs, err := rs.runs.ListBillingRuns(r.Context(), organizationID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, runs, "Billing runs retrieved successfully")
}

func (rs *BillingRunResource) periods(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periods, err := rs.runs.ListBillablePeriods(r.Context(), reviewer)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, periods, "Billable holiday periods retrieved successfully")
}

func (rs *BillingRunResource) start(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &BillingRunRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	run, err := rs.runs.StartHolidayBillingRun(r.Context(), reviewer, req.HolidayPeriodID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, run, "Billing run started successfully")
}

// runKey reads the session and the run ID.
func runKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, false
	}
	runID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, false
	}
	return reviewer, runID, true
}

func (rs *BillingRunResource) detail(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	detail, err := rs.runs.BillingRun(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Billing run retrieved successfully")
}

func (rs *BillingRunResource) correct(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &BillingCorrectionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.runs.CorrectBillingRun(r.Context(), reviewer, runID, capability.BillingCorrectionInput(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, detail, "Billing correction recorded successfully")
}

// release issues the invoices. Any refusal rolls the Träger transaction
// back, so no number is used without its invoice.
func (rs *BillingRunResource) release(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &BillingReleaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.runs.ReleaseBillingRun(r.Context(), reviewer, runID, capability.BillingRelease(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Billing run released successfully")
}

// export answers GET /traeger/billing-runs/{id}/export?format=pdf|xlsx|csv
// with the overview of a released run for the Kämmerei (#4672). A failed
// record of the download rolls back.
func (rs *BillingRunResource) export(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	file, err := rs.runs.ExportBillingRun(r.Context(), reviewer, runID, format)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBillingError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *BillingRunResource) numberRanges(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	ranges, err := rs.runs.ListNumberRanges(r.Context(), organizationID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ranges, "Number ranges retrieved successfully")
}

func (rs *BillingRunResource) setNumberRange(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	req := &NumberRangeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	numberRange, err := rs.runs.SetNumberRange(r.Context(), organizationID, chi.URLParam(r, "service"), req.Pattern)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, numberRange, "Number range saved successfully")
}

// renderBillingError answers a refused run or number range. The period
// refusals keep the codes of the holiday periods.
func renderBillingError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields) && fields["format"] != nil:
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, invalidFieldsCode(fields)))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.Is(err, capability.ErrBillingRunNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingBillingRunNotFound))
	case errors.Is(err, capability.ErrBillingFamilyNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingBillingFamilyNotFound))
	case errors.Is(err, capability.ErrBillingLetterNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingBillingLetterNotFound))
	default:
		renderBillingConflict(w, r, err)
	}
}

// invalidFieldsCode names the refused input by its fields: a number range,
// the letters or a correction.
func invalidFieldsCode(fields validation.Errors) string {
	for _, field := range []struct{ name, code string }{
		{"pattern", common.CodeParentBillingNumberRangeInvalid},
		{"service", common.CodeParentBillingNumberRangeInvalid},
		{"payment_due_on", common.CodeParentBillingBillingLettersInvalid},
		{"notes", common.CodeParentBillingBillingLettersInvalid},
		{"emergency_contacts", common.CodeParentBillingBillingLettersInvalid},
	} {
		if _, ok := fields[field.name]; ok {
			return field.code
		}
	}
	return common.CodeParentBillingBillingCorrectionInvalid
}

func renderBillingConflict(w http.ResponseWriter, r *http.Request, err error) {
	for _, conflict := range []struct {
		err  error
		code string
	}{
		{capability.ErrBillingRunOpen, common.CodeParentBillingBillingRunOpen},
		{capability.ErrBillingRunReleased, common.CodeParentBillingBillingRunReleased},
		{capability.ErrBillingRunNotReleased, common.CodeParentBillingBillingRunNotReleased},
		{capability.ErrBillingRunChanged, common.CodeParentBillingBillingRunChanged},
		{capability.ErrBillingRunEmpty, common.CodeParentBillingBillingRunEmpty},
		{capability.ErrNumberRangeMissing, common.CodeParentBillingNumberRangeMissing},
		{capability.ErrInvoiceNumberTaken, common.CodeParentBillingInvoiceNumberTaken},
		{capability.ErrBillingLettersExist, common.CodeParentBillingBillingLettersExist},
		{capability.ErrBankAccountMissing, common.CodeParentBillingBankAccountMissing},
		{capability.ErrLetterheadTooLong, common.CodeParentBillingLetterheadTooLong},
		{capability.ErrBillingLettersMissing, common.CodeParentBillingBillingLettersMissing},
		{capability.ErrLetterMailPending, common.CodeParentBillingLetterMailPending},
		{capability.ErrRecipientEmailMissing, common.CodeParentBillingRecipientEmailMissing},
	} {
		if errors.Is(err, conflict.err) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, conflict.code))
			return
		}
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}
