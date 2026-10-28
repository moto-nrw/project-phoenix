package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayRegistrationResource serves the public holiday registration
// (#4661, ADR 0110): parents without an account open the one link of a
// published holiday period and register up to three siblings. There is no
// session; the link names the Träger. The administrative transaction only
// resolves the link, everything else runs in the Träger transaction of the
// resolved Träger.
type HolidayRegistrationResource struct {
	registrations capability.HolidayRegistrations
	captcha       capability.CaptchaVerifier
}

// NewHolidayRegistrationResource builds the resource. captcha is always
// required for the public link.
func NewHolidayRegistrationResource(registrations capability.HolidayRegistrations, captcha capability.CaptchaVerifier) *HolidayRegistrationResource {
	if registrations == nil || captcha == nil {
		panic("enrollment http: the holiday registrations and the captcha are required")
	}
	return &HolidayRegistrationResource{registrations: registrations, captcha: captcha}
}

// Router returns the public routes mounted at /traeger/holiday-registrations.
func (rs *HolidayRegistrationResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{token}", rs.offer)
	r.Post("/{token}", rs.submit)
	return r
}

// HolidayRegistrationSchool is a school of the Träger a child may attend.
type HolidayRegistrationSchool struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// HolidayRegistrationTimeModel is a bookable care time of a week. Its price
// reaches the parents only as part of the contribution the Elternabrechnung
// computes for the summary (#4664).
type HolidayRegistrationTimeModel struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// HolidayRegistrationWeek is a bookable week with the name of its host school.
type HolidayRegistrationWeek struct {
	ID             int64                          `json:"id"`
	StartsOn       calendar.Date                  `json:"starts_on"`
	EndsOn         calendar.Date                  `json:"ends_on"`
	HostSchoolName string                         `json:"host_school_name"`
	TimeModels     []HolidayRegistrationTimeModel `json:"time_models"`
}

// HolidayRegistrationOfferResponse is the body of GET
// /traeger/holiday-registrations/{token}.
type HolidayRegistrationOfferResponse struct {
	TraegerName          string                      `json:"traeger_name"`
	Name                 string                      `json:"name"`
	RegistrationOpensOn  calendar.Date               `json:"registration_opens_on"`
	RegistrationClosesOn calendar.Date               `json:"registration_closes_on"`
	Binding              bool                        `json:"binding"`
	Notes                string                      `json:"notes"`
	Open                 bool                        `json:"open"`
	Weeks                []HolidayRegistrationWeek   `json:"weeks"`
	Schools              []HolidayRegistrationSchool `json:"schools"`
	CaptchaSiteKey       string                      `json:"captcha_site_key"`
	Form                 HolidayRegistrationForm     `json:"form"`
}

// HolidayRegistrationForm is the current version of the period's form for
// parents (#5198): its questions, without who saved it when.
type HolidayRegistrationForm struct {
	Version int                           `json:"version"`
	Fields  []capability.HolidayFormField `json:"fields"`
}

type holidayGuardianRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type holidayBookingRequest struct {
	WeekID      int64 `json:"week_id"`
	TimeModelID int64 `json:"time_model_id"`
}

type holidayChildRequest struct {
	FirstName       string                    `json:"first_name"`
	LastName        string                    `json:"last_name"`
	BirthDate       calendar.Date             `json:"birth_date"`
	HomeSchoolID    int64                     `json:"home_school_id"`
	GuestSchoolName string                    `json:"guest_school_name"`
	Allergies       string                    `json:"allergies"`
	Medication      string                    `json:"medication"`
	HealthNotes     string                    `json:"health_notes"`
	Bookings        []holidayBookingRequest   `json:"bookings"`
	CustomData      capability.HolidayAnswers `json:"custom_data"`
}

type holidayContactRequest struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Relationship string `json:"relationship"`
}

