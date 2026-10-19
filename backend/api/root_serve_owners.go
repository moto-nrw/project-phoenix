package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
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
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The owner modules of the serving roots, observed into the metrics sinks.

// buildServeOrganizations composes Organisation & Tenancy.
func buildServeOrganizations(db transactionruntime.Database) (organizationtenancy.Capability, error) {
	return organizationCompose.New(organizationCompose.Dependencies{
		DB: db.Pool(),
		Observe: func(observation organizationCompose.Observation) {
			observability.ObserveOrganizationTenancyOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, organizationtenancy.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// buildServeSchoolStructure composes School Structure.
func buildServeSchoolStructure(db transactionruntime.Database) (*schoolstructure.Module, error) {
	return schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB: db.Pool(),
		Observe: func(observation schoolStructureCompose.Observation) {
			observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolstructure.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// observeFacilities records one Facilities operation.
func observeFacilities(observation facilitiesCompose.Observation) {
	observability.ObserveFacilitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, facilitiesModule.ErrorCode(observation.Err), observation.Err)
}

// buildServeRooms composes the Facilities room catalog every owner reads; the
// room deletion is composed once its guard exists.
func buildServeRooms(db transactionruntime.Database) (*facilitiesModule.Rooms, error) {
	return facilitiesCompose.NewRooms(facilitiesCompose.Dependencies{DB: db.Pool(), Observe: observeFacilities})
}

// buildFacilitiesModule composes Facilities with its room deletion: the
// deletion asks Student Presence for open sessions and Care Plan's
// care-offering catalog for the rooms its offerings need (#4010).
func buildFacilitiesModule(db transactionruntime.Database, observe func(facilitiesCompose.Observation), lock timetable.RecurrenceWriteLock, sessions *studentpresence.Module, offerings careplan.CareOfferingGuards) (*facilitiesModule.Module, error) {
	return facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB: db.Pool(), DeletionLock: lock.LockRecurrenceWrites,
		Usage:   &RoomUsage{Sessions: sessions, Offerings: offerings},
		Observe: observe,
	})
}

// observeSchoolCalendar records one School Calendar operation.
func observeSchoolCalendar(observation schoolCalendarCompose.Observation) {
	observability.ObserveSchoolCalendarOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolcalendar.ErrorCode(observation.Err), observation.Err)
}

// buildSchoolCalendarRecords composes the School Calendar every owner reads,
// over the tenant's federal state; the period administration is composed
// once its guards exist.
func buildSchoolCalendarRecords(db transactionruntime.Database, observe func(schoolCalendarCompose.Observation), settings settingsCompose.SettingsService) (*schoolcalendar.Records, error) {
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

// buildCalendarAdministration composes the School Calendar's period
// administration: the recurrence gate serializes it against every
// recurrence-derived write, and Care Plan's catalog refuses a period change
// a linked care offering still needs.
func buildCalendarAdministration(db transactionruntime.Database, observe func(schoolCalendarCompose.Observation), records *schoolcalendar.Records, lock timetable.RecurrenceWriteLock, offerings careplan.CareOfferingGuards) (*schoolcalendar.Module, error) {
	return schoolCalendarCompose.NewAdministration(records, schoolCalendarCompose.AdministrationDependencies{
		DB: db.Pool(), Observe: observe, RecurrenceGate: lock.LockRecurrenceWrites,
		CareOfferingGuard: func(ctx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error {
			err := offerings.ValidateCalendarPeriodChange(ctx, periodID, calendarPeriodReplacement(replacement))
			if errors.Is(err, careplan.ErrCalendarPeriodCareOfferingConflict) {
				return fmt.Errorf("%w: %w", schoolcalendar.ErrCalendarPeriodRequiredByCareOffering, err)
			}
			return err
		},
	})
}

// buildServeWorkforce composes Workforce over School Membership; a
// Sonderarbeitszeit never sets a target on a statutory holiday.
func buildServeWorkforce(db transactionruntime.Database, membership schoolmembership.Capability, calendar schoolcalendar.Days) (*workforceModule.Module, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: workforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        workforceLiveStaffIDs(membership),
		StatutoryHolidays:   calendar.TenantHolidayDates,
		Observe: func(observation workforceCompose.Observation) {
			observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// buildServeAppointments composes Appointments.
func buildServeAppointments(db transactionruntime.Database) (appointmentsModule.Capability, error) {
	return appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB: db.Pool(),
		Observe: func(observation appointmentsCompose.Observation) {
			observability.ObserveAppointmentsOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, appointmentsModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

// buildCommunicationModule composes Communication over Organisation &
// Tenancy, the People Directory's viewer names and the operator audit log.
func buildCommunicationModule(db transactionruntime.Database, env RootEnvironment, organizations organizationtenancy.Capability, persons peopledirectory.Capability, operatorAuditLog operatorAuditEntries) (*communicationModule.Module, error) {
	return communicationCompose.New(communicationCompose.Dependencies{
		DB:            db.Pool(),
		Organizations: organizations,
		Viewers:       communicationViewerNames{query: persons},
		Audit:         NewCommunicationAnnouncementAudit(operatorAuditLog.Create),
		Observe:       env.Observe.Communication,
	})
}

// buildPeopleDirectory composes the People Directory over its owners, the
// photo runtime and the guardian provider, both built before it. Care Plan's
// companion slice and student profiles are constructible from the database
// alone, so the directory's write path gets them here rather than waiting
// for the full Care Plan, which needs this directory.
func buildPeopleDirectory(db transactionruntime.Database, observe func(peopleCompose.Observation), photos peopleCompose.StudentPhotoRuntime, guardians *peopleCompose.GuardianDirectoryProvider) (*peopledirectory.Module, error) {
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

// observePeopleDirectory records one People Directory operation.
func observePeopleDirectory(observation peopleCompose.Observation) {
	observability.ObservePeopleDirectoryOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, peopledirectory.ErrorCode(observation.Err), observation.Err)
}

// buildServeMealPlan composes the Meal Plan over its settings and the
// participants of the day.
func buildServeMealPlan(db transactionruntime.Database, settings *mealplanCompose.Settings, students StudentRootReads, names studentNames, calendarRecords *schoolcalendar.Records) (*mealplanModule.Module, error) {
	if calendarRecords == nil {
		return nil, errors.New("meal plan: the school calendar is required")
	}
	return mealplanCompose.New(mealplanCompose.Dependencies{
		DB:       db.Pool(),
		Settings: settings,
		Observe: func(observation mealplanCompose.Observation) {
			observability.ObserveMealPlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, observation.Err)
		},
		Now:          time.Now,
		Participants: mealPlanParticipantFinder(students, time.Now),
		StudentNames: mealPlanStudentNames(names),
		Calendar:     mealPlanCalendar(calendarRecords),
	})
}

// studentNames is the People Directory's lookup of children by ID.
type studentNames interface {
	ListStudentNamesByID(context.Context, []int64) ([]peopledirectory.StudentName, error)
}

// mealPlanStudentNames names the children of past orders (#3873); their
// class comes from the order itself.
func mealPlanStudentNames(names studentNames) mealplanCompose.StudentNameFinder {
	return func(ctx context.Context, ids []int64) ([]mealplanCompose.ParticipantCandidate, error) {
		rows, err := names.ListStudentNamesByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make([]mealplanCompose.ParticipantCandidate, 0, len(rows))
		for _, row := range rows {
			result = append(result, mealplanCompose.ParticipantCandidate{StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName})
		}
		return result, nil
	}
}

// mealPlanCalendar binds the School Calendar for the Meal Plan: closing days
// and public holidays close a day, school holiday periods suspend the weekly
// registration pattern (#3873).
func mealPlanCalendar(records *schoolcalendar.Records) mealplanCompose.CalendarFinder {
	return func(ctx context.Context, from, to string) (map[string]bool, map[string]bool, error) {
		closed, err := records.NonWorkingDayDates(ctx, from, to)
		if err != nil {
			return nil, nil, err
		}
		periods, err := records.ListCalendarPeriods(ctx, schoolcalendar.HolidaysOverlapping(from, to))
		if err != nil {
			return nil, nil, err
		}
		holiday := make(map[string]bool)
		for _, period := range periods {
			for day := max(period.StartDate, from); day <= min(period.EndDate, to); day = calendarNextDay(day) {
				holiday[day] = true
			}
		}
		return closed, holiday, nil
	}
}

// calendarNextDay advances a YYYY-MM-DD day by one. The School Calendar
// validated it, so a parse failure only ends the loop.
func calendarNextDay(day string) string {
	value, err := timezone.ParseDate(day)
	if err != nil {
		return "9999-12-31"
	}
	return value.AddDays(1).String()
}

// buildServeFeedback composes Feedback over its settings.
func buildServeFeedback(db transactionruntime.Database, settings *feedbackCompose.Settings) (*feedbackModule.Module, error) {
	return composeFeedback(db, settings)
}

// buildServeTimetable composes the observed Timetable owner over the People
// Directory, the room catalog and School Membership.
func buildServeTimetable(db transactionruntime.Database, persons peopledirectory.StudentQuery, rooms facilitiesModule.Query, membership schoolmembership.Capability) (*timetable.Module, error) {
	return newTimetable(db, persons, rooms, membership, func(observation timetableCompose.Observation) {
		observability.ObserveTimetableActivitiesOperation(
			observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows,
			observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration,
			timetable.ErrorCode(observation.Err), observation.Err,
		)
	})
}
