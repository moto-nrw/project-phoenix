package application

import (
	"cmp"
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The calendar days a sick report reaches and the day locks it takes on them.

// lockReportedDays takes, once and ascending, the day locks of every day of
// the report's range, past and half days included, and of every day the
// absence stampedBy already stamped, before the caller reads those stamps
// (I-2). A materialization or edit holding such a day stamps its rows and
// commits first; one waiting for the day reads the reports after this write
// and stamps accordingly. New stamps land only on the report's days, so the
// set is complete; taking it in one ascending pass keeps the per-day clears
// and the marks that follow from locking out of order. The reversal itself
// is keyed by the stamps: a malformed range adds no days rather than
// blocking it.
func (s *sickCascade) lockReportedDays(ctx context.Context, in workforce.SickCascadeInput, stampedBy int64) error {
	days := make(map[calendar.Date]bool)
	if report, err := parseSickReport(in); err == nil {
		for day := report.dateStart; !day.After(report.dateEnd); day = day.AddDays(1) {
			days[day] = true
		}
	}
	if err := s.addStampedDays(ctx, days, stampedBy); err != nil {
		return err
	}
	return s.acquireCascadeDayLocks(ctx, days)
}

// LockSickReportWrite takes the day locks of a pending sick-report write
// before Workforce locks the staff row, once and ascending (I-2, #6292): every
// day of the requested range, of the stored range of the edited or deleted
// report (absenceID), of the reports a new report may merge, and of their
// stamps. The cascade that follows needs no further day, so the order is
// days → Workforce's absence and staff locks → the cascade, and a
// materialization (days → staff key share) can no longer deadlock against
// it. Malformed dates and unknown ids add nothing; the write itself rejects
// them.
func (s *sickCascade) LockSickReportWrite(ctx context.Context, subjectStaffID, absenceID int64, dateStart, dateEnd string) error {
	if s.absences == nil {
		return nil
	}
	days := make(map[calendar.Date]bool)
	var stampedBy []int64
	if absenceID > 0 {
		stored, err := s.absences.FindStaffAbsence(ctx, absenceID)
		if err != nil || stored.AbsenceType != workforce.AbsenceTypeSick {
			return nil //nolint:nilerr // the write reports a missing absence itself
		}
		addDateRange(days, stored.DateStart, stored.DateEnd)
		addDateRange(days, cmp.Or(dateStart, stored.DateStart), cmp.Or(dateEnd, stored.DateEnd))
		stampedBy = append(stampedBy, absenceID)
	} else {
		addDateRange(days, dateStart, dateEnd)
		merged, err := s.absences.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
			StaffID: subjectStaffID, Types: []string{workforce.AbsenceTypeSick}, OverlapFrom: dateStart, OverlapTo: dateEnd,
		})
		if err != nil {
			return fmt.Errorf("sick report: load overlapping reports: %w", err)
		}
		for _, report := range merged {
			addDateRange(days, report.DateStart, report.DateEnd)
			stampedBy = append(stampedBy, report.ID)
		}
	}
	for _, id := range stampedBy {
		if err := s.addStampedDays(ctx, days, id); err != nil {
			return err
		}
	}
	return s.acquireCascadeDayLocks(ctx, days)
}

// addDateRange adds every day of [start, end]; a malformed range adds none.
func addDateRange(days map[calendar.Date]bool, start, end string) {
	from, errFrom := calendar.ParseDate(start)
	to, errTo := calendar.ParseDate(end)
	if errFrom != nil || errTo != nil {
		return
	}
	for day := from; !day.After(to); day = day.AddDays(1) {
		days[day] = true
	}
}

// addStampedDays adds the days of every block row absenceID stamped.
func (s *sickCascade) addStampedDays(ctx context.Context, days map[calendar.Date]bool, absenceID int64) error {
	rows, err := s.loadStampedBlockRows(ctx, absenceID)
	if err != nil || len(rows) == 0 {
		return err
	}
	instances, err := s.timetableData.GetActivityInstancesByID(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return fmt.Errorf("sick cascade: load stamped instances: %w", err)
	}
	for _, instance := range instances {
		days[instance.Date] = true
	}
	return nil
}

// sickCascadeDays expands the absence range into the full sick days, skipping
// half boundary days — a half sick day never cascades (#1843 product rule).
// The Timetable owner's SickReport decides each day, the rule the
// materializer applies to blocks planned after the report (#6292).
func sickCascadeDays(report sickReport) []calendar.Date {
	plan := timetable.SickReport{
		AbsenceID: report.absenceID, StaffID: report.subjectStaffID,
		DateStart: report.dateStart, DateEnd: report.dateEnd,
		SkipStartDay: report.skipStartDay, SkipEndDay: report.skipEndDay,
	}
	var days []calendar.Date
	for d := report.dateStart; !d.After(report.dateEnd); d = d.AddDays(1) {
		if plan.CoversFullDay(d) {
			days = append(days, d)
		}
	}
	return days
}

func sickCascadeDayDifference(before, after sickReport) (removed, added map[calendar.Date]bool) {
	beforeDays := dateSet(sickCascadeDays(before))
	afterDays := dateSet(sickCascadeDays(after))
	removed = make(map[calendar.Date]bool)
	added = make(map[calendar.Date]bool)
	for day := range beforeDays {
		if !afterDays[day] {
			removed[day] = true
		}
	}
	for day := range afterDays {
		if !beforeDays[day] {
			added[day] = true
		}
	}
	return removed, added
}

func dateSet(days []calendar.Date) map[calendar.Date]bool {
	set := make(map[calendar.Date]bool, len(days))
	for _, day := range days {
		set[day] = true
	}
	return set
}

func unionDateSets(left, right map[calendar.Date]bool) map[calendar.Date]bool {
	union := make(map[calendar.Date]bool, len(left)+len(right))
	for day := range left {
		union[day] = true
	}
	for day := range right {
		union[day] = true
	}
	return union
}

func sortedDateSet(set map[calendar.Date]bool) []calendar.Date {
	days := make([]calendar.Date, 0, len(set))
	for day := range set {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days
}
