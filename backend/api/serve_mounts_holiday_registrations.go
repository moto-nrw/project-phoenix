package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	apiCommon "github.com/moto-ogs/project-phoenix/api/common"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/observability"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/workflows/parentportal"
	parentAPI "github.com/moto-ogs/project-phoenix/workflows/parentportal/http"
)

// holidayRegistrationPath is the public holiday registration and
// holidayQuoteMount the contribution its summary shows. The general request
// limiter leaves both to holidayRegistrationRateLimit.
const (
	holidayRegistrationPath = "/traeger/holiday-registrations"
	holidayQuoteMount       = "/traeger/holiday-quotes"
)

// mountTraegerRoutes serves the Träger-Portal at /traeger, the public
// holiday registration (#4661, ADR 0110) at /traeger/holiday-registrations: the one
// link of a published holiday period, without an account. Turnstile is
// always required there. The Elternabrechnung (#4664) and the
// Betreuungsvertrag (#4678) with its evaluation of overruns (#4683) and the
// office's decision of the parents' changes (#5979) mount beside them.
func mountTraegerRoutes(root, tenant chi.Router, limiters authRateLimiters, identity TraegerPortalIdentity, organizations organizationtenancy.Capability,
	persons TraegerPersons, staff TraegerStaff, audit TraegerAudit, settings TraegerSettings, schoolSettings TraegerSchoolSettings, attendanceMarks owners.MealAttendanceMarks,
	delivery deliveryRuntime.Capability, closureMail owners.HolidayClosureMail, outcomes deliveryRuntime.EmailOutcomes, keys TurnstileKeys,
	healthData enrollmentOwner.HolidayHealthCipher, bankData parentbilling.BankDataCipher, statistics studentpresence.StatisticsReports, changes careplan.OfferingChangeCapability, contractHolidays carecontract.SchoolHolidays,
	holidayCalendar enrollmentOwner.HolidaySchoolCalendar, closureHolidays owners.HolidayClosureHolidays, calendarSchools traegercalendar.Schools, schoolPhases *enrollmentCompose.Phases, schoolCatalog careplan.CareOfferingCatalogCapability, lockRecurrence func(context.Context) error,
	parentsURL string, logger *slog.Logger) error {
	if _, ok := identity.(identityaccess.TraegerMemberProfiles); !ok {
		return errors.New("traeger portal: identity does not serve the names of the Träger-Büro")
	}
	if err := mountTraegerPortalRoutes(root, limiters, identity, organizations, persons, staff, audit, settings, healthData, holidayCalendar, holidayTraegerMail{delivery: delivery}, holidayMailStates{mails: closureMail.TraegerMails}, newHolidayOfficeTraeger(organizations)); err != nil {
		return err
	}
	setups, ok := identity.(identityaccess.HolidayParentSetups)
	if !ok {
		return errors.New("holiday registration: identity does not serve the setup link of a parent account")
	}
	accountLinks := owners.NewHolidayParentAccountLinks(setups, parentsURL)
	publicLink := root
	if limit := holidayRegistrationRateLimit(limiters.enabled(), demoLoopbackExempt()); limit != nil {
		publicLink = root.With(limit)
	}
	publicLink.Mount(holidayRegistrationPath, NewHolidayRegistrationHTTP(organizations, delivery, keys, "", healthData, holidayCalendar, settings, accountLinks, viper.GetString("app_env"), logger).Router())
	mountTraegerClosingDayRoutes(root, calendarSchools, organizations, closureMail, holidayCalendar, closureHolidays)
	mountHolidayClosureNotificationRoutes(root, tenant, organizations, closureMail, holidayCalendar, closureHolidays, logger)
	mountParentBillingRoutes(root, publicLink, organizations, settings, schoolSettings, attendanceMarks, delivery, outcomes, healthData, bankData, holidayCalendar, logger)
	mountCareContractRoutes(root, organizations, settings, schoolSettings, delivery, outcomes, audit.Log,
		owners.NewCareContractNoticeAddressees(persons, identity), contractHolidays, parentsURL)
	mountCareTimeOverrunRoutes(root, organizations, statistics, audit.Log)
	// The office decides the parents' changes through Care Plan's review of
	// the school (#5979).
	mountCareContractChangeRoutes(root, organizations, changes, schoolSettings, audit.Log)
	// The office rolls its phases and offerings out into its schools (#4704).
	billing, intake := newTraegerCatalogBilling(organizations, settings, schoolSettings, attendanceMarks, delivery, outcomes, audit.Log, contractHolidays, parentsURL)
	return mountTraegerCatalogRoutes(root, schoolPhases, schoolCatalog, lockRecurrence, organizations, billing, intake)
}

// TraegerSettings is the Träger level of the settings system: the
// settings of the Träger-Portal and the branding the letters print.
type TraegerSettings interface {
	TraegerPortalSettings
	TraegerBrandingSettings
}

// HolidayOrganizations is Organisation & Tenancy as the holiday registration
// reads the Träger of a link.
type HolidayOrganizations interface {
	FindOrganization(context.Context, int64) (organizationtenancy.Organization, error)
	ListSchoolsByOrganization(context.Context, int64) ([]organizationtenancy.School, error)
}

