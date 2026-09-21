package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	sessionEndCompose "github.com/moto-nrw/project-phoenix/workflows/sessionend/compose"
	"github.com/uptrace/bun"
)

func (ActiveTestModule) WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return WithAttendanceStaff(ctx, staffID, tenantID)
}

type activeTestYard struct {
	facilities.SchulhofService
	supervisiondashboard.Yard
}

type ActiveTestModule struct {
	GroupsTestModule
	IoTDataTestModule
	Settings             config.SettingsService
	Schulhof             activeTestYard
	PickupSchedule       careplan.PickupScheduleService
	ArrivalSchedule      careplan.ArrivalScheduleService
	TimetableOperations  timetableplanning.TimetableOperationsService
	CareDay              careplan.CareDayQuery
	Instance             timetableplanning.InstanceService
	SupervisionDashboard supervisiondashboard.Query
	// SessionEnd is the kiosk session end workflow over the real owners.
	SessionEnd       sessionend.Command
	SessionLifecycle devicescanCompose.SessionLifecycle
}

func (m ActiveTestModule) AttendancePeople() attendanceRoutePeople {
	return NewAttendanceRoutePeople(m.Users)
}

func (m ActiveTestModule) AttendanceStaff() attendanceRouteStaff {
	return NewAttendanceRouteStaff(m.UserContext)
}

// PresenceOperations binds the retained active service behind the presence
// operations contract the active routes consume.
func (m ActiveTestModule) PresenceOperations() presenceservice.PresenceOperations {
	return NewPresenceOperations(m.Active, nil, slog.Default())
}

func NewActiveTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (ActiveTestModule, error) {
	r, err := repositories.NewActiveTestRepositories(db, clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
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
	devices, err := repositories.NewDeviceRepository(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	tt, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return ActiveTestModule{}, err
	}
	logger := slog.Default()
	hub := deliveryCompose.NewRealtimeHub(logger)
	students, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	carePlan, err := repositories.NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return ActiveTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivals(db, func(arrivalTimetable.Observation) {})
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
	bridge := timetableplanning.NewTimetableBridgeService(timetableplanning.TimetableBridgeDependencies{Instances: r.ActivityInstance, InstanceStudents: r.InstanceStudent, CareDays: careDay})
	displayGroups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return ActiveTestModule{}, err
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(r.ActiveGroup)
	presenceDeps := presenceservice.PresenceDependencies{
		PrincipalReader: AttendancePrincipal,
		StudentDisplay:  studentDisplayProjection{students: students, groups: displayGroups},
		SchoolPresence:  newStudentPresence(db, logger),
		YardRoomColor:   yardRoomColorQuery(rooms),
		GroupRepo:       sessionGroups, SessionStartLock: r.SessionStartLock, SupervisorRepo: sessionSupervisors,
		StudentStatusRepo: r.StudentStatusDay, CrossTenantRepo: r.CrossTenant, Schools: newActiveSchoolQuery(organizations),
		StudentRepo: PresenceStudents(db, r.Student), StaffRepo: NewAttendanceStaffDirectory(r.Staff), RoomRepo: NewAttendanceRooms(r.Room),
		ActivityGroupRepo: repositories.NewSessionActivities(r.ActivityGroup), ActivityCatRepo: NewAttendanceActivityCategories(r.ActivityCategory), EducationGroupRepo: NewAttendanceEducationGroups(r.Group, r.Student), DeviceRepo: NewSessionDeviceDirectory(devices, settings.Settings, logger),
		StaffNames: NewAttendanceStaffNames(r.Staff, data.Users), DB: db, Broadcaster: hub, WorkSessionService: work.WorkSession,
		AttendanceSyncer:         timetableplanning.NewAttendanceSyncService(r.ActivityInstance, r.InstanceStudent, logger),
		TimetableBridgeCompleter: bridge, Logger: logger, Now: optionalClock(clocks),
	}
	presence := presenceservice.NewPresence(presenceDeps, presenceservice.WithPresenceSettings(PresenceSettings(settings.Settings)))
	groups.Active = presence
	groups.Users = data.Users
	yard := facilities.NewSchulhofService(data.Facilities, facilitiesLegacy.ActivityCatalog(data.Activities), facilitiesLegacy.OpenGroupCatalog(facilitiesGroupSupervisions(newStudentPresence(db, logger)), facilitiesRoomSessions(newStudentPresence(db, logger)), facilitiesGroupVisits(newStudentPresence(db, logger))), logger)
	autoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db, Records: carePlan, Baselines: pickup, Blocks: r.Timetable, Preview: pickupExcusalTimetable{r.Timetable},
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	pickups, err := NewPickupSchedules(db, carePlan, students, pickup, autoExcusal, logger)
	if err != nil {
		return ActiveTestModule{}, err
	}
	classExceptions, err := NewClassArrivalExceptions(classArrivalQueries, students)
	if err != nil {
		return ActiveTestModule{}, err
	}
	arrivals, err := NewArrivalSchedules(db, carePlan, students, arrival, ClassArrivalPlans(classArrivalQueries), classExceptions, logger)
	if err != nil {
		return ActiveTestModule{}, err
	}
	operations := timetableplanning.NewTimetableOperationsService(timetableplanning.TimetableOperationsDependencies{
		InstanceRepo: r.ActivityInstance, InstanceStaffRepo: r.InstanceStaff, InstanceStudents: r.InstanceStudent, InstanceService: tt.Instance,
		ActiveGroupRepo: r.ActiveGroup, ActivityGroupRepo: r.ActivityGroup, ActiveService: presence,
		ArrivalService: arrivals, PickupService: pickups, CareDayService: careDay, SupervisorRepo: r.GroupSupervisor, Presence: newStudentPresence(db, logger),
		StudentRepo: r.Student, EducationGroupRepo: r.Group, RoomRepo: r.Room, PersonService: data.Users, PlanningTrackRepo: r.PlanningTrack,
		Settings: settings.Settings, Broadcaster: hub, DB: db, Logger: logger, Now: optionalClock(clocks), RecoveryRepo: repositories.NewActivityRecoveryRepository(db, r.InstanceStudent),
	})
	timetableOwner, err := repositories.NewTimetable(db, students, rooms, carePlan)
	if err != nil {
		return ActiveTestModule{}, err
	}
	dashboard, err := supervisiondashboardlegacy.New(supervisiondashboardlegacy.Sources{Active: presence, ActiveGroups: openRoomSessionPresence{newStudentPresence(db, logger), timetableOwner},
		OpenVisits: presence, Rooms: openRoomDirectory{rooms: rooms}, UserContext: supervisionCaller{groups.UserContext}, Education: groups.Education,
		Schulhof: yard, Operations: operations, Settings: settings.Settings, Pickups: pickups, Arrivals: arrivals, Now: optionalClock(clocks)})
	if err != nil {
		return ActiveTestModule{}, fmt.Errorf("compose supervision dashboard projection: %w", err)
	}
	sessionEnd, err := sessionEndCompose.New(sessionEndCompose.Dependencies{
		Presence: newStudentPresence(db, logger), Timetable: timetableOwner, Completion: bridge, Students: students, Rooms: rooms,
		Broadcaster: hub, Observe: func(sessionEndCompose.Observation) {}, Now: optionalClock(clocks),
	})
	if err != nil {
		return ActiveTestModule{}, err
	}
	return ActiveTestModule{GroupsTestModule: groups, IoTDataTestModule: data, Settings: settings.Settings, Schulhof: activeTestYard{SchulhofService: yard, Yard: supervisiondashboardlegacy.NewYard(yard)},
		PickupSchedule: pickups, ArrivalSchedule: arrivals, TimetableOperations: operations, SupervisionDashboard: dashboard, CareDay: careDay, Instance: tt.Instance,
		SessionEnd: sessionEnd, SessionLifecycle: devicescanCompose.NewSessionLifecycle(presence, devicescanCompose.NewSupervisionQuery(newStudentPresence(db, logger)), data.Users, data.IoT, nil, logger)}, nil
}
