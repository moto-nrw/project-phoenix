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

// CareBillingResource serves the monthly contribution of the care contract
// (#4684): the price list at /traeger/care-tariffs and the monthly runs at
// /traeger/care-billing-runs. Every route sits behind the Träger session,
// needs the Träger right "Abrechnung" and runs in the Träger transaction of
// the session's Träger, which comes from the session, never the request.
type CareBillingResource struct {
	billing        capability.CareBilling
	letters        capability.CareLetterService
	requireBilling common.Middleware
}

// NewCareBillingResource builds the resource with the letters of the
// released runs (#4684, ADR 0165). requireBilling admits only a session with
// the Träger right "Abrechnung".
func NewCareBillingResource(billing capability.CareBilling, letters capability.CareLetterService, requireBilling common.Middleware) *CareBillingResource {
	if billing == nil || letters == nil || requireBilling == nil {
		panic("parent billing http: the monthly billing, its letters and the right check are required")
	}
	return &CareBillingResource{billing: billing, letters: letters, requireBilling: requireBilling}
}

// TariffRouter returns the routes mounted at /traeger/care-tariffs.
func (rs *CareBillingResource) TariffRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.tariffs)
		billing.Put("/", rs.savePriceList)
		billing.Delete("/{schoolID}/{validFrom}", rs.deletePriceList)
	})
	return r
}

// RunRouter returns the routes mounted at /traeger/care-billing-runs.
func (rs *CareBillingResource) RunRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.list)
		billing.Post("/", rs.start)
		billing.Get("/{id}", rs.detail)
		billing.Post("/{id}/release", rs.release)
		rs.mountReview(billing)
		rs.mountLetters(billing)
	})
	return r
}

// CarePriceRequest is one price of PUT /traeger/care-tariffs: for a
// weekday or, with days_per_week, a scale step, with the price for every
// further child and the meal note of the module (#6196).
type CarePriceRequest struct {
	CareOfferingID int64  `json:"care_offering_id"`
	ModuleName     string `json:"module_name"`
	Weekday        string `json:"weekday"`
	DaysPerWeek    *int   `json:"days_per_week"`
	MonthlyCents   int64  `json:"monthly_cents"`
	SiblingCents   *int64 `json:"sibling_cents"`
	MealExtra      bool   `json:"meal_extra"`
}

// CarePriceListRequest is the body of PUT /traeger/care-tariffs: the
// prices of one school from the first day of a month on and the months of
// the year it bills nothing for.
type CarePriceListRequest struct {
	SchoolID   int64              `json:"school_id"`
	ValidFrom  calendar.Date      `json:"valid_from"`
	FreeMonths []int              `json:"free_months"`
	Prices     []CarePriceRequest `json:"prices"`
}

// Bind accepts the body; the Owner validates the price list.
func (req *CarePriceListRequest) Bind(*http.Request) error { return nil }

func (req *CarePriceListRequest) list() capability.CarePriceList {
	list := capability.CarePriceList{SchoolID: req.SchoolID, ValidFrom: req.ValidFrom, FreeMonths: req.FreeMonths,
		Prices: make([]capability.CarePrice, len(req.Prices))}
	for i, price := range req.Prices {
		list.Prices[i] = capability.CarePrice(price)
	}
	return list
}

// CareRunRequest is the body of POST /traeger/care-billing-runs: the first
// day of the month to bill, its schools (none bills every school without an
// open run of the month) and an optional invoice date (#5916).
type CareRunRequest struct {
	BillingMonth calendar.Date  `json:"billing_month"`
	SchoolIDs    []int64        `json:"school_ids"`
	InvoiceDate  *calendar.Date `json:"invoice_date"`
}

// Bind accepts the body; the Owner checks the month.
func (req *CareRunRequest) Bind(*http.Request) error { return nil }

func (rs *CareBillingResource) tariffs(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	tariffs, err := rs.billing.CareTariffs(r.Context(), reviewer)
	if err != nil {
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, tariffs, "Care tariffs retrieved successfully")
}

func (rs *CareBillingResource) savePriceList(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CarePriceListRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareTariffInvalid))
		return
	}
	tariffs, err := rs.billing.SaveCarePriceList(r.Context(), reviewer, req.list())
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, tariffs, "Care price list saved successfully")
}

func (rs *CareBillingResource) deletePriceList(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	schoolID, err := common.ParseIDParam(r, "schoolID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	validFrom, err := calendar.ParseDate(chi.URLParam(r, "validFrom"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareTariffInvalid))
		return
	}
	tariffs, err := rs.billing.DeleteCarePriceList(r.Context(), reviewer, schoolID, validFrom)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, tariffs, "Care price list deleted successfully")
}

func (rs *CareBillingResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	runs, err := rs.billing.ListCareRuns(r.Context(), organizationID)
	if err != nil {
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, runs, "Monthly billing runs retrieved successfully")
}

func (rs *CareBillingResource) start(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CareRunRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareRunInvalid))
		return
	}
	run, err := rs.billing.StartCareRun(r.Context(), reviewer,
		capability.CareRunStart{BillingMonth: req.BillingMonth, SchoolIDs: req.SchoolIDs, InvoiceDate: req.InvoiceDate})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, run, "Monthly billing run started successfully")
}

func (rs *CareBillingResource) detail(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	detail, err := rs.billing.CareRun(r.Context(), reviewer, runID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Monthly billing run retrieved successfully")
}

// release issues the invoices. Any refusal rolls the Träger transaction
// back, so no number is used without its invoice.
func (rs *CareBillingResource) release(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareReleaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.billing.ReleaseCareRun(r.Context(), reviewer, runID, capability.CareRelease(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Monthly billing run released successfully")
}

// renderCareError answers a refused price list or monthly run; the refusals
// it shares with the holiday run keep their codes.
func renderCareError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields) && (fields["billing_month"] != nil || fields["school_ids"] != nil ||
		fields["invoice_date"] != nil || fields["reason"] != nil || fields["format"] != nil):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareRunInvalid))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareTariffInvalid))
	case errors.Is(err, capability.ErrCareRunNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareRunNotFound))
	case errors.Is(err, capability.ErrCareTariffNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareTariffNotFound))
	case errors.Is(err, capability.ErrCareExclusionConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareExclusionConflict))
	case errors.Is(err, capability.ErrCareRunOpen):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareRunOpen))
	case errors.Is(err, capability.ErrCareVersionsOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareVersionsOverlap))
	case errors.Is(err, capability.ErrCareTariffMonthBilled):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareTariffMonthBilled))
	default:
		renderBillingConflict(w, r, err)
	}
}
