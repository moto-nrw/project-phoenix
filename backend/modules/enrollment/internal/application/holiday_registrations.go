package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The public holiday registration (#4661, ADR 0110). The link resolution runs
// in the administrative transaction; the offer, the abuse counter and the
// registration run in the Träger transaction of the link's Träger, which the
// caller opens. The registration writes no school table and looks up no
// child: a name and a date of birth unlock nothing.

// Limits of the free texts of a registration. They bound the request.
const (
	holidayPersonNameMaxLength   = 100
	holidayEmailMaxLength        = 254
	holidayPhoneMaxLength        = 40
	holidayContactNameMaxLength  = 120
	holidayRelationshipMaxLength = 60
	holidayGuestSchoolMaxLength  = 120
	holidayHealthFieldMaxLength  = 1000
	holidayHealthNotesMaxLength  = 2000
	// holidayChildMaxAgeYears bounds a plausible date of birth.
	holidayChildMaxAgeYears = 18
)

// HolidayRegistrationStore persists the public registrations of a Träger.
type HolidayRegistrationStore interface {
	// FindHolidayRegistrationLink reads the Träger and the published period
	// of token, with its holiday care wish rule and first day, in the
	// administrative transaction. The link has no Träger name yet.
	FindHolidayRegistrationLink(ctx context.Context, token string) (link enrollment.HolidayRegistrationLink, found bool, err error)
	// CountHolidayRegistrationAttempt counts one attempt in the bucket of
	// value for the Träger and returns the attempts of its running window.
	// The bucket keeps only a SHA-256 digest of value.
	// Counters whose window started before retention are deleted first.
	CountHolidayRegistrationAttempt(ctx context.Context, organizationID int64, keyType, value string, window, retention time.Duration) (int, error)
	// InsertHolidayRegistration stores the registration without its health
	// information and asks seal, once the registration has its ID, for the
	// sealed values to store with it (#4722).
	InsertHolidayRegistration(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayRegistrationInput,
		seal func(registrationID int64) (enrollment.HolidaySealedHealth, error)) (enrollment.HolidayRegistrationReceipt, error)
}

// HolidayRegistrationDependencies are the ports of the public registration.
type HolidayRegistrationDependencies struct {
	Registrations HolidayRegistrationStore
	Periods       HolidayPeriodStore
	Forms         HolidayFormStore
	Proofs        HolidayProofStore
	// Places gives the bookings of a sent registration their places (#4699).
	Places  HolidayPlaceStore
	Traeger enrollment.HolidayTraegerDirectory
	Mail    enrollment.TraegerMailOutbox
	// Health seals the health information of a registration (#4722).
	Health enrollment.HolidayHealthCipher
	// WishModes reads the Träger's holiday wish mode (#4703).
	WishModes enrollment.HolidayWishModes
	// Today is the calendar day the registration window is checked against;
	// Now the instant a statement of the holiday care wish is checked
	// against (#4703).
	Today func() calendar.Date
	Now   func() time.Time
	// Random fills the token of an uploaded proof and Fingerprint names it
	// in the store (#5238); without them no proof is uploaded.
	Random      func([]byte) error
	Fingerprint func([]byte) string
	Logger      *slog.Logger
}

// HolidayRegistrations serves enrollment.HolidayRegistrations.
type HolidayRegistrations struct {
	deps      HolidayRegistrationDependencies
	placement holidayPlacement
}

var _ enrollment.HolidayRegistrations = (*HolidayRegistrations)(nil)

// NewHolidayRegistrations builds the public registration over deps.
func NewHolidayRegistrations(deps HolidayRegistrationDependencies) *HolidayRegistrations {
	if deps.Registrations == nil || deps.Periods == nil || deps.Forms == nil || deps.Proofs == nil || deps.Places == nil || deps.Traeger == nil || deps.Mail == nil {
		panic("enrollment: the holiday registration store, periods, forms, proofs, places, Träger directory and mail outbox are required")
	}
	if deps.Health == nil {
		panic(errHolidayHealthCipherRequired)
	}
	if deps.WishModes == nil {
		panic(errHolidayWishModesRequired)
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &HolidayRegistrations{deps: deps, placement: holidayPlacement{store: deps.Places, traeger: deps.Traeger, mail: deps.Mail}}
}

// ResolveHolidayRegistrationLink names the Träger, the period and the
// Träger's schools of a published period's link.
func (h *HolidayRegistrations) ResolveHolidayRegistrationLink(ctx context.Context, token string) (enrollment.HolidayRegistrationLink, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return enrollment.HolidayRegistrationLink{}, enrollment.ErrHolidayRegistrationNotFound
	}
	link, found, err := h.deps.Registrations.FindHolidayRegistrationLink(ctx, token)
	if err != nil {
		return enrollment.HolidayRegistrationLink{}, fmt.Errorf("enrollment: resolve holiday registration link: %w", err)
	}
	if !found {
		return enrollment.HolidayRegistrationLink{}, enrollment.ErrHolidayRegistrationNotFound
	}
	if link.Traeger, err = h.deps.Traeger.HolidayTraeger(ctx, link.OrganizationID); err != nil {
		return enrollment.HolidayRegistrationLink{}, err
	}
	return link, nil
}

// HolidayRegistrationOffer returns the published period of the link.
func (h *HolidayRegistrations) HolidayRegistrationOffer(ctx context.Context, link enrollment.HolidayRegistrationLink) (enrollment.HolidayRegistrationOffer, error) {
	if link.OrganizationID <= 0 || link.PeriodID <= 0 {
		return enrollment.HolidayRegistrationOffer{}, enrollment.ErrHolidayRegistrationNotFound
	}
	period, err := published(h.deps.Periods.FindHolidayPeriod(ctx, link.OrganizationID, link.PeriodID, false))
	if err != nil {
		return enrollment.HolidayRegistrationOffer{}, err
	}
	form, err := foundHolidayForm(link.PeriodID)(h.deps.Forms.CurrentHolidayPeriodForm(ctx, link.OrganizationID, link.PeriodID))
	if err != nil {
		return enrollment.HolidayRegistrationOffer{}, err
	}
	today := h.deps.Today()
	texts, _ := enrollment.HolidayConsentTextsOf(enrollment.HolidayConsentVersionCurrent)
	wishMode, err := h.offerHolidayWishMode(ctx, link.OrganizationID, period)
	if err != nil {
		return enrollment.HolidayRegistrationOffer{}, err
	}
	return enrollment.HolidayRegistrationOffer{
		HolidayWishMode: wishMode,
		Name:            period.Name, RegistrationOpensOn: period.RegistrationOpensOn, RegistrationClosesOn: period.RegistrationClosesOn,
		Binding: period.Binding, Notes: period.Notes, Weeks: offeredHolidayWeeks(period.Weeks),
		Open: !today.Before(period.RegistrationOpensOn) && !today.After(period.RegistrationClosesOn), Form: form,
		ConsentTexts: texts,
	}, nil
}

// offeredHolidayWeeks are the weeks with the time models parents can book:
// an archived time model is no longer offered (#5994).
func offeredHolidayWeeks(weeks []enrollment.HolidayWeek) []enrollment.HolidayWeek {
	offered := make([]enrollment.HolidayWeek, len(weeks))
	for i, week := range weeks {
		week.TimeModels = slices.DeleteFunc(slices.Clone(week.TimeModels),
			func(model enrollment.HolidayTimeModel) bool { return model.Archived })
		offered[i] = week
	}
	return offered
}

func (h *HolidayRegistrations) publishedPeriod(ctx context.Context, link enrollment.HolidayRegistrationLink) (enrollment.HolidayPeriod, error) {
	if link.OrganizationID <= 0 || link.PeriodID <= 0 {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayRegistrationNotFound
	}
	return published(h.deps.Periods.FindHolidayPeriod(ctx, link.OrganizationID, link.PeriodID, false))
}

// published turns the store's answer into a published period; a draft is
// no period of a link.
func published(period enrollment.HolidayPeriod, found bool, err error) (enrollment.HolidayPeriod, error) {
	if err != nil {
		return enrollment.HolidayPeriod{}, fmt.Errorf("enrollment: read holiday period: %w", err)
	}
	if !found || period.Status != enrollment.HolidayPeriodPublished {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayRegistrationNotFound
	}
	return period, nil
}

// CountHolidayRegistrationAttempt counts the attempt per IP address and per
// e-mail address of the Träger, with the windows and limits of the online
// registration, and refuses once either is crossed. The store keeps only
// SHA-256 digests of the values.
func (h *HolidayRegistrations) CountHolidayRegistrationAttempt(ctx context.Context, organizationID int64, remoteIP, email string) error {
	if organizationID <= 0 {
		return errHolidayPeriodRequired
	}
	for _, key := range submissionAttemptKeys(remoteIP, email) {
		if err := h.countHolidayAttempt(ctx, organizationID, key); err != nil {
			return err
		}
	}
	return nil
}

func (h *HolidayRegistrations) countHolidayAttempt(ctx context.Context, organizationID int64, key rateLimitKey) error {
	bucket := key.bucket
	attempts, err := h.deps.Registrations.CountHolidayRegistrationAttempt(ctx, organizationID, bucket.keyType, key.value, bucket.window, rateLimitRetention)
	if err != nil {
		return fmt.Errorf("enrollment: count holiday %s: %w", bucket.label, err)
	}
	refusal := bucket.refuse(attempts)
	if refusal != nil {
		h.deps.Logger.Info("holiday registration rate-limited",
			slog.String("key_type", bucket.keyType),
			slog.Int("attempts", attempts),
			slog.Int64("organization_id", organizationID))
	}
	return refusal
}

// SubmitHolidayRegistration checks and stores a registration, gives its
// bookings their places and enqueues its receipt mail in the same Träger
// transaction. The registration is inserted before the weeks are locked:
// the Träger-Büro locks the period before the weeks, and the insert's
// foreign key waits for that lock, so no lock order runs the other way
// round (#4699). The answers are checked
// against the current version of the period's form, and the registration
// keeps that version. A parent who filled in an older version is told the
// form changed instead of being refused at fields the page did not show.
func (h *HolidayRegistrations) SubmitHolidayRegistration(ctx context.Context, link enrollment.HolidayRegistrationLink, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationReceipt, error) {
	period, err := h.publishedPeriod(ctx, link)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	today := h.deps.Today()
	if today.Before(period.RegistrationOpensOn) || today.After(period.RegistrationClosesOn) {
		return enrollment.HolidayRegistrationReceipt{}, enrollment.ErrHolidayRegistrationClosed
	}
	form, err := foundHolidayForm(link.PeriodID)(h.deps.Forms.CurrentHolidayPeriodForm(ctx, link.OrganizationID, link.PeriodID))
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	// Consent evidence must name its exact form; only legacy submissions
	// without acknowledgements may default to the current version.
	if len(input.LegalAcceptances) > 0 && input.FormVersion != form.Version {
		return enrollment.HolidayRegistrationReceipt{}, enrollment.InvalidInput(enrollment.CodeHolidayRegistrationFormChanged, "form_version", enrollment.ErrHolidayRegistrationFormChanged)
	}
	if input.FormVersion != 0 && input.FormVersion != form.Version {
		return enrollment.HolidayRegistrationReceipt{}, enrollment.ErrHolidayRegistrationFormChanged
	}
	input.FormVersion = form.Version
	if err := validateHolidayLegalAcceptances(form, input.LegalAcceptances); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	input = normalizeHolidayRegistration(input)
	input.BenefitsProofTokens = normalizeHolidayProofTokens(input.BenefitsProofTokens)
	answers := sanitizeHolidayAnswers(form, decodeHolidayAnswers(input))
	if err := validateHolidayRegistration(input, period, form, answers, link.Traeger.Schools, today); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if !holidayRegistrationHasHealth(input, form, answers) {
		input.HealthConsent = false
	}
	if input, err = withHolidayAnswers(input, answers); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if input, err = h.resolveHolidayWishes(ctx, link, period, input); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	return h.storeSubmitted(ctx, link, period, form, input)
}

// storeSubmitted stores the checked registration, claims its proofs, gives
// its bookings their places and enqueues its receipt. The health
// information leaves the input here: the store sees it only sealed for the
// rows it belongs to (#4722).
func (h *HolidayRegistrations) storeSubmitted(ctx context.Context, link enrollment.HolidayRegistrationLink, period enrollment.HolidayPeriod,
	form enrollment.HolidayPeriodForm, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationReceipt, error) {
	health, input := splitHolidayHealth(input, form)
	receipt, err := h.deps.Registrations.InsertHolidayRegistration(ctx, link.OrganizationID, link.PeriodID, input,
		func(registrationID int64) (enrollment.HolidaySealedHealth, error) {
			return sealHolidayHealth(h.deps.Health, link.OrganizationID, registrationID, health)
		})
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, fmt.Errorf("enrollment: store holiday registration: %w", err)
	}
	if err := h.claimHolidayProofs(ctx, link, receipt.ID, input.BenefitsProofTokens); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if receipt.Placements, err = h.placeSubmitted(ctx, link.OrganizationID, period, input, receipt.ID); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if err := h.enqueueReceipt(ctx, link, period, form, input, receipt); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	return receipt, nil
}

