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

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// HolidayResource serves the holiday contribution (#4664, ADR 0115): in the
// Träger-Portal at /traeger/holiday-contributions the contribution of a
// registration and its confirmation, which freezes it; publicly at
// /traeger/holiday-quotes the contribution of the registration form before
// it is sent.
type HolidayResource struct {
	contributions capability.HolidayContributions
	requireManage common.Middleware
}

// NewHolidayResource builds the resource. requireManage admits only a
// session with the Träger right "Anmeldung verwalten", the right that
// confirms a registration.
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
		manage.Get("/{periodId}/registrations/{registrationId}", rs.contribution)
		manage.Post("/{periodId}/registrations/{registrationId}/confirm", rs.confirm)
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
// an empty body confirms without it.
type HolidayConfirmRequest struct {
	BenefitsExemption bool `json:"benefits_exemption"`
}

// Bind accepts the body.
func (req *HolidayConfirmRequest) Bind(*http.Request) error { return nil }

type holidayQuoteBookingRequest struct {
	WeekID      int64 `json:"week_id"`
	TimeModelID int64 `json:"time_model_id"`
}

type holidayQuoteChildRequest struct {
	Bookings []holidayQuoteBookingRequest `json:"bookings"`
}

// HolidayQuoteRequest is the body of POST /traeger/holiday-quotes/{token}:
// the bookings of every child of the form and the parents' statement on
// benefits. It carries no personal data.
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
		children[i] = capability.HolidayQuoteChild{Bookings: bookings}
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

func (rs *HolidayResource) contribution(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	var exemption *bool
	if raw := r.URL.Query().Get("benefits_exemption"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("benefits_exemption must be true or false: %w", err)))
			return
		}
		exemption = &parsed
	}
	contribution, err := rs.contributions.HolidayContribution(r.Context(), reviewer, periodID, registrationID, exemption)
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
	confirmation, err := rs.contributions.ConfirmHolidayRegistration(r.Context(), reviewer, periodID, registrationID, req.BenefitsExemption)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, confirmation, "Holiday registration confirmed successfully")
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
	switch {
	case errors.Is(err, capability.ErrHolidayLinkNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationNotFound))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationEntryNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationStateConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationStateConflict))
	case errors.Is(err, capability.ErrBenefitsExemptionUnavailable):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingBenefitsExemptionUnavailable))
	case errors.Is(err, capability.ErrInvalidHolidayQuote):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingHolidayQuoteInvalid))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
