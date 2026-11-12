package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// HolidayPeriodResource serves the holiday periods of a Träger in the
// Träger-Portal (#4659). Every route sits behind the Träger session, needs
// the Träger right "Anmeldung verwalten" and runs in the Träger transaction
// of the session's Träger (ADR 0105). The Träger comes from the session,
// never from the request.
type HolidayPeriodResource struct {
	periods       capability.HolidayPeriods
	review        capability.HolidayRegistrationReview
	records       capability.HolidayRegistrationRecords
	wishes        capability.HolidayWishReview
	places        HolidayPlaceReads
	allocation    HolidayAllocationReads
	decide        HolidayAllocationDecision
	schoolNames   capability.HolidaySchoolNames
	known         capability.HolidayKnownChildren
	knownTrail    capability.HolidayKnownChildTrail
	requireManage common.Middleware
	requireHealth common.Middleware
}

// NewHolidayPeriodResource builds the resource with the review of the
// period's registrations (#4663). requireManage admits only a session with
// the Träger right "Anmeldung verwalten"; the root binds it to the right
// Identity & Access bundles into the Träger roles (ADR 0084 section 4).
// schoolNames names the schools in the export. requireHealth admits only a
// session with the Träger right "Gesundheitsangaben sehen" to the health
// information of a registration (#4722).
// places lists the places of the weeks (#4699), allocation names the
// criteria and the proposal, decide decides on the places by them (#4700).
// records reads the registrations for their files (#6691). known finds the
// registered children a school of the Träger knows and knownTrail records that
// comparison (#6683, ADR 0214).
func NewHolidayPeriodResource(periods capability.HolidayPeriods, review capability.HolidayRegistrationReview, records capability.HolidayRegistrationRecords, wishes capability.HolidayWishReview, places HolidayPlaceReads, allocation HolidayAllocationReads, decide HolidayAllocationDecision, schoolNames capability.HolidaySchoolNames, known capability.HolidayKnownChildren, knownTrail capability.HolidayKnownChildTrail, requireManage, requireHealth common.Middleware) *HolidayPeriodResource {
	if periods == nil || review == nil || records == nil || wishes == nil || places == nil || allocation == nil || decide == nil || schoolNames == nil || known == nil || knownTrail == nil || requireManage == nil || requireHealth == nil {
		panic("enrollment http: the holiday periods, the registration review, the registration records, the wish review, the places, the allocation, the decision, the school names, the known children with their trail and the right checks are required")
	}
	return &HolidayPeriodResource{periods: periods, review: review, records: records, wishes: wishes, places: places, allocation: allocation, decide: decide, schoolNames: schoolNames, known: known, knownTrail: knownTrail, requireManage: requireManage, requireHealth: requireHealth}
}

// HolidayAllocationReads names the Träger's criteria, shows the proposal of
// a period and records the Büro's check of answers (#4700).
type HolidayAllocationReads interface {
	HolidayCriteria(ctx context.Context, reviewer capability.HolidayReviewer) (capability.HolidayCriteriaCatalog, error)
	SaveHolidayCriteria(ctx context.Context, reviewer capability.HolidayReviewer, criteria []capability.HolidayCriterionInput) (capability.HolidayCriteriaCatalog, error)
	HolidayAllocationBoard(ctx context.Context, reviewer capability.HolidayReviewer, periodID int64) (capability.HolidayAllocationBoard, error)
	CheckHolidayCriteria(ctx context.Context, reviewer capability.HolidayReviewer, periodID, registrationID int64, checks []capability.HolidayCriterionCheck) (capability.HolidayRegistrationDetail, error)
}

// HolidayAllocationDecision decides on the pending bookings of a period
// (#4700). The root binds the holiday allocation workflow (ADR 0206), which
// holds the period's invoices and the period, lets Enrollment decide and
// computes the contributions of everyone who gained a place, in the
// request's Träger transaction.
type HolidayAllocationDecision interface {
	DecideHolidayAllocation(ctx context.Context, reviewer capability.HolidayReviewer, periodID int64, choices []capability.HolidayAllocationChoice) (capability.HolidayAllocationBoard, error)
}

// HolidayPlaceReads lists the places of the weeks of a period (#4699).
type HolidayPlaceReads interface {
	HolidayWeekPlaces(ctx context.Context, reviewer capability.HolidayReviewer, periodID int64) ([]capability.HolidayWeekPlaces, error)
}