// placeSubmitted gives the bookings of the sent registration their places
// in its weeks and names the placement of each booking.
func (h *HolidayRegistrations) placeSubmitted(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod,
	input enrollment.HolidayRegistrationInput, registrationID int64) ([]enrollment.HolidayReceiptPlacement, error) {
	var weekIDs []int64
	for _, child := range input.Children {
		for _, booking := range child.Bookings {
			weekIDs = append(weekIDs, booking.WeekID)
		}
	}
	own, _, err := h.placement.place(ctx, organizationID, period, weekIDs, registrationID, 0)
	if err != nil {
		return nil, err
	}
	placed := map[[2]int64]bool{}
	for _, booking := range own {
		placed[[2]int64{int64(booking.ChildPosition), booking.WeekID}] = true
	}
	placements := []enrollment.HolidayReceiptPlacement{}
	for i, child := range input.Children {
		for _, booking := range child.Bookings {
			placement := enrollment.HolidayPlacementWaitlisted
			if placed[[2]int64{int64(i + 1), booking.WeekID}] {
				placement = enrollment.HolidayPlacementPlaced
			}
			placements = append(placements, enrollment.HolidayReceiptPlacement{ChildPosition: i + 1, WeekID: booking.WeekID, Placement: placement})
		}
	}
	return placements, nil
}

