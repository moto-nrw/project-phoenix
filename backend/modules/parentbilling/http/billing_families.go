package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// BillingFamilyResource serves the invoices and amounts per family of a
// Träger at /traeger/billing-families (#4691). Every route sits behind the
// Träger session, needs the Träger right "Abrechnung" and runs in the
// Träger transaction of the session's Träger, which comes from the session,
// never from the request.
type BillingFamilyResource struct {
	families       capability.BillingFamilies
	requireBilling common.Middleware
}

// NewBillingFamilyResource builds the resource. requireBilling admits only
// a session with the Träger right "Abrechnung".
func NewBillingFamilyResource(families capability.BillingFamilies, requireBilling common.Middleware) *BillingFamilyResource {
	if families == nil || requireBilling == nil {
		panic("parent billing http: the families and the right check are required")
	}
	return &BillingFamilyResource{families: families, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/billing-families.
func (rs *BillingFamilyResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.list)
		billing.Get("/{id}", rs.detail)
	})
	return r
}

func (rs *BillingFamilyResource) list(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	families, err := rs.families.ListBillingFamilies(r.Context(), reviewer)
	if err != nil {
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, families, "Billing families retrieved successfully")
}

func (rs *BillingFamilyResource) detail(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	familyID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	family, err := rs.families.BillingFamily(r.Context(), reviewer, familyID)
	if err != nil {
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, family, "Billing family retrieved successfully")
}

func renderFamilyError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, capability.ErrFamilyNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingFamilyNotFound))
		return
	}
	renderBillingError(w, r, err)
}
