package parentbillinghttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// HolidayResource serves the holiday contribution (#4664, ADR 0115): in the
// Träger-Portal at /traeger/holiday-contributions the contribution of a
// registration, its confirmation, which freezes it, and its cancellation,
// which computes the rest of its sibling group again (#5662); publicly at
// /traeger/holiday-quotes the contribution of the registration form before
// it is sent.
type HolidayResource struct {
	contributions capability.HolidayContributions
	requireManage common.Middleware
}

// NewHolidayResource builds the resource. requireManage admits only a
// session with the Träger right "Anmeldung verwalten", the right that
// confirms a registration and merges siblings (#5628).
func NewHolidayResource(contributions capability.HolidayContributions, requireManage common.Middleware) *HolidayResource {
	if contributions == nil || requireManage == nil {
		panic("parent billing http: the holiday contributions and the right check are required")
	}
	return &HolidayResource{contributions: contributions, requireManage: requireManage}
}

// TraegerRouter returns the routes mounted at /traeger/holiday-contributions.
// They sit behind the Träger session and run in its Träger transaction.
func (rs *HolidayResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		manage := r.With(rs.requireManage, withTx)
		manage.Get("/confirmation-settings", rs.confirmationSettings)
		manage.Get("/{periodId}/registrations/{registrationId}", rs.contribution)
		manage.Post("/{periodId}/registrations/{registrationId}/confirm", rs.confirm)
		manage.Post("/{periodId}/registrations/{registrationId}/cancel", rs.cancel)
		manage.Put("/{periodId}/registrations/{registrationId}/children/{childId}/bookings", rs.rebookChild)
		manage.Put("/{periodId}/weeks/{weekId}/capacity", rs.setWeekCapacity)
		manage.Put("/{periodId}/registrations/{registrationId}/debtor-number", rs.changeDebtorNumber)
		manage.Get("/{periodId}/sibling-groups", rs.listSiblingGroups)
		manage.Post("/{periodId}/sibling-groups", rs.mergeSiblings)
		manage.Delete("/{periodId}/sibling-groups/registrations/{registrationId}", rs.splitSibling)
	})
	return r
}

// QuoteRouter returns the public route mounted at /traeger/holiday-quotes.
// There is no session; the link names the Träger.
func (rs *HolidayResource) QuoteRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post("/{token}", rs.quote)
	return r
}

// HolidayConfirmRequest is the body of POST
// /traeger/holiday-contributions/{periodId}/registrations/{registrationId}/confirm.
// benefits_exemption applies the exemption for families receiving benefits;
// cost_bearer_id lets that cost bearer of the Träger pay instead (#5627);
// debtor_number is the number of the family in the finance system of the
// Träger (#5621). An empty body confirms without any of them.
type HolidayConfirmRequest struct {
	BenefitsExemption bool   `json:"benefits_exemption"`
	CostBearerID      int64  `json:"cost_bearer_id"`
	DebtorNumber      string `json:"debtor_number"`
}

// Bind accepts the body; the owners check the debtor number.
func (req *HolidayConfirmRequest) Bind(*http.Request) error { return nil }

// HolidayDebtorNumberRequest is the body of PUT
// /traeger/holiday-contributions/{periodId}/registrations/{registrationId}/debtor-number
// (#5621): the new debtor number; an empty one removes it where the Träger
// does not ask for it.
type HolidayDebtorNumberRequest struct {
	DebtorNumber string `json:"debtor_number"`
}

// Bind accepts the body; the owners check the debtor number.
func (req *HolidayDebtorNumberRequest) Bind(*http.Request) error { return nil }

type holidayQuoteBookingRequest struct {
	WeekID      int64           `json:"week_id"`
	TimeModelID int64           `json:"time_model_id"`
	Days        []calendar.Date `json:"days"`
}

type holidayQuoteChildRequest struct {
	Bookings []holidayQuoteBookingRequest `json:"bookings"`
	AgeRank  int                          `json:"age_rank"`
}

// HolidayQuoteRequest is the body of POST /traeger/holiday-quotes/{token}:
// the bookings of every child of the form, the order of the children by age
// and the parents' statement on benefits. It carries no personal data: the
// form sends the order, not the date of birth (#5664).
type HolidayQuoteRequest struct {
	Children         []holidayQuoteChildRequest `json:"children"`
	BenefitsDeclared bool                       `json:"benefits_declared"`
}