func (h *HolidayRegistrations) enqueueReceipt(ctx context.Context, link enrollment.HolidayRegistrationLink, period enrollment.HolidayPeriod,
	form enrollment.HolidayPeriodForm, input enrollment.HolidayRegistrationInput, receipt enrollment.HolidayRegistrationReceipt) error {
	mail := holidayReceiptMail(link.Traeger, period, input, receipt.Placements)
	mail.Declaration = holidayMailDeclaration(period, form, input, receipt.SubmittedAt)
	payload, err := json.Marshal(mail)
	if err != nil {
		return fmt.Errorf("enrollment: encode holiday registration mail: %w", err)
	}
	if err := h.deps.Mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: link.OrganizationID, Kind: enrollment.MailKindHolidayRegistrationReceived,
		Recipient: input.Guardian.Email, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: receipt.ID,
		IdempotencyKey: fmt.Sprintf("holiday-registration-received:%d", receipt.ID),
	}); err != nil {
		return fmt.Errorf("enrollment: enqueue holiday registration mail: %w", err)
	}
	return nil
}

// holidayReceiptMail lists each child with one line per booked week: dates,
// host school and time model, the weeks with a place apart from those on the
// waiting list (#4699). It names no date of birth and no health information.
func holidayReceiptMail(traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod, input enrollment.HolidayRegistrationInput,
	placements []enrollment.HolidayReceiptPlacement) enrollment.HolidayRegistrationMail {
	line := holidayWeekLines(traeger, period)
	waitlisted := map[[2]int64]bool{}
	for _, placement := range placements {
		if placement.Placement == enrollment.HolidayPlacementWaitlisted {
			waitlisted[[2]int64{int64(placement.ChildPosition), placement.WeekID}] = true
		}
	}
	children := make([]enrollment.HolidayRegistrationMailChild, 0, len(input.Children))
	for i, child := range input.Children {
		children = append(children, holidayReceiptChild(child, period, line, func(weekID int64) bool {
			return waitlisted[[2]int64{int64(i + 1), weekID}]
		}))
	}
	return enrollment.HolidayRegistrationMail{
		Recipient: input.Guardian.Email, GuardianName: input.Guardian.FirstName + " " + input.Guardian.LastName,
		TraegerName: traeger.Name, PeriodName: period.Name, Binding: period.Binding, Children: children,
	}
}

