package api

import (
	"context"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
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
		GuardianAuthorizer: src.GuardianAuthorizer, Outbox: src.OutboxEnqueuer,
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
	return enrollmentAdminSubscribers{resolver: deliveryCompose.NewEmailRecipientResolver(subscribers, accounts, securityruntime.HasPermission)}
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
		CompanionLockBusy: peopledirectory.TranslateStudentWriteError(peopledirectory.ErrCompanionLockBusy), People: people,
		GuardianInvitations: src.GuardianInvitations,
		Settings:            intakeSettings(src.Settings), Outbox: src.OutboxEnqueuer,
		FrontendURL: src.FrontendURL, ParentsURL: src.ParentsURL, Logger: src.Logger,
	}
	if src.PersonRepo != nil {
		deps.Reviewers = enrollmentReviewerNames{persons: src.PersonRepo}
	}
	return enrollmentCompose.NewChangeRequests(deps)
}

// intakeMailOutbox binds Enrollment's mail outbox to Delivery's; nil
// without one.
func intakeMailOutbox(outbox MailOutbox) enrollmentCompose.MailOutbox {
	if outbox == nil {
		return nil
	}
	return enrollmentMailOutbox{outbox: outbox}
}

func intakeSettings(settings EnrollmentIntakeSettings) enrollmentCompose.IntakeSettings {
	if settings == nil {
		return nil
	}
	return enrollmentIntakeSettings{settingsModule.NewEnrollmentIntakeValues(settings)}
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
			names[accountID] = peopledirectory.PersonDisplayName(person.FirstName, person.LastName)
		}
	}
	return names, nil
}

// enrollmentIntakeSettings serves the intake's settings port over the
// Settings Platform's intake values; only the legal texts need a type map.
type enrollmentIntakeSettings struct {
	settingsModule.EnrollmentIntakeValues
}

func (s enrollmentIntakeSettings) LegalSettings(ctx context.Context) (enrollmentCompose.LegalSettings, error) {
	legal, err := s.Legal(ctx)
	if err != nil {
		return enrollmentCompose.LegalSettings{}, err
	}
	return enrollmentCompose.LegalSettings(legal), nil
}
