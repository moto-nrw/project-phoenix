package owners

import (
	"context"
	"fmt"
	"time"

	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
)

// BuildServeOrganizations composes Organisation & Tenancy.
func BuildServeOrganizations(db transactionruntime.Database) (organizationtenancy.Capability, error) {
	return organizationCompose.New(organizationCompose.Dependencies{
		DB: db.Pool(),
		Observe: func(observation organizationCompose.Observation) {
			observability.ObserveOrganizationTenancyOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, organizationtenancy.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// BuildServeSchoolStructure composes School Structure.
func BuildServeSchoolStructure(db transactionruntime.Database) (schoolstructure.Owner, error) {
	return schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB: db.Pool(),
		Observe: func(observation schoolStructureCompose.Observation) {
			observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolstructure.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// ObserveFacilities records one Facilities operation.
func ObserveFacilities(observation facilitiesCompose.Observation) {
	observability.ObserveFacilitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, facilitiesModule.ErrorCode(observation.Err), observation.Err)
}

// BuildServeRooms composes the Facilities room catalog every owner reads; the
// room deletion is composed once its guard exists.
func BuildServeRooms(db transactionruntime.Database) (facilitiesModule.Catalog, error) {
	return facilitiesCompose.NewRooms(facilitiesCompose.Dependencies{DB: db.Pool(), Observe: ObserveFacilities})
}

// BuildFacilitiesModule composes Facilities with its room deletion: the
// deletion asks Student Presence for open sessions and Care Plan's
// care-offering catalog for the rooms its offerings need (#4010).
func BuildFacilitiesModule(db transactionruntime.Database, observe func(facilitiesCompose.Observation), lock timetable.RecurrenceWriteLock, sessions studentpresence.Owner, offerings careplan.CareOfferingGuards) (facilitiesModule.Capability, error) {
	return facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB: db.Pool(), DeletionLock: lock.LockRecurrenceWrites,
		Usage:   &RoomUsage{Sessions: sessions, Offerings: offerings},
		Observe: observe,
	})
}

// ObserveSchoolCalendar records one School Calendar operation.
func ObserveSchoolCalendar(observation schoolCalendarCompose.Observation) {
	observability.ObserveSchoolCalendarOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolcalendar.ErrorCode(observation.Err), observation.Err)
}

// BuildSchoolCalendarRecords composes the School Calendar every owner reads,
// over the tenant's federal state; the period administration is composed
// once its guards exist.
func BuildSchoolCalendarRecords(db transactionruntime.Database, observe func(schoolCalendarCompose.Observation), settings settingsCompose.SettingsService) (schoolcalendar.Days, error) {
	return schoolCalendarCompose.NewRecords(schoolCalendarCompose.RecordsDependencies{
		DB: db.Pool(), Observe: observe,
		FederalState: func(ctx context.Context) (string, error) {
			region, err := settings.ResolveString(ctx, configModels.KeyFederalState)
			if err != nil {
				return "", fmt.Errorf("failed to resolve federal state setting: %w", err)
			}
			return region, nil
		},
	})
}

// BuildServeWorkforce composes Workforce over School Membership; a
// Sonderarbeitszeit never sets a target on a statutory holiday.
func BuildServeWorkforce(db transactionruntime.Database, membership schoolmembership.Capability, calendar schoolcalendar.Days) (workforceCompose.WorkTime, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: WorkforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        WorkforceLiveStaffIDs(membership),
		StatutoryHolidays:   calendar.TenantHolidayDates,
		Observe: func(observation workforceCompose.Observation) {
			observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// BuildServeAppointments composes Appointments.
func BuildServeAppointments(db transactionruntime.Database) (appointmentsModule.Owner, error) {
	return appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB: db.Pool(),
		Observe: func(observation appointmentsCompose.Observation) {
			observability.ObserveAppointmentsOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, appointmentsModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// BuildPeopleDirectory composes the People Directory over its owners, the
// photo runtime and the guardian provider, both built before it. Care Plan's
// companion slice and student profiles are constructible from the database
// alone, so the directory's write path gets them here rather than waiting
// for the full Care Plan, which needs this directory.
func BuildPeopleDirectory(db transactionruntime.Database, observe func(peopleCompose.Observation), photos peopleCompose.StudentPhotoRuntime, guardians *peopleCompose.GuardianDirectoryProvider) (*peopleCompose.Directory, error) {
	companions, err := careplanCompose.NewCompanionRecords(db.Pool(), func(careplanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	careProfiles, err := careplanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(careplanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	return ComposePeopleDirectory(peopleCompose.Dependencies{
		StudentClassWriteGateQuery: membershipCompose.StudentClassWriteGate(db.Pool()),
		DB:                         db.Pool(),
		Observe:                    observe,
		StudentPhotos:              photos,
		GuardianProvider:           guardians,
	}, membership, careProfiles, companions, peopleDirectoryAudit(db), newGuardianLinkOwners(db),
		NewPeopleIdentityAccess(db, nil).FindActiveGuardianMemberships)
}

// ObservePeopleDirectory records one People Directory operation.
func ObservePeopleDirectory(observation peopleCompose.Observation) {
	observability.ObservePeopleDirectoryOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, peopledirectory.ErrorCode(observation.Err), observation.Err)
}

// BuildServeMealPlan composes the Meal Plan over its settings and the
// participants of the day.
func BuildServeMealPlan(db transactionruntime.Database, settings *mealplanCompose.Settings, students StudentRootReads) (mealplanModule.Capability, error) {
	return mealplanCompose.New(mealplanCompose.Dependencies{
		DB:       db.Pool(),
		Settings: settings,
		Observe: func(observation mealplanCompose.Observation) {
			observability.ObserveMealPlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, observation.Err)
		},
		Now:          time.Now,
		Participants: mealPlanParticipantFinder(students, time.Now),
	})
}

// BuildServeFeedback composes Feedback over its settings.
func BuildServeFeedback(db transactionruntime.Database, settings *feedbackCompose.Settings) (feedbackModule.Capability, error) {
	return composeFeedback(db, settings)
}

// BuildServeTimetable composes the observed Timetable owner over the People
// Directory, the room catalog and School Membership.
func BuildServeTimetable(db transactionruntime.Database, persons peopledirectory.StudentQuery, rooms facilitiesModule.Query, membership schoolmembership.Capability) (timetable.Owner, error) {
	return newTimetable(db, persons, rooms, membership, func(observation timetableCompose.Observation) {
		observability.ObserveTimetableActivitiesOperation(
			observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows,
			observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration,
			timetable.ErrorCode(observation.Err), observation.Err,
		)
	})
}
