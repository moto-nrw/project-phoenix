package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// enrollmentWiringOwners are the owners the root has composed when it binds
// Enrollment into the service graph (#4012).
type enrollmentWiringOwners struct {
	DB        transactionruntime.Database
	CarePlan  careplan.Capability
	Persons   peopledirectory.Capability
	Calendar  enrollmentCompose.CalendarPeriods
	Timetable timetable.Capability
	Schools   enrollmentOwner.SchoolDirectory
}

// newEnrollmentWiring binds Enrollment to the owners the root composed and
// hands the service graph the flows it builds over the graph's own values
// (ADR 0053 point 8): the booking projection over Care Plan's bookings, the
// People Directory, Care Plan, Timetable and Settings Platform adapters, and
// the builders of the decision, intake, change-request, rollover, report,
// deletion and offering-change flows.
func newEnrollmentWiring(owners enrollmentWiringOwners) EnrollmentWiring {
	bookings := NewEnrollmentBookings()
	careOfferings := enrollmentCompose.NewCareOfferingRecords(owners.CarePlan)
	return EnrollmentWiring{
		Bookings:              bookings,
		ApprovedOfferings:     enrollmentCompose.NewApprovedOfferingProjection(bookings, OfferingStudents{Query: owners.Persons}),
		CareBookings:          enrollmentCareBookings{owner: owners.CarePlan},
		Guardians:             enrollmentGuardianDirectory{guardians: owners.Persons},
		ParentCarePeriods:     parentCarePeriods{owner: bookings},
		ParentOfferingHistory: parentOfferingHistory{owner: bookings},
		OfferingPlanning:      ManualPlanningReader{Query: PlanningQuery{DB: owners.DB}, CourseGroups: owners.Timetable},
		BindSettingsGuards:    bindEnrollmentSettingsGuards(bookings),
		MailRenderers: func(defaultFrom email.Email) map[string]deliveryCompose.MailRenderer {
			return enrollmentMailRenderers(enrollmentCompose.NewMailRenderers(), defaultFrom)
		},
		Notifications: func(settings EnrollmentSettings, outbox platformModels.OutboxEnqueuer) enrollmentOwner.Notifications {
			return NewEnrollmentNotifications(bookings, settings, outbox, owners.Schools)
		},
		Deletions: NewEnrollmentDeletionModule,
		Phases: func(src EnrollmentPhaseSources) *enrollmentCompose.Phases {
			return enrollmentCompose.NewPhases(enrollmentCompose.PhaseDependencies{
				Records: bookings, Offerings: enrollmentPhaseOfferings{carePlan: owners.CarePlan}, Calendar: owners.Calendar,
				LockTemplateRecurrence: src.LockTemplateRecurrence, ValidateCareOfferingPhaseChange: src.ValidateCareOfferingPhaseChange,
				SourcedTemplates: src.SourcedTemplates, Settings: enrollmentCollectionSettings{settings: src.Settings},
				Roster: phaseResponseRoster{query: owners.Persons}, CareExits: phaseResponseCareExits{query: owners.CarePlan},
				PortalAccounts: owners.Persons, Logger: src.Logger,
			})
		},
		Decisions: NewEnrollmentDecisions,
		Capacity: func(settings EnrollmentSettings) enrollmentOwner.OfferingCapacity {
			return NewEnrollmentOfferingCapacity(careOfferings, bookings, settings)
		},
		AdminSubscribers: NewEnrollmentAdminSubscribers,
		Intake:           NewEnrollmentIntake,
		Reports:          NewEnrollmentReports,
		GuardianInvitations: func(invitations EnrollmentGuardianInvitations) enrollmentCompose.GuardianInvitationAvailability {
			return enrollmentGuardianInvitationAvailability{invitations: invitations}
		},
		ChangeRequests:    NewEnrollmentChangeRequests,
		Rollovers:         NewEnrollmentRollovers,
		OfferingChanges:   newOfferingChanges,
		OfferingConflicts: newOfferingChangeConflicts,
	}
}

// enrollmentPhaseRestrictions are Enrollment's reads of the active phases
// that restrict eligibility.
type enrollmentPhaseRestrictions interface {
	HasActiveClassRestrictedPhase(ctx context.Context) (bool, error)
	HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error)
	MaxActivePhaseGrade(ctx context.Context) (int, error)
}

// bindEnrollmentSettingsGuards binds the Settings Platform's collection
// guards to Enrollment's active phases (#1663). Each guard only binds a
// Settings port to an Enrollment read; Settings decides what it refuses. The
// reads run inside the caller's tenant transaction, so the phase lookups are
// RLS-scoped.
func bindEnrollmentSettingsGuards(phases enrollmentPhaseRestrictions) func(settingsCompose.BatchSettingsService) {
	return func(settings settingsCompose.BatchSettingsService) {
		// Disabling concrete-class collection is refused while an active
		// phase restricts eligibility to specific classes.
		if guarded, ok := settings.(interface {
			SetClassRestrictionGuard(func(context.Context) (bool, error))
		}); ok {
			guarded.SetClassRestrictionGuard(phases.HasActiveClassRestrictedPhase)
		}
		// The grade-level restriction survives concrete-class collection
		// being off but not grade-level collection.
		if guarded, ok := settings.(interface {
			SetGradeRestrictionGuard(func(context.Context) (bool, error))
		}); ok {
			guarded.SetGradeRestrictionGuard(phases.HasActiveGradeRestrictedPhase)
		}
		// enrollment.grade_level_max cannot fall below a grade an active
		// phase already restricts itself to, which would leave that phase
		// with no selectable eligible grade.
		if guarded, ok := settings.(interface {
			SetGradeCapGuard(func(context.Context) (int, error))
		}); ok {
			guarded.SetGradeCapGuard(phases.MaxActivePhaseGrade)
		}
	}
}

// newEnrollmentFormSchemas builds the form schema administration where the
// root mounts it, over the collection settings (#4012).
func newEnrollmentFormSchemas(settings enrollmentSettingsReads, logger *slog.Logger) enrollmentOwner.FormSchemaAdministration {
	return enrollmentCompose.NewFormSchemas(NewEnrollmentBookings(), enrollmentCollectionSettings{settings: settings}, logger.With("service", "enrollment-form-schema"))
}

// newEnrollmentCaptcha builds the captcha verifier of the public enrollment
// form where the root mounts it.
func newEnrollmentCaptcha(settings enrollmentSettingsReads, logger *slog.Logger) enrollmentOwner.CaptchaVerifier {
	return enrollmentCompose.NewCaptcha(enrollmentCaptchaSettings{settings: settings}, "", logger.With("service", "enrollment-captcha"))
}

// newEnrollmentPhaseExpiry builds the phase expiry warnings over the People
// Directory's enrolled students and Care Plan's offerings.
func newEnrollmentPhaseExpiry(students peopledirectory.StudentQuery, carePlan careplan.Query) enrollmentOwner.PhaseExpiryWarnings {
	bookings := NewEnrollmentBookings()
	return enrollmentCompose.NewPhaseExpiryWarnings(enrollmentCompose.NewPhaseExpirySnapshots(
		bookings, phaseExpiryStudents{query: students}, phaseExpiryCarePlanDirectory{query: carePlan}, bookings,
	))
}