// HolidayRegistrationRequest is the body of POST
// /traeger/holiday-registrations/{token}. home_school_id 0 makes a child a holiday
// guest. form_version names the version of the form the parent filled in;
// custom_data holds the answers per family, children[].custom_data those
// per child (#5198).
type HolidayRegistrationRequest struct {
	Guardian          holidayGuardianRequest    `json:"guardian"`
	SecondGuardian    *holidayGuardianRequest   `json:"second_guardian"`
	Children          []holidayChildRequest     `json:"children"`
	EmergencyContacts []holidayContactRequest   `json:"emergency_contacts"`
	PickupPersons     []holidayContactRequest   `json:"pickup_persons"`
	PrivacyConsent    bool                      `json:"privacy_consent"`
	HealthConsent     bool                      `json:"health_consent"`
	BindingConfirmed  bool                      `json:"binding_confirmed"`
	BenefitsDeclared  bool                      `json:"benefits_declared"`
	CaptchaToken      string                    `json:"captcha_token"`
	FormVersion       int                       `json:"form_version"`
	CustomData        capability.HolidayAnswers `json:"custom_data"`
}

// Bind accepts the body; the Owner validates the registration as a whole.
func (req *HolidayRegistrationRequest) Bind(*http.Request) error { return nil }

func (req *HolidayRegistrationRequest) input() capability.HolidayRegistrationInput {
	children := make([]capability.HolidayChildInput, len(req.Children))
	for i, child := range req.Children {
		bookings := make([]capability.HolidayBookingInput, len(child.Bookings))
		for j, booking := range child.Bookings {
			bookings[j] = capability.HolidayBookingInput(booking)
		}
		children[i] = capability.HolidayChildInput{
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			HomeSchoolID: child.HomeSchoolID, GuestSchoolName: child.GuestSchoolName, Allergies: child.Allergies,
			Medication: child.Medication, HealthNotes: child.HealthNotes, Bookings: bookings, CustomData: child.CustomData,
		}
	}
	var secondGuardian *capability.HolidayGuardianInput
	if req.SecondGuardian != nil {
		value := capability.HolidayGuardianInput(*req.SecondGuardian)
		secondGuardian = &value
	}
	return capability.HolidayRegistrationInput{
		SecondGuardian:    secondGuardian,
		Guardian:          capability.HolidayGuardianInput(req.Guardian),
		Children:          children,
		EmergencyContacts: holidayContacts(req.EmergencyContacts),
		PickupPersons:     holidayContacts(req.PickupPersons),
		PrivacyConsent:    req.PrivacyConsent, HealthConsent: req.HealthConsent, BindingConfirmed: req.BindingConfirmed,
		BenefitsDeclared: req.BenefitsDeclared, FormVersion: req.FormVersion, CustomData: req.CustomData,
	}
}

func holidayContacts(contacts []holidayContactRequest) []capability.HolidayContactInput {
	out := make([]capability.HolidayContactInput, len(contacts))
	for i, contact := range contacts {
		out[i] = capability.HolidayContactInput(contact)
	}
	return out
}

