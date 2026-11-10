package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SickReportAbsences are Workforce's absence gate and absence rows the
// write lock of a sick report reads (#6232).
type SickReportAbsences interface {
	LockStaffAbsenceWrites(ctx context.Context, staffID int64) error
	FindStaffAbsence(ctx context.Context, id int64) (workforce.StaffAbsence, error)
	ListStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
}

// LockSickReportWrite takes the locks of one absence write before Workforce
// writes it (#6232), in the canonical order of the unit of work:
//
//  1. Workforce's absence gate of the subject, so every absence row of the
//     subject read below stays as read until commit;
//  2. the staff-shift gates of the subject and of the owners of the covers
//     attached to the shifts its sick reports stamped, ascending;
//  3. Timetable's recurrence gate, which the materialization and the series
//     writes hold while they plan rows from the sick reports;
//  4. the day lock of every day the write may stamp or release, ascending:
//     the requested range, the stored range of the edited or deleted
//     absence, the ranges of the subject's sick reports overlapping either
//     (a merge folds them in) and the days of their stamped block rows. Past
//     days are included, because the materializer stamps past blocks.
//
// Workforce locks the staff row only after this, and the cascade the write
// returns needs no gate and no day beyond these. A plan writer that reads
// sick reports holds the recurrence gate, the days it writes or the
// subject's staff-shift gate, so it either waits for this write or this
// write waits for it, and no lock cycle with the key share its rows take on
// the staff row can close. staffID names the
// subject of a new absence; an existing absence names its own. Malformed
// dates and unknown absences add nothing: the write itself reports them.
func (s *sickCascade) LockSickReportWrite(ctx context.Context, staffID, absenceID int64, dateStart, dateEnd string) error {
	if s.absences == nil {
		return errors.New("sick report lock: absence capability is not configured")
	}
	subject, err := s.lockedSubject(ctx, staffID, absenceID)
	if err != nil || subject <= 0 {
		return err
	}
	days, staffIDs, err := s.sickReportWriteScope(ctx, subject, absenceID, dateStart, dateEnd)
	if err != nil || len(staffIDs) == 0 {
		return err
	}
	if err := s.lockStaffSet(ctx, staffIDs); err != nil {
		return err
	}
	if err := s.timetableData.LockRecurrenceWrites(ctx); err != nil {
		return fmt.Errorf("sick report lock: recurrence gate: %w", err)
	}
	for _, day := range sortedDateSet(days) {
		if err := s.timetableData.AcquireSubstituteDayLock(ctx, day); err != nil {
			return fmt.Errorf("sick report lock: day lock %s: %w", day.String(), err)
		}
	}
	return nil
}

// sickReportWriteScope reads, under the subject's absence gate, the days
// and the staff-shift gates one write of the subject may need.
func (s *sickCascade) sickReportWriteScope(ctx context.Context, subject, absenceID int64, dateStart, dateEnd string) (map[calendar.Date]bool, []int64, error) {
	days := make(map[calendar.Date]bool)
	var reportIDs []int64
	var stored workforce.StaffAbsence
	if absenceID > 0 {
		var err error
		stored, err = s.absences.FindStaffAbsence(ctx, absenceID)
		switch {
		case errors.Is(err, workforce.ErrStaffAbsenceNotFound):
		case err != nil:
			return nil, nil, fmt.Errorf("sick report lock: load absence: %w", err)
		case stored.StaffID == subject:
			reportIDs = append(reportIDs, stored.ID)
			// An edit may move one bound only; the other keeps its stored day.
			dateStart, dateEnd = cmp.Or(dateStart, stored.DateStart), cmp.Or(dateEnd, stored.DateEnd)
		}
	}
	// Workforce validates sick ranges before writing. Leave invalid or oversized
	// ranges to that validation without expanding or locking them here. Other
	// absence types may legitimately span longer and have no sick cascade.
	if !addDateRange(days, dateStart, dateEnd) {
		return days, nil, nil
	}
	addDateRange(days, stored.DateStart, stored.DateEnd)
	overlapping, err := s.overlappingSickReports(ctx, subject, days)
	if err != nil {
		return nil, nil, err
	}
	for _, report := range overlapping {
		addDateRange(days, report.DateStart, report.DateEnd)
		reportIDs = append(reportIDs, report.ID)
	}
	staffIDs := []int64{subject}
	for _, id := range reportIDs {
		covers, err := s.reversalStaffIDs(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		staffIDs = append(staffIDs, covers...)
		if err := s.addStampedBlockDays(ctx, days, id); err != nil {
			return nil, nil, err
		}
	}
	return days, staffIDs, nil
}

// lockedSubject takes the absence gate of the write's subject and returns
// the subject; 0 when an edited or deleted absence does not exist.
func (s *sickCascade) lockedSubject(ctx context.Context, staffID, absenceID int64) (int64, error) {
	subject := staffID
	if absenceID > 0 {
		stored, err := s.absences.FindStaffAbsence(ctx, absenceID)
		if errors.Is(err, workforce.ErrStaffAbsenceNotFound) {
			// The write reports the missing absence itself.
			return 0, nil
		}
		if err != nil {
			return 0, fmt.Errorf("sick report lock: load absence: %w", err)
		}
		subject = stored.StaffID
	}
	if subject <= 0 {
		return 0, nil
	}
	if err := s.absences.LockStaffAbsenceWrites(ctx, subject); err != nil {
		return 0, fmt.Errorf("sick report lock: absence gate: %w", err)
	}
	return subject, nil
}

// overlappingSickReports lists the subject's sick reports that overlap the
// days, the reports a new or edited range merges.
func (s *sickCascade) overlappingSickReports(ctx context.Context, subject int64, days map[calendar.Date]bool) ([]workforce.StaffAbsence, error) {
	if len(days) == 0 {
		return nil, nil
	}
	sorted := sortedDateSet(days)
	reports, err := s.absences.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		StaffID: subject, Types: []string{workforce.AbsenceTypeSick},
		OverlapFrom: sorted[0].String(), OverlapTo: sorted[len(sorted)-1].String(),
	})
	if err != nil {
		return nil, fmt.Errorf("sick report lock: load overlapping reports: %w", err)
	}
	return reports, nil
}

