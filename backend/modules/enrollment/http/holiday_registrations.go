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
	proofs        HolidayProofUploads
	captcha       capability.CaptchaVerifier
}

// HolidayProofUploads stores a proof of benefits of the public link
// (#5238) in a Träger unit the root opens. The route counts the upload
// before it reads the body, in a unit of its own (countAttempt), so a
// refused upload still counts, like the uploads of the online registration
// (#4783).
type HolidayProofUploads interface {
	UploadHolidayProof(ctx context.Context, link capability.HolidayRegistrationLink, upload capability.HolidayProofUpload) (capability.HolidayProofReceipt, error)
}

// NewHolidayRegistrationResource builds the resource. captcha is always
// required for the public link; proofs stores the proofs of benefits.
func NewHolidayRegistrationResource(registrations capability.HolidayRegistrations, proofs HolidayProofUploads, captcha capability.CaptchaVerifier) *HolidayRegistrationResource {
	if registrations == nil || proofs == nil || captcha == nil {
		panic("enrollment http: the holiday registrations, the proof uploads and the captcha are required")
	}
	return &HolidayRegistrationResource{registrations: registrations, proofs: proofs, captcha: captcha}
}

// Router returns the public routes mounted at /traeger/holiday-registrations.
func (rs *HolidayRegistrationResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{token}", rs.offer)
	r.Get("/{token}/documents/{documentId}", rs.document)
	r.Post("/{token}", rs.submit)
	r.Post("/{token}/uploads", rs.uploadProof)
	return r
}

// HolidayRegistrationSchool is a school of the Träger a child may attend.
type HolidayRegistrationSchool struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// HolidayRegistrationTimeModel is a bookable care time of a week with its
// price per week (#5237). The price is the base line the Elternabrechnung
// writes for this booking in the summary (#4664); sibling and exemption
// rules apply only there.
type HolidayRegistrationTimeModel struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	PriceCents int64  `json:"price_cents"`
}

// HolidayRegistrationWeek is a bookable week with the name of its host school.
type HolidayRegistrationWeek struct {
	ID             int64                          `json:"id"`
	StartsOn       calendar.Date                  `json:"starts_on"`
	EndsOn         calendar.Date                  `json:"ends_on"`
	HostSchoolName string                         `json:"host_school_name"`
	TimeModels     []HolidayRegistrationTimeModel `json:"time_models"`
	// BookingUnit is "week" or "day"; a week by days lists its CareDays and
	// the FullDays without a free place, and its prices are per day (#4701).
	BookingUnit string          `json:"booking_unit"`
	CareDays    []calendar.Date `json:"care_days"`
	FullDays    []calendar.Date `json:"full_days"`
	// NoCareDays are the days the host school gives no care (#4702); such
	// a week takes no booking.
	NoCareDays []capability.HolidayNoCareDay `json:"no_care_days"`
}

// HolidayRegistrationOfferResponse is the body of GET
// /traeger/holiday-registrations/{token}.
type HolidayRegistrationOfferResponse struct {
	TraegerName          string                    `json:"traeger_name"`
	Name                 string                    `json:"name"`
	RegistrationOpensOn  calendar.Date             `json:"registration_opens_on"`
	RegistrationClosesOn calendar.Date             `json:"registration_closes_on"`
	Binding              bool                      `json:"binding"`
	Notes                string                    `json:"notes"`
	Open                 bool                      `json:"open"`
	Weeks                []HolidayRegistrationWeek `json:"weeks"`
	// MaxDaysPerChild bounds the care days per child, null for no bound
	// (#4701).
	MaxDaysPerChild *int                        `json:"max_days_per_child"`
	Schools         []HolidayRegistrationSchool `json:"schools"`
	CaptchaSiteKey  string                      `json:"captcha_site_key"`
	Form            HolidayRegistrationForm     `json:"form"`
	// ConsentTexts are the German texts of the fixed declarations the form
	// shows; they bind, a translation only helps reading (#5626).
	ConsentTexts capability.HolidayConsentTexts `json:"consent_texts"`
	// HolidayWishMode says whether and how the period takes the holiday
	// care wish of the online registration into account (#4703): off,
	// priority or wish_only. It names a rule, no child.
	HolidayWishMode string `json:"holiday_wish_mode"`
}

