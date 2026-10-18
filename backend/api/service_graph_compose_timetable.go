package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// bookingsAuthoritative reports whether the approved bookings supply the
// care days (enrollment.bookings_authoritative, #2414, ADR 0005).
func (b *graphBuild) bookingsAuthoritative(ctx context.Context) (bool, error) {
	return b.settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
}

// composeCareDays binds the pickup and arrival baselines, the care-day
// derivation (#1747) and the completion of instances whose session somebody
// else ended. The care days intersect the timetable assignments with the
// children's care plans and are shared by every reader.
func composeCareDays(b graphBuild) (graphBuild, error) {
	approved := b.Enrollment.ApprovedOfferings
	pickupBaselines, err := careplanCompose.NewPickupBaselines(b.CarePlan, approved, b.bookingsAuthoritative)
	if err != nil {
		return b, err
	}
	// The arrival mirror: the class timetable supplies the regular time and,
	// with enrollment.bookings_authoritative on, the approved bookings supply
	// the care days, so a stale row on an unbooked weekday stops marking a
	// child expected.
	arrivalBaselines, err := b.CarePlanWiring.ArrivalBaselines(approved, b.bookingsAuthoritative)
	if err != nil {
		return b, err
	}
	b.pickupBaselines, b.arrivalBaselines = pickupBaselines, arrivalBaselines
	b.graph.CareDay = careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{
		ArrivalBaselines: arrivalBaselines, Records: b.CarePlan, PickupBaselines: pickupBaselines,
	})
	repos := b.TimetableWiring.Repositories
	b.timetableRows = arrivalTimetable.OwnerRows{
		Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff, Participants: repos.InstanceStudent,
		Templates: repos.ActivityGroup, Categories: repos.ActivityCategory, Students: b.People.Students,
		EducationGroups:  b.TimetableWiring.EducationGroupNames,
		Rooms:            facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRepository(b.Rooms)),
		PickupExceptions: b.People.PickupExceptions, ArrivalExceptions: b.People.ArrivalExceptions,
	}
	b.timetableRows.DeviationEvents, b.timetableRows.Deviations = b.TimetableWiring.DeviationPorts(b.audits.DeviationEvent)
	// The single entry point for completing instances whose active group
	// somebody else ended finalizes attendance first, so a completed
	// instance never keeps genuinely expected rows (#1747).
	bridge, err := b.TimetableWiring.EndedSessionCompletion(b.timetableRows, b.graph.CareDay)
	if err != nil {
		return b, fmt.Errorf("compose ended session completion: %w", err)
	}
	b.graph.TimetableBridge = bridge
	return b, nil
}

