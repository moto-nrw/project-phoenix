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
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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

// ActiveTestModule is the Care Plan side of the active graph the presence,
// room, check-in and timetable suites run over: the pickup and arrival
// schedules and the care days. The other owners of the same graph come from
// the NewActiveTest* constructors over the same arguments (#4080); each
// composes the graph and returns its owner's part.
type ActiveTestModule struct {
	PickupSchedule  careplan.PickupScheduleService
	ArrivalSchedule careplan.ArrivalScheduleService
	CareDay         careplan.CareDayQuery
}

// ActiveTimetable is the Timetable side of the active graph.
type ActiveTimetable struct {
	TimetableOperations timetable.OperationCapability
	Instance            timetablePorts.InstanceLifecycle
	// Deviations are the Timetable owner's deviation writes over the same
	// repositories (#3424 slice S3).
	Deviations timetable.StaffDeviations
	Activities arrivalTimetable.ActivityService
	Timetable  timetable.Capability
}

// ActiveSettings are the tenant settings of the active graph.
type ActiveSettings struct {
	Settings settingsCompose.BatchSettingsService
}

// ActivePresence is the Student Presence side of the active graph.
type ActivePresence struct {
	Active studentpresence.Presence
}

// PresenceOperations binds the retained active service behind the presence
// operations contract the active routes consume.
func (m ActivePresence) PresenceOperations() presenceservice.PresenceOperations {
	operations, err := presenceservice.NewPresenceOperations(m.Active, nil, slog.Default())
	if err != nil {
		panic(err)
	}
	return operations
}

// ActiveEducation is the School Structure side of the active graph.
type ActiveEducation struct {
	Education schoolstructure.GroupManagement
}

// TeacherGroupIDs exposes the same assignment projection as attendance composition.
func (m ActiveEducation) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	return NewAttendanceTeacherGroups(m.Education).TeacherGroupIDs(ctx, teacherID)
}

// ActivePeople is the People Directory side of the active graph.
type ActivePeople struct {
	Users  peoplePorts.PersonDirectory
	People peopleCompose.GroupRoutePeople
}

func (m ActivePeople) AttendancePeople() AttendanceRoutePeople {
	return NewAttendanceRoutePeople(m.Users)
}

// NewActiveTestAttendanceStaff is the attendance routes' staff port over the
// caller rows of the active graph.
func NewActiveTestAttendanceStaff(caller *CallerRows) peopleCompose.AttendanceRouteStaff {
	return peopleCompose.NewAttendanceRouteStaff(caller)
}

// ActiveDeviceScan is the device-scan side of the active graph.
type ActiveDeviceScan struct {
	SessionLifecycle devicescanCompose.SessionLifecycle
	RoomAvailability devicescanCompose.RoomAvailability
	Configuration    devicescanCompose.ConfigurationQuery
	Directory        devicescanCompose.Directory
	TagAssignments   devicescanCompose.TagAssignments
}

// ActiveFacilities is the Facilities side of the active graph.
type ActiveFacilities struct {
	Facilities facilitiesCompose.RoomService
	// RoomUsage answers the room deletion's usage port over the same
	// sessions and care offerings, as the root binds it.
	RoomUsage       facilitiesCompose.RoomUsage
	SchulhofService facilitiesCompose.SchulhofService
}

// ActiveSupervision is the supervision dashboard side of the active graph.
type ActiveSupervision struct {
	Yard                 supervisiondashboard.Yard
	SupervisionDashboard supervisiondashboard.Query
}

// ActiveSessionEnd is the kiosk session end workflow over the real owners.
type ActiveSessionEnd struct {
	SessionEnd sessionend.Command
}

func buildActiveTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (
	ActiveTestModule, ActiveTimetable, ActiveSettings, ActivePresence, ActiveEducation, ActivePeople, *CallerRows,
	ActiveDeviceScan, DeviceTestModule, ActiveFacilities, ActiveSupervision, ActiveSessionEnd, error,
) {
	fail := func(err error) (ActiveTestModule, ActiveTimetable, ActiveSettings, ActivePresence, ActiveEducation, ActivePeople, *CallerRows,
		ActiveDeviceScan, DeviceTestModule, ActiveFacilities, ActiveSupervision, ActiveSessionEnd, error) {
		return ActiveTestModule{}, ActiveTimetable{}, ActiveSettings{}, ActivePresence{}, ActiveEducation{}, ActivePeople{}, nil,
			ActiveDeviceScan{}, DeviceTestModule{}, ActiveFacilities{}, ActiveSupervision{}, ActiveSessionEnd{}, err
	}
	members := MustNewMembershipTestRepositories(db)
	r, err := NewActiveTestRepositories(db, members, clocks...)
	if err != nil {
		return fail(err)
	}
	rt := withTimetableTestAudit(db, NewActiveTestTimetableRows(db, members, clocks...))
	rCare, rRooms := NewActiveTestCarePlanRows(db, members, clocks...), MustNewTimetableRoomRows(db)
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, rt.Enrollment())
	if err != nil {
		return fail(err)
	}
	groups, _, groupPeople, groupCaller, err := NewGroupsTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	data, dataActivities, dataUsers, dataDevices, dataFacilities, dataRoomUsage, err := NewIoTDataTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	work, _, err := NewWorkSessionTestModule(db, unit, clocks...)
	if err != nil {
		return fail(err)
	}
	devices, err := NewDeviceRepository(db)
	if err != nil {
		return fail(err)
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return fail(err)
	}
	tt, _, _, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return fail(err)
	}
	logger := slog.Default()
	hub := deliveryCompose.NewRealtimeHub(logger)
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return fail(err)
	}
	carePlan, err := NewCarePlan(db, students, rt.InstanceStudent)
	if err != nil {
		return fail(err)
	}
	pickup, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return fail(err)
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivals(db.Pool(), func(arrivalTimetable.Observation) {})
	if err != nil {
		return fail(err)
	}
	arrival, err := NewArrivalBaselines(carePlan, students, classArrivalQueries, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return fail(err)
	}
	careDay := careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{ArrivalBaselines: arrival, Records: carePlan,
		PickupBaselines: pickup})
	care, _, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	careplanCompose.WireCareParticipation(careDay, care.CareLifecycle)
	bridge, err := NewTimetableEndedSessionCompletion(rt.OwnerRows(), careDay)
	if err != nil {
		return fail(err)
	}
	attendanceMirror, err := NewTimetableAttendanceMirror(rt.OwnerRows(), logger)
	if err != nil {
		return fail(err)
	}
	displayGroups, err := NewSchoolStructure(db)
	if err != nil {
		return fail(err)
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return fail(err)
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(r.ActiveGroup)
	presenceDeps := presenceservice.PresenceDependencies{
		PrincipalReader: devicescanCompose.AttendancePrincipal,
		StudentDisplay:  StudentDisplayProjection{Students: students, Groups: displayGroups},
		SchoolPresence:  NewLoggedStudentPresence(db, logger),
		YardRoomColor:   facilitiesModule.YardRoomColor(rooms),
		GroupRepo:       sessionGroups, SessionStartLock: r.SessionStartLock, SupervisorRepo: sessionSupervisors,
		StudentStatusRepo: rCare.StudentStatusDay, CrossTenantRepo: r.CrossTenant, Schools: NewActiveSchoolQuery(organizations),
		StudentRepo: PresenceStudents(db, members.Student), StaffRepo: NewAttendanceStaffDirectory(members.Staff), RoomRepo: facilitiesLegacy.NewAttendanceRooms(rRooms.Room),
		ActivityGroupRepo: NewSessionActivities(rt.GroupReads()), ActivityCatRepo: arrivalTimetable.NewCategoryCount(rt.ActivityCategory), EducationGroupRepo: NewAttendanceEducationGroups(NewSchoolStructureSuiteRepositories(db).Group, members.Student), DeviceRepo: NewSessionDeviceDirectory(devices, settings.Settings, logger),
		StaffNames: NewAttendanceStaffNames(members.Staff, dataUsers), DB: db.Pool(), Broadcaster: NewPresenceEvents(hub, logger), WorkSessionService: presenceAutoCheckIn{checkIn: work.AutoCheckIn},
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge, Logger: logger, Now: OptionalClock(clocks),
	}
	presence := presenceservice.NewPresence(presenceDeps, presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings.Settings)))
	yard := facilitiesCompose.NewSchulhofService(dataFacilities, FacilitiesActivityCatalog(dataActivities.Activities), facilitiesLegacy.OpenGroupCatalog(FacilitiesGroupSupervisions(NewLoggedStudentPresence(db, logger)), FacilitiesRoomSessions(NewLoggedStudentPresence(db, logger)), FacilitiesGroupVisits(NewLoggedStudentPresence(db, logger))), logger)
	autoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: carePlan, Baselines: pickup, Blocks: NewLoggedStudentPresence(db, logger), Preview: NewPickupExcusalTimetable(rt.Timetable, NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return fail(err)
	}
	pickupStudents, err := PickupPlanStudents(students)
	if err != nil {
		return fail(err)
	}
	pickups, err := careplanCompose.NewPickupSchedules(db.Pool(), carePlan, pickup, autoExcusal, pickupStudents, logger)
	if err != nil {
		return fail(err)
	}
	classExceptions, err := NewClassArrivalExceptions(classArrivalQueries, students)
	if err != nil {
		return fail(err)
	}
	arrivalRules, arrivalDeps, err := ArrivalScheduleBinding(students, ClassArrivalPlans(classArrivalQueries), classExceptions, logger)
	if err != nil {
		return fail(err)
	}
	arrivals, err := careplanCompose.NewArrivalSchedules(db.Pool(), carePlan, arrival, arrivalRules, arrivalDeps)
	if err != nil {
		return fail(err)
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return fail(err)
	}
	operationRows := rt.OwnerRows()
	operationRows.Locks = arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), rt.InstanceStudent)
	operations, err := NewTimetableOperations(timetablePorts.OperationDependencies{
		Lifecycle: arrivalTimetable.NewOperationLifecycle(tt.Instance), Sessions: r.ActiveGroup,
		Supervisions: r.GroupSupervisor, Visits: NewLoggedStudentPresence(db, logger),
		People:         TimetableOperationPeople{OperationPeople: newTimetableOperationPeople(dataUsers), Membership: membership},
		PlanningTracks: rt.Timetable, Logger: logger, Now: OptionalClock(clocks),
	}, operationRows, presence, settings.Settings, careDay, arrivals, pickups, hub)
	if err != nil {
		return fail(err)
	}
	timetableOwner, err := NewUnobservedTimetable(db, students, rooms)
	if err != nil {
		return fail(err)
	}
	caller := SupervisionCaller{CallerRows: groupCaller}
	dashboard, err := newSupervisionDashboard(supervisiondashboard.Dependencies{Access: newSupervisionAccess(settings.Settings, caller),
		Sessions: newSupervisionSessions(presence, OpenRoomSessionPresence{Presence: NewLoggedStudentPresence(db, logger), Activities: timetableOwner}, caller),
		Rooms:    OpenRoomDirectory{Rooms: rooms}, Groups: newSupervisionGroups(groups.Education, caller),
		Yard: newSupervisionYard(yard), Schedule: newSupervisionSchedule(operations), Presence: newSupervisionPresence(presence, presence),
		Planning: newSupervisionPlanning(pickups, arrivals), Settings: newSupervisionSettings(settings.Settings), Calendar: supervisionCalendar{}, Now: OptionalClock(clocks)})
	if err != nil {
		return fail(fmt.Errorf("compose supervision dashboard projection: %w", err))
	}
	sessionEnd, err := sessionEndCompose.New(sessionEndCompose.Dependencies{
		Presence: NewLoggedStudentPresence(db, logger), Sessions: NewLoggedStudentPresence(db, logger), Timetable: timetableOwner, Completion: bridge, Students: students, Rooms: rooms,
		Broadcaster: hub, Observe: func(sessionEndCompose.Observation) {}, Now: OptionalClock(clocks),
	})
	if err != nil {
		return fail(err)
	}
	scan := ActiveDeviceScan{
		SessionLifecycle: devicescanCompose.NewSessionLifecycle(presence, devicescanCompose.NewSupervisionQuery(NewLoggedStudentPresence(db, logger)), NewDeviceScanPeople(dataUsers), dataDevices.IoT, nil, logger),
		RoomAvailability: data.RoomAvailability, Configuration: data.Configuration, Directory: data.Directory, TagAssignments: data.TagAssignments,
	}
	return ActiveTestModule{PickupSchedule: pickups, ArrivalSchedule: arrivals, CareDay: careDay},
		ActiveTimetable{TimetableOperations: operations, Instance: tt.Instance, Deviations: tt.TimetableData.Deviations, Activities: dataActivities.Activities, Timetable: dataActivities.Timetable},
		ActiveSettings{Settings: settings.Settings}, ActivePresence{Active: presence}, ActiveEducation{Education: groups.Education},
		ActivePeople{Users: dataUsers, People: groupPeople.People}, groupCaller, scan, dataDevices,
		ActiveFacilities{Facilities: dataFacilities, RoomUsage: dataRoomUsage, SchulhofService: yard},
		ActiveSupervision{Yard: newSupervisionYard(yard), SupervisionDashboard: dashboard}, ActiveSessionEnd{SessionEnd: sessionEnd}, nil
}