// Bind accepts the body; the Owner checks the bookings against the period.
func (req *HolidayQuoteRequest) Bind(*http.Request) error { return nil }

func (req *HolidayQuoteRequest) children() []capability.HolidayQuoteChild {
	children := make([]capability.HolidayQuoteChild, len(req.Children))
	for i, child := range req.Children {
		bookings := make([]capability.HolidayQuoteBooking, len(child.Bookings))
		for j, booking := range child.Bookings {
			bookings[j] = capability.HolidayQuoteBooking(booking)
		}
		children[i] = capability.HolidayQuoteChild{Bookings: bookings, AgeRank: child.AgeRank}
	}
	return children
}

// reviewerOf returns the Träger and the account of the session, or renders
// 401.
func reviewerOf(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return capability.HolidayReviewer{}, false
	}
	return capability.HolidayReviewer{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

// registrationKey reads the session and the period and registration IDs.
func registrationKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, int64, bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, 0, false
	}
	periodID, err := common.ParseIDParam(r, "periodId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	registrationID, err := common.ParseIDParam(r, "registrationId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	return reviewer, periodID, registrationID, true
}

// contribution answers the frozen contribution or the preview with the
// choice the query names: benefits_exemption=true|false and
// cost_bearer_id=<id> (#5627).
func (rs *HolidayResource) contribution(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	exemption, costBearerID, err := previewChoice(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	contribution, err := rs.contributions.HolidayContribution(r.Context(), reviewer, periodID, registrationID, exemption, costBearerID)
	if err != nil {
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, contribution, "Holiday contribution retrieved successfully")
}

// confirm confirms the registration and freezes its contribution. Any
// refusal rolls the Träger transaction back, so no registration is confirmed
// without its frozen contribution.
func (rs *HolidayResource) confirm(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	req := &HolidayConfirmRequest{}
	if err := render.Bind(r, req); err != nil && !errors.Is(err, io.EOF) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	confirmation, err := rs.contributions.ConfirmHolidayRegistration(r.Context(), reviewer, periodID, registrationID,
		req.BenefitsExemption, req.CostBearerID, req.DebtorNumber)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, confirmation, "Holiday registration confirmed successfully")
}

// cancel cancels the registration and computes the rest of its sibling
// group again (#5662). Any refusal rolls the Träger transaction back, so no
// registration is cancelled while its group still ranks it.
func (rs *HolidayResource) cancel(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	cancellation, err := rs.contributions.CancelHolidayRegistration(r.Context(), reviewer, periodID, registrationID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, cancellation, "Holiday registration cancelled successfully")
}

// HolidayRebookingRequest is the body of a rebooking (#4699): the new
// bookings of the child, one time model per week.
type HolidayRebookingRequest struct {
	Bookings []capability.HolidayBookingChoice `json:"bookings"`
}

// Bind accepts the body; Enrollment validates the bookings.
func (req *HolidayRebookingRequest) Bind(*http.Request) error { return nil }

// rebookChild rebooks a child (#4699) and answers with the registration as
// the review shows it. A week it leaves frees its place for the waiting
// list; whoever moves up gets the contribution again. Any refusal rolls the
// Träger transaction back.
func (rs *HolidayResource) rebookChild(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	childID, err := common.ParseIDParam(r, "childId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayRebookingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	change, err := rs.contributions.RebookHolidayChild(r.Context(), reviewer, periodID, registrationID, childID, req.Bookings)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, change.Registration, "Holiday registration rebooked successfully")
}

// HolidayCapacityRequest is the body of a new capacity of a week (#4699);
// a null capacity is unlimited.
type HolidayCapacityRequest struct {
	Capacity *int `json:"capacity"`
}

// Bind accepts the body; Enrollment validates the capacity.
func (req *HolidayCapacityRequest) Bind(*http.Request) error { return nil }

// setWeekCapacity sets the places of a week and moves bookings up into the
// places it frees (#4699).
func (rs *HolidayResource) setWeekCapacity(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "periodId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	weekID, err := common.ParseIDParam(r, "weekId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayCapacityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	change, err := rs.contributions.SetHolidayWeekCapacity(r.Context(), reviewer, periodID, weekID, req.Capacity)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, change, "Holiday week capacity changed successfully")
}

