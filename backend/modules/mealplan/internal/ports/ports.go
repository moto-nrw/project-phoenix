package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

type Store interface {
	FindWeek(context.Context, domain.Date, domain.Date) ([]domain.Entry, int64, error)
	ReplaceDay(context.Context, domain.Date, []domain.Dish) (int64, int64, time.Duration, error)
	ClearDay(context.Context, domain.Date) (int64, int64, time.Duration, error)
}

type Settings interface {
	MealPlanEnabled(context.Context) (bool, error)
}

type Transaction interface {
	Run(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Queries   int64
	Rows      int64
	LockWait  time.Duration
	Err       error
}

type Observer func(Observation)
