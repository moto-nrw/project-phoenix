package active

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// GetMonthExportRows builds the payroll rows for every active staff member
// (#1417 2b). month == 0 means the whole year (January through the running
// month for the current year). It reuses the #1986 prefetch: one load for all
// staff members, then the UNCHANGED month math per staff member and month —
// the export cannot drift from the Monatskarte by construction.
func (s *staffOverviewService) GetMonthExportRows(ctx context.Context, year, month int) ([]MonthExportRow, error) {
	today := s.today()
	if year < 2000 || year > 2100 {
		return nil, fmt.Errorf("%w: year out of range", ErrTimeExportInvalid)
	}
	if month < 0 || month > 12 {
		return nil, fmt.Errorf("%w: month out of range", ErrTimeExportInvalid)
	}
	if year > today.Year {
		return nil, fmt.Errorf("%w: year %d lies in the future", ErrTimeExportInvalid, year)
	}
	if month != 0 && monthOf(today).before(monthKey{Year: year, Month: month}) {
		return nil, fmt.Errorf("%w: month %04d-%02d lies in the future", ErrTimeExportInvalid, year, month)
	}

	firstKey := monthKey{Year: year, Month: month}
	lastKey := firstKey
	if month == 0 {
		firstKey = monthKey{Year: year, Month: 1}
		lastKey = monthKey{Year: year, Month: 12}
		if todayKey := monthOf(today); todayKey.before(lastKey) && year == today.Year {
			lastKey = todayKey
		}
	}

	staffMembers, err := s.activeStaff(ctx)
	if err != nil {
		return nil, err
	}
	sortStaffByName(staffMembers)
	if len(staffMembers) == 0 {
		return []MonthExportRow{}, nil
	}

	// One prefetch for the whole request. `lower` widens the window to the
	// first requested month: for a year export the early months may lie before
	// the account anchor and are then computed as standalone months — exactly
	// like the per-staff Monatskarte does.
	lower := firstKey.firstDay()
	prefetch, err := s.buildPrefetch(ctx, staffMembers, &lower, lastKey)
	if err != nil {
		return nil, err
	}
	monthSvc := newPrefetchedMonthService(prefetch, s.logger)

	rows := make([]MonthExportRow, 0, len(staffMembers))
	for _, staff := range staffMembers {
		for key := firstKey; !lastKey.before(key); key = key.next() {
			summary, err := monthSvc.GetMonthSummary(ctx, staff.ID, key.Year, key.Month)
			if err != nil {
				return nil, fmt.Errorf("failed to compute month summary for staff %d: %w", staff.ID, err)
			}
			rows = append(rows, buildMonthExportRow(staff, summary))
		}
	}
	return rows, nil
}

// buildMonthExportRow copies a MonthSummary into the export shape. The only
// transformation is splitting the adjustment ledger by type — payout,
// comp_time and reset are the categories that later map to DATEV Lohnarten,
// so the file carries them as separate columns.
func buildMonthExportRow(staff *userModels.Staff, summary *MonthSummary) MonthExportRow {
	row := MonthExportRow{
		StaffID:                 summary.StaffID,
		Year:                    summary.Year,
		Month:                   summary.Month,
		CarryInMinutes:          summary.CarryInMinutes,
		TargetMinutes:           summary.TargetMinutesToDate,
		ActualMinutes:           summary.ActualMinutes,
		CreditedSickMinutes:     summary.CreditedSickMinutes,
		CreditedVacationMinutes: summary.CreditedVacationMinutes,
		CreditedOtherMinutes:    summary.CreditedOtherMinutes,
		SickDays:                summary.SickDays,
		VacationDays:            summary.VacationDays,
		BalanceMinutes:          summary.BalanceMinutes,
		ClosingBalanceMinutes:   summary.ClosingBalanceMinutes,
		DriftMinutes:            summary.DriftMinutes,
		IsClosed:                summary.IsClosed,
		ClosedAt:                summary.ClosedAt,
	}
	// The authoritative month-end carry of a closed month is the FROZEN value —
	// the number the following month really starts from (#1986). The live
	// difference stays visible as DriftMinutes instead of being smoothed away.
	if summary.IsClosed && summary.FrozenClosingBalanceMinutes != nil {
		row.ClosingBalanceMinutes = *summary.FrozenClosingBalanceMinutes
	}
	for _, adjustment := range summary.Adjustments {
		switch adjustment.Type {
		case activeModels.BalanceAdjustmentTypePayout:
			row.PayoutMinutes += adjustment.MinutesDelta
		case activeModels.BalanceAdjustmentTypeCompTime:
			row.CompTimeMinutes += adjustment.MinutesDelta
		case activeModels.BalanceAdjustmentTypeReset:
			row.ResetMinutes += adjustment.MinutesDelta
		}
	}
	if staff.EmploymentType != nil {
		row.EmploymentType = *staff.EmploymentType
	}
	row.FirstName, row.LastName = staffNames(staff)
	return row
}

// --- day rows --------------------------------------------------------------

// DayExportRow is one rendered day cell-row of the per-staff export, exposed
// for the cross-staff evidence export.
type DayExportRow struct {
	Date  timezone.Date
	Cells []string
}

// DayExportRows exposes the single-staff export's merged session/absence rows
// (#1417 2b): same data loading, same cell rendering, so a day in the
// cross-staff file is identical to the per-staff download.
func (s *workSessionService) DayExportRows(ctx context.Context, staffID int64, from, to timezone.Date) ([]DayExportRow, error) {
	historyResp, err := s.GetHistory(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions for export: %w", err)
	}
	var absences []*activeModels.StaffAbsence
	if s.absenceRepo != nil {
		absences, err = s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to get absences for export: %w", err)
		}
	}
	merged := s.buildExportRows(historyResp.Sessions, absences)
	rows := make([]DayExportRow, 0, len(merged))
	for _, row := range merged {
		rows = append(rows, DayExportRow{Date: row.Date, Cells: row.Row})
	}
	return rows, nil
}
