package domain

import (
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareBookingPeriod is one effective care-counting booking window. Validity
// is half-open: ValidFrom is inclusive and ValidUntil is the first day on
// which the booking no longer applies. Nil bounds are unbounded.
type CareBookingPeriod struct {
	ValidFrom            *calendar.Date
	ValidUntil           *calendar.Date
	Days                 []string
	SourceRequestChildID int64
	SourceOfferings      []careplan.CareExitSourceOffering
}

// CareBookingFacts is what the evaluator interprets for one child. Names and
// class are read-model data for the operator preview.
type CareBookingFacts struct {
	StudentID     int64
	FirstName     string
	LastName      string
	SchoolClass   string
	EnrolledUntil *calendar.Date
	// ConfirmedBookinglessDay is supplied only by a mutation that explicitly
	// confirmed removing the final care booking. It lets the evaluator retain
	// the boundary after the mutation has removed every source period.
	ConfirmedBookinglessDay *calendar.Date
	Periods                 []CareBookingPeriod
}

// CareBookingEvaluation is the single interpretation of CareBookingFacts used
// by the setting impact check, natural expiry reconciliation and
// operational participation. A nil FirstBookinglessDay means no completion is
// needed.
type CareBookingEvaluation struct {
	StudentID            int64
	FirstName            string
	LastName             string
	SchoolClass          string
	HasCareDays          bool
	FirstBookinglessDay  *calendar.Date
	SourceRequestChildID int64
	SourceOfferings      []careplan.CareExitSourceOffering
}

// canonicalDayISOWeekday maps the stored day abbreviations of a booking to
// their ISO weekday numbers.
var canonicalDayISOWeekday = map[string]int{
	"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6, "sun": 7,
}

// EvaluateCareBookingStates interprets the effective care-counting booking
// windows for one reference day. It is deliberately pure so every trigger and
// reader applies the same half-open date rules.
func EvaluateCareBookingStates(facts []CareBookingFacts, on calendar.Date) []CareBookingEvaluation {
	result := make([]CareBookingEvaluation, 0, len(facts))
	for _, child := range facts {
		result = append(result, evaluateCareBookingState(child, on))
	}
	return result
}

func evaluateCareBookingState(child CareBookingFacts, on calendar.Date) CareBookingEvaluation {
	evaluation := CareBookingEvaluation{
		StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
		SchoolClass: child.SchoolClass,
	}
	periods := effectiveCareBookingPeriods(child.Periods)
	end, source := activeCareComponent(periods, on)
	if source < 0 {
		end, source = latestEndedCareComponent(periods, on)
	} else {
		evaluation.HasCareDays = true
	}
	if source < 0 || end == nil {
		if child.ConfirmedBookinglessDay != nil && !evaluation.HasCareDays {
			gap := *child.ConfirmedBookinglessDay
			evaluation.FirstBookinglessDay = &gap
		}
		return evaluation
	}
	// Enrollment is inclusive, booking validity is exclusive. A booking end
	// after the last care day is therefore the ordinary simultaneous end.
	if child.EnrolledUntil != nil && end.After(*child.EnrolledUntil) {
		return evaluation
	}
	gap := *end
	evaluation.FirstBookinglessDay = &gap
	evaluation.SourceRequestChildID, evaluation.SourceOfferings = bookingSourcesEndingOn(periods, gap)
	return evaluation
}

func effectiveCareBookingPeriods(periods []CareBookingPeriod) []CareBookingPeriod {
	result := make([]CareBookingPeriod, 0, len(periods))
	for _, period := range periods {
		if carePeriodProjectsDay(period) {
			result = append(result, period)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ValidFrom == nil {
			return result[j].ValidFrom != nil
		}
		if result[j].ValidFrom == nil {
			return false
		}
		return result[i].ValidFrom.Before(*result[j].ValidFrom)
	})
	return result
}

// carePeriodProjectsDay reports whether a window contains at least one of its
// booked weekdays.
func carePeriodProjectsDay(period CareBookingPeriod) bool {
	if len(period.Days) == 0 {
		return false
	}
	if period.ValidFrom == nil || period.ValidUntil == nil {
		return true
	}
	if !period.ValidFrom.Before(*period.ValidUntil) {
		return false
	}
	startISO := ISOWeekday(*period.ValidFrom)
	for _, day := range period.Days {
		targetISO, ok := canonicalDayISOWeekday[strings.ToLower(strings.TrimSpace(day))]
		if ok && period.ValidFrom.AddDays((targetISO-startISO+7)%7).Before(*period.ValidUntil) {
			return true
		}
	}
	return false
}

func bookingSourcesEndingOn(periods []CareBookingPeriod, end calendar.Date) (int64, []careplan.CareExitSourceOffering) {
	var requestChildID int64
	offerings := make([]careplan.CareExitSourceOffering, 0)
	for _, period := range periods {
		if period.ValidUntil == nil || *period.ValidUntil != end {
			continue
		}
		if period.ValidFrom != nil && !period.ValidFrom.Before(end) {
			continue
		}
		if requestChildID == 0 {
			requestChildID = period.SourceRequestChildID
		}
		offerings = append(offerings, period.SourceOfferings...)
	}
	return requestChildID, offerings
}

// latestEndedCareComponent preserves an already-created gap when a scheduler
// run was missed. Future-only periods are not evidence of a past care end.
func latestEndedCareComponent(periods []CareBookingPeriod, on calendar.Date) (*calendar.Date, int) {
	var end *calendar.Date
	source := -1
	for index := range periods {
		period := periods[index]
		if period.ValidFrom != nil && period.ValidFrom.After(on) {
			break
		}
		if period.ValidUntil == nil || period.ValidUntil.After(on) {
			continue
		}
		if end == nil || period.ValidUntil.After(*end) {
			candidate := *period.ValidUntil
			end = &candidate
			source = index
		}
	}
	return end, source
}

// activeCareComponent returns the exclusive end of the connected booking
// component containing on. A nil end is unbounded; source is -1 when no care
// booking covers the reference day.
func activeCareComponent(periods []CareBookingPeriod, on calendar.Date) (*calendar.Date, int) {
	end, source := coveringCareEnd(periods, on)
	if source < 0 || end == nil {
		return end, source
	}
	return extendCareComponent(periods, end, source)
}

func coveringCareEnd(periods []CareBookingPeriod, on calendar.Date) (*calendar.Date, int) {
	var end *calendar.Date
	source := -1
	for index := range periods {
		period := periods[index]
		if period.ValidFrom != nil && period.ValidFrom.After(on) {
			break
		}
		if period.ValidUntil != nil && !period.ValidUntil.After(on) {
			continue
		}
		if period.ValidUntil == nil {
			return nil, index
		}
		candidate := *period.ValidUntil
		if end == nil || candidate.After(*end) {
			end = &candidate
			source = index
		}
	}
	return end, source
}

// extendCareComponent follows windows that start before the current end and
// reach past it, so back-to-back bookings form one component.
func extendCareComponent(periods []CareBookingPeriod, end *calendar.Date, source int) (*calendar.Date, int) {
	for index := range periods {
		period := periods[index]
		if period.ValidFrom != nil && period.ValidFrom.After(*end) {
			break
		}
		if period.ValidUntil == nil {
			return nil, index
		}
		if period.ValidUntil.After(*end) {
			candidate := *period.ValidUntil
			end = &candidate
			source = index
		}
	}
	return end, source
}
