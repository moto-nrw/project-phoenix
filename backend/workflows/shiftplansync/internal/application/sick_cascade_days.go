package application

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The calendar days a sick report reaches and the day locks it takes on them.

// lockReportedDays takes the day locks of the report's future sick days
// before the reversal reads its stamps. A materialization holding a day has
// stamped its new rows for this report and commits them first; one waiting
// for the day reads the reports after the deletion and stamps nothing
// (#6292). The reversal itself is keyed by the stamps: a malformed range
// locks nothing rather than blocking it.
func (s *sickCascade) lockReportedDays(ctx context.Context, in workforce.SickCascadeInput) error {
	report, err := parseSickReport(in)
	if err != nil {
		return nil //nolint:nilerr // the stamps, not the range, drive the reversal
	}
	return s.acquireCascadeDayLocks(ctx, dateSet(sickCascadeDays(report)))
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