// addStampedBlockDays adds the day of every block row the absence stamped.
func (s *sickCascade) addStampedBlockDays(ctx context.Context, days map[calendar.Date]bool, absenceID int64) error {
	rows, err := s.loadStampedBlockRows(ctx, absenceID)
	if err != nil || len(rows) == 0 {
		return err
	}
	instances, err := s.timetableData.GetActivityInstancesByID(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return fmt.Errorf("sick report lock: load stamped blocks: %w", err)
	}
	for _, instance := range instances {
		days[instance.Date] = true
	}
	return nil
}

// addDateRange adds every day of a bounded [start, end]. Invalid or oversized
// ranges add none and return false, leaving validation to Workforce.
func addDateRange(days map[calendar.Date]bool, start, end string) bool {
	from, errFrom := calendar.ParseDate(start)
	to, errTo := calendar.ParseDate(end)
	if errFrom != nil || errTo != nil || to.Before(from) || from.DaysUntil(to)+1 > workforce.MaxSickAbsenceRangeDays {
		return false
	}
	for day := from; ; day = day.AddDays(1) {
		days[day] = true
		if day == to {
			return true
		}
	}
}

// reversalStaffIDs lists the owners of the covers attached to a shift stamped
// by the absence.
func (s *sickCascade) reversalStaffIDs(ctx context.Context, absenceID int64) ([]int64, error) {
	shifts, err := s.stampedShifts(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("sick clear: discover stamped shifts: %w", err)
	}
	originShiftIDs := staffShiftIDs(shifts)
	if len(originShiftIDs) == 0 {
		// A non-nil empty OriginShiftIDs would match nothing; the unfiltered
		// listing is never reached through this path.
		return nil, nil
	}
	covers, err := s.shifts.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		OriginShiftIDs: originShiftIDs,
		Order:          []workforce.StaffShiftOrder{{Field: workforce.StaffShiftOrderID}},
	})
	if err != nil {
		return nil, fmt.Errorf("sick clear: discover stamped shift covers: %w", err)
	}
	staffIDs := make([]int64, 0, len(covers))
	for _, cover := range covers {
		staffIDs = append(staffIDs, cover.StaffID)
	}
	return staffIDs, nil
}

// lockStaffSet takes the staff-shift gates of the staff members once each,
// ascending.
func (s *sickCascade) lockStaffSet(ctx context.Context, staffIDs []int64) error {
	sort.Slice(staffIDs, func(i, j int) bool { return staffIDs[i] < staffIDs[j] })
	var previous int64
	for _, staffID := range staffIDs {
		if staffID <= 0 || staffID == previous {
			continue
		}
		if err := s.lockStaffWrites(ctx, staffID); err != nil {
			return err
		}
		previous = staffID
	}
	return nil
}