// holidayMailDeclaration is the parent's copy of the declarations (#5626):
// the typed name, when it was given, the version of the texts and each
// declaration given in its words, then each confirmed notice of the Träger
// from the form version the registration keeps. The texts name no health
// information of the children.
func holidayMailDeclaration(period enrollment.HolidayPeriod, form enrollment.HolidayPeriodForm, input enrollment.HolidayRegistrationInput, givenAt time.Time) *enrollment.HolidayRegistrationMailDeclaration {
	texts, ok := enrollment.HolidayConsentTextsOf(input.ConsentVersion)
	if !ok {
		return nil
	}
	statements := []enrollment.HolidayDeclarationStatement{{Title: "Datenschutz", Text: texts.PrivacyConsent}}
	if input.HealthConsent {
		statements = append(statements, enrollment.HolidayDeclarationStatement{Title: "Gesundheitsangaben", Text: texts.HealthConsent})
	}
	if period.Binding && input.BindingConfirmed {
		statements = append(statements, enrollment.HolidayDeclarationStatement{Title: "Verbindliche Anmeldung", Text: texts.BindingConfirmed})
	}
	statements = append(statements, enrollment.HolidayDeclarationStatement{Title: "Sorgerecht", Text: texts.CustodyConfirmed})
	for _, block := range form.LegalBlocks {
		if block.Enabled && input.LegalAcceptances[block.Key] {
			statements = append(statements, enrollment.HolidayDeclarationStatement{
				Title: block.Title, Text: block.Label, Detail: strings.TrimSpace(block.Text),
			})
		}
	}
	return &enrollment.HolidayRegistrationMailDeclaration{
		Name: input.DeclarationName, GivenAt: givenAt, ConsentVersion: texts.Version, Statements: statements,
	}
}

