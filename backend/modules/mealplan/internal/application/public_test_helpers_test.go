package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type seamStore struct {
	ports.Store
	e *recordingEngine
}

func (s seamStore) ReplaceDay(ctx context.Context, date domain.Date, values []mealplan.Dish) (domain.OperationStats, error) {
	dishes := make([]mealplan.Dish, 0, len(values))
	for _, v := range values {
		dishes = append(dishes, mealplan.Dish{Dish: v.Dish, Note: v.Note})
	}
	return domain.OperationStats{}, s.e.Replace(ctx, date.String(), dishes)
}
func (s seamStore) InsertParticipationSchedule(ctx context.Context, studentID, accountID int64, date domain.Date, values []domain.Weekday) (domain.OperationStats, error) {
	weekdays := make([]mealplan.Weekday, 0, len(values))
	for _, v := range values {
		weekdays = append(weekdays, mealplan.Weekday(v))
	}
	_, err := s.e.ReplaceParticipationSchedule(ctx, studentID, accountID, weekdays)
	return domain.OperationStats{}, err
}

type seamTransaction struct{}

func (seamTransaction) Run(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type seamSettings struct{ e *recordingEngine }

func (s seamSettings) MealPlanEnabled(context.Context) (bool, error)       { return s.e.available, nil }
func (seamSettings) MealRegistrationEnabled(context.Context) (bool, error) { return true, nil }
func (seamSettings) MealParticipationCutoffs(context.Context) (mealplan.Cutoffs, error) {
	return mealplan.Cutoffs{CutoffTime: "09:00"}, nil
}

type seamCalendar struct{}

func (seamCalendar) NonWorkingDayDates(context.Context, string, string) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func newTestModule(e *recordingEngine) *Service {
	return New(seamStore{e: e}, struct{ ports.Directory }{}, seamSettings{e}, seamCalendar{}, seamTransaction{}, func(ports.Observation) {}, func() time.Time { return time.Date(2026, 9, 7, 8, 0, 0, 0, calendar.Berlin) })
}