// HolidayRegistrationReceiptResponse confirms a stored registration without
// repeating its personal data.
type HolidayRegistrationReceiptResponse struct {
	ID          int64     `json:"id"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// resolveLink resolves the link in the administrative transaction.
func (rs *HolidayRegistrationResource) resolveLink(ctx context.Context, token string) (capability.HolidayRegistrationLink, error) {
	var link capability.HolidayRegistrationLink
	err := withinAdmin(ctx, func(adminCtx context.Context) error {
		resolved, err := rs.registrations.ResolveHolidayRegistrationLink(adminCtx, token)
		link = resolved
		return err
	})
	return link, err
}

func (rs *HolidayRegistrationResource) offer(w http.ResponseWriter, r *http.Request) {
	link, err := rs.resolveLink(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	var offer capability.HolidayRegistrationOffer
	err = withinTraeger(r.Context(), link.OrganizationID, func(txCtx context.Context) error {
		loaded, loadErr := rs.registrations.HolidayRegistrationOffer(txCtx, link)
		offer = loaded
		return loadErr
	})
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	siteKey, err := rs.captcha.SiteKey(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, holidayOfferResponse(link, offer, siteKey), "Holiday registration retrieved")
}

func holidayOfferResponse(link capability.HolidayRegistrationLink, offer capability.HolidayRegistrationOffer, siteKey string) HolidayRegistrationOfferResponse {
	names := map[int64]string{}
	schools := make([]HolidayRegistrationSchool, 0, len(link.Traeger.Schools))
	for _, school := range link.Traeger.Schools {
		names[school.ID] = school.Name
		schools = append(schools, HolidayRegistrationSchool(school))
	}
	weeks := make([]HolidayRegistrationWeek, 0, len(offer.Weeks))
	for _, week := range offer.Weeks {
		models := make([]HolidayRegistrationTimeModel, 0, len(week.TimeModels))
		for _, model := range week.TimeModels {
			models = append(models, HolidayRegistrationTimeModel{ID: model.ID, Name: model.Name, StartsAt: model.StartsAt, EndsAt: model.EndsAt})
		}
		weeks = append(weeks, HolidayRegistrationWeek{
			ID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolName: names[week.HostSchoolID], TimeModels: models,
		})
	}
	return HolidayRegistrationOfferResponse{
		TraegerName: link.Traeger.Name, Name: offer.Name, RegistrationOpensOn: offer.RegistrationOpensOn,
		RegistrationClosesOn: offer.RegistrationClosesOn, Binding: offer.Binding, Notes: offer.Notes, Open: offer.Open,
		Weeks: weeks, Schools: schools, CaptchaSiteKey: siteKey,
		Form: HolidayRegistrationForm{Version: offer.Form.Version, Fields: holidayFormFields(offer.Form.Fields)},
	}
}

func holidayFormFields(fields []capability.HolidayFormField) []capability.HolidayFormField {
	if fields == nil {
		return []capability.HolidayFormField{}
	}
	return fields
}

// submit counts the attempt, checks the captcha and stores the registration
// with its receipt mail. The counter commits in a Träger transaction of its
// own, so a refused registration still counts.
func (rs *HolidayRegistrationResource) submit(w http.ResponseWriter, r *http.Request) {
	req := &HolidayRegistrationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	link, err := rs.resolveLink(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	remoteIP := remoteIPFromRequest(r)
	if err := rs.countAttempt(r.Context(), link.OrganizationID, remoteIP, req.Guardian.Email); err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	if err := rs.captcha.Verify(r.Context(), req.CaptchaToken, remoteIP); err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	var receipt capability.HolidayRegistrationReceipt
	err = withinTraeger(r.Context(), link.OrganizationID, func(txCtx context.Context) error {
		stored, submitErr := rs.registrations.SubmitHolidayRegistration(txCtx, link, req.input())
		receipt = stored
		return submitErr
	})
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, HolidayRegistrationReceiptResponse(receipt), "Holiday registration received")
}

// countAttempt commits the attempt even when it crosses the limit.
func (rs *HolidayRegistrationResource) countAttempt(ctx context.Context, organizationID int64, remoteIP, email string) error {
	var limitErr error
	err := withinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		limitErr = rs.registrations.CountHolidayRegistrationAttempt(txCtx, organizationID, remoteIP, email)
		if errors.Is(limitErr, capability.ErrRateLimited) {
			return nil
		}
		return limitErr
	})
	if err != nil {
		return err
	}
	return limitErr
}

// renderHolidayRegistrationError answers a refused public registration.
func renderHolidayRegistrationError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayRegistrationInvalid))
	case errors.Is(err, capability.ErrHolidayRegistrationNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationClosed))
	case errors.Is(err, capability.ErrHolidayRegistrationFormChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationFormChanged))
	case errors.Is(err, capability.ErrRateLimited):
		// The same hint as the online registration: no exact remaining time.
		w.Header().Set("Retry-After", "3600")
		common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(err, common.CodeEnrollmentSubmissionRateLimited))
	case errors.Is(err, capability.ErrCaptchaRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCaptchaRequired))
	case errors.Is(err, capability.ErrCaptchaFailed):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCaptchaFailed))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