// HolidayRegistrationForm is the current version of the period's form for
// parents (#5198): its questions and whether it asks for the postal address
// (#5625), without who saved it when.
type HolidayRegistrationForm struct {
	Version            int                                  `json:"version"`
	Fields             []capability.HolidayFormField        `json:"fields"`
	LegalBlocks        []capability.FormLegalBlock          `json:"legal_blocks"`
	AddressRequirement capability.HolidayAddressRequirement `json:"address_requirement"`
}

type holidayGuardianRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type holidayAddressRequest struct {
	Street     string `json:"street"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
}

type holidayBookingRequest struct {
	WeekID      int64           `json:"week_id"`
	TimeModelID int64           `json:"time_model_id"`
	Days        []calendar.Date `json:"days"`
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
	// WishProof is the sealed statement of the holiday care wish from the
	// prefill of a parent with an account (#4703).
	WishProof string `json:"wish_proof"`
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
// per child (#5198). benefits_proof_tokens name the uploaded proofs of the
// benefits (#5238). consent_version names the version of the consent texts
// the parent agreed to (#4722). custody_confirmed and declaration_name are
// the custody and the typed name of the declaration (#5626).
// guardian_address is the postal address of the parent, when the form
// asks for it (#5625).
type HolidayRegistrationRequest struct {
	Guardian          holidayGuardianRequest    `json:"guardian"`
	GuardianAddress   *holidayAddressRequest    `json:"guardian_address"`
	SecondGuardian    *holidayGuardianRequest   `json:"second_guardian"`
	Children          []holidayChildRequest     `json:"children"`
	EmergencyContacts []holidayContactRequest   `json:"emergency_contacts"`
	PickupPersons     []holidayContactRequest   `json:"pickup_persons"`
	PrivacyConsent    bool                      `json:"privacy_consent"`
	HealthConsent     bool                      `json:"health_consent"`
	BindingConfirmed  bool                      `json:"binding_confirmed"`
	CustodyConfirmed  bool                      `json:"custody_confirmed"`
	DeclarationName   string                    `json:"declaration_name"`
	BenefitsDeclared  bool                      `json:"benefits_declared"`
	BenefitsProofs    []string                  `json:"benefits_proof_tokens"`
	CaptchaToken      string                    `json:"captcha_token"`
	FormVersion       int                       `json:"form_version"`
	CustomData        capability.HolidayAnswers `json:"custom_data"`
	LegalAcceptances  map[string]bool           `json:"legal_acceptances"`
	ConsentVersion    int                       `json:"consent_version"`
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
			WishProof: child.WishProof,
		}
	}
	var secondGuardian *capability.HolidayGuardianInput
	if req.SecondGuardian != nil {
		value := capability.HolidayGuardianInput(*req.SecondGuardian)
		secondGuardian = &value
	}
	var address *capability.HolidayAddress
	if req.GuardianAddress != nil {
		value := capability.HolidayAddress(*req.GuardianAddress)
		address = &value
	}
	return capability.HolidayRegistrationInput{
		GuardianAddress:   address,
		SecondGuardian:    secondGuardian,
		Guardian:          capability.HolidayGuardianInput(req.Guardian),
		Children:          children,
		EmergencyContacts: holidayContacts(req.EmergencyContacts),
		PickupPersons:     holidayContacts(req.PickupPersons),
		PrivacyConsent:    req.PrivacyConsent, HealthConsent: req.HealthConsent, BindingConfirmed: req.BindingConfirmed,
		CustodyConfirmed: req.CustodyConfirmed, DeclarationName: req.DeclarationName,
		LegalAcceptances: req.LegalAcceptances, BenefitsDeclared: req.BenefitsDeclared, BenefitsProofTokens: req.BenefitsProofs,
		FormVersion: req.FormVersion, CustomData: req.CustomData, ConsentVersion: req.ConsentVersion,
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
	// Placements say for every booking whether the child got a place or is
	// on the waiting list (#4699), by the child's position in the form.
	Placements []capability.HolidayReceiptPlacement `json:"placements"`
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
			models = append(models, HolidayRegistrationTimeModel{
				ID: model.ID, Name: model.Name, StartsAt: model.StartsAt, EndsAt: model.EndsAt, PriceCents: model.PriceCents,
			})
		}
		careDays, fullDays := []calendar.Date{}, []calendar.Date{}
		if week.BookingUnit == capability.HolidayBookingUnitDay {
			careDays = append(careDays, week.CareDays...)
			fullDays = append(fullDays, offer.FullDays[week.ID]...)
		}
		weeks = append(weeks, HolidayRegistrationWeek{
			ID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolName: names[week.HostSchoolID], TimeModels: models,
			BookingUnit: week.BookingUnit, CareDays: careDays, FullDays: fullDays,
			NoCareDays: noCareDaysOrEmpty(week.NoCareDays),
		})
	}
	return HolidayRegistrationOfferResponse{
		TraegerName: link.Traeger.Name, Name: offer.Name, RegistrationOpensOn: offer.RegistrationOpensOn,
		RegistrationClosesOn: offer.RegistrationClosesOn, Binding: offer.Binding, Notes: offer.Notes, Open: offer.Open,
		Weeks: weeks, Schools: schools, CaptchaSiteKey: siteKey, MaxDaysPerChild: offer.MaxDaysPerChild,
		Form: HolidayRegistrationForm{
			Version: offer.Form.Version, Fields: holidayFormFields(offer.Form.Fields),
			LegalBlocks: publicHolidayLegalBlocks(offer.Form.LegalBlocks), AddressRequirement: offer.Form.AddressRequirement,
		},
		ConsentTexts: offer.ConsentTexts, HolidayWishMode: offer.HolidayWishMode,
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
	if err := rs.countAttempt(r.Context(), link.OrganizationID, func(txCtx context.Context) error {
		return rs.registrations.CountHolidayRegistrationAttempt(txCtx, link.OrganizationID, remoteIP, req.Guardian.Email)
	}); err != nil {
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

// countAttempt runs one abuse counter of the public link, a registration or
// a proof upload, in a Träger transaction of its own that commits even when
// the count refuses, so a refused attempt still counts.
func (rs *HolidayRegistrationResource) countAttempt(ctx context.Context, organizationID int64, count func(context.Context) error) error {
	var limitErr error
	err := withinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		limitErr = count(txCtx)
		if errors.Is(limitErr, capability.ErrRateLimited) || errors.Is(limitErr, capability.ErrUploadRateLimited) {
			return nil
		}
		return limitErr
	})
	if err != nil {
		return err
	}
	return limitErr
}

// renderHolidayRegistrationError answers a refused public registration or
// proof upload.
func renderHolidayRegistrationError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	var rejection *capability.UploadRejection
	if renderHolidayWeekWithoutCare(w, r, err) {
		return
	}
	var dayLimit *capability.HolidayChildDayLimitRefusal
	switch {
	case common.HasInputRejection(err):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.As(err, &rejection):
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentUploadInvalid,
			map[string]any{"max_mb": rejection.MaxMB, "allowed_types": rejection.AllowedTypes}))
	case errors.Is(err, capability.ErrUploadRateLimited):
		setRateLimitRetryAfter(w)
		common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(err, common.CodeEnrollmentUploadRateLimited))
	case errors.Is(err, capability.ErrHolidayProofExpired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentUploadExpired))
	case errors.As(err, &dayLimit):
		common.RenderError(w, r, holidayChildDayLimitError(err, dayLimit))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayRegistrationInvalid))
	case errors.Is(err, capability.ErrHolidayRegistrationNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationClosed))
	case errors.Is(err, capability.ErrHolidayRegistrationFormChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationFormChanged))
	case errors.Is(err, capability.ErrHolidayWishMissing):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayWishMissing))
	case errors.Is(err, capability.ErrRateLimited):
		setRateLimitRetryAfter(w)
		common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(err, common.CodeEnrollmentSubmissionRateLimited))
	case errors.Is(err, capability.ErrCaptchaRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCaptchaRequired))
	case errors.Is(err, capability.ErrCaptchaFailed):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCaptchaFailed))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// Disabled blocks belong only to the Träger editor, not the public offer.
func publicHolidayLegalBlocks(blocks []capability.FormLegalBlock) []capability.FormLegalBlock {
	visible := make([]capability.FormLegalBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Enabled {
			visible = append(visible, block)
		}
	}
	return visible
}

// holidayChildDayLimitError answers bookings of a child over the care days
// of its period (#4701) with the bound, the days asked for and the child by
// its position in the form, from 1.
func holidayChildDayLimitError(err error, refusal *capability.HolidayChildDayLimitRefusal) render.Renderer {
	return common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentHolidayChildDayLimit,
		map[string]any{"limit": refusal.Limit, "days": refusal.Days, "child": refusal.Child + 1})
}
