package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	"github.com/uptrace/bun"
)

type WorkforceTestModule struct {
	Users                users.PersonService
	StaffDocuments       users.StaffDocumentService
	WorkSession          timetracking.WorkSessionService
	StaffAbsence         timetracking.StaffAbsenceService
	WorkTimeMonth        timetracking.WorkTimeMonthService
	StaffBalanceAdjust   timetracking.StaffBalanceAdjustmentService
	StaffMonthClose      timetracking.StaffMonthCloseService
	StaffOverview        timetracking.StaffOverviewService
	TimeTrackingAuditLog timetracking.TimeTrackingAuditLogService
	StaffTimeExport      timetracking.StaffTimeExportService
	Settings             config.SettingsService
}

func NewWorkforceTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (WorkforceTestModule, error) {
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	repos, err := repositories.NewWorkforceTestRepositories(db, command, clocks...)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	identity, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	logger := slog.Default()
	identityAccess, err := lifecycleTestModule(db, unit, GuardianInvitationTestConfig{Audit: command, Logger: logger})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	identityRoles := roleAdministration{current: func() *identityaccess.Module { return identityAccess }}
	settingsService := settings.Settings
	activeLogger := logger
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonDirectory:  repositories.NewPersonDirectory(repositories.MustNewPeopleDirectory(db)),
		StudentDirectory: repositories.NewStudentDirectory(repositories.MustNewPeopleDirectory(db)),
		PersonRepo:       repos.Person, RFIDRepo: identity.RFIDCard, AccountExists: repositories.AccountExists(identityAccess), StudentRepo: repos.Student,
		StaffRepo: repos.Staff, TeacherRepo: repos.Teacher, LehrkraftRoles: identityRoles, PersonnelNumberAudit: repos.PersonnelNumberChange,
		StaffMasterDataRepo: repos.StaffMasterData, StaffQualificationRepo: repos.StaffQualification, StaffFinancialRepo: repos.StaffFinancialData,
		StammdatenAudit: repos.StaffMasterDataChange, DataAccessLog: repos.DataAccessLog, DB: db, SettingsService: settingsService, Logger: logger,
	})
	staffDocumentService := users.NewStaffDocumentService(db, repos.StaffDocument, repos.Staff, repos.StaffMasterData, repos.StaffMasterDataChange, repos.DataAccessLog, logger)
	calendarAdministration := schoolCalendarAdministration(settingsService, func(context.Context) error { return nil }, nil)
	calendar, err := repositories.NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return WorkforceTestModule{}, err
	}
	nonWorkingDayService := nonWorkingDays{calendar: calendar}
	staffAbsenceTypeService := AbsenceTypes(repos.StaffAbsenceType)
	timeTrackingEvents := TimeTrackingEvents(realtimeHub)
	today := timezone.CalendarDateClock(optionalClock(clocks))
	var shiftPlanSyncer shiftplansync.SickCascade
	workSessionService := timetracking.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, NewWorkSessionAudit(repos.WorkSessionEdit), repos.StaffAbsence, repos.GroupSupervisor, repos.ActiveGroup, WorkSessionStaff(repos.Staff, repositories.MustNewStaffEmployment(db)), NewWorkSessionSchedules(repos.StaffWorkSchedule), NewWorkSessionTimeModels(repos.WorkTimeModel), PresenceSettings(settingsService), activeLogger, db, RenderTimeTrackingPDF, RenderTimeTrackingWorkbook,
		timetracking.WithWorkSessionShifts(NewTimeTrackingShifts(repos.StaffAbsenceType)),
		timetracking.WithWorkSessionEvents(timeTrackingEvents),
		timetracking.WithWorkSessionHolidays(nonWorkingDayService),
		timetracking.WithWorkSessionAbsenceTypes(staffAbsenceTypeService),
	)
	workTimeMonthService := timetracking.NewWorkTimeMonthService(
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		StaffScheduleAssignments(repositories.MustNewStaffEmployment(db)),
		NewWorkScheduleTargets(repos.StaffWorkSchedule),
		NewWorkTimeTargetModels(repos.WorkTimeModel),
		NewTimeTrackingShifts(repos.StaffAbsenceType),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithMonthHolidays(nonWorkingDayService),
		timetracking.WithMonthAdjustments(repos.StaffBalanceAdjust),
		timetracking.WithMonthSnapshots(MonthSnapshotCapability(repos.StaffMonthSnapshot)),
	)

	staffAbsenceService := timetracking.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota, repos.StaffAbsenceAudit, PresenceSettings(settingsService), workTimeMonthService,
		timetracking.WithAbsenceToday(today),
		timetracking.WithAbsenceTypes(staffAbsenceTypeService),
		timetracking.WithAbsenceEvents(timeTrackingEvents),
		timetracking.WithAbsenceLogger(activeLogger),
		timetracking.WithAbsenceDeletionAudit(NewTimeTrackingDeletionAudit(repos.TimeTrackingDeletion)),
		timetracking.WithVacationOpenings(repos.StaffVacationOpening),
		timetracking.WithAbsenceShiftPlanSyncer(shiftplansyncCompose.DeferredSickCascade(func() shiftplansync.SickCascade { return shiftPlanSyncer })),
		timetracking.WithAbsenceMonthSnapshots(MonthSnapshotCapability(repos.StaffMonthSnapshot)),
	)

	staffBalanceAdjustService := timetracking.NewStaffBalanceAdjustmentService(repos.StaffBalanceAdjust, workTimeMonthService, PresenceSettings(settingsService), activeLogger,
		timetracking.WithAdjustmentToday(today),
		timetracking.WithAdjustmentEvents(timeTrackingEvents),
		timetracking.WithAdjustmentSnapshots(MonthSnapshotCapability(repos.StaffMonthSnapshot)),
		timetracking.WithAdjustmentDeletionAudit(NewTimeTrackingDeletionAudit(repos.TimeTrackingDeletion)),
	)

	staffMonthCloseService := timetracking.NewStaffMonthCloseService(
		MonthSnapshotCapability(repos.StaffMonthSnapshot),
		workTimeMonthService,
		MonthCloseStaff(repos.Staff),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithMonthCloseEvents(timeTrackingEvents),
	)

	staffOverviewService := timetracking.NewStaffOverviewService(
		OverviewStaff(repos.Staff),
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		repos.StaffBalanceAdjust,
		repos.StaffVacationQuota,
		MonthSnapshotCapability(repos.StaffMonthSnapshot),
		NewWorkScheduleTargets(repos.StaffWorkSchedule),
		NewWorkTimeTargetModels(repos.WorkTimeModel),
		NewTimeTrackingShifts(repos.StaffAbsenceType),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithOverviewHolidays(nonWorkingDayService),
		timetracking.WithOverviewVacationOpenings(repos.StaffVacationOpening),
	)

	payrollStatusService := config.NewPayrollStatusService(settingsService, func(ctx context.Context) (int, int, error) {
		staff, err := repos.Staff.List(ctx, nil)
		if err != nil {
			return 0, 0, err
		}
		withoutPersonnelNumber := 0
		for _, member := range staff {
			if member.PersonnelNumber == nil || *member.PersonnelNumber == "" {
				withoutPersonnelNumber++
			}
		}
		return len(staff), withoutPersonnelNumber, nil
	})

	staffTimeExportService := timetracking.NewStaffTimeExportService(
		staffOverviewService,
		workSessionService,
		TimeExportStaff(repos.Staff),
		NewTimeTrackingDataAccessAudit(repos.DataAccessLog),
		PayrollExportSettings{Source: payrollStatusService},
		activeLogger,
		RenderTimeTrackingWorkbook,
	)

	timeTrackingAuditLogService := timetracking.NewTimeTrackingAuditLogService(
		NewTimeTrackingAuditReader(repos.TimeTrackingAuditLog),
		StaffDisplayNames(repos.Staff),
		PresenceSettings(settingsService),
	)

	timetable, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	// The #1843 sick cascade is the shift-plan-sync workflow over the
	// Workforce planning composition and the timetable test module, bound
	// the way the factory binds it (#3418).
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: repos.StaffAbsenceType, Staff: repos.Staff, CalendarPeriods: repos.CalendarPeriod, DeviationEvents: repos.DeviationEvent,
		Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff, Rooms: repos.Room, ActivityGroups: repos.ActivityGroup,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel, Holidays: nonWorkingDayService,
		DB: db, Broadcaster: realtimeHub, Logger: logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	shiftPlanSyncer, err = shiftplansyncCompose.NewSickCascade(shiftplansyncCompose.SickCascadeDependencies{
		Planning: planning.Planning(nil), Workforce: repos.StaffAbsenceType, LockStaffShifts: workforceCompose.NewStaffShiftLock(db),
		Instances: timetable.Instance, TimetableData: timetable.TimetableData, InstanceStaff: repos.InstanceStaff,
		Broadcaster: realtimeHub, Logger: logger, Today: today,
	})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	return WorkforceTestModule{Users: usersService, StaffDocuments: staffDocumentService, WorkSession: workSessionService, StaffAbsence: staffAbsenceService, WorkTimeMonth: workTimeMonthService, StaffBalanceAdjust: staffBalanceAdjustService, StaffMonthClose: staffMonthCloseService, StaffOverview: staffOverviewService, TimeTrackingAuditLog: timeTrackingAuditLogService, StaffTimeExport: staffTimeExportService, Settings: settingsService}, nil
}

// CreateAbsenceRequest is the request shape behaviour tests hand the retained
// absence service of a WorkforceTestModule.
type CreateAbsenceRequest = timetracking.CreateAbsenceRequest

// UpdateAbsenceRequest is the matching update shape for the same tests.
type UpdateAbsenceRequest = timetracking.UpdateAbsenceRequest
