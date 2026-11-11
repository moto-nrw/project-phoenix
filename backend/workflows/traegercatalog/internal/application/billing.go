package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// billing is what a rollout writes beside a school's offering copy.
type billing struct {
	pricesFrom *calendar.Date
	template   bool
}

// billingFor decides the tariff and the contract template of one offering at
// one school (ADR 0195 section 6). A school without a copy gets both from
// the month of the service start, at the earliest the current month. A
// school with a copy compares its actual immutable applied version with
// the target, including after missed releases. Changed terms start next
// month; a school that already holds the version gets nothing.
func billingFor(offering careplan.TraegerOffering, next offeringRelease, schoolID int64, phase enrollment.TraegerPhaseFields, today calendar.Date) billing {
	recorded := recordedVersion(offering, schoolID)
	previous := appliedDraft(offering, schoolID)
	result := billing{}
	switch {
	case recorded == nil:
		result.pricesFrom = monthFrom(phase.ServiceStartDate, monthStart(today))
		result.template = true
	case *recorded == next.version:
		return result
	case previous != nil:
		if !sameJSON(previous.Prices, next.draft.Prices) {
			result.pricesFrom = monthFrom(phase.ServiceStartDate, nextMonthStart(today))
		}
		result.template = !sameJSON(previous.ContractTemplate, next.draft.ContractTemplate)
	default:
		result.pricesFrom = monthFrom(phase.ServiceStartDate, nextMonthStart(today))
		result.template = true
	}
	if len(next.draft.Prices) == 0 {
		result.pricesFrom = nil
	}
	result.template = result.template && next.draft.ContractTemplate != nil
	return result
}

func recordedVersion(offering careplan.TraegerOffering, schoolID int64) *int {
	for _, school := range offering.Schools {
		if school.SchoolID == schoolID && school.Outcome != enrollment.TraegerOutcomeWithdrawn {
			return school.AppliedVersion
		}
	}
	return nil
}

// monthFrom is the later of the service start's month and earliest.
func monthFrom(serviceStart enrollment.Date, earliest calendar.Date) *calendar.Date {
	start, err := calendar.ParseDate(string(serviceStart))
	if err == nil && monthStart(start).After(earliest) {
		earliest = monthStart(start)
	}
	return &earliest
}

func monthStart(day calendar.Date) calendar.Date {
	return calendar.NewDate(day.Year(), day.Month(), 1)
}

func nextMonthStart(day calendar.Date) calendar.Date {
	if day.Month() == time.December {
		return calendar.NewDate(day.Year()+1, time.January, 1)
	}
	return calendar.NewDate(day.Year(), day.Month()+1, 1)
}

func appliedDraft(offering careplan.TraegerOffering, schoolID int64) *careplan.TraegerOfferingDraft {
	for _, school := range offering.Schools {
		if school.SchoolID == schoolID && school.Outcome != enrollment.TraegerOutcomeWithdrawn {
			return school.Applied
		}
	}
	return nil
}