// composePresence binds the active service: Student Presence with its
// attendance mirror, the session completion, the NFC auto-check-in and the
// parent event emitter that wakes the guardians.
func composePresence(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	// The Timetable owner's attendance mirror (WP-B10, #3424 slice S3)
	// serves Student Presence's attendance syncer port.
	attendanceSync, err := b.TimetableWiring.AttendanceMirror(
		arrivalTimetable.OwnerRows{Instances: repos.ActivityInstance, Participants: repos.InstanceStudent},
		b.Logger.With("service", "attendance-sync"),
	)
	if err != nil {
		return b, fmt.Errorf("compose timetable attendance mirror: %w", err)
	}
	// The chat-pill emitter (#1803) also wakes the guardians after
	// enrollment writes that change a child's live care data.
	emitter := communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		Runtime: NewCommunicationEventRuntime(b.DB.Pool() != nil, b.TenantRuntime), ThreadRepo: b.Platform.ParentPortal.ParentMessageThread,
		MessageRepo: parentStore.NewParentMessageRepository(b.DB.Pool()), Settings: settingsValues.NewParentEventValues(b.settingsService),
		Broadcaster: b.graph.RealtimeHub, Logger: b.Logger.With("service", "parent-events"),
	})
	b.graph.ParentEventEmitter = emitter
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(b.Presence.SessionRecords)
	logger := b.Logger.With("service", "active")
	b.graph.Active = presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: devicescanCompose.AttendancePrincipal, SchoolPresence: b.Presence.Presence,
		StudentDisplay: b.People.StudentDisplay, GroupRepo: sessionGroups, SessionStartLock: b.Presence.Presence,
		SupervisorRepo: sessionSupervisors, StudentStatusRepo: b.CarePlanWiring.StatusDays,
		CrossTenantRepo: b.People.CrossTenantStudents, Schools: b.People.PresenceSchools,
		StudentRepo: b.People.PresenceStudents, StaffRepo: b.Presence.AttendanceStaff,
		RoomRepo:      facilitiesLegacy.NewAttendanceRooms(facilitiesCompose.NewRoomRepository(b.Rooms)),
		YardRoomColor: facilitiesCompose.YardRoomColor(b.Rooms), ActivityGroupRepo: b.Presence.AttendanceActivities,
		ActivityCatRepo:    arrivalTimetable.NewCategoryCount(repos.ActivityCategory),
		EducationGroupRepo: b.Presence.AttendanceEducationGroups, DeviceRepo: b.Presence.SessionDevices(b.settingsService),
		StaffNames: b.Presence.AttendanceStaffNames(b.graph.Users), DB: b.DB.Pool(),
		Broadcaster: NewPresenceEvents(b.graph.RealtimeHub, logger), Tracker: b.ProductEvents,
		WorkSessionService: presenceAutoCheckIn{checkIn: b.graph.WorkSession.AutoCheckIn}, AttendanceSyncer: attendanceSync,
		TimetableBridgeCompleter: b.graph.TimetableBridge, Logger: logger, Now: b.now,
	},
		// Auto-clear of sick / excused flags respects the tenant's
		// operations.sick_clear_mode and operations.excused_clear_mode.
		presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(b.settingsService)),
		// Session commands without a request transaction (scheduler
		// timeouts, daily session end) open their own tenant transaction.
		presenceservice.WithPresenceTenantRuntime(b.TenantRuntime),
		// Anwesenheitswechsel wecken die Sorgeberechtigten, damit der
		// Tagesstatus in der Eltern-App (#2252) live nachlaedt.
		presenceservice.WithGuardianWaker(emitter),
	)
	return b, nil
}

// composeActivities binds the activities service, the care-offering
// catalog and the yard and WC views over the room service.
func composeActivities(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	activities, err := arrivalTimetable.NewService(
		b.Timetable, repos.ActivityGroup, repos.ActivitySchedule, repos.ActivitySupervisor, repos.StudentEnrollment,
		b.Presence.SessionRecords, NewTimetableStaff(b.People.Staff), b.People.Students,
	)
	if err != nil {
		return b, err
	}
	b.graph.Activities = activities
	// Care-offering validation exists before the services that can mutate
	// its recurrence resources. The booking materialization that keeps
	// offering-sourced rosters and the pickup projection in step is composed
	// later; the catalog resolves it on every edit (#2147 review).
	catalog, err := b.CarePlanWiring.OfferingCatalog(CareOfferingCatalogSources{
		Enrollment: b.Enrollment.Bookings, Settings: b.settingsService,
		Resync:         func() careplan.BookingMaterializationCapability { return b.late.careBookings },
		LockRecurrence: b.TimetableWiring.RecurrenceLock.LockRecurrenceWrites,
		Today:          b.today, Logger: b.Logger.With("service", "enrollment-care-offering"),
	})
	if err != nil {
		return b, fmt.Errorf("compose care offering catalog: %w", err)
	}
	b.careOfferings = catalog
	// The retained room service comes from the root (#4010); Facilities owns
	// the room deletion decision behind its room usage port.
	logger := b.Logger.With("service", "facilities")
	b.graph.Schulhof = facilitiesCompose.NewSchulhofService(b.Presence.RoomService, FacilitiesActivityCatalog(activities), b.Presence.OpenGroups, logger)
	b.graph.WC = facilitiesCompose.NewWCService(b.Presence.RoomService, FacilitiesActivityCatalog(activities), logger)
	return b, nil
}

