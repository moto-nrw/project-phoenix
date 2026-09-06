package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ActiveTestModule struct {
	GroupsTestModule
	IoTDataTestModule
	Settings             config.SettingsService
	Schulhof             facilities.SchulhofService
	PickupSchedule       schedule.PickupScheduleService
	ArrivalSchedule      schedule.ArrivalScheduleService
	TimetableOperations  schedule.TimetableOperationsService
	CareDay              schedule.CareDayService
	Instance             schedule.InstanceService
	SupervisionDashboard supervisiondashboard.Getter
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
	devices, err := repositories.NewDeviceTestRepository(db)
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
	pickup := schedule.NewPickupBaselineServiceWithSettings(r.StudentPickupSchedule, approvedOfferings, r.CareOffering, settings.Settings)
	arrival := schedule.NewArrivalBaselineService(r.StudentArrivalSchedule, r.Student, r.ClassArrivalTime, r.ClassArrivalException, approvedOfferings, r.CareOffering, settings.Settings)
	careDay := schedule.NewCareDayService(schedule.CareDayDependencies{ArrivalBaselines: arrival, ArrivalSchedules: r.StudentArrivalSchedule,
		ArrivalExceptions: r.StudentArrivalException, PickupBaselines: pickup, PickupExceptions: r.StudentPickupException})
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return ActiveTestModule{}, err
	}
	schedule.WireCareParticipation(careDay, care.CareLifecycle)
	bridge := schedule.NewTimetableBridgeService(schedule.TimetableBridgeDependencies{Instances: r.ActivityInstance, InstanceStudents: r.InstanceStudent, CareDays: careDay})
	presence := active.NewService(active.ServiceDependencies{
		GroupRepo: r.ActiveGroup, SessionStartLock: r.SessionStartLock, VisitRepo: r.ActiveVisit, SupervisorRepo: r.GroupSupervisor,
		CombinedGroupRepo: r.CombinedGroup, GroupMappingRepo: r.GroupMapping, AttendanceRepo: r.Attendance,
		StudentStatusRepo: r.StudentStatusDay, CrossTenantRepo: r.CrossTenant, Schools: newActiveSchoolQuery(organizations),
		StudentRepo: r.Student, PersonRepo: r.Person, TeacherRepo: r.Teacher, StaffRepo: r.Staff, RoomRepo: r.Room,
		ActivityGroupRepo: r.ActivityGroup, ActivityCatRepo: r.ActivityCategory, EducationGroupRepo: r.Group, DeviceRepo: devices,
		EducationService: groups.Education, UsersService: data.Users, DB: db, Broadcaster: hub, WorkSessionService: work.WorkSession,
		AttendanceSyncer:         schedule.NewAttendanceSyncService(r.ActivityInstance, r.InstanceStudent, logger),
		TimetableBridgeCompleter: bridge, Logger: logger, Now: optionalClock(clocks),
	})
	presence.SetSettingsService(settings.Settings)
	groups.Active = presence
	groups.Users = data.Users
	yard := facilities.NewSchulhofService(data.Facilities, facilitiesLegacy.ActivityCatalog(data.Activities), facilitiesLegacy.OpenGroupCatalog(presence), logger)
	autoExcusal := schedule.NewPickupAutoExcusalSyncer(r.StudentPickupException, pickup, r.InstanceStudent, db)
	pickups := schedule.NewPickupScheduleServiceWithBulk(r.StudentPickupSchedule, r.StudentPickupException, r.StudentPickupNote, r.Student, r.Person, autoExcusal, pickup, db, logger)
	arrivals := schedule.NewArrivalScheduleServiceWithBaselines(r.StudentArrivalSchedule, r.StudentArrivalException, r.StudentArrivalNote, r.Student, r.Person, arrival, r.ClassArrivalTime, db, logger, schedule.WithClassArrivalExceptions(r.ClassArrivalException))
	operations := schedule.NewTimetableOperationsService(schedule.TimetableOperationsDependencies{
		InstanceRepo: r.ActivityInstance, InstanceStaffRepo: r.InstanceStaff, InstanceStudents: r.InstanceStudent, InstanceService: tt.Instance,
		ActiveGroupRepo: r.ActiveGroup, ActivityGroupRepo: r.ActivityGroup, ActiveService: presence,
		ArrivalService: arrivals, PickupService: pickups, CareDayService: careDay, SupervisorRepo: r.GroupSupervisor, VisitRepo: r.ActiveVisit,
		StudentRepo: r.Student, EducationGroupRepo: r.Group, RoomRepo: r.Room, PersonService: data.Users, PlanningTrackRepo: r.PlanningTrack,
		Settings: settings.Settings, Broadcaster: hub, DB: db, Logger: logger, Now: optionalClock(clocks), RecoveryRepo: repositories.NewActivityRecoveryRepository(db, r.InstanceStudent),
	})
	dashboard := supervisiondashboard.NewService(supervisiondashboard.Dependencies{Active: presence, UserContext: groups.UserContext, Education: groups.Education,
		Schulhof: yard, Operations: operations, Settings: settings.Settings, Pickups: pickups, Arrivals: arrivals})
	return ActiveTestModule{GroupsTestModule: groups, IoTDataTestModule: data, Settings: settings.Settings, Schulhof: yard,
		PickupSchedule: pickups, ArrivalSchedule: arrivals, TimetableOperations: operations, SupervisionDashboard: dashboard, CareDay: careDay, Instance: tt.Instance}, nil
}
