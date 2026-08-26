package users

import (
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// EvaluateCareBookingStates interprets effective care-counting booking
// windows for one reference day. It is deliberately pure so every trigger and
// reader applies the same half-open date rules.
func EvaluateCareBookingStates(facts []userModels.CareBookingFacts, on timezone.Date) []userModels.CareBookingEvaluation {
	result := make([]userModels.CareBookingEvaluation, 0, len(facts))
	for _, child := range facts {
		result = append(result, evaluateCareBookingState(child, on))
	}
	return result
}

func evaluateCareBookingState(child userModels.CareBookingFacts, on timezone.Date) userModels.CareBookingEvaluation {
	evaluation := bookingEvaluationIdentity(child)
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

func effectiveCareBookingPeriods(periods []userModels.CareBookingPeriod) []userModels.CareBookingPeriod {
	result := make([]userModels.CareBookingPeriod, 0, len(periods))
	for _, period := range periods {
		if carePeriodProjectsDay(period) {
			result = append(result, period)
		}
	}
	return sortedCareBookingPeriods(result)
}

func carePeriodProjectsDay(period userModels.CareBookingPeriod) bool {
	if len(period.Days) == 0 {
		return false
	}
	if period.ValidFrom == nil || period.ValidUntil == nil {
		return true
	}
	if !period.ValidFrom.Before(*period.ValidUntil) {
		return false
	}
	startISO := isoWeekday(period.ValidFrom.Weekday())
	for _, day := range period.Days {
		targetISO, ok := enrollmentModels.CanonicalDayToISOWeekday(day)
		if ok && period.ValidFrom.AddDays((targetISO-startISO+7)%7).Before(*period.ValidUntil) {
			return true
		}
	}
	return false
}

func isoWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func bookingEvaluationIdentity(child userModels.CareBookingFacts) userModels.CareBookingEvaluation {
	return userModels.CareBookingEvaluation{
		StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
		SchoolClass: child.SchoolClass,
	}
}

func sortedCareBookingPeriods(periods []userModels.CareBookingPeriod) []userModels.CareBookingPeriod {
	result := append([]userModels.CareBookingPeriod(nil), periods...)
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

func bookingSourcesEndingOn(
	periods []userModels.CareBookingPeriod, end timezone.Date,
) (int64, []userModels.CareExitSourceOffering) {
	var requestChildID int64
	offerings := make([]userModels.CareExitSourceOffering, 0)
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
func latestEndedCareComponent(periods []userModels.CareBookingPeriod, on timezone.Date) (*timezone.Date, int) {
	var end *timezone.Date
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
func activeCareComponent(periods []userModels.CareBookingPeriod, on timezone.Date) (*timezone.Date, int) {
	var end *timezone.Date
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
	if source < 0 || end == nil {
		return end, source
	}
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
