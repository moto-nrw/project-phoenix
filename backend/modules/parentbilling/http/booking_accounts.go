package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The revenue accounts and cost centers of the Träger (#4821) at
// /traeger/finance-exports/booking-accounts, behind the right
// "Finanz-Export".

// RevenueAccountRequest is the body of PUT …/booking-accounts/revenue-accounts:
// the scope and its account, empty to remove it.
type RevenueAccountRequest struct {
	Scope   string `json:"scope"`
	ScopeID int64  `json:"scope_id"`
	Account string `json:"account"`
}

// Bind accepts the body; the owner validates it.
func (req *RevenueAccountRequest) Bind(*http.Request) error { return nil }

// CostCenterRequest is the body of POST …/booking-accounts/cost-centers.
type CostCenterRequest struct {
	Scope     string `json:"scope"`
	ScopeID   int64  `json:"scope_id"`
	ValidFrom string `json:"valid_from"`
	Code      string `json:"code"`
}

// Bind reads the day; the owner validates the rest.
func (req *CostCenterRequest) Bind(*http.Request) error {
	if _, err := calendar.ParseDate(req.ValidFrom); err != nil {
		return validation.Errors{"valid_from": errors.New("must be a date YYYY-MM-DD")}
	}
	return nil
}

func (rs *FinanceExportResource) bookingAccounts(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	accounts, err := rs.accounts.BookingAccounts(r.Context(), reviewer)
	if err != nil {
		renderBookingAccountError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, accounts, "Booking accounts retrieved successfully")
}

func (rs *FinanceExportResource) setRevenueAccount(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &RevenueAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accounts, err := rs.accounts.SetRevenueAccount(r.Context(), reviewer, req.Scope, req.ScopeID, req.Account)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBookingAccountError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, accounts, "Revenue account saved successfully")
}

func (rs *FinanceExportResource) addCostCenter(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CostCenterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	validFrom, _ := calendar.ParseDate(req.ValidFrom) // Bind checked it
	accounts, err := rs.accounts.AddCostCenter(r.Context(), reviewer, capability.CostCenter{
		Scope: req.Scope, ScopeID: req.ScopeID, ValidFrom: validFrom, Code: req.Code})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBookingAccountError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, accounts, "Cost center added successfully")
}

func (rs *FinanceExportResource) deleteCostCenter(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(capability.ErrCostCenterNotFound))
		return
	}
	accounts, err := rs.accounts.DeleteCostCenter(r.Context(), reviewer, id)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBookingAccountError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, accounts, "Cost center deleted successfully")
}

// renderBookingAccountError answers a refused change of the accounts. A
// wrong value or a scope or cost center of another Träger takes the code of
// its error class; the page shows the field or reloads.
func renderBookingAccountError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, capability.ErrBookingScopeNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, capability.ErrCostCenterNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, capability.ErrCostCenterDateTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCostCenterDateTaken))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
