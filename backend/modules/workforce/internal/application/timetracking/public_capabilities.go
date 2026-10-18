package timetracking

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The adapters in this file and in public_work_sessions.go,
// public_absences.go, public_time_clock.go and public_month_snapshots.go
// serve the public Workforce time-tracking contracts from the work session,
// absence, month, ledger, month-close, overview, audit-log and export
// services (#2690, #3413). Every wire shape is copied field by field; every
// sentinel failure of a service is reported as the matching public kind with
// its wording intact.

// timeTrackingSentinels pairs each retained sentinel with its public kind.
var timeTrackingSentinels = []struct {
	legacy error
	kind   error
}{
	{ErrManagerControlledAbsence, workforce.ErrManagerControlledAbsence},
	{ErrAllowanceBookingOverlap, workforce.ErrAllowanceBookingOverlap},
	{ErrAbsenceRebookingBlocked, workforce.ErrAbsenceRebookingBlocked},
	{ErrAbsenceTypeInactive, workforce.ErrAbsenceTypeInactive},
	{ErrAbsenceTypeNotFound, workforce.ErrAbsenceTypeNotFound},
	{ErrAbsenceTypeAllowanceExceeded, workforce.ErrAbsenceTypeAllowanceExceeded},
	{ErrAbsenceTypeAllowanceInvalid, workforce.ErrAbsenceTypeAllowanceInvalid},
	{ErrVacationQuotaInvalid, workforce.ErrVacationQuotaInvalid},
	{ErrVacationQuotaExceeded, workforce.ErrVacationQuotaExceeded},
	{ErrAdjustmentInvalid, workforce.ErrAdjustmentInvalid},
	{ErrAdjustmentNotFound, workforce.ErrAdjustmentNotFound},
	{ErrAdjustmentExceedsBalance, workforce.ErrAdjustmentExceedsBalance},
	{ErrAdjustmentInClosedMonth, workforce.ErrAdjustmentInClosedMonth},
	{ErrAdjustmentHasDependentReset, workforce.ErrAdjustmentHasDependentReset},
	{ErrBalanceAlreadyReset, workforce.ErrBalanceAlreadyReset},
	{ErrOpeningAlreadyExists, workforce.ErrOpeningAlreadyExists},
	{ErrMonthCloseInvalid, workforce.ErrMonthCloseInvalid},
	{ErrMonthNotClosable, workforce.ErrMonthNotClosable},
	{ErrMonthNotClosed, workforce.ErrMonthNotClosed},
	{ErrLaterMonthClosed, workforce.ErrLaterMonthClosed},
	{ErrMonthOutOfRange, workforce.ErrMonthOutOfRange},
	{ErrVacationOpeningInvalid, workforce.ErrVacationOpeningInvalid},
	{ErrVacationOpeningNotFound, workforce.ErrVacationOpeningNotFound},
	{ErrVacationOpeningExists, workforce.ErrVacationOpeningExists},
	{ErrVacationOpeningAbsencesBeforeCutoff, workforce.ErrVacationOpeningAbsencesBeforeCutoff},
	{ErrInvalidTargetRange, workforce.ErrInvalidTargetRange},
	{ErrOverviewInvalid, workforce.ErrOverviewInvalid},
	{ErrAuditLogInvalid, workforce.ErrAuditLogInvalid},
	{ErrTimeExportInvalid, workforce.ErrTimeExportInvalid},
	{ErrPayrollConfigIncomplete, workforce.ErrPayrollConfigIncomplete},
	{ErrScheduleValidation, workforce.ErrScheduleValidation},
}