// holidayReceiptChild lists the booked weeks of a child in the order of the
// period, those with a place apart from those on the waiting list.
func holidayReceiptChild(child enrollment.HolidayChildInput, period enrollment.HolidayPeriod, line func(weekID, timeModelID int64) string,
	waitlisted func(weekID int64) bool) enrollment.HolidayRegistrationMailChild {
	mailChild := enrollment.HolidayRegistrationMailChild{
		Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == 0, Weeks: []string{},
	}
	booked := map[int64]int64{}
	for _, booking := range child.Bookings {
		booked[booking.WeekID] = booking.TimeModelID
	}
	for _, week := range period.Weeks {
		model, ok := booked[week.ID]
		switch {
		case !ok:
		case waitlisted(week.ID):
			mailChild.WaitlistWeeks = append(mailChild.WaitlistWeeks, line(week.ID, model))
		default:
			mailChild.Weeks = append(mailChild.Weeks, line(week.ID, model))
		}
	}
	return mailChild
}

func normalizeHolidayRegistration(input enrollment.HolidayRegistrationInput) enrollment.HolidayRegistrationInput {
	input.DeclarationName = strings.TrimSpace(input.DeclarationName)
	guardian := &input.Guardian
	guardian.FirstName = strings.TrimSpace(guardian.FirstName)
	guardian.LastName = strings.TrimSpace(guardian.LastName)
	guardian.Email = strings.TrimSpace(guardian.Email)
	guardian.Phone = strings.TrimSpace(guardian.Phone)
	children := make([]enrollment.HolidayChildInput, len(input.Children))
	for i, child := range input.Children {
		child.FirstName = strings.TrimSpace(child.FirstName)
		child.LastName = strings.TrimSpace(child.LastName)
		child.GuestSchoolName = strings.TrimSpace(child.GuestSchoolName)
		if child.HomeSchoolID != 0 {
			child.GuestSchoolName = ""
		}
		child.Allergies = strings.TrimSpace(child.Allergies)
		child.Medication = strings.TrimSpace(child.Medication)
		child.HealthNotes = strings.TrimSpace(child.HealthNotes)
		children[i] = child
	}
	input.GuardianAddress = normalizeHolidayAddress(input.GuardianAddress)
	input.SecondGuardian = normalizeHolidaySecondGuardian(input.SecondGuardian)
	input.Children = children
	input.EmergencyContacts = normalizeHolidayContacts(input.EmergencyContacts)
	input.PickupPersons = normalizeHolidayContacts(input.PickupPersons)
	return input
}

func normalizeHolidayContacts(contacts []enrollment.HolidayContactInput) []enrollment.HolidayContactInput {
	out := make([]enrollment.HolidayContactInput, len(contacts))
	for i, contact := range contacts {
		out[i] = enrollment.HolidayContactInput{
			Name: strings.TrimSpace(contact.Name), Phone: strings.TrimSpace(contact.Phone),
			Relationship: strings.TrimSpace(contact.Relationship),
		}
	}
	return out
}

// holidayRegistrationHasHealth reports whether the registration carries
// health information: a fixed health field of a child or an answer to a
// question of the form marked as health information.
func holidayRegistrationHasHealth(input enrollment.HolidayRegistrationInput, form enrollment.HolidayPeriodForm, answers holidayAnswerSet) bool {
	for _, child := range input.Children {
		if child.Allergies != "" || child.Medication != "" || child.HealthNotes != "" {
			return true
		}
	}
	return holidayAnswersHaveHealth(form, answers)
}