// composeShiftPlanning binds the Dienstplan side of Workforce (#3418):
// shifts, series, shift types, the self-service assignments and the week
// overview over the owner's native rows. The calendar period comes from the
// School Calendar; the timetable blocks, their staffing and the Angebote
// arrive in the Timetable owner's public vocabulary (#3424); the shift_moved
// Änderungsprotokoll entry (#1884) and the SSE invalidation are bound here.
func composeShiftPlanning(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: b.WorkTime, Staff: NewPlanStaffDirectory(b.People.Staff), CalendarPeriods: b.Calendar, ShiftMoveAudit: NewShiftMoveAudit(b.audits.DeviationEvent),
		Instances:      arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:  arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms:          facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRepository(b.Rooms)),
		ActivityGroups: arrivalTimetable.NewTimetableGroupReads(repos.ActivityGroup),
		WorkSchedules:  b.WorkTime,
		WorkModels:     b.WorkTime,
		Holidays:       schoolCalendarCompose.NonWorkingDays{Calendar: b.Calendar}, NonWorkingDays: b.Calendar,
		CategoryLinker: arrivalTimetable.ShiftTypeCategoryLinker(b.graph.Activities.SetCategoryShiftTypeLinks),
		DB:             b.DB.Pool(), Broadcaster: b.graph.RealtimeHub, Logger: b.Logger.With("service", "shift_planning"),
		Today: b.today,
	})
	if err != nil {
		return b, err
	}
	b.shiftPlanning = planning
	b.graph.StaffAssignments = planning.Assignments
	b.graph.StaffScheduleOverview = planning.Overview
	b.graph.ShiftTypes = planning.ShiftTypes
	return b, nil
}

// composeCareSchedules binds the pickup auto excusal (#2360, #3261), the
// pickup schedules and the partial absences of Care Plan, and the Device
// Fleet owner the kiosk reads.
func composeCareSchedules(b graphBuild) (graphBuild, error) {
	excusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: b.DB.Pool(), Records: b.CarePlan, Baselines: b.pickupBaselines, Blocks: b.Presence.Presence,
		Preview: b.CarePlanWiring.PickupExcusalTimetable, Extensions: b.CarePlanWiring.PickupExcusalTimetable,
	})
	if err != nil {
		return b, err
	}
	b.pickupAutoExcusal = excusal
	pickups, err := careplanCompose.NewPickupSchedules(b.DB.Pool(), b.CarePlan, b.pickupBaselines, excusal,
		b.CarePlanWiring.PickupStudents, b.Logger.With("service", "pickup-schedule"))
	if err != nil {
		return b, err
	}
	b.graph.PickupSchedule = pickups
	if b, err = composeDeviceFleet(b); err != nil {
		return b, err
	}
	partialAbsences, err := careplanCompose.NewPartialAbsences(b.DB.Pool(), b.CarePlan, b.Presence.Presence, excusal)
	if err != nil {
		return b, err
	}
	b.graph.PartialAbsence = partialAbsences
	return b, nil
}

