package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// parentScheduleDate also respects persisted orders: changing the school's
// deadlines or rhythm must not reopen days that have already been ordered.
func (s *Service) parentScheduleDate(ctx context.Context, rules domain.OrderingRules) (domain.Date, domain.OperationStats, error) {
	date := rules.NextEffectiveDate(s.now())
	// Deadlines reach at most two weeks back; the ordering reach also
	// covers weeks frozen under earlier rules.
	state, stats, err := s.store.FindOrders(ctx, date, date.AddDays(orderReachDays))
	if err != nil {
		return "", stats, err
	}
	// A pattern affects every later day too, including orders separated
	// from today by days that have not been ordered yet.
	for _, order := range state.Orders {
		for _, day := range order.Days {
			if !day.Before(date) {
				date = day.AddDays(1)
			}
		}
	}
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDays(1)
	}
	return date, stats, nil
}