// previewChoice reads the exemption and the cost bearer of a preview from
// the query; both are optional.
func previewChoice(r *http.Request) (*bool, int64, error) {
	var exemption *bool
	if raw := r.URL.Query().Get("benefits_exemption"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("benefits_exemption must be true or false: %w", err)
		}
		exemption = &parsed
	}
	var costBearerID int64
	if raw := r.URL.Query().Get("cost_bearer_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, 0, errors.New("cost_bearer_id must be a positive ID")
		}
		costBearerID = parsed
	}
	return exemption, costBearerID, nil
}

// confirmationSettings answers what a confirmation of the session's Träger
// needs (#5621).
func (rs *HolidayResource) confirmationSettings(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	settings, err := rs.contributions.HolidayConfirmationSettings(r.Context(), reviewer)
	if err != nil {
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, settings, "Holiday confirmation settings retrieved successfully")
}

// changeDebtorNumber changes the debtor number of a confirmed registration
// until an issued invoice bills it (#5621). Any refusal rolls the Träger
// transaction back.
func (rs *HolidayResource) changeDebtorNumber(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	req := &HolidayDebtorNumberRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	changed, err := rs.contributions.ChangeHolidayDebtorNumber(r.Context(), reviewer, periodID, registrationID, req.DebtorNumber)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, changed, "Holiday debtor number changed successfully")
}

// quote resolves the link in the administrative transaction and computes
// the contribution in the Träger transaction of the link's Träger. It stores
// nothing.
func (rs *HolidayResource) quote(w http.ResponseWriter, r *http.Request) {
	req := &HolidayQuoteRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	link, err := rs.resolveLink(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		renderHolidayError(w, r, err)
		return
	}
	var contribution capability.HolidayContribution
	err = tenant.WithinTraeger(r.Context(), link.OrganizationID, func(txCtx context.Context) error {
		quoted, quoteErr := rs.contributions.QuoteHolidayContribution(txCtx, link, req.children(), req.BenefitsDeclared)
		contribution = quoted
		return quoteErr
	})
	if err != nil {
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, contribution, "Holiday contribution computed")
}

// resolveLink resolves the public link in the administrative transaction,
// before any Träger context exists, like the registration form itself.
func (rs *HolidayResource) resolveLink(ctx context.Context, token string) (capability.HolidayLink, error) {
	var link capability.HolidayLink
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		resolved, err := rs.contributions.ResolveHolidayLink(adminCtx, token)
		link = resolved
		return err
	})
	return link, err
}

// renderHolidayError answers a refused contribution. The registration
// refusals keep the codes of the registration review (ADR 0114).
func renderHolidayError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	var dayLimit *capability.HolidayChildDayLimitRefusal
	switch {
	case errors.As(err, &dayLimit):
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentHolidayChildDayLimit,
			map[string]any{"limit": dayLimit.Limit, "days": dayLimit.Days, "child": 1}))
	case errors.Is(err, capability.ErrInvalidHolidayPlaces):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayRegistrationInvalid))
	case errors.Is(err, capability.ErrHolidayWeekNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayWeekNotFound))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingDebtorNumberInvalid))
	case errors.Is(err, capability.ErrDebtorNumberLocked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingDebtorNumberLocked))
	case errors.Is(err, capability.ErrHolidayLinkNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationNotFound))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationEntryNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationStateConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationStateConflict))
	case errors.Is(err, capability.ErrHolidayRegistrationFrozen):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationFrozen))
	case errors.Is(err, capability.ErrBenefitsExemptionUnavailable):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingBenefitsExemptionUnavailable))
	case errors.Is(err, capability.ErrCostTakeoverWithExemption):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCostTakeoverWithExemption))
	case errors.Is(err, capability.ErrCostBearerNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCostBearerNotFound))
	case errors.Is(err, capability.ErrCostBearerInactive):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCostBearerInactive))
	case errors.Is(err, capability.ErrInvalidHolidaySiblingGroup):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSiblingGroupInvalid))
	case errors.Is(err, capability.ErrHolidaySiblingGroupNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingSiblingGroupNotFound))
	case errors.Is(err, capability.ErrHolidaySiblingsInvoiced):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSiblingGroupInvoiced))
	case errors.Is(err, capability.ErrInvalidHolidayQuote):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingHolidayQuoteInvalid))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