// mapTimeTrackingFailure reports a retained sentinel as its public kind and
// otherwise falls back to the stamp classification; the cause and its wording
// always survive.
func mapTimeTrackingFailure(err error) error {
	if err == nil {
		return nil
	}
	// The #1843 sick cascade wraps the planning layer's overlap: a shift whose
	// freed window was re-planned collides when the report is reversed.
	if errors.Is(err, workforce.ErrStaffShiftOverlap) {
		return &workforce.TimeTrackingError{Kind: workforce.ErrStaffShiftOverlap, Cause: err}
	}
	for _, pair := range timeTrackingSentinels {
		if errors.Is(err, pair.legacy) {
			return &workforce.TimeTrackingError{Kind: pair.kind, Cause: err}
		}
	}
	return MapTimeTrackingError(err)
}

// parseCapabilityDate turns a public YYYY-MM-DD day into the calendar type
// the retained services take; a malformed day is an invalid request.
func parseCapabilityDate(value, field string) (calendar.Date, error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return calendar.Date(""), &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: fmt.Errorf("%s must be YYYY-MM-DD", field)}
	}
	return date, nil
}

// parseCapabilityRange parses the from/to pair every range read takes.
func parseCapabilityRange(from, to string) (calendar.Date, calendar.Date, error) {
	fromDate, err := parseCapabilityDate(from, "from")
	if err != nil {
		return calendar.Date(""), calendar.Date(""), err
	}
	toDate, err := parseCapabilityDate(to, "to")
	if err != nil {
		return calendar.Date(""), calendar.Date(""), err
	}
	return fromDate, toDate, nil
}

