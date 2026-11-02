package timetracking

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Settings is every tenant setting the time-tracking services resolve. The
// composition root binds one resolver over the Settings Platform.
type Settings interface {
	settingsResolver
	monthSettingsResolver
	retentionSettingsResolver
	absenceEmailSettingResolver
}

// ScheduleTargetReader serves the contractual daily Soll of the staff work
// schedules to the month card and the overview.
type ScheduleTargetReader interface {
	monthScheduleReader
	OverviewWorkSchedules
}

// TargetModelReader serves the work time models the Soll falls back to.
type TargetModelReader interface {
	monthModelReader
	OverviewWorkTimeModels
}

// ShiftReader serves the planned shifts the auto-checkout, the month card
// and the overview compare against.
type ShiftReader interface {
	WorkSessionShifts
	monthShiftReader
	StaffOverviewShifts
}

// Rows are the Workforce capability's own rows and reads the services
// work on; the Workforce composition binds them.
type Rows struct {
	// TargetOverrides serves the Sonderarbeitszeiten (#3259).
	TargetOverrides  workforce.StaffTargetOverrideQuery
	MonthSnapshots   MonthSnapshots
	AbsenceTypes     AbsenceTypeReader
	Sessions         WorkSessionRecords
	Breaks           WorkSessionBreakRecords
	Adjustments      BalanceAdjustmentRecords
	VacationQuotas   VacationQuotaRecords
	VacationOpenings VacationOpeningRecords
}

// Ports are the consumer-owned readers and writers the composition root
// binds: the absence rows with their staff and group relations, the audit
// trails, the staff, schedule, shift, calendar and settings reads of other
// owners, and the renderers.
type Ports struct {
	Absences      AbsenceRequestRecords
	AbsenceAudits AbsenceAuditRecords

	EditAudit       WorkSessionAudit
	DeletionAudit   TimeTrackingDeletionAudit
	DataAccessAudit DataAccessAudit
	AuditReader     TimeTrackingAuditReader

	Supervisions   GroupSupervisorRepository
	SessionRecords GroupRepository

	Staff           WorkSessionStaff
	StaffSchedules  StaffScheduleQuery
	MonthCloseStaff MonthCloseStaffQuery
	OverviewStaff   OverviewStaffQuery
	ExportStaff     TimeExportStaffQuery
	StaffNames      StaffDisplayNameQuery
	// ExternalCaregivers keeps the auto-stamp off external caregivers.
	ExternalCaregivers ExternalCaregiverStaff

	Schedules       WorkSessionSchedules
	TimeModels      WorkSessionTimeModels
	ScheduleTargets ScheduleTargetReader
	TargetModels    TargetModelReader
	Shifts          ShiftReader
	Holidays        HolidayDatesReader
	// StatutoryHolidays are the public holidays of the school's federal
	// state alone, without closing days: the holiday surcharge (#4807)
	// counts on them.
	StatutoryHolidays HolidayDatesReader
	// PayrollRules is the school's wage types and surcharge rules in force
	// (#4807): the export books with them, the month close freezes them.
	PayrollRules workforce.PayrollRuleSetReader

	Settings Settings
	Payroll  PayrollConfigurationReader

	DB             DatabaseHandle
	RenderPDF      TimeTrackingPDFRenderer
	RenderWorkbook TimeTrackingWorkbookRenderer
	Events         EventPublisher
	// AbsenceEmail is optional: nil sends no absence email.
	AbsenceEmail *AbsenceEmailDeps
	Logger       *slog.Logger
	// Today is optional and pins the absence and ledger calendar day.
	Today func() calendar.Date
}

// Dependencies are the collaborators the time-tracking services are built
// from.
type Dependencies struct {
	Rows
	Ports
}

// Services are the composed time-tracking services. The Workforce
// composition serves the public contracts from them.
type Services struct {
	Sessions WorkSessionService
	Months   WorkTimeMonthService
	Absences StaffAbsenceService
	Ledger   StaffBalanceAdjustmentService
	Closing  StaffMonthCloseService
	Overview StaffOverviewService
	Export   StaffTimeExportService
	AuditLog TimeTrackingAuditLogService
}

// Clock is every service but the absence lifecycle. The absence lifecycle
// cascades a sick report through the plans (#1843), and the cascade's
// Timetable side reaches Student Presence, which stamps through the work
// sessions; the clock exists first so the cascade can be built over it.
// WithAbsences takes the cascade.
type Clock struct {
	deps     Dependencies
	services Services
}

