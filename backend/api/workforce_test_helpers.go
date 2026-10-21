//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// WorkforceTestModule is the Workforce owner's personnel-record
// administration and time tracking.
type WorkforceTestModule struct {
	StaffAdmin *workforce.StaffAdmin
	// TimeTracking is the Workforce time-tracking side composed the way
	// the service graph composes it (#3413).
	TimeTracking *workforceCompose.TimeTracking
}

// NewWorkforceTestModule composes the Workforce owner over db and returns it
// with the People Directory's person directory it reads staff through.
func NewWorkforceTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (WorkforceTestModule, peoplePorts.PersonDirectory, error) {
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	members := owners.MustNewMembershipTestRepositories(db)
	repos, err := NewWorkforceTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	ttRows, err := owners.NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	ttRows = withTimetableTestAudit(db, ttRows)
	presence, err := NewTimetablePresenceRows(db, ttRows, members, clocks...)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	rooms, err := owners.NewTimetableRoomRows(db)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	schoolCalendar, err := NewTimetableSchoolCalendar(db)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	audits := owners.NewTestAuditPersistence(db, command)
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	logger := slog.Default()
	identityAccess, err := lifecycleTestModule(db, unit, GuardianInvitationTestConfig{Audit: command, Logger: logger})
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	settingsService := settings.Settings
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	usersService := peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  owners.MustNewPeopleDirectory(db).PersonWrites,
		StudentDirectory: owners.MustNewPeopleDirectory(db).StudentDirectory,
		PersonRepo:       members.Person, RFIDRepo: owners.NewPeopleIdentityAccess(db, nil), AccountExists: identityaccessCompose.AccountExists(identityAccess), StudentRepo: members.Student,
		TeacherRepo:    members.Teacher,
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, LehrkraftRoles: identityAccess, MayManageStaff: StaffAdministrationPermitted}),
	})
	staffAdmin, err := NewStaffAdmin(db, members.Staff, members.Person, membership, StaffAdminDependencies{
		MasterDataAudit: audits.StaffMasterDataChange, PersonnelNumber: audits.PersonnelNumberChange, DataAccessLog: audits.DataAccessLog, Logger: logger,
	})
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	calendar, err := NewSchoolCalendarWithAdministration(db, settingsService, func(context.Context) error { return nil }, nil)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	nonWorkingDayService := schoolCalendarCompose.NewNonWorkingDays(calendar)
	today := timezone.CalendarDateClock(OptionalClock(clocks))
	timetable, _, _, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	// The #1843 sick cascade is the shift-plan-sync workflow over the
	// Workforce planning composition and the timetable test module, bound
	// the way the service graph binds it (#3418).
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: repos.StaffAbsenceType, Staff: owners.NewPlanStaffDirectory(members.Staff), CalendarPeriods: schoolCalendar.SchoolCalendar(), ShiftMoveAudit: owners.NewShiftMoveAudit(NewTimetableDeviationAudit(db).DeviationEvent),
		Instances: timetableCompose.NewTimetableInstanceReads(ttRows.ActivityInstance), InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(ttRows.InstanceStaff),
		Rooms: facilitiesCompose.NewRoomNameDirectory(rooms.Room), ActivityGroups: timetableCompose.NewTimetableGroupReads(ttRows.Timetable),
		WorkSchedules: repos.Workforce, WorkModels: repos.Workforce, Holidays: nonWorkingDayService,
		DB: db.Pool(), Broadcaster: realtimeHub, Logger: logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	shiftPlanSyncer, err := shiftplansyncCompose.NewSickCascade(shiftplansyncCompose.SickCascadeDependencies{
		Planning: planning.Planning(nil), Workforce: repos.StaffAbsenceType, LockStaffShifts: workforceCompose.NewStaffShiftLock(db.Pool()),
		Deviations: timetable.TimetableData.Deviations, TimetableData: ttRows.OwnerRows().SickCascadeRows(timetableCompose.SubstituteDayLock(db.Pool())),
		InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(ttRows.InstanceStaff), Broadcaster: realtimeHub, Logger: logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	timeTracking, err := newTimeTrackingTestModule(db, repos.WorkSessionTestRepositories, members.Staff, presence, settingsService, timeTrackingTestGraph{
		Audits: owners.TimeTrackingAudits{
			WorkSessionEdit: owners.NewTestAuditPersistence(db, nil).WorkSessionEdit, TimeTrackingDeletion: audits.TimeTrackingDeletion,
			DataAccessLog: audits.DataAccessLog, TimeTrackingAuditLog: audits.TimeTrackingAuditLog,
		},
		People: usersService, Holidays: nonWorkingDayService,
		Events:        owners.TimeTrackingEvents(realtimeHub),
		ShiftPlanSync: shiftPlanSyncer,
		Logger:        logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, nil, err
	}
	return WorkforceTestModule{StaffAdmin: staffAdmin, TimeTracking: timeTracking}, usersService, nil
}

// MustNewStaffEmployment is workforceCompose.MustNewStaffEmployment for the
// test graphs outside this package whose test roles may not name the Workforce
// composition.
func MustNewStaffEmployment(db transactionruntime.Database) workforce.StaffEmployments {
	return workforceCompose.MustNewStaffEmployment(db.Pool())
}

// StaffAdminDependencies are the audit trails and the runtime of the
// personnel-record administration for the test graphs outside this package
// whose test roles may not name the Workforce composition.
type StaffAdminDependencies struct {
	MasterDataAudit auditModels.StaffMasterDataChangeCreator
	PersonnelNumber auditModels.PersonnelNumberChangeCreator
	DataAccessLog   auditModels.DataAccessLogRepository
	Observe         func(workforceCompose.Observation)
	Logger          *slog.Logger
	Now             func() time.Time
}

// NewStaffAdmin composes the personnel-record administration over the
// People Directory's staff and person rows, the School Membership owner and
// the audit trails and runtime in deps.
func NewStaffAdmin(db transactionruntime.Database, staff peoplePorts.StaffRepository, persons peoplePorts.PersonRepository, membership schoolmembership.Capability, deps StaffAdminDependencies) (*workforce.StaffAdmin, error) {
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return owners.ComposeStaffAdmin(workforceCompose.StaffAdminDependencies{
		DB: db.Pool(), Observe: deps.Observe, Logger: deps.Logger, Now: deps.Now,
	}, staff, persons, people, membership, deps.MasterDataAudit, deps.PersonnelNumber, deps.DataAccessLog)
}

// StaffAbsenceRow and AbsenceRequestFilter name the Workforce rows the staff
// absence decorators read and write, for the suites outside this package
// whose test roles may not name the Workforce composition.
type (
	StaffAbsenceRow      = workforceCompose.StaffAbsence
	AbsenceRequestFilter = workforceCompose.AbsenceRequestFilter
)
