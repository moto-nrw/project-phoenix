package application

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
)

var weekdays = map[string]time.Weekday{
	"monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday,
}

// orderingRules parses the stored settings. A value the model does not use
// is not checked, so a school can switch models without cleaning up first.
func orderingRules(settings ports.OrderingSettings) (domain.OrderingRules, error) {
	rules := domain.OrderingRules{
		Model:         domain.RegistrationModel(settings.Model),
		Registration:  deadline(settings.RegistrationKind, settings.RegistrationLeadDays, settings.RegistrationWeekday, settings.RegistrationTime),
		Cancellation:  deadline(settings.CancellationKind, settings.CancellationLeadDays, settings.CancellationWeekday, settings.CancellationTime),
		AnnualChanges: domain.AnnualChanges(settings.AnnualChanges),
		Sick:          domain.SickMode(settings.SickAfterDeadline),
		Rhythm:        domain.Rhythm(settings.OrderRhythm),
		Detail:        domain.Detail(settings.OrderDetail),
	}
	if rules.Model == domain.ModelAnnual && settings.AnnualDeadline != "" {
		date, err := domain.ParseDate(settings.AnnualDeadline)
		if err != nil {
			return domain.OrderingRules{}, fmt.Errorf("%w: annual deadline %q", ErrInvalidCutoff, settings.AnnualDeadline)
		}
		rules.AnnualDeadline = date
	}
	if !rules.Valid() {
		return domain.OrderingRules{}, fmt.Errorf("%w: %+v", ErrInvalidCutoff, settings)
	}
	return rules, nil
}

func deadline(kind string, leadDays int, weekday, clock string) domain.Deadline {
	return domain.Deadline{Kind: domain.DeadlineKind(kind), LeadDays: leadDays, Weekday: weekdays[weekday], Clock: clock}
}