// NewClock builds the work session, month, ledger, month-close, overview,
// export and audit-log services.
func NewClock(deps Dependencies) (*Clock, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}
	logger := loggerOrDefault(deps.Logger)
	snapshots, absenceTypes := deps.MonthSnapshots, deps.AbsenceTypes
	sessions, months := deps.sessionsAndMonths(absenceTypes, snapshots, logger)
	ledger := deps.ledger(months, snapshots, logger)
	// Monatsabschluss (#1417): freezes a month's closing balance so a
	// retroactive correction can no longer rewrite every later Übertrag.
	closing := NewStaffMonthCloseService(snapshots, months, deps.MonthCloseStaff, deps.Settings, logger, WithMonthCloseEvents(deps.Events), WithMonthCloseUnitOfWork(runTimeTrackingUnit), WithMonthClosePayrollRules(deps.PayrollRules))
	overview := deps.overview(snapshots, logger)
	// Cross-staff payroll/evidence export (#1417 2b): rows via the overview's
	// prefetch (month) and the single-staff export cells (day); every download
	// writes an audit.data_access_logs row or fails.
	export := NewStaffTimeExportService(overview, sessions, deps.ExportStaff, deps.DataAccessAudit, deps.Payroll, logger, deps.RenderWorkbook)
	export.payrollRules = deps.PayrollRules
	// Cross-staff audit feed (#1417): merges the four change trails into one
	// keyset-paginated view. Read-only; permission gating at the route.
	auditLog := NewTimeTrackingAuditLogService(deps.AuditReader, deps.StaffNames, deps.Settings)

	return &Clock{deps: deps, services: Services{
		Sessions: sessions, Months: months, Ledger: ledger,
		Closing: closing, Overview: overview, Export: export, AuditLog: auditLog,
	}}, nil
}

// Sessions is the work session service of the clock.
func (c *Clock) Sessions() WorkSessionService {
	return c.services.Sessions
}

// WithAbsences builds the absence lifecycle over the clock's month service
// and returns every service. Its sick-report writes return their plan for the
// shift-plan-sync workflow to apply (#4825).
func (c *Clock) WithAbsences() *Services {
	deps, services := c.deps, c.services
	services.Absences = NewStaffAbsenceService(deps.Absences, deps.Sessions, deps.VacationQuotas, deps.AbsenceAudits, deps.Settings, services.Months,
		append(deps.absenceOptions(deps.AbsenceTypes, deps.MonthSnapshots, loggerOrDefault(deps.Logger)), WithAbsenceUnitOfWork(runTimeTrackingUnit))...)
	return &services
}

// sessionsAndMonths builds the work session service and the Monatskarte
// (#1842), which reference each other: the weekly history summaries re-price
// Sonderarbeitszeit weeks (#3259) through the month service.
func (deps Dependencies) sessionsAndMonths(absenceTypes AbsenceTypeReader, snapshots MonthSnapshots, logger *slog.Logger) (WorkSessionService, WorkTimeMonthService) {
	var months WorkTimeMonthService
	sessions := NewWorkSessionService(deps.Sessions, deps.Breaks, deps.EditAudit, deps.Absences, deps.Supervisions, deps.SessionRecords,
		deps.Staff, deps.Schedules, deps.TimeModels, deps.Settings, logger, deps.DB, deps.RenderPDF, deps.RenderWorkbook,
		// Planned-shift lookups for the auto-checkout job (#1798).
		WithWorkSessionShifts(deps.Shifts),
		WithWorkSessionEvents(deps.Events),
		WithWorkSessionHolidays(deps.Holidays),
		WithWorkSessionExternalCaregivers(deps.ExternalCaregivers),
		WithWorkSessionAbsenceTypes(absenceTypes),
		WithWorkSessionTargetOverrides(staffTargetOverrideWeeks{overrides: deps.TargetOverrides, months: func() WorkTimeMonthService { return months }}),
	)
	// Everything computed on read, the Übertrag is live.
	months = NewWorkTimeMonthService(deps.Sessions, deps.Breaks, deps.Absences, deps.StaffSchedules, deps.ScheduleTargets,
		deps.TargetModels, deps.Shifts, deps.Settings, logger,
		WithMonthHolidays(deps.Holidays),
		// Sonderarbeitszeiten (#3259) win over closing days and the schedule.
		WithMonthTargetOverrides(deps.TargetOverrides),
		// Stundenkonto transactions (#1420) enter the carry chain by effective date.
		WithMonthAdjustments(deps.Adjustments),
		// Frozen months (#1417) short-circuit the carry chain so a retroactive
		// correction can no longer rewrite a closed month's Übertrag.
		WithMonthSnapshots(snapshots),
	)
	return sessions, months
}

// ledger builds the Stundenkonto lifecycle transactions (#1420): payout,
// comp-time grants, school-year reset. It reads the live balance through the
// month service.
func (deps Dependencies) ledger(months WorkTimeMonthService, snapshots MonthSnapshots, logger *slog.Logger) StaffBalanceAdjustmentService {
	options := []StaffBalanceAdjustmentOption{
		WithAdjustmentUnitOfWork(runTimeTrackingUnit),
		WithAdjustmentEvents(deps.Events),
		// A booking inside a closed month (#1417) could not move its frozen
		// closing balance, so the ledger rejects it.
		WithAdjustmentSnapshots(snapshots),
		// Deletes leave an append-only tombstone in the cross-staff audit log
		// (#1417). The delete path fails without this wiring.
		WithAdjustmentDeletionAudit(deps.DeletionAudit),
	}
	if deps.Today != nil {
		today := deps.Today
		options = append(options, func(s *staffBalanceAdjustmentService) { s.todayFunc = today })
	}
	return NewStaffBalanceAdjustmentService(deps.Adjustments, months, deps.Settings, logger, options...)
}