// validateHolidayRegistration checks a registration against the period, the
// version of its form and the Träger's schools. Field keys follow the
// request body, such as "children.1.bookings" or
// "children.0.custom_data.tetanus_impfschutz".
func validateHolidayRegistration(input enrollment.HolidayRegistrationInput, period enrollment.HolidayPeriod, form enrollment.HolidayPeriodForm, answers holidayAnswerSet, schools []enrollment.HolidaySchool, today calendar.Date) error {
	errs := validation.Errors{}
	if guardianErrs := validateHolidayGuardian(input.Guardian); len(guardianErrs) > 0 {
		errs["guardian"] = guardianErrs
	}
	if addressErrs := validateHolidayAddress(form.AddressRequirement, input.GuardianAddress); len(addressErrs) > 0 {
		errs["guardian_address"] = addressErrs
	}
	if input.SecondGuardian != nil {
		if guardianErrs := validateHolidaySecondGuardian(*input.SecondGuardian); len(guardianErrs) > 0 {
			errs["second_guardian"] = guardianErrs
		}
	}
	byKey := buildFieldsByKey(holidayFormSchema(form))
	if answerErrs := validateHolidayAnswers(form, false, answers.family,
		fieldVisibilityContext{guardianAnswers: answers.family, fieldsByKey: byKey}); len(answerErrs) > 0 {
		errs["custom_data"] = answerErrs
	}
	switch {
	case len(input.Children) == 0:
		errs["children"] = errors.New("at least one child is required")
	case len(input.Children) > enrollment.HolidayRegistrationMaxChildren:
		errs["children"] = fmt.Errorf("at most %d children", enrollment.HolidayRegistrationMaxChildren)
	default:
		childErrs := validateHolidayChildren(input.Children, schools, today)
		addHolidayBookingErrors(childErrs, input.Children, period)
		addHolidayChildAnswerErrors(childErrs, form, answers, byKey)
		if len(childErrs) > 0 {
			errs["children"] = childErrs
		}
	}
	if len(input.EmergencyContacts) == 0 {
		errs["emergency_contacts"] = errors.New("at least one emergency contact is required")
	} else if contactErrs := validateHolidayContacts(input.EmergencyContacts, enrollment.HolidayRegistrationMaxEmergencyContacts, true); contactErrs != nil {
		errs["emergency_contacts"] = contactErrs
	}
	if contactErrs := validateHolidayContacts(input.PickupPersons, enrollment.HolidayRegistrationMaxPickupPersons, false); contactErrs != nil {
		errs["pickup_persons"] = contactErrs
	}
	validateHolidayConsents(input, period, form, answers, errs)
	validateHolidayProofTokens(input, errs)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// addHolidayBookingErrors adds the refused bookings of each child below its
// other errors, as "children.<i>.bookings". The Träger-Büro's edit of the
// details checks no bookings: only a rebooking changes them (#5629).
func addHolidayBookingErrors(childErrs validation.Errors, children []enrollment.HolidayChildInput, period enrollment.HolidayPeriod) {
	for i, child := range children {
		if bookingErr := validateHolidayBookings(child.Bookings, period, nil); bookingErr != nil {
			nestHolidayChildError(childErrs, i, "bookings", bookingErr)
		}
	}
}

// nestHolidayChildError adds err as "children.<i>.<field>".
func nestHolidayChildError(childErrs validation.Errors, i int, field string, err error) {
	key := fmt.Sprint(i)
	nested, _ := childErrs[key].(validation.Errors)
	if nested == nil {
		nested = validation.Errors{}
	}
	nested[field] = err
	childErrs[key] = nested
}

// addHolidayChildAnswerErrors adds the refused answers of each child below
// its other errors, as "children.<i>.custom_data.<key>".
func addHolidayChildAnswerErrors(childErrs validation.Errors, form enrollment.HolidayPeriodForm, answers holidayAnswerSet, byKey map[string]*enrollment.FormField) {
	for i, child := range answers.children {
		answerErrs := validateHolidayAnswers(form, true, child, fieldVisibilityContext{
			guardianAnswers: answers.family, childAnswers: child, fieldsByKey: byKey,
		})
		if len(answerErrs) > 0 {
			nestHolidayChildError(childErrs, i, "custom_data", answerErrs)
		}
	}
}

func validateHolidayConsents(input enrollment.HolidayRegistrationInput, period enrollment.HolidayPeriod, form enrollment.HolidayPeriodForm, answers holidayAnswerSet, errs validation.Errors) {
	if !input.PrivacyConsent {
		errs["privacy_consent"] = errors.New("the consent to the data processing is required")
	}
	if holidayRegistrationHasHealth(input, form, answers) && !input.HealthConsent {
		errs["health_consent"] = errors.New("the consent to the health information is required")
	}
	if period.Binding && !input.BindingConfirmed {
		errs["binding_confirmed"] = errors.New("the binding registration must be confirmed")
	}
	// Every new registration gives the declarations of the current version
	// (#5626): the custody and the typed name. The texts a parent agreed to
	// must be named exactly, like the version of the form.
	if !input.CustodyConfirmed {
		errs["custody_confirmed"] = errors.New("the custody must be confirmed")
	}
	requireHolidayText(errs, "declaration_name", input.DeclarationName, enrollment.HolidayDeclarationNameMaxLength)
	if input.ConsentVersion != enrollment.HolidayConsentVersionCurrent {
		errs["consent_version"] = errors.New("the consent texts of this version are not the current ones")
	}
}

func validateHolidayGuardian(guardian enrollment.HolidayGuardianInput) validation.Errors {
	errs := validation.Errors{}
	requireHolidayText(errs, "first_name", guardian.FirstName, holidayPersonNameMaxLength)
	requireHolidayText(errs, "last_name", guardian.LastName, holidayPersonNameMaxLength)
	requireHolidayText(errs, "phone", guardian.Phone, holidayPhoneMaxLength)
	if guardian.Email == "" {
		errs["email"] = errors.New("e-mail address is required")
	} else if parsed, err := mail.ParseAddress(guardian.Email); err != nil || parsed.Address != guardian.Email ||
		utf8.RuneCountInString(guardian.Email) > holidayEmailMaxLength {
		errs["email"] = errors.New("e-mail address is invalid")
	}
	return errs
}

// validateHolidayChildren checks the details of each child: names, date of
// birth, school and the lengths of the free texts. The bookings are checked
// apart (addHolidayBookingErrors), so the Träger-Büro's edit of the details
// reuses this check without them (#5629).
func validateHolidayChildren(children []enrollment.HolidayChildInput, schools []enrollment.HolidaySchool, today calendar.Date) validation.Errors {
	errs := validation.Errors{}
	known := map[int64]bool{}
	for _, school := range schools {
		known[school.ID] = true
	}
	for i, child := range children {
		childErrs := validation.Errors{}
		requireHolidayText(childErrs, "first_name", child.FirstName, holidayPersonNameMaxLength)
		requireHolidayText(childErrs, "last_name", child.LastName, holidayPersonNameMaxLength)
		if child.BirthDate.IsZero() {
			childErrs["birth_date"] = errors.New("date of birth is required")
		} else if !child.BirthDate.Before(today) || child.BirthDate.Before(today.AddDays(-366*holidayChildMaxAgeYears)) {
			childErrs["birth_date"] = errors.New("date of birth is not plausible")
		}
		if child.HomeSchoolID < 0 || (child.HomeSchoolID > 0 && !known[child.HomeSchoolID]) {
			childErrs["home_school_id"] = errors.New("the school is not a school of the Träger")
		}
		limitHolidayText(childErrs, "guest_school_name", child.GuestSchoolName, holidayGuestSchoolMaxLength)
		limitHolidayText(childErrs, "allergies", child.Allergies, holidayHealthFieldMaxLength)
		limitHolidayText(childErrs, "medication", child.Medication, holidayHealthFieldMaxLength)
		limitHolidayText(childErrs, "health_notes", child.HealthNotes, holidayHealthNotesMaxLength)
		if len(childErrs) > 0 {
			errs[fmt.Sprint(i)] = childErrs
		}
	}
	return errs
}

// validateHolidayBookings requires at least one week, each week of the
// period at most once and a time model of that week. A child takes part in
// only one of the weeks that share a day (#5689). An archived time model
// takes no new booking; a booking of kept, the child's bookings so far, may
// stay on it (#5994).
func validateHolidayBookings(bookings []enrollment.HolidayBookingInput, period enrollment.HolidayPeriod, kept []enrollment.HolidayBookingInput) error {
	if len(bookings) == 0 {
		return errors.New("at least one week is required")
	}
	weeks := map[int64]enrollment.HolidayWeek{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	errs := validation.Errors{}
	seen := map[int64]bool{}
	for i, booking := range bookings {
		week, ok := weeks[booking.WeekID]
		switch {
		case !ok:
			errs[fmt.Sprint(i)] = errors.New("the week is not a week of this period")
		case seen[booking.WeekID]:
			errs[fmt.Sprint(i)] = errors.New("the week is booked twice")
		case !holidayWeekOffers(week, booking.TimeModelID, slices.Contains(kept, booking)):
			errs[fmt.Sprint(i)] = errors.New("the time model is not a bookable time model of this week")
		case holidayBookingOverlaps(week, bookings[:i], weeks):
			errs[fmt.Sprint(i)] = errors.New("the week shares a day with another booked week")
		}
		seen[booking.WeekID] = true
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func holidayWeekOffers(week enrollment.HolidayWeek, timeModelID int64, kept bool) bool {
	return slices.ContainsFunc(week.TimeModels, func(model enrollment.HolidayTimeModel) bool {
		return model.ID == timeModelID && (!model.Archived || kept)
	})
}

// holidayBookingOverlaps reports whether week shares a day with a week of the
// earlier bookings. A booking of an unknown week is refused on its own.
func holidayBookingOverlaps(week enrollment.HolidayWeek, earlier []enrollment.HolidayBookingInput, weeks map[int64]enrollment.HolidayWeek) bool {
	return slices.ContainsFunc(earlier, func(booking enrollment.HolidayBookingInput) bool {
		other, ok := weeks[booking.WeekID]
		return ok && other.ID != week.ID && holidayDaysOverlap(week.StartsOn, week.EndsOn, other.StartsOn, other.EndsOn)
	})
}

func validateHolidayContacts(contacts []enrollment.HolidayContactInput, maxContacts int, phoneRequired bool) error {
	if len(contacts) > maxContacts {
		return fmt.Errorf("at most %d entries", maxContacts)
	}
	errs := validation.Errors{}
	for i, contact := range contacts {
		contactErrs := validation.Errors{}
		requireHolidayText(contactErrs, "name", contact.Name, holidayContactNameMaxLength)
		if phoneRequired {
			requireHolidayText(contactErrs, "phone", contact.Phone, holidayPhoneMaxLength)
		} else {
			limitHolidayText(contactErrs, "phone", contact.Phone, holidayPhoneMaxLength)
		}
		limitHolidayText(contactErrs, "relationship", contact.Relationship, holidayRelationshipMaxLength)
		if len(contactErrs) > 0 {
			errs[fmt.Sprint(i)] = contactErrs
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func requireHolidayText(errs validation.Errors, field, value string, maxLength int) {
	if value == "" {
		errs[field] = errors.New("is required")
		return
	}
	limitHolidayText(errs, field, value, maxLength)
}

func limitHolidayText(errs validation.Errors, field, value string, maxLength int) {
	if utf8.RuneCountInString(value) > maxLength {
		errs[field] = fmt.Errorf("is longer than %d characters", maxLength)
	}
}

// HolidayRegistrationRetentionStore deletes ended registrations across every
// Träger in the administrative transaction.
type HolidayRegistrationRetentionStore interface {
	// DeleteHolidayRegistrationsEndedBefore deletes the registrations of the
	// periods whose last week ended before cutoff, with their children,
	// bookings and contacts, and the abuse counters whose window started
	// before counterRetention.
	DeleteHolidayRegistrationsEndedBefore(ctx context.Context, cutoff calendar.Date, counterRetention time.Duration) (int, error)
	// DeleteExpiredHolidayRegistrationDeletions deletes the rows of the
	// deletion log older than years.
	DeleteExpiredHolidayRegistrationDeletions(ctx context.Context, years int) (int, error)
	// DeleteUnclaimedHolidayProofs deletes the proofs no registration
	// claimed that were uploaded before createdBefore (#5238).
	DeleteUnclaimedHolidayProofs(ctx context.Context, createdBefore time.Time) (int, error)
}

// HolidayRegistrationRetention serves enrollment.HolidayRegistrationRetention.
type HolidayRegistrationRetention struct {
	store HolidayRegistrationRetentionStore
	today func() calendar.Date
	now   func() time.Time
}

// NewHolidayRegistrationRetention builds the retention over store; a nil
// today uses the Berlin calendar day.
func NewHolidayRegistrationRetention(store HolidayRegistrationRetentionStore, today func() calendar.Date) *HolidayRegistrationRetention {
	if store == nil {
		panic("enrollment: the holiday registration retention store is required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayRegistrationRetention{store: store, today: today, now: time.Now}
}

// DeleteExpiredHolidayRegistrations deletes the registrations of every
// period whose last week ended more than the retention window ago, with an
// entry in the deletion log of its Träger, the expired entries of that log
// and the proofs no registration claimed within a day (#5238). The claimed
// proofs go with their registration.
func (r *HolidayRegistrationRetention) DeleteExpiredHolidayRegistrations(ctx context.Context) (int, error) {
	cutoff := r.today().AddDays(-enrollment.HolidayRegistrationRetentionDays)
	deleted, err := r.store.DeleteHolidayRegistrationsEndedBefore(ctx, cutoff, rateLimitRetention)
	if err != nil {
		return 0, fmt.Errorf("enrollment: delete ended holiday registrations: %w", err)
	}
	logged, err := r.store.DeleteExpiredHolidayRegistrationDeletions(ctx, enrollment.HolidayRegistrationDeletionLogRetentionYears)
	if err != nil {
		return 0, fmt.Errorf("enrollment: delete expired holiday registration deletion log: %w", err)
	}
	proofs, err := r.store.DeleteUnclaimedHolidayProofs(ctx, r.now().Add(-enrollment.UnclaimedUploadLifetime))
	if err != nil {
		return 0, fmt.Errorf("enrollment: delete unclaimed holiday proofs: %w", err)
	}
	return deleted + logged + proofs, nil
}
