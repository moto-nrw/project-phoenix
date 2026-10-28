package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
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

// The abuse counters of the public link follow the online registration.
const (
	holidayRateLimitKeyIP    = "ip"
	holidayRateLimitKeyEmail = "email"
)

// HolidayRegistrationStore persists the public registrations of a Träger.
type HolidayRegistrationStore interface {
	// FindHolidayRegistrationLink reads the Träger and the published period
	// of token in the administrative transaction.
	FindHolidayRegistrationLink(ctx context.Context, token string) (organizationID, periodID int64, found bool, err error)
	// CountHolidayRegistrationAttempt counts one attempt in the bucket of
	// value for the Träger and returns the attempts of its running window.
	// The bucket keeps only a SHA-256 digest of value.
	CountHolidayRegistrationAttempt(ctx context.Context, organizationID int64, keyType, value string, window time.Duration) (int, error)
	InsertHolidayRegistration(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationReceipt, error)
}

// HolidayRegistrationDependencies are the ports of the public registration.
type HolidayRegistrationDependencies struct {
	Registrations HolidayRegistrationStore
	Periods       HolidayPeriodStore
	Traeger       enrollment.HolidayTraegerDirectory
	Mail          enrollment.TraegerMailOutbox
	// Today is the calendar day the registration window is checked against.
	Today  func() calendar.Date
	Logger *slog.Logger
}

// HolidayRegistrations serves enrollment.HolidayRegistrations.
type HolidayRegistrations struct {
	deps HolidayRegistrationDependencies
}

var _ enrollment.HolidayRegistrations = (*HolidayRegistrations)(nil)

// NewHolidayRegistrations builds the public registration over deps.
func NewHolidayRegistrations(deps HolidayRegistrationDependencies) *HolidayRegistrations {
	if deps.Registrations == nil || deps.Periods == nil || deps.Traeger == nil || deps.Mail == nil {
		panic("enrollment: the holiday registration store, periods, Träger directory and mail outbox are required")
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &HolidayRegistrations{deps: deps}
}

// ResolveHolidayRegistrationLink names the Träger, the period and the
// Träger's schools of a published period's link.
func (h *HolidayRegistrations) ResolveHolidayRegistrationLink(ctx context.Context, token string) (enrollment.HolidayRegistrationLink, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return enrollment.HolidayRegistrationLink{}, enrollment.ErrHolidayRegistrationNotFound
	}
	organizationID, periodID, found, err := h.deps.Registrations.FindHolidayRegistrationLink(ctx, token)
	if err != nil {
		return enrollment.HolidayRegistrationLink{}, fmt.Errorf("enrollment: resolve holiday registration link: %w", err)
	}
	if !found {
		return enrollment.HolidayRegistrationLink{}, enrollment.ErrHolidayRegistrationNotFound
	}
	traeger, err := h.deps.Traeger.HolidayTraeger(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayRegistrationLink{}, err
	}
	return enrollment.HolidayRegistrationLink{OrganizationID: organizationID, PeriodID: periodID, Traeger: traeger}, nil
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
	today := h.deps.Today()
	return enrollment.HolidayRegistrationOffer{
		Name: period.Name, RegistrationOpensOn: period.RegistrationOpensOn, RegistrationClosesOn: period.RegistrationClosesOn,
		Binding: period.Binding, Notes: period.Notes, Weeks: period.Weeks,
		Open: !today.Before(period.RegistrationOpensOn) && !today.After(period.RegistrationClosesOn),
	}, nil
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
	buckets := []struct {
		keyType string
		value   string
		window  time.Duration
		max     int
	}{
		{holidayRateLimitKeyIP, strings.TrimSpace(remoteIP), rateLimitWindowIP, rateLimitMaxAttemptsIP},
		{holidayRateLimitKeyEmail, strings.ToLower(strings.TrimSpace(email)), rateLimitWindowEmail, rateLimitMaxAttemptsMail},
	}
	for _, bucket := range buckets {
		if bucket.value == "" {
			continue
		}
		attempts, err := h.deps.Registrations.CountHolidayRegistrationAttempt(ctx, organizationID, bucket.keyType, bucket.value, bucket.window)
		if err != nil {
			return fmt.Errorf("enrollment: count holiday registration attempt: %w", err)
		}
		if attempts > bucket.max {
			h.deps.Logger.Info("holiday registration rate-limited",
				slog.String("key_type", bucket.keyType),
				slog.Int("attempts", attempts),
				slog.Int64("organization_id", organizationID))
			return enrollment.ErrRateLimited
		}
	}
	return nil
}