// overview builds the tenant-wide time-tracking views (#1417 2a). It
// prefetches all inputs once and runs the SAME per-staff month math over
// in-memory readers, so the list can never drift from the /staff/{id} detail
// view.
func (deps Dependencies) overview(snapshots MonthSnapshots, logger *slog.Logger) StaffOverviewService {
	return NewStaffOverviewService(deps.OverviewStaff, deps.Sessions, deps.Breaks, deps.Absences, deps.Adjustments, deps.VacationQuotas,
		snapshots, deps.ScheduleTargets, deps.TargetModels, deps.Shifts, deps.Settings, logger,
		WithOverviewHolidays(deps.Holidays),
		WithOverviewTargetOverrides(deps.TargetOverrides),
		WithOverviewPayrollRules(deps.PayrollRules, deps.StatutoryHolidays),
		// Vacation takeover (#2132): the Resturlaub column subtracts
		// pre-introduction days exactly like the /staff/{id} detail view.
		WithOverviewVacationOpenings(deps.VacationOpenings),
	)
}

func (deps Dependencies) absenceOptions(types AbsenceTypeReader, snapshots MonthSnapshots, logger *slog.Logger) []StaffAbsenceOption {
	options := []StaffAbsenceOption{
		WithAbsenceTypes(types),
		WithAbsenceEvents(deps.Events),
		WithAbsenceLogger(logger),
		// Deletes leave an append-only tombstone in the cross-staff audit log
		// (#1417). The delete paths fail without this wiring.
		WithAbsenceDeletionAudit(deps.DeletionAudit),
		// Vacation takeover at the moto introduction (#2132): the summary
		// subtracts pre-introduction days, the write paths book/delete them.
		WithVacationOpenings(deps.VacationOpenings),
		// A rebooking inside a closed month (#3258) could not move its frozen
		// closing balance, so the service rejects it.
		WithAbsenceMonthSnapshots(snapshots),
	}
	if deps.Today != nil {
		today := deps.Today
		options = append(options, func(s *staffAbsenceService) { s.todayFunc = today })
	}
	if deps.AbsenceEmail != nil {
		// Absence email notifications (#1419 4d).
		options = append(options, WithAbsenceEmail(*deps.AbsenceEmail))
	}
	return options
}

func (deps Dependencies) validate() error {
	switch {
	case deps.TargetOverrides == nil || deps.MonthSnapshots == nil || deps.AbsenceTypes == nil:
		return errors.New("time tracking: the target overrides, month snapshots and absence types are required")
	case deps.Sessions == nil || deps.Breaks == nil || deps.Adjustments == nil || deps.VacationQuotas == nil ||
		deps.VacationOpenings == nil || deps.Absences == nil || deps.AbsenceAudits == nil:
		return errors.New("time tracking: the work session, break, ledger, vacation and absence rows are required")
	case deps.EditAudit == nil || deps.DeletionAudit == nil || deps.DataAccessAudit == nil || deps.AuditReader == nil:
		return errors.New("time tracking: the edit, deletion, data-access and audit-feed trails are required")
	case deps.Supervisions == nil || deps.SessionRecords == nil:
		return errors.New("time tracking: the supervision and session records are required")
	case deps.Staff == nil || deps.StaffSchedules == nil || deps.MonthCloseStaff == nil ||
		deps.OverviewStaff == nil || deps.ExportStaff == nil || deps.StaffNames == nil || deps.ExternalCaregivers == nil:
		return errors.New("time tracking: the staff readers are required")
	case deps.Schedules == nil || deps.TimeModels == nil || deps.ScheduleTargets == nil || deps.TargetModels == nil ||
		deps.Shifts == nil || deps.Holidays == nil:
		return errors.New("time tracking: the schedule, model, shift and holiday readers are required")
	case deps.Settings == nil || deps.Payroll == nil || deps.PayrollRules == nil || deps.StatutoryHolidays == nil:
		return errors.New("time tracking: the settings, payroll configuration, payroll rules and statutory holidays are required")
	case deps.RenderPDF == nil || deps.RenderWorkbook == nil || deps.Events == nil:
		return errors.New("time tracking: the document renderers and the change notification are required")
	}
	return nil
}

// CleanupDependencies are the collaborators of the retention cleanup, which
// the cleanup CLI composes without the rest of the time-tracking services.
type CleanupDependencies struct {
	Sessions WorkSessionRecords
	Absences AbsenceRecords
	// Audit records the deletion evidence; nil runs without it.
	Audit    DeletionAudit
	Settings Settings
	Logger   *slog.Logger
}

// NewCleanup builds the time-tracking retention cleanup (Tranche 0b): it
// deletes work sessions and staff absences older than the tenant's
// retention window.
func NewCleanup(deps CleanupDependencies) (TimeTrackingCleanupService, error) {
	if deps.Sessions == nil || deps.Absences == nil || deps.Settings == nil {
		return nil, errors.New("time tracking cleanup: the session and absence rows and the settings are required")
	}
	return NewTimeTrackingCleanupService(deps.Sessions, deps.Absences, deps.Audit, deps.Settings, deps.Logger), nil
}