// NewHolidayRegistrationHTTP binds the public holiday registration to
// Organisation & Tenancy, Delivery and Turnstile; healthData seals the health
// information (#4722) and the statements of the holiday care wish,
// schoolCalendar names the host schools' days without care (#4702), settings
// reads the Träger's holiday wish mode (#4703), accountLinks issues the
// setup link the local seeder receives in the answer of a registration that
// asks for a parent account (ADR 0216); the offer mail issues its own when
// it is sent. An empty
// verifyURL uses Turnstile's endpoint; tests point it at a local provider.
func NewHolidayRegistrationHTTP(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, keys TurnstileKeys, verifyURL string, healthData enrollmentOwner.HolidayHealthCipher, schoolCalendar enrollmentOwner.HolidaySchoolCalendar, settings TraegerBrandingSettings, accountLinks enrollmentOwner.HolidayParentAccountLinks, appEnv string, logger *slog.Logger) *enrollmentAPI.HolidayRegistrationResource {
	registrations := newHolidayRegistrations(organizations, delivery, healthData, schoolCalendar, settings, logger.With("service", "holiday-registrations"))
	captcha := enrollmentCompose.NewCaptcha(holidayCaptchaSettings(keys), verifyURL, logger.With("service", "holiday-registration-captcha"))
	// The local seeder alone receives a setup link for the offered parent
	// account (ADR 0216), like the invitation tokens it redeems.
	return enrollmentAPI.NewHolidayRegistrationResource(registrations, holidayProofUnits{registrations: registrations}, captcha,
		func(r *http.Request) bool { return apiCommon.ShouldExposeSeedInvitationToken(r, appEnv) }, accountLinks)
}

// holidayCaptchaSettings always require the captcha with the platform's
// Turnstile keys: a Träger has no setting to switch it off yet (ADR 0110).
type holidayCaptchaSettings TurnstileKeys

func (s holidayCaptchaSettings) CaptchaRequired(context.Context) (bool, error) { return true, nil }

func (s holidayCaptchaSettings) CaptchaSecretKey(context.Context) (string, error) {
	return s.SecretKey, nil
}

func (s holidayCaptchaSettings) CaptchaSiteKey(context.Context) (string, error) {
	return s.SiteKey, nil
}

// holidayTraegerDirectory reads the Träger of a link and its undeleted
// schools. A deactivated, deleted or unknown Träger has no open link.
type holidayTraegerDirectory struct {
	organizations HolidayOrganizations
}

func (d holidayTraegerDirectory) HolidayTraeger(ctx context.Context, organizationID int64) (enrollmentOwner.HolidayTraeger, error) {
	organization, err := d.organizations.FindOrganization(ctx, organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationNotFound) || (err == nil && (!organization.Active || organization.IsDeleted())) {
		return enrollmentOwner.HolidayTraeger{}, enrollmentOwner.ErrHolidayRegistrationNotFound
	}
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	schools, err := d.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	traeger := enrollmentOwner.HolidayTraeger{Name: organization.Name, Schools: []enrollmentOwner.HolidaySchool{}}
	for _, school := range schools {
		if !school.IsDeleted() {
			traeger.Schools = append(traeger.Schools, enrollmentOwner.HolidaySchool{ID: school.ID, Name: school.Name})
		}
	}
	return traeger, nil
}

// holidayTraegerMail enqueues the receipt of a holiday registration as a
// mail of the Träger in the caller's Träger transaction.
type holidayTraegerMail struct {
	delivery deliveryRuntime.Capability
}

func (m holidayTraegerMail) EnqueueTraegerMail(ctx context.Context, mail enrollmentOwner.TraegerMail) error {
	_, err := m.delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: mail.Kind, Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient},
		Payload: mail.Payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: mail.RelatedType, ID: mail.RelatedID},
	})
	return err
}

// holidayMailStates reads what became of the Träger's mails of the holiday
// registrations, by registration and by idempotency key, in the caller's
// Träger transaction (#6737, #6976), and names those that did not reach the
// parents in Enrollment's words.
type holidayMailStates struct {
	mails deliveryRuntime.TraegerMails
}

func (s holidayMailStates) HolidayMails(ctx context.Context, organizationID int64, registrationIDs []int64, keys []string) ([]enrollmentOwner.HolidayMailState, error) {
	if s.mails == nil {
		return nil, errors.New("holiday registration: the Träger mails of Delivery are not configured")
	}
	statuses, err := s.mails.TraegerEmailStatusesOf(ctx, organizationID, enrollmentOwner.MailRelatedHolidayRegistration, registrationIDs, keys)
	if err != nil {
		return nil, err
	}
	states := make([]enrollmentOwner.HolidayMailState, 0, len(statuses))
	for _, status := range statuses {
		state := enrollmentOwner.HolidayMailState{Kind: status.Kind, Outcome: string(status.Outcome), Problem: holidayMailProblem(status.Outcome)}
		if status.RelatedID != nil {
			state.RegistrationID = *status.RelatedID
		}
		if status.IdempotencyKey != nil {
			state.IdempotencyKey = *status.IdempotencyKey
		}
		states = append(states, state)
	}
	return states, nil
}

