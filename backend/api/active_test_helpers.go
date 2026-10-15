package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardCompose "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	sessionEndCompose "github.com/moto-nrw/project-phoenix/workflows/sessionend/compose"
)

func (ActiveTestModule) WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return services.WithAttendanceStaff(ctx, staffID, tenantID)
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
	Instance            *arrivalTimetable.InstanceLifecycleService
	// Deviations are the Timetable owner's deviation writes over the same
	// repositories (#3424 slice S3).
	Deviations           timetable.StaffDeviations
	SupervisionDashboard supervisiondashboard.Query
	// SessionEnd is the kiosk session end workflow over the real owners.
	SessionEnd       sessionend.Command
	SessionLifecycle devicescanCompose.SessionLifecycle
}

func (m ActiveTestModule) AttendancePeople() services.AttendanceRoutePeople {
	return services.NewAttendanceRoutePeople(m.Users)
}

func (m ActiveTestModule) AttendanceStaff() peopleCompose.AttendanceRouteStaff {
	return peopleCompose.NewAttendanceRouteStaff(m.UserContext)
}

// PresenceOperations binds the retained active service behind the presence
// operations contract the active routes consume.
func (m ActiveTestModule) PresenceOperations() presenceservice.PresenceOperations {
	return services.NewPresenceOperations(m.Active, nil, slog.Default())
}

func NewActiveTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (ActiveTestModule, error) {
	r, err := NewActiveTestRepositories(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
	r.TimetableTestRepositories = withTimetableTestAudit(db, r.TimetableTestRepositories)
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
	work, err := NewWorkSessionTestModule(db, unit, clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
	devices, err := NewDeviceRepository(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	organizations, err := services.NewOrganizationTenancy(db.Pool())
	if err != nil {
		return ActiveTestModule{}, err
	}
	tt, err := NewTimetableTestModule(db, unit, clocks...)
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
		PrincipalReader: services.AttendancePrincipal,
		StudentDisplay:  StudentDisplayProjection{Students: students, Groups: displayGroups},
		SchoolPresence:  services.NewLoggedStudentPresence(db.Pool(), logger),
		YardRoomColor:   services.YardRoomColorQuery(rooms),
		GroupRepo:       sessionGroups, SessionStartLock: r.SessionStartLock, SupervisorRepo: sessionSupervisors,
		StudentStatusRepo: r.StudentStatusDay, CrossTenantRepo: r.CrossTenant, Schools: NewActiveSchoolQuery(organizations),
		StudentRepo: PresenceStudents(db, r.Student), StaffRepo: services.NewAttendanceStaffDirectory(r.Staff), RoomRepo: services.NewAttendanceRooms(r.Room),
		ActivityGroupRepo: services.NewSessionActivities(r.GroupReads()), ActivityCatRepo: services.NewAttendanceActivityCategories(r.ActivityCategory), EducationGroupRepo: services.NewAttendanceEducationGroups(r.Group, r.Student), DeviceRepo: services.NewSessionDeviceDirectory(devices, settings.Settings, logger),
		StaffNames: services.NewAttendanceStaffNames(r.Staff, data.Users), DB: db.Pool(), Broadcaster: hub, WorkSessionService: work.TimeTracking.AutoCheckIn,
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge, Logger: logger, Now: services.OptionalClock(clocks),
	}
	presence := presenceservice.NewPresence(presenceDeps, presenceservice.WithPresenceSettings(services.PresenceSettings(settings.Settings)))
	groups.Active = presence
	groups.Users = data.Users
	yard := facilitiesCompose.NewSchulhofService(data.Facilities, facilitiesLegacy.ActivityCatalog(data.Activities), facilitiesLegacy.OpenGroupCatalog(services.FacilitiesGroupSupervisions(services.NewLoggedStudentPresence(db.Pool(), logger)), services.FacilitiesRoomSessions(services.NewLoggedStudentPresence(db.Pool(), logger)), services.FacilitiesGroupVisits(services.NewLoggedStudentPresence(db.Pool(), logger))), logger)
	autoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: carePlan, Baselines: pickup, Blocks: services.NewLoggedStudentPresence(db.Pool(), logger), Preview: NewPickupExcusalTimetable(r.Timetable, services.NewPartialAbsencePreview(db.Pool())),
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
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return ActiveTestModule{}, err
	}
	operationRows := r.OwnerRows()
	operationRows.Locks = arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), r.InstanceStudent)
	operations, err := NewTimetableOperations(services.TimetableOperationInputs{
		Rows: operationRows, Lifecycle: tt.Instance.OperationLifecycle(),
		Sessions: r.ActiveGroup, Presence: presence,
		Arrivals: arrivals, Pickups: pickups, CareDays: careDay, Supervisions: r.GroupSupervisor, Visits: services.NewLoggedStudentPresence(db.Pool(), logger),
		People: TimetableOperationPeople{OperationPeople: data.Users, Membership: membership}, PlanningTracks: r.Timetable,
		Settings: settings.Settings, Broadcaster: hub, Logger: logger, Now: services.OptionalClock(clocks),
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	timetableOwner, err := NewUnobservedTimetable(db, students, rooms)
	if err != nil {
		return ActiveTestModule{}, err
	}
	dashboard, err := supervisiondashboardCompose.New(supervisiondashboardCompose.Sources{Active: presence, ActiveGroups: services.OpenRoomSessionPresence{Presence: services.NewLoggedStudentPresence(db.Pool(), logger), Activities: timetableOwner},
		OpenVisits: presence, Rooms: services.OpenRoomDirectory{Rooms: rooms}, UserContext: SupervisionCaller{CallerRows: groups.UserContext}, Education: groups.Education,
		Schulhof: yard, Operations: operations, Settings: settings.Settings, Pickups: pickups, Arrivals: arrivals, Now: services.OptionalClock(clocks)})
	if err != nil {
		return ActiveTestModule{}, fmt.Errorf("compose supervision dashboard projection: %w", err)
	}
	sessionEnd, err := sessionEndCompose.New(sessionEndCompose.Dependencies{
		Presence: services.NewLoggedStudentPresence(db.Pool(), logger), Sessions: services.NewLoggedStudentPresence(db.Pool(), logger), Timetable: timetableOwner, Completion: bridge, Students: students, Rooms: rooms,
		Broadcaster: hub, Observe: func(sessionEndCompose.Observation) {}, Now: services.OptionalClock(clocks),
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	return ActiveTestModule{GroupsTestModule: groups, IoTDataTestModule: data, Settings: settings.Settings, Schulhof: activeTestYard{SchulhofService: yard, Yard: supervisiondashboardCompose.NewYard(yard)},
		PickupSchedule: pickups, ArrivalSchedule: arrivals, TimetableOperations: operations, SupervisionDashboard: dashboard, CareDay: careDay, Instance: tt.Instance, Deviations: tt.TimetableData.Deviations,
		SessionEnd: sessionEnd, SessionLifecycle: devicescanCompose.NewSessionLifecycle(presence, devicescanCompose.NewSupervisionQuery(services.NewLoggedStudentPresence(db.Pool(), logger)), data.Users, data.IoT, nil, logger)}, nil
}
