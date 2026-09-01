package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

type Store interface {
	FindWeek(context.Context, domain.Date, domain.Date) ([]domain.Entry, domain.OperationStats, error)
	ReplaceDay(context.Context, domain.Date, []domain.Dish) (domain.OperationStats, error)
	ClearDay(context.Context, domain.Date) (domain.OperationStats, error)
	FindParticipation(context.Context, int64, domain.Date, domain.Date) (domain.ParticipationData, domain.OperationStats, error)
	InsertParticipationSchedule(context.Context, int64, int64, domain.Date, []domain.Weekday) (domain.OperationStats, error)
	UpsertParticipationOverride(context.Context, int64, int64, domain.Date, bool) (domain.OperationStats, error)
	DeleteParticipationOverride(context.Context, int64, domain.Date) (domain.OperationStats, error)
	FindDailyCandidates(context.Context, domain.Date, time.Time) ([]domain.DailyCandidate, domain.OperationStats, error)
}

type Settings interface {
	MealPlanEnabled(context.Context) (bool, error)
	MealRegistrationEnabled(context.Context) (bool, error)
	MealRegistrationCutoff(context.Context) (string, error)
}

type Transaction interface {
	Run(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
