package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/tenant"
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
		billing.Put("/debtor-number", rs.setDebtorNumber)
		billing.Get("/debtor-number-format", rs.debtorNumberFormat)
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

// FamilyDebtorNumberRequest is the body of PUT
// /traeger/billing-families/debtor-number (#6692): the family by its key and
// its number, empty for none.
type FamilyDebtorNumberRequest struct {
	FamilyEmail  string `json:"family_email"`
	DebtorNumber string `json:"debtor_number"`
}

// Bind accepts the body; the owner checks the family and the number.
func (req *FamilyDebtorNumberRequest) Bind(*http.Request) error { return nil }

// setDebtorNumber sets the number of a family, also of one without a care
// contract (#6692). Any refusal rolls the Träger transaction back.
func (rs *BillingFamilyResource) setDebtorNumber(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &FamilyDebtorNumberRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.families.SetFamilyDebtorNumber(r.Context(), reviewer,
		capability.FamilyDebtorNumberInput{FamilyEmail: req.FamilyEmail, DebtorNumber: req.DebtorNumber})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, saved, "Family debtor number saved successfully")
}

// debtorNumberFormat answers the format of the Träger's debtor numbers
// (#6675), so the list and the form of a family can name it.
func (rs *BillingFamilyResource) debtorNumberFormat(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	format, err := rs.families.DebtorNumberFormat(r.Context(), reviewer)
	if err != nil {
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, format, "Debtor number format retrieved successfully")
}

func renderFamilyError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	if renderDebtorNumberFormat(w, r, err) {
		return
	}
	if errors.Is(err, capability.ErrFamilyNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingFamilyNotFound))
		return
	}
	if errors.As(err, &fields) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingDebtorNumberInvalid))
		return
	}
	renderBillingError(w, r, err)
}