// SubmitHolidayRegistration checks and stores a registration and enqueues
// its receipt mail in the same Träger transaction.
func (h *HolidayRegistrations) SubmitHolidayRegistration(ctx context.Context, link enrollment.HolidayRegistrationLink, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationReceipt, error) {
	period, err := h.publishedPeriod(ctx, link)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	today := h.deps.Today()
	if today.Before(period.RegistrationOpensOn) || today.After(period.RegistrationClosesOn) {
		return enrollment.HolidayRegistrationReceipt{}, enrollment.ErrHolidayRegistrationClosed
	}
	input = normalizeHolidayRegistration(input)
	if err := validateHolidayRegistration(input, period, link.Traeger.Schools, today); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if !holidayRegistrationHasHealth(input) {
		input.HealthConsent = false
	}
	receipt, err := h.deps.Registrations.InsertHolidayRegistration(ctx, link.OrganizationID, link.PeriodID, input)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, fmt.Errorf("enrollment: store holiday registration: %w", err)
	}
	if err := h.enqueueReceipt(ctx, link, period, input, receipt); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	return receipt, nil
}

func (h *HolidayRegistrations) enqueueReceipt(ctx context.Context, link enrollment.HolidayRegistrationLink, period enrollment.HolidayPeriod, input enrollment.HolidayRegistrationInput, receipt enrollment.HolidayRegistrationReceipt) error {
	payload, err := json.Marshal(holidayReceiptMail(link.Traeger, period, input))
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
// host school and time model. It names no date of birth and no health
// information.
func holidayReceiptMail(traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod, input enrollment.HolidayRegistrationInput) enrollment.HolidayRegistrationMail {
	schools := map[int64]string{}
	for _, school := range traeger.Schools {
		schools[school.ID] = school.Name
	}
	weeks := map[int64]enrollment.HolidayWeek{}
	models := map[int64]enrollment.HolidayTimeModel{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
		for _, model := range week.TimeModels {
			models[model.ID] = model
		}
	}
	children := make([]enrollment.HolidayRegistrationMailChild, 0, len(input.Children))
	for _, child := range input.Children {
		lines := make([]string, 0, len(child.Bookings))
		for _, week := range period.Weeks {
			for _, booking := range child.Bookings {
				if booking.WeekID != week.ID {
					continue
				}
				model := models[booking.TimeModelID]
				line := fmt.Sprintf("%s bis %s: %s, %s (%s bis %s Uhr)", weeks[week.ID].StartsOn.Format("02.01.2006"),
					weeks[week.ID].EndsOn.Format("02.01.2006"), schools[week.HostSchoolID], model.Name, model.StartsAt, model.EndsAt)
				lines = append(lines, line)
			}
		}
		children = append(children, enrollment.HolidayRegistrationMailChild{
			Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == 0, Weeks: lines,
		})
	}
	return enrollment.HolidayRegistrationMail{
		Recipient: input.Guardian.Email, GuardianName: input.Guardian.FirstName + " " + input.Guardian.LastName,
		TraegerName: traeger.Name, PeriodName: period.Name, Binding: period.Binding, Children: children,
	}
}

func normalizeHolidayRegistration(input enrollment.HolidayRegistrationInput) enrollment.HolidayRegistrationInput {
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

func holidayRegistrationHasHealth(input enrollment.HolidayRegistrationInput) bool {
	for _, child := range input.Children {
		if child.Allergies != "" || child.Medication != "" || child.HealthNotes != "" {
			return true
		}
	}
	return false
}

// validateHolidayRegistration checks a registration against the period and
// the Träger's schools. Field keys follow the request body, such as
// "children.1.bookings".
func validateHolidayRegistration(input enrollment.HolidayRegistrationInput, period enrollment.HolidayPeriod, schools []enrollment.HolidaySchool, today calendar.Date) error {
	errs := validation.Errors{}
	if guardianErrs := validateHolidayGuardian(input.Guardian); len(guardianErrs) > 0 {
		errs["guardian"] = guardianErrs
	}
	if input.SecondGuardian != nil {
		if guardianErrs := validateHolidaySecondGuardian(*input.SecondGuardian); len(guardianErrs) > 0 {
			errs["second_guardian"] = guardianErrs
		}
	}
	switch {
	case len(input.Children) == 0:
		errs["children"] = errors.New("at least one child is required")
	case len(input.Children) > enrollment.HolidayRegistrationMaxChildren:
		errs["children"] = fmt.Errorf("at most %d children", enrollment.HolidayRegistrationMaxChildren)
	default:
		if childErrs := validateHolidayChildren(input.Children, period, schools, today); len(childErrs) > 0 {
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
	validateHolidayConsents(input, period, errs)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateHolidayConsents(input enrollment.HolidayRegistrationInput, period enrollment.HolidayPeriod, errs validation.Errors) {
	if !input.PrivacyConsent {
		errs["privacy_consent"] = errors.New("the consent to the data processing is required")
	}
	if holidayRegistrationHasHealth(input) && !input.HealthConsent {
		errs["health_consent"] = errors.New("the consent to the health information is required")
	}
	if period.Binding && !input.BindingConfirmed {
		errs["binding_confirmed"] = errors.New("the binding registration must be confirmed")
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

func validateHolidayChildren(children []enrollment.HolidayChildInput, period enrollment.HolidayPeriod, schools []enrollment.HolidaySchool, today calendar.Date) validation.Errors {
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
		if bookingErr := validateHolidayBookings(child.Bookings, period); bookingErr != nil {
			childErrs["bookings"] = bookingErr
		}
		if len(childErrs) > 0 {
			errs[fmt.Sprint(i)] = childErrs
		}
	}
	return errs
}

// validateHolidayBookings requires at least one week, each week of the
// period at most once and a time model of that week.
func validateHolidayBookings(bookings []enrollment.HolidayBookingInput, period enrollment.HolidayPeriod) error {
	if len(bookings) == 0 {
		return errors.New("at least one week is required")
	}
	weeks := map[int64]map[int64]bool{}
	for _, week := range period.Weeks {
		models := map[int64]bool{}
		for _, model := range week.TimeModels {
			models[model.ID] = true
		}
		weeks[week.ID] = models
	}
	errs := validation.Errors{}
	seen := map[int64]bool{}
	for i, booking := range bookings {
		models, ok := weeks[booking.WeekID]
		switch {
		case !ok:
			errs[fmt.Sprint(i)] = errors.New("the week is not a week of this period")
		case seen[booking.WeekID]:
			errs[fmt.Sprint(i)] = errors.New("the week is booked twice")
		case !models[booking.TimeModelID]:
			errs[fmt.Sprint(i)] = errors.New("the time model is not a time model of this week")
		}
		seen[booking.WeekID] = true
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
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
	// bookings and contacts, and the expired abuse counters.
	DeleteHolidayRegistrationsEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error)
	// DeleteExpiredHolidayRegistrationDeletions deletes the rows of the
	// deletion log older than years.
	DeleteExpiredHolidayRegistrationDeletions(ctx context.Context, years int) (int, error)
}

// HolidayRegistrationRetention serves enrollment.HolidayRegistrationRetention.
type HolidayRegistrationRetention struct {
	store HolidayRegistrationRetentionStore
	today func() calendar.Date
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
	return &HolidayRegistrationRetention{store: store, today: today}
}

// DeleteExpiredHolidayRegistrations deletes the registrations of every
// period whose last week ended more than the retention window ago, with an
// entry in the deletion log of its Träger, and the expired entries of that
// log.
func (r *HolidayRegistrationRetention) DeleteExpiredHolidayRegistrations(ctx context.Context) (int, error) {
	cutoff := r.today().AddDays(-enrollment.HolidayRegistrationRetentionDays)
	deleted, err := r.store.DeleteHolidayRegistrationsEndedBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("enrollment: delete ended holiday registrations: %w", err)
	}
	logged, err := r.store.DeleteExpiredHolidayRegistrationDeletions(ctx, enrollment.HolidayRegistrationDeletionLogRetentionYears)
	if err != nil {
		return 0, fmt.Errorf("enrollment: delete expired holiday registration deletion log: %w", err)
	}
	return deleted + logged, nil
}
