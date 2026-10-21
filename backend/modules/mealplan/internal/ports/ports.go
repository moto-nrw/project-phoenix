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
	UpsertParticipationOverrides(context.Context, int64, int64, []domain.Date, bool) (domain.OperationStats, error)
	DeleteParticipationOverride(context.Context, int64, domain.Date) (domain.OperationStats, error)
	FindDailyParticipation(context.Context, []int64, domain.Date, time.Time) (map[int64]domain.DailyParticipation, domain.OperationStats, error)
	// InsertRegistrationChanges appends to the registration log.
	InsertRegistrationChanges(context.Context, []domain.RegistrationChange) (domain.OperationStats, error)
	OrderStore
	SupplyStore
}

// SupplyStore holds the school's supply by its caterer (ADR 0075).
type SupplyStore interface {
	// FindActiveSupply reads the running supply; forUpdate locks it for the
	// unit of work.
	FindActiveSupply(ctx context.Context, forUpdate bool) (domain.Supply, bool, domain.OperationStats, error)
	InsertSupply(ctx context.Context, catererID int64, deliveryAddress string, accountID int64, startedAt time.Time) (domain.OperationStats, error)
	EndSupply(ctx context.Context, accountID int64, endedAt time.Time) (bool, domain.OperationStats, error)
	// ConfirmOrder confirms an order frozen since the supply started once;
	// false: no such order.
	ConfirmOrder(ctx context.Context, orderID, accountID int64, since time.Time, confirmedAt time.Time) (bool, domain.OperationStats, error)
}

// OrderStore holds the school's orders (#3873). LockOrders serializes every
// fixing and numbering of a school for the rest of the unit of work.
type OrderStore interface {
	LockOrders(context.Context) (domain.OperationStats, error)
	FindOrders(ctx context.Context, from, to domain.Date) (domain.OrderState, domain.OperationStats, error)
	InsertOrder(ctx context.Context, order domain.NewOrder, frozenAt time.Time) (int64, domain.OperationStats, error)
	// InsertOrderChanges numbers the changes after the school's last one.
	InsertOrderChanges(ctx context.Context, changes []domain.OrderChange, detectedAt time.Time) ([]domain.OrderChange, domain.OperationStats, error)
}

// Calendar is the school calendar of the tenant in the caller's unit of
// work. One read runs three statements: the federal state, the closing days
// and the holiday periods.
type Calendar interface {
	Days(context.Context, domain.Date, domain.Date) (domain.CalendarDays, domain.OperationStats, error)
}

type Directory interface {
	FindDailyCandidates(context.Context, domain.Date) ([]domain.DailyCandidate, domain.OperationStats, error)
	// FindStudentNames names the children of past orders, enrolled or not.
	FindStudentNames(context.Context, []int64) (map[int64]domain.DailyCandidate, domain.OperationStats, error)
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
	// CatererNames is gdpr.meal_caterer_names_enabled (ADR 0075).
	CatererNames bool
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
