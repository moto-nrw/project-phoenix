package enrollmenthttp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// HolidayPeriodResource serves the holiday periods of a Träger in the
// Träger-Portal (#4659). Every route sits behind the Träger session, needs
// the Träger right "Anmeldung verwalten" and runs in the Träger transaction
// of the session's Träger (ADR 0105). The Träger comes from the session,
// never from the request.
type HolidayPeriodResource struct {
	periods       capability.HolidayPeriods
	review        capability.HolidayRegistrationReview
	schoolNames   capability.HolidaySchoolNames
	requireManage common.Middleware
}

// NewHolidayPeriodResource builds the resource with the review of the
// period's registrations (#4663). requireManage admits only a session with
// the Träger right "Anmeldung verwalten"; the root binds it to the right
// Identity & Access bundles into the Träger roles (ADR 0084 section 4).
// schoolNames names the schools in the export.
func NewHolidayPeriodResource(periods capability.HolidayPeriods, review capability.HolidayRegistrationReview, schoolNames capability.HolidaySchoolNames, requireManage common.Middleware) *HolidayPeriodResource {
	if periods == nil || review == nil || schoolNames == nil || requireManage == nil {
		panic("enrollment http: the holiday periods, the registration review, the school names and the right check are required")
	}
	return &HolidayPeriodResource{periods: periods, review: review, schoolNames: schoolNames, requireManage: requireManage}
}

// TraegerRouter returns the routes mounted at /traeger/holiday-periods,
// with the review of each period's registrations.
func (rs *HolidayPeriodResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		manage := r.With(rs.requireManage, withTx)
		manage.Get("/", rs.list)
		manage.Post("/", rs.create)
		manage.Get("/{id}", rs.get)
		manage.Put("/{id}", rs.update)
		manage.Post("/{id}/publish", rs.publish)
		rs.registrationRoutes(r, withTx)
	})
	return r
}

type holidayTimeModelRequest struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	PriceCents int64  `json:"price_cents"`
}

type holidayWeekRequest struct {
	ID           int64                     `json:"id"`
	StartsOn     calendar.Date             `json:"starts_on"`
	EndsOn       calendar.Date             `json:"ends_on"`
	HostSchoolID int64                     `json:"host_school_id"`
	TimeModels   []holidayTimeModelRequest `json:"time_models"`
}

// HolidayPeriodRequest is the body of POST /traeger/holiday-periods and PUT
// /traeger/holiday-periods/{id}: the whole period with its weeks. Prices are
// whole cent.
type HolidayPeriodRequest struct {
	Name                 string               `json:"name"`
	RegistrationOpensOn  calendar.Date        `json:"registration_opens_on"`
	RegistrationClosesOn calendar.Date        `json:"registration_closes_on"`
	Binding              bool                 `json:"binding"`
	Notes                string               `json:"notes"`
	Weeks                []holidayWeekRequest `json:"weeks"`
}

// Bind accepts the body; the Owner validates the period as a whole.
func (req *HolidayPeriodRequest) Bind(*http.Request) error { return nil }

func (req *HolidayPeriodRequest) input() capability.HolidayPeriodInput {
	weeks := make([]capability.HolidayWeekInput, len(req.Weeks))
	for i, week := range req.Weeks {
		models := make([]capability.HolidayTimeModelInput, len(week.TimeModels))
		for j, model := range week.TimeModels {
			models[j] = capability.HolidayTimeModelInput(model)
		}
		weeks[i] = capability.HolidayWeekInput{
			ID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolID: week.HostSchoolID, TimeModels: models,
		}
	}
	return capability.HolidayPeriodInput{
		Name: req.Name, RegistrationOpensOn: req.RegistrationOpensOn, RegistrationClosesOn: req.RegistrationClosesOn,
		Binding: req.Binding, Notes: req.Notes, Weeks: weeks,
	}
}

// traegerOf returns the Träger of the session, or renders 401.
func traegerOf(w http.ResponseWriter, r *http.Request) (int64, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, false
	}
	return principal.OrganizationID(), true
}

func (rs *HolidayPeriodResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	periods, err := rs.periods.ListHolidayPeriods(r.Context(), organizationID)
	if err != nil {
		renderHolidayPeriodError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, periods, "Holiday periods retrieved successfully")
}

func (rs *HolidayPeriodResource) get(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	period, err := rs.periods.HolidayPeriod(r.Context(), organizationID, periodID)
	if err != nil {
		renderHolidayPeriodError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, period, "Holiday period retrieved successfully")
}

func (rs *HolidayPeriodResource) create(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	req := &HolidayPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	period, err := rs.periods.CreateHolidayPeriod(r.Context(), organizationID, req.input())
	if err != nil {
		renderHolidayPeriodError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, period, "Holiday period created successfully")
}

func (rs *HolidayPeriodResource) update(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	period, err := rs.periods.UpdateHolidayPeriod(r.Context(), organizationID, periodID, req.input())
	if err != nil {
		renderHolidayPeriodError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, period, "Holiday period updated successfully")
}

func (rs *HolidayPeriodResource) publish(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	period, err := rs.periods.PublishHolidayPeriod(r.Context(), organizationID, periodID)
	if err != nil {
		renderHolidayPeriodError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, period, "Holiday period published successfully")
}

// renderHolidayPeriodError answers a refused holiday period operation. Every
// refusal rolls the Träger transaction back: a write may already have run.
func renderHolidayPeriodError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	var fields validation.Errors
	var refusedHost *capability.HolidayHostSchoolRefusal
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayPeriodInvalid))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.As(err, &refusedHost):
		field := fmt.Sprintf("weeks.%d.host_school_id", refusedHost.Week)
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeEnrollmentHolidayHostSchoolOutsideTraeger, field))
	case errors.Is(err, capability.ErrHolidayWeekBooked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayWeekBooked))
	case errors.Is(err, capability.ErrHolidayPeriodIncomplete):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayPeriodIncomplete))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
