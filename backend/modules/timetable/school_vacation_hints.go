package timetable

import (
	"context"
	"sort"
)

// SchoolVacationHintLookaheadDays is how far ahead the vacation hint looks
// (#5345): vacations that start within the next eight weeks, or are running.
const SchoolVacationHintLookaheadDays = 56

// SchoolVacationWindow is one vacation of the school's federal state, as the
// School Calendar names it. Acknowledged marks a window the school confirmed
// as planned care.
type SchoolVacationWindow struct {
	Name         string
	StartDate    string
	EndDate      string
	Acknowledged bool
}

// SchoolVacationCandidate is one planned Regeltermin (a template-backed,
// period-bound occurrence) with the facts the hint needs.
type SchoolVacationCandidate struct {
	Date string
	// SeriesIncludesClosingDays marks a series planned on closing days on
	// purpose (holiday care); its occurrences never count.
	SeriesIncludesClosingDays bool
	// InHolidayPeriod marks a series planned in a holiday period the school
	// entered itself; its occurrences are deliberate holiday care too.
	InHolidayPeriod bool
}

// SchoolVacationHint names one vacation window with Regeltermine the school
// may not have meant to plan, and how many of them lie in it from CancelFrom
// on. CancelFrom is the later of the first vacation day and today, the first
// day the bulk cancellation would touch.
type SchoolVacationHint struct {
	Name       string `json:"name"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	CancelFrom string `json:"cancel_from"`
	Count      int    `json:"count"`
}

// SchoolVacationHintQuery answers the vacation hint of the caller's school.
type SchoolVacationHintQuery interface {
	SchoolVacationHints(ctx context.Context) ([]SchoolVacationHint, error)
}

// SelectSchoolVacationHints decides the hints (#5345). A window counts when
// the school has not confirmed it, it has not ended before today and it
// starts no later than horizon. Inside it, from today on, a candidate counts
// unless its day is a statutory holiday or a closing day, or its series is
// deliberate holiday care (include_closing_days or a holiday period). A
// window without a counted Regeltermin gives no hint. Hints come ordered by
// start date. The selection never changes a Termin.
func SelectSchoolVacationHints(windows []SchoolVacationWindow, candidates []SchoolVacationCandidate, days NonWorkingDays, today, horizon string) []SchoolVacationHint {
	hints := make([]SchoolVacationHint, 0, len(windows))
	for _, window := range windows {
		if window.Acknowledged || window.EndDate < today || window.StartDate > horizon {
			continue
		}
		from := max(window.StartDate, today)
		count := countVacationCandidates(candidates, days, from, window.EndDate)
		if count == 0 {
			continue
		}
		hints = append(hints, SchoolVacationHint{
			Name: window.Name, StartDate: window.StartDate, EndDate: window.EndDate, CancelFrom: from, Count: count,
		})
	}
	sort.SliceStable(hints, func(i, j int) bool { return hints[i].StartDate < hints[j].StartDate })
	return hints
}

// countVacationCandidates counts the candidates of [from, to] that are
// neither on a non-working day nor deliberate holiday care.
func countVacationCandidates(candidates []SchoolVacationCandidate, days NonWorkingDays, from, to string) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.Date < from || candidate.Date > to {
			continue
		}
		if candidate.SeriesIncludesClosingDays || candidate.InHolidayPeriod || days.Skip(candidate.Date, false) != SkipNone {
			continue
		}
		count++
	}
	return count
}