func parseOptionalCapabilityDate(value, field string) (*calendar.Date, error) {
	if value == "" {
		return nil, nil
	}
	date, err := parseCapabilityDate(value, field)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

type workTimeMonthCapability struct {
	months WorkTimeMonthService
}

// WorkTimeMonthCapability serves workforce.WorkTimeMonths from the retained
// month service.
func WorkTimeMonthCapability(months WorkTimeMonthService) workforce.WorkTimeMonths {
	if months == nil {
		panic("work time month capability: month service is required")
	}
	return workTimeMonthCapability{months: months}
}

func (c workTimeMonthCapability) MonthSummary(ctx context.Context, staffID int64, year, month int) (*workforce.MonthSummary, error) {
	summary, err := c.months.GetMonthSummary(ctx, staffID, year, month)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if summary == nil {
		return nil, nil
	}
	result := &workforce.MonthSummary{
		StaffID: summary.StaffID, Year: summary.Year, Month: summary.Month, CarryInMinutes: summary.CarryInMinutes,
		TargetMinutes: summary.TargetMinutes, TargetMinutesToDate: summary.TargetMinutesToDate, ActualMinutes: summary.ActualMinutes,
		CreditedSickMinutes: summary.CreditedSickMinutes, CreditedVacationMinutes: summary.CreditedVacationMinutes,
		CreditedTrainingMinutes: summary.CreditedTrainingMinutes, CreditedOtherMinutes: summary.CreditedOtherMinutes,
		SickDays: summary.SickDays, VacationDays: summary.VacationDays, TrainingDays: summary.TrainingDays,
		PlannedShiftMinutes: summary.PlannedShiftMinutes, AdjustmentMinutes: summary.AdjustmentMinutes,
		BalanceMinutes: summary.BalanceMinutes, ClosingBalanceMinutes: summary.ClosingBalanceMinutes, IsClosed: summary.IsClosed,
		ClosedAt: summary.ClosedAt, ClosedBy: summary.ClosedBy, CloseReason: summary.CloseReason,
		FrozenClosingBalanceMinutes: summary.FrozenClosingBalanceMinutes, DriftMinutes: summary.DriftMinutes,
		CarryInFrozen: summary.CarryInFrozen, CarryInFrozenFromMonth: summary.CarryInFrozenFromMonth,
	}
	if summary.Adjustments != nil {
		result.Adjustments = make([]workforce.AdjustmentView, 0, len(summary.Adjustments))
		for _, adjustment := range summary.Adjustments {
			result.Adjustments = append(result.Adjustments, workforce.AdjustmentView{
				ID: adjustment.ID, Type: adjustment.Type, MinutesDelta: adjustment.MinutesDelta, EffectiveDate: adjustment.EffectiveDate.String(),
				Note: adjustment.Note, DecidedBy: adjustment.DecidedBy, DecidedAt: adjustment.DecidedAt,
			})
		}
	}
	return result, nil
}

func (c workTimeMonthCapability) DailyTargets(ctx context.Context, staffID int64, from, to string) ([]workforce.DailyTarget, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	targets, err := c.months.GetDailyTargets(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if targets == nil {
		return nil, nil
	}
	result := make([]workforce.DailyTarget, 0, len(targets))
	for _, target := range targets {
		result = append(result, workforce.DailyTarget{Date: target.Date.String(), TargetMinutes: target.TargetMinutes})
	}
	return result, nil
}

func (c workTimeMonthCapability) DailyProjection(ctx context.Context, staffID int64, from, to string) ([]workforce.DailyProjection, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	days, err := c.months.GetDailyProjection(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if days == nil {
		return nil, nil
	}
	result := make([]workforce.DailyProjection, 0, len(days))
	for _, day := range days {
		result = append(result, workforce.DailyProjection{
			Date: day.Date.String(), TargetMinutes: day.TargetMinutes, TargetSource: day.TargetSource, CreditMinutes: day.CreditMinutes,
			ActualMinutes: day.ActualMinutes, BalanceMinutes: day.BalanceMinutes,
		})
	}
	return result, nil
}

type balanceAdjustmentCapability struct {
	ledger StaffBalanceAdjustmentService
}

// BalanceAdjustmentCapability serves workforce.BalanceAdjustments from the
// retained ledger service.
func BalanceAdjustmentCapability(ledger StaffBalanceAdjustmentService) workforce.BalanceAdjustments {
	if ledger == nil {
		panic("balance adjustment capability: ledger service is required")
	}
	return balanceAdjustmentCapability{ledger: ledger}
}

func publicBalanceAdjustment(entity *StaffBalanceAdjustment) *workforce.StaffBalanceAdjustment {
	if entity == nil {
		return nil
	}
	return &workforce.StaffBalanceAdjustment{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Type: entity.Type, MinutesDelta: entity.MinutesDelta,
		EffectiveDate: entity.EffectiveDate.String(), Note: entity.Note, DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func (c balanceAdjustmentCapability) adjustment(entity *StaffBalanceAdjustment, err error) (*workforce.StaffBalanceAdjustment, error) {
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicBalanceAdjustment(entity), nil
}

func (c balanceAdjustmentCapability) ListAdjustments(ctx context.Context, staffID int64, from, to string) ([]*workforce.StaffBalanceAdjustment, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	entities, err := c.ledger.ListAdjustments(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if entities == nil {
		return nil, nil
	}
	result := make([]*workforce.StaffBalanceAdjustment, 0, len(entities))
	for _, entity := range entities {
		result = append(result, publicBalanceAdjustment(entity))
	}
	return result, nil
}

func (c balanceAdjustmentCapability) CreateAdjustment(ctx context.Context, staffID, decidedBy int64, request workforce.CreateBalanceAdjustmentRequest) (*workforce.StaffBalanceAdjustment, error) {
	effective, err := parseCapabilityDate(request.EffectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	return c.adjustment(c.ledger.CreateAdjustment(ctx, staffID, decidedBy, CreateBalanceAdjustmentRequest{
		Type: request.Type, MinutesDelta: request.MinutesDelta, EffectiveDate: effective, Note: request.Note,
	}))
}

func (c balanceAdjustmentCapability) DeleteAdjustment(ctx context.Context, staffID, adjustmentID, deletedBy int64) error {
	return mapTimeTrackingFailure(c.ledger.DeleteAdjustment(ctx, staffID, adjustmentID, deletedBy))
}

func (c balanceAdjustmentCapability) ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, carryoverMinutes int, note string) (*workforce.StaffBalanceAdjustment, error) {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	return c.adjustment(c.ledger.ResetBalance(ctx, staffID, decidedBy, effective, carryoverMinutes, note))
}

func (c balanceAdjustmentCapability) CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, balanceMinutes int, note string) (*workforce.StaffBalanceAdjustment, error) {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	return c.adjustment(c.ledger.CreateOpeningBalance(ctx, staffID, decidedBy, effective, balanceMinutes, note))
}

// OpeningBalanceBookingCapability exposes preview and booking through one
// Workforce boundary without widening the general ledger administration port.
func OpeningBalanceBookingCapability(ledger StaffBalanceAdjustmentService) workforce.OpeningBalanceBookings {
	if ledger == nil {
		return nil
	}
	return balanceAdjustmentCapability{ledger: ledger}
}

func (c balanceAdjustmentCapability) ValidateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, balanceMinutes int, note string) error {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return err
	}
	return mapTimeTrackingFailure(c.ledger.ValidateOpeningBalance(ctx, staffID, decidedBy, effective, balanceMinutes, note))
}

type monthClosingCapability struct {
	closing StaffMonthCloseService
}

// MonthClosingCapability serves workforce.MonthClosing from the retained
// month-close service.
func MonthClosingCapability(closing StaffMonthCloseService) workforce.MonthClosing {
	if closing == nil {
		panic("month closing capability: month close service is required")
	}
	return monthClosingCapability{closing: closing}
}

func (c monthClosingCapability) CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (*workforce.MonthCloseResult, error) {
	result, err := c.closing.CloseMonth(ctx, closedBy, year, month, reason)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if result == nil {
		return nil, nil
	}
	return &workforce.MonthCloseResult{
		Year: result.Year, Month: result.Month, ClosedStaff: result.ClosedStaff, SkippedStaff: result.SkippedStaff,
		Snapshots: publicMonthSnapshots(result.Snapshots),
	}, nil
}

func (c monthClosingCapability) ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) error {
	return mapTimeTrackingFailure(c.closing.ReopenMonth(ctx, staffID, reopenedBy, year, month, reason))
}

func (c monthClosingCapability) ListMonthStatus(ctx context.Context, year, month int) ([]*workforce.StaffMonthBalanceSnapshot, error) {
	snapshots, err := c.closing.ListMonthStatus(ctx, year, month)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicMonthSnapshots(snapshots), nil
}

type staffOverviewCapability struct {
	overview StaffOverviewService
}

// StaffOverviewCapability serves workforce.StaffOverview from the retained
// overview service.
func StaffOverviewCapability(overview StaffOverviewService) workforce.StaffOverview {
	if overview == nil {
		panic("staff overview capability: overview service is required")
	}
	return staffOverviewCapability{overview: overview}
}

func (c staffOverviewCapability) DashboardSummary(ctx context.Context, period string) (*workforce.DashboardSummary, error) {
	summary, err := c.overview.GetDashboardSummary(ctx, period)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if summary == nil {
		return nil, nil
	}
	return &workforce.DashboardSummary{
		ActiveStaffCount: summary.ActiveStaffCount, SickToday: summary.SickToday, VacationToday: summary.VacationToday,
		CurrentlyClockedIn: summary.CurrentlyClockedIn, ExpectedClockedIn: summary.ExpectedClockedIn,
		Period:                  workforce.DashboardPeriodTotals(summary.Period),
		SaldoSchoolTotalMinutes: summary.SaldoSchoolTotalMinutes, PendingRequestsCount: summary.PendingRequestsCount,
	}, nil
}

func (c staffOverviewCapability) TimeTrackingOverview(ctx context.Context, filters workforce.OverviewFilters) (*workforce.TimeTrackingOverview, error) {
	overview, err := c.overview.GetTimeTrackingOverview(ctx, OverviewFilters(filters))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if overview == nil {
		return nil, nil
	}
	result := &workforce.TimeTrackingOverview{Year: overview.Year, Month: overview.Month, Rows: make([]workforce.TimeTrackingOverviewRow, 0, len(overview.Rows))}
	for _, row := range overview.Rows {
		result.Rows = append(result.Rows, workforce.TimeTrackingOverviewRow(row))
	}
	return result, nil
}

type timeTrackingAuditLogCapability struct {
	log TimeTrackingAuditLogService
}

// TimeTrackingAuditLogCapability serves workforce.TimeTrackingAuditLog from
// the retained audit-log service.
func TimeTrackingAuditLogCapability(log TimeTrackingAuditLogService) workforce.TimeTrackingAuditLog {
	if log == nil {
		panic("time tracking audit log capability: audit log service is required")
	}
	return timeTrackingAuditLogCapability{log: log}
}

func (c timeTrackingAuditLogCapability) ListAuditLog(ctx context.Context, request workforce.AuditLogListRequest) (*workforce.AuditLogPage, error) {
	from, err := parseOptionalCapabilityDate(request.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseOptionalCapabilityDate(request.To, "to")
	if err != nil {
		return nil, err
	}
	page, err := c.log.ListAuditLog(ctx, AuditLogListRequest{
		From: from, To: to, StaffID: request.StaffID, ActorStaffID: request.ActorStaffID, Sources: request.Sources,
		Cursor: request.Cursor, Limit: request.Limit,
	})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if page == nil {
		return nil, nil
	}
	result := &workforce.AuditLogPage{NextCursor: page.NextCursor, RetentionCutoff: page.RetentionCutoff}
	if page.Events != nil {
		result.Events = make([]*workforce.AuditLogEvent, 0, len(page.Events))
		for _, event := range page.Events {
			if event == nil {
				result.Events = append(result.Events, nil)
				continue
			}
			public := &workforce.AuditLogEvent{
				OccurredAt: event.OccurredAt, Source: event.Source, EntryID: event.EntryID, Actor: workforce.AuditLogActor(event.Actor),
				Reason: event.Reason, Detail: event.Detail,
			}
			if event.Staff != nil {
				public.Staff = new(workforce.AuditLogPerson(*event.Staff))
			}
			result.Events = append(result.Events, public)
		}
	}
	return result, nil
}

type staffTimeExportCapability struct {
	export StaffTimeExportService
}

// StaffTimeExportCapability serves workforce.StaffTimeExport from the retained
// export service.
func StaffTimeExportCapability(export StaffTimeExportService) workforce.StaffTimeExport {
	if export == nil {
		panic("staff time export capability: export service is required")
	}
	return staffTimeExportCapability{export: export}
}

func (c staffTimeExportCapability) ExportStaffTime(ctx context.Context, request workforce.TimeExportRequest, actorAccountID int64, actorRole string) (*workforce.ExportFile, error) {
	file, err := c.export.Export(ctx, TimeExportRequest(request), actorAccountID, actorRole)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicExportFile(file), nil
}

func (c staffTimeExportCapability) DatevReport(ctx context.Context, request workforce.TimeExportRequest) (*workforce.DatevExportReport, error) {
	report, err := c.export.DatevReport(ctx, TimeExportRequest(request))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if report == nil {
		return nil, nil
	}
	result := &workforce.DatevExportReport{
		Format: report.Format, Year: report.Year, Month: report.Month, LineCount: report.LineCount, StaffExported: report.StaffExported,
		UnconfiguredCategories: report.UnconfiguredCategories, OpenMonth: report.OpenMonth,
	}
	if report.StaffSkipped != nil {
		result.StaffSkipped = make([]workforce.DatevSkippedStaff, 0, len(report.StaffSkipped))
		for _, skipped := range report.StaffSkipped {
			result.StaffSkipped = append(result.StaffSkipped, workforce.DatevSkippedStaff(skipped))
		}
	}
	return result, nil
}
