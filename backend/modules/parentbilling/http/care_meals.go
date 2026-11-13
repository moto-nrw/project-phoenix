package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// CareMealTariffResource serves the Mensa prices of the Träger (#4686) at
// /traeger/care-meal-tariffs: how each school's Mensa is billed from a month
// on. Every route needs the Träger right "Abrechnung" and runs in the
// Träger transaction of the session's Träger.
type CareMealTariffResource struct {
	billing        capability.CareMealBilling
	requireBilling common.Middleware
}

// NewCareMealTariffResource builds the resource.
func NewCareMealTariffResource(billing capability.CareMealBilling, requireBilling common.Middleware) *CareMealTariffResource {
	if billing == nil || requireBilling == nil {
		panic("parent billing http: the Mensa prices and the right check are required")
	}
	return &CareMealTariffResource{billing: billing, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/care-meal-tariffs.
func (rs *CareMealTariffResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.tariffs)
		billing.Put("/", rs.save)
		billing.Delete("/{schoolID}/{validFrom}", rs.delete)
	})
	return r
}

// CareMealPriceRequest is the body of PUT /traeger/care-meal-tariffs.
type CareMealPriceRequest struct {
	SchoolID    int64         `json:"school_id"`
	ValidFrom   calendar.Date `json:"valid_from"`
	Mode        string        `json:"mode"`
	AmountCents int64         `json:"amount_cents"`
}

// Bind accepts the body; the Owner validates the price.
func (req *CareMealPriceRequest) Bind(*http.Request) error { return nil }

func (rs *CareMealTariffResource) tariffs(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	tariffs, err := rs.billing.CareMealTariffs(r.Context(), reviewer)
	if err != nil {
		renderCareMealError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, tariffs, "Mensa prices retrieved successfully")
}

func (rs *CareMealTariffResource) save(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CareMealPriceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareMealTariffInvalid))
		return
	}
	tariffs, err := rs.billing.SaveCareMealTariff(r.Context(), reviewer, capability.CareMealPrice(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareMealError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, tariffs, "Mensa price saved successfully")
}

func (rs *CareMealTariffResource) delete(w http.ResponseWriter, r *http.Request) {
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
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareMealTariffInvalid))
		return
	}
	tariffs, err := rs.billing.DeleteCareMealTariff(r.Context(), reviewer, schoolID, validFrom)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareMealError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, tariffs, "Mensa price deleted successfully")
}

// renderCareMealError answers a refused Mensa price.
func renderCareMealError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrCareMealTariffInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareMealTariffInvalid))
	case errors.Is(err, capability.ErrCareMealTariffNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCareMealTariffNotFound))
	case errors.Is(err, capability.ErrCareMealTariffMonthBilled):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCareMealTariffMonthBilled))
	default:
		renderCareError(w, r, err)
	}
}
