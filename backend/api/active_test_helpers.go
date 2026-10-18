//go:build testhooks

package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	sessionEndCompose "github.com/moto-nrw/project-phoenix/workflows/sessionend/compose"
)

func (ActiveTestModule) WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return devicescanCompose.WithAttendanceStaff(ctx, staffID, tenantID)
}

type activeTestYard struct {
	facilitiesCompose.SchulhofService
	supervisiondashboard.Yard
}

type ActiveTestModule struct {
	GroupsTestModule
	IoTDataTestModule
	Settings            settingsCompose.BatchSettingsService
	Schulhof            activeTestYard
	PickupSchedule      careplan.PickupScheduleService
	ArrivalSchedule     careplan.ArrivalScheduleService
	TimetableOperations timetable.OperationCapability
	CareDay             careplan.CareDayQuery
	Instance            timetablePorts.InstanceLifecycle
	// Deviations are the Timetable owner's deviation writes over the same
	// repositories (#3424 slice S3).
	Deviations           timetable.StaffDeviations
	SupervisionDashboard supervisiondashboard.Query
	// SessionEnd is the kiosk session end workflow over the real owners.
	SessionEnd       sessionend.Command
	SessionLifecycle devicescanCompose.SessionLifecycle
}

func (m ActiveTestModule) AttendancePeople() AttendanceRoutePeople {
	return NewAttendanceRoutePeople(m.Users)
}

func (m ActiveTestModule) AttendanceStaff() peopleCompose.AttendanceRouteStaff {
	return peopleCompose.NewAttendanceRouteStaff(m.UserContext)
}

// PresenceOperations binds the retained active service behind the presence
// operations contract the active routes consume.
func (m ActiveTestModule) PresenceOperations() presenceservice.PresenceOperations {
	operations, err := presenceservice.NewPresenceOperations(m.Active, nil, slog.Default())
	if err != nil {
		panic(err)
	}
	return operations
}

func NewActiveTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (ActiveTestModule, error) {
	r, err := NewActiveTestRepositories(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
	r.TimetableTestRepositories = withTimetableTestAudit(db, r.TimetableTestRepositories)
	r.TimetableDeviationAudit = NewTimetableDeviationAudit(db)
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return ActiveTestModule{}, err
	}
	groups, err := NewGroupsTestModule(db, unit)
	if err != nil {
		return ActiveTestModule{}, err
	}
	data, err := NewIoTDataTestModule(db, unit)
	if err != nil {
		return ActiveTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return ActiveTestModule{}, err
	}
	work, _, err := NewWorkSessionTestModule(db, unit, clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
	devices, err := NewDeviceRepository(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	tt, _, _, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
	logger := slog.Default()
	hub := deliveryCompose.NewRealtimeHub(logger)
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	carePlan, err := NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return ActiveTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivals(db.Pool(), func(arrivalTimetable.Observation) {})
	if err != nil {
		return ActiveTestModule{}, err
	}
	arrival, err := NewArrivalBaselines(carePlan, students, classArrivalQueries, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	careDay := careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{ArrivalBaselines: arrival, Records: carePlan,
		PickupBaselines: pickup})
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return ActiveTestModule{}, err
	}
	careplanCompose.WireCareParticipation(careDay, care.CareLifecycle)
	bridge, err := NewTimetableEndedSessionCompletion(r.OwnerRows(), careDay)
	if err != nil {
		return ActiveTestModule{}, err
	}
	attendanceMirror, err := NewTimetableAttendanceMirror(r.OwnerRows(), logger)
	if err != nil {
		return ActiveTestModule{}, err
	}
	displayGroups, err := NewSchoolStructure(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return ActiveTestModule{}, err
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(r.ActiveGroup)
	presenceDeps := presenceservice.PresenceDependencies{
		PrincipalReader: devicescanCompose.AttendancePrincipal,
		StudentDisplay:  StudentDisplayProjection{Students: students, Groups: displayGroups},
		SchoolPresence:  NewLoggedStudentPresence(db, logger),
		YardRoomColor:   facilitiesCompose.YardRoomColor(rooms),
		GroupRepo:       sessionGroups, SessionStartLock: r.SessionStartLock, SupervisorRepo: sessionSupervisors,
		StudentStatusRepo: r.StudentStatusDay, CrossTenantRepo: r.CrossTenant, Schools: NewActiveSchoolQuery(organizations),
		StudentRepo: PresenceStudents(db, r.Student), StaffRepo: NewAttendanceStaffDirectory(r.Staff), RoomRepo: facilitiesLegacy.NewAttendanceRooms(r.Room),
		ActivityGroupRepo: NewSessionActivities(r.GroupReads()), ActivityCatRepo: arrivalTimetable.NewCategoryCount(r.ActivityCategory), EducationGroupRepo: NewAttendanceEducationGroups(r.Group, r.Student), DeviceRepo: NewSessionDeviceDirectory(devices, settings.Settings, logger),
		StaffNames: NewAttendanceStaffNames(r.Staff, data.Users), DB: db.Pool(), Broadcaster: NewPresenceEvents(hub, logger), WorkSessionService: presenceAutoCheckIn{checkIn: work.AutoCheckIn},
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge, Logger: logger, Now: OptionalClock(clocks),
	}
	presence := presenceservice.NewPresence(presenceDeps, presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings.Settings)))
	groups.Active = presence
	groups.Users = data.Users
	yard := facilitiesCompose.NewSchulhofService(data.Facilities, FacilitiesActivityCatalog(data.Activities), facilitiesLegacy.OpenGroupCatalog(FacilitiesGroupSupervisions(NewLoggedStudentPresence(db, logger)), FacilitiesRoomSessions(NewLoggedStudentPresence(db, logger)), FacilitiesGroupVisits(NewLoggedStudentPresence(db, logger))), logger)
	autoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: carePlan, Baselines: pickup, Blocks: NewLoggedStudentPresence(db, logger), Preview: NewPickupExcusalTimetable(r.Timetable, NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	pickupStudents, err := PickupPlanStudents(students)
	if err != nil {
		return ActiveTestModule{}, err
	}
	pickups, err := careplanCompose.NewPickupSchedules(db.Pool(), carePlan, pickup, autoExcusal, pickupStudents, logger)
	if err != nil {
		return ActiveTestModule{}, err
	}
	classExceptions, err := NewClassArrivalExceptions(classArrivalQueries, students)
	if err != nil {
		return ActiveTestModule{}, err
	}
	arrivalRules, arrivalDeps, err := ArrivalScheduleBinding(students, ClassArrivalPlans(classArrivalQueries), classExceptions, logger)
	if err != nil {
		return ActiveTestModule{}, err
	}
	arrivals, err := careplanCompose.NewArrivalSchedules(db.Pool(), carePlan, arrival, arrivalRules, arrivalDeps)
	if err != nil {
		return ActiveTestModule{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	operationRows := r.OwnerRows()
	operationRows.Locks = arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), r.InstanceStudent)
	operations, err := NewTimetableOperations(timetablePorts.OperationDependencies{
		Lifecycle: arrivalTimetable.NewOperationLifecycle(tt.Instance), Sessions: r.ActiveGroup,
		Supervisions: r.GroupSupervisor, Visits: NewLoggedStudentPresence(db, logger),
		People:         TimetableOperationPeople{OperationPeople: newTimetableOperationPeople(data.Users), Membership: membership},
		PlanningTracks: r.Timetable, Logger: logger, Now: OptionalClock(clocks),
	}, operationRows, presence, settings.Settings, careDay, arrivals, pickups, hub)
	if err != nil {
		return ActiveTestModule{}, err
	}
	timetableOwner, err := NewUnobservedTimetable(db, students, rooms)
	if err != nil {
		return ActiveTestModule{}, err
	}
	caller := SupervisionCaller{CallerRows: groups.UserContext}
	dashboard, err := newSupervisionDashboard(supervisiondashboard.Dependencies{Access: newSupervisionAccess(settings.Settings, caller),
		Sessions: newSupervisionSessions(presence, OpenRoomSessionPresence{Presence: NewLoggedStudentPresence(db, logger), Activities: timetableOwner}, caller),
		Rooms:    OpenRoomDirectory{Rooms: rooms}, Groups: newSupervisionGroups(groups.Education, caller),
		Yard: newSupervisionYard(yard), Schedule: newSupervisionSchedule(operations), Presence: newSupervisionPresence(presence, presence),
		Planning: newSupervisionPlanning(pickups, arrivals), Settings: newSupervisionSettings(settings.Settings), Calendar: supervisionCalendar{}, Now: OptionalClock(clocks)})
	if err != nil {
		return ActiveTestModule{}, fmt.Errorf("compose supervision dashboard projection: %w", err)
	}
	sessionEnd, err := sessionEndCompose.New(sessionEndCompose.Dependencies{
		Presence: NewLoggedStudentPresence(db, logger), Sessions: NewLoggedStudentPresence(db, logger), Timetable: timetableOwner, Completion: bridge, Students: students, Rooms: rooms,
		Broadcaster: hub, Observe: func(sessionEndCompose.Observation) {}, Now: OptionalClock(clocks),
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	return ActiveTestModule{GroupsTestModule: groups, IoTDataTestModule: data, Settings: settings.Settings, Schulhof: activeTestYard{SchulhofService: yard, Yard: newSupervisionYard(yard)},
		PickupSchedule: pickups, ArrivalSchedule: arrivals, TimetableOperations: operations, SupervisionDashboard: dashboard, CareDay: careDay, Instance: tt.Instance, Deviations: tt.TimetableData.Deviations,
		SessionEnd: sessionEnd, SessionLifecycle: devicescanCompose.NewSessionLifecycle(presence, devicescanCompose.NewSupervisionQuery(NewLoggedStudentPresence(db, logger)), NewDeviceScanPeople(data.Users), data.IoT, nil, logger)}, nil
}

// Session-end HTTP fixtures use the same values the root binds in production.
type iotTestSessionLifecycle = devicescan.SessionLifecycle
type iotTestSessionToEnd = devicescan.SessionToEnd
type iotTestSessionEndResult = sessionend.Result
type iotTestSessionConflict = devicescan.ConflictInfoResponse

func iotSessionEndTestFailures() (notFound, alreadyEnded error) {
	return sessionend.ErrSessionNotFound, sessionend.ErrSessionAlreadyEnded
}

func iotSessionEndTestInvalidRequest(message string) error {
	return devicescan.InvalidRequest(message)
}
