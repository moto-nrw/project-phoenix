package schoolcalendar

import (
	"strconv"
	"time"
)

// WeekCycle is the A/B-week alternation of a calendar period: Length weeks
// repeat from the Monday-anchored Anchor date (DateLayout, empty when unset).
type WeekCycle struct {
	Length int
	Anchor string
}

// WeekCycle returns the period's alternation settings.
func (p CalendarPeriod) WeekCycle() WeekCycle {
	return WeekCycle{Length: p.WeekCycleLength, Anchor: p.WeekCycleAnchor}
}

// WeekCycleOf reads the alternation of any row shaped like a period: the
// cycle length and an optional anchor of any calendar-date string type. A
// nil anchor means no anchor is set.
func WeekCycleOf[D ~string](length int, anchor *D) WeekCycle {
	cycle := WeekCycle{Length: length}
	if anchor != nil {
		cycle.Anchor = string(*anchor)
	}
	return cycle
}

// WeekPatternApplies reports whether a planning row with the given
// weekPattern occurs on date inside cycle. weekPattern 0 means every week;
// 1, 2, ... name week A, B, ... of the cycle.
//
// This is the single A/B-week engine: materialization, shift series, staff
// notices and care-offering validation all decide through it, so a catalog
// link is never accepted for a day the materializer would skip.
//
// The week index is a day-based difference from the anchor (never ISO week
// numbers, which break at the turn of the year). Both days are anchored at
// UTC midnight before subtracting, so DST transitions in Europe/Berlin
// (167- or 169-hour weeks) cannot skew the count. An unparseable date or
// anchor counts as "no anchor set": the row is allowed rather than silently
// dropped.
func WeekPatternApplies(weekPattern int, date string, cycle WeekCycle) bool {
	if weekPattern == 0 {
		return true
	}
	if cycle.Length <= 1 || cycle.Anchor == "" {
		return true
	}
	anchor, err := time.Parse(DateLayout, cycle.Anchor)
	if err != nil {
		return true
	}
	day, err := time.Parse(DateLayout, date)
	if err != nil {
		return true
	}
	daysDiff := int(day.Sub(anchor).Hours() / 24)
	weeksDiff := daysDiff / 7
	// Go's integer division truncates toward zero; floor for negative
	// offsets that do not fall on a week boundary.
	if daysDiff < 0 && daysDiff%7 != 0 {
		weeksDiff--
	}
	currentPattern := ((weeksDiff % cycle.Length) + cycle.Length) % cycle.Length
	currentPattern++ // 1-based: 1=A, 2=B, ...
	return currentPattern == weekPattern
}

// DefaultSchoolYear names the German school year containing today: it starts
// on August 1st of the current year when today is in August or later,
// otherwise on August 1st of the previous year, and always ends on July 31st
// of the following year. today uses DateLayout; an unparseable value yields
// empty results, which the period validation then rejects.
//
// MUST stay in sync with the frontend helper schoolYearPeriodDefaults in
// frontend/src/app/[tenant]/(protected)/timetables/page.tsx and with
// schoolYearBounds in backend/simulate/fullday.go — all three derive the same
// name ("Schuljahr YYYY/YYYY+1") and the same date bounds, so the bootstrap
// endpoint, the client-side prefill, and the demo school-year rollover never
// diverge. (simulate cannot import this module, hence its own copy.)
func DefaultSchoolYear(today string) (name, startDate, endDate string) {
	day, err := time.Parse(DateLayout, today)
	if err != nil {
		return "", "", ""
	}
	startYear := day.Year()
	if day.Month() < time.August {
		startYear--
	}
	name = "Schuljahr " + strconv.Itoa(startYear) + "/" + strconv.Itoa(startYear+1)
	return name,
		time.Date(startYear, time.August, 1, 0, 0, 0, 0, time.UTC).Format(DateLayout),
		time.Date(startYear+1, time.July, 31, 0, 0, 0, 0, time.UTC).Format(DateLayout)
}
