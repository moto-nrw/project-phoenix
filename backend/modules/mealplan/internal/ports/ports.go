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
	UpsertParticipationOverrides(context.Context, int64, int64, []domain.Date, bool) (domain.OperationStats, error)
	DeleteParticipationOverride(context.Context, int64, domain.Date) (domain.OperationStats, error)
	FindDailyParticipation(context.Context, []int64, domain.Date, time.Time) (map[int64]domain.DailyParticipation, domain.OperationStats, error)
}

// Calendar is the school calendar of the tenant in the caller's unit of
// work. One read runs three statements: the federal state, the closing days
// and the holiday periods.
type Calendar interface {
	Days(context.Context, domain.Date, domain.Date) (domain.CalendarDays, domain.OperationStats, error)
}

type Directory interface {
	FindDailyCandidates(context.Context, domain.Date) ([]domain.DailyCandidate, domain.OperationStats, error)
}

type Settings interface {
	MealPlanEnabled(context.Context) (bool, error)
	MealRegistrationEnabled(context.Context) (bool, error)
	MealOrderingSettings(context.Context) (OrderingSettings, error)
}

// OrderingSettings are the school's meal ordering settings as stored (#3873).
// The application parses and validates them into domain.OrderingRules.
type OrderingSettings struct {
	Model                string
	RegistrationKind     string
	RegistrationLeadDays int
	RegistrationWeekday  string
	RegistrationTime     string
	CancellationKind     string
	CancellationLeadDays int
	CancellationWeekday  string
	CancellationTime     string
	AnnualDeadline       string
	AnnualChanges        string
	SickAfterDeadline    string
	OrderRhythm          string
	OrderDetail          string
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
