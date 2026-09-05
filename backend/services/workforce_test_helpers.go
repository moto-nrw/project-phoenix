package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return WorkforceTestModule{}, err
	}
	repos.WorkSessionTestRepositories = repos.WithConfigRuntime(newSettingsRuntime(db, &unit).WithSchoolMembership(membership))
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
	workSessionService := active.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, repos.WorkSessionEdit, repos.StaffAbsence, repos.GroupSupervisor, repos.ActiveGroup, repos.Staff, repos.StaffWorkSchedule, repos.WorkTimeModel, settingsService, activeLogger, db)
	workSessionService.SetStaffShiftRepo(repos.StaffShift)
	if broadcastAware, ok := workSessionService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	workTimeMonthService := active.NewWorkTimeMonthService(
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		repos.Staff,
		repos.StaffWorkSchedule,
		repos.WorkTimeModel,
		repos.StaffShift,
		settingsService,
		activeLogger,
	)

	holidayService := schedule.NewHolidayService(settingsService, schoolCalendarHolidayAdapter{query: calendar}, logger.With("service", "holidays"))
	closingDayService := schedule.NewClosingDayService(repos.ClosingDay)
	nonWorkingDayService := schedule.NewNonWorkingDayResolver(holidayService, closingDayService)
	workTimeMonthService.SetHolidayReader(nonWorkingDayService)
	workTimeMonthService.SetAdjustmentReader(repos.StaffBalanceAdjust)
	workTimeMonthService.SetSnapshotReader(repos.StaffMonthSnapshot)
	if holidayAware, ok := workSessionService.(interface {
		SetHolidayReader(active.HolidayDatesReader)
	}); ok {
		holidayAware.SetHolidayReader(nonWorkingDayService)
	}

	staffAbsenceTypeService := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, activeLogger)
	if allowanceAware, ok := staffAbsenceTypeService.(interface {
		SetAllowanceRepositories(
			activeModels.StaffAbsenceTypeAllowanceRepository,
			activeModels.StaffAbsenceTypeAllowanceChangeRepository,
			activeModels.StaffAbsenceRepository,
		)
	}); ok {
		allowanceAware.SetAllowanceRepositories(
			repos.StaffAbsenceTypeAllowance,
			repos.StaffAbsenceTypeAllowanceChange,
			repos.StaffAbsence,
		)
	}
	if typeAware, ok := workSessionService.(interface {
		SetAbsenceTypeService(active.StaffAbsenceTypeService)
	}); ok {
		typeAware.SetAbsenceTypeService(staffAbsenceTypeService)
	}

	staffAbsenceService := active.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota, repos.StaffAbsenceAudit, settingsService, workTimeMonthService)
	if typeAware, ok := staffAbsenceService.(interface {
		SetAbsenceTypeService(active.StaffAbsenceTypeService)
	}); ok {
		typeAware.SetAbsenceTypeService(staffAbsenceTypeService)
	}
	if broadcastAware, ok := staffAbsenceService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	if loggerAware, ok := staffAbsenceService.(interface {
		SetLogger(*slog.Logger)
	}); ok {
		loggerAware.SetLogger(activeLogger)
	}

	staffBalanceAdjustService := active.NewStaffBalanceAdjustmentService(repos.StaffBalanceAdjust, workTimeMonthService, settingsService, activeLogger)
	if broadcastAware, ok := staffBalanceAdjustService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	staffBalanceAdjustService.SetSnapshotReader(repos.StaffMonthSnapshot)
	if deletionAware, ok := staffBalanceAdjustService.(interface {
		SetDeletionAudit(auditModels.TimeTrackingDeletionRepository)
	}); ok {
		deletionAware.SetDeletionAudit(repos.TimeTrackingDeletion)
	}
	if deletionAware, ok := staffAbsenceService.(interface {
		SetDeletionAudit(auditModels.TimeTrackingDeletionRepository)
	}); ok {
		deletionAware.SetDeletionAudit(repos.TimeTrackingDeletion)
	}
	if openingAware, ok := staffAbsenceService.(interface {
		SetVacationOpeningRepository(activeModels.StaffVacationOpeningRepository)
	}); ok {
		openingAware.SetVacationOpeningRepository(repos.StaffVacationOpening)
	}

	staffMonthCloseService := active.NewStaffMonthCloseService(
		repos.StaffMonthSnapshot,
		workTimeMonthService,
		repos.Staff,
		settingsService,
		activeLogger,
	)
	if broadcastAware, ok := staffMonthCloseService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}

	staffOverviewService := active.NewStaffOverviewService(
		repos.Staff,
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		repos.StaffBalanceAdjust,
		repos.StaffVacationQuota,
		repos.StaffMonthSnapshot,
		repos.StaffWorkSchedule,
		repos.WorkTimeModel,
		repos.StaffShift,
		settingsService,
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
		repos.Staff,
		repos.DataAccessLog,
		payrollStatusService,
		activeLogger,
	)

	timeTrackingAuditLogService := active.NewTimeTrackingAuditLogService(
		repos.TimeTrackingAuditLog,
		repos.Staff,
		settingsService,
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
