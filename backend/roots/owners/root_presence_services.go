package owners

import (
	"context"
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableDays are the Timetable owner's rows over the retained adapters
// (#3551) and the completion of instances whose session somebody else
// ended, which finalizes attendance first, so a completed instance never
// keeps genuinely expected rows (#1747).
type TimetableDays struct {
	Rows   arrivalTimetable.OwnerRows
	Bridge timetable.EndedSessionCompletion
}

// BuildTimetableDays binds the Timetable owner rows over the retained
// repositories, the Care Plan exception rows and the Audit deviation trail,
// and the ended-session completion over the care days.
func BuildTimetableDays(owner TimetableOwner, people PeopleRows, rooms facilitiesModule.Catalog, careRows TimetableCareRows, audit AuditOwner, careDay careplan.CareDayQuery) (TimetableDays, error) {
	repos := owner.Repositories
	rows := arrivalTimetable.OwnerRows{
		Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff, Participants: repos.InstanceStudent,
		Templates: repos.ActivityGroup, Categories: repos.ActivityCategory, Students: NewTimetableRosterStudents(people.Students),
		EducationGroups:  owner.EducationGroupNames,
		Rooms:            facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRecords(rooms)),
		PickupExceptions: careRows.PickupExceptions, ArrivalExceptions: careRows.ArrivalExceptions,
	}
	rows.DeviationEvents, rows.Deviations = TimetableDeviationPorts(audit.Persistence.DeviationEvent)
	bridge, err := NewTimetableEndedSessionCompletion(rows, careDay)
	if err != nil {
		return TimetableDays{}, fmt.Errorf("compose ended session completion: %w", err)
	}
	return TimetableDays{Rows: rows, Bridge: bridge}, nil
}

// BuildActivePresence binds the active service: Student Presence with the
// Timetable owner's attendance mirror (WP-B10, #3424 slice S3), the session
// completion, the NFC auto-check-in and the parent event emitter that wakes
// the guardians (#2252).
func BuildActivePresence(
	db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, hub *realtime.Hub,
	settings settingsCompose.BatchSettingsService, presence PresenceOwner, presencePeople PresencePeopleReads,
	carePlanRows CarePlanRows, people PeopleRows, users peoplePorts.PersonDirectory, rooms facilitiesModule.Catalog,
	devices devicefleet.Capability, owner TimetableOwner, days TimetableDays, autoCheckIn workforceCompose.AutoCheckIn,
	emitter eventContract.Emitter, tracker ProductEventTracker, principal func(context.Context) studentpresence.RequestPrincipal,
) (studentpresence.Presence, error) {
	repos := owner.Repositories
	attendanceSync, err := NewTimetableAttendanceMirror(
		arrivalTimetable.OwnerRows{Instances: repos.ActivityInstance, Participants: repos.InstanceStudent},
		env.Logger.With("service", "attendance-sync"),
	)
	if err != nil {
		return nil, fmt.Errorf("compose timetable attendance mirror: %w", err)
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(presence.SessionRecords)
	logger := env.Logger.With("service", "active")
	return presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: principal, SchoolPresence: presence.Presence,
		StudentDisplay: presencePeople.StudentDisplay, GroupRepo: sessionGroups, SessionStartLock: presence.Presence,
		SupervisorRepo: sessionSupervisors, StudentStatusRepo: carePlanRows.StatusDays,
		CrossTenantRepo: presencePeople.CrossTenantStudents, Schools: presencePeople.PresenceSchools,
		StudentRepo: presencePeople.PresenceStudents, StaffRepo: presence.AttendanceStaff,
		RoomRepo:      facilitiesCompose.NewAttendanceRooms(facilitiesCompose.NewRoomRecords(rooms)),
		YardRoomColor: facilitiesModule.YardRoomColor(rooms), ActivityGroupRepo: presence.AttendanceActivities,
		ActivityCatRepo:    arrivalTimetable.NewCategoryCount(repos.ActivityCategory),
		EducationGroupRepo: presence.AttendanceEducationGroups,
		DeviceRepo:         NewSessionDeviceDirectory(devicefleetRepositoryAdapter.NewDeviceRepository(devices), settings, logger),
		StaffNames:         NewAttendanceStaffNames(people.Staff, users), DB: db.Pool(),
		Broadcaster: NewPresenceEvents(hub, logger), Tracker: tracker,
		WorkSessionService: PresenceAutoCheckIn{CheckIn: autoCheckIn}, AttendanceSyncer: attendanceSync,
		TimetableBridgeCompleter: days.Bridge, Logger: logger, Now: env.Now,
	},
		// Auto-clear of sick / excused flags respects the tenant's
		// operations.sick_clear_mode and operations.excused_clear_mode.
		presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings)),
		// Session commands without a request transaction (scheduler
		// timeouts, daily session end) open their own tenant transaction.
		presenceservice.WithPresenceTenantRuntime(unit),
		// Anwesenheitswechsel wecken die Sorgeberechtigten, damit der
		// Tagesstatus in der Eltern-App (#2252) live nachlaedt.
		presenceservice.WithGuardianWaker(emitter),
	), nil
}

// BuildDeviceFleet composes the observed Device Fleet owner (#2676). It owns
// iot.devices and display.displays; the info-point dashboard reads
// Facilities and Student Presence through their public capabilities and the
// remaining cross-owner facts through this owner's consumer-owned ports.
func BuildDeviceFleet(
	db transactionruntime.Database, env RootEnvironment, settings settingsCompose.BatchSettingsService,
	rooms facilitiesModule.Catalog, presence PresenceOwner, owner TimetableOwner,
	pickups careplan.PickupScheduleService, organizations organizationtenancy.Query,
) (devicefleet.Capability, error) {
	repos := owner.Repositories
	observe := env.Observe.DeviceFleet
	return devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB: db.Pool(), Rooms: rooms, Presence: presence.Presence,
		Dashboard: devicefleetCompose.NewDashboardSources(devicefleetCompose.DashboardDependencies{
			ActiveGroups: presence.SessionRecords, Templates: FleetTemplateReader(repos.NativeRecords()), Instances: FleetInstanceReader(repos.NativeRecords()),
			PickupSchedule: pickups,
		}),
		Tenants:             NewDisplayTenantFacts(displaySchoolDirectory{schools: organizations}, settings),
		Now:                 timezone.Now,
		OnlineWindowMinutes: NewOnlineWindowMinutes(settings, env.Logger.With("module", "device-fleet")),
		APIKeys:             DeviceAPIKeys{},
		Observe: func(observation devicefleetCompose.Observation) {
			observe(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, devicefleet.ErrorCode(observation.Err), observation.Err)
		},
	})
}
