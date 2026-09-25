package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// The bindings below compose Enrollment's intake and change requests (#3565)
// over the Enrollment owner, Care Plan's offerings and bookings, the People
// Directory rows of the retained repositories, the Settings Platform and the
// Delivery outbox.

// EnrollmentIntakeSettings resolves the tenant settings of the intake by key.
// A resolver that also answers ResolveMany is read through one snapshot.
type EnrollmentIntakeSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentIntakeSources are the owners and retained repositories the root
// binds the intake to. A nil source skips the work that needs it, the way
// the intake always treated an unwired repository.
type EnrollmentIntakeSources struct {
	Requests           enrollmentCompose.IntakeRequests
	Children           enrollmentCompose.IntakeChildren
	Bookings           enrollmentOwner.CareBookingCommands
	Guardians          enrollmentCompose.IntakeGuardians
	LateInviteRepo     enrollmentCompose.IntakeLateInvites
	CareOfferingRepo   enrollmentCompose.IntakeOfferings
	Capacity           enrollmentOwner.OfferingCapacity
	Catalog            enrollmentCompose.IntakeCatalog
	SchoolRepo         enrollmentOwner.SchoolDirectory
	Notifications      enrollmentOwner.Notifications
	StudentRepo        enrollmentCompose.StudentMatches
	GuardianAuthorizer enrollmentCompose.GuardianStudentAuthorizer
	RateLimitRepo      enrollmentCompose.SubmissionRateLimiter
	OutboxEnqueuer     platformModels.OutboxEnqueuer
	Settings           EnrollmentIntakeSettings
	ManualDecider      enrollmentCompose.ManualEnrollmentDecider
	// FrontendURL is the base of the staff links, ParentsURL of the
	// parent-facing ones; it falls back to FrontendURL.
	FrontendURL string
	ParentsURL  string
	Logger      *slog.Logger
}

// NewEnrollmentIntake composes Enrollment's intake over the sources.
func NewEnrollmentIntake(src EnrollmentIntakeSources) *enrollmentCompose.Intake {
	return enrollmentCompose.NewIntake(enrollmentCompose.IntakeDependencies{
		Requests: src.Requests, Children: src.Children, Guardians: src.Guardians, LateInvites: src.LateInviteRepo,
		Catalog: src.Catalog, RateLimits: src.RateLimitRepo, Offerings: src.CareOfferingRepo, Bookings: src.Bookings,
		Capacity: src.Capacity, Schools: src.SchoolRepo, Notifications: src.Notifications, Students: src.StudentRepo,
		GuardianAuthorizer: src.GuardianAuthorizer, Outbox: intakeMailOutbox(src.OutboxEnqueuer),
		Settings: intakeSettings(src.Settings), ManualDecider: src.ManualDecider,
		Random: securityruntime.FillRandom, Fingerprint: securityruntime.Fingerprint,
		FrontendURL: src.FrontendURL, ParentsURL: src.ParentsURL, Logger: src.Logger,
	})
}

// EnrollmentChangeRequestSources are the owners and retained repositories the
// root binds the change requests to.
type EnrollmentChangeRequestSources struct {
	Bookings           enrollmentCompose.CareBookingChanges
	Requests           enrollmentCompose.ChangeRequestRecords
	Children           enrollmentCompose.IntakeChildren
	Guardians          enrollmentCompose.IntakeGuardians
	LateInviteRepo     enrollmentCompose.DecisionLateInvites
	CareOfferingRepo   enrollmentCompose.IntakeOfferings
	Capacity           enrollmentOwner.OfferingCapacity
	Catalog            enrollmentCompose.IntakeCatalog
	Notifications      enrollmentOwner.Notifications
	GuardianProfiles   userModels.GuardianProfileRepository
	GuardianPhones     userModels.GuardianPhoneNumberRepository
	Persons            EnrollmentReviewerPersons
	StudentRepo        enrollmentCompose.StudentMatches
	GuardianAuthorizer enrollmentCompose.GuardianStudentAuthorizer
	// Decisions applies an approved change to a child the decision flow
	// already approved; BookingGates wrap those writes. A nil Decisions
	// leaves every child to the booking replacement.
	Decisions            enrollmentOwner.ApprovedChildChanges
	BookingGates         enrollmentCompose.CareBookingGates
	CompanionGraphLocker enrollmentCompose.CompanionGraphCoordinator
	Settings             EnrollmentIntakeSettings
	OutboxEnqueuer       platformModels.OutboxEnqueuer
	FrontendURL          string
	ParentsURL           string
	Logger               *slog.Logger
}