// composeDeviceFleet composes the Device Fleet owner (#2676). It owns
// iot.devices and display.displays; the info-point dashboard reads
// Facilities and Student Presence through their public capabilities and the
// remaining cross-owner facts through this owner's consumer-owned ports.
func composeDeviceFleet(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	fleet, err := devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB: b.DB.Pool(), Rooms: b.Rooms, Presence: b.Presence.Presence,
		Dashboard: devicefleetLegacy.NewDashboardSources(devicefleetLegacy.DashboardDependencies{
			ActiveGroups: b.Presence.SessionRecords, Templates: FleetTemplateReader(repos.NativeRecords()), Instances: FleetInstanceReader(repos.NativeRecords()),
			PickupSchedule: b.graph.PickupSchedule,
		}),
		Tenants:             NewDisplayTenantFacts(b.People.DisplaySchools, b.settingsService),
		Now:                 timezone.Now,
		OnlineWindowMinutes: NewOnlineWindowMinutes(b.settingsService, b.Logger.With("module", "device-fleet")),
		APIKeys:             DeviceAPIKeys{},
		Observe: func(observation devicefleetCompose.Observation) {
			b.ObserveDeviceFleet(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, devicefleetModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return b, err
	}
	b.deviceFleet = fleet
	b.graph.IoT = devicefleetCompose.NewIoTService(fleet)
	return b, nil
}

// composeMaterialization binds the recurrence engine (WP-B8): it turns
// activity templates into concrete instances with their staff and students
// for a date window, for the scheduler task and the manual admin endpoint.
func composeMaterialization(b graphBuild) (graphBuild, error) {
	deps := b.TimetableWiring.Materialization(TimetableMaterializationInputs{
		Rows: b.templateRows, CareBounds: b.People.Students,
		// Holidays and closing days carry no series occurrences unless the
		// series opts into closing days (#3594); the School Calendar answers.
		NonWorkingDays: b.Calendar, RecurrenceLock: b.TimetableWiring.RecurrenceLock,
		Broadcaster: b.graph.RealtimeHub, Logger: b.Logger.With("service", "materialization"),
	})
	deps.Transactions = arrivalTimetable.NewTenantTransactions(b.DB.Pool())
	materialization, err := arrivalTimetable.NewMaterialization(deps)
	if err != nil {
		return b, fmt.Errorf("compose timetable materialization: %w", err)
	}
	b.graph.Materialization = materialization
	return b, nil
}

// composeLifecycle binds the Timetable owner's conflict detection and
// staffing (#3550) and the instance lifecycle, which the template split
// later reuses for its deviation snapshots.
func composeLifecycle(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	b.timetableRows.Locks = arrivalTimetable.NewRetainedActivityRecoveryRepository(b.DB.Pool(), repos.InstanceStudent)
	conflicts, err := b.TimetableWiring.ConflictDetection(TimetableConflictReaders{
		Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff, InstanceStudents: repos.InstanceStudent,
		Exceptions: repos.ActivityException, Schedules: repos.ActivitySchedule, Staff: NewTimetableStaff(b.People.Staff),
		CalendarPeriods: b.templateRows.CalendarPeriods, ArrivalExceptions: b.People.ArrivalExceptions,
		Sessions: b.Presence.SessionRecords, Shifts: b.Workforce.ShiftRows, Presence: b.Presence.Presence,
		ArrivalBaselines: b.arrivalBaselines, Logger: b.Logger.With("service", "timetable-conflicts"),
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable conflict detection: %w", err)
	}
	b.timetableConflicts = conflicts
	substituteConflicts, err := arrivalTimetable.NewSubstituteConflictsOverRows(b.timetableRows)
	if err != nil {
		return b, fmt.Errorf("compose timetable substitute conflicts: %w", err)
	}
	instances, err := b.TimetableWiring.Lifecycle(TimetableLifecycleInputs{
		Rows: b.timetableRows,
		Planning: timetablePorts.InstanceLifecycleDependencies{
			IdempotencyRepo: repos.InstanceIdempotency, ExceptionRepo: repos.ActivityException,
			CalendarPeriodRepo: b.templateRows.CalendarPeriods, Database: arrivalTimetable.NewTenantDatabase(b.DB.Pool()),
		},
		Staff: NewTimetableStaff(b.People.Staff), Sessions: b.Presence.SessionRecords, Supervisions: b.groupSupervisions,
		Presence: b.Presence.Presence, SessionEnder: b.graph.Active, CareDays: b.graph.CareDay, CareDayLocks: b.CarePlan,
		// The cancellation notice (#2601) rides on the announcement service,
		// which is composed later.
		GuardianNotices: communicationCompose.LateCareCancellationPublisher(func() communication.CareCancellationPublisher {
			return b.late.guardianNoticePublisher
		}),
		Settings: b.settingsService, Materialization: b.graph.Materialization,
		RecurrenceLock: b.TimetableWiring.RecurrenceLock, StartConflicts: conflicts, SubstituteConflicts: substituteConflicts,
		Broadcaster: b.graph.RealtimeHub, Logger: b.Logger.With("service", "instance-lifecycle"), Now: b.now,
		// Development and test seed profiles create completed instances at
		// the current clock time; dedicated tests cover the policy.
		EnforceTimePolicy: enforceInstanceTimePolicy(b.Config.AppEnv),
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable instance lifecycle: %w", err)
	}
	b.instances = instances
	b.graph.Instance = instances
	return composeDeviations(b)
}

// composeDeviations binds the Timetable owner's deviation writes
// (Vertretungsplan, #3424 slice S3): the cancel branch and the understaffed
// acknowledgement hand over to the lifecycle.
func composeDeviations(b graphBuild) (graphBuild, error) {
	deps, err := b.TimetableWiring.StaffDeviations(TimetableDeviationInputs{
		Rows: b.timetableRows, Staff: NewTimetableStaff(b.People.Staff), Supervisions: b.groupSupervisions,
		Lifecycle: b.instances.DeviationLifecycle(), Broadcaster: b.graph.RealtimeHub,
		Logger: b.Logger.With("service", "timetable-deviations"), Now: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable deviations: %w", err)
	}
	deps.Locks = arrivalTimetable.NewTenantDatabase(b.DB.Pool())
	deviations, err := arrivalTimetable.NewStaffDeviations(deps)
	if err != nil {
		return b, fmt.Errorf("compose timetable deviations: %w", err)
	}
	b.staffDeviations = deviations
	return b, nil
}

// composeCleanups binds the retention sweeps of Timetable (WP-B14), time
// tracking (Tranche 0b) and the per-child change history (#1455), and the
// automatic start and end of instances.
func composeCleanups(b graphBuild) (graphBuild, error) {
	timetableCleanup, err := b.TimetableWiring.Cleanup(TimetableRetentionInputs{
		Owner: b.Timetable, Deletions: b.audits.DataDeletion, Deviations: b.audits.DeviationEvent,
		Settings: b.settingsService, Logger: b.Logger.With("service", "timetable-cleanup"), Clock: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable cleanup: %w", err)
	}
	b.graph.TimetableCleanup = timetableCleanup
	timeTrackingCleanup, err := workforceCompose.NewTimeTrackingCleanup(workforceCompose.TimeTrackingCleanupDependencies{
		Workforce: b.WorkTime, Absences: NewStaffAbsenceRepository(b.WorkTime, b.Membership, b.Persons),
		Audit:    b.Platform.TimeTrackingRetentionAudit(b.audits.DataDeletion),
		Settings: settingsCompose.PresenceSettings(b.settingsService), Logger: b.Logger.With("service", "time-tracking-cleanup"),
	})
	if err != nil {
		return b, fmt.Errorf("compose time tracking cleanup: %w", err)
	}
	b.graph.TimeTrackingCleanup = timeTrackingCleanup
	b.graph.StudentChangeLogCleanup = b.People.StudentChangeLogCleanup(b.audits.StudentFieldEdit, b.audits.DataDeletion,
		b.settingsService, b.Logger.With("service", "student-change-log-cleanup"))
	autoStart, autoEnd, err := arrivalTimetable.NewLifecycleSchedulers(
		b.timetableRows, b.instances, b.timetableConflicts, b.Logger.With("service", "timetable-auto-start"),
	)
	if err != nil {
		return b, fmt.Errorf("compose timetable auto start and end: %w", err)
	}
	b.graph.AutoStart, b.graph.AutoEnd = autoStart, autoEnd
	return b, nil
}

// composeOperations binds Care Plan's arrival schedules and the Timetable
// operational day over them.
func composeOperations(b graphBuild) (graphBuild, error) {
	arrivals, err := careplanCompose.NewArrivalSchedules(b.DB.Pool(), b.CarePlan, b.arrivalBaselines, b.CarePlanWiring.ArrivalRules, b.CarePlanWiring.ArrivalDependencies)
	if err != nil {
		return b, err
	}
	b.graph.ArrivalSchedule = arrivals
	operations, err := b.TimetableWiring.Operations(TimetableOperationInputs{
		Rows: b.timetableRows, PlanningTracks: b.Timetable, Sessions: b.Presence.SessionRecords,
		Supervisions: b.groupSupervisions, Visits: b.Presence.Presence, Presence: b.graph.Active,
		People: newTimetableOperationPeople(b.graph.Users), Settings: b.settingsService, CareDays: b.graph.CareDay,
		Arrivals: arrivals, Pickups: b.graph.PickupSchedule, Lifecycle: arrivalTimetable.NewOperationLifecycle(b.instances),
		Broadcaster: b.graph.RealtimeHub, Logger: b.Logger.With("service", "timetable-operations"), Now: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable operations: %w", err)
	}
	b.graph.TimetableOperations = operations
	return b, nil
}
