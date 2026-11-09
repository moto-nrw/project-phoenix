package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareNoticeResource serves the Jahresbescheide of the monthly care (#6219,
// ADR 0182) at /traeger/care-notice-runs. Every route sits behind the Träger
// session, needs the Träger right "Abrechnung" and runs in the Träger
// transaction of the session's Träger.
type CareNoticeResource struct {
	notices        capability.CareFeeNotices
	letters        capability.CareNoticeLetterService
	requireBilling common.Middleware
}

// NewCareNoticeResource builds the resource over the notices and their
// stored originals (#6347); requireBilling admits only a session with the
// Träger right "Abrechnung".
func NewCareNoticeResource(notices capability.CareFeeNotices, letters capability.CareNoticeLetterService, requireBilling common.Middleware) *CareNoticeResource {
	if notices == nil || letters == nil || requireBilling == nil {
		panic("parent billing http: the fee notices, their letters and the right check are required")
	}
	return &CareNoticeResource{notices: notices, letters: letters, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/care-notice-runs.
func (rs *CareNoticeResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.list)
		billing.Post("/", rs.start)
		billing.Get("/{id}", rs.detail)
		billing.Post("/{id}/release", rs.release)
		billing.Get("/{id}/notices/{contractID}/letter", rs.letter)
		rs.mountLetters(billing)
	})
	return r
}

// CareNoticeRunRequest is the body of POST /traeger/care-notice-runs: the
// year, the first month it assesses, its schools (none takes every school)
// and an optional notice date.
type CareNoticeRunRequest struct {
	Year       int            `json:"year"`
	FromMonth  calendar.Date  `json:"from_month"`
	SchoolIDs  []int64        `json:"school_ids"`
	NoticeDate *calendar.Date `json:"notice_date"`
}

// Bind accepts the body; the Owner checks year and month.
func (req *CareNoticeRunRequest) Bind(*http.Request) error { return nil }

func (rs *CareNoticeResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	runs, err := rs.notices.ListCareNoticeRuns(r.Context(), organizationID)
	if err != nil {
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, runs, "Fee notice runs retrieved successfully")
}

func (rs *CareNoticeResource) start(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CareNoticeRunRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareNoticeRunInvalid))
		return
	}
	run, err := rs.notices.StartCareNoticeRun(r.Context(), reviewer, capability.CareNoticeRunStart{
		Year: req.Year, FromMonth: req.FromMonth, SchoolIDs: req.SchoolIDs, NoticeDate: req.NoticeDate})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, run, "Fee notice run started successfully")
}

func (rs *CareNoticeResource) detail(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	detail, err := rs.notices.CareNoticeRun(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Fee notice run retrieved successfully")
}

// release issues the notices. Any refusal rolls the Träger transaction
// back, so no number is used without its notice.
func (rs *CareNoticeResource) release(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareReleaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.notices.ReleaseCareNoticeRun(r.Context(), reviewer, runID, capability.CareRelease(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Fee notice run released successfully")
}

func (rs *CareNoticeResource) letter(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	contractID, err := common.ParseIDParam(r, "contractID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	letter, err := rs.notices.CareNoticeLetter(r.Context(), reviewer, runID, contractID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, letter, "Fee notice retrieved successfully")
}

// renderNoticeError answers a refused notice run; the refusals it shares
// with the monthly run keep their codes.
func renderNoticeError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareNoticeRunInvalid))
	case errors.Is(err, capability.ErrCareNoticeRunNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareNoticeRunNotFound))
	case errors.Is(err, capability.ErrCareNoticeNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareNoticeNotFound))
	case errors.Is(err, capability.ErrCareNoticeRunOpen):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareNoticeRunOpen))
	case errors.Is(err, capability.ErrBillingLetterNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingBillingLetterNotFound))
	case errors.Is(err, capability.ErrCareRecipientChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareRecipientChanged))
	case errors.Is(err, capability.ErrCareVersionsOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareVersionsOverlap))
	default:
		renderBillingConflict(w, r, err)
	}
}