// TraegerRouter returns the routes mounted at /traeger/holiday-periods,
// with the review of each period's registrations.
func (rs *HolidayPeriodResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		manage := r.With(rs.requireManage, withTx)
		manage.Get("/criteria", rs.listCriteria)
		manage.Put("/criteria", rs.saveCriteria)
		manage.Get("/", rs.list)
		manage.Post("/", rs.create)
		manage.Get("/{id}", rs.get)
		manage.Put("/{id}", rs.update)
		manage.Post("/{id}/publish", rs.publish)
		manage.Get("/{id}/form", rs.getForm)
		manage.Put("/{id}/form", rs.saveForm)
		manage.Post("/{id}/form/documents", rs.uploadFormDocument)
		manage.Get("/{id}/form/documents/{documentId}", rs.getFormDocument)
		manage.Delete("/{id}/form/documents/{documentId}", rs.deleteFormDocument)
		rs.registrationRoutes(r, withTx)
		rs.allocationRoutes(r, withTx)
	})
	return r
}

type holidayTimeModelRequest struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	PriceCents int64  `json:"price_cents"`
	Archived   bool   `json:"archived"`
}

type holidayWeekRequest struct {
	ID           int64                     `json:"id"`
	StartsOn     calendar.Date             `json:"starts_on"`
	EndsOn       calendar.Date             `json:"ends_on"`
	HostSchoolID int64                     `json:"host_school_id"`
	TimeModels   []holidayTimeModelRequest `json:"time_models"`
	// BookingUnit is "week" or "day"; empty is a week (#4701).
	BookingUnit string `json:"booking_unit"`
}

// HolidayPeriodRequest is the body of POST /traeger/holiday-periods and PUT
// /traeger/holiday-periods/{id}: the whole period with its weeks. Prices are
// whole cent.
type HolidayPeriodRequest struct {
	Name                 string        `json:"name"`
	RegistrationOpensOn  calendar.Date `json:"registration_opens_on"`
	RegistrationClosesOn calendar.Date `json:"registration_closes_on"`
	Binding              bool          `json:"binding"`
	Notes                string        `json:"notes"`
	// AcceptanceMailText is the text of the acceptance mail (#6672).
	AcceptanceMailText string `json:"acceptance_mail_text"`
	HolidayWishRule    bool   `json:"holiday_wish_rule"`
	// MaxDaysPerChild bounds the care days per child, null for no bound
	// (#4701).
	MaxDaysPerChild *int `json:"max_days_per_child"`
	// AllocationMode is "arrival" or "criteria"; empty is "arrival" (#4700).
	AllocationMode string               `json:"allocation_mode"`
	Weeks          []holidayWeekRequest `json:"weeks"`
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
			BookingUnit: week.BookingUnit,
		}
	}
	return capability.HolidayPeriodInput{
		Name: req.Name, RegistrationOpensOn: req.RegistrationOpensOn, RegistrationClosesOn: req.RegistrationClosesOn,
		Binding: req.Binding, Notes: req.Notes, AcceptanceMailText: req.AcceptanceMailText, HolidayWishRule: req.HolidayWishRule, Weeks: weeks, MaxDaysPerChild: req.MaxDaysPerChild,
		AllocationMode: req.AllocationMode,
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
	var refusedDates *capability.HolidayWeeksBookedTogetherRefusal
	var refusedDays *capability.HolidayWeekDayLimitRefusal
	if renderHolidayWeekWithoutCare(w, r, err) {
		return
	}
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayPeriodInvalid))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.As(err, &refusedHost):
		field := fmt.Sprintf("weeks.%d.host_school_id", refusedHost.Week)
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeEnrollmentHolidayHostSchoolOutsideTraeger, field))
	case errors.Is(err, capability.ErrHolidayHostSchoolOutsideTraeger):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayHostSchoolOutsideTraeger))
	case errors.As(err, &refusedDates):
		field := fmt.Sprintf("weeks.%d.starts_on", refusedDates.Week)
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeEnrollmentHolidayWeeksBookedTogether, field))
	case errors.As(err, &refusedDays):
		field := fmt.Sprintf("weeks.%d.ends_on", refusedDays.Week)
		if refusedDays.StartsEarlier {
			field = fmt.Sprintf("weeks.%d.starts_on", refusedDays.Week)
		}
		common.RenderError(w, r, common.ErrorConflictOnField(err, common.CodeEnrollmentHolidayWeekDayLimit, field))
	case errors.Is(err, capability.ErrHolidayBookingUnitBooked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayBookingUnitBooked))
	case errors.Is(err, capability.ErrHolidayWeekBooked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayWeekBooked))
	case errors.Is(err, capability.ErrHolidayPeriodIncomplete):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayPeriodIncomplete))
	case errors.Is(err, capability.ErrHolidayAllocationModeLocked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayAllocationModeLocked))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
