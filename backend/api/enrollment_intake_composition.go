package api

import (
	"context"
	"fmt"
	"strings"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
)

// The bindings below compose Enrollment's intake and change requests (#3565)
// over the Enrollment owner, Care Plan's offerings and bookings, the People
// Directory rows of the retained repositories, the Settings Platform and the
// Delivery outbox.

// NewEnrollmentIntake composes Enrollment's intake over the sources.
func NewEnrollmentIntake(src EnrollmentIntakeSources) *enrollmentCompose.Intake {
	return enrollmentCompose.NewIntake(enrollmentCompose.IntakeDependencies{
		Requests: src.Requests, Children: src.Children, Guardians: src.Guardians, LateInvites: src.LateInviteRepo,
		Catalog: src.Catalog, RateLimits: src.RateLimitRepo, Offerings: src.CareOfferingRepo, Bookings: src.Bookings,
		Capacity: src.Capacity, Schools: src.SchoolRepo, Notifications: src.Notifications, Students: src.StudentRepo,
		GuardianAuthorizer: src.GuardianAuthorizer, Outbox: intakeMailOutbox(src.OutboxEnqueuer),
		Settings: intakeSettings(src.Settings), AdminSubscribers: src.AdminSubscribers, ManualDecider: src.ManualDecider,
		ParentChanges: src.ParentChanges,
		Random:        securityruntime.FillRandom, Fingerprint: securityruntime.Fingerprint,
		FrontendURL: src.FrontendURL, ParentsURL: src.ParentsURL, Logger: src.Logger,
	})
}

// NewEnrollmentAdminSubscribers binds Delivery's opt-in e-mail resolver to the
// intake's admin notification (#3780): the configured addresses plus every
// enrollment manager of the school who switched "Neue Anmeldung" on.
func NewEnrollmentAdminSubscribers(subscribers deliveryCompose.EmailSubscribers, accounts deliveryCompose.EmailAccounts) enrollmentCompose.AdminMailSubscribers {
	return enrollmentAdminSubscribers{resolver: deliveryCompose.NewEmailRecipientResolver(subscribers, accounts)}
}

type enrollmentAdminSubscribers struct {
	resolver deliveryCompose.EmailRecipientResolver
}

func (s enrollmentAdminSubscribers) AdminNotificationRecipients(ctx context.Context, configured []string) ([]string, error) {
	return s.resolver.Recipients(ctx, deliveryCompose.NotificationEnrollmentSubmitted, configured)
}

// NewEnrollmentChangeRequests composes Enrollment's change requests over the
// sources.
func NewEnrollmentChangeRequests(src EnrollmentChangeRequestSources) enrollmentOwner.ChangeRequests {
	people := enrollmentCompose.PeopleDirectory{}
	if src.GuardianProfileRepo != nil {
		people.GuardianProfiles = enrollmentGuardianProfiles{repo: src.GuardianProfileRepo}
	}
	if src.GuardianPhoneRepo != nil {
		people.GuardianPhones = enrollmentGuardianPhones{repo: src.GuardianPhoneRepo}
	}
	deps := enrollmentCompose.ChangeRequestDependencies{
		Requests: src.Requests, Children: src.Children, Guardians: src.Guardians, LateInvites: src.LateInviteRepo,
		Catalog: src.Catalog, Offerings: src.CareOfferingRepo, Bookings: src.Bookings, Capacity: src.Capacity,
		Notifications: src.Notifications, Students: src.StudentRepo, GuardianAuthorizer: src.GuardianAuthorizer,
		Decisions: src.Decisions, BookingGates: src.BookingGates, Companions: src.CompanionGraphLocker,
		CompanionLockBusy: userModels.ErrCompanionLockBusy, People: people,
		GuardianInvitations: src.GuardianInvitations,
		Settings:            intakeSettings(src.Settings), Outbox: intakeMailOutbox(src.OutboxEnqueuer),
		FrontendURL: src.FrontendURL, ParentsURL: src.ParentsURL, Logger: src.Logger,
	}
	if src.PersonRepo != nil {
		deps.Reviewers = enrollmentReviewerNames{persons: src.PersonRepo}
	}
	return enrollmentCompose.NewChangeRequests(deps)
}

func intakeMailOutbox(outbox platformModels.OutboxEnqueuer) enrollmentCompose.MailOutbox {
	if outbox == nil {
		return nil
	}
	return enrollmentMailOutbox{outbox: outbox}
}

func intakeSettings(settings EnrollmentIntakeSettings) enrollmentCompose.IntakeSettings {
	if settings == nil {
		return nil
	}
	return enrollmentIntakeSettings{settings: settings}
}

// enrollmentReviewerNames renders the deciding staffer's display name.
type enrollmentReviewerNames struct {
	persons EnrollmentReviewerPersons
}

func (r enrollmentReviewerNames) ReviewerNames(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	persons, err := r.persons.FindByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(persons))
	for accountID, person := range persons {
		if person != nil {
			names[accountID] = strings.TrimSpace(person.FirstName + " " + person.LastName)
		}
	}
	return names, nil
}

// enrollmentIntakeSettings resolves the intake settings with the fallbacks
// the registry defines.
type enrollmentIntakeSettings struct {
	settings EnrollmentIntakeSettings
}

func (s enrollmentIntakeSettings) EnrollmentEnabled(ctx context.Context) bool {
	return settingsCompose.ResolveBoolOrDefault(ctx, s.settings, configModels.KeyEnrollmentEnabled, false, nil)
}

