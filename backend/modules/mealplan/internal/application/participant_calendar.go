package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// participantCalendar keeps a failed range read off the remaining days. A
// later malformed calendar must not preempt the current day's roster failure.
func (s *Service) participantCalendar(ctx context.Context, date, through domain.Date, prepared *domain.CalendarDays) (domain.CalendarDays, *domain.CalendarDays, domain.OperationStats, error) {
	if prepared != nil {
		return *prepared, prepared, domain.OperationStats{}, nil
	}
	days, stats, err := s.calendar.Days(ctx, date, through)
	if err == nil {
		return days, &days, stats, nil
	}
	if through == date {
		return days, nil, stats, err
	}
	day, dayStats, err := s.calendar.Days(ctx, date, date)
	return day, nil, addStats(stats, dayStats), err
}
