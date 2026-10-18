//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

type WorkforceTestModule struct {
	Users      peoplePorts.PersonDirectory
	StaffAdmin *workforce.StaffAdmin
	// TimeTracking is the Workforce time-tracking side composed the way
	// the service graph composes it (#3413).
	TimeTracking *workforceCompose.TimeTracking
	Settings     settingsCompose.SettingsService
}

func NewWorkforceTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (WorkforceTestModule, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	repos, err := NewWorkforceTestRepositories(db, command, clocks...)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	logger := slog.Default()
	identityAccess, err := lifecycleTestModule(db, unit, GuardianInvitationTestConfig{Audit: command, Logger: logger})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	settingsService := settings.Settings
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	usersService := peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
		StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
		PersonRepo:       repos.Person, RFIDRepo: NewPeopleIdentityAccess(db, nil), AccountExists: identityaccessCompose.AccountExists(identityAccess), StudentRepo: repos.Student,
		TeacherRepo:    repos.Teacher,
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: repos.Person, Staff: repos.Staff, Teachers: repos.Teacher, LehrkraftRoles: identityAccess, MayManageStaff: StaffAdministrationPermitted}),
	})
	staffAdmin, err := NewStaffAdmin(StaffAdminDependencies{
		DB: db, Staff: repos.Staff, Persons: repos.Person, Membership: repos.Membership,
		MasterDataAudit: repos.StaffMasterDataChange, PersonnelNumber: repos.PersonnelNumberChange, DataAccessLog: repos.DataAccessLog, Logger: logger,
	})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	calendarAdministration := SchoolCalendarAdministration(settingsService, func(context.Context) error { return nil }, nil)
	calendar, err := NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return WorkforceTestModule{}, err
	}
	nonWorkingDayService := schoolCalendarCompose.NonWorkingDays{Calendar: calendar}
	today := timezone.CalendarDateClock(OptionalClock(clocks))
	timetable, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	// The #1843 sick cascade is the shift-plan-sync workflow over the
	// Workforce planning composition and the timetable test module, bound
	// the way the service graph binds it (#3418).
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: repos.StaffAbsenceType, Staff: NewPlanStaffDirectory(repos.Staff), CalendarPeriods: repos.SchoolCalendar(), ShiftMoveAudit: NewShiftMoveAudit(repos.DeviationEvent),
		Instances: timetableCompose.NewTimetableInstanceReads(repos.ActivityInstance), InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms: facilitiesCompose.NewRoomNameDirectory(repos.Room), ActivityGroups: timetableCompose.NewTimetableGroupReads(repos.ActivityGroup),
		WorkSchedules: repos.Workforce, WorkModels: repos.Workforce, Holidays: nonWorkingDayService,
		DB: db.Pool(), Broadcaster: realtimeHub, Logger: logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	shiftPlanSyncer, err := shiftplansyncCompose.NewSickCascade(shiftplansyncCompose.SickCascadeDependencies{
		Planning: planning.Planning(nil), Workforce: repos.StaffAbsenceType, LockStaffShifts: workforceCompose.NewStaffShiftLock(db.Pool()),
		Deviations: timetable.TimetableData.Deviations, TimetableData: repos.OwnerRows().SickCascadeRows(timetableCompose.SubstituteDayLock(db.Pool())),
		InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(repos.InstanceStaff), Broadcaster: realtimeHub, Logger: logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	timeTracking, err := newTimeTrackingTestModule(db, repos.WorkSessionTestRepositories, timeTrackingTestGraph{
		Audits: timeTrackingAudits{
			WorkSessionEdit: repos.WorkSessionEdit, TimeTrackingDeletion: repos.TimeTrackingDeletion,
			DataAccessLog: repos.DataAccessLog, TimeTrackingAuditLog: repos.TimeTrackingAuditLog,
		},
		Settings: settingsService, People: usersService, Holidays: nonWorkingDayService,
		Events:        timeTrackingEvents(realtimeHub),
		ShiftPlanSync: shiftPlanSyncer,
		Logger:        logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	return WorkforceTestModule{Users: usersService, StaffAdmin: staffAdmin, TimeTracking: timeTracking, Settings: settingsService}, nil
}

// MustNewStaffEmployment is workforceCompose.MustNewStaffEmployment for the
// test graphs outside this package whose test roles may not name the Workforce
// composition.
func MustNewStaffEmployment(db transactionruntime.Database) workforce.StaffEmployments {
	return workforceCompose.MustNewStaffEmployment(db.Pool())
}

// StaffAdminDependencies are the collaborators of the personnel-record
// administration for the test graphs outside this package whose test roles
// may not name the Workforce composition.
type StaffAdminDependencies struct {
	DB         transactionruntime.Database
	Staff      peoplePorts.StaffRepository
	Persons    peoplePorts.PersonRepository
	Membership schoolmembership.Capability

	MasterDataAudit auditModels.StaffMasterDataChangeCreator
	PersonnelNumber auditModels.PersonnelNumberChangeCreator
	DataAccessLog   auditModels.DataAccessLogRepository
	Observe         func(workforceCompose.Observation)
	Logger          *slog.Logger
	Now             func() time.Time
}

// NewStaffAdmin composes the personnel-record administration over one flat
// dependency set.
func NewStaffAdmin(deps StaffAdminDependencies) (*workforce.StaffAdmin, error) {
	people, err := NewPeopleDirectory(deps.DB)
	if err != nil {
		return nil, err
	}
	return composeStaffAdmin(workforceCompose.StaffAdminDependencies{
		DB: deps.DB.Pool(), Observe: deps.Observe, Logger: deps.Logger, Now: deps.Now,
	}, deps.Staff, deps.Persons, people, deps.Membership, deps.MasterDataAudit, deps.PersonnelNumber, deps.DataAccessLog)
}

// StaffAbsenceRow and AbsenceRequestFilter name the Workforce rows the staff
// absence decorators read and write, for the suites outside this package
// whose test roles may not name the Workforce composition.
type (
	StaffAbsenceRow      = workforceCompose.StaffAbsence
	AbsenceRequestFilter = workforceCompose.AbsenceRequestFilter
)
