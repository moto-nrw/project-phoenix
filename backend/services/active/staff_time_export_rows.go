package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
			row := buildMonthExportRow(staff, summary)
			rows = append(rows, row)
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
		CreditedTrainingMinutes: summary.CreditedTrainingMinutes,
		CreditedOtherMinutes:    summary.CreditedOtherMinutes,
		SickDays:                summary.SickDays,
		VacationDays:            summary.VacationDays,
		TrainingDays:            summary.TrainingDays,
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
		case activeModels.BalanceAdjustmentTypeOpening:
			row.OpeningMinutes += adjustment.MinutesDelta
		}
	}
	if staff.EmploymentType != nil {
		row.EmploymentType = *staff.EmploymentType
	}
	if staff.PersonnelNumber != nil {
		row.PersonnelNumber = *staff.PersonnelNumber
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

// DayExportRows exposes the single-staff export's merged session/absence rows.
// It delegates to the batched path so both export forms use the same loading
// and rendering code.
func (s *workSessionService) DayExportRows(ctx context.Context, staffID int64, from, to timezone.Date) ([]DayExportRow, error) {
	rowsByStaff, err := s.DayExportRowsByStaffIDs(ctx, []int64{staffID}, from, to)
	if err != nil {
		return nil, err
	}
	return rowsByStaff[staffID], nil
}

// DayExportRowsByStaffIDs builds all cross-staff evidence rows with a constant
// number of repository calls. It intentionally skips weekly summaries because
// the export only consumes the per-session cells.
func (s *workSessionService) DayExportRowsByStaffIDs(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]DayExportRow, error) {
	result := make(map[int64][]DayExportRow, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}

	sessionsByStaff, err := s.repo.GetHistoryByStaffIDs(ctx, staffIDs, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions for export: %w", err)
	}

	absencesByStaff := make(map[int64][]*activeModels.StaffAbsence, len(staffIDs))
	if s.absenceRepo != nil {
		absencesByStaff, err = s.absenceRepo.GetByStaffIDsAndDateRange(ctx, staffIDs, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to get absences for export: %w", err)
		}
	}

	allSessions := make([]*activeModels.WorkSession, 0)
	for _, staffID := range staffIDs {
		allSessions = append(allSessions, sessionsByStaff[staffID]...)
	}
	responses, err := s.buildDayExportSessionResponses(ctx, allSessions)
	if err != nil {
		return nil, err
	}
	responsesByStaff := make(map[int64][]*SessionResponse, len(staffIDs))
	for _, response := range responses {
		responsesByStaff[response.StaffID] = append(responsesByStaff[response.StaffID], response)
	}

	for _, staffID := range staffIDs {
		absences := clampAbsencesToRange(absencesByStaff[staffID], from, to)
		merged := s.buildExportRows(responsesByStaff[staffID], absences)
		rows := make([]DayExportRow, 0, len(merged))
		for _, row := range merged {
			rows = append(rows, DayExportRow{Date: row.Date, Cells: row.Row})
		}
		result[staffID] = rows
	}
	return result, nil
}

func (s *workSessionService) buildDayExportSessionResponses(ctx context.Context, sessions []*activeModels.WorkSession) ([]*SessionResponse, error) {
	sessionIDs := make([]int64, len(sessions))
	sessionIDValues := make([]interface{}, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.ID
		sessionIDValues[i] = session.ID
	}

	editCounts, err := s.auditRepo.CountManualBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get edit counts: %w", err)
	}

	breaksBySession := make(map[int64][]*activeModels.WorkSessionBreak, len(sessions))
	if len(sessionIDValues) > 0 {
		options := modelBase.NewQueryOptions()
		options.Filter.In("session_id", sessionIDValues...)
		breaks, err := s.breakRepo.List(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("failed to get breaks for export: %w", err)
		}
		for _, brk := range breaks {
			breaksBySession[brk.SessionID] = append(breaksBySession[brk.SessionID], brk)
		}
	}

	now := time.Now()
	responses := make([]*SessionResponse, len(sessions))
	for i, session := range sessions {
		breaks := breaksBySession[session.ID]
		responses[i] = &SessionResponse{
			WorkSession:  session,
			BreakMinutes: totalBreakMinutes(session, breaks, now),
			NetMinutes:   netMinutesWithBreaks(session, breaks, now),
			EditCount:    editCounts[session.ID],
		}
	}
	return responses, nil
}
