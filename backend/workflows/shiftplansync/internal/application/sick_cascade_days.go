package application

import (
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
	rows, err := s.loadStampedBlockRows(ctx, stampedBy)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		instances, err := s.timetableData.GetActivityInstancesByID(ctx, instanceStaffInstanceIDs(rows))
		if err != nil {
			return fmt.Errorf("sick cascade: load stamped instances: %w", err)
		}
		for _, instance := range instances {
			days[instance.Date] = true
		}
	}
	return s.acquireCascadeDayLocks(ctx, days)
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
