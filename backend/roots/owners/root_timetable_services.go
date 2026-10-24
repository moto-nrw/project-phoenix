package owners

import (
	"context"
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// TimetableLifecycle are the Timetable owner's recurrence engine (WP-B8),
// its conflict detection and staffing (#3550), the instance lifecycle, the
// deviation writes (Vertretungsplan, #3424 slice S3), the retention sweep
// (WP-B14), the automatic start and end of instances and the operational
// day. Rows are the owner rows with the recovery locks the lifecycle and
// everything after it take.
type TimetableLifecycle struct {
	Rows            arrivalTimetable.OwnerRows
	Materialization timetable.MaterializationCapability
	Conflicts       timetable.ConflictDetectionCapability
	Instances       timetablePorts.InstanceLifecycle
	Deviations      timetable.StaffDeviations
	Cleanup         timetable.TimetableCleanup
	AutoStart       timetable.InstanceAutoStart
	AutoEnd         timetable.InstanceAutoEnd
	Operations      timetable.OperationCapability
	// BlockMetadata names each planned block's room and planning track (#4413).
	BlockMetadata timetablePorts.BlockDisplayMetadata
}

// BuildTimetableMaterialization binds the recurrence engine: it turns
// activity templates into concrete instances with their staff and students
// for a date window, for the scheduler task and the manual admin endpoint.
// Holidays and closing days carry no series occurrences unless the series
// opts into closing days (#3594); the School Calendar answers.
func BuildTimetableMaterialization(db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, owner TimetableOwner, people PeopleRows, calendar schoolcalendar.Days) (timetable.MaterializationCapability, error) {
	deps := TimetableMaterializationDependencies(owner.TemplateRows, people.Students, calendar,
		owner.RecurrenceLock, hub, env.Logger.With("service", "materialization"))
	deps.Transactions = arrivalTimetable.NewTenantTransactions(db.Pool())
	materialization, err := arrivalTimetable.NewMaterialization(deps)
	if err != nil {
		return nil, fmt.Errorf("compose timetable materialization: %w", err)
	}
	return materialization, nil
}

// BuildTimetableLifecycle binds the conflict detection, the instance
// lifecycle and the deviation writes. The cancellation notice (#2601) rides
// on the parent announcements, which are composed before the lifecycle.
func BuildTimetableLifecycle(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, owner TimetableOwner, days TimetableDays,
	people PeopleRows, careRows TimetableCareRows, presence PresenceOwner, supervisions studentpresence.SupervisionRecords,
	shifts timetablePorts.ConflictShifts, schedules CarePlanSchedules, carePlan careplan.Capability, active studentpresence.Presence,
	materialization timetable.MaterializationCapability, notices communication.CareCancellationPublisher, settings settingsCompose.BatchSettingsService,
) (TimetableLifecycle, error) {
	repos := owner.Repositories
	rows := days.Rows
	rows.Locks = arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), repos.InstanceStudent)
	conflicts, err := NewTimetableConflictDetection(timetablePorts.ConflictDetectionDependencies{
		Instances: repos.ActivityInstance.Reads, InstanceStaff: repos.InstanceStaff, InstanceStudents: repos.InstanceStudent,
		Exceptions: repos.ActivityException, Schedules: repos.ActivitySchedule, Staff: NewTimetableStaff(people.Staff),
		CalendarPeriods: owner.TemplateRows.CalendarPeriods, ArrivalExceptions: careRows.ArrivalExceptions,
		Sessions: presence.SessionRecords, Shifts: shifts, Presence: presence.Presence,
		Logger: env.Logger.With("service", "timetable-conflicts"),
	}, schedules.ArrivalBaselines)
	if err != nil {
		return TimetableLifecycle{}, fmt.Errorf("compose timetable conflict detection: %w", err)
	}
	substituteConflicts, err := arrivalTimetable.NewSubstituteConflictsOverRows(rows)
	if err != nil {
		return TimetableLifecycle{}, fmt.Errorf("compose timetable substitute conflicts: %w", err)
	}
	instances, err := NewTimetableLifecycle(timetablePorts.InstanceLifecycleDependencies{
		IdempotencyRepo: repos.InstanceIdempotency, ExceptionRepo: repos.ActivityException,
		CalendarPeriodRepo: owner.TemplateRows.CalendarPeriods, Database: arrivalTimetable.NewTenantDatabase(db.Pool()),
		StaffRepo: NewTimetableStaff(people.Staff), ActiveGroupRepo: presence.SessionRecords, SupervisorRepo: supervisions,
		Presence: presence.Presence, ActiveService: active, CareDayLocks: carePlan,
		Materialization: materialization, RecurrenceLock: owner.RecurrenceLock,
		StartConflicts: conflicts, SubstituteConflicts: substituteConflicts,
		Logger: env.Logger.With("service", "instance-lifecycle"), Now: env.Now,
		// Development and test seed profiles create completed instances at
		// the current clock time; dedicated tests cover the policy.
		EnforceTimePolicy: enforceInstanceTimePolicy(env.Config.AppEnv),
	}, rows, schedules.CareDay, notices, settings, hub)
	if err != nil {
		return TimetableLifecycle{}, fmt.Errorf("compose timetable instance lifecycle: %w", err)
	}
	deviations, err := buildTimetableDeviations(db, env, hub, rows, people, supervisions, instances)
	if err != nil {
		return TimetableLifecycle{}, err
	}
	return TimetableLifecycle{
		Rows: rows, Materialization: materialization, Conflicts: conflicts, Instances: instances, Deviations: deviations,
		BlockMetadata: owner.BlockMetadata,
	}, nil
}

