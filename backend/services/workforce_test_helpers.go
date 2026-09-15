package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type WorkforceTestModule struct {
	Users                users.PersonService
	StaffDocuments       users.StaffDocumentService
	WorkSession          active.WorkSessionService
	StaffAbsence         active.StaffAbsenceService
	WorkTimeMonth        active.WorkTimeMonthService
	StaffBalanceAdjust   active.StaffBalanceAdjustmentService
	StaffMonthClose      active.StaffMonthCloseService
	StaffOverview        active.StaffOverviewService
	TimeTrackingAuditLog active.TimeTrackingAuditLogService
	StaffTimeExport      active.StaffTimeExportService
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
	settingsService := settings.Settings
	logger := slog.Default()
	activeLogger := logger
	realtimeHub := deliveryCompose.NewRealtimeHub(logger)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo: repos.Person, RFIDRepo: identity.RFIDCard, AccountRepo: identity.Account, StudentRepo: repos.Student,
		StaffRepo: repos.Staff, TeacherRepo: repos.Teacher, RoleRepo: identity.Role, PersonnelNumberAudit: repos.PersonnelNumberChange,
		StaffMasterDataRepo: repos.StaffMasterData, StaffQualificationRepo: repos.StaffQualification, StaffFinancialRepo: repos.StaffFinancialData,
		StammdatenAudit: repos.StaffMasterDataChange, DataAccessLog: repos.DataAccessLog, DB: db, SettingsService: settingsService, Logger: logger,
	})
	staffDocumentService := users.NewStaffDocumentService(db, repos.StaffDocument, repos.Staff, repos.StaffMasterData, repos.StaffMasterDataChange, repos.DataAccessLog, logger)
	workSessionService := active.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, NewWorkSessionAudit(repos.WorkSessionEdit), repos.StaffAbsence, repos.GroupSupervisor, repos.ActiveGroup, WorkSessionStaff(repos.Staff), NewWorkSessionSchedules(repos.StaffWorkSchedule), NewWorkSessionTimeModels(repos.WorkTimeModel), PresenceSettings(settingsService), activeLogger, db, RenderTimeTrackingPDF, RenderTimeTrackingWorkbook)
	workSessionService.SetStaffShiftRepo(NewTimeTrackingShifts(repos.StaffShift))
	if broadcastAware, ok := workSessionService.(interface {
		SetBroadcaster(active.EventPublisher)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	workTimeMonthService := active.NewWorkTimeMonthService(
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		StaffScheduleAssignments(repos.Staff),
		NewWorkScheduleTargets(repos.StaffWorkSchedule),
		NewWorkTimeTargetModels(repos.WorkTimeModel),
		NewTimeTrackingShifts(repos.StaffShift),
		PresenceSettings(settingsService),
		activeLogger,
	)

	holidayService := schedule.NewHolidayService(settingsService, schoolCalendarHolidayAdapter{query: calendar}, logger.With("service", "holidays"))
	closingDayService := schedule.NewClosingDayService(repos.ClosingDay)
	nonWorkingDayService := schedule.NewNonWorkingDayResolver(holidayService, closingDayService)
	workTimeMonthService.SetHolidayReader(nonWorkingDayService)
	workTimeMonthService.SetAdjustmentReader(repos.StaffBalanceAdjust)
	workTimeMonthService.SetSnapshotReader(MonthSnapshotCapability(repos.StaffMonthSnapshot))
	if holidayAware, ok := workSessionService.(interface {
		SetHolidayReader(active.HolidayDatesReader)
	}); ok {
		holidayAware.SetHolidayReader(nonWorkingDayService)
	}

	staffAbsenceTypeService := AbsenceTypes(repos.StaffAbsenceType)
	if typeAware, ok := workSessionService.(interface {
		SetAbsenceTypeService(active.AbsenceTypeReader)
	}); ok {
		typeAware.SetAbsenceTypeService(staffAbsenceTypeService)
	}

	staffAbsenceService := active.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota, repos.StaffAbsenceAudit, PresenceSettings(settingsService), workTimeMonthService, timezone.CalendarDateClock(optionalClock(clocks)))
	if typeAware, ok := staffAbsenceService.(interface {
		SetAbsenceTypeService(active.AbsenceTypeReader)
	}); ok {
		typeAware.SetAbsenceTypeService(staffAbsenceTypeService)
	}
	if broadcastAware, ok := staffAbsenceService.(interface {
		SetBroadcaster(active.EventPublisher)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	if loggerAware, ok := staffAbsenceService.(interface {
		SetLogger(*slog.Logger)
	}); ok {
		loggerAware.SetLogger(activeLogger)
	}

	staffBalanceAdjustService := active.NewStaffBalanceAdjustmentService(repos.StaffBalanceAdjust, workTimeMonthService, PresenceSettings(settingsService), activeLogger, timezone.CalendarDateClock(optionalClock(clocks)))
	if broadcastAware, ok := staffBalanceAdjustService.(interface {
		SetBroadcaster(active.EventPublisher)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	staffBalanceAdjustService.SetSnapshotReader(MonthSnapshotCapability(repos.StaffMonthSnapshot))
	if deletionAware, ok := staffBalanceAdjustService.(interface {
		SetDeletionAudit(active.TimeTrackingDeletionAudit)
	}); ok {
		deletionAware.SetDeletionAudit(NewTimeTrackingDeletionAudit(repos.TimeTrackingDeletion))
	}
	if deletionAware, ok := staffAbsenceService.(interface {
		SetDeletionAudit(active.TimeTrackingDeletionAudit)
	}); ok {
		deletionAware.SetDeletionAudit(NewTimeTrackingDeletionAudit(repos.TimeTrackingDeletion))
	}
	if openingAware, ok := staffAbsenceService.(interface {
		SetVacationOpeningRepository(activeModels.StaffVacationOpeningRepository)
	}); ok {
		openingAware.SetVacationOpeningRepository(repos.StaffVacationOpening)
	}

	staffMonthCloseService := active.NewStaffMonthCloseService(
		MonthSnapshotCapability(repos.StaffMonthSnapshot),
		workTimeMonthService,
		MonthCloseStaff(repos.Staff),
		PresenceSettings(settingsService),
		activeLogger,
	)
	if broadcastAware, ok := staffMonthCloseService.(interface {
		SetBroadcaster(active.EventPublisher)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}

	staffOverviewService := active.NewStaffOverviewService(
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
	)
	staffOverviewService.SetHolidayReader(nonWorkingDayService)
	staffOverviewService.SetVacationOpeningReader(repos.StaffVacationOpening)

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

	staffTimeExportService := active.NewStaffTimeExportService(
		staffOverviewService,
		workSessionService,
		TimeExportStaff(repos.Staff),
		NewDataAccessAudit(repos.DataAccessLog),
		PayrollExportSettings{Source: payrollStatusService},
		activeLogger,
		RenderTimeTrackingWorkbook,
	)

	timeTrackingAuditLogService := active.NewTimeTrackingAuditLogService(
		NewTimeTrackingAuditReader(repos.TimeTrackingAuditLog),
		StaffDisplayNames(repos.Staff),
		PresenceSettings(settingsService),
	)

	timetable, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	shifts := schedule.NewStaffShiftService(repos.StaffShift, repos.Staff, schedule.NewShiftTypeService(repos.ShiftType, logger), db, logger)
	shifts.SetSeriesExceptionRepo(repos.StaffShiftSeriesException)
	shifts.SetDeviationEventRepo(repos.DeviationEvent)
	shifts.(interface{ SetBroadcaster(realtime.Broadcaster) }).SetBroadcaster(realtimeHub)
	staffAbsenceService.SetShiftPlanSyncer(schedule.NewShiftPlanSyncService(shifts, timetable.Instance, timetable.TimetableData,
		repos.StaffShift, repos.InstanceStaff, realtimeHub, db, logger, timezone.CalendarDateClock(optionalClock(clocks))))
	return WorkforceTestModule{Users: usersService, StaffDocuments: staffDocumentService, WorkSession: workSessionService, StaffAbsence: staffAbsenceService, WorkTimeMonth: workTimeMonthService, StaffBalanceAdjust: staffBalanceAdjustService, StaffMonthClose: staffMonthCloseService, StaffOverview: staffOverviewService, TimeTrackingAuditLog: timeTrackingAuditLogService, StaffTimeExport: staffTimeExportService, Settings: settingsService}, nil
}
