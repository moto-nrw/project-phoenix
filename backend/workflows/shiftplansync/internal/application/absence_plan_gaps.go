package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// AbsencePlanGaps lists the absences in force whose plan rows from today on
// still lack their effect (#7021): an absence entered before every type
// cascaded (#6847) left the shifts and blocks planned at that time alone.
// It is the dry run of MarkSickForRange: a shift counts when the cascade
// would cancel it, a block row when the cascade would mark it absent. The
// reads are set-based: one absence listing, one shift listing, one
// assignment listing and one instance load, whatever the school's size.
func (s *sickCascade) AbsencePlanGaps(ctx context.Context) ([]workforce.AbsencePlanGap, error) {
	today := s.todayDate()
	reports, err := s.absencesInForce(ctx, today)
	if err != nil || len(reports) == 0 {
		return nil, err
	}
	staffIDs, last := gapScope(reports, today)
	shifts, err := s.shifts.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		StaffIDs: staffIDs, From: today.String(), To: last.String(), Cancelled: new(false),
	})
	if err != nil {
		return nil, fmt.Errorf("absence plan gaps: load shifts: %w", err)
	}
	rows, err := s.sickStamps.FindByStaffIDsAndDateRange(ctx, staffIDs, today, last)
	if err != nil {
		return nil, fmt.Errorf("absence plan gaps: load assignments: %w", err)
	}
	rows = slices.DeleteFunc(rows, func(row *timetable.InstanceStaff) bool { return row.IsAbsent })
	instances, err := s.timetableData.GetActivityInstancesByID(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return nil, fmt.Errorf("absence plan gaps: load instances: %w", err)
	}
	openShifts := openShiftDays(shifts)
	openBlocks := openBlockDays(rows, instances)
	gaps := make([]workforce.AbsencePlanGap, 0)
	// Two absences of one person may overlap; each day's rows count once,
	// for the first absence that covers it, as the first take-over stamps
	// them for that absence.
	claimed := make(map[staffDay]bool)
	for _, report := range reports {
		if gap := planGapOf(report, today, openShifts, openBlocks, claimed); gap.OpenShifts > 0 || gap.OpenBlocks > 0 {
			gaps = append(gaps, gap)
		}
	}
	return gaps, nil
}

// planGapOf counts the open rows on the absence's full days from today on
// that no earlier absence of the person claimed.
func planGapOf(report sickReport, today calendar.Date, openShifts, openBlocks map[staffDay]int, claimed map[staffDay]bool) workforce.AbsencePlanGap {
	gap := workforce.AbsencePlanGap{
		AbsenceID: report.absenceID, StaffID: report.subjectStaffID, AbsenceType: report.absenceType,
		DateStart: report.dateStart.String(), DateEnd: report.dateEnd.String(),
	}
	for _, day := range sickCascadeDays(report, today) {
		key := staffDay{staffID: report.subjectStaffID, day: day}
		if day.Before(today) || claimed[key] {
			continue
		}
		claimed[key] = true
		gap.OpenShifts += openShifts[key]
		gap.OpenBlocks += openBlocks[key]
	}
	return gap
}

// MarkAbsenceInForce re-applies the cascade of one stored absence on behalf
// of the actor (#7021). It reports false and writes nothing when the absence
// is no longer in force or does not exist in the school. The caller holds the write locks of the absence
// (LockSickReportWrite with its id), so the row read here stays as read.
func (s *sickCascade) MarkAbsenceInForce(ctx context.Context, absenceID, actorStaffID int64, actorAccountID *int64) (bool, error) {
	absence, err := s.absences.FindStaffAbsence(ctx, absenceID)
	if errors.Is(err, workforce.ErrStaffAbsenceNotFound) {
		return false, nil // deleted meanwhile, or not this school's
	}
	if err != nil {
		return false, fmt.Errorf("absence plan sync: load absence %d: %w", absenceID, err)
	}
	if !slices.Contains(workforce.PlanEffectiveAbsenceStatuses(), absence.Status) {
		return false, nil
	}
	skipStart, skipEnd := absence.HalfBoundaryDays()
	if err := s.MarkSickForRange(ctx, workforce.SickCascadeInput{
		SubjectStaffID: absence.StaffID, AbsenceType: absence.AbsenceType,
		DateStart: absence.DateStart, DateEnd: absence.DateEnd,
		SkipStartDay: skipStart, SkipEndDay: skipEnd,
		AbsenceID: absence.ID, ActorStaffID: actorStaffID, ActorAccountID: actorAccountID,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// absencesInForce lists the absences with plan effect that reach today or
// later, inside the cascade's window from today.
func (s *sickCascade) absencesInForce(ctx context.Context, today calendar.Date) ([]sickReport, error) {
	rows, err := s.absences.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		Statuses:    workforce.PlanEffectiveAbsenceStatuses(),
		OverlapFrom: today.String(),
		OverlapTo:   today.AddDays(workforce.MaxSickAbsenceRangeDays - 1).String(),
		Order: []workforce.StaffAbsenceOrder{
			{Field: workforce.StaffAbsenceOrderStaffID},
			{Field: workforce.StaffAbsenceOrderID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("absence plan gaps: load absences: %w", err)
	}
	reports := make([]sickReport, 0, len(rows))
	for _, row := range rows {
		skipStart, skipEnd := row.HalfBoundaryDays()
		report, err := parseSickReport(workforce.SickCascadeInput{
			SubjectStaffID: row.StaffID, AbsenceType: row.AbsenceType, AbsenceID: row.ID,
			DateStart: row.DateStart, DateEnd: row.DateEnd, SkipStartDay: skipStart, SkipEndDay: skipEnd,
		})
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// gapScope is the staff set and the last cascade day the gap reads cover.
func gapScope(reports []sickReport, today calendar.Date) ([]int64, calendar.Date) {
	staffIDs := make([]int64, 0, len(reports))
	var last calendar.Date
	for _, report := range reports {
		staffIDs = append(staffIDs, report.subjectStaffID)
		if _, end := cascadeWindow(report.dateStart, report.dateEnd, today); last == "" || end.After(last) {
			last = end
		}
	}
	slices.Sort(staffIDs)
	return slices.Compact(staffIDs), last
}

type staffDay struct {
	staffID int64
	day     calendar.Date
}

// openShiftDays counts per staff member and day the planned shifts the
// cascade would cancel: own shifts, not the covers a person owns for a
// colleague (shiftTakesSickCancellation). The listing holds no cancelled
// shift.
func openShiftDays(shifts []workforce.StaffShift) map[staffDay]int {
	counts := make(map[staffDay]int)
	for _, shift := range shifts {
		if shift.OriginShiftID != nil {
			continue
		}
		day, err := calendar.ParseDate(shift.Date)
		if err != nil {
			continue
		}
		counts[staffDay{staffID: shift.StaffID, day: day}]++
	}
	return counts
}

// openBlockDays counts per staff member and day the staffed rows of
// plannable blocks the cascade would mark absent (markBlocksForSickDay).
func openBlockDays(rows []*timetable.InstanceStaff, instances map[int64]*timetable.ScheduledInstance) map[staffDay]int {
	counts := make(map[staffDay]int)
	for _, row := range rows {
		instance := instances[row.InstanceID]
		if instance == nil || !isPlannableInstance(instance) {
			continue
		}
		counts[staffDay{staffID: row.StaffID, day: instance.Date}]++
	}
	return counts
}