func (s enrollmentIntakeSettings) AllowSubmissionEdit(ctx context.Context) bool {
	return settingsCompose.ResolveBoolOrDefault(ctx, s.settings, configModels.KeyEnrollmentAllowSubmissionEdit, true, nil)
}

func (s enrollmentIntakeSettings) StatusTokenTTLDays(ctx context.Context) int {
	return settingsCompose.ResolveIntOrDefault(ctx, s.settings, configModels.KeyEnrollmentStatusTokenTTLDays, 365, nil)
}

func (s enrollmentIntakeSettings) AdminNotificationEmails(ctx context.Context) string {
	return settingsCompose.ResolveStringOrDefault(ctx, s.settings, configModels.KeyEnrollmentNotificationEmails, "", nil)
}

func (s enrollmentIntakeSettings) DuplicateHandling(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModels.KeyEnrollmentDuplicateHandling)
}

func (s enrollmentIntakeSettings) CollectGradeLevel(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectGradeLevel)
}

func (s enrollmentIntakeSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectSchoolClass)
}

func (s enrollmentIntakeSettings) CareOfferingsEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCareOfferingsEnabled)
}

func (s enrollmentIntakeSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

func (s enrollmentIntakeSettings) ChangeRequestMailsEnabled(ctx context.Context) bool {
	enabled, err := s.settings.ResolveBool(ctx, configModels.KeyEnrollmentChangeRequestEmailNotificationsEnabled)
	return err == nil && enabled
}

// enrollmentLegalSettingKeys are read in one snapshot when the resolver
// supports it.
var enrollmentLegalSettingKeys = []string{
	configModels.KeyEnrollmentLegalAGBText,
	configModels.KeyEnrollmentLegalAGBDocumentURL,
	configModels.KeyEnrollmentLegalAGBDisplayMode,
	configModels.KeyEnrollmentLegalDSGVOText,
	configModels.KeyEnrollmentLegalEmailContactText,
	configModels.KeyEnrollmentLegalPhotoText,
	configModels.KeyEnrollmentLegalTermsEnabled,
	configModels.KeyEnrollmentLegalDSGVOEnabled,
	configModels.KeyEnrollmentLegalEmailContactEnabled,
	configModels.KeyEnrollmentLegalPhotoEnabled,
}

// LegalSettings resolves the legal texts and toggles. No env fallback: a
// plain resolve (tenant override, then the registry default "") is correct,
// and a failure is propagated because the texts drive legal blocks.
func (s enrollmentIntakeSettings) LegalSettings(ctx context.Context) (enrollmentCompose.LegalSettings, error) {
	if batch, ok := s.settings.(interface {
		ResolveMany(context.Context, []string) (*settingsCompose.SettingsSnapshot, error)
	}); ok {
		snapshot, err := batch.ResolveMany(ctx, enrollmentLegalSettingKeys)
		if err != nil {
			return enrollmentCompose.LegalSettings{}, fmt.Errorf("resolve legal settings: %w", err)
		}
		if snapshot != nil {
			ctx = settingsCompose.WithSettingsSnapshot(ctx, snapshot)
		}
	}
	var out enrollmentCompose.LegalSettings
	for _, text := range []struct {
		key   string
		label string
		into  *string
	}{
		{configModels.KeyEnrollmentLegalAGBText, "AGB legal text", &out.AGB},
		{configModels.KeyEnrollmentLegalAGBDocumentURL, "AGB legal document", &out.AGBDocumentURL},
		{configModels.KeyEnrollmentLegalAGBDisplayMode, "AGB legal display mode", &out.AGBDisplayMode},
		{configModels.KeyEnrollmentLegalDSGVOText, "DSGVO legal text", &out.DSGVO},
		{configModels.KeyEnrollmentLegalEmailContactText, "email contact legal text", &out.EmailContact},
		{configModels.KeyEnrollmentLegalPhotoText, "photo legal text", &out.Photo},
	} {
		value, err := s.settings.ResolveString(ctx, text.key)
		if err != nil {
			return enrollmentCompose.LegalSettings{}, fmt.Errorf("resolve %s: %w", text.label, err)
		}
		*text.into = value
	}
	return s.legalToggles(ctx, out)
}

// legalToggles resolves the legal block toggles. A toggle without a tenant
// override is off; a failure fails closed, because the toggle decides whether
// a required legal block is rendered and enforced.
func (s enrollmentIntakeSettings) legalToggles(ctx context.Context, out enrollmentCompose.LegalSettings) (enrollmentCompose.LegalSettings, error) {
	for _, toggle := range []struct {
		key   string
		label string
		into  *bool
	}{
		{configModels.KeyEnrollmentLegalTermsEnabled, "AGB terms", &out.TermsEnabled},
		{configModels.KeyEnrollmentLegalDSGVOEnabled, "DSGVO legal block", &out.DSGVOEnabled},
		{configModels.KeyEnrollmentLegalEmailContactEnabled, "email contact legal block", &out.EmailContactEnabled},
		{configModels.KeyEnrollmentLegalPhotoEnabled, "photo legal block", &out.PhotoEnabled},
	} {
		has, err := s.settings.HasTenantOverride(ctx, toggle.key)
		if err != nil {
			return enrollmentCompose.LegalSettings{}, fmt.Errorf("check %s setting override: %w", toggle.label, err)
		}
		if !has {
			continue
		}
		value, err := s.settings.ResolveBool(ctx, toggle.key)
		if err != nil {
			return enrollmentCompose.LegalSettings{}, fmt.Errorf("resolve %s setting: %w", toggle.label, err)
		}
		*toggle.into = value
	}
	return out, nil
}