// EnrollmentReviewerPersons resolves the persons behind reviewer accounts.
type EnrollmentReviewerPersons interface {
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*userModels.Person, error)
}

// NewEnrollmentChangeRequests composes Enrollment's change requests over the
// sources.
func NewEnrollmentChangeRequests(src EnrollmentChangeRequestSources) enrollmentOwner.ChangeRequests {
	people := enrollmentCompose.PeopleDirectory{}
	if src.GuardianProfiles != nil {
		people.GuardianProfiles = enrollmentGuardianProfiles{repo: src.GuardianProfiles}
	}
	if src.GuardianPhones != nil {
		people.GuardianPhones = enrollmentGuardianPhones{repo: src.GuardianPhones}
	}
	deps := enrollmentCompose.ChangeRequestDependencies{
		Requests: src.Requests, Children: src.Children, Guardians: src.Guardians, LateInvites: src.LateInviteRepo,
		Catalog: src.Catalog, Offerings: src.CareOfferingRepo, Bookings: src.Bookings, Capacity: src.Capacity,
		Notifications: src.Notifications, Students: src.StudentRepo, GuardianAuthorizer: src.GuardianAuthorizer,
		Decisions: src.Decisions, BookingGates: src.BookingGates, Companions: src.CompanionGraphLocker,
		CompanionLockBusy: userModels.ErrCompanionLockBusy, People: people,
		Settings: intakeSettings(src.Settings), Outbox: intakeMailOutbox(src.OutboxEnqueuer),
		FrontendURL: src.FrontendURL, ParentsURL: src.ParentsURL, Logger: src.Logger,
	}
	if src.Persons != nil {
		deps.Reviewers = enrollmentReviewerNames{persons: src.Persons}
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
type enrollmentReviewerNames struct{ persons EnrollmentReviewerPersons }

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
type enrollmentIntakeSettings struct{ settings EnrollmentIntakeSettings }

func (s enrollmentIntakeSettings) EnrollmentEnabled(ctx context.Context) bool {
	return config.ResolveBoolOrDefault(ctx, s.settings, configModels.KeyEnrollmentEnabled, false, nil)
}

func (s enrollmentIntakeSettings) AllowSubmissionEdit(ctx context.Context) bool {
	return config.ResolveBoolOrDefault(ctx, s.settings, configModels.KeyEnrollmentAllowSubmissionEdit, true, nil)
}

func (s enrollmentIntakeSettings) StatusTokenTTLDays(ctx context.Context) int {
	return config.ResolveIntOrDefault(ctx, s.settings, configModels.KeyEnrollmentStatusTokenTTLDays, 365, nil)
}

func (s enrollmentIntakeSettings) AdminNotificationEmails(ctx context.Context) string {
	return config.ResolveStringOrDefault(ctx, s.settings, configModels.KeyEnrollmentNotificationEmails, "", nil)
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
		ResolveMany(context.Context, []string) (*config.SettingsSnapshot, error)
	}); ok {
		snapshot, err := batch.ResolveMany(ctx, enrollmentLegalSettingKeys)
		if err != nil {
			return enrollmentCompose.LegalSettings{}, fmt.Errorf("resolve legal settings: %w", err)
		}
		if snapshot != nil {
			ctx = config.WithSettingsSnapshot(ctx, snapshot)
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
