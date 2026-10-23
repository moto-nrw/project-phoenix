package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

type Store interface {
	FindWeek(context.Context, domain.Date, domain.Date) ([]domain.Entry, domain.OperationStats, error)
	ReplaceDay(context.Context, domain.Date, []mealplan.Dish) (domain.OperationStats, error)
	ClearDay(context.Context, domain.Date) (domain.OperationStats, error)
	FindParticipation(context.Context, int64, domain.Date, domain.Date) (domain.ParticipationData, domain.OperationStats, error)
	InsertParticipationSchedule(context.Context, int64, int64, domain.Date, []domain.Weekday) (domain.OperationStats, error)
	UpsertParticipationOverride(context.Context, int64, int64, domain.Date, bool) (domain.OperationStats, error)
	DeleteParticipationOverride(context.Context, int64, domain.Date) (domain.OperationStats, error)
	FindDailyParticipation(context.Context, []int64, domain.Date, time.Time) (map[int64]domain.DailyParticipation, domain.OperationStats, error)
}

type Directory interface {
	FindDailyCandidates(context.Context, domain.Date) ([]mealplan.ParticipantCandidate, domain.OperationStats, error)
}

type Settings interface {
	MealPlanEnabled(context.Context) (bool, error)
	MealRegistrationEnabled(context.Context) (bool, error)
	// MealParticipationCutoffs are the school's registration and
	// cancellation cutoffs.
	MealParticipationCutoffs(context.Context) (mealplan.Cutoffs, error)
}

// Calendar is the school calendar of the caller's tenant.
type Calendar interface {
	// NonWorkingDayDates are the statutory holidays and closing days in
	// [from, to], as YYYY-MM-DD.
	NonWorkingDayDates(ctx context.Context, from, to string) (map[string]bool, error)
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