// buildTimetableDeviations binds the deviation writes: the cancel branch and
// the understaffed acknowledgement hand over to the lifecycle.
func buildTimetableDeviations(db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, rows arrivalTimetable.OwnerRows, people PeopleRows, supervisions studentpresence.SupervisionRecords, instances timetablePorts.InstanceLifecycle) (timetable.StaffDeviations, error) {
	deps, err := TimetableStaffDeviationDependencies(rows, NewTimetableStaff(people.Staff), supervisions,
		instances.DeviationLifecycle(), hub, env.Logger.With("service", "timetable-deviations"), env.Now)
	if err != nil {
		return nil, fmt.Errorf("compose timetable deviations: %w", err)
	}
	deps.Locks = arrivalTimetable.NewTenantDatabase(db.Pool())
	deviations, err := arrivalTimetable.NewStaffDeviations(deps)
	if err != nil {
		return nil, fmt.Errorf("compose timetable deviations: %w", err)
	}
	return deviations, nil
}

// BuildTimetableDay binds the retention sweep, the automatic start and end
// of instances and the operational day over Care Plan's schedules. The
// operational day resolves the acting account's staff ID through School
// Membership alone (#2753).
func BuildTimetableDay(
	env RootEnvironment, hub *realtime.Hub, lifecycle TimetableLifecycle, timetableOwner timetable.Capability, audit AuditOwner,
	settings settingsCompose.BatchSettingsService, presence PresenceOwner, supervisions studentpresence.SupervisionRecords,
	users peoplePorts.PersonDirectory, membership schoolmembership.Capability, active studentpresence.Presence, schedules CarePlanSchedules, session timetablePorts.RequestSession, withStaff func(context.Context, int64, int64) context.Context,
) (TimetableLifecycle, error) {
	cleanup, err := NewTimetableCleanup(timetableOwner, audit.Persistence.DataDeletion, audit.Persistence.DeviationEvent,
		settings, env.Logger.With("service", "timetable-cleanup"), env.Now)
	if err != nil {
		return TimetableLifecycle{}, fmt.Errorf("compose timetable cleanup: %w", err)
	}
	autoStart, autoEnd, err := arrivalTimetable.NewLifecycleSchedulers(
		lifecycle.Rows, lifecycle.Instances, lifecycle.Conflicts, env.Logger.With("service", "timetable-auto-start"),
	)
	if err != nil {
		return TimetableLifecycle{}, fmt.Errorf("compose timetable auto start and end: %w", err)
	}
	operations, err := NewTimetableOperations(timetablePorts.OperationDependencies{
		Session: session, PlanningTracks: timetableOwner, BlockMetadata: lifecycle.BlockMetadata,
		Sessions: presence.SessionRecords, Supervisions: supervisions,
		Visits: presence.Presence, People: TimetableOperationPeople{OperationPeople: NewTimetableOperationPeople(users), Membership: membership},
		Lifecycle: arrivalTimetable.NewOperationLifecycle(lifecycle.Instances),
		Logger:    env.Logger.With("service", "timetable-operations"), Now: env.Now, Homework: timetableOwner,
	}, lifecycle.Rows, active, settings, schedules.CareDay, schedules.Arrivals, schedules.Pickups, hub, withStaff)
	if err != nil {
		return TimetableLifecycle{}, fmt.Errorf("compose timetable operations: %w", err)
	}
	lifecycle.Cleanup, lifecycle.AutoStart, lifecycle.AutoEnd, lifecycle.Operations = cleanup, autoStart, autoEnd, operations
	return lifecycle, nil
}

// WorkforceShiftRows serves the Timetable conflict detection's shift reads
// from Workforce.
func WorkforceShiftRows(workTime workforce.Capability) timetablePorts.ConflictShifts {
	return newWorkforceShiftRows(workTime)
}
