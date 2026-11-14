package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
		billing.Get("/children", rs.children)
		billing.Get("/{id}", rs.detail)
		billing.Put("/debtor-number", rs.setDebtorNumber)
		billing.Put("/child-debtor-number", rs.setChildDebtorNumber)
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

// children lists the families with their children and numbers (#7007).
func (rs *BillingFamilyResource) children(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	families, err := rs.families.FamilyChildren(r.Context(), reviewer)
	if err != nil {
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, families, "Billing family children retrieved successfully")
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

// ChildDebtorNumberRequest is the body of PUT
// /traeger/billing-families/child-debtor-number (#7007): the child by its
// family, its names and its date of birth, and its number, empty for none.
type ChildDebtorNumberRequest struct {
	FamilyEmail    string         `json:"family_email"`
	ChildFirstName string         `json:"child_first_name"`
	ChildLastName  string         `json:"child_last_name"`
	ChildBirthDate *calendar.Date `json:"child_birth_date"`
	DebtorNumber   string         `json:"debtor_number"`
}

// Bind accepts the body; the owner checks the child and the number.
func (req *ChildDebtorNumberRequest) Bind(*http.Request) error { return nil }

// setChildDebtorNumber sets the number of a child of a family when the
// Träger keeps one per child (#7007). Any refusal rolls the Träger
// transaction back.
func (rs *BillingFamilyResource) setChildDebtorNumber(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &ChildDebtorNumberRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.families.SetChildDebtorNumber(r.Context(), reviewer, capability.ChildDebtorNumberInput{
		FamilyEmail: req.FamilyEmail, ChildFirstName: req.ChildFirstName, ChildLastName: req.ChildLastName,
		ChildBirthDate: req.ChildBirthDate, DebtorNumber: req.DebtorNumber})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, saved, "Child debtor number saved successfully")
}

// debtorNumberSettings is the format of the Träger's debtor numbers and
// whom they name.
type debtorNumberSettings struct {
	capability.DebtorNumberFormat
	Scope string `json:"scope"`
}

// debtorNumberFormat answers the format of the Träger's debtor numbers
// (#6675) and whom they name (#7007), so the list and the form of a family
// can name it.
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
	scope, err := rs.families.DebtorNumberScope(r.Context(), reviewer)
	if err != nil {
		renderFamilyError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, debtorNumberSettings{DebtorNumberFormat: format, Scope: scope},
		"Debtor number format retrieved successfully")
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
	if errors.Is(err, capability.ErrChildNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingChildNotFound))
		return
	}
	if errors.As(err, &fields) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingDebtorNumberInvalid))
		return
	}
	renderBillingError(w, r, err)
}
