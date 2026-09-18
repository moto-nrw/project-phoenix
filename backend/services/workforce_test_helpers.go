package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	shiftplanning "github.com/moto-nrw/project-phoenix/modules/workforce/legacy/shiftplanning"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	calendar, err := repositories.NewSchoolCalendar(db)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	logger := slog.Default()
	_, identityAccess, err := lifecycleTestModule(db, unit, GuardianInvitationTestConfig{Audit: command, Logger: logger})
	if err != nil {
		return WorkforceTestModule{}, err
	}
	identityRoles := roleAdministration{current: func() *identityaccess.Module { return identityAccess.module }}
	settingsService := settings.Settings
	activeLogger := logger
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonDirectory: repositories.NewPersonDirectory(repositories.MustNewPeopleDirectory(db)),
		PersonRepo:      repos.Person, RFIDRepo: identity.RFIDCard, AccountRepo: identity.Account, StudentRepo: repos.Student,
		StaffRepo: repos.Staff, TeacherRepo: repos.Teacher, LehrkraftRoles: identityRoles, PersonnelNumberAudit: repos.PersonnelNumberChange,
		StaffMasterDataRepo: repos.StaffMasterData, StaffQualificationRepo: repos.StaffQualification, StaffFinancialRepo: repos.StaffFinancialData,
		StammdatenAudit: repos.StaffMasterDataChange, DataAccessLog: repos.DataAccessLog, DB: db, SettingsService: settingsService, Logger: logger,
	})
	staffDocumentService := users.NewStaffDocumentService(db, repos.StaffDocument, repos.Staff, repos.StaffMasterData, repos.StaffMasterDataChange, repos.DataAccessLog, logger)
	holidayService := timetableplanning.NewHolidayService(settingsService, schoolCalendarHolidayAdapter{query: calendar}, logger.With("service", "holidays"))
	closingDayService := timetableplanning.NewClosingDayService(repos.ClosingDay)
	nonWorkingDayService := timetableplanning.NewNonWorkingDayResolver(holidayService, closingDayService)
	staffAbsenceTypeService := AbsenceTypes(repos.StaffAbsenceType)
	timeTrackingEvents := TimeTrackingEvents(realtimeHub)
	today := timezone.CalendarDateClock(optionalClock(clocks))
	var shiftPlanSyncer shiftplanning.ShiftPlanSyncer
	workSessionService := timetracking.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, NewWorkSessionAudit(repos.WorkSessionEdit), repos.StaffAbsence, repos.GroupSupervisor, repos.ActiveGroup, WorkSessionStaff(repos.Staff), NewWorkSessionSchedules(repos.StaffWorkSchedule), NewWorkSessionTimeModels(repos.WorkTimeModel), PresenceSettings(settingsService), activeLogger, db, RenderTimeTrackingPDF, RenderTimeTrackingWorkbook,
		timetracking.WithWorkSessionShifts(NewTimeTrackingShifts(repos.StaffShift)),
		timetracking.WithWorkSessionEvents(timeTrackingEvents),
		timetracking.WithWorkSessionHolidays(nonWorkingDayService),
		timetracking.WithWorkSessionAbsenceTypes(staffAbsenceTypeService),
	)
	workTimeMonthService := timetracking.NewWorkTimeMonthService(
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		StaffScheduleAssignments(repos.Staff),
		NewWorkScheduleTargets(repos.StaffWorkSchedule),
		NewWorkTimeTargetModels(repos.WorkTimeModel),
		NewTimeTrackingShifts(repos.StaffShift),
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
		timetracking.WithAbsenceShiftPlanSyncer(ShiftPlanSyncBridge(func() shiftplanning.ShiftPlanSyncer { return shiftPlanSyncer })),
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
		NewTimeTrackingShifts(repos.StaffShift),
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
	shifts := shiftplanning.NewStaffShiftService(repos.StaffShift, repos.Staff, shiftplanning.NewShiftTypeService(repos.ShiftType, logger), db, logger,
		shiftplanning.WithStaffShiftSeriesExceptions(repos.StaffShiftSeriesException),
		shiftplanning.WithStaffShiftDeviationEvents(repos.DeviationEvent),
		shiftplanning.WithStaffShiftBroadcaster(realtimeHub))
	shiftPlanSyncer = shiftplanning.NewShiftPlanSyncService(shifts, timetable.Instance, timetable.TimetableData,
		repos.StaffShift, repos.InstanceStaff, realtimeHub, db, logger, today)
	return WorkforceTestModule{Users: usersService, StaffDocuments: staffDocumentService, WorkSession: workSessionService, StaffAbsence: staffAbsenceService, WorkTimeMonth: workTimeMonthService, StaffBalanceAdjust: staffBalanceAdjustService, StaffMonthClose: staffMonthCloseService, StaffOverview: staffOverviewService, TimeTrackingAuditLog: timeTrackingAuditLogService, StaffTimeExport: staffTimeExportService, Settings: settingsService}, nil
}

// CreateAbsenceRequest is the request shape behaviour tests hand the retained
// absence service of a WorkforceTestModule.
type CreateAbsenceRequest = timetracking.CreateAbsenceRequest

// UpdateAbsenceRequest is the matching update shape for the same tests.
type UpdateAbsenceRequest = timetracking.UpdateAbsenceRequest