// holidayMailProblem names an outcome of a mail that did not reach the
// parents; a mail on its way, accepted, delivered or withdrawn has none.
func holidayMailProblem(outcome deliveryContract.EmailOutcome) string {
	switch outcome {
	case deliveryContract.EmailSendFailed, deliveryContract.EmailUndeliverable:
		return enrollmentOwner.HolidayMailFailed
	case deliveryContract.EmailBlocked, deliveryContract.EmailProviderBlocked, deliveryContract.EmailHeldByStop,
		deliveryContract.EmailComplained, deliveryContract.EmailUnsubscribed:
		return enrollmentOwner.HolidayMailBlocked
	case deliveryContract.EmailOutcomeUnknown, deliveryContract.EmailOverdue:
		return enrollmentOwner.HolidayMailUnknown
	default:
		return ""
	}
}

// parentHolidayRegistrationLinks resolves the link of a holiday period for
// the parents portal's prefill (#4662, ADR 0113): the Träger and its
// undeleted schools, in the caller's administrative transaction.
type parentHolidayRegistrationLinks struct {
	registrations enrollmentOwner.HolidayRegistrations
}

// newParentHolidayRegistrationLinks binds the parents portal to Enrollment's
// link resolution over the same Träger directory as the public link.
func newParentHolidayRegistrationLinks(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, healthData enrollmentOwner.HolidayHealthCipher, settings TraegerBrandingSettings, logger *slog.Logger) parentAPI.HolidayRegistrationLinks {
	return parentHolidayRegistrationLinks{registrations: newHolidayRegistrations(organizations, delivery, healthData, nil, settings, logger.With("service", "parent-holiday-registrations"))}
}

// newHolidayRegistrations binds Enrollment's holiday registrations to the
// Träger directory, the Träger's mail and Security Runtime's entropy and
// fingerprint for the tokens of uploaded proofs (#5238), and the health
// information to the root's cipher (#4722).
func newHolidayRegistrations(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, healthData enrollmentOwner.HolidayHealthCipher, schoolCalendar enrollmentOwner.HolidaySchoolCalendar, settings TraegerBrandingSettings, logger *slog.Logger) enrollmentOwner.HolidayRegistrations {
	return enrollmentCompose.NewHolidayRegistrations(holidayTraegerDirectory{organizations: organizations},
		holidayTraegerMail{delivery: delivery}, healthData, schoolCalendar, holidayWishModes{settings: settings}, holidayRetentionSettings{settings: settings}, securityruntime.FillRandom, securityruntime.Fingerprint, logger, observability.RecordRateLimitRejection)
}

// holidayWishModes binds Enrollment's holiday wish mode (#4703) to the
// Träger level of the settings system, in the caller's Träger transaction.
type holidayWishModes struct {
	settings TraegerBrandingSettings
}

func (m holidayWishModes) HolidayWishMode(ctx context.Context, organizationID int64) (string, error) {
	if m.settings == nil {
		return "", errors.New("holiday registration: the Träger settings are not configured")
	}
	texts, err := m.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyHolidayRegistrationWishMode})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyHolidayRegistrationWishMode], nil
}

// holidayRetentionSettings binds Enrollment's retention of the holiday
// registrations (#6691) to the Träger level of the settings system, in the
// caller's Träger transaction.
type holidayRetentionSettings struct {
	settings TraegerBrandingSettings
}

func (s holidayRetentionSettings) HolidayRegistrationRetentionDays(ctx context.Context, organizationID int64) (int, error) {
	if s.settings == nil {
		return 0, errors.New("holiday registration: the Träger settings are not configured")
	}
	days, err := s.settings.TraegerInts(ctx, organizationID, []string{configModels.KeyHolidayRegistrationRetentionDays})
	if err != nil {
		return 0, err
	}
	return days[configModels.KeyHolidayRegistrationRetentionDays], nil
}

func (l parentHolidayRegistrationLinks) ResolveHolidayRegistrationSchools(ctx context.Context, token string) (parentportal.HolidayRegistrationScope, error) {
	link, err := l.registrations.ResolveHolidayRegistrationLink(ctx, token)
	if errors.Is(err, enrollmentOwner.ErrHolidayRegistrationNotFound) {
		return parentportal.HolidayRegistrationScope{}, parentAPI.ErrHolidayRegistrationLinkNotFound
	}
	if err != nil {
		return parentportal.HolidayRegistrationScope{}, err
	}
	schoolIDs := make([]int64, 0, len(link.Traeger.Schools))
	for _, school := range link.Traeger.Schools {
		schoolIDs = append(schoolIDs, school.ID)
	}
	return parentportal.HolidayRegistrationScope{
		OrganizationID: link.OrganizationID, PeriodID: link.PeriodID, SchoolIDs: schoolIDs,
		HolidayWishRule: link.HolidayWishRule, FirstDay: link.FirstDay,
	}, nil
}