// NewActiveTestModule composes the active graph and returns its Care Plan
// side.
func NewActiveTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (ActiveTestModule, error) {
	care, _, _, _, _, _, _, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	return care, err
}

func mustActiveTestModule(err error) {
	if err != nil {
		panic(fmt.Sprintf("compose active test module: %v", err))
	}
}

// NewActiveTestTimetable composes the active graph and returns its ActiveTimetable part.
func NewActiveTestTimetable(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveTimetable {
	_, part, _, _, _, _, _, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestSettings composes the active graph and returns its ActiveSettings part.
func NewActiveTestSettings(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveSettings {
	_, _, part, _, _, _, _, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestPresence composes the active graph and returns its ActivePresence part.
func NewActiveTestPresence(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActivePresence {
	_, _, _, part, _, _, _, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestEducation composes the active graph and returns its ActiveEducation part.
func NewActiveTestEducation(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveEducation {
	_, _, _, _, part, _, _, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestPeople composes the active graph and returns its ActivePeople part.
func NewActiveTestPeople(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActivePeople {
	_, _, _, _, _, part, _, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestCaller composes the active graph and returns its CallerRows part.
func NewActiveTestCaller(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) *CallerRows {
	_, _, _, _, _, _, part, _, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestDeviceScan composes the active graph and returns its ActiveDeviceScan part.
func NewActiveTestDeviceScan(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveDeviceScan {
	_, _, _, _, _, _, _, part, _, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestDevices composes the active graph and returns its DeviceTestModule part.
func NewActiveTestDevices(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) DeviceTestModule {
	_, _, _, _, _, _, _, _, part, _, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestFacilities composes the active graph and returns its ActiveFacilities part.
func NewActiveTestFacilities(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveFacilities {
	_, _, _, _, _, _, _, _, _, part, _, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestSupervision composes the active graph and returns its ActiveSupervision part.
func NewActiveTestSupervision(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveSupervision {
	_, _, _, _, _, _, _, _, _, _, part, _, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
}

// NewActiveTestSessionEnd composes the active graph and returns its ActiveSessionEnd part.
func NewActiveTestSessionEnd(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) ActiveSessionEnd {
	_, _, _, _, _, _, _, _, _, _, _, part, err := buildActiveTestModule(db, unit, clocks...)
	mustActiveTestModule(err)
	return part
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
